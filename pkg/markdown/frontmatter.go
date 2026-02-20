// PicoClaw - Ultra-lightweight personal AI agent
// Markdown frontmatter parsing with YAML support

package markdown

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParsedFrontmatter represents parsed frontmatter key-value pairs.
// Values are stored as strings after type coercion.
type ParsedFrontmatter map[string]string

// extractMultiLineValue extracts a multi-line value from frontmatter.
// Handles indented continuation lines (YAML-style).
func extractMultiLineValue(lines []string, startIndex int) (string, int) {
	if startIndex >= len(lines) {
		return "", 0
	}

	startLine := lines[startIndex]
	match := regexp.MustCompile(`^([\w-]+):\s*(.*)$`).FindStringSubmatch(startLine)
	if len(match) < 3 {
		return "", 1
	}

	inlineValue := strings.TrimSpace(match[2])
	if inlineValue != "" {
		return inlineValue, 1
	}

	// Extract multi-line value (indented lines)
	var valueLines []string
	i := startIndex + 1
	for i < len(lines) {
		line := lines[i]
		if line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		valueLines = append(valueLines, line)
		i++
	}

	combined := strings.Join(valueLines, "\n")
	return strings.TrimSpace(combined), i - startIndex
}

// parseLineBasedFrontmatter parses frontmatter using line-by-line approach.
// This is a fallback for non-YAML frontmatter.
func parseLineBasedFrontmatter(block string) ParsedFrontmatter {
	frontmatter := make(ParsedFrontmatter)
	lines := strings.Split(block, "\n")

	for i := 0; i < len(lines); {
		line := lines[i]
		match := regexp.MustCompile(`^([\w-]+):\s*(.*)$`).FindStringSubmatch(line)
		if len(match) < 3 {
			i++
			continue
		}

		key := strings.TrimSpace(match[1])
		if key == "" {
			i++
			continue
		}

		inlineValue := strings.TrimSpace(match[2])

		// Check for multi-line value
		if inlineValue == "" && i+1 < len(lines) {
			nextLine := lines[i+1]
			if strings.HasPrefix(nextLine, " ") || strings.HasPrefix(nextLine, "\t") {
				value, linesConsumed := extractMultiLineValue(lines, i)
				if value != "" {
					frontmatter[key] = value
				}
				i += linesConsumed
				continue
			}
		}

		// Use inline value with quotes stripped
		value := stripQuotes(inlineValue)
		if value != "" {
			frontmatter[key] = value
		}
		i++
	}

	return frontmatter
}

// stripQuotes removes surrounding single or double quotes from a string.
func stripQuotes(value string) string {
	if len(value) >= 2 {
		if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
			(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
			return value[1 : len(value)-1]
		}
	}
	return value
}

// coerceValue converts any YAML value to a string representation.
func coerceValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case bool, int, int64, float64:
		return fmt.Sprintf("%v", v)
	case []interface{}:
		var parts []string
		for _, item := range v {
			parts = append(parts, coerceValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		// For complex objects, convert to YAML-like string
		var buf bytes.Buffer
		buf.WriteString("{")
		first := true
		for k, val := range v {
			if !first {
				buf.WriteString(", ")
			}
			buf.WriteString(k)
			buf.WriteString(": ")
			buf.WriteString(coerceValue(val))
			first = false
		}
		buf.WriteString("}")
		return buf.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseYamlFrontmatter attempts to parse frontmatter as YAML.
func parseYamlFrontmatter(block string) ParsedFrontmatter {
	var data map[string]interface{}
	err := yaml.Unmarshal([]byte(block), &data)
	if err != nil {
		return nil
	}

	if data == nil {
		return nil
	}

	result := make(ParsedFrontmatter)
	for key, value := range data {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		coerced := coerceValue(value)
		if coerced != "" {
			result[key] = coerced
		}
	}

	return result
}

// ParseFrontmatterBlock extracts and parses frontmatter from markdown content.
// Supports both YAML and line-based formats.
//
// Format:
// ---
// key: value
// key2: "quoted value"
// key3: |
//   multi-line
//   value
// ---
func ParseFrontmatterBlock(content string) ParsedFrontmatter {
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	if !strings.HasPrefix(content, "---") {
		return make(ParsedFrontmatter)
	}

	endIndex := strings.Index(content[4:], "\n---")
	if endIndex == -1 {
		return make(ParsedFrontmatter)
	}

	block := content[4 : 4+endIndex]

	// Try YAML parsing first
	yamlParsed := parseYamlFrontmatter(block)
	lineParsed := parseLineBasedFrontmatter(block)

	// If YAML parsing failed, use line-based
	if yamlParsed == nil {
		return lineParsed
	}

	// Merge: YAML as base, line-based overrides for complex values
	// (line-based parser handles some edge cases better)
	result := yamlParsed
	for key, value := range lineParsed {
		// Let line-based parser override for arrays/objects
		if strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") {
			result[key] = value
		}
	}

	return result
}

// StripFrontmatter removes the frontmatter block from content.
func StripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}

	endIndex := strings.Index(content, "\n---")
	if endIndex == -1 {
		return content
	}

	// Start after the closing "---"
	start := endIndex + len("\n---")
	result := content[start:]
	// Remove leading whitespace after frontmatter
	return strings.TrimLeft(result, " \t\n\r")
}

// CompactPath converts absolute paths to use ~ for home directory.
// This saves tokens in prompts.
func CompactPath(path string, homeDir string) string {
	if homeDir == "" {
		homeDir = os.Getenv("HOME")
	}
	if homeDir == "" {
		return path
	}

	// Ensure homeDir doesn't have trailing separator
	homeDir = strings.TrimSuffix(homeDir, "/")
	homeDir = strings.TrimSuffix(homeDir, "\\")

	// Add separator for matching
	prefix := homeDir + string(os.PathSeparator)

	if strings.HasPrefix(path, prefix) {
		return "~" + string(os.PathSeparator) + path[len(prefix):]
	}

	return path
}
