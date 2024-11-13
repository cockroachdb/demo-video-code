package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/vectorstores"

	"crdb/ai_ml/rag/app/pkg/model"
	"crdb/ai_ml/rag/app/pkg/vec"

	"github.com/fatih/color"
	"github.com/tmc/langchaingo/documentloaders"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/textsplitter"
)

var (
	green = color.New(color.FgGreen).SprintFunc()
)

func main() {
	url := flag.String("url", "", "database connection string")
	question := flag.String("question", "", "question to ask")

	var links model.SliceFlag
	flag.Var(&links, "link", "link to fetch for providing context")

	flag.Parse()

	model, err := ollama.New(ollama.WithModel("llama3.1"))
	if err != nil {
		log.Fatalf("creating model: %v", err)
	}

	store, err := getVectorStore(*url, model)
	if err != nil {
		log.Fatalf("getting vector store: %v", err)
	}

	if len(links) > 0 {
		if err = loadLinks(store, links); err != nil {
			log.Fatalf("error loading web sources: %v", err)
		}
	}

	if *question != "" {
		result, err := ragSearch(store, *question, model, 3)
		if err != nil {
			log.Fatalf("error searching: %v", err)
		}

		fmt.Println(green(result))
	}
}

func getVectorStore(url string, model *ollama.LLM) (vectorstores.VectorStore, error) {
	embedder, err := embeddings.NewEmbedder(model)
	if err != nil {
		return nil, fmt.Errorf("creating embedder: %w", err)
	}

	store, err := vec.New(
		context.Background(),
		vec.WithConnectionURL(url),
		vec.WithEmbedder(embedder),
	)
	if err != nil {
		return nil, fmt.Errorf("creating pgvector: %w", err)
	}

	return store, nil
}

func ragSearch(store vectorstores.VectorStore, question string, model llms.Model, numOfResults int) (string, error) {
	result, err := chains.Run(
		context.Background(),
		chains.NewRetrievalQAFromLLM(
			model,
			vectorstores.ToRetriever(store, numOfResults),
		),
		question,
		chains.WithMaxTokens(4096),
	)
	if err != nil {
		return "", fmt.Errorf("running chain: %w", err)
	}

	return result, nil
}

func loadLinks(store vectorstores.VectorStore, sources []string) error {
	for _, source := range sources {
		docs, err := getLinkDocs(source)
		if err != nil {
			return fmt.Errorf("getting docs: %w", err)
		}

		fmt.Printf("documents to be loaded: %d\n", len(docs))

		_, err = store.AddDocuments(context.Background(), docs)
		if err != nil {
			return fmt.Errorf("adding docs: %w", err)
		}
	}

	return nil
}

func getLinkDocs(source string) ([]schema.Document, error) {
	resp, err := http.Get(source)
	if err != nil {
		return nil, fmt.Errorf("getting source: %w", err)
	}
	defer resp.Body.Close()

	docs, err := documentloaders.
		NewHTML(resp.Body).
		LoadAndSplit(context.Background(), textsplitter.NewRecursiveCharacter())
	if err != nil {
		return nil, fmt.Errorf("creating document loaders: %w", err)
	}

	return docs, nil
}
