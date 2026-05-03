# Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        AI Clients                                │
│  Claude · ChatGPT · Gemini CLI · Cursor · Custom agents         │
└──────────────────────────────┬──────────────────────────────────┘
                               │ HTTP (Streamable MCP)
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Caddy (reverse proxy)                        │
│  Auto-TLS · X-Forwarded-For · mcp.gtmeditor.com:443             │
└──────────────────────────────┬──────────────────────────────────┘
                               │ :8080
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      gtm-mcp-server                             │
│                                                                 │
│  ┌────────────┐  ┌────────────┐  ┌──────────────────────────┐  │
│  │ Rate Limit │→ │    Auth    │→ │     MCP Handler          │  │
│  │ middleware │  │ middleware │  │  (tools/resources/prompts)│  │
│  └────────────┘  └────────────┘  └─────────────┬────────────┘  │
│                                                 │               │
│                                                 ▼               │
│                                  ┌──────────────────────────┐   │
│                                  │      gtm/ package        │   │
│                                  │  (GTM API client layer)  │   │
│                                  └─────────────┬────────────┘   │
└─────────────────────────────────────────────────┼───────────────┘
                                                  │ HTTPS
                                                  ▼
                                   ┌──────────────────────────┐
                                   │  Google Tag Manager API   │
                                   │  tagmanager.googleapis.com│
                                   └──────────────────────────┘
```

## Package Responsibilities

### `main.go`
Entry point. Wires config, auth, middleware, MCP server, and HTTP mux together. Handles graceful shutdown (SIGINT/SIGTERM, 10s timeout).

### `config/`
Loads environment variables (with `.env`/`.env.local` dotenv support). Single `Config` struct, no globals. Validation is deferred — server starts even without OAuth credentials so `/health` and `/ping` always work.

### `auth/`
OAuth 2.1 implementation with two modes:

| Mode | When | Flow |
|------|------|------|
| **OAuth** | `GOOGLE_CLIENT_ID` set | Client → `/authorize` → Google → `/callback` → code → `/token` → access+refresh |
| **S2S** | `SERVICE_ACCOUNT_API_KEY` set | Client sends API key as Bearer → server uses Google Service Account for GTM calls |

Key design decisions:
- **In-memory token store.** No database. Redeployment requires re-auth. Acceptable because tokens are short-lived and the server is single-instance.
- **Auto-refresh in middleware.** When an access token expires but the refresh token is valid, middleware refreshes the Google token in-place and extends the access token TTL — the client's bearer stays valid without re-auth.
- **PKCE required.** No client_secret needed from MCP clients. Code binding via SHA256 challenge.
- **RFC compliance.** RFC 8414 (server metadata), RFC 9728 (protected resource metadata), RFC 7591 (dynamic client registration).

### `gtm/`
All Google Tag Manager API interaction. Split into layers:

```
tool_*.go        → MCP tool handlers (parse input, call client, format output)
mutations.go     → Create/Update/Delete logic (fingerprint handling, field remapping)
*.go (read)      → List/Get operations (accounts, containers, tags, triggers, etc.)
handler_helpers  → resolveWorkspace/resolveContainer (DRY: parse IDs → authenticated client)
types.go         → Input/output structs shared between tools and client methods
validation.go    → Input validation (required fields, format checks)
errors.go        → Google API error → user-friendly message + retry with backoff
```

**Fingerprint pattern:** Update operations fetch the current entity first, then pass `current.Fingerprint` as a URL parameter (not body field) to the Google API for optimistic concurrency control.

**autoEventFilter remapping:** The GTM API silently drops `autoEventFilter` for `linkClick`/`click`/`formSubmission` triggers. `mutations.go` owns the remapping to `filter` — tool handlers don't duplicate this logic.

### `middleware/`
- **Rate limiting.** Per-IP token bucket (`golang.org/x/time/rate`). When `TRUST_PROXY=true`, uses `X-Forwarded-For` for client IP; otherwise uses `RemoteAddr` only to prevent spoofing.
- **Logging.** Structured JSON logs for MCP request/response.

## Request Lifecycle

```
1. HTTP POST / (MCP streamable transport)
2. Rate limiter checks per-IP bucket
3. Auth middleware:
   a. Extract Bearer token from Authorization header
   b. S2S mode: if token == API key → inject SA token source into context
   c. OAuth mode: look up token in store
      - Valid → inject TokenInfo + GoogleToken into context
      - Expired → auto-refresh (extend in-place) → inject refreshed token
      - Not found → 401 with RFC 9728 metadata pointer
