package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

func main() {
	databaseURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		log.Fatalf("missing DATABASE_URL env var")
	}

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}

	if err = db.Ping(context.Background()); err != nil {
		log.Fatalf("error testing database connection: %v", err)
	}

	router := fiber.New()
	router.Post("/auth/register", handleRegister(db))
	router.Post("/auth/login", handleLogin(db))
	router.Patch("/accounts/:id/avatar", handleAvatarUpdate(db))

	log.Fatal(router.Listen(":3000"))
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleRegister(db *pgxpool.Pool) fiber.Handler {
	argon := newArgon2idHash()

	const stmt = `INSERT INTO account (email, password_hash, password_salt) VALUES ($1, $2, $3)
								RETURNING id`

	return func(c *fiber.Ctx) error {
		var request registerRequest
		if err := c.BodyParser(&request); err != nil {
			log.Printf("error parsing register request: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid request")
		}

		// Hash password, returning hash and salt.
		hs, err := argon.generateHash([]byte(request.Password), nil)
		if err != nil {
			log.Printf("error hashing password: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "registration error")
		}

		// Create account and return id.
		row := db.QueryRow(c.Context(), stmt, request.Email, hs.hash, hs.salt)

		var id string
		if err := row.Scan(&id); err != nil {
			log.Printf("error creating account: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "registration error")
		}

		return c.JSON(fiber.Map{
			"id": id,
		})
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(db *pgxpool.Pool) fiber.Handler {
	argon := newArgon2idHash()

	const stmt = `SELECT password_hash, password_salt FROM account WHERE email = $1`

	return func(c *fiber.Ctx) error {
		var request loginRequest
		if err := c.BodyParser(&request); err != nil {
			log.Printf("error parsing register request: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid request")
		}

		// Fetch account.
		row := db.QueryRow(c.Context(), stmt, request.Email)

		hs := hashSalt{}
		if err := row.Scan(&hs.hash, &hs.salt); err != nil {
			log.Printf("error fetching account: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "login error")
		}

		// Validate password.
		if err := argon.compare(hs.hash, hs.salt, []byte(request.Password)); err != nil {
			log.Printf("invalid password: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "incorrect username or password")
		}
		return nil
	}
}

type profileUpdateRequest struct {
	AvatarBase64 string `json:"avatar_base64"`
}

func handleAvatarUpdate(db *pgxpool.Pool) fiber.Handler {
	const stmt = `UPSERT INTO avatar (account_id, image) VALUES ($1, $2)`

	return func(c *fiber.Ctx) error {
		accountID := c.Params("id")

		var request profileUpdateRequest
		if err := c.BodyParser(&request); err != nil {
			log.Printf("error parsing profile update request: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid request")
		}

		// Update profile.
		if _, err := db.Exec(c.Context(), stmt, accountID, request.AvatarBase64); err != nil {
			log.Printf("error updating avatar: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "update error")
		}

		return nil
	}
}

type argon2idHash struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

func newArgon2idHash() *argon2idHash {
	return &argon2idHash{
		memory:      64 * 1024,
		iterations:  3,
		parallelism: 2,
		saltLength:  32,
		keyLength:   32,
	}
}

type hashSalt struct {
	hash []byte
	salt []byte
}

func (a *argon2idHash) generateHash(password, salt []byte) (hashSalt, error) {
	var err error
	if len(salt) == 0 {
		if salt, err = randomSecret(a.saltLength); err != nil {
			return hashSalt{}, err
		}
	}

	hash := argon2.IDKey(password, salt, a.iterations, a.memory, a.parallelism, a.keyLength)

	return hashSalt{hash: hash, salt: salt}, nil
}

func (a *argon2idHash) compare(hash, salt, password []byte) error {
	hashSalt, err := a.generateHash(password, salt)
	if err != nil {
		return err
	}

	if !bytes.Equal(hash, hashSalt.hash) {
		return fmt.Errorf("hash doesn't match")
	}
	return nil
}

func randomSecret(length uint32) ([]byte, error) {
	secret := make([]byte, length)

	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}

	return secret, nil
}
