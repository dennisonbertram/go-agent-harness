package spinner

// fallbackLabel is shown only when the caller has not told the spinner what is
// happening. It is deliberately the single most neutral true statement we can
// make: a run is in progress.
//
// This replaced a pool of fifteen near-synonyms for "thinking" that rotated
// roughly once a second (issue #1415). Rotating decorative words tells the user
// nothing — the label changed but the meaning did not, which reads as a stuck
// animation rather than a live one. Callers should set a real action via
// SetAction; see currentSpinnerAction in the parent tui package.
const fallbackLabel = "Working"
