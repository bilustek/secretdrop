#!/usr/bin/env bash
#
# Smoke test for SecretDrop API.
# Usage: ./scripts/bash/smoke-test.sh [base_url]
#
# Requires: curl, jq
# Start the server first:
#   GOLANG_ENV=development go run ./backend

set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
PASS=0
FAIL=0
TEST_EMAIL="smoketest@example.com"
SECRET_TEXT="SMOKE_TEST_SECRET=$(date +%s)"

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

# ------------------------------------------------------------------
bold "=== SecretDrop Smoke Test ==="
bold "Target: $BASE_URL"
echo ""

# ------------------------------------------------------------------
bold "1. GET /healthz"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/healthz")
assert_eq "status code is 200" "$HTTP_CODE" "200"

BODY=$(curl -s "$BASE_URL/healthz")
assert_contains "body contains ok" "$BODY" '"status":"ok"'
echo ""

# ------------------------------------------------------------------
bold "2. POST /api/v1/secrets (create secret)"
CREATE_RESP=$(curl -s -w "\n%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets" \
    -H "Content-Type: application/json" \
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
bold "3. POST /api/v1/secrets/{token}/reveal (reveal secret)"
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
bold "4. POST /api/v1/secrets/{token}/reveal (second reveal should fail)"
SECOND_RESP=$(curl -s -w "\n%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets/$TOKEN/reveal" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$TEST_EMAIL\",\"key\":\"$KEY\"}")

SECOND_CODE=$(echo "$SECOND_RESP" | tail -1)
assert_eq "status code is 404 (burned)" "$SECOND_CODE" "404"
echo ""

# ------------------------------------------------------------------
bold "5. POST /api/v1/secrets (validation: empty text)"
EMPTY_RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets" \
    -H "Content-Type: application/json" \
    -d '{"text":"","to":["a@b.com"]}')
assert_eq "status code is 422" "$EMPTY_RESP" "422"
echo ""

# ------------------------------------------------------------------
bold "6. POST /api/v1/secrets (validation: invalid email)"
INVALID_RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/secrets" \
    -H "Content-Type: application/json" \
    -d '{"text":"secret","to":["not-an-email"]}')
assert_eq "status code is 422" "$INVALID_RESP" "422"
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
