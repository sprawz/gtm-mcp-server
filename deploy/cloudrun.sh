#!/usr/bin/env bash
set -euo pipefail

# Deploys the GTM MCP server to Google Cloud Run.
#
# Reads GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET (plus optional REDIS_URL)
# from the environment or a .env file in the repo root. The .env file must be
# simple KEY=value lines for this script to source it.
#
# Usage:
#   ./deploy/cloudrun.sh
#
# Overrides (env vars):
#   SERVICE       Cloud Run service name    (default: gtm-mcp)
#   REGION        Cloud Run region          (default: us-central1)
#   MIN_INSTANCES Keep N instances warm     (default: scale to zero)
#
# First deploy bootstraps BASE_URL: Cloud Run only assigns the service URL
# after a deploy exists, so we deploy with a placeholder, read the URL back,
# and re-apply it. Subsequent runs keep the existing URL and skip that step.

cd "$(dirname "$0")/.."

SERVICE="${SERVICE:-gtm-mcp}"
REGION="${REGION:-us-central1}"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

: "${GOOGLE_CLIENT_ID:?Set GOOGLE_CLIENT_ID in .env or the environment}"
: "${GOOGLE_CLIENT_SECRET:?Set GOOGLE_CLIENT_SECRET in .env or the environment}"

if ! command -v gcloud >/dev/null; then
  echo "gcloud CLI not found — install it: https://cloud.google.com/sdk/docs/install" >&2
  exit 1
fi

EXISTING_URL="$(gcloud run services describe "$SERVICE" --region "$REGION" \
  --format 'value(status.url)' 2>/dev/null || true)"

# '|' as the env-var delimiter: client secrets and Redis URLs can contain
# commas or '@', but never a pipe.
ENV_VARS="GOOGLE_CLIENT_ID=${GOOGLE_CLIENT_ID}"
ENV_VARS+="|GOOGLE_CLIENT_SECRET=${GOOGLE_CLIENT_SECRET}"
ENV_VARS+="|BASE_URL=${EXISTING_URL:-https://placeholder}"
if [[ -n "${REDIS_URL:-}" ]]; then
  ENV_VARS+="|REDIS_URL=${REDIS_URL}"
fi

ARGS=(run deploy "$SERVICE"
  --source .
  --region "$REGION"
  --allow-unauthenticated
  --set-env-vars "^|^${ENV_VARS}")
if [[ -n "${MIN_INSTANCES:-}" ]]; then
  ARGS+=(--min-instances "$MIN_INSTANCES")
fi

gcloud "${ARGS[@]}"

URL="$(gcloud run services describe "$SERVICE" --region "$REGION" \
  --format 'value(status.url)')"

if [[ "$URL" != "${EXISTING_URL}" ]]; then
  echo "Bootstrapping BASE_URL to ${URL}..."
  gcloud run services update "$SERVICE" --region "$REGION" \
    --update-env-vars "BASE_URL=${URL}"
fi

cat <<DONE

Deployed: ${URL}

If you haven't yet, add this redirect URI to your OAuth client in
Google Cloud Console (APIs & Services -> Credentials):

  ${URL}/oauth/callback

Then connect Claude: Settings -> Connectors -> Add custom connector -> ${URL}
DONE
