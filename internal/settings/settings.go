// Package settings persists user preferences (currently the default proxy) to
// a JSON file in the user config directory so they survive across runs.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings holds persistent user preferences.
type Settings struct {
	// Proxy is the default proxy address applied to every request.
	Proxy string `json:"proxy"`
}

// userConfigDir is a test seam; production code always uses os.UserConfigDir.
var userConfigDir = os.UserConfigDir

// Path returns the settings file location:
//
//	Windows: %AppData%\sarrast\settings.json
//	Unix:    ~/.config/sarrast/settings.json
func Path() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "sarrast", "settings.json"), nil
}

// Load reads the settings file, returning zero-value Settings when the file
// does not exist yet.
func Load() (Settings, error) {
	var s Settings
	path, err := Path()
	if err != nil {
		return s, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, fmt.Errorf("read settings: %w", err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("parse settings: %w", err)
	}
	return s, nil
}

// Save writes the settings file, creating parent directories as needed.
func (s Settings) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}
