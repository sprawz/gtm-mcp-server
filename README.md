# GTM MCP Server

[![License](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-Streamable_HTTP-8A2BE2)](https://modelcontextprotocol.io/)
[![Security Checks](https://github.com/paolobietolini/gtm-mcp-server/actions/workflows/security.yml/badge.svg)](https://github.com/paolobietolini/gtm-mcp-server/actions/workflows/security.yml)
[![GitHub release](https://img.shields.io/github/v/release/paolobietolini/gtm-mcp-server)](https://github.com/paolobietolini/gtm-mcp-server/releases)

GTM MCP Server connects MCP clients to the Google Tag Manager API. It can
inspect containers, create and update workspace entities, create versions, and
publish a selected version after explicit confirmation.

Use the hosted server at:

```text
https://mcp.gtmeditor.com
```

The server supports browser-based Google OAuth for individual users and
service-account authentication for self-hosted automation.

## Project status

| Item | Current state |
|---|---|
| Version in `server.json` | `1.10.1` |
| Transport | MCP Streamable HTTP |
| Runtime tools | 64 GTM tools by default; 94 with `GTM_TOOL_GROUPS=all`, plus 2 utility tools |
| MCP resources | 8 resource definitions |
| MCP prompts | 6 prompts |
| Official GTM API coverage | 101 of 106 methods |
| Product parity target | 101 of 106 methods, reached |
| Hosted endpoint | `https://mcp.gtmeditor.com` |

The agreed API parity scope is complete. The project implements 101 methods
from Google's 106-method GTM v2 discovery surface. The five
`accounts.user_permissions` methods are intentionally excluded because granting
and revoking GTM access needs a separate privilege-management design.

Tool count and API-method count are different. Some tools provide local
guidance, while some helpers cover more than one Google API call.

## Connect an MCP client

Add the hosted URL as a remote HTTP MCP server. The client should discover the
OAuth metadata, open Google sign-in, and reconnect with the issued bearer
token.

### Claude Code

```bash
claude mcp add --transport http gtm https://mcp.gtmeditor.com
```

### Gemini CLI

```bash
gemini mcp add --transport http gtm https://mcp.gtmeditor.com
```

Gemini CLI can also use this `settings.json` entry:

```json
{
  "mcpServers": {
    "gtm": {
      "httpUrl": "https://mcp.gtmeditor.com"
    }
  }
}
```

### Cursor

Add the following to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "gtm": {
      "url": "https://mcp.gtmeditor.com"
    }
  }
}
```

### ChatGPT, Codex, and Claude web

Add a custom or remote MCP connection in the client and use
`https://mcp.gtmeditor.com` as the server URL. Product menus change more often
than this server, so refer to the client's current MCP connection instructions
if the label differs.

After connecting, try:

```text
List my GTM accounts, containers, and workspaces. Do not make changes.
```

The Google account used during OAuth determines which GTM accounts the server
can access.

## Typical workflow

1. Call `list_accounts`, `list_containers`, and `list_workspaces` to discover
   IDs. Do not guess IDs.
2. Inspect the workspace with list/get tools or the `audit_container` prompt.
3. Create or update tags, triggers, variables, templates, clients, or
   transformations in the selected workspace.
4. Call `get_workspace_status` and resolve conflicts before versioning.
5. Call `create_version` to snapshot the workspace.
6. Inspect the saved version with `get_version`.
7. Call `publish_version` with `confirm: true` only when the selected version
   is ready to go live.
8. Verify the published state with `get_live_version`.

Example requests:

- "Audit this workspace for duplicate tags and unused triggers."
- "Create a GA4 purchase event tag, but do not publish it."
- "Show the difference between the latest saved version and the live version."
- "Generate a Markdown tracking plan from this workspace."
- "Import the iubenda template from the Community Template Gallery."

## Tools

All GTM tools use the authenticated Google identity from the current MCP
request. Inputs and outputs are structured JSON.

### Utility

| Tool | Purpose |
|---|---|
| `ping` | Test MCP connectivity |
| `auth_status` | Check whether the current request is authenticated |

### Accounts and containers

| Tool | Purpose |
|---|---|
| `list_accounts` | List accessible GTM accounts |
| `update_account` | Rename an account |
| `list_containers` | List containers and their public IDs and settings |
| `lookup_container` | Find a container by destination ID or GTM public tag ID |
| `get_container_snippet` | Get a web install snippet or server-container configuration |
| `create_container` | Create a web, app, AMP, or server container |
| `update_container` | Rename a container while preserving its other settings |
| `delete_container` | Permanently delete a container; requires `confirm: true` |
| `combine_containers` | Merge a source container into a target; requires `confirm: true` |
| `move_tag_id` | Move a tag ID into a new container; requires confirmation and terms acceptance |

### Workspaces

| Tool | Purpose |
|---|---|
| `list_workspaces` | List workspaces in a container |
| `create_workspace` | Create a workspace |
| `get_workspace` | Get workspace metadata and its current fingerprint |
| `update_workspace` | Update selected workspace fields |
| `delete_workspace` | Delete a workspace; requires `confirm: true` |
| `quick_preview_workspace` | Compile an ephemeral preview without saving or publishing |
| `get_workspace_status` | Show pending changes and merge conflicts |
| `bulk_update_workspace` | Apply multiple entity changes; requires `confirm: true` |
| `resolve_workspace_conflict` | Replace a conflict with a resolved entity; requires `confirm: true` |
| `sync_workspace` | Synchronize with the latest container version; requires `confirm: true` |

Bulk update and conflict resolution accept raw GTM Entity JSON so every entity
type supported by the official API remains available.

### Tags

| Tool | Purpose |
|---|---|
| `list_tags` | List workspace tags |
| `get_tag` | Get complete tag details |
| `create_tag` | Create a tag |
| `update_tag` | Update a tag with fingerprint-based concurrency control |
| `delete_tag` | Delete a tag; requires `confirm: true` |

### Triggers

| Tool | Purpose |
|---|---|
| `list_triggers` | List workspace triggers |
| `get_trigger` | Get complete trigger details |
| `create_trigger` | Create a trigger |
| `update_trigger` | Update a trigger with fingerprint-based concurrency control |
| `delete_trigger` | Delete a trigger; requires `confirm: true` |

For `update_trigger`, omit `filterJson`, `customEventFilterJson`,
`autoEventFilterJson`, or `parameterJson` to preserve the current field. Pass
the JSON string `"[]"` to clear a field, or a non-empty JSON array to replace
it. For click, link-click, and form-submission triggers, use `filterJson` because
GTM drops `autoEventFilter` for those trigger types.

### Variables

| Tool | Purpose |
|---|---|
| `list_variables` | List workspace variables |
| `get_variable` | Get complete variable details |
| `create_variable` | Create a variable |
| `update_variable` | Update a variable with fingerprint-based concurrency control |
| `delete_variable` | Delete a variable; requires `confirm: true` |

### Folders and built-in variables

| Tool | Purpose |
|---|---|
| `list_folders` | List workspace folders |
| `get_folder` | Get complete folder metadata |
| `create_folder` | Create a folder |
| `update_folder` | Update a folder with fingerprint protection |
| `delete_folder` | Delete a folder; requires `confirm: true` |
| `get_folder_entities` | List tags, triggers, and variables assigned to a folder |
| `move_entities_to_folder` | Move tags, triggers, and variables; requires `confirm: true` |
| `revert_folder` | Discard workspace folder changes; requires `confirm: true` |
| `list_built_in_variables` | List enabled built-in variables |
| `enable_built_in_variables` | Enable built-in variable types |
| `disable_built_in_variables` | Disable built-in variable types; requires `confirm: true` |

Use `revert_workspace_entity` to discard changes to a built-in variable.

### Zones

| Tool | Purpose |
|---|---|
| `list_zones` | List all zones across every result page |
| `get_zone` | Get boundary, child-container, and type-restriction configuration |
| `create_zone` | Create a workspace zone |
| `update_zone` | Update selected fields with fingerprint concurrency control |
| `delete_zone` | Delete a zone; requires `confirm: true` |

Use `revert_workspace_entity` to discard changes to a zone.

### Environments

The `environments` tool group is optional. Enable it with
`GTM_TOOL_GROUPS=all` or add `environments` to an explicit group list.

| Tool | Purpose |
|---|---|
| `list_environments` | List all container environments across every result page |
| `get_environment` | Get environment configuration and authorization metadata |
| `create_environment` | Create a user environment |
| `update_environment` | Update selected fields with fingerprint concurrency control |
| `reauthorize_environment` | Rotate the authorization code; requires `confirm: true` |
| `delete_environment` | Delete a user environment; requires `confirm: true` |

The Live and Latest environments are managed by GTM. Creation and deletion
apply to user environments; GTM permits URL and debug updates on other types.

### Destinations and Google tag configurations

These tools are in the optional `destinations` and `gtag` groups.

| Tool | Purpose |
|---|---|
| `list_destinations` | List Google tag destinations linked to a container |
| `get_destination` | Get a destination by its link ID |
| `link_destination` | Move a destination to a container; requires `confirm: true` |
| `list_google_tag_configs` | List Google tag configurations in a workspace |
| `get_google_tag_config` | Get a Google tag configuration |
| `create_google_tag_config` | Create a Google tag configuration |
| `update_google_tag_config` | Update a configuration with fingerprint protection |
| `delete_google_tag_config` | Delete a configuration; requires `confirm: true` |

Container combine, tag-ID move, and destination link operations do not copy or
enable user permissions. Account permission management remains outside the
current parity target.

### Custom templates

| Tool | Purpose |
|---|---|
| `list_templates` | List custom templates |
| `get_template` | Get template metadata and template code |
| `create_template` | Create a custom template from `.tpl` code |
| `update_template` | Update template code |
| `delete_template` | Delete an unused template; requires `confirm: true` |
| `import_gallery_template` | Import a Community Template Gallery template |
| `get_tag_templates` | Return compact GA4 and Custom HTML input examples |
| `get_trigger_templates` | Return compact trigger input examples |

The full JSON examples live in the
`gtm://best-practices/tool-input-formats` resource so every tool listing does
not repeat them.

### Server-side containers

| Tool | Purpose |
|---|---|
| `list_clients` | List server-container clients |
| `get_client` | Get a client |
| `create_client` | Create a client |
| `update_client` | Update a client |
| `delete_client` | Delete a client; requires `confirm: true` |
| `list_transformations` | List transformations |
| `get_transformation` | Get a transformation |
| `create_transformation` | Create a transformation |
| `update_transformation` | Update a transformation |
| `delete_transformation` | Delete a transformation; requires `confirm: true` |

Use `revert_workspace_entity` to discard changes to a client or transformation.

### Versions and publication

| Tool | Purpose |
|---|---|
| `list_versions` | List all saved version headers across every result page |
| `get_latest_version_header` | Get the latest saved header, which may differ from live |
| `get_version` | Get a saved version and its complete entity collections |
| `get_live_version` | Get the currently published version and its entities |
| `create_version` | Create a version from a conflict-free workspace |
| `publish_version` | Publish a selected version; requires `confirm: true` |
| `update_version` | Update a saved version's name or description |
| `delete_version` | Soft-delete a version; requires `confirm: true` |
| `undelete_version` | Restore a soft-deleted version; requires `confirm: true` |
| `set_latest_version` | Make a version Latest without publishing; requires `confirm: true` |

### Workspace reverts

| Tool | Purpose |
|---|---|
| `revert_workspace_entity` | Discard workspace changes to a built-in variable, client, tag, template, transformation, trigger, variable, or zone; requires `confirm: true` |

The tool fetches the current entity fingerprint before calling the matching
official revert method. A successful result can have `existsAfterRevert: false`
when the entity does not exist in the latest container version.

## Safety model

- Delete operations, publication, and disabling built-in variables require
  `confirm: true`.
- Update operations fetch the current resource fingerprint and use Google's
  optimistic concurrency checks.
- `create_version` first checks that the workspace has changes and no merge
  conflicts.
- Read tools distinguish latest saved state from published live state.
- Google API errors are mapped to clearer not-found, permission, conflict, and
  rate-limit failures. Retryable API failures use bounded backoff.
- MCP request bodies are limited to 5 MiB. OAuth and dynamic registration
  endpoints have stricter rate and body limits.

Most entity edits affect a workspace and are not live until a version is
published. Account and container operations act directly on those resources.
Always review the target IDs and the generated version before publication.

## Resources

The server exposes two concrete resources and six URI templates:

| URI | Content |
|---|---|
| `gtm://accounts` | Accessible accounts |
| `gtm://accounts/{accountId}/containers` | Containers |
| `gtm://accounts/{accountId}/containers/{containerId}/workspaces` | Workspaces |
| `gtm://accounts/{accountId}/containers/{containerId}/workspaces/{workspaceId}/tags` | Tags |
| `gtm://accounts/{accountId}/containers/{containerId}/workspaces/{workspaceId}/triggers` | Triggers |
| `gtm://accounts/{accountId}/containers/{containerId}/workspaces/{workspaceId}/variables` | Variables |
| `gtm://best-practices` | Best-practice topic index |
| `gtm://best-practices/{topic}` | One embedded guidance document |

Best-practice topics include naming and organization, safe edits, GA4 and
consent, server-side containers, and detailed tool input formats.

## Prompts

| Prompt | Purpose |
|---|---|
| `audit_container` | Review tags, triggers, and variables for quality problems |
| `generate_tracking_plan` | Build a Markdown tracking plan from a workspace |
| `suggest_ga4_setup` | Recommend a GA4 structure from stated goals |
| `find_gallery_template` | Guide Community Gallery discovery and import |
| `best_practices_review` | Score a workspace against embedded guidance |
| `plan_safe_edit` | Produce a staged edit/version/publish plan |

Prompts prepare context and instructions for the model. They do not bypass tool
authentication or mutation safeguards.

## Authentication

### Hosted OAuth

The hosted server implements MCP OAuth discovery and Google authorization. It
supports PKCE, Dynamic Client Registration, Client ID Metadata Documents,
authorization-server metadata, and protected-resource metadata.

The server never receives a Google password. It receives Google OAuth tokens
after consent. A self-hosted operator can keep issued MCP and Google tokens
across restarts by setting `TOKEN_STORE_PATH`; without that setting they remain
in memory and disappear when the process stops.

Expired MCP access tokens can be renewed transparently while their Google
refresh credentials remain valid. `AUTH_AUTO_REFRESH_MAX_AGE` limits how long
one bearer can be silently extended; its default is seven days.

### Service-account mode

Service-account mode gives every holder of one server API key the GTM access
granted to the configured Google service account.

1. Create a Google service account.
2. Add its email address to the required GTM account with only the permissions
   it needs.
3. Set a strong `SERVICE_ACCOUNT_API_KEY` on this server.
4. Set `GOOGLE_SERVICE_ACCOUNT_KEY_JSON` to the key JSON, or use Application
   Default Credentials on Google Cloud.
5. Configure the MCP client to send
   `Authorization: Bearer <SERVICE_ACCOUNT_API_KEY>`.

OAuth and service-account mode can run together. Requests with the configured
API key use the service account; OAuth users keep their own Google identity and
GTM permissions.

## Self-hosting

### Requirements

- Go 1.26 or Docker
- A Google Cloud project with the Tag Manager API enabled
- A Google OAuth web client for user OAuth, or a Google service account for S2S
- HTTPS and a stable public URL for remote OAuth deployments

### Google OAuth setup

Create an OAuth 2.0 Web application in Google Cloud and add this authorized
redirect URI:

```text
https://your-host.example/oauth/callback
```

The scheme and host must match `BASE_URL` exactly. For local development, use:

```text
http://localhost:8080/oauth/callback
```

### Run from source

```bash
git clone https://github.com/paolobietolini/gtm-mcp-server.git
cd gtm-mcp-server

cat > .env <<'EOF'
BASE_URL=http://localhost:8080
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
TOKEN_STORE_PATH=./data/tokens.json
EOF

go run .
```

The repository reads `.env` and then `.env.local`; `.env.local` overrides
`.env`. Both are ignored by Git.

### Run with Docker

```bash
docker build -t gtm-mcp-server .

docker run --rm \
  --name gtm-mcp-server \
  -p 8080:8080 \
  --env-file .env \
  -v gtm-mcp-tokens:/data \
  gtm-mcp-server
```

When using the named volume, set `TOKEN_STORE_PATH=/data/tokens.json` in
`.env`.

### Configuration

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `BASE_URL` | `http://localhost:8080` | Canonical public URL used by OAuth metadata and callbacks |
| `GOOGLE_CLIENT_ID` | empty | Google OAuth web-client ID |
| `GOOGLE_CLIENT_SECRET` | empty | Google OAuth web-client secret |
| `ACCESS_TOKEN_TTL` | `8h` | Lifetime of MCP access tokens |
| `AUTH_AUTO_REFRESH_MAX_AGE` | `168h` | Maximum silent-renewal chain age |
| `TOKEN_STORE_PATH` | empty | Optional persisted token-store file |
| `SERVICE_ACCOUNT_API_KEY` | empty | Bearer API key that enables S2S mode |
| `GOOGLE_SERVICE_ACCOUNT_KEY_JSON` | empty | Service-account JSON; omit when ADC is available |
| `ALLOWED_HOSTS` | empty | Additional trusted hosts for Docker/internal URL resolution |
| `TRUST_PROXY` | `false` | Trust the rightmost `X-Forwarded-For` hop for rate limiting |
| `LOG_LEVEL` | `info` | Set `debug` for additional structured logs |
| `GTM_TOOL_GROUPS` | current groups | Comma-separated tool families advertised through MCP |

If OAuth and service-account credentials are both absent, the server starts in
open mode. Use open mode only for isolated local development.

`TRUST_PROXY=true` is appropriate only when a trusted reverse proxy overwrites
or appends `X-Forwarded-For`. The implementation uses the rightmost value. With
multiple proxy hops, configure and test the trust boundary before relying on
per-client rate limits.

`ALLOWED_HOSTS` is a comma-separated allowlist used when the same server is
reached through trusted internal Docker hostnames. Do not add arbitrary public
hosts.

`GTM_TOOL_GROUPS` controls schema size for clients that need only part of the
API. Available groups are `accounts`, `workspaces`, `tags`, `triggers`,
`variables`, `folders`, `builtins`, `zones`, `templates`, `server`, `guidance`,
`environments`, `destinations`, `gtag`, `container-admin`, `folder-admin`, and
`workspace-admin`, `version-admin`, and `reverts`. Leaving it unset preserves
the established 64-tool surface. New parity groups are opt-in. Use `all` to
include every current and future parity family, or select a subset:

```text
GTM_TOOL_GROUPS=accounts,workspaces,tags,triggers,variables
```

The two connection utility tools remain available regardless of this setting.

## Releases and deployment

`server.json` is the source of truth for the runtime version. It is embedded in
the Go binary and returned by `/health`; there is no second version constant in
Go code.

To publish a release:

1. Update `server.json`.
2. Commit the release changes.
3. Push a matching tag such as `v1.11.0`.

The release workflow rejects a tag that does not match `server.json`, creates
cross-platform archives with GoReleaser, and then deploys the tagged source to
the production VPS. The deployment uses the GitHub environment
`auto-deployment` and expects these secrets:

- `SSH_PRIVATE_KEY`
- `VPS_KNOWN_HOSTS`
- `VPS_HOST`
- `VPS_USER`

The workflow preserves server-only `.env`, `docker-compose.yml`, and token data,
keeps a rollback image, rebuilds the service, and waits until `/health` reports
the expected version.

Recent commits on `main` can be newer than the latest tagged release. Check the
[release page](https://github.com/paolobietolini/gtm-mcp-server/releases) when you need a
reproducible published artifact.

## Development

```bash
go test ./... -count=1
go vet ./...
staticcheck ./...
go run ./cmd/tool-schema-report
go run ./cmd/tool-schema-report -groups tags,zones
```

The schema report prints the GTM tool count, serialized `tools/list` size, token
estimate, and largest definitions. The default GTM tool surface serializes to
78,734 bytes for 64 tools. A regression test enforces an 80,000-byte ceiling so
new parity work does not silently consume unlimited model context. The `all`
group exposes 94 GTM tools and serializes to 115,748 bytes.

Pull requests run `govulncheck`, `gosec`, Gitleaks, Trivy, `staticcheck`, and
CodeQL. Request-level GTM tests use local fake Google endpoints. Mutating live
tests must use a disposable container and clean up their entities.

See [ARCHITECTURE.md](ARCHITECTURE.md) for package boundaries, request flow,
authentication internals, token persistence, and security invariants.

For a deep dive into the Google Tag Manager MCP server, read the [Deep Wiki](https://deepwiki.com/paolobietolini/gtm-mcp-server).


## Current limitations

- The 101-method GTM v2 API parity target is complete. The five account
  user-permission methods remain outside the product scope.
- The server supports Streamable HTTP only. Stdio transport is planned but is
  not implemented.
- The optional connection dashboard is under review in
  [PR #107](https://github.com/paolobietolini/gtm-mcp-server/pull/107); it is not part of
  `main`.
- The hosted service processes OAuth tokens. Self-host the server when your
  policy requires control of the runtime and token store.
- Some GTM resource families depend on container type or account entitlement.

## Additional context

- [`llms.txt`](llms.txt) is served at
  [`https://mcp.gtmeditor.com/llms.txt`](https://mcp.gtmeditor.com/llms.txt).
- [`skills/gtm-mcp/gtm-mcp.md`](skills/gtm-mcp/gtm-mcp.md) contains a Claude
  Code skill with workflow guidance.
- [`examples/gtm_agent.py`](examples/gtm_agent.py) demonstrates a programmatic
  MCP client.
- [Google Tag Manager API v2 reference](https://developers.google.com/tag-platform/tag-manager/api/reference/rest)
- [Model Context Protocol](https://modelcontextprotocol.io/)

## License

BSD 3-Clause. See [LICENSE](LICENSE).

Maintained by [Paolo Bietolini](https://github.com/paolobietolini).
