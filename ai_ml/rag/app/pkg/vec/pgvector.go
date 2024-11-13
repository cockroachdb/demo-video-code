package vec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/vectorstores"
)

var (
	ErrEmbedderWrongNumberVectors = errors.New("number of vectors from embedder does not match number of documents")
	ErrInvalidScoreThreshold      = errors.New("score threshold must be between 0 and 1")
	ErrInvalidFilters             = errors.New("invalid filters")
	ErrUnsupportedOptions         = errors.New("unsupported options")
)

// Store is a wrapper around the pgvector client.
type Store struct {
	embedder              embeddings.Embedder
	conn                  *pgx.Conn
	postgresConnectionURL string
	embeddingTableName    string
	collectionTableName   string
	collectionName        string
	collectionUUID        string
	collectionMetadata    map[string]any
	preDeleteCollection   bool
	vectorDimensions      int
	hnswIndex             *HNSWIndex
}

type HNSWIndex struct {
	m                int
	efConstruction   int
	distanceFunction string
}

func WithConnectionURL(connectionURL string) Option {
	return func(p *Store) {
		p.postgresConnectionURL = connectionURL
	}
}

func WithEmbedder(e embeddings.Embedder) Option {
	return func(p *Store) {
		p.embedder = e
	}
}

var _ vectorstores.VectorStore = Store{}

// New creates a new Store with options.
func New(ctx context.Context, opts ...Option) (Store, error) {
	store, err := applyClientOptions(opts...)
	if err != nil {
		return Store{}, err
	}
	if store.conn == nil {
		store.conn, err = pgx.Connect(ctx, store.postgresConnectionURL)
		if err != nil {
			return Store{}, err
		}
	}

	if err = store.conn.Ping(ctx); err != nil {
		return Store{}, err
	}
	if err = store.init(ctx); err != nil {
		return Store{}, err
	}
	return store, nil
}

// Option is a function type that can be used to modify the client.
type Option func(p *Store)

const (
	DefaultCollectionName           = "langchain"
	DefaultPreDeleteCollection      = false
	DefaultEmbeddingStoreTableName  = "langchain_pg_embedding"
	DefaultCollectionStoreTableName = "langchain_pg_collection"
)

func applyClientOptions(opts ...Option) (Store, error) {
	o := &Store{
		collectionName:      DefaultCollectionName,
		preDeleteCollection: DefaultPreDeleteCollection,
		embeddingTableName:  DefaultEmbeddingStoreTableName,
		collectionTableName: DefaultCollectionStoreTableName,
	}

	for _, opt := range opts {
		opt(o)
	}

	if o.postgresConnectionURL == "" {
		o.postgresConnectionURL = os.Getenv("DATABASE_URL")
	}

	if o.postgresConnectionURL == "" && o.conn == nil {
		return Store{}, fmt.Errorf("missing connection string")
	}

	if o.embedder == nil {
		return Store{}, fmt.Errorf("missing embedder")
	}

	return *o, nil
}

func (s *Store) init(ctx context.Context) error {
	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return err
	}

	if err := s.createCollectionTableIfNotExists(ctx, tx); err != nil {
		return err
	}
	if err := s.createEmbeddingTableIfNotExists(ctx, tx); err != nil {
		return err
	}
	if s.preDeleteCollection {
		if err := s.RemoveCollection(ctx, tx); err != nil {
			return err
		}
	}
	if err := s.createOrGetCollection(ctx, tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s Store) createCollectionTableIfNotExists(ctx context.Context, tx pgx.Tx) error {
	sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
												name varchar,
												cmetadata json,
												"uuid" uuid NOT NULL,
												UNIQUE (name),
												PRIMARY KEY (uuid)
											)`,
		s.collectionTableName)
	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	return nil
}

func (s Store) createEmbeddingTableIfNotExists(ctx context.Context, tx pgx.Tx) error {
	vectorDimensions := ""
	if s.vectorDimensions > 0 {
		vectorDimensions = fmt.Sprintf("(%d)", s.vectorDimensions)
	}

	sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
												collection_id uuid,
												embedding vector%s,
												document varchar,
												cmetadata json,
												"uuid" uuid NOT NULL,
												CONSTRAINT langchain_pg_embedding_collection_id_fkey
												FOREIGN KEY (collection_id) REFERENCES %s (uuid) ON DELETE CASCADE,
												PRIMARY KEY (uuid)
											)`,
		s.embeddingTableName, vectorDimensions, s.collectionTableName)
	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}

	sql = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_collection_id ON %s (collection_id)`, s.embeddingTableName, s.embeddingTableName)
	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}

	// See this for more details on HNWS indexes: https://github.com/pgvector/pgvector#hnsw
	if s.hnswIndex != nil {
		sql = fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS %s_embedding_hnsw ON %s USING hnsw (embedding %s)`,
			s.embeddingTableName, s.embeddingTableName, s.hnswIndex.distanceFunction,
		)
		if s.hnswIndex.m > 0 && s.hnswIndex.efConstruction > 0 {
			sql = fmt.Sprintf("%s WITH (m=%d, ef_construction = %d)", sql, s.hnswIndex.m, s.hnswIndex.efConstruction)
		}
		if _, err := tx.Exec(ctx, sql); err != nil {
			return err
		}
	}

	return nil
}

