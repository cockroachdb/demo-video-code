package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/codingconcepts/ring"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

const customersPerMarket = 10

var (
	averageBrowseLatency   = ring.New[time.Duration](100)
	averageBasketLatency   = ring.New[time.Duration](100)
	averageCheckoutLatency = ring.New[time.Duration](100)
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

	go printLoop()

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
		m := m

		for i := 0; i < customersPerMarket; i++ {
			eg.Go(func() error {
				// Stagger the workers.
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(10000)))
				return worker(db, m)
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

func worker(db *pgxpool.Pool, market string) error {
	customerID, err := createCustomer(db)
	if err != nil {
		return fmt.Errorf("creating customer: %w", err)
	}

	log.Printf("customer: %s, market: %s", customerID, market)

	browseTick := time.NewTicker(time.Second * 1).C
	basketTick := time.NewTicker(time.Second * 10).C
	checkoutTick := time.NewTicker(time.Second * 30).C

	var products []marketProduct

	for {
		select {
		case <-browseTick:
			start := time.Now()

			if products, err = browse(db, market); err != nil {
				return fmt.Errorf("browsing products (market = %q): %w", market, err)
			}
			averageBrowseLatency.Add(time.Since(start))

		case <-basketTick:
			start := time.Now()

			p := lo.Sample(products)
			if err = basket(db, customerID, p.ID, 1); err != nil {
				return fmt.Errorf("adding product to basket (product_market = %q): %w", p.ID, err)
			}
			averageBasketLatency.Add(time.Since(start))

		case <-checkoutTick:
			start := time.Now()

			if err = checkout(db, customerID); err != nil {
				return fmt.Errorf("checking out: %w", err)
			}
			averageCheckoutLatency.Add(time.Since(start))

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

func browse(db *pgxpool.Pool, market string) ([]marketProduct, error) {
	const stmt = `SELECT
									mp.id, p.name, mp.amount, mp.currency
								FROM market_product mp
								JOIN product p ON mp.product_id = p.id
								WHERE mp.market = $1
								AND mp.amount BETWEEN $2 AND $3`

	min := rand.Intn(255 - 10)
	max := min + 10

	rows, err := db.Query(context.Background(), stmt, market, min, max)
	if err != nil {
		return nil, fmt.Errorf("querying products: %w", err)
	}

	var products []marketProduct
	for rows.Next() {
		var p marketProduct
		if err = rows.Scan(&p.ID, &p.Name, &p.Amount, &p.Currency); err != nil {
			return nil, fmt.Errorf("scanning product: %w", err)
		}
		products = append(products, p)
	}

	return products, nil
}

func basket(db *pgxpool.Pool, customerID, productID string, quantity uint) error {
	const stmt = "SELECT add_to_basket($1, $2, $3)"

	if _, err := db.Exec(context.Background(), stmt, customerID, productID, quantity); err != nil {
		return fmt.Errorf("adding product to basket: %w", err)
	}

	return nil
}

func checkout(db *pgxpool.Pool, customerID string) error {
	var purchaseID string

	err := crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		// One last stock check before checking out.
		stmt := `CALL can_checkout($1)`
		if _, err := tx.Exec(context.Background(), stmt, customerID); err != nil {
			return err
		}

		// Create order.
		stmt = `INSERT INTO purchase ("customer_id", "market_product_ids", "amount") VALUES
				($1, basket_products($1), basket_total($1)) RETURNING id`

		row := tx.QueryRow(context.Background(), stmt, customerID)
		if err := row.Scan(&purchaseID); err != nil {
			return err
		}

		// Update stock.
		stmt = `CALL remove_purchase_stock($1);`
		if _, err := tx.Exec(context.Background(), stmt, customerID); err != nil {
			return err
		}

		// Delete customer basket.
		stmt = `DELETE FROM basket WHERE customer_id = $1`
		if _, err := tx.Exec(context.Background(), stmt, customerID); err != nil {
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

func printLoop() {
	browseTick := time.NewTicker(time.Second * 2).C
	basketTick := time.NewTicker(time.Second * 20).C
	checkoutTick := time.NewTicker(time.Second * 60).C

	for {
		select {
		case <-browseTick:
			printAverageLatencies(averageBrowseLatency, "browse")

		case <-basketTick:
			printAverageLatencies(averageBasketLatency, "basket")

		case <-checkoutTick:
			printAverageLatencies(averageCheckoutLatency, "checkout")
		}
	}
}

func printAverageLatencies(r *ring.Ring[time.Duration], name string) {
	latencies := r.Slice()
	sum := lo.Sum(latencies)
	log.Printf(
		"[%s] avg latency: %dms (samples: %d)",
		name,
		sum.Milliseconds()/int64(len(latencies)),
		len(latencies),
	)
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

func rollingAverage(period int) func(time.Duration) time.Duration {
	var i int
	var sum time.Duration
	var storage = make([]time.Duration, 0, period)
	var mu sync.Mutex

	return func(input time.Duration) time.Duration {
		mu.Lock()
		defer mu.Unlock()

		if len(storage) < period {
			sum += input
			storage = append(storage, input)
		}

		sum += input - storage[i]
		storage[i], i = input, (i+1)%period
		return sum / time.Duration(len(storage))
	}
}
