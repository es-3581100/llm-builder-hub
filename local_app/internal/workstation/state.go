package workstation

import (
	"errors"
	"fmt"
	"sort"
)

const SchemaVersion = "1.0.0"

type ProjectState struct {
	Name      string `json:"name"`
	Root      string `json:"root"`
	Workspace string `json:"workspace"`
	Branch    string `json:"branch"`
}

type ToolSlot struct {
	Slot     int    `json:"slot"`
	Icon     string `json:"icon"`
	Label    string `json:"label"`
	Tooltip  string `json:"tooltip"`
	Target   string `json:"target"`
	Shortcut string `json:"shortcut"`
	Enabled  bool   `json:"enabled"`
}

type SliderState struct {
	ID    string     `json:"id"`
	Level int        `json:"level"`
	Tools []ToolSlot `json:"tools"`
}

type Document struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Editable bool   `json:"editable"`
	Order    int    `json:"order"`
}

type State struct {
	SchemaVersion string        `json:"schema_version"`
	Authority     string        `json:"authority"`
	Revision      int64         `json:"revision"`
	Project       ProjectState  `json:"project"`
	Sliders       []SliderState `json:"sliders"`
	Documents     []Document    `json:"documents"`
}

func defaultTools(labels [5]string, icons [5]string, targets [5]string, shortcutPrefix string) []ToolSlot {
	tools := make([]ToolSlot, 5)
	for i := range tools {
		tools[i] = ToolSlot{
			Slot:     i + 1,
			Icon:     icons[i],
			Label:    labels[i],
			Tooltip:  labels[i],
			Target:   targets[i],
			Shortcut: fmt.Sprintf("%s%d", shortcutPrefix, i+1),
			Enabled:  true,
		}
	}
	return tools
}

func DefaultState() State {
	return State{
		SchemaVersion: SchemaVersion,
		Authority:     "LOCAL",
		Revision:      1,
		Project: ProjectState{
			Name:      "sample-local-project",
			Root:      "./fixtures/sample-project",
			Workspace: "Local Workspace",
			Branch:    "main",
		},
		Sliders: []SliderState{
			{
				ID: "workspace", Level: 1,
				Tools: defaultTools(
					[5]string{"Prompt Tree", "Files", "Runs", "Reports", "Search"},
					[5]string{"◈", "▤", "▶", "≣", "⌕"},
					[5]string{"prompt-tree", "files", "runs", "reports", "search"},
					"Alt+",
				),
			},
			{
				ID: "project", Level: 2,
				Tools: defaultTools(
					[5]string{"Nodes", "Ledger", "Artifacts", "History", "Publish"},
					[5]string{"◈", "▤", "◇", "↶", "⇧"},
					[5]string{"nodes", "ledger", "artifacts", "history", "publish"},
					"Ctrl+Alt+",
				),
			},
			{
				ID: "branch", Level: 3,
				Tools: defaultTools(
					[5]string{"Current View", "Projection", "Context", "Verification", "Settings"},
					[5]string{"◆", "▣", "¶", "✓", "⚙"},
					[5]string{"current", "projection", "context", "verification", "settings"},
					"Shift+Alt+",
				),
			},
		},
		Documents: []Document{
			{ID: "project-readme", Kind: "markdown", Title: "PROJECT README", Editable: true, Order: 1, Content: "# Sample Local Project\n\nThis fixture demonstrates the local-first workstation shell."},
			{ID: "build-ledger", Kind: "yaml", Title: "BUILD LEDGER", Editable: true, Order: 2, Content: "phase: 1\nstatus: active\nauthority: local"},
			{ID: "task-payload", Kind: "text", Title: "TASK PAYLOAD", Editable: true, Order: 3, Content: "Project decisions live here. Builder Hub transports the resulting payload."},
			{ID: "manifest", Kind: "json", Title: "MANIFEST", Editable: false, Order: 4, Content: "{\n  \"projection\": \"sample\",\n  \"authority\": \"LOCAL\"\n}"},
			{ID: "verification", Kind: "text", Title: "VERIFICATION", Editable: true, Order: 5, Content: "No execution is performed in Phase 1. This document records workstation-shell checks."},
			{ID: "audit", Kind: "markdown", Title: "AUDIT", Editable: true, Order: 6, Content: "## Audit\n\nStatic projection and explicit save semantics are required."},
			{ID: "report", Kind: "text", Title: "REPORT", Editable: true, Order: 7, Content: "Local workstation vertical slice."},
			{ID: "notes", Kind: "text", Title: "NOTES", Editable: true, Order: 8, Content: "Use visible controls. Avoid hidden authority transitions."},
		},
	}
}

func (s State) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema_version %q", s.SchemaVersion)
	}
	if s.Authority != "LOCAL" {
		return fmt.Errorf("authority must be LOCAL, got %q", s.Authority)
	}
	if len(s.Sliders) != 3 {
		return fmt.Errorf("exactly 3 sliders required, got %d", len(s.Sliders))
	}
	expected := []string{"workspace", "project", "branch"}
	for i, slider := range s.Sliders {
		if slider.ID != expected[i] || slider.Level != i+1 {
			return fmt.Errorf("slider %d must be %s level %d", i, expected[i], i+1)
		}
		if len(slider.Tools) != 5 {
			return fmt.Errorf("slider %s must contain exactly 5 tool slots", slider.ID)
		}
		for n, tool := range slider.Tools {
			if tool.Slot != n+1 {
				return fmt.Errorf("slider %s tool index %d has slot %d", slider.ID, n, tool.Slot)
			}
			if tool.Label == "" || tool.Target == "" {
				return fmt.Errorf("slider %s slot %d requires label and target", slider.ID, tool.Slot)
			}
		}
	}
	if len(s.Documents) == 0 {
		return errors.New("at least one document is required")
	}
	ids := map[string]bool{}
	orders := make([]int, 0, len(s.Documents))
	for _, doc := range s.Documents {
		if doc.ID == "" || doc.Kind == "" || doc.Title == "" {
			return errors.New("documents require id, kind, and title")
		}
		if ids[doc.ID] {
			return fmt.Errorf("duplicate document id %q", doc.ID)
		}
		ids[doc.ID] = true
		orders = append(orders, doc.Order)
	}
	sort.Ints(orders)
	for i, order := range orders {
		if order != i+1 {
			return fmt.Errorf("document order must be contiguous starting at 1")
		}
	}
	return nil
}
