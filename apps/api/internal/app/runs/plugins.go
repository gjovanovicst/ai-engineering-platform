package runs

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PluginCheck is one named sandbox command from the project config.
type PluginCheck struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// ParsePluginChecks validates the project JSON (empty means none).
func ParsePluginChecks(raw string) ([]PluginCheck, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var checks []PluginCheck
	if err := json.Unmarshal([]byte(raw), &checks); err != nil {
		return nil, fmt.Errorf("plugin checks: %w", err)
	}
	for _, check := range checks {
		if strings.TrimSpace(check.Name) == "" || strings.TrimSpace(check.Command) == "" {
			return nil, fmt.Errorf("plugin checks: name and command are required")
		}
	}
	return checks, nil
}
