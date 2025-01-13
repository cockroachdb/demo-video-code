package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

func main() {
	databaseURL := flag.String("database-url", "", "url to the database")
	shoppers := flag.Int("shoppers", 1, "number of shoppers to simulate")
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

	if err = simulateShoppers(db, *shoppers); err != nil {
		log.Fatalf("error simulating shoppers: %v", err)
	}
}

func simulateShoppers(db *pgxpool.Pool, count int) error {
	eg := errgroup.Group{}

	for i := 0; i < count; i++ {
		eg.Go(func() error {
			return simulateShopper(db)
		})
	}

	return eg.Wait()
}

func simulateShopper(db *pgxpool.Pool) error {
	// Create account.
	id, err := createShopper(db)
	if err != nil {
		return fmt.Errorf("creating shopper: %w", err)
	}

	browseTicks := time.Tick(time.Second * time.Duration(rand.IntN(5)+1))
	purchaseTicks := time.Tick(time.Second * time.Duration(rand.IntN(15)+1))

	for {
		select {
		case <-browseTicks:
			if err = browse(db); err != nil {
				log.Printf("error browsing for products: %v", err)
				continue
			}
			fmt.Println("browse")

		case <-purchaseTicks:
			if err = purchase(db, id); err != nil {
				log.Printf("error placing order: %v", err)
				continue
			}
			fmt.Println("purchase")
		}
	}
}

type product struct {
	ID    string  `json:"id"`
	Price float64 `json:"price"`
}

func createShopper(db *pgxpool.Pool) (string, error) {
	const stmt = `INSERT INTO member (id, email) VALUES ($1, $2)`

	id := gofakeit.UUID()
	email := gofakeit.Email()

	if _, err := db.Exec(context.Background(), stmt, id, email); err != nil {
		return "", fmt.Errorf("executing query: %w", err)
	}

	return id, nil
}

func browse(db *pgxpool.Pool) error {
	const stmt = `SELECT id, name FROM product LIMIT 10`

	rows, err := db.Query(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("executing query: %w", err)
	}
	rows.Close()

	return nil
}

func purchase(db *pgxpool.Pool, memberID string) error {
	const stmt = `INSERT INTO purchase (member_id, amount, receipt, ts) VALUES ($1, $2, $3, $4)`

	products, total, err := randomProductsJSON()
	if err != nil {
		return fmt.Errorf("creating products: %w", err)
	}

	ts := gofakeit.DateRange(time.Now().Truncate(time.Hour*24), time.Now())

	_, err = db.Exec(context.Background(), stmt, memberID, total, products, ts)
	if err != nil {
		return fmt.Errorf("executing query: %w", err)
	}

	return nil
}

func randomProductsJSON() (string, float64, error) {
	var products []product
	var total float64

	productCount := rand.IntN(5) + 1
	for i := 0; i < productCount; i++ {
		p := product{
			ID:    gofakeit.UUID(),
			Price: gofakeit.Price(1, 100),
		}

		products = append(products, p)
		total += p.Price
	}

	b, err := json.Marshal(products)
	if err != nil {
		return "", 0, fmt.Errorf("marshalling products: %w", err)
	}

	return string(b), total, nil
}
