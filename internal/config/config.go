package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	appDirName  = "blinkcli"
	configName  = "config.json"
	ordersName  = "orders.json"
	filePerm0600 = 0o600
)

// Session holds the values required to call Blinkit endpoints.
type Session struct {
	AccessToken string            `json:"access_token"`
	AuthKey     string            `json:"auth_key"`
	DeviceID    string            `json:"device_id"`
	SessionID   string            `json:"session_uuid"`
	Lat         float64           `json:"lat"`
	Lon         float64           `json:"lon"`
	WebAppVersion string          `json:"web_app_version,omitempty"`
	AppVersion    string          `json:"app_version,omitempty"`
	RNBundleVersion string        `json:"rn_bundle_version,omitempty"`
	UserAgent   string            `json:"user_agent,omitempty"`
	Phone       string            `json:"phone,omitempty"`
	UserID      string            `json:"user_id,omitempty"`
	Cookies     map[string]string `json:"cookies,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Config is stored on disk in the user's config directory.
type Config struct {
	Session *Session `json:"session,omitempty"`
}

// ConfigDir returns the OS-specific config directory for blinkcli.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

// ConfigPath returns the full path to config.json.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configName), nil
}

// OrdersPath returns the full path to orders.json.
func OrdersPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ordersName), nil
}

// Load reads config.json if it exists.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes config.json with 0600 permissions.
func Save(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, filePerm0600)
}

// Clear removes config.json if present.
func Clear() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
