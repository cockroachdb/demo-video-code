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

	databaseCert, ok := os.LookupEnv("DATABASE_CERT")
	if ok {
		if err := writeCert(databaseCert); err != nil {
			log.Fatalf("error writing cert: %v", err)
		}
		databaseURL += "&sslrootcert=ca.crt"
	}

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

	server(db)
}

func mustLoadDatabaseURL() string {
	flyRegion, ok := os.LookupEnv("FLY_REGION")
	if ok {
		switch flyRegion {
		case "lhr", "fra":
			url, ok := os.LookupEnv("DATABASE_URL_EU")
			if !ok {
				log.Fatalf("missing DATABASE_URL_EU env var")
			}
			return url
		case "iad":
			url, ok := os.LookupEnv("DATABASE_URL_US")
			if !ok {
				log.Fatalf("missing DATABASE_URL_US env var")
			}
			return url
		case "sin":
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

func server(db *pgxpool.Pool) {
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
			if products, err = browse(db, region, market); err != nil {
				log.Printf("browsing products (market = %q): %v", market, err)
				continue
			}

		case <-basketTick:
			p := lo.Sample(products)
			if err = basket(db, customerID, p.ID, region, 1); err != nil {
				log.Printf("adding product to basket (product_market = %q): %v", p.ID, err)
				continue
			}

		case <-checkoutTick:
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
								AND mp.amount BETWEEN $3 AND $4
								LIMIT 20`

	min := rand.Intn(255 - 10)
	max := min + 10

	var products []marketProduct
	rows, err := db.Query(context.Background(), stmt, region, market, min, max)
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

	var purchaseID string

	err := crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		// One last stock check before checking out.
		stmt := `CALL can_checkout($1, $2)`
		if _, err := tx.Exec(context.Background(), stmt, customerID, region); err != nil {
			return err
		}

		// Create order.
		stmt = `INSERT INTO purchase ("customer_id", "market_product_ids", "amount", "crdb_region")
						VALUES
							(
								$1,
								basket_products($1, $2),
								basket_total($1, $2),
								$2
							) RETURNING id`

		row := tx.QueryRow(context.Background(), stmt, customerID, region)
		if err := row.Scan(&purchaseID); err != nil {
			return err
		}

		// Update stock.
		stmt = `CALL remove_purchase_stock($1, $2);`
		if _, err := tx.Exec(context.Background(), stmt, customerID, region); err != nil {
			return err
		}

		// Delete customer basket.
		stmt = `DELETE FROM basket WHERE customer_id = $1 AND crdb_region = $2`
		if _, err := tx.Exec(context.Background(), stmt, customerID, region); err != nil {
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

func writeCert(content string) error {
	file, err := os.Create("ca.crt")
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	if _, err = file.WriteString(content); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	log.Println("wrote certificate")
	return nil
}

// RandomTicker represents a ticker that emits ticks at random intervals
// within a specified range.
type RandomTicker struct {
	C        chan time.Time
	min, max time.Duration
	done     chan struct{}
}

// NewRandomTicker creates a new RandomFreqTicker with specified minimum and maximum durations.
func NewRandomTicker(min, max time.Duration) *RandomTicker {
	rft := &RandomTicker{
		min:  min,
		max:  max,
		C:    make(chan time.Time, 1),
		done: make(chan struct{}, 1),
	}
	go rft.startTicking()
	return rft
}

// startTicking starts the internal goroutine that emits ticks.
func (rft *RandomTicker) startTicking() {
	for {
		select {
		case <-rft.done:
			return
		case <-time.After(rft.randomDuration()):
			rft.C <- time.Now()
		}
	}
}

// randomDuration generates a random duration within the ticker's range.
func (rft *RandomTicker) randomDuration() time.Duration {
	delta := rft.max - rft.min
	if delta <= 0 {
		return rft.min
	}
	return rft.min + time.Duration(rand.Int63n(int64(delta)))
}

// Stop stops the ticker, closing the tick channel.
func (rft *RandomTicker) Stop() {
	close(rft.done)
}
