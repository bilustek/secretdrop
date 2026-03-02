# User Timezone Support — Design Document

## Problem

Email notifications display secret expiry times in UTC. Users in different
timezones must mentally convert the time, which is error-prone and unfriendly.

## Solution

Store each user's IANA timezone (e.g. `"Europe/Istanbul"`), auto-detect it from
the browser on every login, and format email expiry times in the user's local
timezone alongside UTC.

## Decision Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| When to update timezone | Every login (auto) | Handles relocation without manual intervention |
| API design | Dedicated `PUT /api/v1/me/timezone` | Clean separation, no changes to auth flow |
| Timezone scope | Email only | API always returns UTC; frontend converts locally |
| Email format | Sender's TZ + UTC in parentheses | Recipient may not know sender's TZ; UTC as reference |
| Storage format | IANA timezone string | DST-safe, Go stdlib compatible |

## Database

Add column to existing `users` table via inline `ALTER TABLE` in `New()`:

```sql
ALTER TABLE users ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC'
```

Existing rows receive `'UTC'` as default. No data migration needed.

## User Model

```go
type User struct {
    // ... existing fields ...
    Timezone string // IANA timezone, e.g. "Europe/Istanbul"
}
```

## User Repository

New method on the interface:

```go
UpdateTimezone(ctx context.Context, userID int64, timezone string) error
```

SQLite implementation updates the `timezone` column.

## API

### `PUT /api/v1/me/timezone`

**Request:**
```json
{"timezone": "Europe/Istanbul"}
```

**Responses:**
- `204 No Content` — success
- `400 Bad Request` — invalid IANA timezone
- `401 Unauthorized` — not authenticated

**Validation:** `time.LoadLocation(tz)` — rejects unknown timezone names.

### `GET /api/v1/me` (updated response)

```json
{
  "email": "user@example.com",
  "timezone": "Europe/Istanbul",
  "...": "..."
}
```

## Email Formatting

Current format:
```
This secret will expire on Jan 2, 2026 at 3:04 PM UTC
```

New format (sender has timezone set):
```
This secret will expire on Jan 2, 2026 at 3:04 PM TRT (12:04 PM UTC)
```

When sender timezone is `"UTC"` (default), only UTC is shown (no redundant
parenthetical).

Implementation:
```go
loc, _ := time.LoadLocation(sender.Timezone)
local := expiresAt.In(loc).Format("Jan 2, 2006 at 3:04 PM MST")
utc := expiresAt.UTC().Format("3:04 PM UTC")
// combine: local + " (" + utc + ")" — skip parenthetical if loc == UTC
```

## Dockerfile

Already updated — `tzdata` added to Alpine certs stage, `/usr/share/zoneinfo`
copied to final image (required for `time.LoadLocation` at runtime).

## Frontend Flow

1. OAuth redirect lands on `/auth-callback`
2. `auth.refreshUser()` fetches `/api/v1/me` (existing behavior)
3. Browser timezone detected: `Intl.DateTimeFormat().resolvedOptions().timeZone`
4. If browser timezone differs from `me.timezone` → `PUT /api/v1/me/timezone`
5. Redirect to `/dashboard` (existing behavior)

No UI for manual timezone selection — fully automatic.

## Testing

**Backend:**
- Repository: `UpdateTimezone` — save and read back
- Handler: `PUT /api/v1/me/timezone` — valid (204), invalid (400), unauth (401)
- Email: dual format with sender timezone + UTC parenthetical
- Edge case: sender timezone is `"UTC"` — no redundant parenthetical

**Frontend:**
- Auth callback triggers timezone update when browser TZ differs from server TZ
