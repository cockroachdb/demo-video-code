package main

import (
	"context"
	"log"
	"os"
	"time"

	"crdb-use-cases/_code/random"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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

	router := fiber.New()
	router.Use(cors.New(cors.ConfigDefault))

	router.Post("/payments", handlePayment(db))

	log.Fatal(router.Listen(":3001"))
}

func handlePayment(db *pgxpool.Pool) fiber.Handler {
	weightedSet := random.MakeWeightedItems(
		random.WeightedItem{Value: "accepted", Weight: 70},
		random.WeightedItem{Value: "declined", Weight: 20},
		random.WeightedItem{Value: "failed", Weight: 10},
	)

	return func(c *fiber.Ctx) error {
		switch weightedSet.Choose() {
		case "accepted":
			return c.JSON(fiber.Map{
				"psp": uuid.NewString(),
			})
		case "declined":
			return fiber.NewError(fiber.StatusUnauthorized, "declined")
		default:
			return fiber.NewError(fiber.StatusInternalServerError, "failed")
		}
	}
}
