package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	appDirName   = "blinkcli"
	configName   = "config.json"
	ordersName   = "orders.json"
	filePerm0600 = 0o600
)

// Session holds the values required to call Blinkit endpoints.
type Session struct {
	AccessToken     string            `json:"access_token"`
	AuthKey         string            `json:"auth_key"`
	DeviceID        string            `json:"device_id"`
	SessionID       string            `json:"session_uuid"`
	Lat             float64           `json:"lat"`
	Lon             float64           `json:"lon"`
	Locality        string            `json:"locality,omitempty"`
	Landmark        string            `json:"landmark,omitempty"`
	WebAppVersion   string            `json:"web_app_version,omitempty"`
	AppVersion      string            `json:"app_version,omitempty"`
	RNBundleVersion string            `json:"rn_bundle_version,omitempty"`
	UserAgent       string            `json:"user_agent,omitempty"`
	Phone           string            `json:"phone,omitempty"`
	UserID          string            `json:"user_id,omitempty"`
	Cookies         map[string]string `json:"cookies,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at"`
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

// PopulateDerivedCookies seeds missing session cookies from known session fields.
func PopulateDerivedCookies(session *Session) {
	if session == nil {
		return
	}
	if session.Cookies == nil {
		session.Cookies = map[string]string{}
	}
	if session.AccessToken != "" {
		if _, ok := session.Cookies["gr_1_accessToken"]; !ok {
			session.Cookies["gr_1_accessToken"] = url.QueryEscape(session.AccessToken)
		}
	}
	if session.DeviceID != "" {
		if _, ok := session.Cookies["gr_1_deviceId"]; !ok {
			session.Cookies["gr_1_deviceId"] = session.DeviceID
		}
	}
	if session.Lat != 0 {
		if _, ok := session.Cookies["gr_1_lat"]; !ok {
			session.Cookies["gr_1_lat"] = fmt.Sprintf("%f", session.Lat)
		}
	}
	if session.Lon != 0 {
		if _, ok := session.Cookies["gr_1_lon"]; !ok {
			session.Cookies["gr_1_lon"] = fmt.Sprintf("%f", session.Lon)
		}
	}
	if session.Locality != "" {
		if _, ok := session.Cookies["gr_1_locality"]; !ok {
			session.Cookies["gr_1_locality"] = url.QueryEscape(session.Locality)
		}
	}
	if session.Landmark != "" {
		if _, ok := session.Cookies["gr_1_landmark"]; !ok {
			session.Cookies["gr_1_landmark"] = url.QueryEscape(session.Landmark)
		}
	}
}
