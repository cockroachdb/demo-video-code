package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	pool, _ := pgxpool.New(
		context.Background(),
		"postgres://root@localhost:26257/defaultdb?sslmode=disable",
	)
	defer pool.Close()

	for range time.Tick(time.Second * 2) {
		query(pool)
	}
}

func query(pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()

	row := pool.QueryRow(ctx, "SELECT create_statement FROM [SHOW CREATE TABLE person]")

	var createStatement string
	row.Scan(&createStatement)

	fmt.Print("\033[H\033[2J")
	fmt.Println(createStatement)
}