// AddDocuments adds documents to the Postgres collection associated with 'Store'.
// and returns the ids of the added documents.
func (s Store) AddDocuments(
	ctx context.Context,
	docs []schema.Document,
	options ...vectorstores.Option,
) ([]string, error) {
	opts := s.getOptions(options...)
	if opts.ScoreThreshold != 0 || opts.Filters != nil || opts.NameSpace != "" {
		return nil, ErrUnsupportedOptions
	}

	docs = s.deduplicate(ctx, opts, docs)

	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.PageContent)
	}

	embedder := s.embedder
	if opts.Embedder != nil {
		embedder = opts.Embedder
	}
	vectors, err := embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		return nil, err
	}

	if len(vectors) != len(docs) {
		return nil, ErrEmbedderWrongNumberVectors
	}

	b := &pgx.Batch{}
	sql := fmt.Sprintf(`INSERT INTO %s (uuid, document, embedding, cmetadata, collection_id)
		VALUES($1, $2, $3, $4, $5)`, s.embeddingTableName)

	ids := make([]string, len(docs))
	for docIdx, doc := range docs {
		id := uuid.New().String()
		ids[docIdx] = id
		b.Queue(sql, id, doc.PageContent, pgvector.NewVector(vectors[docIdx]), doc.Metadata, s.collectionUUID)
	}
	return ids, s.conn.SendBatch(ctx, b).Close()
}

//nolint:cyclop
func (s Store) SimilaritySearch(ctx context.Context, query string, numDocuments int, options ...vectorstores.Option) ([]schema.Document, error) {
	opts := s.getOptions(options...)
	collectionName := s.getNameSpace(opts)
	scoreThreshold, err := s.getScoreThreshold(opts)
	if err != nil {
		return nil, err
	}

	filter, err := s.getFilters(opts)
	if err != nil {
		return nil, err
	}

	embedder := s.embedder
	if opts.Embedder != nil {
		embedder = opts.Embedder
	}

	embedderData, err := embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	whereQuerys := make([]string, 0)
	if scoreThreshold != 0 {
		whereQuerys = append(whereQuerys, fmt.Sprintf("data.distance < %f", 1-scoreThreshold))
	}

	for k, v := range filter {
		whereQuerys = append(whereQuerys, fmt.Sprintf("(data.cmetadata ->> '%s') = '%s'", k, v))
	}

	whereQuery := strings.Join(whereQuerys, " AND ")
	if len(whereQuery) == 0 {
		whereQuery = "TRUE"
	}

	dims := len(embedderData)
	sql := fmt.Sprintf(`WITH filtered_embedding_dims AS MATERIALIZED (
												SELECT *
												FROM %s
												WHERE vector_dims (embedding) = $1
										  )
											SELECT
												data.document,
												data.cmetadata,
												data.distance
											FROM (
												SELECT
													filtered_embedding_dims.*,
													embedding <=> $2 AS distance
												FROM
													filtered_embedding_dims
													JOIN %s ON filtered_embedding_dims.collection_id=%s.uuid WHERE %s.name='%s') AS data
											WHERE %s
											ORDER BY data.distance
											LIMIT $3`,
		s.embeddingTableName,
		s.collectionTableName, s.collectionTableName, s.collectionTableName, collectionName,
		whereQuery)

	rows, err := s.conn.Query(ctx, sql, dims, pgvector.NewVector(embedderData), numDocuments)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := make([]schema.Document, 0)
	for rows.Next() {
		doc := schema.Document{}
		if err := rows.Scan(&doc.PageContent, &doc.Metadata, &doc.Score); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

//nolint:cyclop
func (s Store) Search(ctx context.Context, numDocuments int, options ...vectorstores.Option) ([]schema.Document, error) {
	opts := s.getOptions(options...)
	collectionName := s.getNameSpace(opts)
	filter, err := s.getFilters(opts)
	if err != nil {
		return nil, err
	}
	whereQuerys := make([]string, 0)
	for k, v := range filter {
		whereQuerys = append(whereQuerys, fmt.Sprintf("(%s.cmetadata ->> '%s') = '%s'", s.embeddingTableName, k, v))
	}
	whereQuery := strings.Join(whereQuerys, " AND ")
	if len(whereQuery) == 0 {
		whereQuery = "TRUE"
	}
	sql := fmt.Sprintf(`SELECT
												%[1]s.document,
												%[1]s.cmetadata
											FROM %[1]s
											JOIN %s ON %[1]s.collection_id=%s.uuid
											WHERE %s.name='%s' AND %s
											LIMIT $1`,
		s.embeddingTableName, s.collectionTableName, s.collectionTableName, s.collectionTableName, collectionName,
		whereQuery)
	rows, err := s.conn.Query(ctx, sql, numDocuments)
	if err != nil {
		return nil, err
	}
	docs := make([]schema.Document, 0)
	defer rows.Close()

	for rows.Next() {
		doc := schema.Document{}
		if err := rows.Scan(&doc.PageContent, &doc.Metadata); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// Close closes the connection.
func (s Store) Close(ctx context.Context) error {
	return s.conn.Close(ctx)
}

func (s Store) DropTables(ctx context.Context) error {
	if _, err := s.conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, s.embeddingTableName)); err != nil {
		return err
	}
	if _, err := s.conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, s.collectionTableName)); err != nil {
		return err
	}
	return nil
}

