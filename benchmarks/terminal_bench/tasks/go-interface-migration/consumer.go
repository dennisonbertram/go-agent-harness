package main

import "fmt"

// Inventory uses a Storage backend to manage items.
type Inventory struct {
	store Storage
}

// NewInventory creates an Inventory with the given Storage.
func NewInventory(s Storage) *Inventory {
	return &Inventory{store: s}
}

// AddItem stores an item.
func (inv *Inventory) AddItem(name string, data []byte) error {
	return inv.store.Put(name, data)
}

// GetItem retrieves an item.
func (inv *Inventory) GetItem(name string) ([]byte, error) {
	return inv.store.Get(name)
}

// HasItem checks if an item exists.
func (inv *Inventory) HasItem(name string) (bool, error) {
	return inv.store.Exists(name)
}

// ListItems returns items matching a prefix.
func (inv *Inventory) ListItems(prefix string) ([]string, error) {
	return inv.store.List(prefix)
}

// RemoveItem deletes an item.
func (inv *Inventory) RemoveItem(name string) error {
	exists, err := inv.store.Exists(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("item %q not found", name)
	}
	return inv.store.Delete(name)
}
