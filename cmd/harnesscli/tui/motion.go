package tui

import "os"

// ReducedMotion reports whether the user has requested reduced motion.
// Checks the NO_MOTION env var and the PREFERS_REDUCED_MOTION env var.
func ReducedMotion() bool {
	return os.Getenv("NO_MOTION") != "" || os.Getenv("PREFERS_REDUCED_MOTION") == "1"
}

// SpinnerFrames returns the appropriate spinner frames based on motion preference.
// If reduced motion is enabled, returns a single static frame instead of animation.
func SpinnerFrames(reduced bool) []string {
	if reduced {
		return []string{"…"}
	}
	return []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟"}
}
