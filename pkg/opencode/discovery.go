package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// ServiceRegistration represents the contents of OpenCode's local service registration file (service.json).
type ServiceRegistration struct {
	ID       string `json:"id,omitempty"`
	Version  string `json:"version,omitempty"`
	URL      string `json:"url"`
	PID      int    `json:"pid"`
	Password string `json:"password,omitempty"`
}

// ServiceConfigFile represents the contents of OpenCode's CLI configuration file (~/.config/opencode/service.json).
type ServiceConfigFile struct {
	Disabled       *bool             `json:"disabled,omitempty"`
	Remote         *RemoteConfig     `json:"remote,omitempty"`
	RemoteAccepted *bool             `json:"remoteAccepted,omitempty"`
	Hostname       *string           `json:"hostname,omitempty"`
	Port           *int              `json:"port,omitempty"`
	Password       *string           `json:"password,omitempty"`
	Cors           []string          `json:"cors,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
}

type RemoteConfig struct {
	Route string `json:"route"`
}

var invalidChannelChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// ServiceFilename returns the registration file name for the given release channel.
func ServiceFilename(channel string) string {
	if channel == "" || channel == "latest" || channel == "dev" || channel == "beta" || channel == "next" {
		return "service.json"
	}
	safeChannel := invalidChannelChars.ReplaceAllString(channel, "-")
	return fmt.Sprintf("service-%s.json", safeChannel)
}

// DefaultStateDir returns the state directory where OpenCode registers active services.
// Defaults to $XDG_STATE_HOME/opencode or ~/.local/state/opencode across platforms (including Windows).
func DefaultStateDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "opencode"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "opencode"), nil
}

// DefaultConfigDir returns the config directory where OpenCode stores persistent service config.
// Defaults to $XDG_CONFIG_HOME/opencode or ~/.config/opencode.
func DefaultConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "opencode"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "opencode"), nil
}

// DiscoverLocalService attempts to find and read the registration file of a running OpenCode instance.
func DiscoverLocalService(channel string) (*ServiceRegistration, error) {
	stateDir, err := DefaultStateDir()
	if err != nil {
		return nil, err
	}
	filename := ServiceFilename(channel)
	regPath := filepath.Join(stateDir, filename)

	reg, err := ReadRegistrationFile(regPath)
	if err != nil {
		return nil, fmt.Errorf("could not read registration file at %s: %w", regPath, err)
	}

	// If the registration file does not contain a password, attempt to read it from config
	if reg.Password == "" {
		if configDir, err := DefaultConfigDir(); err == nil {
			configPath := filepath.Join(configDir, filename)
			if cfg, err := ReadConfigFile(configPath); err == nil && cfg.Password != nil {
				reg.Password = *cfg.Password
			}
		}
	}

	return reg, nil
}

// ReadRegistrationFile reads and parses an OpenCode service registration file.
func ReadRegistrationFile(path string) (*ServiceRegistration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var reg ServiceRegistration
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("invalid registration json: %w", err)
	}
	return &reg, nil
}

// ReadConfigFile reads and parses an OpenCode service config file.
func ReadConfigFile(path string) (*ServiceConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ServiceConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config json: %w", err)
	}
	return &cfg, nil
}
