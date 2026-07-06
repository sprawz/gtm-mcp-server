package gtm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"gtm-mcp-server/gtm/bestpractices"
)

// RegisterPrompts adds all GTM prompts to the MCP server.
func RegisterPrompts(server *mcp.Server) {
	// Audit container prompt - analyzes workspace for issues
	server.AddPrompt(&mcp.Prompt{
		Name:        "audit_container",
		Description: "Analyze a GTM workspace for potential issues, duplicates, naming inconsistencies, and best practice violations",
		Arguments: []*mcp.PromptArgument{
			{Name: "accountId", Description: "The GTM account ID", Required: true},
			{Name: "containerId", Description: "The GTM container ID", Required: true},
			{Name: "workspaceId", Description: "The GTM workspace ID", Required: true},
		},
	}, handleAuditContainerPrompt)

	// Generate tracking plan prompt - creates markdown documentation
	server.AddPrompt(&mcp.Prompt{
		Name:        "generate_tracking_plan",
		Description: "Generate a Markdown tracking plan document from existing tags, triggers, and variables in a workspace",
		Arguments: []*mcp.PromptArgument{
			{Name: "accountId", Description: "The GTM account ID", Required: true},
			{Name: "containerId", Description: "The GTM container ID", Required: true},
			{Name: "workspaceId", Description: "The GTM workspace ID", Required: true},
		},
	}, handleGenerateTrackingPlanPrompt)

	// Suggest GA4 setup prompt - recommends tag structure
	server.AddPrompt(&mcp.Prompt{
		Name:        "suggest_ga4_setup",
		Description: "Recommend a GA4 tag structure based on tracking goals and requirements",
		Arguments: []*mcp.PromptArgument{
			{Name: "goals", Description: "Description of tracking goals (e.g., 'ecommerce purchase tracking, form submissions, button clicks')", Required: true},
		},
	}, handleSuggestGA4SetupPrompt)

	// Find gallery template prompt - guides LLM to discover templates
	server.AddPrompt(&mcp.Prompt{
		Name:        "find_gallery_template",
		Description: "Guide to find and import a Community Template Gallery template by name",
		Arguments: []*mcp.PromptArgument{
			{Name: "templateName", Description: "The name of the template to find (e.g., 'iubenda', 'cookiebot', 'facebook pixel')", Required: true},
		},
	}, handleFindGalleryTemplatePrompt)

	// Best practices review - scored review against embedded rule docs
	server.AddPrompt(&mcp.Prompt{
		Name:        "best_practices_review",
		Description: "Score a GTM workspace against configuration best practices (naming, safe edits, GA4/consent, server-side) with pass/warn/fail per category and concrete fixes",
		Arguments: []*mcp.PromptArgument{
			{Name: "accountId", Description: "The GTM account ID", Required: true},
			{Name: "containerId", Description: "The GTM container ID", Required: true},
			{Name: "workspaceId", Description: "The GTM workspace ID", Required: true},
		},
	}, handleBestPracticesReviewPrompt)

	// Plan safe edit - step-by-step plan following the safe-edit workflow
	server.AddPrompt(&mcp.Prompt{
		Name:        "plan_safe_edit",
		Description: "Produce a step-by-step plan for a GTM change following the safe-edit workflow: workspace, diff review, version, approved publish",
		Arguments: []*mcp.PromptArgument{
			{Name: "accountId", Description: "The GTM account ID", Required: true},
			{Name: "containerId", Description: "The GTM container ID", Required: true},
			{Name: "change_description", Description: "Description of the change to make (e.g., 'Add GA4 purchase event tracking')", Required: true},
		},
	}, handlePlanSafeEditPrompt)
}

func handleAuditContainerPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	accountID := req.Params.Arguments["accountId"]
	containerID := req.Params.Arguments["containerId"]
	workspaceID := req.Params.Arguments["workspaceId"]

	if accountID == "" || containerID == "" || workspaceID == "" {
		return nil, fmt.Errorf("accountId, containerId, and workspaceId are required")
	}

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch all workspace data
	tags, err := client.ListTags(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	triggers, err := client.ListTriggers(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list triggers: %w", err)
	}

	variables, err := client.ListVariables(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list variables: %w", err)
	}

	// Build the workspace data JSON
	workspaceData := map[string]any{
		"tags":      tags,
		"triggers":  triggers,
		"variables": variables,
		"summary": map[string]int{
			"totalTags":      len(tags),
			"totalTriggers":  len(triggers),
			"totalVariables": len(variables),
		},
	}

	dataJSON, err := json.MarshalIndent(workspaceData, "", "  ")
	if err != nil {
		return nil, err
	}

	namingRules, err := bestpractices.Get("naming-organization")
	if err != nil {
		return nil, err
	}
	consentRules, err := bestpractices.Get("ga4-consent")
	if err != nil {
		return nil, err
	}

	return &mcp.GetPromptResult{
		Description: "Container audit analysis request",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(`Please audit this GTM workspace for potential issues. Here is the current configuration:

%s

Please analyze and report on:

1. **Naming Consistency**
   - Are tag, trigger, and variable names following a consistent pattern?
   - Are there any names that are unclear or non-descriptive?

2. **Duplicate Detection**
   - Are there any tags that appear to be duplicates (same type and similar configuration)?
   - Are there triggers that fire on the same conditions?

3. **Orphaned Items**
   - Are there any triggers that are not used by any tags?
   - Are there any variables that don't appear to be referenced?

4. **Best Practices**
   - Are tags properly organized with appropriate triggers?
   - Are there any paused tags that might be forgotten?
   - Are there missing triggers for common use cases?

5. **GA4 Configuration** (if applicable)
   - Is there a GA4 configuration tag?
   - Are event tags properly linked to the configuration?
   - Are ecommerce events configured correctly?

6. **Security Concerns**
   - Are there any custom HTML tags that might pose security risks?
   - Are there any tags loading external scripts?

Use the following rules as the reference standard for sections 1, 3, 4, and 5. If the container consistently follows its own different convention, treat that as acceptable and note the difference:

%s

---

%s

Please provide specific recommendations for improvements.`, string(dataJSON), namingRules, consentRules),
				},
			},
		},
	}, nil
}

func handleGenerateTrackingPlanPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	accountID := req.Params.Arguments["accountId"]
	containerID := req.Params.Arguments["containerId"]
	workspaceID := req.Params.Arguments["workspaceId"]

	if accountID == "" || containerID == "" || workspaceID == "" {
		return nil, fmt.Errorf("accountId, containerId, and workspaceId are required")
	}

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch all workspace data
	tags, err := client.ListTags(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	triggers, err := client.ListTriggers(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list triggers: %w", err)
	}

	variables, err := client.ListVariables(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list variables: %w", err)
	}

	// Create trigger lookup map
	triggerMap := make(map[string]string)
	for _, t := range triggers {
		triggerMap[t.TriggerID] = t.Name
	}

	// Build the workspace data JSON
	workspaceData := map[string]any{
		"tags":       tags,
		"triggers":   triggers,
		"variables":  variables,
		"triggerMap": triggerMap,
	}

	dataJSON, err := json.MarshalIndent(workspaceData, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.GetPromptResult{
		Description: "Generate tracking plan documentation",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(`Please generate a comprehensive Markdown tracking plan document from this GTM workspace configuration:

%s

Generate a document with the following structure:

# Tracking Plan

## Overview
- Summary of the tracking implementation
- Total counts (tags, triggers, variables)

## Events

For each tag, create a section:

### [Event Name]
- **Tag Name:** [name]
- **Tag Type:** [type]
- **Trigger(s):** [list of trigger names]
- **Description:** [inferred purpose]
- **Parameters:** [if applicable]

## Triggers

For each trigger:

### [Trigger Name]
- **Type:** [type]
- **Conditions:** [filter conditions if any]
- **Used by:** [list of tags using this trigger]

## Variables

For each variable:

### [Variable Name]
- **Type:** [type]
- **Purpose:** [inferred purpose]

## Data Layer Requirements

List all dataLayer events and variables that need to be pushed from the website.

## Implementation Notes

Any observations about the implementation, dependencies, or recommendations.

Format the output as clean, professional Markdown.`, string(dataJSON)),
				},
			},
		},
	}, nil
}

func handleSuggestGA4SetupPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	goals := req.Params.Arguments["goals"]

	if goals == "" {
		return nil, fmt.Errorf("goals description is required")
	}

	// Get the available tag and trigger templates
	tagTemplates := GetTagTemplates()
	triggerTemplates := GetTriggerTemplates()

	templatesData := map[string]any{
		"tagTemplates":     tagTemplates,
		"triggerTemplates": triggerTemplates,
	}

	templatesJSON, err := json.MarshalIndent(templatesData, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.GetPromptResult{
		Description: "GA4 setup recommendations",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(`I need help setting up GA4 tracking in Google Tag Manager for the following goals:

**Tracking Goals:**
%s

Here are the available tag and trigger templates that can be used:

%s

Please provide:

1. **Recommended Tags**
   - List each tag needed with:
     - Tag name (following naming convention: "[Category] - [Action]")
     - Tag type
     - Configuration details
     - Which trigger to use

2. **Recommended Triggers**
   - List each trigger needed with:
     - Trigger name
     - Trigger type
     - Filter conditions (if any)

3. **Required Variables**
   - List any Data Layer variables needed
   - List any built-in variables to enable

4. **Data Layer Requirements**
   - Specify what dataLayer pushes the website needs to implement
   - Provide example code snippets for each event

5. **Implementation Order**
   - Step-by-step order to create the tags, triggers, and variables

6. **Testing Checklist**
   - Key scenarios to test
   - Expected GA4 events and parameters

Please be specific about the GTM configuration - use the exact parameter formats shown in the templates.`, goals, string(templatesJSON)),
				},
			},
		},
	}, nil
}

func handleBestPracticesReviewPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	accountID := req.Params.Arguments["accountId"]
	containerID := req.Params.Arguments["containerId"]
	workspaceID := req.Params.Arguments["workspaceId"]

	if accountID == "" || containerID == "" || workspaceID == "" {
		return nil, fmt.Errorf("accountId, containerId, and workspaceId are required")
	}

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	tags, err := client.ListTags(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	triggers, err := client.ListTriggers(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list triggers: %w", err)
	}
	variables, err := client.ListVariables(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list variables: %w", err)
	}

	workspaceData := map[string]any{
		"tags":      tags,
		"triggers":  triggers,
		"variables": variables,
	}
	dataJSON, err := json.MarshalIndent(workspaceData, "", "  ")
	if err != nil {
		return nil, err
	}

	var rules strings.Builder
	for _, topic := range []string{"naming-organization", "ga4-consent", "server-side"} {
		doc, err := bestpractices.Get(topic)
		if err != nil {
			return nil, err
		}
		rules.WriteString(doc)
		rules.WriteString("\n\n---\n\n")
	}

	return &mcp.GetPromptResult{
		Description: "Best practices review request",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(`Please review this GTM workspace against the configuration best practices below.

## Workspace configuration

%s

## Best practice rules

%s

## Instructions

For each category (Naming and Organization, GA4 and Consent, Server-Side if applicable), score the workspace:

- **pass** — rules followed
- **warn** — minor deviations, list them
- **fail** — clear violations, list them

For every warn/fail, give the concrete fix: exact entity name, what to rename/change it to, or which tool call to make. If the container has its own consistent convention that differs from these rules, treat consistency with the existing convention as passing and note the difference instead. Skip the Server-Side category for web containers (no clients present). End with a prioritized fix list.`, string(dataJSON), rules.String()),
				},
			},
		},
	}, nil
}

func handlePlanSafeEditPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	accountID := req.Params.Arguments["accountId"]
	containerID := req.Params.Arguments["containerId"]
	changeDescription := req.Params.Arguments["change_description"]

	if accountID == "" || containerID == "" || changeDescription == "" {
		return nil, fmt.Errorf("accountId, containerId, and change_description are required")
	}

	workflow, err := bestpractices.Get("safe-edit-workflow")
	if err != nil {
		return nil, err
	}
	naming, err := bestpractices.Get("naming-organization")
	if err != nil {
		return nil, err
	}

	return &mcp.GetPromptResult{
		Description: "Safe edit plan request",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(`I want to make the following change to GTM container %s (account %s):

**Change:** %s

Follow this workflow strictly:

%s

Apply this naming convention to any new entities:

%s

Produce a step-by-step execution plan with the exact tool calls (create_workspace, then entity creation in dependency order with proposed names, then get_workspace_status, create_version with a proposed version name, and finally publish_version pending my approval). Show me the plan before executing anything.`, containerID, accountID, changeDescription, workflow, naming),
				},
			},
		},
	}, nil
}

func handleFindGalleryTemplatePrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	templateName := req.Params.Arguments["templateName"]

	if templateName == "" {
		return nil, fmt.Errorf("templateName is required")
	}

	return &mcp.GetPromptResult{
		Description: "Find and import a Community Template Gallery template",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(`I need to find and import the "%s" template from the GTM Community Template Gallery.

**How to find a Community Template:**

1. **Search the web** for: "%s GTM community template github"
   - Community templates are hosted on GitHub
   - Look for results from github.com

2. **Extract the repository info** from the GitHub URL:
   - URL format: github.com/{owner}/{repository}
   - Example: github.com/iubenda/gtm-cookie-solution
     - galleryOwner: "iubenda"
     - galleryRepository: "gtm-cookie-solution"

3. **Browse the Gallery directly** (optional):
   - Visit: https://tagmanager.google.com/gallery/#/?filter=%s
   - Click on the template to see details

**Common templates for reference:**

| Template | galleryOwner | galleryRepository |
|----------|--------------|-------------------|
| iubenda Cookie Solution | iubenda | gtm-cookie-solution |
| Cookiebot | nicktue-gtm-templates | cookiebot-gtm |
| Facebook Pixel | nicktue-gtm-templates | facebook-pixel |

**Once you have the owner and repository:**

Use the import_gallery_template tool:
- galleryOwner: [owner from GitHub]
- galleryRepository: [repository from GitHub]

The tool will return the template type (cvt_{containerId}_{templateId}) to use when creating tags.

Please search for the "%s" template and provide the galleryOwner and galleryRepository values.`, templateName, templateName, templateName, templateName),
				},
			},
		},
	}, nil
}
