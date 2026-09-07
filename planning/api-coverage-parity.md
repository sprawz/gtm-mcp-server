# Plan: close the GTM API coverage gap

Status: **not implemented**. This document is the TODO list for it.

Gap measured against `stape-io/google-tag-manager-mcp-server` on 21 August
2026, by reading its `packages/core/src/tools/*.ts` action enums directly.

## Where the two servers stand

This server exposes **50 tools**, one for each operation. stape-io exposes
**18 tools**, one for each resource, each with an `action` enum. The tool count
is not the coverage: 18 multiplexed tools cover more GTM API surface than 50
single-purpose ones.

## Decision: keep one tool per operation

Confirmed 21 August 2026. The new tools follow the existing convention
(`create_zone`, `list_zones`, …), not stape-io's action-enum shape.

The cost is real and should be stated. This plan adds roughly 30 tools, taking
the server past 80. That consumes more of the model's tool budget, and some
clients cap the combined length of the server name plus the tool name — Cursor
at 60 characters, which stape-io documents in its own README. The benefit is
that each schema describes one operation, so parameters that an operation
requires can be mandatory rather than optional-and-checked-at-runtime, and the
server stays internally consistent.

If the tool count becomes a practical problem, the fallback is to gate whole
resource groups behind configuration rather than to multiplex tools, so the
schemas stay honest.

## Missing resources

Each needs a `gtm/tool_<name>.go`, registration in `gtm/tools.go`, tests, and
a README table entry.

- [ ] **Zones** — `create`, `get`, `list`, `update`, `delete`, `revert`.
      Zones subdivide a container for delegated access. Independent of the
      other items here; a reasonable first one to build.
- [ ] **Environments** — `create`, `get`, `list`, `update`, `delete`,
      `reauthorize`. `reauthorize` rotates an environment's auth token and has
      no counterpart in the existing tools; treat it as a destructive
      operation and require confirmation.
- [ ] **Google tag configurations** (`gtag_config`) — `create`, `get`,
      `list`, `update`, `delete`.
- [ ] **Destinations** — `get`, `list`, `link`, `unlink`. `link` and `unlink`
      change what a container publishes to; require confirmation.
- [ ] **Version headers** — `list`, `latest`. Cheap, and useful for finding a
      version to publish without listing full versions.

## Missing operations on resources already supported

- [ ] `container.combine`, `container.lookup`, `container.snippet`
- [ ] `version.live`, `version.undelete`
- [ ] `workspace.sync`, `workspace.quick_preview`, `workspace.resolve_conflict`
- [ ] `revert` on tags, triggers, variables, folders, clients,
      transformations, templates, and built-in variables

`revert` is the largest single item: eight near-identical tools. Build one,
settle the shape, then repeat.

## User permissions: out of scope

Decided 21 August 2026: **not built.** Not deferred pending a design — left out.

`user_permission` grants and removes access to GTM accounts and containers. An
AI assistant that can call it can give a stranger administrator access to a
container, or lock out the owner. The confirmation pattern in this server was
designed for "you will lose a tag", not for "you will give someone else
administrator access", and the two are not the same sentence to a user.

The gap against stape-io is therefore deliberate. If it is ever revisited, the
starting position is `get` and `list` only, with writes behind configuration
that is off by default.

## Sequence

1. Version headers and zones — small, self-contained, establish the pattern for
   a new resource file.
2. Environments and gtag configs — the same pattern, with one unusual
   operation each.
3. The missing operations on existing resources, `revert` last since it is
   repetitive.
4. Destinations.

## TODO — per resource

For each resource, in this order:

- [ ] Failing tests first, covering the happy path, a not-found error, and the
      confirmation requirement on destructive operations.
- [ ] `gtm/tool_<name>.go` following the existing file shape.
- [ ] Registration in `gtm/tools.go`.
- [ ] MCP resource URIs under `gtm://` if the resource is readable as a list.
- [ ] README table entry.
- [ ] `llms.txt` entry, so agents that read it learn the new tools.

## Interaction with the tools/list token cost

Measured 21 August 2026, by serialising the real `tools/list` response.

| Component | Bytes | Share |
|---|---|---|
| Input schemas | ~30,500 | 49% |
| JSON structure and tool names | ~24,000 | 39% |
| Tool description prose | ~7,300 | 12% |
| **Total** | **62,379** | **~15,600 tokens** |

That payload is sent on every conversation, before the user asks anything.
This plan adds roughly 30 tools and takes it toward 25,000 tokens.

**Trimming prose does not solve this.** It was tried in #91 and closed: every
mechanical edit available — dropping the article from the three ID
descriptions repeated 47, 44 and 38 times, dropping "This is a safety guard"
from seven confirmations, and removing the partial-update rule repeated on
eleven `update_tag` fields — saved 1,237 bytes, 2.0%. Only 12% of the payload
was ever prose. The number is made of structure.

**Decision, 21 August 2026: multiplex the tools at some future point.** Merging
the 50 single-operation tools into roughly 18 resource tools with an `action`
enum is the only lever that moves this materially, worth an estimated 5,000 to
7,000 tokens. It is deferred, not rejected.

This has a consequence for the sequence in this plan. New tools added now are
tools that a future multiplexing pass must also convert. Two options:

- Add the resources here as single-operation tools, consistent with the
  existing 50, and convert everything in one pass later.
- Wait, multiplex first, then add the new resources directly in the new shape.

The second does less total work. The first delivers coverage sooner. Decide
before starting the implementation, not during it.

Do not trim descriptions to buy room. The measurement says that room is not
there.

## Not in scope

- User permissions, for the reason above.
- Changing the existing 50 tools.
- Multiplexing tools behind an `action` parameter.
- The stdio transport — see `planning/stdio-transport.md`.
