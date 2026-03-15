package spinner

// DefaultVerbs is the pool of whimsical verbs displayed by the spinner.
// These are the same verbs Claude Code uses in its thinking indicator.
var DefaultVerbs = []string{
	"Thinking", "Reasoning", "Pondering", "Analyzing", "Processing",
	"Computing", "Synthesizing", "Evaluating", "Reflecting", "Deliberating",
	"Considering", "Examining", "Contemplating", "Strategizing", "Planning",
}

// fallbackVerb is used when the verb pool is empty.
const fallbackVerb = "Thinking"

// pickVerb selects a random verb from pool using the provided rng.
// Returns fallbackVerb if pool is empty.
func pickVerb(pool []string, rng interface{ Intn(int) int }) string {
	if len(pool) == 0 {
		return fallbackVerb
	}
	return pool[rng.Intn(len(pool))]
}
