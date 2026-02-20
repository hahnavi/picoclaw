package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/markdown"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

const (
	MaxNameLength        = 64
	MaxDescriptionLength = 1024
)

// SkillMetadata holds parsed frontmatter from a skill's SKILL.md file.
// Enhanced with OpenClaw-compatible fields for rich skill descriptions.
type SkillMetadata struct {
	// Core fields (required)
	Name        string `json:"name"`
	Description string `json:"description"`

	// Display fields
	Emoji    string `json:"emoji,omitempty"`    // Icon for display
	Homepage string `json:"homepage,omitempty"` // Documentation URL

	// Invocation control
	Always                 bool   `json:"always,omitempty"`                  // Always load this skill
	SkillKey               string `json:"skillKey,omitempty"`                // Custom invocation key
	UserInvocable          bool   `json:"userInvocable,omitempty"`           // User can invoke (default: true)
	DisableModelInvocation bool   `json:"disableModelInvocation,omitempty"`  // Model cannot auto-invoke (default: false)

	// Environment requirements
	PrimaryEnv string   `json:"primaryEnv,omitempty"` // Primary environment (node, python, go, etc.)
	OS         []string `json:"os,omitempty"`         // Platform restrictions (linux, darwin, windows)

	// Dependencies
	Requires *SkillRequires `json:"requires,omitempty"` // System requirements

	// Installation specs (for future use)
	Install []SkillInstallSpec `json:"install,omitempty"` // Installation instructions

	// Agent type support
	AgentTypes []string `json:"agentTypes,omitempty"` // Agent types that can use this skill (e.g., "chat", "specialist")
	Priority   int      `json:"priority,omitempty"`   // Loading priority (higher = earlier, default: 0)

	// Internal tracking
	LoadedAt time.Time `json:"-"` // When metadata was loaded
}

// SkillRequires defines system requirements for a skill.
type SkillRequires struct {
	Bins   []string `json:"bins,omitempty"`   // Required binaries (all must be present)
	AnyBin []string `json:"anyBin,omitempty"` // Optional binaries (at least one must be present)
	Env    []string `json:"env,omitempty"`    // Required environment variables
	Config []string `json:"config,omitempty"` // Required config keys
}

// SkillInstallSpec describes how to install a skill's dependencies.
type SkillInstallSpec struct {
	Kind string `json:"kind"` // brew, node, go, uv, download
	ID   string `json:"id,omitempty"`
}

// SkillInfo represents a discovered skill with its metadata.
type SkillInfo struct {
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	Source      string          `json:"source"`
	Description string          `json:"description"`
	Metadata    *SkillMetadata  `json:"metadata,omitempty"`
	CompactPath string          `json:"compactPath,omitempty"` // Path with ~ for home dir
}

