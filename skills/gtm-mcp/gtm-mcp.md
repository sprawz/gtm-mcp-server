---
name: gtm-mcp
description: Guide for using the GTM MCP Server to manage Google Tag Manager through AI. Covers discovery, tag/trigger creation, auditing, publishing, and safety rules.
---

# GTM MCP Server Usage Guide

You are connected to the GTM MCP Server, which gives you full access to manage Google Tag Manager through the MCP protocol. This skill teaches you how to use it effectively.

## GTM Mental Model

**Hierarchy:** Account → Container → Workspace → Entities

- **Account**: organizational unit (usually one per company)
- **Container**: corresponds to a website or app (holds all tracking config)
- **Workspace**: a draft/staging area for changes (like a git branch)
- **Entities**: Tags, Triggers, Variables (plus Clients and Transformations for server-side containers)

**Critical rule**: all changes happen in a Workspace. Nothing goes live until you explicitly version and publish.

## Discovery First

Never assume IDs. Always discover them:

```
list_accounts → accountId
list_containers(accountId) → containerId
list_workspaces(accountId, containerId) → workspaceId
```

Most containers have a "Default Workspace" — use that unless the user specifies otherwise.

Cache these IDs for the session. You'll need them for every subsequent call.

## Before Creating Tags or Triggers

**Always call `get_tag_templates` and/or `get_trigger_templates` first.** The GTM API uses a non-obvious nested parameter format. These tools return the exact structure you need.

Key format rules:
- GA4 Config tag type = `gaawc`, GA4 Event tag type = `gaawe`
- `measurementId` must be an empty `tagReference`; the actual value goes in `measurementIdOverride`
- Event parameters: `list` → `map` → `template` with `name`/`value` keys
- Custom event triggers use `customEventFilterJson`, not `filterJson`
- Click/form triggers use `autoEventFilterJson`

## Creating Entities — Order Matters

1. **Variables first** — if tags/triggers reference custom variables
2. **Triggers second** — tags need trigger IDs to attach to
3. **Tags last** — reference the trigger IDs from step 2

When creating a tag, pass `firingTriggerId` as an array of trigger ID strings.

## Publishing Workflow

This is a three-step process. Never skip steps.

1. **Check status**: `get_workspace_status` — look for merge conflicts and review pending changes
2. **Create version**: `create_version` — snapshots the workspace into an immutable version
3. **Publish**: `publish_version` with `confirm: true` — pushes the version live

If `get_workspace_status` shows conflicts, resolve them before versioning.

## Configuration Standards

The server ships opinionated best-practice rules as readable resources. **Read the relevant one before creating or editing entities:**

| Resource | When to read |
|----------|-------------|
| `gtm://best-practices` | Index — start here |
| `gtm://best-practices/naming-organization` | Before creating/renaming any entity |
| `gtm://best-practices/safe-edit-workflow` | Before any edit session |
| `gtm://best-practices/ga4-consent` | When touching GA4 tags or consent setup |
| `gtm://best-practices/server-side` | When the container is server-side |

Core rules in brief:
- **Naming**: `<Platform> - <Type> - <Descriptor>` (e.g. `GA4 - Event - purchase`, `DLV - transaction_id`). Variables for every hardcoded ID.
- **Safe edits**: dedicated workspace per change → make changes → show `get_workspace_status` diff to the user → `create_version` with descriptive name → `publish_version` only after explicit approval.
- **GA4**: one config tag with the measurement ID from a lookup table variable (`LT - GA4 Measurement ID`) keyed on hostname/environment; event parameters from data layer variables; consent mode tags fire on Consent Initialization.
- **Existing conventions win**: if the container already follows a different consistent convention, match it and flag the difference instead of mixing conventions.

## Destructive Operations

These tools require `confirm: true` — the server will reject the call without it:
- `delete_tag`, `delete_trigger`, `delete_variable`
- `delete_container`, `delete_client`, `delete_transformation`, `delete_template`
- `disable_built_in_variables`
- `publish_version`

Always explain what you're about to delete/publish and get user confirmation before calling.

## Using Prompts

Six built-in prompts handle complex workflows:

| Prompt | When to use |
|--------|-------------|
| `audit_container` | User asks to review, audit, or check their container for issues |
| `best_practices_review` | User wants their config scored against best practices with concrete fixes |
| `plan_safe_edit` | User describes a change to make — produces a safe step-by-step execution plan |
| `generate_tracking_plan` | User needs documentation of their tracking setup |
| `suggest_ga4_setup` | User describes tracking goals and needs a recommendation |
| `find_gallery_template` | User wants to import a community template (Cookiebot, iubenda, etc.) |

Prompts need IDs — run discovery first.

## Common Task Patterns

### "Set up GA4 tracking for [goal]"
1. Discover IDs (list_accounts → list_containers → list_workspaces)
2. `get_tag_templates` for parameter format
3. `get_trigger_templates` for trigger format
4. Create triggers for each event
5. Create GA4 event tags referencing those triggers
6. Offer to version and publish

### "Audit my container"
1. Discover IDs
2. Use `audit_container` prompt — it fetches all data and structures the analysis request

### "Import [template name] from the gallery"
1. Use `find_gallery_template` prompt to locate the GitHub repo
2. Search for the template's GitHub URL
3. `import_gallery_template` with galleryOwner and galleryRepository

### "What's in my container?"
1. Discover IDs
2. `list_tags` + `list_triggers` + `list_variables` to survey
3. Or use `generate_tracking_plan` prompt for structured documentation

### "Publish the changes"
1. `get_workspace_status` — show the user what's pending
2. `create_version` with a descriptive name
3. `publish_version` with `confirm: true` — only after user agrees

## Anti-Patterns to Avoid

- **Don't guess parameter format** — always check templates first
- **Don't create tags before triggers** — you need the trigger ID
- **Don't publish without versioning** — create_version must come first
- **Don't skip get_workspace_status** — catch conflicts early
- **Don't assume IDs** — always discover via list operations
- **Don't delete without asking** — even with `confirm: true`, confirm with the user verbally first

## Server-Side Containers

Server-side containers (usage context = "server") have two additional entity types:
- **Clients**: receive incoming requests (e.g., GA4 client)
- **Transformations**: modify event data with allow/exclude/augment rules

Use `list_clients` / `list_transformations` to check if you're working with a server-side container.

## Known Limitation

`autoEventFilter` on click (`linkClick`) and form (`formSubmission`) triggers is silently dropped by the Google Tag Manager API. The API returns 200 but doesn't persist the filter. Tell users to set those conditions manually in the GTM web interface.
