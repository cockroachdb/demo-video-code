package main

import (
	"log"

	"crdb-use-cases/_code/random"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
)

func main() {
	log.SetFlags(0)

	router := fiber.New()
	router.Use(cors.New(cors.ConfigDefault))

	router.Post("/payments", handlePayment())

	log.Fatal(router.Listen(":3001"))
}

func handlePayment() fiber.Handler {
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