func (s Store) RemoveCollection(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE name = $1`, s.collectionTableName), s.collectionName)
	return err
}

func (s *Store) createOrGetCollection(ctx context.Context, tx pgx.Tx) error {
	sql := fmt.Sprintf(`INSERT INTO %s (uuid, name, cmetadata)
											VALUES($1, $2, $3) ON CONFLICT (name) DO
											UPDATE SET cmetadata = $3`,
		s.collectionTableName)
	if _, err := tx.Exec(ctx, sql, uuid.New().String(), s.collectionName, s.collectionMetadata); err != nil {
		return err
	}
	sql = fmt.Sprintf(`SELECT uuid FROM %s WHERE name = $1 ORDER BY name limit 1`, s.collectionTableName)
	return tx.QueryRow(ctx, sql, s.collectionName).Scan(&s.collectionUUID)
}

// getOptions applies given options to default Options and returns it
// This uses options pattern so clients can easily pass options without changing function signature.
func (s Store) getOptions(options ...vectorstores.Option) vectorstores.Options {
	opts := vectorstores.Options{}
	for _, opt := range options {
		opt(&opts)
	}
	return opts
}

func (s Store) getNameSpace(opts vectorstores.Options) string {
	if opts.NameSpace != "" {
		return opts.NameSpace
	}
	return s.collectionName
}

func (s Store) getScoreThreshold(opts vectorstores.Options) (float32, error) {
	if opts.ScoreThreshold < 0 || opts.ScoreThreshold > 1 {
		return 0, ErrInvalidScoreThreshold
	}
	return opts.ScoreThreshold, nil
}

// getFilters return metadata filters, now only support map[key]value pattern
func (s Store) getFilters(opts vectorstores.Options) (map[string]any, error) {
	if opts.Filters != nil {
		if filters, ok := opts.Filters.(map[string]any); ok {
			return filters, nil
		}
		return nil, ErrInvalidFilters
	}
	return map[string]any{}, nil
}

func (s Store) deduplicate(
	ctx context.Context,
	opts vectorstores.Options,
	docs []schema.Document,
) []schema.Document {
	if opts.Deduplicater == nil {
		return docs
	}

	filtered := make([]schema.Document, 0, len(docs))
	for _, doc := range docs {
		if !opts.Deduplicater(ctx, doc) {
			filtered = append(filtered, doc)
		}
	}

	return filtered
}