package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/grafana"
)

const (
	ScratchpadTag        = "monitoring-assistant-scratchpad"
	UserIDTagPrefix      = "user-id:"
	SessionIDTagPrefix   = "session-id:"
	LastUsedTagPrefix    = "assistant-last-used:"
	scratchpadPanelID    = 1
	defaultScratchpadURL = "/grafana/d/"
)

// Manager handles per-session scratchpad dashboards.
type Manager struct {
	client      *grafana.Client
	folderTitle string
}

// NewManager creates a scratchpad dashboard manager.
func NewManager(client *grafana.Client, folderTitle string) *Manager {
	if folderTitle == "" {
		folderTitle = "Assistant Scratchpads"
	}
	return &Manager{client: client, folderTitle: folderTitle}
}

// GetOrCreateScratchpad finds or creates a scratchpad dashboard for the session.
func (m *Manager) GetOrCreateScratchpad(ctx context.Context, user *grafana.User, sessionID string) (string, int, string, error) {
	if user == nil {
		return "", 0, "", fmt.Errorf("user is required")
	}
	if sessionID == "" {
		return "", 0, "", fmt.Errorf("session ID is required")
	}

	tags := []string{
		ScratchpadTag,
		UserIDTagPrefix + fmt.Sprintf("%d", user.ID),
		SessionIDTagPrefix + sessionID,
	}

	hits, err := m.client.SearchDashboards(ctx, tags)
	if err != nil {
		return "", 0, "", err
	}
	if len(hits) > 0 && hits[0].UID != "" {
		uid := hits[0].UID
		return uid, scratchpadPanelID, defaultScratchpadURL + uid, nil
	}

	folderUID, err := m.ensureFolder(ctx)
	if err != nil {
		return "", 0, "", err
	}

	model, err := loadTemplate()
	if err != nil {
		return "", 0, "", err
	}

	title := fmt.Sprintf("Scratchpad — %s — %s", displayName(user), shortSessionID(sessionID))
	model["title"] = title
	model["tags"] = buildTags(user.ID, sessionID, time.Now())

	resp, err := m.client.CreateDashboard(ctx, model, folderUID, false)
	if err != nil {
		return "", 0, "", err
	}
	if resp == nil || resp.UID == "" {
		return "", 0, "", fmt.Errorf("grafana returned empty dashboard UID")
	}

	return resp.UID, scratchpadPanelID, defaultScratchpadURL + resp.UID, nil
}

// UpdatePanel updates a single panel on the scratchpad dashboard.
func (m *Manager) UpdatePanel(ctx context.Context, uid string, panelID int, query, title, description, panelType string, datasource map[string]any, timeRange map[string]string) error {
	if uid == "" {
		return fmt.Errorf("dashboard UID is required")
	}
	if panelID == 0 {
		panelID = scratchpadPanelID
	}

	dash, err := m.client.GetDashboard(ctx, uid)
	if err != nil {
		return err
	}
	var model map[string]any
	if err := json.Unmarshal(dash.Dashboard, &model); err != nil {
		return fmt.Errorf("decode dashboard: %w", err)
	}

	updated := false
	panels, ok := model["panels"].([]any)
	if !ok {
		return fmt.Errorf("dashboard panels missing or invalid")
	}
	for _, p := range panels {
		panel, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := panel["id"].(float64); int(id) != panelID {
			continue
		}
		if title != "" {
			panel["title"] = title
		}
		if description != "" {
			panel["description"] = description
		}
		panelType = choosePanelType(panelType, query, title, description)
		if panelType != "" {
			panel["type"] = panelType
		}
		if datasource != nil {
			panel["datasource"] = datasource
		}
		if query != "" {
			applyQuery(panel, query, panelType, datasource)
		}
		updated = true
		break
	}
	if !updated {
		return fmt.Errorf("panel ID %d not found in dashboard", panelID)
	}

	if timeRange != nil {
		if from, ok := timeRange["from"]; ok && from != "" {
			model["time"] = map[string]any{"from": from, "to": timeRange["to"]}
		}
	}

	model["tags"] = mergeTags(model["tags"], buildTags(0, "", time.Now()))

	_, err = m.client.UpdateDashboard(ctx, uid, model, dash.Meta.FolderUID, true)
	if err != nil {
		return err
	}
	return nil
}

