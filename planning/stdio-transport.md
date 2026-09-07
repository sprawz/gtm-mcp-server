# Plan: stdio transport mode

Status: **not implemented**. This document is the TODO list for it.

## Why

The server is HTTP-only today. Every user therefore sends Google credentials
through a server — the hosted one at `mcp.gtmeditor.com`, or their own.

A stdio mode removes that requirement. The MCP client starts the binary as a
subprocess and talks over stdin/stdout, so credentials never leave the user's
machine and there is no server to operate or trust. This is the one capability
`stape-io/google-tag-manager-mcp-server` offers that this server does not (its
`npx` CLI runs on stdio with a service account key or a refresh token).

## Why it is small

`getClient()` (`gtm/tools.go:92`) resolves credentials from the **context**,
not from the HTTP request. Its S2S branch already accepts a plain
`oauth2.TokenSource` with no HTTP dependency:

```go
if saTS := auth.GetSATokenSource(ctx); saTS != nil {
    return NewClient(ctx, saTS)
}
```

So stdio does not need a parallel code path through the tools. It needs a
different way to populate that same context key. **No tool changes at all** —
all 50 keep working unmodified.

Everything else required already exists:

| Piece | Where |
|---|---|
| `mcp.StdioTransport` | SDK v1.7.0, `mcp/transport.go:114` |
| Context injection hook | `server.AddReceivingMiddleware`, already used for logging (`main.go:61`) |
| Service-account token source | `auth.NewServiceAccountTokenSource` (`auth/service_account.go:16`) — handles key JSON and ADC |
| Config field | `cfg.ServiceAccountKeyJSON`, already loaded |
| stdout hygiene | already correct — `main.go:33` logs to stderr, stdout reserved for MCP |

## TODO

- [ ] **Config.** Add `Transport` to `config.Config`, from `MCP_TRANSPORT`
      (`http` default, `stdio` opt-in). Accept a `--stdio` flag that overrides
      the env var, since MCP client configs pass args more naturally than env.
- [ ] **Credential resolution for stdio.** Build the token source at start-up:
      1. `GOOGLE_SERVICE_ACCOUNT_KEY_JSON` if set, via the existing
         `NewServiceAccountTokenSource`.
      2. Application Default Credentials otherwise (same function already
         falls back to ADC).
      3. **New:** `GOOGLE_REFRESH_TOKEN` + client ID/secret, via
         `oauth2.Config.TokenSource`. This is the credential shape stape-io's
         CLI accepts and the only genuinely new code in this plan.
      Fail with a clear message naming all three options if none is present.
- [ ] **Middleware.** Add a receiving middleware that puts the token source at
      `auth.SATokenSourceKey` in the context, so `getClient` resolves it
      unchanged. Register it only in stdio mode.
- [ ] **Transport branch in `main`.** In stdio mode call
      `server.Run(ctx, &mcp.StdioTransport{})` and return. Skip the HTTP
      server, the OAuth authorization server, the token store, and the cleanup
      goroutine entirely — none of them have meaning without HTTP.
- [ ] **stdout audit.** Grep for `fmt.Print*` and any `log` writer that is not
      stderr. A single stray byte on stdout corrupts the JSON-RPC stream and
      the failure is opaque. `gtm/client.go` uses `log.Printf` (stderr by
      default) — confirm nothing overrides that, especially under `GTM_DEBUG`.
- [ ] **Signals.** Handle SIGINT/SIGTERM so the process exits when the client
      closes the pipe.

## TODO — tests

- [ ] `getClient` resolves the injected source with no HTTP middleware present.
- [ ] The refresh-token credential path produces a working token source.
- [ ] Start-up fails, with a message naming the three credential options, when
      none is configured.
- [ ] Nothing is written to stdout during start-up and a tool call. Assert on a
      captured stdout buffer — this is the failure that is hardest to notice by
      hand and the most damaging.

## TODO — docs

- [ ] README: a stdio section with a Claude Desktop config block.
- [ ] README: update the comparison table, which currently records stdio as a
      capability this server lacks.

## Out of scope

Serving both transports in one process. The modes are alternatives: one
process, one transport, chosen at start-up.

## Estimate

Roughly 60–100 lines across `main.go` and `config/config.go`, plus tests. Half
a day. The refresh-token credential mode and the stdout test are the only parts
that are not mechanical.