4. MCP handler dispatches to tool/resource/prompt by method+name
5. Tool handler:
   a. resolveWorkspace() → extracts IDs, builds authenticated GTM client from context
   b. Validates input
   c. Calls gtm.Client method
   d. Returns structured JSON result
6. Response streamed back to client
```

## Authentication Context Flow

```go
// After auth middleware, context carries:
ctx.Value(TokenInfoKey)      → *TokenInfo     (our token metadata)
ctx.Value(GoogleTokenKey)    → *oauth2.Token  (Google OAuth token)
ctx.Value(SATokenSourceKey)  → oauth2.TokenSource  (service account, if configured)
ctx.Value(TokenStoreKey)     → TokenStore
ctx.Value(GoogleProviderKey) → *GoogleProvider

// resolveWorkspace() in handler_helpers.go reads these to build a gtm.Client
// If SATokenSourceKey is present, it's used for GTM API calls (both S2S and OAuth+SA mode)
```

## Data Flow for Write Operations

```
Tool Input (JSON from AI)
    │
    ▼
Validation (gtm/validation.go)
    │
    ▼
Fetch current entity (for updates — needed for fingerprint + field preservation)
    │
    ▼
Merge input onto current (only override provided fields)
    │
    ▼
Apply remappings (e.g. autoEventFilter → filter for click triggers)
    │
    ▼
Google API call with .Fingerprint(current.Fingerprint)
    │
    ▼
Map response → tool output struct
```

## Configuration

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `PORT` | No | `8080` | Server listen port |
| `BASE_URL` | No | `http://localhost:8080` | Public URL for OAuth redirects |
| `GOOGLE_CLIENT_ID` | For OAuth | — | Google OAuth client |
| `GOOGLE_CLIENT_SECRET` | For OAuth | — | Google OAuth secret |
| `JWT_SECRET` | For OAuth | — | Token signing |
| `ACCESS_TOKEN_TTL` | No | `8h` | Access token lifetime |
| `TRUST_PROXY` | No | `false` | Trust X-Forwarded-For for rate limiting |
| `ALLOWED_HOSTS` | No | — | Additional trusted hosts for URL resolution |
| `SERVICE_ACCOUNT_API_KEY` | For S2S | — | Shared API key for team access |
| `GOOGLE_SERVICE_ACCOUNT_KEY_JSON` | For S2S | — | SA credentials (omit on GCP for Workload Identity) |
| `LOG_LEVEL` | No | `info` | `debug` or `info` |

## Deployment

Single Docker container behind Caddy. No external state (no DB, no Redis).

```
deploy.sh:
  1. SCP source files to VPS
  2. docker compose down && up --build
  3. Health check (GET /health)
  4. HTTPS verification (curl production URL)
```

Caddy handles TLS certificate provisioning automatically via Let's Encrypt.

## Testing

```bash
go test ./...          # All tests
go test ./auth/ -v     # Auth package (OAuth flow, middleware, token store)
go test ./gtm/ -v      # GTM package (error mapping)
go test ./middleware/   # Rate limiting
```

Test coverage focuses on:
- OAuth flow correctness (PKCE, state, token lifecycle)
- Auto-refresh behavior (expired → refresh → continue)
- Rate limiter isolation (per-IP, trust proxy behavior)
- Error mapping (Google API errors → user-friendly messages)

Integration tests (`auth/integration_test.go`) run the full OAuth flow with a mock Google token server.

## Key Invariants

- Every MCP tool call is authenticated (middleware rejects before dispatch).
- GTM write operations always use fingerprint-based optimistic concurrency.
- Token auto-refresh never invalidates the client's bearer token.
- Delete and publish operations require explicit `confirm: true`.
- The "unknown member" pattern: all dimension-like lookups (payment methods, trigger types) have exhaustive coverage so no tool call produces an unhandled case.
