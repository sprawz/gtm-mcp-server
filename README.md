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

**An AI assistant can control your Google Tag Manager containers.**

The server connects an AI assistant to the Google Tag Manager API. You give
instructions in usual language. The assistant creates tags, examines
configurations, writes tracking plans, and publishes changes.

**URL:** `https://mcp.gtmeditor.com`

---

## Table of Contents

- [Supported AI Clients](#supported-ai-clients)
- [What You Can Do](#what-you-can-do)
- [Quick Start](#quick-start)
  - [Claude (Web and Desktop)](#claude-web-and-desktop)
  - [ChatGPT](#chatgpt)
  - [Gemini CLI](#gemini-cli)
  - [Cursor](#cursor)
- [Functions](#functions)
  - [Tag Control](#tag-control)
  - [Trigger Control](#trigger-control)
  - [Container Operations](#container-operations)
  - [Server-Side Containers](#server-side-containers)
  - [Community Template Gallery](#community-template-gallery)
  - [AI Workflows](#ai-workflows)
- [Examples of Use](#examples-of-use)
- [How the Server Operates](#how-the-server-operates)
- [Safety Functions](#safety-functions)
- [Comparison with Other GTM MCP Servers](#comparison-with-other-gtm-mcp-servers)
- [Self-Hosting](#self-hosting)
  - [Service Account Mode (S2S)](#service-account-mode-s2s)
  - [Docker Setup](#docker-setup)
  - [Google Cloud Setup](#google-cloud-setup)
- [Available Tools](#available-tools)
- [Resources and Prompts](#resources-and-prompts)
- [More Context for AI Assistants](#more-context-for-ai-assistants)
- [Architecture](#architecture)
- [Known Problems](#known-problems)
- [Links](#links)
- [Author](#author)
- [License](#license)

---

## Supported AI Clients

| Client | Transport | Authentication | Status |
|--------|-----------|----------------|--------|
| [Claude](https://claude.ai) (Web and Desktop) | Streamable HTTP | OAuth 2.1 and PKCE | Supported |
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (CLI) | Streamable HTTP | OAuth 2.1 and PKCE | Supported |
| [ChatGPT](https://chatgpt.com) | Streamable HTTP | OAuth 2.1 and PKCE | Supported |
| [Gemini CLI](https://github.com/google-gemini/gemini-cli) | Streamable HTTP | OAuth 2.1, PKCE and DCR | Supported |
| [Cursor](https://cursor.com) | Streamable HTTP | OAuth 2.1 and PKCE | Supported |

The server does not require a specified client. Each MCP client that has OAuth
2.1 with PKCE on an HTTP transport can connect to the server. This includes
clients with Dynamic Client Registration (RFC 7591) and clients without it.

---

## What You Can Do

Give these instructions to your AI assistant:

- *"List all my GTM containers"*
- *"Create a GA4 event tag for form submissions"*
- *"Examine this container for problems and duplicates"*
- *"Write a tracking plan document for the marketing team"*
- *"Set up ecommerce tracking for purchases"*
- *"Publish the changes"*

You do not use the GTM interface for these tasks. You do not copy
configurations manually. You give the instruction in usual language.

---

## Quick Start

### Claude (Web and Desktop)

For Claude.ai:

1. Select **Settings**, then **Connectors**, then **Add Custom Connector**.
2. Type this URL: `https://mcp.gtmeditor.com`
3. Click **Add**.
4. Sign in with your Google account.

For Claude Code (CLI), use this command:

```bash
claude mcp add -t http gtm https://mcp.gtmeditor.com
```

### ChatGPT

1. Go to the [OpenAI Apps Platform](https://platform.openai.com/apps).
2. Add an MCP integration with this URL: `https://mcp.gtmeditor.com`
3. Give permission with your Google account.

### Gemini CLI

```bash
gemini mcp add --transport http --url https://mcp.gtmeditor.com gtm
```

### Cursor

1. Open **Settings**, then **MCP**.
2. Click **Add new MCP server**.
3. Set the type to **URL**.
4. Type this URL: `https://mcp.gtmeditor.com/authorize`
5. Give permission with your Google account.

As an alternative, add this configuration to your `.cursor/mcp.json` file:

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

## Functions

### Tag Control

The server creates and changes all GTM tag types:

- **GA4 configuration and events.** Set up Google Analytics 4 with the correct
  measurement IDs.
- **Ecommerce tracking.** Use purchase, add-to-cart, and view-item events.
- **Custom HTML.** Add scripts, pixels, and custom code.
- **Custom image.** Add tracking pixels with cache prevention.

### Trigger Control

The server creates triggers for these conditions:

- Page views on all pages or on specified URLs
- Custom dataLayer events
- Clicks
- Form submissions
- Timers
- Trigger groups for complex conditions

### Container Operations

- Read accounts, containers, and workspaces.
- Create a version from the changes in a workspace.
- Publish a version to make it live.
- Put items in folders.
- Enable or disable the built-in variables.

### Server-Side Containers

The server has full support for server-side GTM containers:

- **Clients.** Create, change, and delete server-side clients. The GA4 client
  is an example.
- **Transformations.** Control event parameters with allow, exclude, and
  augment rules.

### Community Template Gallery

The server imports templates from the Google Community Template Gallery. Give
instructions such as these:

- *"Import the iubenda cookie consent template"*
- *"Add Cookiebot to my container"*
- *"Set up Facebook Pixel with the gallery template"*

The AI assistant finds the template and its GitHub repository. Then the
assistant imports the template automatically.

### AI Workflows

**Container examination.** Give the instruction *"Examine my container for
problems"*. The assistant examines the workspace for these conditions:

- Names that do not agree
- Tags that occur more than once
- Triggers that no tag uses
- Risks to security
- Configurations that do not obey the best practices

**Tracking plan.** Give the instruction *"Write a tracking plan"*. The
assistant writes a markdown document with this content:

- All events and their triggers
- The necessary dataLayer values
- The definitions of the variables
- Notes about the implementation

**GA4 recommendations.** Give the instruction *"Help me set up GA4 for
ecommerce"*. The assistant recommends the tags, the triggers, the necessary
variables, and the dataLayer code.

---

## Examples of Use

### Make a Complete Tracking Setup

The assistant can make a full GA4 ecommerce implementation. Give the
instruction *"Set up GA4 ecommerce tracking for my store"*. The assistant then
does these steps:

1. It creates 12 tags or more. These include the configuration tag and all the
   ecommerce event tags.
2. It creates one trigger for each dataLayer event.
3. It creates dataLayer variables for the items, the currency, the value, and
   the transaction ID.
4. It obeys the Google recommendations for event names and parameters.

### Add Consent Control

You can connect privacy tools such as OneTrust to your tracking. Give the
instruction *"Make GA4 fire only when the user gives consent for analytics"*.
The assistant then does these steps:

1. It creates the variables that read the consent.
2. It creates the related triggers.
3. It changes the applicable tags.

### Change Many Items

You can control containers that have many items. Give instructions such as
these:

- *"Add the prefix 'ecom -' to all ecommerce triggers"*
- *"Change all tags to use a measurement ID variable"*

The assistant can change or move many items in one operation.

### Make Custom Variables

You can make complex tracking logic. Give instructions such as these:

- *"Create a variable that gives the local time"*
- *"Add a custom parameter to the purchase tag"*

The assistant makes custom JavaScript variables and dataLayer variables.

### Use by an Agency

- Control the containers of more than one client.
- Use the same implementation for all clients.
- Set up a new project quickly.
- Make a version and publish the changes safely.

---

## How the Server Operates

The server connects AI assistants to the Google Tag Manager API. It uses the
[Model Context Protocol](https://modelcontextprotocol.io). When you give an
instruction, the server does these steps:

1. It authenticates you with your Google account through OAuth 2.1.
2. It reads the configuration of your container.
3. It makes the changes that you request.
4. It asks for your approval before a destructive operation.

The server does not keep your Google password. The server uses tokens. You can
cancel a token at any time from your Google account.

---

## Safety Functions

- The server asks for approval before a deletion or a publication.
- The server changes only the workspace. No change is live until you publish
  it.
- The server makes a version before each publication.
- The server writes a log of the changes.

---

## Comparison with Other GTM MCP Servers

More than one MCP server for Google Tag Manager is available. The most usual
alternative is
[stape-io/google-tag-manager-mcp-server](https://github.com/stape-io/google-tag-manager-mcp-server).
The two projects are different in their design. This section gives the facts.
It does not say that one project is better than the other. Use the facts to
select the server that agrees with your conditions.

The data is correct on 21 August 2026.

### Design and Distribution

| Item | This server | stape-io |
|------|-------------|----------|
| Language | Go | TypeScript |
| Distribution | One binary, or a Docker image | npm packages, and a Cloudflare Worker |
| Hosted server | `mcp.gtmeditor.com` | `gtm-mcp.stape.ai` |
| Local operation | Self-hosted HTTP server | `npx` CLI on stdio, or a self-hosted worker |
| Transports | Streamable HTTP | Streamable HTTP, and stdio |
| License | BSD-3-Clause | Apache-2.0 |

A local CLI on stdio keeps the credentials on your computer. This server does
not have a stdio mode. If you must not send credentials to a server, the
stape-io CLI obeys that condition and this server does not.

### Tool Design

| Item | This server | stape-io |
|------|-------------|----------|
| Number of tools | 50 | 18 |
| Tool design | One tool for each operation | One tool for each resource, with an `action` parameter |
| Example | `create_tag`, `list_tags`, `delete_tag` | `gtm_tag` with `action: "create"` |

Each design has an effect. 50 tools use more of the tool budget of the model.
Some clients also have a limit on the length of the server name plus the tool
name. But each tool has a schema for one operation only, and the parameters
that this operation needs are mandatory.

18 tools use less of the tool budget. But one schema must serve six
operations. Thus almost all parameters are optional, and the server examines
them when it receives the request.

### MCP Functions

| Item | This server | stape-io |
|------|-------------|----------|
| Tools | Yes | Yes |
| Resources | 16 | 0 |
| Prompts | 6 | 0 |
| Best-practice documents | 4, supplied as resources | 0 |

This server supplies GTM rules as MCP resources at `gtm://best-practices`. An
AI assistant can read these rules before it makes a change. The topics are
names and organization, the safe-edit workflow, GA4 and consent, and
server-side containers. The stape-io server supplies tools only.

### Authentication

| Item | This server | stape-io |
|------|-------------|----------|
| Authentication to the server | OAuth 2.1 authorization server in the project | Google OAuth on the hosted worker |
| Dynamic Client Registration (RFC 7591) | Yes | Not supplied |
| Client ID Metadata Documents | Yes | Not supplied |
| Protected resource metadata (RFC 9728) | Yes | Not supplied |
| Token persistence after a restart | Yes, with `TOKEN_STORE_PATH` | Not applicable to the CLI |
| Machine-to-machine access | Yes, with an API key and a service account | Yes, with a service account key in the CLI |
| Credentials for the local mode | Not applicable | Service account key, or refresh token |

### Coverage of the GTM API

The stape-io server has more of the GTM API. It has these resources, and this
server does not have them:

- Destinations
- Environments
- Google tag configurations (`gtag_config`)
- User permissions
- Zones
- Version headers

The stape-io server also has these operations, and this server does not have
them: `container.combine`, `container.lookup`, `container.snippet`,
`version.live`, `version.undelete`, `workspace.sync`, `workspace.quickPreview`,
`workspace.resolveConflict`, and `revert` on the applicable resources.

This server has these operations, and the stape-io server does not have them:

- Import of a template from the Community Template Gallery
- Parameter examples for tags and triggers (`get_tag_templates` and
  `get_trigger_templates`)

Both servers have accounts, containers, workspaces, tags, triggers, variables,
built-in variables, folders, clients, transformations, templates, versions, and
the status of a workspace.

### Summary of the Differences

- If you need the maximum coverage of the GTM API, or a local stdio mode,
  examine the stape-io server.
- If you need MCP resources and prompts, built-in best-practice documents, or
  an OAuth authorization server with Dynamic Client Registration, examine this
  server.

---

## Self-Hosting

You can operate your own instance of the server.

### Service Account Mode (S2S)

A self-hosted server can use a Google Service Account. Then all the members of
your team have access through that one account. The members do not need their
own GTM permissions.

**Operation:**

- The server authenticates to Google Tag Manager with the Service Account.
- The members of the team connect with an API key that they share.
- The AI clients do one OAuth sign-in to the server. All GTM operations then
  use the Service Account.
- Programs, scripts, and CI/CD systems do not use OAuth. They use the API key.

**Setup:**

1. Go to the [Google Cloud Console](https://console.cloud.google.com/). Select
   **IAM and Admin**, then **Service Accounts**. Create a Service Account.
2. Go to [Google Tag Manager](https://tagmanager.google.com). Select
   **Account**, then **Admin**, then **User Management**. Add the email address
   of the Service Account as an **Account Administrator**.
3. Download the JSON key file.
4. Configure the server:

```bash
SERVICE_ACCOUNT_API_KEY=$(openssl rand -hex 32)   # give this key to your team
GOOGLE_SERVICE_ACCOUNT_KEY_JSON=$(cat key.json)   # the content of the JSON file
go run main.go
```

On Google Cloud Run, GKE, or Compute Engine, do not set
`GOOGLE_SERVICE_ACCOUNT_KEY_JSON`. The server uses Workload Identity
automatically.

**Connection from Claude Code:**

Set the API key as a header. Claude Code then uses the S2S mode automatically.

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

**Access from a program:**

An HTTP client can send requests to the server directly. A browser and an OAuth
flow are not necessary.

```bash
curl -H "Authorization: Bearer your-api-key" \
     -H "Content-Type: application/json" \
     http://your-server:8080/mcp \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

The file [`examples/gtm_agent.py`](examples/gtm_agent.py) contains a complete
Python agent. The agent uses Claude and the API key to control GTM.

---

### Docker Setup

```bash
git clone https://github.com/paolobietolini/gtm-mcp-server.git
cd gtm-mcp-server

# Make the .env file
cat > .env << 'EOF'
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
JWT_SECRET=$(openssl rand -base64 32)
BASE_URL=http://localhost:8080
EOF

# Start the server
docker compose up -d

# Add the server to Claude
claude mcp add -t http gtm http://localhost:8080
```

#### Reverse Proxy (TRUST_PROXY)

A reverse proxy can be in front of the server. Caddy, nginx, and Cloudflare are
examples. Set `TRUST_PROXY=true` for this condition. The rate limiter then uses
the real IP address of the client from the `X-Forwarded-For` header. If you do
not set this variable, the rate limiter uses the address of the proxy.

The rate limiter reads the **last** entry in the header. A proxy adds the
address of its peer to the end of the header. Thus the last entry is the entry
that your own proxy wrote. The entries before it come from the client, and a
client can give false values for them. If the header occurs more than one time,
the server reads the last entry of the last occurrence.

This operation assumes one proxy between the client and the server. If you use
two proxies, the last entry is the address of the proxy that is nearest to the
client, not the address of the client. All clients behind that proxy then share
one limit.

```bash
TRUST_PROXY=true
```

The Docker Compose configuration sets this variable automatically, because the
container operates behind Caddy.

**Caution:** If you start the binary without a proxy, do not set this variable,
or set it to `false`. If you set it to `true` without a proxy, a client can
give a false IP address. The client can then get more requests than the rate
limit permits.

#### Docker-to-Docker Connections

A different container can connect to the MCP server with an internal Docker
network name. Add the `ALLOWED_HOSTS` variable to your `.env` file for this
condition:

```bash
ALLOWED_HOSTS=gtm-mcp:8080
```

The server then finds its URL dynamically, but only for the internal host names
that you list. Other host names cannot change the URL. Thus an attacker cannot
use the Host header to change the URL.

#### Token Persistence (TOKEN_STORE_PATH)

The server keeps the tokens in memory only, if you do not set
`TOKEN_STORE_PATH`. Thus each restart of the server disconnects all users. Each
user must then authenticate again.

To keep the sessions after a restart, set `TOKEN_STORE_PATH` to a file on a
persistent volume:

```bash
TOKEN_STORE_PATH=/data/tokens.json
```

The server writes the file with the permissions `0600`. The file contains
refresh tokens. Thus you must keep the volume secret.

The directory must be writable by the user that operates the server. The Docker
image operates as the user `appuser`. The image supplies the directory `/data`
with the owner `appuser` and the mode `0700`. If the server makes the directory
itself, the directory gets the same mode.

**Caution:** The token store fails closed. If the server cannot make or read
the file, the server stops at start-up. The server does not discard the
sessions without a message.

#### Bearer Renewal Cap (AUTH_AUTO_REFRESH_MAX_AGE)

A client can send a bearer token that is expired. The server then refreshes the
Google token and extends the same bearer token. The session continues, and the
user does not sign in again.

The server does not replace the bearer token in this operation. Thus the expiry
time of the bearer token is not a limit. An expired bearer token is sufficient
to get a new period, while the entry is in the store.

`AUTH_AUTO_REFRESH_MAX_AGE` gives a limit to this sequence. The limit is the
total age of the token. The default value is `168h`, which is 7 days.

```env
AUTH_AUTO_REFRESH_MAX_AGE=168h
```

After the limit, the server sends the response `401` and the `WWW-Authenticate`
header of RFC 9728. The server does not extend the token. The client must then
use the standard OAuth refresh grant. That grant replaces the two credentials
and makes a new entry with a new age. Thus a correct client continues without a
sign-in, and only the sequence of an unauthorized token stops.

A client without the refresh grant must sign in again one time in each limit
period. Without this limit, the client signs in again one time in each
`ACCESS_TOKEN_TTL` period.

To disable the limit, set the value to `0`. The renewal sequence then has no
limit.

The server writes the log event `auth_auto_refresh_capped` for each refusal.
The event contains the client ID and the age of the refused sequence.

### Google Cloud Setup

1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. Enable the **Tag Manager API**.
3. Create **OAuth 2.0 credentials** for a web application.
4. Add these redirect URIs:
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
| `list_accounts` | Gives all the GTM accounts |
| `list_containers` | Gives the containers in an account |
| `list_workspaces` | Gives the workspaces in a container |
| `list_tags` | Gives all the tags in a workspace |
| `get_tag` | Gives the data of one tag |
| `list_triggers` | Gives all the triggers |
| `get_trigger` | Gives the data of one trigger |
| `list_variables` | Gives all the variables |
| `get_variable` | Gives the data of one variable |
| `list_folders` | Gives the folders in a workspace |
| `get_folder_entities` | Gives the tags, triggers, and variables in a folder |
| `list_built_in_variables` | Gives the enabled built-in variables in a workspace |

### Utility

| Tool | Description |
|------|-------------|
| `ping` | Tests the connection to the server |
| `auth_status` | Gives the status of the authentication |

### Write Operations

| Tool | Description |
|------|-------------|
| `update_account` | Changes the name of a GTM account |
| `create_container` | Creates a container in an account |
| `update_container` | Changes the name of a container. Keeps the usage context, the domain, and the notes |
| `delete_container` | Deletes a container. Asks for approval |
| `create_workspace` | Creates a workspace in a container |
| `create_tag` | Creates a tag |
| `update_tag` | Changes a tag |
| `delete_tag` | Deletes a tag. Asks for approval |
| `create_trigger` | Creates a trigger |
| `update_trigger` | Changes a trigger |
| `delete_trigger` | Deletes a trigger. Asks for approval |
| `create_variable` | Creates a variable |
| `update_variable` | Changes a variable |
| `delete_variable` | Deletes a variable. Asks for approval |
| `enable_built_in_variables` | Enables built-in variable types in a workspace |
| `disable_built_in_variables` | Disables built-in variable types. Asks for approval |

### Server-Side Container Tools

| Tool | Description |
|------|-------------|
| `list_clients` | Gives all the clients in a workspace |
| `get_client` | Gives the data of one client |
| `create_client` | Creates a client |
| `update_client` | Changes a client |
| `delete_client` | Deletes a client. Asks for approval |
| `list_transformations` | Gives all the transformations in a workspace |
| `get_transformation` | Gives the data of one transformation |
| `create_transformation` | Creates a transformation |
| `update_transformation` | Changes a transformation |
| `delete_transformation` | Deletes a transformation. Asks for approval |

### Publication

| Tool | Description |
|------|-------------|
| `get_workspace_status` | Gives the changes and the merge conflicts before a version |
| `list_versions` | Gives all the container versions with the counts of the items |
| `create_version` | Creates a version from the changes in a workspace |
| `publish_version` | Publishes a version. Asks for approval |

### Templates

| Tool | Description |
|------|-------------|
| `get_tag_templates` | Gives parameter examples for GA4 tags and HTML tags |
| `get_trigger_templates` | Gives configuration examples for triggers |
| `list_templates` | Gives the custom templates in a workspace |
| `get_template` | Gives the data of one template with its code |
| `create_template` | Creates a custom template from `.tpl` code |
| `update_template` | Changes a custom template |
| `delete_template` | Deletes a custom template. Asks for approval |
| `import_gallery_template` | Imports a template from the Community Gallery |

---

## Resources and Prompts

### Resources

The server gives access to GTM data through these URIs:

```
gtm://accounts
gtm://accounts/{id}/containers
gtm://accounts/{id}/containers/{id}/workspaces
gtm://accounts/.../workspaces/{id}/tags
gtm://accounts/.../workspaces/{id}/triggers
gtm://accounts/.../workspaces/{id}/variables
```

The server also gives best-practice documents. These documents are markdown
text. Authentication is not necessary to read them.

```
gtm://best-practices                        # The index of all the documents
gtm://best-practices/naming-organization    # Names, folders, and unused items
gtm://best-practices/safe-edit-workflow     # Workspace, difference, version, publication
gtm://best-practices/ga4-consent            # GA4 patterns and consent mode v2
gtm://best-practices/server-side            # Clients, transformations, PII, first-party domains
```

### Prompts

| Prompt | Description |
|--------|-------------|
| `audit_container` | Examines a container against the best practices |
| `best_practices_review` | Gives a result of pass, warning, or failure for each category, with the corrections |
| `plan_safe_edit` | Gives the steps for a change that obeys the safe-edit workflow |
| `generate_tracking_plan` | Writes a tracking plan in markdown |
| `suggest_ga4_setup` | Gives recommendations for a GA4 implementation |
| `find_gallery_template` | Gives the steps to find and import a Community Gallery template |

---

## More Context for AI Assistants

The server gives two resources that help an AI assistant. One resource is for
each LLM or agent. The other resource is for Claude Code users. The server also
gives GTM rules as MCP resources at `gtm://best-practices`. Each connected
agent can read these rules before it makes a change.

### llms.txt for Each LLM or Agent

The server supplies an [`llms.txt`](https://mcp.gtmeditor.com/llms.txt) file.
Each LLM or agent can read this file to get context. The file contains the GTM
hierarchy, all the tools, the usual workflows, the safety rules, and the format
of the GA4 parameters.

```
https://mcp.gtmeditor.com/llms.txt
```

The file obeys the [llms.txt](https://llmstxt.org/) standard. Agent systems
with support for llms.txt read this file automatically. You can also read the
file manually, or put it in a system prompt for your own integration.

### Claude Code Skill

Claude Code users can install the **GTM MCP skill**. The skill gives workflows,
the patterns to obey, and the patterns to prevent.

Use this command to install the skill:

```bash
curl -sL https://github.com/paolobietolini/gtm-mcp-server/archive/main.tar.gz | tar xz && \
  mkdir -p ~/.claude/skills && \
  cp -r gtm-mcp-server-main/skills/gtm-mcp ~/.claude/skills/ && \
  rm -rf gtm-mcp-server-main
```

As an alternative, clone the repository and copy the directory:

```bash
git clone https://github.com/paolobietolini/gtm-mcp-server.git
cp -r gtm-mcp-server/skills/gtm-mcp ~/.claude/skills/
```

The skill teaches Claude to find the IDs and to create tags with the correct
parameters. It also teaches Claude to obey the publication workflow and to
prevent the usual errors.

### GTM API Skill

The **GTM API skill** gives more context about the API. It contains the
parameter schemas, the validation rules, and request templates for all the
entity types.

For Claude Code:

```bash
curl -sL https://github.com/paolobietolini/gtm-api-for-llms/archive/main.tar.gz | tar xz && \
  mkdir -p ~/.claude/skills && \
  cp -r gtm-api-for-llms-main/skills/gtm-api ~/.claude/skills/ && \
  rm -rf gtm-api-for-llms-main
```

For OpenAI Codex:

```bash
curl -sL https://github.com/paolobietolini/gtm-api-for-llms/archive/main.tar.gz | tar xz && \
  mkdir -p ~/.codex/skills && \
  cp -r gtm-api-for-llms-main/skills/gtm-api ~/.codex/skills/ && \
  rm -rf gtm-api-for-llms-main
```

The [GTM API for LLMs](https://github.com/paolobietolini/gtm-api-for-llms)
repository contains documentation for LLMs. It has request templates,
validation rules, workflow algorithms, and the full schemas of all the GTM
entity types. The server-side container types are included.

---

## Architecture

- **Protocol:** Model Context Protocol (MCP) on HTTP
- **Authentication:** OAuth 2.1 with PKCE
- **Standards:** RFC 8414, RFC 7591, RFC 9728

---

## Known Problems

### The GTM API removes `autoEventFilter`

The `autoEventFilter` field sets the conditions for "Some Link Clicks" and
"Some Form Submissions". The Google Tag Manager API removes this field without
a message. This occurs when you create or change a `linkClick`, `click`, or
`formSubmission` trigger through the API. The API sends the response `200 OK`
with a new fingerprint, but it does not keep the `autoEventFilter` field.

Tests at the HTTP level show this behavior. The request contains the correct
JSON, but the response of Google does not contain the field. The `filter` field
and the `customEventFilter` field operate correctly.

**Alternative procedure:** Set the `autoEventFilter` conditions manually in the
[GTM web interface](https://tagmanager.google.com). The MCP server can read a
trigger that has an `autoEventFilter` field from the interface.

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
