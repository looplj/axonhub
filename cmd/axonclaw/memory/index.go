package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var headingPattern = regexp.MustCompile(`^#{1,6}\s+`)

type indexedDocument struct {
	Path        string
	Label       string
	Scope       string
	ContentHash string
	Content     string
	Chunks      []indexedChunk
	UpdatedAt   time.Time
}

type indexedChunk struct {
	ID        string
	Label     string
	Scope     string
	Heading   string
	LineStart int
	LineEnd   int
	Content   string
	Hash      string
	Embedding []float64
}

type storedChunkEmbedding struct {
	Model string
	JSON  string
}

type dbState struct {
	once sync.Once
	db   *sql.DB
	err  error
}

func (s *Store) openDB() (*sql.DB, error) {
	s.db.once.Do(func() {
		if err := os.MkdirAll(s.layout.CacheDir(), 0o755); err != nil {
			s.db.err = fmt.Errorf("create memory cache directory: %w", err)
			return
		}

		db, err := sql.Open("sqlite", s.layout.IndexPath())
		if err != nil {
			s.db.err = fmt.Errorf("open memory index: %w", err)
			return
		}

		if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
			_ = db.Close()
			s.db.err = fmt.Errorf("enable sqlite foreign_keys: %w", err)
			return
		}

		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS documents (
				path TEXT PRIMARY KEY,
				label TEXT NOT NULL,
				scope TEXT NOT NULL,
				content_hash TEXT NOT NULL,
				updated_at TEXT NOT NULL
			);

			CREATE TABLE IF NOT EXISTS chunks (
				id TEXT PRIMARY KEY,
				document_path TEXT NOT NULL,
				label TEXT NOT NULL,
				scope TEXT NOT NULL,
				heading TEXT NOT NULL,
				line_start INTEGER NOT NULL,
				line_end INTEGER NOT NULL,
				content TEXT NOT NULL,
				content_hash TEXT NOT NULL,
				embedding_model TEXT NOT NULL DEFAULT '',
				embedding_json TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL,
				FOREIGN KEY(document_path) REFERENCES documents(path) ON DELETE CASCADE
			);

			CREATE INDEX IF NOT EXISTS idx_memory_chunks_document_path ON chunks(document_path);
			CREATE INDEX IF NOT EXISTS idx_memory_chunks_scope ON chunks(scope);
		`); err != nil {
			_ = db.Close()
			s.db.err = fmt.Errorf("initialize memory index schema: %w", err)
			return
		}

		s.db.db = db
	})

	return s.db.db, s.db.err
}

func (s *Store) ensureIndexed(ctx context.Context) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}

	existing, err := s.documentHashes(ctx, db)
	if err != nil {
		return err
	}

	files, err := s.layout.MemoryFiles()
	if err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(files))
	for _, path := range files {
		seen[path] = struct{}{}

		doc, err := s.loadDocument(path)
		if err != nil {
			return err
		}

		if existing[path] == doc.ContentHash {
			s.ensureEmbeddingsForDocument(ctx, db, path)
			continue
		}

		if err := s.replaceDocument(ctx, db, doc); err != nil {
			return err
		}
	}

	for path := range existing {
		if _, ok := seen[path]; ok {
			continue
		}

		if _, err := db.ExecContext(ctx, `DELETE FROM documents WHERE path = ?`, path); err != nil {
			return fmt.Errorf("delete stale indexed document %q: %w", path, err)
		}
	}

	return nil
}

func (s *Store) documentHashes(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT path, content_hash FROM documents`)
	if err != nil {
		return nil, fmt.Errorf("query indexed documents: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var (
			path string
			hash string
		)
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, fmt.Errorf("scan indexed document: %w", err)
		}
		out[path] = hash
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indexed documents: %w", err)
	}

	return out, nil
}

func (s *Store) replaceDocument(ctx context.Context, db *sql.DB, doc indexedDocument) error {
	reusable, err := s.loadReusableEmbeddings(ctx, db, doc.Path)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin memory index transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	updatedAt := doc.UpdatedAt.UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO documents(path, label, scope, content_hash, updated_at)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			label = excluded.label,
			scope = excluded.scope,
			content_hash = excluded.content_hash,
			updated_at = excluded.updated_at
	`, doc.Path, doc.Label, doc.Scope, doc.ContentHash, updatedAt); err != nil {
		return fmt.Errorf("upsert indexed document %q: %w", doc.Path, err)
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM chunks WHERE document_path = ?`, doc.Path); err != nil {
		return fmt.Errorf("clear indexed chunks for %q: %w", doc.Path, err)
	}

	for _, chunk := range doc.Chunks {
		embeddingModel := ""
		embeddingJSON := ""
		if reusableChunk, ok := reusable[chunk.Hash]; ok {
			if s.embedder == nil || reusableChunk.Model == s.embedder.Model() {
				embeddingModel = reusableChunk.Model
				embeddingJSON = reusableChunk.JSON
			}
		}

		if _, err = tx.ExecContext(ctx, `
			INSERT INTO chunks(
				id, document_path, label, scope, heading, line_start, line_end, content, content_hash, embedding_model, embedding_json, updated_at
			)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, chunk.ID, doc.Path, chunk.Label, chunk.Scope, chunk.Heading, chunk.LineStart, chunk.LineEnd, chunk.Content, chunk.Hash, embeddingModel, embeddingJSON, updatedAt); err != nil {
			return fmt.Errorf("insert indexed chunk %q: %w", chunk.ID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit memory index transaction: %w", err)
	}

	s.ensureEmbeddingsForDocument(ctx, db, doc.Path)

	return nil
}

func (s *Store) loadReusableEmbeddings(ctx context.Context, db *sql.DB, path string) (map[string]storedChunkEmbedding, error) {
	rows, err := db.QueryContext(ctx, `SELECT content_hash, embedding_model, embedding_json FROM chunks WHERE document_path = ?`, path)
	if err != nil {
		return nil, fmt.Errorf("query reusable embeddings for %q: %w", path, err)
	}
	defer rows.Close()

	out := map[string]storedChunkEmbedding{}
	for rows.Next() {
		var (
			hash  string
			model string
			data  string
		)
		if err := rows.Scan(&hash, &model, &data); err != nil {
			return nil, fmt.Errorf("scan reusable embedding for %q: %w", path, err)
		}
		if hash == "" || data == "" {
			continue
		}
		out[hash] = storedChunkEmbedding{Model: model, JSON: data}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reusable embeddings for %q: %w", path, err)
	}

	return out, nil
}

func (s *Store) ensureEmbeddingsForDocument(ctx context.Context, db *sql.DB, path string) {
	if s.embedder == nil {
		return
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, content FROM chunks
		WHERE document_path = ? AND (embedding_model = '' OR embedding_model != ? OR embedding_json = '')
		ORDER BY line_start ASC
	`, path, s.embedder.Model())
	if err != nil {
		return
	}
	defer rows.Close()

	var (
		ids   []string
		texts []string
	)

	for rows.Next() {
		var id string
		var content string
		if err := rows.Scan(&id, &content); err != nil {
			return
		}
		ids = append(ids, id)
		texts = append(texts, content)
	}

	if len(ids) == 0 {
		return
	}

	vectors, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		return
	}

	for i, id := range ids {
		payload, err := json.Marshal(vectors[i])
		if err != nil {
			continue
		}
		_, _ = db.ExecContext(ctx, `
			UPDATE chunks
			SET embedding_model = ?, embedding_json = ?, updated_at = ?
			WHERE id = ?
		`, s.embedder.Model(), string(payload), time.Now().UTC().Format(time.RFC3339Nano), id)
	}
}

