# GTM MCP Server

[![License](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)
[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-Model_Context_Protocol-8A2BE2)](https://modelcontextprotocol.io)
[![Claude](https://img.shields.io/badge/Claude-Compatible-D97757?logo=anthropic&logoColor=white)](https://claude.ai)
[![ChatGPT](https://img.shields.io/badge/ChatGPT-Compatible-74aa9c?logo=openai&logoColor=white)](https://chatgpt.com)
[![Gemini](https://img.shields.io/badge/Gemini_CLI-Compatible-4285F4?logo=google&logoColor=white)](https://geminicli.com)
[![Cursor](https://img.shields.io/badge/Cursor-Compatible-00A67E?logo=cursor&logoColor=white)](https://cursor.com)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)](https://github.com/paolobietolini/gtm-mcp-server)
[![GitHub stars](https://img.shields.io/github/stars/paolobietolini/gtm-mcp-server?style=social)](https://github.com/paolobietolini/gtm-mcp-server)

**Let AI manage your Google Tag Manager containers.**

Create tags, audit configurations, generate tracking plans, and publish changes, all through natural conversation with Claude, ChatGPT, Gemini, Cursor, and more.

**URL:** `https://mcp.gtmeditor.com`

---

## Table of Contents

- [Supported AI Clients](#supported-ai-clients)
- [What Can You Do?](#what-can-you-do)
- [Quick Start](#quick-start)
- [Features](#features)
- [Use Cases](#use-cases)
- [How It Works](#how-it-works)
- [Safety Features](#safety-features)
- [Self-Hosting](#self-hosting)
- [Available Tools](#available-tools)
- [Resources & Prompts](#resources--prompts)
- [Better AI Context](#better-ai-context)
- [Architecture](#architecture)
- [Known Issues](#known-issues)
- [Links](#links)
- [Author](#author)
- [License](#license)

---

## Supported AI Clients

| Client | Transport | Auth Flow | Status |
|--------|-----------|-----------|--------|
| [Claude](https://claude.ai) (Web & Desktop) | Streamable HTTP | OAuth 2.1 + PKCE | Supported |
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (CLI) | Streamable HTTP | OAuth 2.1 + PKCE | Supported |
| [ChatGPT](https://chatgpt.com) | Streamable HTTP | OAuth 2.1 + PKCE | Supported |
| [Gemini CLI](https://github.com/google-gemini/gemini-cli) | Streamable HTTP | OAuth 2.1 + PKCE (DCR) | Supported |
| [Cursor](https://cursor.com) | Streamable HTTP | OAuth 2.1 + PKCE | Supported |

The server is **client-agnostic** — any MCP client that supports OAuth 2.1 with PKCE over HTTP transport should work out of the box, including clients that use Dynamic Client Registration (RFC 7591) and those that don't.

---

## What Can You Do?

Ask your AI assistant to:

- *"List all my GTM containers"*
- *"Create a GA4 event tag for form submissions"*
- *"Audit this container for issues and duplicates"*
- *"Generate a tracking plan document for the marketing team"*
- *"Set up ecommerce tracking for purchases"*
- *"Publish the changes we just made"*

No more clicking through the GTM interface. No more copy-pasting configurations. Just describe what you need.

---

## Quick Start

### Claude (Web & Desktop)

**Claude.ai:**
1. Go to **Settings** → **Connectors** → **Add Custom Connector**
2. Enter: `https://mcp.gtmeditor.com`
3. Click **Add** and sign in with Google

**Claude Code (CLI):**
```bash
claude mcp add -t http gtm https://mcp.gtmeditor.com
```

### ChatGPT

1. Go to [OpenAI Apps Platform](https://platform.openai.com/apps)
2. Add an MCP integration with URL: `https://mcp.gtmeditor.com`
3. Authorize with your Google account

### Gemini CLI

```bash
gemini mcp add --transport http --url https://mcp.gtmeditor.com gtm
```

### Cursor

1. Open **Settings** > **MCP**
2. Click **Add new MCP server**
3. Set type to **URL** and enter: `https://mcp.gtmeditor.com/authorize`
4. Authorize with your Google account

Or add to your `.cursor/mcp.json`:
```json
{
  "mcpServers": {
    "gtm": {
      "url": "https://mcp.gtmeditor.com/authorize"
    }
  }
}
```

---

## Features

### Tag Management
Create and modify any GTM tag type:
- **GA4 Configuration & Events** — Set up Google Analytics 4 with proper measurement IDs
- **Ecommerce Tracking** — Purchase, add-to-cart, view-item events
- **Custom HTML** — Inject scripts, pixels, and custom code
- **Custom Image** — Tracking pixels with cache busting

### Trigger Management
Build triggers for any scenario:
- Page views (all pages or specific URLs)
- Custom dataLayer events
- Click tracking
- Form submissions
- Timer-based triggers
- Trigger groups for complex conditions

### Container Operations
- Browse accounts, containers, and workspaces
- Create versions from workspace changes
- Publish versions to go live
- Organize with folders
- Enable/disable built-in variables

### Server-Side Containers
Full support for server-side GTM containers:
- **Clients** — Create, update, and delete server-side clients (e.g. GA4 client)
- **Transformations** — Control event parameters with allow, exclude, and augment rules

### Community Template Gallery
Import templates from Google's Community Template Gallery:
- *"Import the iubenda cookie consent template"*
- *"Add Cookiebot to my container"*
- *"Set up Facebook Pixel using the gallery template"*

The AI will search for the template, find the GitHub repository, and import it automatically.

### AI-Powered Workflows

**Container Audit**
*"Audit my container for issues"* — Analyzes your workspace for:
- Naming inconsistencies
- Duplicate tags
- Orphaned triggers
- Security concerns
- Best practice violations

**Tracking Plan Generation**
*"Generate a tracking plan"* — Creates markdown documentation of:
- All events and their triggers
- Data layer requirements
- Variable definitions
- Implementation notes

**GA4 Setup Recommendations**
*"Help me set up GA4 for ecommerce"* — Recommends:
- Which tags to create
- Trigger configurations
- Required variables
- Data layer implementation code

---

## Use Cases

### Build Complete Tracking Setups
Ask AI to create a full GA4 ecommerce implementation from scratch:
- *"Set up GA4 ecommerce tracking for my store"*
- Creates 12+ tags (configuration + all ecommerce events)
- Creates matching triggers for each dataLayer event
- Creates data layer variables for items, currency, value, transaction_id
- Follows Google's recommended event naming and parameters

### Implement Consent Management
Integrate privacy tools like OneTrust with your tracking:
- *"Make GA4 fire only when analytics consent is granted"*
- Creates consent-checking variables
- Sets up conditional triggers
- Updates existing tags to respect user choices

### Bulk Operations & Renaming
Manage containers at scale:
- *"Add 'ecom -' prefix to all ecommerce triggers"*
- *"Update all tags to use a measurement ID variable"*
- Rename, update, or organize dozens of items through conversation

### Custom Variables & Logic
Create sophisticated tracking logic:
- *"Create a variable that returns the local timestamp"*
- *"Add a custom parameter to the purchase tag"*
- Custom JavaScript variables, data layer mappings, and more

### For Agencies
- Manage multiple client containers (7+ accounts shown in demo)
- Standardize implementations across clients
- Rapid setup for new projects
- Version and publish changes safely

---

## How It Works

The GTM MCP Server connects AI assistants to the Google Tag Manager API using the [Model Context Protocol](https://modelcontextprotocol.io). When you ask Claude or ChatGPT to manage your GTM, it:

1. **Authenticates** with your Google account (OAuth 2.1)
2. **Reads** your container configurations
3. **Executes** the changes you request
4. **Confirms** before destructive operations

Your credentials are never stored—the server uses token-based authentication that you can revoke anytime from your Google account.

---

## Safety Features

- **Confirmation required** for deletions and publishing
- **Workspace-only changes** — nothing goes live until you publish
- **Version control** — all changes create a version first
- **Audit logging** — track what was changed

---

## Self-Hosting

Want to run your own instance?

### Service Account Mode (S2S)

Self-hosted deployments can use a Google Service Account so the whole team shares access — no individual GTM permissions needed.

**How it works:**
- The server authenticates to Google Tag Manager using a Service Account
- Team members connect with a shared API key — no personal GTM access required
- AI clients (Claude Code, ChatGPT, etc.) still do a one-time OAuth login *to the server*, but all GTM calls run under the Service Account
- Programmatic clients (scripts, CI/CD, APIs) skip OAuth entirely and use the API key directly

**Setup:**

1. Create a Service Account in [Google Cloud Console](https://console.cloud.google.com/) → IAM & Admin → Service Accounts
2. In [Google Tag Manager](https://tagmanager.google.com) → Account → Admin → User Management → add the Service Account email as **Account Administrator**
3. Download the JSON key file
4. Configure the server:

```bash
SERVICE_ACCOUNT_API_KEY=$(openssl rand -hex 32)   # share this with your team
GOOGLE_SERVICE_ACCOUNT_KEY_JSON=$(cat key.json)   # paste JSON content
go run main.go
```

On GCP (Cloud Run, GKE, Compute Engine): omit `GOOGLE_SERVICE_ACCOUNT_KEY_JSON` — Workload Identity is used automatically.

**Connecting Claude Code:**

Add the API key as a pre-configured header so Claude Code uses S2S automatically:

```json
{
  "mcpServers": {
    "gtm": {
      "type": "http",
      "url": "http://your-server:8080",
      "headers": {
        "Authorization": "Bearer your-api-key"
      }
    }
  }
}
```

**Programmatic / API access:**

Any HTTP client can call the server directly — no browser, no OAuth:

```bash
curl -H "Authorization: Bearer your-api-key" \
     -H "Content-Type: application/json" \
     http://your-server:8080/mcp \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

See [`examples/gtm_agent.py`](examples/gtm_agent.py) for a complete Python agent that uses Claude to manage GTM programmatically via the API key.

---

### Docker Setup

```bash
git clone https://github.com/paolobietolini/gtm-mcp-server.git
cd gtm-mcp-server

# Create .env file
cat > .env << 'EOF'
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
JWT_SECRET=$(openssl rand -base64 32)
BASE_URL=http://localhost:8080
EOF

# Start the server
docker compose up -d

# Add to Claude
claude mcp add -t http gtm http://localhost:8080
```

#### Docker-to-Docker

If another container needs to reach the MCP server via an internal Docker network alias, add `ALLOWED_HOSTS` to your `.env`:

```bash
ALLOWED_HOSTS=gtm-mcp:8080
```

This enables dynamic URL resolution for trusted internal hostnames while keeping the server secure against host header injection.

### Google Cloud Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Enable the **Tag Manager API**
3. Create **OAuth 2.0 credentials** (Web application)
4. Add redirect URIs:
   ```
   https://claude.ai/api/mcp/auth_callback
   https://claude.com/api/mcp/auth_callback
   https://chatgpt.com/connector_platform_oauth_redirect
   https://your-domain.com/oauth/callback
   ```

---

## Available Tools

### Read Operations
| Tool | Description |
|------|-------------|
| `list_accounts` | List all GTM accounts |
| `list_containers` | List containers in an account |
| `list_workspaces` | List workspaces in a container |
| `list_tags` | List all tags in a workspace |
| `get_tag` | Get tag details by ID |
| `list_triggers` | List all triggers |
| `get_trigger` | Get trigger details by ID |
| `list_variables` | List all variables |
| `get_variable` | Get variable details by ID |
| `list_folders` | List folders in a workspace |
| `get_folder_entities` | Get tags/triggers/variables in a folder |
| `list_built_in_variables` | List enabled built-in variables in a workspace |

### Utility
| Tool | Description |
|------|-------------|
| `ping` | Test server connectivity |
| `auth_status` | Check authentication status |

### Write Operations
| Tool | Description |
|------|-------------|
| `update_account` | Rename a GTM account |
| `create_container` | Create a new container in an account |
| `update_container` | Rename a container (preserves usage context, domain, notes) |
| `delete_container` | Remove a container (requires confirmation) |
| `create_workspace` | Create a new workspace in a container |
| `create_tag` | Create a new tag |
| `update_tag` | Modify an existing tag |
| `delete_tag` | Remove a tag (requires confirmation) |
| `create_trigger` | Create a new trigger |
| `update_trigger` | Modify an existing trigger |
| `delete_trigger` | Remove a trigger (requires confirmation) |
| `create_variable` | Create a new variable |
| `update_variable` | Modify an existing variable |
| `delete_variable` | Remove a variable (requires confirmation) |
| `enable_built_in_variables` | Enable built-in variable types in a workspace |
| `disable_built_in_variables` | Disable built-in variable types (requires confirmation) |

### Server-Side Container Tools
| Tool | Description |
|------|-------------|
| `list_clients` | List all clients in a workspace |
| `get_client` | Get client details by ID |
| `create_client` | Create a new client |
| `update_client` | Modify an existing client |
| `delete_client` | Remove a client (requires confirmation) |
| `list_transformations` | List all transformations in a workspace |
| `get_transformation` | Get transformation details by ID |
| `create_transformation` | Create a new transformation |
| `update_transformation` | Modify an existing transformation |
| `delete_transformation` | Remove a transformation (requires confirmation) |

### Publishing
| Tool | Description |
|------|-------------|
| `get_workspace_status` | Check pending changes and merge conflicts before versioning |
| `list_versions` | List all container versions with tag/trigger/variable counts |
| `create_version` | Create a version from workspace changes |
| `publish_version` | Publish a version (requires confirmation) |

### Templates
| Tool | Description |
|------|-------------|
| `get_tag_templates` | Get GA4/HTML tag parameter examples |
| `get_trigger_templates` | Get trigger configuration examples |
| `list_templates` | List custom templates in a workspace |
| `get_template` | Get template details including template code |
| `create_template` | Create a custom template from .tpl code |
| `update_template` | Modify an existing template |
| `delete_template` | Remove a template (requires confirmation) |
| `import_gallery_template` | Import a template from the Community Gallery |

---

## Resources & Prompts

### Resources (URI-based access)
Access GTM data via structured URIs:
```
gtm://accounts
gtm://accounts/{id}/containers
gtm://accounts/{id}/containers/{id}/workspaces
gtm://accounts/.../workspaces/{id}/tags
gtm://accounts/.../workspaces/{id}/triggers
gtm://accounts/.../workspaces/{id}/variables
```

### Prompts (Workflow templates)
| Prompt | Description |
|--------|-------------|
| `audit_container` | Comprehensive container analysis |
| `generate_tracking_plan` | Markdown documentation generator |
| `suggest_ga4_setup` | GA4 implementation recommendations |
| `find_gallery_template` | Guide to find and import Community Gallery templates |

---

## Better AI Context

For best results, install the **GTM API skill** so your AI assistant understands GTM's API structure, parameter formats, and validation rules.

### Claude Code

```bash
# One-liner install
curl -sL https://github.com/paolobietolini/gtm-api-for-llms/archive/main.tar.gz | tar xz && \
  mkdir -p ~/.claude/skills && \
  cp -r gtm-api-for-llms-main/skills/gtm-api ~/.claude/skills/ && \
  rm -rf gtm-api-for-llms-main
```

Or clone and copy:
```bash
git clone https://github.com/paolobietolini/gtm-api-for-llms.git
cp -r gtm-api-for-llms/skills/gtm-api ~/.claude/skills/
```

### OpenAI Codex

```bash
curl -sL https://github.com/paolobietolini/gtm-api-for-llms/archive/main.tar.gz | tar xz && \
  mkdir -p ~/.codex/skills && \
  cp -r gtm-api-for-llms-main/skills/gtm-api ~/.codex/skills/ && \
  rm -rf gtm-api-for-llms-main
```

### What does the skill include?

The [GTM API for LLMs](https://github.com/paolobietolini/gtm-api-for-llms) repository provides LLM-optimized documentation: request templates, validation rules, workflow algorithms, and complete schemas for all GTM entity types including server-side containers.

---

## Architecture

- **Protocol:** Model Context Protocol (MCP) over HTTP
- **Authentication:** OAuth 2.1 with PKCE
- **Standards:** RFC 8414, RFC 7591, RFC 9728

---

## Known Issues
### 🐛 `autoEventFilter` silently dropped by Google Tag Manager API

When creating or updating `linkClick`, `click`, or `formSubmission` triggers via the API, the `autoEventFilter` field (used for "Some Link Clicks"/"Some Form Submissions" conditions) is silently dropped by the Google Tag Manager API. The API returns `200 OK` with a new fingerprint but does not persist the `autoEventFilter`.

This has been confirmed by HTTP-level debugging: the correct JSON is sent in the request body, but Google's response omits the field. The `filter` and `customEventFilter` fields work correctly.

**Workaround:** Configure `autoEventFilter` conditions manually through the [GTM web interface](https://tagmanager.google.com). The MCP server can read triggers that have `autoEventFilter` set via the UI.

**Status:** [#33](https://github.com/paolobietolini/gtm-mcp-server/issues/33)

---

## Links

- [GitHub Repository](https://github.com/paolobietolini/gtm-mcp-server)
- [GTM API Reference](https://github.com/paolobietolini/gtm-api-for-llms)
- [MCP Specification](https://modelcontextprotocol.io)

---

## Author

**Paolo Bietolini**

mcp@paolobietolini.com

---

## License

[BSD-3-Clause](LICENSE)
