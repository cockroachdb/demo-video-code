package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log.SetFlags(0)

	databaseURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		log.Fatalf("missing DATABASE_URL env var")
	}

	jwtSecret, ok := os.LookupEnv("JWT_SECRET")
	if !ok {
		log.Fatalf("missing JWT_SECRET env var")
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
	router.Use(cors.New())
	router.Post("/auth/login", handleLogin(db, jwtSecret))

	router.Use(jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: []byte(jwtSecret)},
	}))

	router.Get("/folders/:id", handleFolder(db))
	router.Get("/files/:id", handleFile(db))
	router.Get("/users", handleUserTopLevel(db))
	router.Post("/share/:id", handleShare(db))

	log.Fatal(router.Listen(":3000"))
}

type permission string

const (
	permission_editor  permission = "edit"
	permission_comment permission = "comment"
	permission_read    permission = "read"
	permission_none    permission = ""
)

func handleUserTopLevel(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := getUser(c)

		files, err := getUserTopLevel(c.Context(), db, userID)
		if err != nil {
			log.Printf("error fetching user top level: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error fetching user top level")
		}

		return c.JSON(fiber.Map{
			"files": files,
		})
	}
}

func handleFolder(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := getUser(c)
		fileID := c.Params("id")

		permission, err := getPermission(c.Context(), db, userID, fileID)
		if err != nil {
			log.Printf("error fetching permission: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error fetching permission")
		}
		if permission == permission_none {
			return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
		}

		files, err := getFiles(c.Context(), db, userID, fileID)
		if err != nil {
			log.Printf("error fetching files: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error fetching files")
		}

		return c.JSON(fiber.Map{
			"files": files,
		})
	}
}

func handleFile(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := getUser(c)
		fileID := c.Params("id")

		permission, err := getPermission(c.Context(), db, userID, fileID)
		if err != nil {
			log.Printf("error fetching permission: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error fetching permission")
		}
		if permission == permission_none {
			return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
		}

		content, err := getFile(c.Context(), db, userID, fileID)
		if err != nil {
			log.Printf("error fetching files: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error fetching files")
		}

		return c.SendString(content)
	}
}

type shareRequest struct {
	UserID     string `json:"user_id"`
	Permission string `json:"permission"`
}

func handleShare(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := getUser(c)
		fileID := c.Params("id")

		var r shareRequest
		if err := c.BodyParser(&r); err != nil {
			log.Printf("invalid request: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid request")
		}

		permission, err := getPermission(c.Context(), db, userID, fileID)
		if err != nil {
			log.Printf("error fetching permission: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error fetching permission")
		}
		if permission != permission_editor {
			return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
		}

		if err = share(c.Context(), db, r.UserID, fileID, string(permission)); err != nil {
			log.Printf("error sharing file: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error sharing file")
		}

		return nil
	}
}

type loginRequest struct {
	Username string `json:"username"`
}

func handleLogin(db *pgxpool.Pool, jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var r loginRequest
		if err := c.BodyParser(&r); err != nil {
			log.Printf("error parsing request: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "error parsing request")
		}

		claims, err := login(c.Context(), db, r.Username)
		if err != nil {
			log.Printf("error logging in: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error logging in")
		}

		claims.RegisteredClaims = jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 1)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "Acme Inc.",
			ID:        uuid.NewString(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			log.Printf("error logging in: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error logging in")
		}

		return c.JSON(fiber.Map{
			"jwt": tokenString,
		})
	}
}

type claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func login(ctx context.Context, db *pgxpool.Pool, username string) (claims, error) {
	const stmt = `SELECT id
								FROM users
								WHERE username = $1`

	row := db.QueryRow(ctx, stmt, username)

	var userID string

	if err := row.Scan(&userID); err != nil {
		return claims{}, fmt.Errorf("scanning results: %w", err)
	}

	claims := claims{
		UserID: userID,
	}

	return claims, nil
}

type file struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Permission string `json:"permission"`
	IsFolder   bool   `json:"is_folder"`
}

func getUserTopLevel(ctx context.Context, db *pgxpool.Pool, userID string) ([]file, error) {
	const stmt = `WITH RECURSIVE accessible_files AS (
									SELECT
										f.id,
										f.name,
										f.parent_id,
										f.is_folder,
										p.type AS permission_type,
										f.id AS root_id
									FROM file f
									JOIN permissions p ON f.id = p.file_id
									JOIN users u ON p.user_id = u.id
									WHERE u.id = $1
								
									UNION ALL
								
									SELECT
										f.id,
										f.name,
										f.parent_id,
										f.is_folder,
										af.permission_type,
										af.root_id
									FROM file f
									JOIN accessible_files af ON f.parent_id = af.id
								)
								SELECT DISTINCT
									af.id,
									af.name,
									af.permission_type,
									af.is_folder
								FROM accessible_files af
								WHERE af.root_id = af.id`

	rows, err := db.Query(ctx, stmt, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching files: %w", err)
	}

	var files []file
	for rows.Next() {
		var f file
		if err = rows.Scan(&f.ID, &f.Name, &f.Permission, &f.IsFolder); err != nil {
			return nil, fmt.Errorf("scanning file: %w", err)
		}
		files = append(files, f)
	}

	return files, nil
}

