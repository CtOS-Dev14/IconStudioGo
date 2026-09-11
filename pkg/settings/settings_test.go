package settings

import (
	"os"
	"testing"
)

func TestSaveAndLoadSettings(t *testing.T) {
	tempConfig, err := os.MkdirTemp("", "iconstudio_cfg_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempConfig)

	// Test default settings
	d := DefaultSettings()
	if d.Theme != "system" || d.Width != 1120 || d.Height != 720 {
		t.Fatalf("Unexpected default settings: %+v", d)
	}

	// Test custom values
	custom := AppSettings{
		Theme:       "light",
		Width:       1280,
		Height:      800,
		X:           150,
		Y:           120,
		IsMaximized: false,
		HasPosition: true,
	}

	err = SaveSettings(custom)
	if err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}

	loaded := LoadSettings()
	if loaded.Theme != "light" || loaded.Width != 1280 || loaded.Height != 800 {
		t.Fatalf("Loaded settings mismatch: %+v", loaded)
	}
	t.Logf("Settings successfully persisted and verified: %+v", loaded)
}