func (s *Store) loadIndexedChunks(ctx context.Context) ([]indexedChunk, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, label, scope, heading, line_start, line_end, content, content_hash, embedding_json
		FROM chunks
		ORDER BY scope DESC, label ASC, line_start ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query indexed chunks: %w", err)
	}
	defer rows.Close()

	var chunks []indexedChunk
	for rows.Next() {
		var (
			chunk         indexedChunk
			embeddingJSON string
		)
		if err := rows.Scan(&chunk.ID, &chunk.Label, &chunk.Scope, &chunk.Heading, &chunk.LineStart, &chunk.LineEnd, &chunk.Content, &chunk.Hash, &embeddingJSON); err != nil {
			return nil, fmt.Errorf("scan indexed chunk: %w", err)
		}
		if strings.TrimSpace(embeddingJSON) != "" {
			_ = json.Unmarshal([]byte(embeddingJSON), &chunk.Embedding)
		}
		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indexed chunks: %w", err)
	}

	return chunks, nil
}

func (s *Store) loadDocument(path string) (indexedDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return indexedDocument{}, fmt.Errorf("read memory file %q: %w", path, err)
	}

	content := strings.TrimSpace(string(data))
	hash := sha256.Sum256([]byte(content))
	doc := indexedDocument{
		Path:        path,
		Label:       s.layout.DisplayPath(path),
		Scope:       s.layout.ScopeForPath(path),
		ContentHash: hex.EncodeToString(hash[:]),
		Content:     content,
		UpdatedAt:   time.Now(),
	}

	doc.Chunks = chunkMarkdown(doc.Label, doc.Scope, content)

	return doc, nil
}

func chunkMarkdown(label, scope, content string) []indexedChunk {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	chunks := make([]indexedChunk, 0, len(lines))
	currentHeading := ""
	blockLines := []string{}
	lineStart := 0

	flush := func(lineEnd int) {
		if len(blockLines) == 0 {
			lineStart = 0
			return
		}

		blockContent := strings.TrimSpace(strings.Join(blockLines, "\n"))
		blockLines = nil
		if blockContent == "" {
			lineStart = 0
			return
		}

		hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", label, lineStart, blockContent)))
		chunks = append(chunks, indexedChunk{
			ID:        hex.EncodeToString(hash[:]),
			Label:     label,
			Scope:     scope,
			Heading:   currentHeading,
			LineStart: lineStart,
			LineEnd:   lineEnd,
			Content:   blockContent,
			Hash:      digestString(blockContent),
		})
		lineStart = 0
	}

	for idx, rawLine := range lines {
		lineNo := idx + 1
		trimmed := strings.TrimSpace(rawLine)

		if headingPattern.MatchString(trimmed) {
			flush(lineNo - 1)
			currentHeading = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			continue
		}

		if trimmed == "" {
			flush(lineNo - 1)
			continue
		}

		if lineStart == 0 {
			lineStart = lineNo
		}

		if isStandaloneListItem(trimmed) {
			flush(lineNo - 1)
			lineStart = lineNo
			blockLines = append(blockLines, trimmed)
			flush(lineNo)
			lineStart = 0
			continue
		}

		blockLines = append(blockLines, trimmed)
	}

	flush(len(lines))

	sort.SliceStable(chunks, func(i, j int) bool {
		if chunks[i].Label == chunks[j].Label {
			return chunks[i].LineStart < chunks[j].LineStart
		}
		return chunks[i].Label < chunks[j].Label
	})

	return chunks
}

func isStandaloneListItem(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return true
	}

	for i, r := range line {
		if r < '0' || r > '9' {
			return i > 0 && strings.HasPrefix(line[i:], ". ")
		}
	}

	return false
}

func digestString(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func (s *Store) close() error {
	if s.db.db == nil {
		return nil
	}

	return s.db.db.Close()
}

func mustAbs(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return abs
}
