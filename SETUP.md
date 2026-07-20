# Setup Guide — RapidWebLaunch fork

How to get this GTM MCP server connected to Claude so it can manage your own
and your clients' Google Tag Manager containers.

There are two ways to use it. Pick one:

| | Option A: Hosted server | Option B: Self-host this fork |
|---|---|---|
| Effort | ~2 minutes | ~30–45 minutes one-time |
| Infrastructure | None (uses `mcp.gtmeditor.com`, run by the upstream author) | Cloud Run or any Docker host |
| Where your Google tokens live | Upstream author's server (in memory) | Your own server |
| Cost | Free | Cloud Run free tier covers light agency use; a small VPS also works |

Option A is the fastest way to try it out. Option B keeps the OAuth tokens
for your and your clients' GTM accounts on infrastructure you control —
worth it if you manage client data under your own agency policies.

---

## Option A — Use the hosted server (2 minutes)

**Claude.ai / Cowork / Claude Desktop:**

1. Go to **Settings → Connectors → Add custom connector**
2. Enter `https://mcp.gtmeditor.com`
3. Click **Add**, then sign in with the Google account that has access to
   your GTM accounts

**Claude Code (CLI):**

```bash
claude mcp add -t http gtm https://mcp.gtmeditor.com
```

Done. Skip to [Client access model](#client-access-model).

---

## Option B — Self-host this fork

### Step 1 — Google Cloud setup (one-time)

1. Go to [Google Cloud Console](https://console.cloud.google.com/) and create
   a project (or reuse an existing one), e.g. `rapidweblaunch-gtm-mcp`.
2. **APIs & Services → Library** → search **Tag Manager API** → **Enable**.
3. **APIs & Services → OAuth consent screen**:
   - If your Google account is on Google Workspace (e.g. a
     `@rapidweblaunch.com` account), choose **Internal**. This avoids app
     verification entirely and refresh tokens never expire from testing-mode
     limits.
   - Otherwise choose **External** and add yourself as a test user. Note: in
     testing mode Google expires refresh tokens after **7 days**, so you'll
     re-authenticate weekly until the app is published.
   - Scopes: add `https://www.googleapis.com/auth/tagmanager.edit.containers`,
     `.../tagmanager.publish`, and `.../tagmanager.readonly` (the server
     requests these at sign-in).
4. **APIs & Services → Credentials → Create Credentials → OAuth client ID**:
   - Application type: **Web application**
   - Authorized redirect URIs:
     ```
     https://claude.ai/api/mcp/auth_callback
     https://claude.com/api/mcp/auth_callback
     https://YOUR-SERVER-DOMAIN/oauth/callback
     ```
     (Add `https://chatgpt.com/connector_platform_oauth_redirect` too if you
     ever want ChatGPT access.)
5. Copy the **Client ID** and **Client Secret**.

### Step 2 — Deploy

#### Path 1: Google Cloud Run (recommended — no server to maintain, HTTPS included)

```bash
# From the repo root, first deploy (BASE_URL is a placeholder for now):
gcloud run deploy gtm-mcp \
  --source . \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars "GOOGLE_CLIENT_ID=your-id.apps.googleusercontent.com,GOOGLE_CLIENT_SECRET=your-secret,BASE_URL=https://placeholder"

# Grab the URL Cloud Run assigned (e.g. https://gtm-mcp-xxxxx-uc.a.run.app):
gcloud run services describe gtm-mcp --region us-central1 --format 'value(status.url)'

# Set BASE_URL to that real URL:
gcloud run services update gtm-mcp --region us-central1 \
  --set-env-vars "BASE_URL=https://gtm-mcp-xxxxx-uc.a.run.app"
```

Then go back to your OAuth client in Google Cloud Console and set the third
redirect URI to `https://gtm-mcp-xxxxx-uc.a.run.app/oauth/callback`.

Notes:
- For production hygiene, move the client secret to Secret Manager
  (`--set-secrets` instead of `--set-env-vars`).
- Tokens are stored **in memory**, so a new Cloud Run revision or cold start
  means reconnecting the connector in Claude. Setting
  `--min-instances 1` reduces this (small always-on cost).

#### Path 2: Docker on a VPS you already run

```bash
git clone https://github.com/pattitudez/gtm-mcp-server.git
cd gtm-mcp-server

cp .env.example .env        # fill in GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, BASE_URL
cp Caddyfile.example Caddyfile   # set your real domain

# Point a DNS A record for that domain at the VPS, then:
docker compose up -d
```

Caddy provisions HTTPS automatically. Set the OAuth redirect URI to
`https://your-domain/oauth/callback`.

### Step 3 — Connect Claude

**Claude.ai / Cowork / Claude Desktop:**
Settings → Connectors → Add custom connector → enter your server URL
(e.g. `https://gtm-mcp-xxxxx-uc.a.run.app`) → sign in with Google.

**Claude Code (CLI):**

```bash
claude mcp add -t http gtm https://your-server-url
```

**Verify:** ask Claude to run the `ping` tool, then `list_accounts`. You
should see every GTM account your Google login can access.

---

## Client access model

The server sees exactly what the signed-in Google account can see in GTM —
nothing more. To manage a client's container:

1. Have the client add your Google account in
   **GTM → Admin → User Management** on their account, with **Edit** (or
   **Publish** if you release changes for them) permission.
2. That container then shows up automatically in `list_accounts` /
   `list_containers` — no server changes needed per client.

This is the normal agency pattern and means one connector covers your own
site plus every client site.

---

## Safety notes

- All edits land in a GTM **workspace** — nothing goes live until you
  explicitly ask to `create_version` and `publish_version`, and publishing
  requires confirmation.
- Known upstream issue: `autoEventFilter` conditions ("Some Link Clicks" /
  "Some Form Submissions") are silently dropped by Google's API — configure
  those specific conditions in the GTM web UI
  ([upstream issue #33](https://github.com/paolobietolini/gtm-mcp-server/issues/33)).
- Tokens are held in memory only; restarting the server logs everyone out
  (reconnect the connector to sign back in).

## Better results in Claude Code

Install the bundled skill so Claude follows the correct GTM workflows
(ID discovery, parameter formats, safe publish flow):

```bash
mkdir -p ~/.claude/skills && cp -r skills/gtm-mcp ~/.claude/skills/
```
