package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"
)

func main() {
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

	mustClear(db)

	aircraft := mustPopulateAircraft(db)

	mustPopulateAircraftAdministration(db, aircraft)
	mustPopulateAircraftDiagnostics(db, aircraft)
}

func mustClear(db *pgxpool.Pool) {
	fmt.Println("clearing tables...")

	stmt := `DELETE FROM aircraft_diagnostics WHERE true`
	if _, err := db.Exec(context.Background(), stmt); err != nil {
		log.Fatalf("error clearing diagnostics data")
	}

	stmt = `DELETE FROM aircraft_administration WHERE true`
	if _, err := db.Exec(context.Background(), stmt); err != nil {
		log.Fatalf("error clearing administration data")
	}

	stmt = `DELETE FROM aircraft WHERE true`
	if _, err := db.Exec(context.Background(), stmt); err != nil {
		log.Fatalf("error clearing aircraft data")
	}

	fmt.Println("cleared tables")
}

type aircraft struct {
	id               string
	category         string
	model            string
	capacity         int
	rangeKM          int
	base             string
	operationalSince time.Time
}

func mustPopulateAircraft(db *pgxpool.Pool) []aircraft {
	fmt.Println("populating aircraft...")

	stmt := `INSERT INTO aircraft (
					   id,
						 category,
						 model,
						 capacity,
						 range_km,
						 base,
						 operational_since
					 ) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	var aircraft []aircraft
	batch := pgx.Batch{}
	for i := 0; i < 50; i++ {
		a := randomAircraft()
		switch i {
		case 0:
			a = coolPlane()
		case 1:
			a = ufo()
		}

		batch.Queue(
			stmt,
			a.id,
			a.category,
			a.model,
			a.capacity,
			a.rangeKM,
			a.base,
			a.operationalSince,
		)

		aircraft = append(aircraft, a)
	}

	if _, err := db.SendBatch(context.Background(), &batch).Exec(); err != nil {
		log.Fatalf("error populating aircraft: %v", err)
	}

	fmt.Println("populated aircraft")
	return aircraft
}

type aircraftAdministration struct {
	nextService time.Time
	flightHours int
}

func mustPopulateAircraftAdministration(db *pgxpool.Pool, aircraft []aircraft) {
	fmt.Println("populating aircraft administration...")

	stmt := `INSERT INTO aircraft_administration (
					   aircraft_id,
						 next_service,
						 flight_hours
					 ) VALUES ($1, $2, $3)`

	batch := pgx.Batch{}
	for _, a := range aircraft {
		admin := randomAircraftAdministration()

		batch.Queue(
			stmt,
			a.id,
			admin.nextService,
			admin.flightHours,
		)

		aircraft = append(aircraft, a)
	}

	if _, err := db.SendBatch(context.Background(), &batch).Exec(); err != nil {
		log.Fatalf("error populating aircraft: %v", err)
	}

	fmt.Println("populated aircraft administration")
}

type aircraftDiagnostics struct {
	batteryLevel float32
	lat          float64
	lon          float64
	heading      int
}

func mustPopulateAircraftDiagnostics(db *pgxpool.Pool, aircraft []aircraft) {
	fmt.Println("populating aircraft diagnostics...")

	stmtFmt := `INSERT INTO aircraft_diagnostics (
								aircraft_id,
								battery_level,
								location,
								heading,
								last_update_latency_ms
							) VALUES ($1, $2, 'POINT(%f %f)', $3, $4)`

	batch := pgx.Batch{}
	for _, a := range aircraft {
		d := randomAircraftDiagnostics()
		if a.category == "cool_plane" {
			d.lat, d.heading = 35, 90
		}

		batch.Queue(
			fmt.Sprintf(stmtFmt, d.lon, d.lat),
			a.id,
			d.batteryLevel,
			d.heading,
			0,
		)

		aircraft = append(aircraft, a)
	}

	if _, err := db.SendBatch(context.Background(), &batch).Exec(); err != nil {
		log.Fatalf("error populating aircraft: %v", err)
	}

	fmt.Println("populated aircraft diagnostics")
}

type aircraftType struct {
	category string
	models   []string
	capacity [2]int
	rangeKM  [2]int
}

var aircraftTypes = []aircraftType{
	{"fixed_wing", []string{"Fixed-wing transport", "Fixed-wing cargo", "Fixed-wing ISR"}, [2]int{20, 120}, [2]int{3000, 9000}},
	{"helicopter", []string{"Utility helicopter", "Medevac helicopter", "Heavy-lift helicopter"}, [2]int{4, 30}, [2]int{400, 900}},
	{"drone", []string{"Small UAS", "Medium UAS", "Long-endurance UAS"}, [2]int{0, 1}, [2]int{50, 3000}},
}

func randomAircraft() aircraft {
	t := lo.Sample(aircraftTypes)
	return aircraft{
		id:               uuid.NewString(),
		category:         t.category,
		model:            lo.Sample(t.models),
		capacity:         t.capacity[0] + rand.Intn(t.capacity[1]-t.capacity[0]+1),
		rangeKM:          t.rangeKM[0] + rand.Intn(t.rangeKM[1]-t.rangeKM[0]+1),
		base:             lo.Sample([]string{"LAX", "IAD", "LHR", "CDG", "SIN", "BOM"}),
		operationalSince: gofakeit.DateRange(time.Now().Add(-time.Hour*87600), time.Now()),
	}
}

func ufo() aircraft {
	return aircraft{
		id:               uuid.NewString(),
		category:         "ufo",
		model:            "Unidentified",
		capacity:         0,
		rangeKM:          999999,
		base:             "???",
		operationalSince: time.Date(1947, time.July, 8, 0, 0, 0, 0, time.UTC),
	}
}

func coolPlane() aircraft {
	return aircraft{
		id:               uuid.NewString(),
		category:         "cool_plane",
		model:            "Cool Plane (SR-71)",
		capacity:         2,
		rangeKM:          5400,
		base:             "LAX",
		operationalSince: time.Date(1966, time.January, 7, 0, 0, 0, 0, time.UTC),
	}
}

func randomAircraftDiagnostics() aircraftDiagnostics {
	return aircraftDiagnostics{
		batteryLevel: 100,
		lat:          gofakeit.Float64Range(-45, 60),
		lon:          gofakeit.Longitude(),
		heading:      lo.Sample([]int{90, 270}) + rand.Intn(51) - 25,
	}
}

func randomAircraftAdministration() aircraftAdministration {
	return aircraftAdministration{
		nextService: gofakeit.DateRange(time.Now(), time.Now().Add(time.Hour*8760)),
		flightHours: rand.Intn(100000-1000) + 1000,
	}
}
