#!/usr/bin/env bash
#
# Smoke test for SecretDrop API.
# Usage: ./scripts/bash/smoke-test.sh [port]
#
# Requires: curl, jq, openssl, sqlite3, go
#
# This script is self-contained: it starts its own backend server with an
# isolated test database, runs the test suite, then cleans up everything
# (server process + test DB) regardless of success or failure.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

TEST_PORT="${1:-9099}"
BASE_URL="http://localhost:$TEST_PORT"
# DB path is relative to backend/ since the server runs from there.
TEST_DB_NAME="secretdrop-smoke-test.db"
TEST_DB_DIR="$PROJECT_ROOT/backend/db"
TEST_DB_FILE="$TEST_DB_DIR/$TEST_DB_NAME"
TEST_DB_URL="file:db/${TEST_DB_NAME}?_journal_mode=WAL"
SERVER_PID=""
PASS=0
FAIL=0
TEST_EMAIL="smoketest@example.com"
SECRET_TEXT="SMOKE_TEST_SECRET=$(date +%s)"

# Dev-mode JWT secret (matches backend fallback in main.go)
JWT_SECRET="dev-jwt-secret-do-not-use-in-production"

# ------------------------------------------------------------------
# Cleanup: kill server + remove test database on exit
# ------------------------------------------------------------------
cleanup() {
    echo ""
    bold "=== Cleanup ==="
    if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null
        wait "$SERVER_PID" 2>/dev/null || true
        green "  Server (pid=$SERVER_PID) stopped"
    fi

    for f in "$TEST_DB_FILE" "${TEST_DB_FILE}-wal" "${TEST_DB_FILE}-shm"; do
        if [[ -f "$f" ]]; then
            rm -f "$f"
        fi
    done
    green "  Test database removed"
}
trap cleanup EXIT

# ------------------------------------------------------------------
# Helpers
# ------------------------------------------------------------------

green()  { printf "\033[32m%s\033[0m\n" "$*"; }
red()    { printf "\033[31m%s\033[0m\n" "$*"; }
bold()   { printf "\033[1m%s\033[0m\n" "$*"; }

assert_eq() {
    local label="$1" got="$2" want="$3"
    if [[ "$got" == "$want" ]]; then
        green "  PASS: $label"
        PASS=$((PASS + 1))
    else
        red "  FAIL: $label (got=$got, want=$want)"
        FAIL=$((FAIL + 1))
    fi
}

assert_contains() {
    local label="$1" got="$2" want="$3"
    if [[ "$got" == *"$want"* ]]; then
        green "  PASS: $label"
        PASS=$((PASS + 1))
    else
        red "  FAIL: $label (output does not contain '$want')"
        FAIL=$((FAIL + 1))
    fi
}

assert_not_empty() {
    local label="$1" got="$2"
    if [[ -n "$got" && "$got" != "null" ]]; then
        green "  PASS: $label"
        PASS=$((PASS + 1))
    else
        red "  FAIL: $label (value is empty or null)"
        FAIL=$((FAIL + 1))
    fi
}

# ------------------------------------------------------------------
# JWT helper: generate a HS256 token using openssl
# ------------------------------------------------------------------
base64url_encode() {
    openssl enc -base64 -A | tr '+/' '-_' | tr -d '='
}

generate_jwt() {
    local user_id="$1" email="$2" tier="$3"
    local now exp header payload signature

    now=$(date +%s)
    exp=$((now + 3600))

    header=$(printf '{"alg":"HS256","typ":"JWT"}' | base64url_encode)
    payload=$(printf '{"sub":%d,"email":"%s","tier":"%s","exp":%d,"iat":%d}' \
        "$user_id" "$email" "$tier" "$exp" "$now" | base64url_encode)

    signature=$(printf '%s.%s' "$header" "$payload" \
        | openssl dgst -sha256 -hmac "$JWT_SECRET" -binary \
        | base64url_encode)

    printf '%s.%s.%s' "$header" "$payload" "$signature"
}

