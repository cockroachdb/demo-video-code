package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log.SetFlags(0)

	databaseURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		log.Fatalf("missing DATABASE_URL env var")
	}

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}

	if err = db.Ping(context.Background()); err != nil {
		log.Fatalf("error testing database connection: %v", err)
	}

	region, err := loadRegion(db)
	if err != nil {
		log.Fatalf("error loading region: %v", err)
	}

	log.Printf("running in region: %q", region)

	go work(db, region)

	router := fiber.New()
	router.Get("/", handleGetAircraftDiagnostics(db))

	log.Fatal(router.Listen(":3000"))
}

func handleGetAircraftDiagnostics(db *pgxpool.Pool) fiber.Handler {
	const stmt = `SELECT
									aircraft_id,
									battery_level,
									st_x(location),
									st_y(location),
									heading,
									crdb_region,
									last_update
								FROM aircraft_diagnostics`

	return func(c *fiber.Ctx) error {
		rows, err := db.Query(c.Context(), stmt)
		if err != nil {
			log.Printf("error fetching locations: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error fetching locations")
		}

		var diagnostics []aircraftDiagnostics
		for rows.Next() {
			var d aircraftDiagnostics
			if err = rows.Scan(&d.AircraftID, &d.BatteryLevel, &d.Lon, &d.Lat, &d.Heading, &d.Region, &d.LastUpdate); err != nil {
				log.Printf("error scanning location: %v", err)
				return fiber.NewError(fiber.StatusInternalServerError, "error scanning location")
			}

			diagnostics = append(diagnostics, d)
		}

		return c.JSON(diagnostics)
	}
}

type aircraftDiagnostics struct {
	AircraftID   string    `json:"aircraft_id"`
	BatteryLevel float64   `json:"battery_id"`
	Lat          float64   `json:"lat"`
	Lon          float64   `json:"lon"`
	Heading      int       `json:"heading"`
	Region       string    `json:"region"`
	LastUpdate   time.Time `json:"last_update"`
}

func work(db *pgxpool.Pool, region string) {
	for range time.NewTicker(time.Second * 1).C {
		aircraft, err := fetchAircraft(db, region)
		if err != nil {
			log.Printf("error fetching aircraft: %v", err)
			continue
		}

		if err = updateAircraft(db, aircraft); err != nil {
			log.Printf("error moving aircraft: %v", err)
			continue
		}
	}
}

func fetchAircraft(db *pgxpool.Pool, region string) ([]aircraftDiagnostics, error) {
	const stmt = `SELECT aircraft_id, battery_level, ST_Y(location), ST_X(location), heading
								FROM aircraft_diagnostics
								WHERE crdb_region = $1`

	rows, err := db.Query(context.Background(), stmt, region)
	if err != nil {
		return nil, fmt.Errorf("querying aircraft: %w", err)
	}

	var results []aircraftDiagnostics
	for rows.Next() {
		r := aircraftDiagnostics{}
		if err = rows.Scan(&r.AircraftID, &r.BatteryLevel, &r.Lat, &r.Lon, &r.Heading); err != nil {
			return nil, fmt.Errorf("scanning aircraft: %w", err)
		}

		results = append(results, r)
	}

	return results, nil
}

func updateAircraft(db *pgxpool.Pool, aircraft []aircraftDiagnostics) error {
	const stmtFmt = `UPDATE aircraft_diagnostics SET
									   battery_level = $1,
										 location = 'POINT(%f %f)',
										 last_update = now()
									 WHERE aircraft_id = $2`

	batch := pgx.Batch{}
	for _, a := range aircraft {
		lat, lon := moveAircraft(a.Lat, a.Lon, 100, float64(a.Heading))

		batch.Queue(
			fmt.Sprintf(stmtFmt, lat, lon),
			a.BatteryLevel-0.25,
			a.AircraftID,
		)
	}

	if _, err := db.SendBatch(context.Background(), &batch).Exec(); err != nil {
		log.Fatalf("error moving aircraft: %v", err)
	}

	return nil
}

func loadRegion(db *pgxpool.Pool) (string, error) {
	// Allow the environment to override the region.
	region, ok := os.LookupEnv("REGION")
	if ok {
		return region, nil
	}

	const stmt = `SELECT gateway_region()`

	row := db.QueryRow(context.Background(), stmt)
	if err := row.Scan(&region); err != nil {
		return "", fmt.Errorf("fetching gateway region: %w", err)
	}

	return region, nil
}

func moveAircraft(lat, lon, distance, bearing float64) (float64, float64) {
	earthRadius := 6378.1
	bearingR := bearing * math.Pi / 180
	latR := lat * math.Pi / 180
	lonR := lon * math.Pi / 180

	latR = math.Asin(math.Sin(latR)*math.Cos(distance/earthRadius) +
		math.Cos(latR)*math.Sin(distance/earthRadius)*math.Cos(bearingR))

	lonR = lonR + math.Atan2(math.Sin(bearingR)*math.Sin(distance/earthRadius)*math.Cos(latR),
		math.Cos(distance/earthRadius)-math.Sin(latR)*math.Sin(latR))

	return latR * 180 / math.Pi, lonR * 180 / math.Pi
}