func (info SkillInfo) validate() error {
	var errs error
	if info.Name == "" {
		errs = errors.Join(errs, errors.New("name is required"))
	} else {
		if len(info.Name) > MaxNameLength {
			errs = errors.Join(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
		}
		if !namePattern.MatchString(info.Name) {
			errs = errors.Join(errs, errors.New("name must be alphanumeric with hyphens"))
		}
	}

	if info.Description == "" {
		errs = errors.Join(errs, errors.New("description is required"))
	} else if len(info.Description) > MaxDescriptionLength {
		errs = errors.Join(errs, fmt.Errorf("description exceeds %d character", MaxDescriptionLength))
	}
	return errs
}

type SkillsLoader struct {
	workspace       string
	workspaceSkills string // workspace skills (项目级别)
	globalSkills    string // 全局 skills (~/.picoclaw/skills)
	builtinSkills   string // 内置 skills
}

func NewSkillsLoader(workspace string, globalSkills string, builtinSkills string) *SkillsLoader {
	return &SkillsLoader{
		workspace:       workspace,
		workspaceSkills: filepath.Join(workspace, "skills"),
		globalSkills:    globalSkills, // ~/.picoclaw/skills
		builtinSkills:   builtinSkills,
	}
}

func (sl *SkillsLoader) ListSkills() []SkillInfo {
	skills := make([]SkillInfo, 0)

	// Get home directory for path compaction
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE") // Windows fallback
	}

	if sl.workspaceSkills != "" {
		if dirs, err := os.ReadDir(sl.workspaceSkills); err == nil {
			for _, dir := range dirs {
				if dir.IsDir() {
					skillFile := filepath.Join(sl.workspaceSkills, dir.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						info := SkillInfo{
							Name:   dir.Name(),
							Path:   skillFile,
							Source: "workspace",
						}
						metadata := sl.getSkillMetadata(skillFile)
						if metadata != nil {
							info.Description = metadata.Description
							info.Name = metadata.Name
							info.Metadata = metadata
						}
						info.CompactPath = markdown.CompactPath(skillFile, homeDir)
						if err := info.validate(); err != nil {
							slog.Warn("invalid skill from workspace", "name", info.Name, "error", err)
							continue
						}
						skills = append(skills, info)
					}
				}
			}
		}
	}

	// 全局 skills (~/.picoclaw/skills) - 被 workspace skills 覆盖
	if sl.globalSkills != "" {
		if dirs, err := os.ReadDir(sl.globalSkills); err == nil {
			for _, dir := range dirs {
				if dir.IsDir() {
					skillFile := filepath.Join(sl.globalSkills, dir.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						// 检查是否已被 workspace skills 覆盖
						exists := false
						for _, s := range skills {
							if s.Name == dir.Name() && s.Source == "workspace" {
								exists = true
								break
							}
						}
						if exists {
							continue
						}

						info := SkillInfo{
							Name:   dir.Name(),
							Path:   skillFile,
							Source: "global",
						}
						metadata := sl.getSkillMetadata(skillFile)
						if metadata != nil {
							info.Description = metadata.Description
							info.Name = metadata.Name
							info.Metadata = metadata
						}
						info.CompactPath = markdown.CompactPath(skillFile, homeDir)
						if err := info.validate(); err != nil {
							slog.Warn("invalid skill from global", "name", info.Name, "error", err)
							continue
						}
						skills = append(skills, info)
					}
				}
			}
		}
	}

	if sl.builtinSkills != "" {
		if dirs, err := os.ReadDir(sl.builtinSkills); err == nil {
			for _, dir := range dirs {
				if dir.IsDir() {
					skillFile := filepath.Join(sl.builtinSkills, dir.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						// 检查是否已被 workspace 或 global skills 覆盖
						exists := false
						for _, s := range skills {
							if s.Name == dir.Name() && (s.Source == "workspace" || s.Source == "global") {
								exists = true
								break
							}
						}
						if exists {
							continue
						}

						info := SkillInfo{
							Name:   dir.Name(),
							Path:   skillFile,
							Source: "builtin",
						}
						metadata := sl.getSkillMetadata(skillFile)
						if metadata != nil {
							info.Description = metadata.Description
							info.Name = metadata.Name
							info.Metadata = metadata
						}
						info.CompactPath = markdown.CompactPath(skillFile, homeDir)
						if err := info.validate(); err != nil {
							slog.Warn("invalid skill from builtin", "name", info.Name, "error", err)
							continue
						}
						skills = append(skills, info)
					}
				}
			}
		}
	}

	return skills
}

