package gtm

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPlanSafeEditPrompt(t *testing.T) {
	req := &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{
			Arguments: map[string]string{
				"accountId":          "123",
				"containerId":        "456",
				"change_description": "Add GA4 purchase event tracking",
			},
		},
	}
	res, err := handlePlanSafeEditPrompt(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := res.Messages[0].Content.(*mcp.TextContent).Text
	for _, want := range []string{"Safe-Edit Workflow", "Add GA4 purchase event tracking", "create_workspace", "get_workspace_status", "create_version", "publish_version"} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt text missing %q", want)
		}
	}
}

func TestPlanSafeEditPromptMissingArgs(t *testing.T) {
	req := &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{Arguments: map[string]string{"accountId": "123"}},
	}
	if _, err := handlePlanSafeEditPrompt(context.Background(), req); err == nil {
		t.Fatal("expected error for missing arguments")
	}
}

func TestBestPracticesReviewPromptMissingArgs(t *testing.T) {
	req := &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{Arguments: map[string]string{}},
	}
	if _, err := handleBestPracticesReviewPrompt(context.Background(), req); err == nil {
		t.Fatal("expected error for missing arguments")
	}
}
