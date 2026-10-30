package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"image/png"
	"log"
	mrand "math/rand"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/o1egl/govatar"
	"github.com/samber/lo"
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
	router.Use(cors.New(cors.ConfigDefault))

	router.Post("/auth/register", handleRegister(db))
	router.Post("/auth/login", handleLogin(db))
	router.Get("/accounts/:id/profile", handleFetchProfile(db))
	router.Patch("/accounts/:id/account", handleAccountUpdate(db))

	log.Fatal(router.Listen(":3000"))
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleRegister(db *pgxpool.Pool) fiber.Handler {
	argon := newArgon2idHash()

	const accountStmt = `INSERT INTO user_account (email, password_hash, password_salt) VALUES ($1, $2, $3)
											 RETURNING id`

	const profileStmt = `INSERT INTO user_profile (account_id, image) VALUES ($1, $2)`

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
		row := db.QueryRow(c.Context(), accountStmt, request.Email, hs.hash, hs.salt)

		var id string
		if err := row.Scan(&id); err != nil {
			log.Printf("error creating account: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "registration error")
		}

		// Create avatar.
		avatar, err := generateAvatarForEmail(request.Email)
		if err != nil {
			log.Printf("error generating avatar: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "avatar error")
		}

		if _, err = db.Exec(c.Context(), profileStmt, id, avatar); err != nil {
			log.Printf("error storing avatar: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "avatar error")
		}

		return c.JSON(fiber.Map{
			"id":     id,
			"avatar": avatar,
		})
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(db *pgxpool.Pool) fiber.Handler {
	argon := newArgon2idHash()

	const stmt = `SELECT id, password_hash, password_salt FROM user_account WHERE email = $1`

	return func(c *fiber.Ctx) error {
		var request loginRequest
		if err := c.BodyParser(&request); err != nil {
			log.Printf("error parsing register request: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid request")
		}

		// Fetch account.
		row := db.QueryRow(c.Context(), stmt, request.Email)

		var id string
		hs := hashSalt{}
		if err := row.Scan(&id, &hs.hash, &hs.salt); err != nil {
			log.Printf("error fetching account: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "login error")
		}

		// Validate password.
		if err := argon.compare(hs.hash, hs.salt, []byte(request.Password)); err != nil {
			log.Printf("invalid password: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "incorrect username or password")
		}

		return c.JSON(fiber.Map{
			"id": id,
		})
	}
}

type accountUpdateRequest struct {
	Email string `json:"email"`
}

func handleAccountUpdate(db *pgxpool.Pool) fiber.Handler {
	const stmt = `UPDATE user_account SET email = $1 WHERE id = $2`

	return func(c *fiber.Ctx) error {
		accountID := c.Params("id")

		var request accountUpdateRequest
		if err := c.BodyParser(&request); err != nil {
			log.Printf("error parsing account update request: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid request")
		}

		// Update account.
		if _, err := db.Exec(c.Context(), stmt, request.Email, accountID); err != nil {
			log.Printf("error updating account: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "update error")
		}

		return nil
	}
}

type fetchProfileResponse struct {
	Email        string    `json:"email"`
	RegisteredOn time.Time `json:"registered_on"`
	Image        []byte    `json:"image"`
}

func handleFetchProfile(db *pgxpool.Pool) fiber.Handler {
	const stmt = `SELECT
									ua.email,
									up.image,
									ua.registered_on
								FROM user_account ua
								JOIN user_profile up ON ua.id = up.account_id
								WHERE account_id = $1`

	return func(c *fiber.Ctx) error {
		accountID := c.Params("id")

		row := db.QueryRow(c.Context(), stmt, accountID)

		var r fetchProfileResponse
		if err := row.Scan(&r.Email, &r.Image, &r.RegisteredOn); err != nil {
			log.Printf("error fetching profile: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "error fetching profile")
		}

		return c.JSON(r)
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

func generateAvatarForEmail(email string) ([]byte, error) {
	gender := lo.Ternary(mrand.Int()%2 == 0, govatar.FEMALE, govatar.MALE)

	image, err := govatar.GenerateForUsername(gender, email)
	if err != nil {
		return nil, fmt.Errorf("generating avatar: %w", err)
	}

	var buf bytes.Buffer
	if err = png.Encode(&buf, image); err != nil {
		return nil, fmt.Errorf("generating avatar bytes: %w", err)
	}

	return buf.Bytes(), nil
}
