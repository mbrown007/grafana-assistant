package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	investigationActionPlan         = "plan"
	investigationActionFetchMetrics = "fetch_metrics"
	investigationActionFetchLogs    = "fetch_logs"
	investigationActionSummarize    = "summarize"
	investigationActionNextStep     = "next_step"
)

var investigationManageActions = []string{
	investigationActionPlan,
	investigationActionFetchMetrics,
	investigationActionFetchLogs,
	investigationActionSummarize,
	investigationActionNextStep,
}

var investigationPayloadByAction = map[string]string{
	investigationActionPlan:         "plan",
	investigationActionFetchMetrics: "fetch_metrics",
	investigationActionFetchLogs:    "fetch_logs",
	investigationActionSummarize:    "summarize",
	investigationActionNextStep:     "next_step",
}

func investigationManageToolParameters() json.RawMessage {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "High-level investigation action.",
				"enum":        investigationManageActions,
			},
			"investigationId": map[string]any{
				"type":        "string",
				"description": "Optional stable identifier for a multi-step investigation.",
			},
			"plan": map[string]any{
				"type":                 "object",
				"description":          "Payload for action=plan.",
				"additionalProperties": false,
				"properties": map[string]any{
					"goal": map[string]any{
						"type":        "string",
						"description": "Investigation objective.",
					},
					"scope": map[string]any{
						"type":        "string",
						"description": "Optional system/service scope.",
					},
					"hypotheses": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
					"constraints": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
					"timeRange": map[string]any{
						"type":                 "object",
						"description":          "Optional time range override.",
						"additionalProperties": map[string]any{"type": "string"},
					},
					"maxSteps": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     10,
						"description": "Maximum planning steps.",
					},
				},
				"required": []string{"goal"},
			},
			"fetch_metrics": map[string]any{
				"type":                 "object",
				"description":          "Payload for action=fetch_metrics.",
				"additionalProperties": false,
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "PromQL expression.",
					},
					"datasource": map[string]any{
						"type":        "object",
						"description": "Optional Grafana datasource object (uid/type).",
					},
					"timeRange": map[string]any{
						"type":                 "object",
						"description":          "Optional time range override.",
						"additionalProperties": map[string]any{"type": "string"},
					},
					"instant": map[string]any{
						"type":        "boolean",
						"description": "Run as an instant query.",
					},
					"step": map[string]any{
						"type":        "string",
						"description": "Optional query resolution (for example 1m).",
					},
					"limit": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     5000,
						"description": "Optional max returned points/series.",
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Why this metrics query is needed.",
					},
				},
				"required": []string{"query"},
			},
			"fetch_logs": map[string]any{
				"type":                 "object",
				"description":          "Payload for action=fetch_logs.",
				"additionalProperties": false,
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "LogQL expression.",
					},
					"datasource": map[string]any{
						"type":        "object",
						"description": "Optional Grafana datasource object (uid/type).",
					},
					"timeRange": map[string]any{
						"type":                 "object",
						"description":          "Optional time range override.",
						"additionalProperties": map[string]any{"type": "string"},
					},
					"limit": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     5000,
						"description": "Optional max lines.",
					},
					"direction": map[string]any{
						"type":        "string",
						"enum":        []string{"backward", "forward"},
						"description": "Log query direction.",
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Why this logs query is needed.",
					},
				},
				"required": []string{"query"},
			},
			"summarize": map[string]any{
				"type":                 "object",
				"description":          "Payload for action=summarize.",
				"additionalProperties": false,
				"properties": map[string]any{
					"findings": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
					"objective": map[string]any{
						"type":        "string",
						"description": "Original investigation objective.",
					},
					"severity": map[string]any{
						"type":        "string",
						"enum":        []string{"info", "warning", "critical"},
						"description": "Optional severity level.",
					},
					"includeEvidence": map[string]any{
						"type":        "boolean",
						"description": "Whether to include supporting evidence references.",
					},
				},
				"required": []string{"findings"},
			},
			"next_step": map[string]any{
				"type":                 "object",
				"description":          "Payload for action=next_step.",
				"additionalProperties": false,
				"properties": map[string]any{
					"currentState": map[string]any{
						"type":        "string",
						"description": "Current investigation state.",
					},
					"options": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
					"preferredOption": map[string]any{
						"type":        "string",
						"description": "Preferred next action.",
					},
					"needsApproval": map[string]any{
						"type":        "boolean",
						"description": "Whether human approval is required.",
					},
					"blockers": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"currentState"},
			},
		},
		"required": []string{"action"},
	}

	data, err := json.Marshal(schema)
	if err != nil {
		// Fallback to the minimum viable contract.
		return json.RawMessage(`{"type":"object","properties":{"action":{"type":"string"}},"required":["action"]}`)
	}
	return json.RawMessage(data)
}

func parseInvestigationManageArgs(args map[string]any) (string, string, map[string]any, error) {
	rawAction, _ := args["action"].(string)
	action := strings.ToLower(strings.TrimSpace(rawAction))
	if action == "" {
		return "", "", nil, fmt.Errorf("action is required for investigation__manage")
	}

	payloadKey, ok := investigationPayloadByAction[action]
	if !ok {
		return "", "", nil, fmt.Errorf(
			"unsupported action %q for investigation__manage (supported: %s)",
			action,
			strings.Join(investigationManageActions, ", "),
		)
	}

	rawPayload, ok := args[payloadKey]
	if !ok {
		return "", "", nil, fmt.Errorf("%s payload is required when action=%q", payloadKey, action)
	}

	payload, ok := rawPayload.(map[string]any)
	if !ok {
		return "", "", nil, fmt.Errorf("%s payload must be an object", payloadKey)
	}

	return action, payloadKey, payload, nil
}
