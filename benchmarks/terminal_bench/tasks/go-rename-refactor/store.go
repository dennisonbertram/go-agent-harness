package main

import "fmt"

// UserRepo manages user data persistence.
type UserRepo struct {
	users map[string]string
}

// NewUserRepo creates a ready-to-use UserRepo.
func NewUserRepo() *UserRepo {
	return &UserRepo{users: make(map[string]string)}
}

// GetUser looks up a user by ID.
func (r *UserRepo) GetUser(id string) (string, bool) {
	name, ok := r.users[id]
	return name, ok
}

// SetUser stores a user by ID.
func (r *UserRepo) SetUser(id, name string) {
	r.users[id] = name
}

// DeleteUser removes a user by ID.
func (r *UserRepo) DeleteUser(id string) error {
	if _, ok := r.users[id]; !ok {
		return fmt.Errorf("user %s not found", id)
	}
	delete(r.users, id)
	return nil
}

// CountUsers returns the number of stored users.
func (r *UserRepo) CountUsers() int {
	return len(r.users)
}
