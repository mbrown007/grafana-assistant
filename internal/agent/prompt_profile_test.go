package agent

import "testing"

func TestResolvePromptProfile(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantName    string
		wantCompact bool
	}{
		{name: "balanced default", input: "", wantName: PromptProfileBalanced, wantCompact: false},
		{name: "compact", input: "compact", wantName: PromptProfileCompact, wantCompact: true},
		{name: "strict", input: "strict", wantName: PromptProfileStrict, wantCompact: false},
		{name: "unknown fallback", input: "custom-profile", wantName: PromptProfileBalanced, wantCompact: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolvePromptProfile(tc.input)
			if got.Name != tc.wantName {
				t.Fatalf("ResolvePromptProfile(%q).Name = %q, want %q", tc.input, got.Name, tc.wantName)
			}
			if got.CompactToolDescriptions != tc.wantCompact {
				t.Fatalf("ResolvePromptProfile(%q).CompactToolDescriptions = %t, want %t", tc.input, got.CompactToolDescriptions, tc.wantCompact)
			}
		})
	}
}
