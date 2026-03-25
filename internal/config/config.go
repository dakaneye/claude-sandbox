// Package config handles .claude-sandbox.yaml configuration parsing and gate detection.
package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents .claude-sandbox.yaml configuration.
type Config struct {
	Gates   GatesConfig `mapstructure:"gates"`
	Retries int         `mapstructure:"retries"`
	Timeout string      `mapstructure:"timeout"`
}

// GatesConfig holds custom gate commands.
type GatesConfig struct {
	Build    string `mapstructure:"build"`
	Lint     string `mapstructure:"lint"`
	Test     string `mapstructure:"test"`
	Security string `mapstructure:"security"`
}

// Load loads configuration from .claude-sandbox.yaml in the given directory.
func Load(dir string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("retries", 3)
	v.SetDefault("timeout", "2h")

	// Look for config file
	v.SetConfigName(".claude-sandbox")
	v.SetConfigType("yaml")
	v.AddConfigPath(dir)

	// Read config (handle both missing file and parse errors)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Real parse error
			return nil, err
		}
		// File not found is fine, use defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// DetectGateCommand auto-detects the command for a gate based on project files.
func DetectGateCommand(dir string, gate string) string {
	switch gate {
	case "build":
		return detectBuildCommand(dir)
	case "lint":
		return detectLintCommand(dir)
	case "test":
		return detectTestCommand(dir)
	case "security":
		return detectSecurityCommand(dir)
	default:
		return ""
	}
}

// GetGateCommand returns the command for a gate, using config override or auto-detection.
func (c *Config) GetGateCommand(dir string, gate string) string {
	// Check config override first
	if override := c.gateOverride(gate); override != "" {
		return override
	}

	// Fall back to auto-detection
	return DetectGateCommand(dir, gate)
}

// gateOverride returns the config override for a gate, or empty string if none.
func (c *Config) gateOverride(gate string) string {
	switch gate {
	case "build":
		return c.Gates.Build
	case "lint":
		return c.Gates.Lint
	case "test":
		return c.Gates.Test
	case "security":
		return c.Gates.Security
	default:
		return ""
	}
}

func detectBuildCommand(dir string) string {
	// Priority: Makefile > go.mod > package.json
	if fileExists(dir, "Makefile") && makeTargetExists(dir, "build") {
		return "make build"
	}
	if fileExists(dir, "go.mod") {
		return "go build ./..."
	}
	if fileExists(dir, "package.json") && npmScriptExists(dir, "build") {
		return "npm run build"
	}
	return ""
}

func detectLintCommand(dir string) string {
	if fileExists(dir, "Makefile") && makeTargetExists(dir, "lint") {
		return "make lint"
	}
	if fileExists(dir, "go.mod") {
		return "golangci-lint run ./..."
	}
	if fileExists(dir, "package.json") && npmScriptExists(dir, "lint") {
		return "npm run lint"
	}
	return ""
}

func detectTestCommand(dir string) string {
	if fileExists(dir, "Makefile") && makeTargetExists(dir, "test") {
		return "make test"
	}
	if fileExists(dir, "go.mod") {
		return "go test ./..."
	}
	if fileExists(dir, "package.json") && npmScriptExists(dir, "test") {
		return "npm test"
	}
	return ""
}

func detectSecurityCommand(dir string) string {
	if fileExists(dir, "Makefile") && makeTargetExists(dir, "security") {
		return "make security"
	}
	if fileExists(dir, "go.mod") {
		return "govulncheck ./..."
	}
	if fileExists(dir, "package.json") {
		return "npm audit"
	}
	return ""
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func makeTargetExists(dir, target string) bool {
	content, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		return false
	}

	// Look for "target:" at the start of a line
	targetPrefix := []byte(target + ":")
	for _, line := range bytes.Split(content, []byte("\n")) {
		if bytes.HasPrefix(line, targetPrefix) {
			return true
		}
	}
	return false
}

func npmScriptExists(dir, script string) bool {
	content, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return false
	}

	_, exists := pkg.Scripts[script]
	return exists
}
