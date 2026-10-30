package connect

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func MustConnectDB(urlEnvVar string) *pgxpool.Pool {
	url, ok := os.LookupEnv(urlEnvVar)
	if !ok {
		log.Fatalf("missing %q env var", urlEnvVar)
	}

	db, err := pgxpool.New(context.Background(), url)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}

	timeout, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = db.Ping(timeout); err != nil {
		log.Fatalf("error testing database connection: %v", err)
	}

	return db
}
