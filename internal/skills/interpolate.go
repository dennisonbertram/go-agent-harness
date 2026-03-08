package skills

import (
	"strings"
)

// Interpolate replaces variables in a skill body with provided values.
//
// Supported variables:
//   - $ARGUMENTS - full argument string
//   - $1 through $9 - positional arguments (split by whitespace)
//   - $WORKSPACE - workspace path
//   - $SKILL_DIR - directory containing the SKILL.md
//
// Undefined variables are replaced with empty string.
func Interpolate(body string, vars map[string]string) string {
	result := body

	// Replace named variables first (longer names before shorter to avoid partial matches)
	namedVars := []string{"$ARGUMENTS", "$WORKSPACE", "$SKILL_DIR"}
	for _, v := range namedVars {
		val := vars[v]
		result = strings.ReplaceAll(result, v, val)
	}

	// Replace positional variables $9 down to $1 (reverse order to avoid $1 matching in $10 etc.)
	for i := 9; i >= 1; i-- {
		key := "$" + string(rune('0'+i))
		val := vars[key]
		result = strings.ReplaceAll(result, key, val)
	}

	return result
}
