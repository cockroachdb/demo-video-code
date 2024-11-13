package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

func main() {
	url, ok := os.LookupEnv("CONNECTION_STRING")
	if !ok {
		log.Fatalf("missing CONNECTION_STRING env var")
	}

	db, err := sql.Open("mysql", url)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(time.Minute * 1)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	log.Fatal(work(db))
}

func work(db *sql.DB) error {
	for range time.NewTicker(time.Second * 1).C {
		id := uuid.NewString()

		// Insert purchase.
		stmt := `INSERT INTO purchase (id, basket_id, member_id, amount) VALUES (?, ?, ?, ?)`
		if _, err := db.Exec(stmt, id, uuid.NewString(), uuid.NewString(), rand.Float64()*100); err != nil {
			return fmt.Errorf("inserting purchase: %w", err)
		}

		// Select purchase.
		stmt = `SELECT amount FROM purchase WHERE id = ?`
		row := db.QueryRow(stmt, id)

		var value float64
		if err := row.Scan(&value); err != nil {
			return fmt.Errorf("selecting purchase: %w", err)
		}

		// Select database version.
		stmt = `SELECT version()`
		row = db.QueryRow(stmt)

		var version string
		if err := row.Scan(&version); err != nil {
			return fmt.Errorf("selecting version: %w", err)
		}

		// Feedback.
		log.Printf("ok (%s)", strings.Split(version, "(")[0])
	}

	return fmt.Errorf("application unexpectedly exited")
}
