package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

const (
	accounts      = 10000
	hourFrequency = time.Second * 10 // Simulates 24h in 4m.
)

func main() {
	log.SetFlags(0)

	zone, region, url, err := getLocalSettings()
	if err != nil {
		log.Fatalf("error getting timezone: %v", err)
	}

	workerCount := runtime.NumCPU()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatalf("error parsing connection string: %v", err)
	}
	cfg.MaxConns = int32(workerCount)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("error connecting to db: %v", err)
	}
	defer db.Close()

	ids, err := fetchIDs(db, accounts, region)
	if err != nil {
		log.Fatalf("error fetching ids: %v", err)
	}

	currentHour := time.Now().In(zone).Hour()
	work := make(chan struct{}, workerCount)

	// Print app info.
	log.Printf("timezone:   %q", zone.String())
	log.Printf("start hour: %d", currentHour)
	log.Printf("region:     %q", region)
	log.Printf("cpus:       %d", workerCount)

	go simulateDay(currentHour, hourFrequency, work)

	if err = workers(db, ids, workerCount, work, region); err != nil {
		log.Fatalf("error in worker: %v", err)
	}
}

func workers(db *pgxpool.Pool, ids []string, count int, work <-chan struct{}, region string) error {
	var eg errgroup.Group

	for range count {
		eg.Go(func() error {
			return worker(db, ids, work, region)
		})
	}

	return eg.Wait()
}

func worker(db *pgxpool.Pool, ids []string, work <-chan struct{}, region string) error {
	for range work {
		pair := lo.Samples(ids, 2)

		err := performTransfer(db, pair[0], pair[1], region, rand.IntN(100))
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("error: %v", err)
			continue
		}
	}

	return nil
}

func simulateDay(startHour int, hourFrequency time.Duration, work chan<- struct{}) {
	currentHour := startHour

	minuteTicks := time.Tick(hourFrequency)
	workTicks := time.Tick(getWorkTickFrequency(currentHour))

	printTicks := time.Tick(time.Second)
	var requestCount int

	for {
		select {
		case <-minuteTicks:
			currentHour = (currentHour + 1) % 24
			fmt.Printf("simluating hour: %d\n", currentHour)

			workTicks = time.Tick(getWorkTickFrequency(currentHour))

		case <-workTicks:
			work <- struct{}{}
			requestCount++

		case <-printTicks:
			log.Printf("hour: %s requests/s: %d", fmt.Sprintf("%02d:00", currentHour), requestCount)
			requestCount = 0
		}
	}
}

/*
00:00 - 06:00		10
07:00							100
08:00								250
09:00 - 11:00					500
12:00 - 13:00				250
14:00 - 17:00					500
18:00								250
19:00							100
20:00	- 23:00		10
*/
func getWorkTickFrequency(hour int) time.Duration {
	switch hour {
	case 7, 19:
		return time.Second / 100
	case 8, 12, 13:
		return time.Second / 250
	case 9, 10, 11, 14, 15, 16, 17:
		return time.Second / 500
	case 18:
		return time.Second / 250
	default:
		return time.Second / 10
	}
}

func getLocalSettings() (loco *time.Location, region string, url string, err error) {
	url, hasFallbackURL := os.LookupEnv("CONNECTION_STRING")

	switch os.Getenv("FLY_REGION") {
	case "fra":
		loco, err = time.LoadLocation("Europe/Berlin")
		region = "aws-eu-central-1"
		localURL, ok := os.LookupEnv("CONNECTION_STRING_EU")

		if ok {
			url = localURL
		} else if !hasFallbackURL {
			err = fmt.Errorf("missing CONNECTION_STRING_EU and not fallback CONNECTION_STRING")
		}

	case "iad":
		loco, err = time.LoadLocation("America/New_York")
		region = "aws-us-east-1"
		localURL, ok := os.LookupEnv("CONNECTION_STRING_US")

		if ok {
			url = localURL
		} else if !hasFallbackURL {
			err = fmt.Errorf("missing CONNECTION_STRING_US and not fallback CONNECTION_STRING")
		}

	case "sin":
		loco, err = time.LoadLocation("Asia/Singapore")
		region = "aws-ap-southeast-1"
		localURL, ok := os.LookupEnv("CONNECTION_STRING_AP")

		if ok {
			url = localURL
		} else if !hasFallbackURL {
			err = fmt.Errorf("missing CONNECTION_STRING_AP and not fallback CONNECTION_STRING")
		}

	default:
		loco = time.UTC
		region = "aws-eu-central-1"
		if !hasFallbackURL {
			err = fmt.Errorf("no region provided and missing fallback CONNECTION_STRING")
		}
	}

	return
}

func performTransfer(db *pgxpool.Pool, from, to, region string, amount int) error {
	timeout, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	const stmt = `UPDATE account
									SET balance = CASE 
										WHEN id = $1 THEN balance - $3
										WHEN id = $2 THEN balance + $3
									END
								WHERE id IN ($1, $2)
								AND crdb_region = $4;`

	tx, err := db.BeginTx(timeout, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback(timeout)
		} else {
			tx.Commit(timeout)
		}
	}()

	_, err = tx.Exec(timeout, stmt, from, to, amount, region)
	if err != nil {
		return fmt.Errorf("executing ts: %w", err)
	}

	return nil
}

func fetchIDs(db *pgxpool.Pool, count int, region string) ([]string, error) {
	const stmt = `SELECT id
								FROM account
								WHERE crdb_region = $1
								ORDER BY random()
								LIMIT $2`

	rows, err := db.Query(context.Background(), stmt, region, count)
	if err != nil {
		return nil, fmt.Errorf("querying for rows: %w", err)
	}

	var accountIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning id: %w", err)
		}
		accountIDs = append(accountIDs, id)
	}

	return accountIDs, nil
}
