// Package bestpractices embeds the GTM configuration best-practice
// documents served via MCP resources and injected into prompts.
package bestpractices

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed docs/*.md
var docsFS embed.FS

// Topics returns the available best-practice topic names (without .md).
func Topics() []string {
	entries, err := docsFS.ReadDir("docs")
	if err != nil {
		return nil
	}
	topics := make([]string, 0, len(entries))
	for _, e := range entries {
		topics = append(topics, strings.TrimSuffix(e.Name(), ".md"))
	}
	sort.Strings(topics)
	return topics
}

// Get returns the markdown content for a topic.
func Get(topic string) (string, error) {
	data, err := docsFS.ReadFile("docs/" + topic + ".md")
	if err != nil {
		return "", fmt.Errorf("unknown best-practices topic %q; valid topics: %s", topic, strings.Join(Topics(), ", "))
	}
	return string(data), nil
}