# ------------------------------------------------------------------
# Seed a test user into the SQLite database
# ------------------------------------------------------------------
seed_test_user() {
    # Wait for the users table to be created by the server migrations.
    local attempt=0
    while ! sqlite3 "$TEST_DB_FILE" "SELECT 1 FROM users LIMIT 0;" 2>/dev/null; do
        attempt=$((attempt + 1))
        if [[ $attempt -ge 10 ]]; then
            echo "users table not found after 10s" >&2
            return 1
        fi
        sleep 1
    done

    sqlite3 -noheader -csv "$TEST_DB_FILE" "
        INSERT OR IGNORE INTO users (provider, provider_id, email, name, tier, secrets_used)
        VALUES ('smoke-test', 'smoke-test-1', '$TEST_EMAIL', 'Smoke Test', 'pro', 0);
    "

    sqlite3 -noheader -csv "$TEST_DB_FILE" "
        SELECT id FROM users WHERE provider='smoke-test' AND provider_id='smoke-test-1';
    "
}

# ------------------------------------------------------------------
# Wait for server to become ready
# ------------------------------------------------------------------
wait_for_server() {
    local max_attempts=30
    local attempt=0
    while ! curl -s -o /dev/null "$BASE_URL/healthz" 2>/dev/null; do
        attempt=$((attempt + 1))
        if [[ $attempt -ge $max_attempts ]]; then
            red "  FAIL: server did not start within ${max_attempts}s"
            exit 1
        fi
        sleep 1
    done
}

# ==================================================================
bold "=== SecretDrop Smoke Test ==="
bold "Target: $BASE_URL (isolated test database)"
echo ""

# Kill any leftover process on the test port.
if lsof -ti:"$TEST_PORT" >/dev/null 2>&1; then
    lsof -ti:"$TEST_PORT" | xargs kill 2>/dev/null
    sleep 1
fi

# ------------------------------------------------------------------
bold "0. Start backend server"

# Ensure test DB directory exists
mkdir -p "$TEST_DB_DIR"

cd "$PROJECT_ROOT/backend"
GOLANG_ENV=development \
DATABASE_URL="$TEST_DB_URL" \
PORT="$TEST_PORT" \
    go run -race ./cmd/secretdrop/ &
SERVER_PID=$!

echo "  Server starting (pid=$SERVER_PID)..."
wait_for_server
green "  OK: server is ready"
echo ""

# ------------------------------------------------------------------
bold "1. Seed test user"
TEST_USER_ID=$(seed_test_user)
if [[ -z "$TEST_USER_ID" ]]; then
    red "  FAIL: could not seed test user"
    exit 1
fi
green "  OK: test user id=$TEST_USER_ID"

AUTH_TOKEN=$(generate_jwt "$TEST_USER_ID" "$TEST_EMAIL" "pro")
green "  OK: JWT generated"
echo ""

# ------------------------------------------------------------------
bold "2. GET /healthz"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/healthz")
assert_eq "status code is 200" "$HTTP_CODE" "200"

BODY=$(curl -s "$BASE_URL/healthz")
assert_contains "body contains ok" "$BODY" '"status":"ok"'
assert_contains "body contains version" "$BODY" '"version"'
echo ""

# ------------------------------------------------------------------
bold "3. GET /docs"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/docs")
assert_eq "status code is 200" "$HTTP_CODE" "200"
echo ""

# ------------------------------------------------------------------
bold "4. GET /docs/openapi.yaml"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/docs/openapi.yaml")
assert_eq "status code is 200" "$HTTP_CODE" "200"
echo ""

# ------------------------------------------------------------------
bold "5. POST /api/v1/secrets without auth (should fail)"
NOAUTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets" \
    -H "Content-Type: application/json" \
    -d '{"text":"secret","to":["a@b.com"]}')
assert_eq "status code is 401" "$NOAUTH_CODE" "401"
echo ""