func getFiles(ctx context.Context, db *pgxpool.Pool, userID, fileID string) ([]file, error) {
	const stmt = `WITH RECURSIVE
									user_access AS (
										SELECT f.id, f.parent_id, f.name, f.is_folder, p.type AS permission
										FROM file f
										JOIN permissions p ON f.id = p.file_id
										WHERE p.user_id = $1
										
										UNION ALL
										
										SELECT f.id, f.parent_id, f.name, f.is_folder, ua.permission
										FROM file f
										JOIN user_access ua ON f.parent_id = ua.id
										WHERE ua.is_folder = true
									),
									directory_files AS (
										SELECT f.id, f.name, f.is_folder
										FROM file f
										JOIN user_access ua ON f.id = ua.id
										WHERE f.parent_id = $2
										OR f.id = $2
									)
								SELECT df.*
								FROM directory_files df
								WHERE df.id != $2;`

	rows, err := db.Query(ctx, stmt, userID, fileID)
	if err != nil {
		return nil, fmt.Errorf("fetching files: %w", err)
	}

	var files []file
	for rows.Next() {
		var f file
		if err = rows.Scan(&f.ID, &f.Name, &f.IsFolder); err != nil {
			return nil, fmt.Errorf("scanning file: %w", err)
		}
		files = append(files, f)
	}

	return files, nil
}

func getFile(ctx context.Context, db *pgxpool.Pool, userID, fileID string) (string, error) {
	const stmt = `WITH RECURSIVE
									parent_permissions AS (
										SELECT 
											f.id, 
											f.parent_id, 
											TRUE AS has_permission
										FROM file f
										JOIN permissions p ON f.id = p.file_id
										WHERE p.user_id = $1
									
										UNION
									
										SELECT 
											f.id, 
											f.parent_id, 
											pp.has_permission
										FROM file f
										JOIN parent_permissions pp ON f.parent_id = pp.id
										WHERE pp.has_permission
									),
									file_content AS (
										SELECT 
											f.id, 
											f.content
										FROM file f
										JOIN parent_permissions pp ON f.id = pp.id
										WHERE f.is_folder = false
											AND f.id = $2
									)
								SELECT fc.content
								FROM file_content fc`

	row := db.QueryRow(ctx, stmt, userID, fileID)

	var content string
	if err := row.Scan(&content); err != nil {
		return "", fmt.Errorf("scanning content: %w", err)
	}

	return content, nil
}

func getPermission(ctx context.Context, db *pgxpool.Pool, userID, fileID string) (permission, error) {
	const stmt = `WITH RECURSIVE upward_permissions AS (
									SELECT
										file.id,
										file.parent_id,
										permissions.type AS permission_type,
										1 AS depth
									FROM file
									LEFT JOIN permissions ON file.id = permissions.file_id
										AND permissions.user_id = $1
									WHERE file.id = $2
									
									UNION ALL
									
									SELECT
										file.id,
										file.parent_id,
										COALESCE(upward_permissions.permission_type, permissions.type) AS permission_type,
										upward_permissions.depth + 1
									FROM file
									JOIN upward_permissions ON file.id = upward_permissions.parent_id
									LEFT JOIN permissions ON file.id = permissions.file_id
										AND permissions.user_id = $1
									WHERE upward_permissions.permission_type IS NULL
								)
								SELECT permission_type FROM upward_permissions
								WHERE permission_type IS NOT NULL
								ORDER BY depth ASC
								LIMIT 1`

	row := db.QueryRow(ctx, stmt, userID, fileID)
	var perm string
	if err := row.Scan(&perm); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return permission_none, nil
		}

		return permission_none, fmt.Errorf("fetching permission: %w", err)
	}

	return permission(perm), nil
}

func share(ctx context.Context, db *pgxpool.Pool, userID, fileID, permission string) error {
	const stmt = `INSERT INTO permissions (user_id, file_id, type) VALUES
								($1, $2, $3)`

	if _, err := db.Exec(ctx, stmt, userID, fileID, permission); err != nil {
		return fmt.Errorf("adding permission: %w", err)
	}

	return nil
}

func getUser(c *fiber.Ctx) string {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	return claims["user_id"].(string)
}
