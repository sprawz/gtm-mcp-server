package gtm

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBestPracticesIndexResource(t *testing.T) {
	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "gtm://best-practices"},
	}
	res, err := handleBestPracticesIndexResource(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(res.Contents))
	}
	c := res.Contents[0]
	if c.MIMEType != "text/markdown" {
		t.Errorf("expected text/markdown, got %q", c.MIMEType)
	}
	if !strings.Contains(c.Text, "GTM Configuration Best Practices") {
		t.Errorf("index content missing title, got: %.100s", c.Text)
	}
}

func TestBestPracticesTopicResource(t *testing.T) {
	tests := []struct {
		topic    string
		expected string
	}{
		{"naming-organization", "Naming and Organization"},
		{"safe-edit-workflow", "Safe-Edit Workflow"},
		{"ga4-consent", "GA4 and Consent"},
		{"server-side", "Server-Side Container"},
	}
	for _, tt := range tests {
		req := &mcp.ReadResourceRequest{
			Params: &mcp.ReadResourceParams{URI: "gtm://best-practices/" + tt.topic},
		}
		res, err := handleBestPracticesTopicResource(context.Background(), req)
		if err != nil {
			t.Errorf("topic %q: unexpected error: %v", tt.topic, err)
			continue
		}
		if !strings.Contains(res.Contents[0].Text, tt.expected) {
			t.Errorf("topic %q: content missing %q", tt.topic, tt.expected)
		}
	}
}

func TestBestPracticesUnknownTopic(t *testing.T) {
	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "gtm://best-practices/bogus"},
	}
	_, err := handleBestPracticesTopicResource(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unknown topic")
	}
	if !strings.Contains(err.Error(), "valid topics") {
		t.Errorf("error should list valid topics, got: %v", err)
	}
}
