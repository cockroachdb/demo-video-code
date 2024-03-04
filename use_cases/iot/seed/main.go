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
	model            string
	capacity         int
	rangeKM          int
	base             string
	operationalSince time.Time
}

func mustPopulateAircraft(db *pgxpool.Pool) []aircraft {
	fmt.Println("populating aircraft...")

	stmt := `INSERT INTO aircraft (id, model, capacity, range_km, base, operational_since) VALUES ($1, $2, $3, $4, $5, $6)`

	var aircraft []aircraft
	batch := pgx.Batch{}
	for i := 0; i < 50; i++ {
		a := randomAircraft()

		batch.Queue(
			stmt,
			a.id,
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

	stmt := `INSERT INTO aircraft_administration (aircraft_id, next_service, flight_hours) VALUES ($1, $2, $3)`

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
	batteryLevel        float32
	lat                 float64
	lon                 float64
	heading             int
	lastUpdateLatencyMS int
}

func mustPopulateAircraftDiagnostics(db *pgxpool.Pool, aircraft []aircraft) {
	fmt.Println("populating aircraft diagnostics...")

	stmtFmt := `INSERT INTO aircraft_diagnostics (aircraft_id, battery_level, location, heading, last_update_latency_ms) VALUES ($1, $2, 'POINT(%f %f)', $3, $4)`

	batch := pgx.Batch{}
	for _, a := range aircraft {
		d := randomAircraftDiagnostics()

		batch.Queue(
			fmt.Sprintf(stmtFmt, d.lat, d.lon),
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

func randomAircraft() aircraft {
	return aircraft{
		id:               uuid.NewString(),
		model:            lo.Sample([]string{"A220", "A300", "A310", "A320", "A330", "A340", "A350", "A380"}),
		capacity:         rand.Intn(500-50) + 50,
		rangeKM:          rand.Intn(15000-5000) + 5000,
		base:             lo.Sample([]string{"LAX", "IAD", "LHR", "CDG", "SIN", "BOM"}),
		operationalSince: gofakeit.DateRange(time.Now().Add(-time.Hour*87600), time.Now()),
	}
}

func randomAircraftDiagnostics() aircraftDiagnostics {
	return aircraftDiagnostics{
		batteryLevel: 100,
		lat:          gofakeit.Latitude(),
		lon:          gofakeit.Longitude(),
		heading:      rand.Intn(360),
	}
}

func randomAircraftAdministration() aircraftAdministration {
	return aircraftAdministration{
		nextService: gofakeit.DateRange(time.Now(), time.Now().Add(time.Hour*8760)),
		flightHours: rand.Intn(100000-1000) + 1000,
	}
}
