package agent

import "strings"

const (
	PromptProfileBalanced = "balanced"
	PromptProfileCompact  = "compact"
	PromptProfileStrict   = "strict"
)

// PromptProfile controls prompt variant behavior for model families.
type PromptProfile struct {
	Name                     string
	CompactToolDescriptions  bool
	CompactResponseGuidance  bool
	StrictVerificationPolicy bool
}

// ResolvePromptProfile maps a configured profile name into runtime prompt behavior.
func ResolvePromptProfile(name string) PromptProfile {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case PromptProfileCompact:
		return PromptProfile{
			Name:                    PromptProfileCompact,
			CompactToolDescriptions: true,
			CompactResponseGuidance: true,
		}
	case PromptProfileStrict:
		return PromptProfile{
			Name:                     PromptProfileStrict,
			StrictVerificationPolicy: true,
		}
	default:
		return PromptProfile{Name: PromptProfileBalanced}
	}
}
