package main

// Storage defines the full persistence interface.
type Storage interface {
	// Get retrieves the value for a key.
	Get(key string) ([]byte, error)

	// Put stores a value under a key.
	Put(key string, data []byte) error

	// Exists checks whether a key exists.
	Exists(key string) (bool, error)

	// List returns all keys matching a prefix, sorted alphabetically.
	List(prefix string) ([]string, error)

	// Delete removes a key. Returns an error if it doesn't exist.
	Delete(key string) error
}
