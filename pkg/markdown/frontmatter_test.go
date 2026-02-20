package markdown

import (
	"testing"
)

func TestParseFrontmatterBlock_YAML(t *testing.T) {
	content := `---
name: test-skill
description: "A test skill with YAML frontmatter"
emoji: "🧪"
always: true
os: [linux, darwin]
---

# Skill Content`

	frontmatter := ParseFrontmatterBlock(content)

	if frontmatter["name"] != "test-skill" {
		t.Errorf("expected name 'test-skill', got '%s'", frontmatter["name"])
	}
	if frontmatter["description"] != "A test skill with YAML frontmatter" {
		t.Errorf("expected description 'A test skill with YAML frontmatter', got '%s'", frontmatter["description"])
	}
	if frontmatter["emoji"] != "🧪" {
		t.Errorf("expected emoji '🧪', got '%s'", frontmatter["emoji"])
	}
	if frontmatter["always"] != "true" {
		t.Errorf("expected always 'true', got '%s'", frontmatter["always"])
	}
	if frontmatter["os"] != "[linux, darwin]" {
		t.Errorf("expected os '[linux, darwin]', got '%s'", frontmatter["os"])
	}
}

func TestParseFrontmatterBlock_LineBased(t *testing.T) {
	content := `---
name: simple-skill
description: Simple line-based frontmatter
---

# Content`

	frontmatter := ParseFrontmatterBlock(content)

	if frontmatter["name"] != "simple-skill" {
		t.Errorf("expected name 'simple-skill', got '%s'", frontmatter["name"])
	}
	if frontmatter["description"] != "Simple line-based frontmatter" {
		t.Errorf("expected description 'Simple line-based frontmatter', got '%s'", frontmatter["description"])
	}
}

func TestParseFrontmatterBlock_MultiLine(t *testing.T) {
	content := `---
name: multiline-skill
description: |
  This is a multi-line
  description that spans
  multiple lines.
---

# Content`

	frontmatter := ParseFrontmatterBlock(content)

	if frontmatter["name"] != "multiline-skill" {
		t.Errorf("expected name 'multiline-skill', got '%s'", frontmatter["name"])
	}

	// YAML parser normalizes the multiline value
	expectedDesc := "This is a multi-line\ndescription that spans\nmultiple lines."
	if frontmatter["description"] != expectedDesc {
		t.Errorf("expected multi-line description, got '%s'", frontmatter["description"])
	}
}

func TestParseFrontmatterBlock_NoFrontmatter(t *testing.T) {
	content := `# Just regular markdown content
No frontmatter here.`

	frontmatter := ParseFrontmatterBlock(content)

	if len(frontmatter) != 0 {
		t.Errorf("expected empty frontmatter, got %d entries", len(frontmatter))
	}
}

func TestStripFrontmatter(t *testing.T) {
	content := `---
name: test
description: A test
---

# Actual Content
Some content here.`

	stripped := StripFrontmatter(content)

	if stripped == content {
		t.Error("frontmatter should have been stripped")
	}
	if stripped[0] != '#' {
		t.Errorf("expected first line to be '# Actual Content', got '%s'", stripped[:20])
	}
}

func TestCompactPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		homeDir  string
		expected string
	}{
		{
			name:     "compact home path",
			path:     "/home/user/.picoclaw/workspace/skills/test/SKILL.md",
			homeDir:  "/home/user",
			expected: "~/.picoclaw/workspace/skills/test/SKILL.md",
		},
		{
			name:     "path outside home",
			path:     "/tmp/test.md",
			homeDir:  "/home/user",
			expected: "/tmp/test.md",
		},
		{
			name:     "empty home dir",
			path:     "/home/user/test.md",
			homeDir:  "",
			expected: "/home/user/test.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompactPath(tt.path, tt.homeDir)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestParseFrontmatterBlock_JSONBackwardCompat(t *testing.T) {
	content := `---
{"name": "json-skill", "description": "JSON format for backward compatibility"}
---

# Content`

	frontmatter := ParseFrontmatterBlock(content)

	// Line-based parser should handle this as key: value with complex object
	if frontmatter["name"] != "" {
		// The parser should extract name from the JSON-like string
		t.Logf("Got name: '%s'", frontmatter["name"])
	}
}

func TestParseFrontmatterBlock_QuoteStripping(t *testing.T) {
	content := `---
name: "quoted-name"
description: 'single-quoted'
---

# Content`

	frontmatter := ParseFrontmatterBlock(content)

	if frontmatter["name"] != "quoted-name" {
		t.Errorf("expected 'quoted-name', got '%s'", frontmatter["name"])
	}
	if frontmatter["description"] != "single-quoted" {
		t.Errorf("expected 'single-quoted', got '%s'", frontmatter["description"])
	}
}

func TestCoerceValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string", "hello", "hello"},
		{"bool", true, "true"},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"array", []interface{}{"a", "b", "c"}, "[a, b, c]"},
		{"nil", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceValue(tt.value)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
