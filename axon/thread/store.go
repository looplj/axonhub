package thread

// Store defines the interface for thread persistence.
type Store interface {
	// Get retrieves a thread by ID. Returns an error if not found.
	Get(id string) (*Thread, error)
	// Save persists a thread, creating or updating as needed.
	Save(thread *Thread) error
	// Delete removes a thread by ID.
	Delete(id string) error
	// List returns all stored threads.
	List() ([]*Thread, error)
}
