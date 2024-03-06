package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_http "crdb-use-cases/_code/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log.SetFlags(0)

	databaseURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		log.Fatalf("missing DATABASE_URL env var")
	}

	secret, ok := os.LookupEnv("ENCRYPTION_KEY")
	if !ok {
		log.Fatalf("missing ENCRYPTION_KEY env var")
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

	processorClient := _http.NewClient("http://localhost:3001")

	router := fiber.New()
	router.Use(cors.New(cors.ConfigDefault))

	router.Post("/users/:id/cards", handleAddCard(db, secret))
	router.Post("/orders/:id/checkout", handlePayment(db, processorClient, secret))

	log.Fatal(router.Listen(":3000"))
}

type card struct {
	CardNumber string      `json:"card_number"`
	Expiry     pgtype.Date `json:"expiry"`
	IsDefault  bool        `json:"is_default"`
}

type date time.Time

func (d *date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}

	*d = date(parsed)
	return nil
}

func handleAddCard(db *pgxpool.Pool, secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Params("id")

		var card card
		if err := c.BodyParser(&card); err != nil {
			log.Printf("error parsing card: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "error parsing card")
		}

		if err := addCard(c.Context(), db, userID, secret, card); err != nil {
			log.Printf("error adding card: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error adding card")
		}

		return nil
	}
}

type paymentResult struct {
	PSP string `json:"psp"`
}

func handlePayment(db *pgxpool.Pool, client *http.Client, secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		orderID := c.Params("id")

		card, err := fetchCard(c.Context(), db, orderID, secret)
		if err != nil {
			log.Printf("error fetching card: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error making payment")
		}

		result, err := makePayment(client, card)
		if err != nil {
			log.Printf("error making payment: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error making payment")
		}

		if err = updateOrder(c.Context(), db, orderID, result.PSP); err != nil {
			log.Printf("error updating order: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error updating order")
		}

		return c.JSON(result)
	}
}

func addCard(ctx context.Context, db *pgxpool.Pool, userID, secret string, card card) error {
	const stmt = `WITH
									crypto AS (
										SELECT gen_random_bytes(32) AS iv
									)
								INSERT INTO card_details ("user_id", "card_number", "card_number_iv", "expiry", "is_default")
								SELECT
									$1,
									encrypt_iv(
										$2::BYTES,
										$3::BYTES,
										iv,
										'aes'
									),
									iv,
									$4,
									$5
								FROM crypto`

	if _, err := db.Exec(ctx, stmt, userID, card.CardNumber, secret, card.Expiry, card.IsDefault); err != nil {
		return fmt.Errorf("inserting card: %w", err)
	}

	return nil
}

func fetchCard(ctx context.Context, db *pgxpool.Pool, orderID, secret string) (card, error) {
	const stmt = `SELECT
									convert_from(
										decrypt_iv(
											card_number,
											$1::BYTES,
											card_number_iv,
											'aes'
										),
										'UTF8'
									) AS card_number,
									cd.expiry
								FROM card_details cd
								JOIN orders o ON cd.user_id = o.user_id
								WHERE o.id = $2
								AND cd.is_default`

	row := db.QueryRow(ctx, stmt, secret, orderID)

	var c card
	if err := row.Scan(&c.CardNumber, &c.Expiry); err != nil {
		return card{}, fmt.Errorf("fetching card: %w", err)
	}

	return c, nil
}

func makePayment(client *http.Client, c card) (paymentResult, error) {
	body, err := json.Marshal(c)
	if err != nil {
		return paymentResult{}, fmt.Errorf("preparing payment request: %w", err)
	}

	response, err := client.Post("/payments", "application/json", bytes.NewReader(body))
	if err != nil {
		return paymentResult{}, fmt.Errorf("making payment request: %w", err)
	}
	defer response.Body.Close()

	var buf bytes.Buffer
	if _, err = buf.ReadFrom(response.Body); err != nil {
		return paymentResult{}, fmt.Errorf("reading payment result: %w", err)
	}

	switch response.StatusCode {
	case http.StatusOK:
		var result paymentResult
		if err = json.Unmarshal(buf.Bytes(), &result); err != nil {
			return result, fmt.Errorf("reading payment result: %w", err)
		}

		return result, nil
	default:
		return paymentResult{}, fmt.Errorf(string(buf.Bytes()))
	}
}

func updateOrder(ctx context.Context, db *pgxpool.Pool, orderID, psp string) error {
	const stmt = `UPDATE orders SET
									status = 'paid',
									payment_reference = $1
								WHERE id = $2`

	if _, err := db.Exec(ctx, stmt, psp, orderID); err != nil {
		return fmt.Errorf("updating order: %w", err)
	}

	return nil
}
