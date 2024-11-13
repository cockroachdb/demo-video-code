package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	pool, _ := pgxpool.New(context.Background(), "postgres://root@localhost:26257/defaultdb?sslmode=disable")
	for range time.Tick(time.Second * 3) {
		query(pool)
	}
}

func query(pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	defer cancel()

	fmt.Print("\033[H\033[2J")
	showCreateTable(ctx, pool, "SELECT create_statement FROM [SHOW CREATE TABLE person]")
	showCreateTable(ctx, pool, "SELECT create_statement FROM [SHOW CREATE TABLE pet]")
}

func showCreateTable(ctx context.Context, pool *pgxpool.Pool, stmt string) {
	row := pool.QueryRow(ctx, stmt)

	var createStatement string
	row.Scan(&createStatement)
	fmt.Println(createStatement)
}
