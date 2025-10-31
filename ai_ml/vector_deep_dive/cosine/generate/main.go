package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/brianvoe/gofakeit/v7/data"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	model := flag.String("model", "llama3.2", "name of the model to use for embedding generation")
	add := flag.String("add", "", "add a single additional animal")
	flag.Parse()

	db, err := pgxpool.New(context.Background(), "postgres://root@localhost:26257?sslmode=disable")
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}
	defer db.Close()

	animals := data.Animal["animal"]
	if *add != "" {
		animals = []string{*add}
	}

	if err = generateEmbeddings(db, *model, animals); err != nil {
		log.Fatalf("error generating embeddings: %v", err)
	}
}

func generateEmbeddings(db *pgxpool.Pool, model string, values []string) error {
	const stmt = `INSERT INTO animal (name, vec) VALUES ($1, $2)`

	for i, name := range values {
		embedding, err := getEmbedding(name, model)
		if err != nil {
			return fmt.Errorf("generating embedding for %q: %w", name, err)
		}

		vector := formatVector(embedding)
		if _, err := db.Exec(context.Background(), stmt, name, vector); err != nil {
			return fmt.Errorf("inserting embedding: %w", err)
		}

		fmt.Printf("%d/%d %s\n", i+1, len(values), name)
	}

	return nil
}

func formatVector(vec []float64) string {
	strValues := make([]string, len(vec))
	for i, v := range vec {
		strValues[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(strValues, ",") + "]"
}

type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

func getEmbedding(animal string, model string) ([]float64, error) {
	url := "http://localhost:11434/api/embeddings"

	reqBody := EmbeddingRequest{
		Model:  model,
		Prompt: animal,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return embResp.Embedding, nil
}
