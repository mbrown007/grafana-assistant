#!/usr/bin/env bash
# setup_mattermost.sh — Create a Mattermost team, admin user, and bot account
# for ChatOps dev/demo. Idempotent: safe to run multiple times.
set -euo pipefail

MM_URL="${MM_URL:-http://localhost:18065}"
TEAM_NAME="${MM_TEAM_NAME:-monitoring}"
ADMIN_USER="${MM_ADMIN_USER:-admin}"
ADMIN_PASS="${MM_ADMIN_PASS:-Admin1234!}"
ADMIN_EMAIL="${MM_ADMIN_EMAIL:-admin@localhost}"
BOT_USERNAME="${MM_BOT_USERNAME:-assistant-bot}"
BOT_DISPLAY="${MM_BOT_DISPLAY:-Monitoring Assistant}"

# --- Helpers ----------------------------------------------------------------

api() {
  local method="$1" path="$2"
  shift 2
  curl -s -X "$method" "${MM_URL}/api/v4${path}" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN:-}" \
    "$@"
}

wait_healthy() {
  echo "Waiting for Mattermost at ${MM_URL} ..."
  local i=0
  while ! curl -sf "${MM_URL}/api/v4/system/ping" >/dev/null 2>&1; do
    i=$((i + 1))
    if [ "$i" -ge 60 ]; then
      echo "ERROR: Mattermost did not become healthy after 60s"
      exit 1
    fi
    sleep 1
  done
  echo "Mattermost is healthy."
}

# --- Main -------------------------------------------------------------------

wait_healthy

# 1. Create admin user (first user becomes system admin).
echo "Creating admin user '${ADMIN_USER}'..."
RESULT=$(curl -s -X POST "${MM_URL}/api/v4/users" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${ADMIN_EMAIL}\",\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}" 2>&1)

if echo "$RESULT" | grep -q '"id"'; then
  echo "  Admin user created."
elif echo "$RESULT" | grep -qi 'already exists\|taken\|already_taken'; then
  echo "  Admin user already exists."
else
  echo "  Warning: $RESULT"
fi

# 2. Log in and get token.
echo "Logging in as '${ADMIN_USER}'..."
LOGIN=$(curl -s -X POST "${MM_URL}/api/v4/users/login" \
  -H "Content-Type: application/json" \
  -d "{\"login_id\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}" \
  -D - 2>/dev/null)

TOKEN=$(echo "$LOGIN" | grep -i '^Token:' | tr -d '[:space:]' | cut -d: -f2)
if [ -z "$TOKEN" ]; then
  echo "ERROR: Failed to log in. Response:"
  echo "$LOGIN"
  exit 1
fi
echo "  Logged in (token obtained)."

# 3. Create team.
echo "Creating team '${TEAM_NAME}'..."
TEAM_RESULT=$(api POST /teams -d "{\"name\":\"${TEAM_NAME}\",\"display_name\":\"${TEAM_NAME}\",\"type\":\"O\"}")
TEAM_ID=$(echo "$TEAM_RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || true)

if [ -n "$TEAM_ID" ] && [ "$TEAM_ID" != "" ]; then
  echo "  Team created: ${TEAM_ID}"
else
  # Team may already exist — look it up.
  TEAM_ID=$(api GET "/teams/name/${TEAM_NAME}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || true)
  if [ -n "$TEAM_ID" ]; then
    echo "  Team already exists: ${TEAM_ID}"
  else
    echo "  Warning: could not create or find team."
  fi
fi

# 4. Create bot account.
echo "Creating bot '${BOT_USERNAME}'..."
BOT_RESULT=$(api POST /bots -d "{\"username\":\"${BOT_USERNAME}\",\"display_name\":\"${BOT_DISPLAY}\"}")
BOT_USER_ID=$(echo "$BOT_RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('user_id',''))" 2>/dev/null || true)

if [ -n "$BOT_USER_ID" ] && [ "$BOT_USER_ID" != "" ]; then
  echo "  Bot created: ${BOT_USER_ID}"
else
  # Bot may already exist.
  BOT_USER_ID=$(api GET "/bots/username/${BOT_USERNAME}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('user_id',''))" 2>/dev/null || true)
  if [ -z "$BOT_USER_ID" ]; then
    echo "  Warning: could not create or find bot."
  else
    echo "  Bot already exists: ${BOT_USER_ID}"
  fi
fi

# 5. Add bot to team.
if [ -n "$TEAM_ID" ] && [ -n "$BOT_USER_ID" ]; then
  api POST "/teams/${TEAM_ID}/members" -d "{\"team_id\":\"${TEAM_ID}\",\"user_id\":\"${BOT_USER_ID}\"}" >/dev/null 2>&1 || true
  echo "  Bot added to team '${TEAM_NAME}'."
fi

# 6. Generate personal access token for bot.
echo "Generating personal access token..."
PAT_RESULT=$(api POST "/users/${BOT_USER_ID}/tokens" -d "{\"description\":\"chatops-bridge\"}")
BOT_TOKEN=$(echo "$PAT_RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || true)

if [ -n "$BOT_TOKEN" ] && [ "$BOT_TOKEN" != "" ]; then
  echo ""
  echo "============================================================"
  echo "  Mattermost bot setup complete!"
  echo ""
  echo "  Add this to your config.yaml:"
  echo ""
  echo "  feature_flags:"
  echo "    chatops_enabled: true"
  echo ""
  echo "  chatops:"
  echo "    provider: mattermost"
  echo "    bot_user_id: 99"
  echo "    bot_user_name: ${BOT_USERNAME}"
  echo "    bot_org_id: 1"
  echo "    mattermost:"
  echo "      url: ${MM_URL}"
  echo "      token: ${BOT_TOKEN}"
  echo "      team_name: ${TEAM_NAME}"
  echo "      channel_ids: []"
  echo ""
  echo "  Open Mattermost: ${MM_URL}"
  echo "  Login: ${ADMIN_USER} / ${ADMIN_PASS}"
  echo "============================================================"
else
  echo "  Warning: could not generate token. You may need to create one manually."
  echo "  Bot user ID: ${BOT_USER_ID}"
fi
