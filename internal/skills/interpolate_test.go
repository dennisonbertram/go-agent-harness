package skills

import (
	"testing"
)

func TestInterpolate(t *testing.T) {
	tests := []struct {
		name string
		body string
		vars map[string]string
		want string
	}{
		{
			name: "arguments variable",
			body: "Run: $ARGUMENTS",
			vars: map[string]string{"$ARGUMENTS": "hello world"},
			want: "Run: hello world",
		},
		{
			name: "workspace variable",
			body: "Dir: $WORKSPACE",
			vars: map[string]string{"$WORKSPACE": "/home/user/project"},
			want: "Dir: /home/user/project",
		},
		{
			name: "skill dir variable",
			body: "Skill at: $SKILL_DIR",
			vars: map[string]string{"$SKILL_DIR": "/skills/my-skill"},
			want: "Skill at: /skills/my-skill",
		},
		{
			name: "positional arguments",
			body: "First: $1, Second: $2, Third: $3",
			vars: map[string]string{"$1": "alpha", "$2": "beta", "$3": "gamma"},
			want: "First: alpha, Second: beta, Third: gamma",
		},
		{
			name: "undefined variables replaced with empty",
			body: "A=$1 B=$2 C=$3",
			vars: map[string]string{"$1": "only"},
			want: "A=only B= C=",
		},
		{
			name: "all variables combined",
			body: "Args: $ARGUMENTS\nFirst: $1\nWS: $WORKSPACE\nDir: $SKILL_DIR",
			vars: map[string]string{
				"$ARGUMENTS": "foo bar",
				"$1":         "foo",
				"$WORKSPACE": "/ws",
				"$SKILL_DIR": "/sd",
			},
			want: "Args: foo bar\nFirst: foo\nWS: /ws\nDir: /sd",
		},
		{
			name: "empty vars map",
			body: "Nothing: $ARGUMENTS $1 $WORKSPACE $SKILL_DIR",
			vars: map[string]string{},
			want: "Nothing:    ",
		},
		{
			name: "no variables in body",
			body: "Plain text with no vars",
			vars: map[string]string{"$ARGUMENTS": "ignored"},
			want: "Plain text with no vars",
		},
		{
			name: "multiple occurrences",
			body: "$1 and $1 again",
			vars: map[string]string{"$1": "val"},
			want: "val and val again",
		},
		{
			name: "all nine positional args",
			body: "$1 $2 $3 $4 $5 $6 $7 $8 $9",
			vars: map[string]string{
				"$1": "a", "$2": "b", "$3": "c",
				"$4": "d", "$5": "e", "$6": "f",
				"$7": "g", "$8": "h", "$9": "i",
			},
			want: "a b c d e f g h i",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Interpolate(tt.body, tt.vars)
			if got != tt.want {
				t.Errorf("Interpolate() = %q, want %q", got, tt.want)
			}
		})
	}
}