func (sl *SkillsLoader) LoadSkill(name string) (string, bool) {
	// 1. 优先从 workspace skills 加载（项目级别）
	if sl.workspaceSkills != "" {
		skillFile := filepath.Join(sl.workspaceSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	// 2. 其次从全局 skills 加载 (~/.picoclaw/skills)
	if sl.globalSkills != "" {
		skillFile := filepath.Join(sl.globalSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	// 3. 最后从内置 skills 加载
	if sl.builtinSkills != "" {
		skillFile := filepath.Join(sl.builtinSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	return "", false
}

func (sl *SkillsLoader) LoadSkillsForContext(skillNames []string) string {
	if len(skillNames) == 0 {
		return ""
	}

	var parts []string
	for _, name := range skillNames {
		content, ok := sl.LoadSkill(name)
		if ok {
			parts = append(parts, fmt.Sprintf("### Skill: %s\n\n%s", name, content))
		}
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// BuildSkillsSummary generates an XML summary of available skills.
// Uses compact paths (~ substitution) to save tokens.
func (sl *SkillsLoader) BuildSkillsSummary() string {
	allSkills := sl.ListSkills()
	if len(allSkills) == 0 {
		return ""
	}

	// Get home directory for path compaction
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE") // Windows fallback
	}

	var lines []string
	lines = append(lines, "<skills>")
	for _, s := range allSkills {
		// Use compact path if available, otherwise use original path
		displayPath := s.CompactPath
		if displayPath == "" {
			displayPath = markdown.CompactPath(s.Path, homeDir)
		}

		escapedName := escapeXML(s.Name)
		escapedDesc := escapeXML(s.Description)
		escapedPath := escapeXML(displayPath)

		lines = append(lines, fmt.Sprintf("  <skill>"))
		lines = append(lines, fmt.Sprintf("    <name>%s</name>", escapedName))
		lines = append(lines, fmt.Sprintf("    <description>%s</description>", escapedDesc))
		lines = append(lines, fmt.Sprintf("    <location>%s</location>", escapedPath))
		lines = append(lines, fmt.Sprintf("    <source>%s</source>", s.Source))

		// Add extended metadata if present
		if s.Metadata != nil {
			if len(s.Metadata.AgentTypes) > 0 {
				agentTypes := escapeXML(strings.Join(s.Metadata.AgentTypes, ", "))
				lines = append(lines, fmt.Sprintf("    <agentTypes>%s</agentTypes>", agentTypes))
			}
			if s.Metadata.Priority != 0 {
				lines = append(lines, fmt.Sprintf("    <priority>%d</priority>", s.Metadata.Priority))
			}
		}
		lines = append(lines, "  </skill>")
	}
	lines = append(lines, "</skills>")

	return strings.Join(lines, "\n")
}

// getSkillMetadata extracts and parses metadata from a skill's SKILL.md file.
// Uses the enhanced frontmatter parser that supports YAML and line-based formats.
func (sl *SkillsLoader) getSkillMetadata(skillPath string) *SkillMetadata {
	content, err := os.ReadFile(skillPath)
	if err != nil {
		logger.WarnCF("skills", "Failed to read skill metadata",
			map[string]interface{}{
				"skill_path": skillPath,
				"error":      err.Error(),
			})
		return nil
	}

	frontmatter := markdown.ParseFrontmatterBlock(string(content))
	if len(frontmatter) == 0 {
		return &SkillMetadata{
			Name:     filepath.Base(filepath.Dir(skillPath)),
			LoadedAt: time.Now(),
		}
	}

	// Try JSON first (for backward compatibility)
	var jsonMeta struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(content), &jsonMeta); err == nil {
		return &SkillMetadata{
			Name:        jsonMeta.Name,
			Description: jsonMeta.Description,
			LoadedAt:    time.Now(),
		}
	}

	// Use enhanced frontmatter parser
	meta := &SkillMetadata{
		Name:     frontmatter["name"],
		Description: frontmatter["description"],
		LoadedAt: time.Now(),
	}

	// Parse enhanced fields
	if v, ok := frontmatter["emoji"]; ok && v != "" {
		meta.Emoji = v
	}
	if v, ok := frontmatter["homepage"]; ok && v != "" {
		meta.Homepage = v
	}
	if v, ok := frontmatter["always"]; ok && v != "" {
		meta.Always = strings.ToLower(v) == "true" || v == "1"
	}
	if v, ok := frontmatter["skillKey"]; ok && v != "" {
		meta.SkillKey = v
	}
	if v, ok := frontmatter["primaryEnv"]; ok && v != "" {
		meta.PrimaryEnv = v
	}
	if v, ok := frontmatter["userInvocable"]; ok && v != "" {
		meta.UserInvocable = strings.ToLower(v) != "false" && v != "0"
	} else {
		meta.UserInvocable = true // Default: user can invoke
	}
	if v, ok := frontmatter["disableModelInvocation"]; ok && v != "" {
		meta.DisableModelInvocation = strings.ToLower(v) == "true" || v == "1"
	}

	// Parse OS list
	if v, ok := frontmatter["os"]; ok && v != "" {
		// Handle both array-like "[linux, darwin]" and comma-separated
		v = strings.TrimPrefix(v, "[")
		v = strings.TrimSuffix(v, "]")
		for _, os := range strings.Split(v, ",") {
			os = strings.TrimSpace(os)
			if os != "" {
				meta.OS = append(meta.OS, os)
			}
		}
	}

	// Parse multi-agent support fields
	if v, ok := frontmatter["agentTypes"]; ok && v != "" {
		v = strings.TrimPrefix(v, "[")
		v = strings.TrimSuffix(v, "]")
		for _, agentType := range strings.Split(v, ",") {
			agentType = strings.TrimSpace(agentType)
			if agentType != "" {
				meta.AgentTypes = append(meta.AgentTypes, agentType)
			}
		}
	}
	if v, ok := frontmatter["priority"]; ok && v != "" {
		// Parse priority as integer
		if priority, err := parsePriority(v); err == nil {
			meta.Priority = priority
		}
	}

	// Fallback name from directory if not specified
	if meta.Name == "" {
		meta.Name = filepath.Base(filepath.Dir(skillPath))
	}

	return meta
}

// stripFrontmatter removes the frontmatter block from skill content.
func (sl *SkillsLoader) stripFrontmatter(content string) string {
	return markdown.StripFrontmatter(content)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// parsePriority parses a priority value from string to int.
// Supports both numeric strings and integer values.
func parsePriority(v string) (int, error) {
	var priority int
	_, err := fmt.Sscanf(v, "%d", &priority)
	return priority, err
}
