package systemprompt

import (
	"path"
	"strings"
)

func (e *FileEngine) resolveModelProfile(model, override string) (name string, fallback bool, err error) {
	override = strings.TrimSpace(override)
	if override != "" {
		if _, ok := e.profileByName[override]; !ok {
			return "", false, invalid("prompt_profile", override, "profile not found")
		}
		return override, false, nil
	}

	model = strings.TrimSpace(model)
	for _, profile := range e.modelProfiles {
		matched, matchErr := path.Match(profile.Match, model)
		if matchErr != nil {
			return "", false, invalid("model_profiles.match", profile.Match, matchErr.Error())
		}
		if matched {
			return profile.Name, profile.Name == e.defaults.modelProfile, nil
		}
	}

	return e.defaults.modelProfile, true, nil
}
