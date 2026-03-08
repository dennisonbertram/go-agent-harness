package skills

import (
	"reflect"
	"testing"
)

func TestExtractTriggers(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        []string
	}{
		{
			name:        "single trigger",
			description: "Does something. Trigger: phrase one",
			want:        []string{"phrase one"},
		},
		{
			name:        "multiple triggers",
			description: "Does something. Trigger: phrase one, phrase two, phrase three",
			want:        []string{"phrase one", "phrase two", "phrase three"},
		},
		{
			name:        "triggers plural keyword",
			description: "A skill. Triggers: foo, bar",
			want:        []string{"foo", "bar"},
		},
		{
			name:        "no trigger keyword",
			description: "Just a plain description",
			want:        nil,
		},
		{
			name:        "empty after trigger keyword",
			description: "Something. Trigger:",
			want:        nil,
		},
		{
			name:        "case insensitive keyword",
			description: "Something. trigger: my phrase",
			want:        []string{"my phrase"},
		},
		{
			name:        "trigger in middle of description",
			description: "Start text. Trigger: alpha, beta. End text is ignored because comma-split takes all",
			want:        []string{"alpha", "beta. End text is ignored because comma-split takes all"},
		},
		{
			name:        "whitespace trimming",
			description: "Desc. Trigger:  spaced out ,  another  ",
			want:        []string{"spaced out", "another"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTriggers(tt.description)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractTriggers(%q) = %v, want %v", tt.description, got, tt.want)
			}
		})
	}
}

func TestMatchTrigger(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		triggers []string
		want     bool
	}{
		{
			name:     "exact match",
			text:     "deploy my app",
			triggers: []string{"deploy my app"},
			want:     true,
		},
		{
			name:     "substring match",
			text:     "please deploy my app now",
			triggers: []string{"deploy my app"},
			want:     true,
		},
		{
			name:     "case insensitive",
			text:     "Deploy My App",
			triggers: []string{"deploy my app"},
			want:     true,
		},
		{
			name:     "no match",
			text:     "build the project",
			triggers: []string{"deploy my app"},
			want:     false,
		},
		{
			name:     "one of many matches",
			text:     "run tests please",
			triggers: []string{"deploy", "run tests", "build"},
			want:     true,
		},
		{
			name:     "empty triggers",
			text:     "anything",
			triggers: nil,
			want:     false,
		},
		{
			name:     "empty text",
			text:     "",
			triggers: []string{"something"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchTrigger(tt.text, tt.triggers)
			if got != tt.want {
				t.Errorf("MatchTrigger(%q, %v) = %v, want %v", tt.text, tt.triggers, got, tt.want)
			}
		})
	}
}
