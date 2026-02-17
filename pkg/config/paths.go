package config

import (
	"os"
	"path/filepath"
)

// GetStateDir returns the PicoClaw state directory.
// Checks PICOCLAW_HOME env var first, then defaults to ~/.picoclaw
func GetStateDir() string {
	if home := os.Getenv("PICOCLAW_HOME"); home != "" {
		return expandHome(home)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".picoclaw")
}

// GetConfigPath returns the path to config.json
func GetConfigPath() string {
	return filepath.Join(GetStateDir(), "config.json")
}

// GetAuthPath returns the path to auth.json
func GetAuthPath() string {
	return filepath.Join(GetStateDir(), "auth.json")
}

// GetGlobalSkillsPath returns the path to global skills directory
func GetGlobalSkillsPath() string {
	return filepath.Join(GetStateDir(), "skills")
}

// GetBuiltinSkillsPath returns the path to builtin skills directory
func GetBuiltinSkillsPath(stateDir string) string {
	return filepath.Join(stateDir, "picoclaw", "skills")
}

// GetDefaultWorkspace returns the default workspace path.
// When PICOCLAW_HOME is set, returns $PICOCLAW_HOME/workspace
// Otherwise returns ~/.picoclaw/workspace
func GetDefaultWorkspace() string {
	return filepath.Join(GetStateDir(), "workspace")
}
