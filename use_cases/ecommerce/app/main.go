package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
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

	timeout, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = db.Ping(timeout); err != nil {
		log.Fatalf("error testing database connection: %v", err)
	}

	region, err := loadRegion(db)
	if err != nil {
		log.Fatalf("error loading region: %v", err)
	}

	markets, err := loadMarkets(db, region)
	if err != nil {
		log.Fatalf("error loading markets: %v", err)
	}

	fmt.Println(markets)

	go func() {
		if err = workers(db, markets); err != nil {
			log.Fatalf("error in worker: %v", err)
		}
	}()

	server(db)
}

func server(db *pgxpool.Pool) {
	router := fiber.New()
	router.Use(cors.New())

	addr, ok := os.LookupEnv("LISTEN_ADDR")
	if !ok {
		addr = ":3000"
	}

	log.Fatal(router.Listen(addr))
}

func workers(db *pgxpool.Pool, markets []string) error {
	var eg errgroup.Group
	for _, m := range markets {
		eg.Go(func() error {
			return worker(db, m)
		})
	}

	return eg.Wait()
}

type marketProduct struct {
}

func worker(db *pgxpool.Pool, market string) error {
	customerID, err := createCustomer(db)
	if err != nil {
		return fmt.Errorf("creating customer: %w", err)
	}

	browseTick := time.NewTicker(time.Second * 1).C
	basketTick := time.NewTicker(time.Second * 10).C
	checkoutTick := time.NewTicker(time.Second * 30).C

	var products []marketProduct

	for {
		select {
		case <-browseTick:
			if products, err = browse(db, market); err != nil {
				return fmt.Errorf("browsing products (market = %q): %w", market, err)
			}
		case <-basketTick:
		case <-checkoutTick:
		}
	}
}

func createCustomer(db *pgxpool.Pool) (string, error) {
	panic("implement me")
}

func browse(db *pgxpool.Pool, market string) ([]marketProduct, error) {
	panic("implement me")
}

func basket(db *pgxpool.Pool, market string) ([]marketProduct, error) {
	panic("implement me")
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

func loadMarkets(db *pgxpool.Pool, region string) ([]string, error) {
	const stmt = `SELECT market FROM region_market
								WHERE region = $1`

	rows, err := db.Query(context.Background(), stmt, region)
	if err != nil {
		return nil, fmt.Errorf("running query: %w", err)
	}

	var markets []string
	for rows.Next() {
		var m string
		if err = rows.Scan(&m); err != nil {
			return nil, fmt.Errorf("scanning market: %w", err)
		}
		markets = append(markets, m)
	}

	return markets, nil
}
