package sessionpicker

// SessionSelectedMsg is emitted when the user presses Enter on a session entry.
type SessionSelectedMsg struct {
	Entry SessionEntry
}

// SessionDeletedMsg is emitted when the user presses 'd' on a session entry.
// The model should delete the session with the given ID from the store.
type SessionDeletedMsg struct {
	ID string
}
