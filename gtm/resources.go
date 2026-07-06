package gtm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yosida95/uritemplate/v3"

	"gtm-mcp-server/gtm/bestpractices"
)

// URI template patterns for GTM resources
const (
	uriAccounts   = "gtm://accounts"
	uriContainers = "gtm://accounts/{accountId}/containers"
	uriWorkspaces = "gtm://accounts/{accountId}/containers/{containerId}/workspaces"
	uriTags       = "gtm://accounts/{accountId}/containers/{containerId}/workspaces/{workspaceId}/tags"
	uriTriggers   = "gtm://accounts/{accountId}/containers/{containerId}/workspaces/{workspaceId}/triggers"
	uriVariables  = "gtm://accounts/{accountId}/containers/{containerId}/workspaces/{workspaceId}/variables"

	uriBestPractices      = "gtm://best-practices"
	uriBestPracticesTopic = "gtm://best-practices/{topic}"
)

// Compiled URI templates for extracting parameters
var (
	tmplContainers = uritemplate.MustNew(uriContainers)
	tmplWorkspaces = uritemplate.MustNew(uriWorkspaces)
	tmplTags       = uritemplate.MustNew(uriTags)
	tmplTriggers   = uritemplate.MustNew(uriTriggers)
	tmplVariables  = uritemplate.MustNew(uriVariables)

	tmplBestPracticesTopic = uritemplate.MustNew(uriBestPracticesTopic)
)

// RegisterResources adds all GTM resource templates to the MCP server.
func RegisterResources(server *mcp.Server) {
	// gtm://accounts - list all accounts
	server.AddResource(&mcp.Resource{
		Name:        "GTM Accounts",
		Description: "List of all Google Tag Manager accounts accessible to the authenticated user",
		MIMEType:    "application/json",
		URI:         uriAccounts,
	}, handleAccountsResource)

	// gtm://accounts/{accountId}/containers - list containers in account
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "GTM Containers",
		Description: "List of containers in a GTM account",
		MIMEType:    "application/json",
		URITemplate: uriContainers,
	}, handleContainersResource)

	// gtm://accounts/{accountId}/containers/{containerId}/workspaces
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "GTM Workspaces",
		Description: "List of workspaces in a GTM container",
		MIMEType:    "application/json",
		URITemplate: uriWorkspaces,
	}, handleWorkspacesResource)

	// gtm://accounts/{accountId}/containers/{containerId}/workspaces/{workspaceId}/tags
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "GTM Tags",
		Description: "List of all tags in a GTM workspace",
		MIMEType:    "application/json",
		URITemplate: uriTags,
	}, handleTagsResource)

	// gtm://accounts/{accountId}/containers/{containerId}/workspaces/{workspaceId}/triggers
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "GTM Triggers",
		Description: "List of all triggers in a GTM workspace",
		MIMEType:    "application/json",
		URITemplate: uriTriggers,
	}, handleTriggersResource)

	// gtm://accounts/{accountId}/containers/{containerId}/workspaces/{workspaceId}/variables
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "GTM Variables",
		Description: "List of all variables in a GTM workspace",
		MIMEType:    "application/json",
		URITemplate: uriVariables,
	}, handleVariablesResource)

	// gtm://best-practices - configuration best-practices index (static, no auth)
	server.AddResource(&mcp.Resource{
		Name:        "GTM Best Practices",
		Description: "Opinionated rules for good GTM configuration: naming, safe edits, GA4/consent, server-side",
		MIMEType:    "text/markdown",
		URI:         uriBestPractices,
	}, handleBestPracticesIndexResource)

	// gtm://best-practices/{topic} - individual best-practices document
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "GTM Best Practices Topic",
		Description: "A single best-practices document: naming-organization, safe-edit-workflow, ga4-consent, or server-side",
		MIMEType:    "text/markdown",
		URITemplate: uriBestPracticesTopic,
	}, handleBestPracticesTopicResource)
}

func handleAccountsResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	accounts, err := client.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(map[string]any{"accounts": accounts}, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func handleContainersResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Extract accountId from URI using regex
	match := tmplContainers.Regexp().FindStringSubmatch(req.Params.URI)
	if len(match) < 2 {
		return nil, fmt.Errorf("invalid URI: could not extract accountId")
	}
	accountID := match[1]

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	containers, err := client.ListContainers(ctx, accountID)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(map[string]any{"containers": containers}, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func handleWorkspacesResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	match := tmplWorkspaces.Regexp().FindStringSubmatch(req.Params.URI)
	if len(match) < 3 {
		return nil, fmt.Errorf("invalid URI: could not extract accountId and containerId")
	}
	accountID := match[1]
	containerID := match[2]

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	workspaces, err := client.ListWorkspaces(ctx, accountID, containerID)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(map[string]any{"workspaces": workspaces}, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func handleTagsResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	match := tmplTags.Regexp().FindStringSubmatch(req.Params.URI)
	if len(match) < 4 {
		return nil, fmt.Errorf("invalid URI: could not extract accountId, containerId, and workspaceId")
	}
	accountID := match[1]
	containerID := match[2]
	workspaceID := match[3]

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	tags, err := client.ListTags(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(map[string]any{"tags": tags}, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func handleBestPracticesIndexResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	doc, err := bestpractices.Get("index")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "text/markdown", Text: doc},
		},
	}, nil
}

func handleBestPracticesTopicResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	match := tmplBestPracticesTopic.Regexp().FindStringSubmatch(req.Params.URI)
	if len(match) < 2 {
		return nil, fmt.Errorf("invalid URI: could not extract topic")
	}
	doc, err := bestpractices.Get(match[1])
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "text/markdown", Text: doc},
		},
	}, nil
}

func handleTriggersResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	match := tmplTriggers.Regexp().FindStringSubmatch(req.Params.URI)
	if len(match) < 4 {
		return nil, fmt.Errorf("invalid URI: could not extract accountId, containerId, and workspaceId")
	}
	accountID := match[1]
	containerID := match[2]
	workspaceID := match[3]

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	triggers, err := client.ListTriggers(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(map[string]any{"triggers": triggers}, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func handleVariablesResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	match := tmplVariables.Regexp().FindStringSubmatch(req.Params.URI)
	if len(match) < 4 {
		return nil, fmt.Errorf("invalid URI: could not extract accountId, containerId, and workspaceId")
	}
	accountID := match[1]
	containerID := match[2]
	workspaceID := match[3]

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	variables, err := client.ListVariables(ctx, accountID, containerID, workspaceID)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(map[string]any{"variables": variables}, "", "  ")
	if err != nil {
		return nil, err
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}
