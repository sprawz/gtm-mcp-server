package gtm

import (
	"context"
	"fmt"
	"net/http"

	"gtm-mcp-server/auth"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterTools adds all GTM tools to the MCP server.
func RegisterTools(server *mcp.Server, tokenProvider auth.TokenProvider) {
	globalTokenProvider = tokenProvider

	// Read operations
	registerListAccounts(server)
	registerListContainers(server)
	registerListWorkspaces(server)
	registerListTags(server)
	registerGetTag(server)
	registerListTriggers(server)
	registerGetTrigger(server)
	registerListVariables(server)
	registerGetVariable(server)
	registerListFolders(server)
	registerGetFolderEntities(server)
	registerListTemplates(server)
	registerGetTemplate(server)
	registerListVersions(server)

	// Write operations
	registerCreateTag(server)
	registerUpdateTag(server)
	registerDeleteTag(server)
	registerCreateTrigger(server)
	registerUpdateTrigger(server)
	registerDeleteTrigger(server)
	registerCreateVariable(server)
	registerUpdateVariable(server)
	registerDeleteVariable(server)
	registerCreateContainer(server)
	registerDeleteContainer(server)
	registerCreateWorkspace(server)

	// Workspace status
	registerGetWorkspaceStatus(server)

	// Version operations
	registerCreateVersion(server)
	registerPublishVersion(server)

	// Template operations
	registerImportGalleryTemplate(server)
	registerCreateTemplate(server)
	registerUpdateTemplate(server)
	registerDeleteTemplate(server)

	// Built-in variables
	registerListBuiltInVariables(server)
	registerEnableBuiltInVariables(server)
	registerDisableBuiltInVariables(server)

	// Clients (server-side containers)
	registerListClients(server)
	registerGetClient(server)
	registerCreateClient(server)
	registerUpdateClient(server)
	registerDeleteClient(server)

	// Transformations (server-side containers)
	registerListTransformations(server)
	registerGetTransformation(server)
	registerCreateTransformation(server)
	registerUpdateTransformation(server)
	registerDeleteTransformation(server)

	// Templates (help LLMs with correct parameter formats)
	registerGetTagTemplates(server)
	registerGetTriggerTemplates(server)

	// Resources (URI-based read access)
	RegisterResources(server)

	// Prompts (template workflows)
	RegisterPrompts(server)
}

// globalTokenProvider holds the injected strategy for obtaining tokens.
var globalTokenProvider auth.TokenProvider

// getClient creates a GTM client using the injected token provider.
func getClient(ctx context.Context) (*Client, error) {
	if globalTokenProvider == nil {
		return nil, fmt.Errorf("token provider not configured")
	}

	// We pass a dummy request here as the user OAuth provider pulls from context.
	// A more robust implementation might attach the *http.Request to context upstream.
	dummyReq, _ := http.NewRequestWithContext(ctx, "GET", "/", nil)
	tokenSource, err := globalTokenProvider.GetTokenSource(ctx, dummyReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get token source: %w", err)
	}

	return NewClient(ctx, tokenSource)
}
