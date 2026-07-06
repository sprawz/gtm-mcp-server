package bestpractices

import (
	"slices"
	"strings"
	"testing"
)

func TestTopicsReturnsAllTopics(t *testing.T) {
	topics := Topics()
	expected := []string{"index", "naming-organization", "safe-edit-workflow", "ga4-consent", "server-side"}
	if len(topics) != len(expected) {
		t.Fatalf("expected %d topics, got %d: %v", len(expected), len(topics), topics)
	}
	for _, e := range expected {
		if !slices.Contains(topics, e) {
			t.Errorf("expected topic %q in %v", e, topics)
		}
	}
}

func TestGetReturnsNonEmptyDocs(t *testing.T) {
	for _, topic := range Topics() {
		doc, err := Get(topic)
		if err != nil {
			t.Errorf("Get(%q) returned error: %v", topic, err)
			continue
		}
		if len(strings.TrimSpace(doc)) == 0 {
			t.Errorf("Get(%q) returned empty doc", topic)
		}
		if !strings.HasPrefix(strings.TrimSpace(doc), "#") {
			t.Errorf("Get(%q) doc does not start with a markdown heading", topic)
		}
	}
}

func TestGetUnknownTopic(t *testing.T) {
	_, err := Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown topic, got nil")
	}
	if !strings.Contains(err.Error(), "naming-organization") {
		t.Errorf("error should list valid topics, got: %v", err)
	}
}
