package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigServiceAddsScreeningDefaultsForLegacyConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	legacyConfig := map[string]any{
		"theme":           "light",
		"candleColorMode": "red-up",
		"aiConfigs":       []any{},
		"indicators": map[string]any{
			"ma": map[string]any{
				"enabled": true,
				"periods": []int{5, 10, 20},
			},
		},
	}

	writeJSONFile(t, configPath, legacyConfig)

	cs, err := NewConfigService(tempDir)
	if err != nil {
		t.Fatalf("NewConfigService() error = %v", err)
	}

	screening := readScreeningConfig(t, cs)
	assertScreeningConfig(t, screening, map[string]any{
		"markets": map[string]any{
			"shanghai": true,
			"shenzhen": true,
			"beijing":  false,
			"indices":  false,
		},
		"initialSyncDays":    float64(30),
		"retentionMode":      "forever",
		"retentionDays":      float64(60),
		"autoSyncEnabled":    false,
		"autoSyncTime":       "18:00",
		"defaultResultLimit": float64(100),
		"sqlTimeoutSeconds":  float64(0),
	})
}

func TestConfigServicePreservesScreeningConfigAcrossReload(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	customScreening := map[string]any{
		"markets": map[string]any{
			"shanghai": false,
			"shenzhen": true,
			"beijing":  true,
			"indices":  true,
		},
		"initialSyncDays":    45,
		"retentionMode":      "days",
		"retentionDays":      30,
		"autoSyncEnabled":    false,
		"autoSyncTime":       "19:30",
		"defaultResultLimit": 200,
	}

	configPayload := map[string]any{
		"theme":           "light",
		"candleColorMode": "red-up",
		"aiConfigs":       []any{},
		"screening":       customScreening,
		"indicators": map[string]any{
			"ma": map[string]any{
				"enabled": true,
				"periods": []int{5, 10, 20},
			},
		},
	}

	writeJSONFile(t, configPath, configPayload)

	cs, err := NewConfigService(tempDir)
	if err != nil {
		t.Fatalf("NewConfigService() error = %v", err)
	}
	if err := cs.UpdateConfig(cs.GetConfig()); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	reloaded, err := NewConfigService(tempDir)
	if err != nil {
		t.Fatalf("NewConfigService() reload error = %v", err)
	}

	screening := readScreeningConfig(t, reloaded)
	assertScreeningConfig(t, screening, map[string]any{
		"markets": map[string]any{
			"shanghai": false,
			"shenzhen": true,
			"beijing":  true,
			"indices":  true,
		},
		"initialSyncDays":    float64(45),
		"retentionMode":      "days",
		"retentionDays":      float64(30),
		"autoSyncEnabled":    false,
		"autoSyncTime":       "19:30",
		"defaultResultLimit": float64(200),
		"sqlTimeoutSeconds":  float64(0),
	})
}

func TestConfigServiceAddsLayoutTextScaleDefaultForLegacyConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	legacyConfig := map[string]any{
		"theme":           "light",
		"candleColorMode": "red-up",
		"aiConfigs":       []any{},
		"layout": map[string]any{
			"leftPanelWidth":    320,
			"rightPanelWidth":   420,
			"bottomPanelHeight": 200,
		},
		"indicators": map[string]any{
			"ma": map[string]any{
				"enabled": true,
				"periods": []int{5, 10, 20},
			},
		},
	}

	writeJSONFile(t, configPath, legacyConfig)

	cs, err := NewConfigService(tempDir)
	if err != nil {
		t.Fatalf("NewConfigService() error = %v", err)
	}

	raw, err := json.Marshal(cs.GetConfig())
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	layout, ok := payload["layout"].(map[string]any)
	if !ok {
		t.Fatalf("expected layout config in payload, got %#v", payload["layout"])
	}

	if got := layout["textScalePercent"]; got != float64(100) {
		t.Fatalf("expected default textScalePercent 100, got %#v", got)
	}
}

func TestConfigServiceAddsLayoutKlineZoomDefaultForLegacyConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	legacyConfig := map[string]any{
		"theme":           "light",
		"candleColorMode": "red-up",
		"aiConfigs":       []any{},
		"layout": map[string]any{
			"leftPanelWidth":    320,
			"rightPanelWidth":   420,
			"bottomPanelHeight": 200,
			"textScalePercent":  110,
		},
		"indicators": map[string]any{
			"ma": map[string]any{
				"enabled": true,
				"periods": []int{5, 10, 20},
			},
		},
	}

	writeJSONFile(t, configPath, legacyConfig)

	cs, err := NewConfigService(tempDir)
	if err != nil {
		t.Fatalf("NewConfigService() error = %v", err)
	}

	raw, err := json.Marshal(cs.GetConfig())
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	layout, ok := payload["layout"].(map[string]any)
	if !ok {
		t.Fatalf("expected layout config in payload, got %#v", payload["layout"])
	}

	if got := layout["klineZoomPercent"]; got != float64(100) {
		t.Fatalf("expected default klineZoomPercent 100, got %#v", got)
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}

func readScreeningConfig(t *testing.T, cs *ConfigService) map[string]any {
	t.Helper()

	raw, err := json.Marshal(cs.GetConfig())
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	screening, ok := payload["screening"].(map[string]any)
	if !ok {
		t.Fatalf("expected screening config in payload, got %#v", payload["screening"])
	}

	return screening
}

func assertScreeningConfig(t *testing.T, got map[string]any, want map[string]any) {
	t.Helper()

	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(got) error = %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal(want) error = %v", err)
	}

	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("screening config mismatch\nwant: %s\ngot:  %s", wantJSON, gotJSON)
	}
}
