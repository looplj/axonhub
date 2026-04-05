package memory

import (
	"context"
	"time"
)

// Entry represents a single entry in the memory store
type Entry struct {
	ID        string    `json:"id"`         // unique identifier of the entry
	Path      string    `json:"path"`       // path of the entry
	Content   string    `json:"content"`    // content of the entry
	Source    string    `json:"source"`     // source of the entry
	CreatedAt time.Time `json:"created_at"` // creation time
}

// Store defines the interface for memory storage
type Store interface {
	// Add adds a new entry to the store
	Add(ctx context.Context, path, content, source string) error

	// Get retrieves entry content by path
	Get(ctx context.Context, path string) (string, error)

	// Search searches entries by query
	Search(ctx context.Context, query string, limit int) ([]Entry, error)

	// List lists all entries with a limit
	List(ctx context.Context, limit int) ([]Entry, error)

	// Delete deletes an entry by path
	Delete(ctx context.Context, path string) error
}