# ------------------------------------------------------------------
bold "6. POST /api/v1/secrets (create secret with auth)"
CREATE_RESP=$(curl -s -w "\n%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -d "{\"text\":\"$SECRET_TEXT\",\"to\":[\"$TEST_EMAIL\"]}")

CREATE_CODE=$(echo "$CREATE_RESP" | tail -1)
CREATE_BODY=$(echo "$CREATE_RESP" | sed '$d')

assert_eq "status code is 201" "$CREATE_CODE" "201"

BATCH_ID=$(echo "$CREATE_BODY" | jq -r '.id')
assert_contains "response has batch id" "$BATCH_ID" "-"

LINK=$(echo "$CREATE_BODY" | jq -r '.recipients[0].link')
assert_contains "link contains /s/" "$LINK" "/s/"
assert_contains "link contains # fragment" "$LINK" "#"

# extract token and key from link: .../s/{token}#{key}
TOKEN=$(echo "$LINK" | sed 's|.*\/s\/||' | cut -d'#' -f1)
KEY=$(echo "$LINK" | cut -d'#' -f2)

echo "  token=$TOKEN"
echo "  key=${KEY:0:12}..."
echo ""

# ------------------------------------------------------------------
bold "7. POST /api/v1/secrets/{token}/reveal (reveal secret)"
REVEAL_RESP=$(curl -s -w "\n%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets/$TOKEN/reveal" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$TEST_EMAIL\",\"key\":\"$KEY\"}")

REVEAL_CODE=$(echo "$REVEAL_RESP" | tail -1)
REVEAL_BODY=$(echo "$REVEAL_RESP" | sed '$d')

assert_eq "status code is 200" "$REVEAL_CODE" "200"

REVEALED_TEXT=$(echo "$REVEAL_BODY" | jq -r '.text')
assert_eq "revealed text matches" "$REVEALED_TEXT" "$SECRET_TEXT"
echo ""

# ------------------------------------------------------------------
bold "8. POST /api/v1/secrets/{token}/reveal (second reveal should fail)"
SECOND_RESP=$(curl -s -w "\n%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets/$TOKEN/reveal" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$TEST_EMAIL\",\"key\":\"$KEY\"}")

SECOND_CODE=$(echo "$SECOND_RESP" | tail -1)
assert_eq "status code is 404 (burned)" "$SECOND_CODE" "404"
echo ""

# ------------------------------------------------------------------
bold "9. GET /api/v1/me (with auth)"
ME_RESP=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    "$BASE_URL/api/v1/me")

ME_CODE=$(echo "$ME_RESP" | tail -1)
ME_BODY=$(echo "$ME_RESP" | sed '$d')

assert_eq "status code is 200" "$ME_CODE" "200"

ME_EMAIL=$(echo "$ME_BODY" | jq -r '.email')
assert_eq "me email matches" "$ME_EMAIL" "$TEST_EMAIL"

ME_TIER=$(echo "$ME_BODY" | jq -r '.tier')
assert_eq "me tier is pro" "$ME_TIER" "pro"
echo ""

# ------------------------------------------------------------------
bold "10. GET /api/v1/me without auth (should fail)"
NOAUTH_ME_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    "$BASE_URL/api/v1/me")
assert_eq "status code is 401" "$NOAUTH_ME_CODE" "401"
echo ""

# ------------------------------------------------------------------
bold "11. POST /api/v1/secrets (validation: empty text)"
EMPTY_RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -d '{"text":"","to":["a@b.com"]}')
assert_eq "status code is 422" "$EMPTY_RESP" "422"
echo ""

# ------------------------------------------------------------------
bold "12. POST /api/v1/secrets (validation: invalid email)"
INVALID_RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -d '{"text":"secret","to":["not-an-email"]}')
assert_eq "status code is 422" "$INVALID_RESP" "422"
echo ""

# ------------------------------------------------------------------
bold "13. GET /auth/google (should redirect)"
GOOGLE_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    "$BASE_URL/auth/google")
assert_eq "status code is 307 (redirect to Google)" "$GOOGLE_CODE" "307"
echo ""

# ------------------------------------------------------------------
bold "14. GET /auth/github (should redirect)"
GITHUB_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    "$BASE_URL/auth/github")
assert_eq "status code is 307 (redirect to GitHub)" "$GITHUB_CODE" "307"
echo ""

# ------------------------------------------------------------------
bold "=== Results ==="
green "Passed: $PASS"
if [[ $FAIL -gt 0 ]]; then
    red "Failed: $FAIL"
    exit 1
else
    green "All tests passed!"
fi
