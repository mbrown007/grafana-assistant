package agent

import (
	"strings"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
)

func TestBuildSystemPrompt_IntentSectionMatrix(t *testing.T) {
	reqCtx := &api.DashboardContext{
		UID:       "abc123",
		TimeRange: map[string]string{"from": "now-1h", "to": "now"},
	}

	type intentExpectations struct {
		contains []string
		omits    []string
	}

	cases := map[IntentClass]intentExpectations{
		IntentLiveData: {
			contains: []string{
				"## Artifact System",
				"## Scratchpad Tool",
				"## Explore Tool",
				"## Composite Investigation Tool",
				"## Tool Policy",
				"## Enabled Tool Summary",
				"## Current Dashboard Context",
				"ALWAYS use tools to fetch real data before answering data-related questions.",
			},
		},
		IntentQueryHelp: {
			contains: []string{
				"## Explore Tool",
				"## Tool Policy",
				"## Enabled Tool Summary",
				"## Current Dashboard Context",
				"ALWAYS use tools to fetch real data before answering data-related questions.",
			},
			omits: []string{
				"## Artifact System",
				"## Scratchpad Tool",
				"## Composite Investigation Tool",
			},
		},
		IntentDashboardLookup: {
			contains: []string{
				"## Tool Policy",
				"## Enabled Tool Summary",
				"## Current Dashboard Context",
				"ALWAYS use tools to fetch real data before answering data-related questions.",
			},
			omits: []string{
				"## Artifact System",
				"## Scratchpad Tool",
				"## Explore Tool",
				"## Composite Investigation Tool",
			},
		},
		IntentHowToDocs: {
			contains: []string{
				"## Security Rules",
				"## Guidelines",
			},
			omits: []string{
				"## Artifact System",
				"## Scratchpad Tool",
				"## Explore Tool",
				"## Composite Investigation Tool",
				"## Tool Policy",
				"## Enabled Tool Summary",
				"## Current Dashboard Context",
				"ALWAYS use tools to fetch real data before answering data-related questions.",
			},
		},
		IntentIncidentSummary: {
			contains: []string{
				"## Artifact System",
				"## Scratchpad Tool",
				"## Composite Investigation Tool",
				"## Tool Policy",
				"## Enabled Tool Summary",
				"## Current Dashboard Context",
				"ALWAYS use tools to fetch real data before answering data-related questions.",
			},
			omits: []string{
				"## Explore Tool",
			},
		},
	}

	for intent, expected := range cases {
		t.Run(string(intent), func(t *testing.T) {
			prompt := BuildSystemPrompt(PromptContext{
				Intent:            intent,
				DashboardContext:  reqCtx,
				Tools:             sampleTools(),
				CompositeToolMode: true,
				Profile:           ResolvePromptProfile(PromptProfileBalanced),
			})

			if !strings.Contains(prompt, "## Security Rules") {
				t.Fatalf("intent=%q missing security rules section", intent)
			}
			if !strings.Contains(prompt, "## Guidelines") {
				t.Fatalf("intent=%q missing guidelines section", intent)
			}

			for _, token := range expected.contains {
				if !strings.Contains(prompt, token) {
					t.Fatalf("intent=%q missing expected token %q", intent, token)
				}
			}
			for _, token := range expected.omits {
				if strings.Contains(prompt, token) {
					t.Fatalf("intent=%q should omit token %q", intent, token)
				}
			}
		})
	}
}

func TestBuildSystemPrompt_IntentSizeRegression(t *testing.T) {
	reqCtx := &api.DashboardContext{
		UID:       "abc123",
		TimeRange: map[string]string{"from": "now-1h", "to": "now"},
	}
	profile := ResolvePromptProfile(PromptProfileBalanced)
	tools := sampleTools()

	sizeByIntent := map[IntentClass]int{}
	for _, intent := range []IntentClass{
		IntentLiveData,
		IntentIncidentSummary,
		IntentQueryHelp,
		IntentDashboardLookup,
		IntentHowToDocs,
	} {
		sizeByIntent[intent] = len(BuildSystemPrompt(PromptContext{
			Intent:            intent,
			DashboardContext:  reqCtx,
			Tools:             tools,
			CompositeToolMode: true,
			Profile:           profile,
		}))
	}

	liveSize := sizeByIntent[IntentLiveData]
	for _, intent := range []IntentClass{
		IntentIncidentSummary,
		IntentQueryHelp,
		IntentDashboardLookup,
		IntentHowToDocs,
	} {
		if sizeByIntent[intent] >= liveSize {
			t.Fatalf("expected %q prompt (%d chars) to remain smaller than live_data (%d chars)",
				intent, sizeByIntent[intent], liveSize)
		}
	}

	reduction := 1.0 - float64(sizeByIntent[IntentHowToDocs])/float64(liveSize)
	if reduction < 0.30 {
		t.Fatalf("how_to_docs prompt should be >=30%% smaller than live_data; got %.1f%% reduction (live=%d, docs=%d)",
			reduction*100, liveSize, sizeByIntent[IntentHowToDocs])
	}
}
