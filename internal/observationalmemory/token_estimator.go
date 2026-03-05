package observationalmemory

import "unicode/utf8"

type TokenEstimator interface {
	EstimateTextTokens(text string) int
	EstimateMessagesTokens(messages []TranscriptMessage) int
}

type RuneTokenEstimator struct{}

func (RuneTokenEstimator) EstimateTextTokens(text string) int {
	runes := utf8.RuneCountInString(text)
	if runes <= 0 {
		return 0
	}
	return (runes + 3) / 4
}

func (e RuneTokenEstimator) EstimateMessagesTokens(messages []TranscriptMessage) int {
	total := 0
	for _, msg := range messages {
		total += e.EstimateTextTokens(msg.Content)
	}
	return total
}
