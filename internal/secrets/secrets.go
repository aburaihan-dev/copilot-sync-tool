package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Finding describes a potential secret found in a file.
type Finding struct {
	File    string
	Key     string
	Pattern string
	Snippet string // short preview, value truncated
}

// secretPatterns are compiled regexes matched against JSON string values.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^ghp_[A-Za-z0-9]{36,}$`),                     // GitHub PAT classic
	regexp.MustCompile(`^github_pat_[A-Za-z0-9_]{82,}$`),             // GitHub PAT fine-grained
	regexp.MustCompile(`^ghs_[A-Za-z0-9]{36,}$`),                     // GitHub Actions token
	regexp.MustCompile(`^sk-[A-Za-z0-9]{32,}$`),                      // OpenAI API key
	regexp.MustCompile(`^sk-proj-[A-Za-z0-9_\-]{40,}$`),              // OpenAI project key
	regexp.MustCompile(`^xoxb-[0-9]+-[0-9A-Za-z\-]+$`),               // Slack bot token
	regexp.MustCompile(`^xoxp-[0-9]+-[0-9A-Za-z\-]+$`),               // Slack user token
	regexp.MustCompile(`^AKIA[0-9A-Z]{16}$`),                          // AWS Access Key ID
	regexp.MustCompile(`^[A-Za-z0-9/+]{40}$`),                        // AWS Secret (base64-ish 40 chars)
	regexp.MustCompile(`^AIza[0-9A-Za-z\-_]{35,}$`),                  // Google API key
	regexp.MustCompile(`^ey[A-Za-z0-9_\-]{20,}\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+$`), // JWT
	regexp.MustCompile(`^[0-9a-f]{32,64}$`),                          // Generic hex secret (≥32 chars)
}

// looksLikePlaceholder returns true if a value is an env var placeholder like ${FOO} or $FOO.
func looksLikePlaceholder(v string) bool {
	return strings.HasPrefix(v, "${") || (strings.HasPrefix(v, "$") && !strings.Contains(v, " "))
}

// ScanJSONFile inspects all string values in a JSON file for secret patterns.
func ScanJSONFile(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil // not valid JSON; skip
	}

	var findings []Finding
	walkJSON(path, raw, "", &findings)
	return findings, nil
}

func walkJSON(file string, v interface{}, keyPath string, findings *[]Finding) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			path := k
			if keyPath != "" {
				path = keyPath + "." + k
			}
			walkJSON(file, child, path, findings)
		}
	case []interface{}:
		for i, child := range val {
			walkJSON(file, child, fmt.Sprintf("%s[%d]", keyPath, i), findings)
		}
	case string:
		if looksLikePlaceholder(val) {
			return
		}
		for _, re := range secretPatterns {
			if re.MatchString(val) {
				snippet := val
				if len(snippet) > 16 {
					snippet = snippet[:8] + "..." + snippet[len(snippet)-4:]
				}
				*findings = append(*findings, Finding{
					File:    file,
					Key:     keyPath,
					Pattern: re.String(),
					Snippet: snippet,
				})
				break // one match per value is enough
			}
		}
	}
}
