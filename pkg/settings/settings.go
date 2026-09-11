package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type AppSettings struct {
	Theme       string `json:"theme"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	IsMaximized bool   `json:"is_maximized"`
	HasPosition bool   `json:"has_position"`
}

var (
	settingsMu sync.Mutex
)

// DefaultSettings returns initial sane defaults
func DefaultSettings() AppSettings {
	return AppSettings{
		Theme:       "system",
		Width:       1120,
		Height:      720,
		X:           -1,
		Y:           -1,
		IsMaximized: false,
		HasPosition: false,
	}
}

func getSettingsPath() (string, error) {
	// 1. Portable PE Mode: Save directly next to the executable
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		return filepath.Join(exeDir, "settings.json"), nil
	}

	// 2. Fallback to current working directory
	return "settings.json", nil
}

// LoadSettings reads the saved settings or returns defaults
func LoadSettings() AppSettings {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	defaults := DefaultSettings()
	filePath, err := getSettingsPath()
	if err != nil {
		return defaults
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		// Migration check: If appdata settings exist, migrate them to local
		if configDir, errCfg := os.UserConfigDir(); errCfg == nil {
			oldPath := filepath.Join(configDir, "IconStudio", "settings.json")
			if oldData, errOld := os.ReadFile(oldPath); errOld == nil {
				data = oldData
				_ = os.WriteFile(filePath, oldData, 0644) // Save locally
			} else {
				return defaults
			}
		} else {
			return defaults
		}
	}

	var s AppSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaults
	}

	// Validate min boundaries
	if s.Width < 840 {
		s.Width = defaults.Width
	}
	if s.Height < 540 {
		s.Height = defaults.Height
	}
	if s.Theme == "" {
		s.Theme = "system"
	}

	return s
}

// SaveSettings writes the settings to disk atomically
func SaveSettings(s AppSettings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	filePath, err := getSettingsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}
