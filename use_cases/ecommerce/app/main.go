package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

const (
	customersPerMarket = 1
	browseFrequency    = time.Second * 1
	basketFrequency    = time.Second * 10
	checkoutFrequency  = time.Second * 30
)

func main() {
	log.SetFlags(0)

	databaseURL := mustLoadDatabaseURL()

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}
	log.Printf("[host]:    %q", db.Config().ConnConfig.Host)

	timeout, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = db.Ping(timeout); err != nil {
		log.Fatalf("error testing database connection: %v", err)
	}

	region, err := loadRegion(db)
	if err != nil {
		log.Fatalf("error loading region: %v", err)
	}
	log.Printf("[region]:  %q", region)

	markets, err := loadMarkets(db, region)
	if err != nil {
		log.Fatalf("error loading markets: %v", err)
	}
	log.Printf("[markets]: %q", markets)

	go func() {
		if err = workers(db, region, markets); err != nil {
			log.Fatalf("error in worker: %v", err)
		}
	}()

	server()
}

func mustLoadDatabaseURL() string {
	flyRegion, ok := os.LookupEnv("FLY_REGION")
	if ok {
		switch flyRegion {

		case "lhr", "fra", "ams", "cdg":
			log.Printf("using DATABASE_URL_EU for %q", flyRegion)
			url, ok := os.LookupEnv("DATABASE_URL_EU")
			if !ok {
				log.Fatalf("missing DATABASE_URL_EU env var")
			}
			return url

		case "iad":
			log.Printf("using DATABASE_URL_US for %q", flyRegion)
			url, ok := os.LookupEnv("DATABASE_URL_US")
			if !ok {
				log.Fatalf("missing DATABASE_URL_US env var")
			}
			return url

		case "sin":
			log.Printf("using DATABASE_URL_AP for %q", flyRegion)
			url, ok := os.LookupEnv("DATABASE_URL_AP")
			if !ok {
				log.Fatalf("missing DATABASE_URL_AP env var")
			}
			return url

		default:
			log.Fatalf("unexpected FLY_REGION: %q", flyRegion)
		}
	}

	databaseURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		log.Fatalf("missing DATABASE_URL env var")
	}

	return databaseURL
}

func server() {
	router := fiber.New()
	router.Use(cors.New())

	addr, ok := os.LookupEnv("LISTEN_ADDR")
	if !ok {
		addr = ":3000"
	}

	log.Fatal(router.Listen(addr))
}

func workers(db *pgxpool.Pool, region string, markets []string) error {
	var eg errgroup.Group
	for _, m := range markets {
		m := m

		for i := 0; i < customersPerMarket; i++ {
			eg.Go(func() error {
				// Stagger the workers.
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(10000)))
				return worker(db, region, m)
			})
		}
	}

	return eg.Wait()
}

type marketProduct struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

func worker(db *pgxpool.Pool, region, market string) error {
	customerID, err := createCustomer(db)
	if err != nil {
		return fmt.Errorf("creating customer: %w", err)
	}

	log.Printf("customer: %s, market: %s", customerID, market)

	browseTick := time.NewTicker(browseFrequency).C
	basketTick := time.NewTicker(basketFrequency).C
	checkoutTick := time.NewTicker(checkoutFrequency).C

	var products []marketProduct

	for {
		select {
		case <-browseTick:
			stagger()
			if products, err = browse(db, region, market); err != nil {
				log.Printf("browsing products (market = %q): %v", market, err)
				continue
			}

		case <-basketTick:
			stagger()
			p := lo.Sample(products)
			if err = basket(db, customerID, p.ID, region, 1); err != nil {
				log.Printf("adding product to basket (product_market = %q): %v", p.ID, err)
				continue
			}

		case <-checkoutTick:
			stagger()
			if err = checkout(db, customerID, region); err != nil {
				log.Printf("checking out: %v", err)
				continue
			}
		}
	}
}

func createCustomer(db *pgxpool.Pool) (string, error) {
	const stmt = `INSERT INTO customer (email) VALUES ($1) RETURNING id`

	row := db.QueryRow(context.Background(), stmt, gofakeit.Email())

	var id string
	if err := row.Scan(&id); err != nil {
		return "", fmt.Errorf("inserting customer: %w", err)
	}

	return id, nil
}

func browse(db *pgxpool.Pool, region, market string) ([]marketProduct, error) {
	start := time.Now()
	defer func() {
		log.Printf("[* BROWSE] %dms", time.Since(start).Milliseconds())
	}()

	const stmt = `SELECT
									mp.id, p.name, mp.amount, mp.currency
								FROM market_product mp
								JOIN product p ON mp.product_id = p.id
									AS OF SYSTEM TIME follower_read_timestamp()
								WHERE mp.crdb_region = $1
								AND mp.market = $2
								LIMIT 20`

	var products []marketProduct
	rows, err := db.Query(context.Background(), stmt, region, market)
	if err != nil {
		return nil, fmt.Errorf("querying products: %w", err)
	}

	for rows.Next() {
		var p marketProduct
		if err = rows.Scan(&p.ID, &p.Name, &p.Amount, &p.Currency); err != nil {
			return nil, fmt.Errorf("scanning product: %w", err)
		}
		products = append(products, p)
	}

	return products, nil
}

func basket(db *pgxpool.Pool, customerID, productID, region string, quantity uint) error {
	start := time.Now()
	defer func() {
		log.Printf("[* * BASKET] %dms", time.Since(start).Milliseconds())
	}()

	const stmt = `SELECT add_to_basket($1, $2, $3, $4)`

	if _, err := db.Exec(context.Background(), stmt, customerID, productID, region, quantity); err != nil {
		return fmt.Errorf("adding product to basket: %w", err)
	}

	return nil
}

func checkout(db *pgxpool.Pool, customerID, region string) error {
	start := time.Now()
	defer func() {
		log.Printf("[* * * CHECKOUT] %dms", time.Since(start).Milliseconds())
	}()

	err := crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		stmt := `SELECT checkout($1, $2)`
		row := tx.QueryRow(context.Background(), stmt, customerID, region)

		var purchaseID string
		if err := row.Scan(&purchaseID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		log.Printf("error checking out: %v", err)
		return fiber.NewError(fiber.StatusUnprocessableEntity, "error checking out")
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

func stagger() {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(250)))
}
