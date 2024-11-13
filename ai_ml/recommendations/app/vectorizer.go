package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

const (
	minAge, maxAge  = 18, 100
	updateBatchSize = 5000
)

var (
	genderAgeWeightings = []float32{
		// Gender features.
		0.2, 0.2, 0.2,
		// Date of birth.
		0.2,
		// Location (lat/long).
		0.1, 0.1,
	}
)

func main() {
	databaseURL := flag.String("database-url", "", "url to the database")
	flag.Parse()

	if *databaseURL == "" {
		flag.Usage()
		os.Exit(2)
	}

	db, err := pgxpool.New(context.Background(), *databaseURL)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}
	defer db.Close()

	err = vectorize(db)
	if err != nil {
		log.Fatalf("error vectorizing: %v", err)
	}
}

type customer struct {
	id          string
	gender      string
	dateOfBirth time.Time
	lat         float64
	lon         float64
}

func vectorize(db *pgxpool.Pool) error {
	for {
		customers, err := fetchCustomers(db, updateBatchSize)
		if err != nil {
			return fmt.Errorf("fetching customers: %w", err)
		}
		fmt.Printf("fetched %d customers\n", len(customers))

		if len(customers) == 0 {
			break
		}

		if err = updateCustomers(db, customers, genderAgeWeightings); err != nil {
			return fmt.Errorf("updating customers: %w", err)
		}
	}

	return nil
}

func fetchCustomers(db *pgxpool.Pool, limit int) ([]customer, error) {
	const stmt = `SELECT
									id,
									gender,
									date_of_birth,
									ST_Y(location),
									ST_X(location)
								FROM customer
								WHERE vec IS NULL
								AND gender IS NOT NULL
								LIMIT $1`

	rows, err := db.Query(context.Background(), stmt, limit)
	if err != nil {
		if err == pgx.ErrNoRows {
			return []customer{}, nil
		}
		return nil, fmt.Errorf("querying: %w", err)
	}

	var customers []customer
	var c customer
	for rows.Next() {
		err = rows.Scan(&c.id, &c.gender, &c.dateOfBirth, &c.lat, &c.lon)
		if err != nil {
			return nil, fmt.Errorf("scanning customer: %w", err)
		}

		customers = append(customers, c)
	}

	return customers, nil
}

func updateCustomers(db *pgxpool.Pool, customers []customer, weightings []float32) error {
	const stmt = `UPDATE customer SET vec = $1 WHERE id = $2`

	conn, err := db.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	batch := &pgx.Batch{}
	for _, c := range customers {
		batch.Queue(stmt, toVec(c, weightings), c.id)
	}

	_, err = conn.SendBatch(context.Background(), batch).Exec()
	return err
}

func toVec(c customer, weightings []float32) pgvector.Vector {
	var vec []float32

	vec = append(vec, normalizeGender(c.gender)...)
	vec = append(vec, normalizeDateOfBirth(c.dateOfBirth))
	vec = append(vec, normalizeLocation(c.lat, c.lon)...)

	vec = applyWeightings(vec, weightings)

	return pgvector.NewVector(vec)
}

func normalizeGender(g string) []float32 {
	// Perform a one-hot (or one-of-k) encoding of the genders, as
	// no ordinal relationship exists between the genders.
	switch strings.ToLower(g) {
	case "male", "trans-male":
		return []float32{0, 0, 1}
	case "female", "trans-female":
		return []float32{0, 1, 0}
	case "non-binary":
		return []float32{1, 0, 0}
	default:
		panic("unsupported gender")
	}
}

func normalizeDateOfBirth(dob time.Time) float32 {
	// Get a numeric value between the min and max date based
	// on a person's age.
	age := time.Since(dob).Hours() / 24 / 365
	return float32((age - minAge) / (maxAge - minAge))
}

func normalizeLocation(lat, lon float64) []float32 {
	// Convert to radians.
	latRad := lat * math.Pi / 180
	lonRad := lon * math.Pi / 180

	// Convert to Cartesian coordinates.
	x := math.Cos(latRad) * math.Cos(lonRad)
	y := math.Cos(latRad) * math.Sin(lonRad)

	return []float32{float32(x), float32(y)}
}

func applyWeightings(v []float32, w []float32) []float32 {
	// Apply emphasis to specified indices.
	for i, w := range w {
		if i >= 0 && i < len(v) {
			v[i] *= w
		}
	}

	return v
}