// TouchLastUsed updates the last-used tag on the dashboard.
func (m *Manager) TouchLastUsed(ctx context.Context, uid string, ts time.Time) error {
	if uid == "" {
		return fmt.Errorf("dashboard UID is required")
	}
	dash, err := m.client.GetDashboard(ctx, uid)
	if err != nil {
		return err
	}
	var model map[string]any
	if err := json.Unmarshal(dash.Dashboard, &model); err != nil {
		return fmt.Errorf("decode dashboard: %w", err)
	}
	model["tags"] = mergeTags(model["tags"], []string{LastUsedTagPrefix + fmt.Sprintf("%d", ts.Unix())})
	_, err = m.client.UpdateDashboard(ctx, uid, model, dash.Meta.FolderUID, true)
	return err
}

func (m *Manager) ensureFolder(ctx context.Context) (string, error) {
	folder, err := m.client.GetFolderByTitle(ctx, m.folderTitle)
	if err != nil {
		return "", err
	}
	if folder != nil && folder.UID != "" {
		return folder.UID, nil
	}
	created, err := m.client.CreateFolder(ctx, m.folderTitle)
	if err != nil {
		return "", err
	}
	if created == nil || created.UID == "" {
		return "", fmt.Errorf("grafana returned empty folder UID")
	}
	return created.UID, nil
}

func loadTemplate() (map[string]any, error) {
	var model map[string]any
	if err := json.Unmarshal(scratchpadTemplate, &model); err != nil {
		return nil, fmt.Errorf("parse scratchpad template: %w", err)
	}
	return model, nil
}

func buildTags(userID int64, sessionID string, ts time.Time) []string {
	tags := []string{ScratchpadTag}
	if userID != 0 {
		tags = append(tags, UserIDTagPrefix+fmt.Sprintf("%d", userID))
	}
	if sessionID != "" {
		tags = append(tags, SessionIDTagPrefix+sessionID)
	}
	if !ts.IsZero() {
		tags = append(tags, LastUsedTagPrefix+fmt.Sprintf("%d", ts.Unix()))
	}
	return tags
}

func mergeTags(existing any, replacements []string) []string {
	var tags []string
	if raw, ok := existing.([]any); ok {
		for _, item := range raw {
			if s, ok := item.(string); ok {
				tags = append(tags, s)
			}
		}
	} else if raw, ok := existing.([]string); ok {
		tags = append(tags, raw...)
	}

	filtered := tags[:0]
	for _, tag := range tags {
		if strings.HasPrefix(tag, LastUsedTagPrefix) {
			continue
		}
		filtered = append(filtered, tag)
	}
	tags = filtered
	tags = append(tags, replacements...)
	return tags
}

func applyQuery(panel map[string]any, query, panelType string, datasource map[string]any) {
	targets, ok := panel["targets"].([]any)
	if !ok || len(targets) == 0 {
		target := map[string]any{
			"refId": "A",
			"expr":  query,
		}
		if panelType == "table" {
			target["format"] = "table"
		}
		panel["targets"] = []any{target}
		return
	}
	target, ok := targets[0].(map[string]any)
	if !ok {
		return
	}
	target["expr"] = query
	if panelType == "table" {
		target["format"] = "table"
	}
	if datasource != nil {
		target["datasource"] = datasource
	}
}

func displayName(user *grafana.User) string {
	if user == nil {
		return "user"
	}
	if user.Name != "" {
		return user.Name
	}
	if user.Login != "" {
		return user.Login
	}
	return "user"
}

func shortSessionID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func choosePanelType(explicit, query, title, description string) string {
	if explicit != "" {
		switch explicit {
		case "timeseries", "stat", "table":
			return explicit
		default:
			return "timeseries"
		}
	}

	lowerTitle := strings.ToLower(title)
	lowerDesc := strings.ToLower(description)
	lowerQuery := strings.ToLower(query)

	if strings.Contains(lowerTitle, "table") || strings.Contains(lowerDesc, "table") {
		return "table"
	}
	if strings.Contains(lowerQuery, "topk(") || strings.Contains(lowerQuery, "bottomk(") {
		return "table"
	}

	if strings.Contains(lowerTitle, "total") || strings.Contains(lowerTitle, "current") || strings.Contains(lowerDesc, "current") {
		if !strings.Contains(lowerQuery, "[") {
			return "stat"
		}
	}

	return "timeseries"
}
