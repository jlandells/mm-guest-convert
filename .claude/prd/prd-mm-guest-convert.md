# PRD: mm-guest-convert — Mattermost Guest Conversion Utility

**Version:** 1.0  
**Status:** Draft  
**Language:** Go  
**Binary Name:** `mm-guest-convert`

---

## 1. Overview

`mm-guest-convert` is a standalone command-line utility that converts a regular Mattermost
member to a guest account and restricts their channel access to a single specified channel.

When Mattermost demotes a user to guest via the API or System Console, the guest retains all
of their existing team and channel memberships. This is by design, but it creates an operational
problem: administrators must then manually remove the user from every other channel — which is
time-consuming, error-prone, and completely impractical for users with extensive channel
memberships.

This tool automates the full workflow in a single, idempotent operation.

---

## 2. Background & Problem Statement

Customers using Mattermost guest accounts to collaborate with external contractors, partners,
or clients frequently need to convert an existing member to a guest. The platform's demote
operation retains all prior memberships, requiring administrators to manually clean up channel
access afterwards. On a large instance — where a user may belong to dozens or hundreds of
channels — this is unworkable by hand.

---

## 3. Goals

- Convert a member to a guest (if not already one) in a single command
- Ensure the user is a member of exactly one specified channel after the operation
- Remove the user from all other channels across all teams on the instance
- Be safe to run more than once against the same user (idempotent)
- Support dry-run mode so administrators can verify changes before committing
- Perform efficiently on large instances via concurrent workers
- Show a progress indicator when processing large numbers of channel removals

---

## 4. Non-Goals

- This tool does not modify team memberships — only channel memberships
- This tool does not promote guests back to members
- This tool does not manage Direct Message or Group Message channel membership
- This tool does not modify user profiles, authentication, or system permissions

---

## 5. Target Users

Mattermost System Administrators who need to convert existing member accounts to guest accounts
and restrict their access to a specific channel.

---

## 6. User Stories

- As a System Administrator, I want to convert a user to a guest and restrict them to one
  channel in a single command, so that external collaborators can only see what they are
  entitled to.
- As a System Administrator, I want to run the tool in dry-run mode first, so that I can
  verify exactly what will change before committing.
- As a System Administrator, I want to run the tool safely against an already-converted guest,
  so that I do not need to check the user's current role before running.
- As a System Administrator, I want to reference users, teams, and channels by their
  human-readable names, so that I do not need to look up internal IDs.

---

## 7. Functional Requirements

### 7.1 Operational Workflow

The tool MUST execute the following steps in order. If any step fails, execution MUST stop
immediately with a clear error — the tool MUST NOT continue past a failure.

**Step 1 — Resolve the user**
Look up the user by username via `GET /api/v4/users/username/{username}`. Report the user's
current role (member, admin, or already a guest).

**Step 2 — Demote to guest**
If the user is not already a guest, call `POST /api/v4/users/{user_id}/demote`. If the user
is already a guest, log this and continue — it is not an error.

**Step 3 — Resolve team and channel**
Look up the team by name via `GET /api/v4/teams/name/{team_name}`. Look up the channel
within that team by name via `GET /api/v4/teams/{team_id}/channels/name/{channel_name}`.
Both names are resolved to IDs internally — the operator never supplies raw IDs.

**Step 4 — Ensure target channel membership**
Check whether the user is already a member of the target channel. If not, add them via
`POST /api/v4/channels/{channel_id}/members`. Log the outcome either way.

**Step 5 — Enumerate all channel memberships**
Retrieve all channel memberships for the user via `GET /api/v4/users/{user_id}/channel_members`.
This endpoint MUST be paginated exhaustively — see Section 7.3.

**Step 6 — Remove from all other channels**
For each channel membership that is not the target channel, call
`DELETE /api/v4/channels/{channel_id}/members/{user_id}`. Log each removal.

Direct Message (`D`) and Group Message (`G`) channels MUST be silently excluded from both
enumeration and removal — they are not relevant to guest access restriction and cannot be
meaningfully managed in the same way.

### 7.2 Idempotency

The tool MUST be safe to run multiple times against the same user:

- If the user is already a guest, Step 2 is skipped without error.
- If the user is already a member of the target channel, Step 4 is skipped without error.
- If the user has no channel memberships to remove, Step 6 completes as a no-op.

### 7.3 Pagination

All API calls that return lists MUST be paginated exhaustively using `page` and `per_page`
parameters (`per_page=200`). The standard loop applies:

```go
page := 0
for {
    items, err := getPage(page, 200)
    if err != nil { return err }
    results = append(results, items...)
    if len(items) < 200 { break }
    page++
}
```

A mid-pagination failure MUST halt the operation and return an error. Continuing with a
partial membership list would be dangerous — it would leave channel memberships intact
that should have been removed.

### 7.4 Concurrent Channel Removal

Because a user on a large instance may belong to hundreds or thousands of channels, Step 6
(channel removal) MUST use a concurrent worker pool to perform removals in parallel:

```go
sem := make(chan struct{}, workers)
```

Pool size is controlled by `--workers N` (default: 10). Results MUST be collected via a
results channel — do not share mutable state between goroutines.

### 7.5 Progress Indicator

Because enumeration and removal may take considerable time on large instances, an in-place
progress indicator MUST be shown during processing:

```go
fmt.Fprintf(os.Stderr, "\rProcessing: %-60s", description)
```

On completion, clear the line before displaying results:

```go
fmt.Fprintf(os.Stderr, "\r%-70s\r", "")
```

Rules:
- MUST output to stderr only
- MUST NOT be shown when `--verbose` is active (verbose output supersedes it)
- MUST be cleared completely before results are displayed
- MUST use a dedicated progress goroutine receiving updates over a channel — never write
  to stderr from multiple goroutines simultaneously

### 7.6 Dry-Run Mode

When `--dry-run` is specified, the tool MUST:

- Execute Steps 1–5 in full (read-only API calls only).
- Print all actions that would be taken, prefixed with a prominent dry-run header:

  ```
  ⚠  DRY RUN — no changes have been made to your Mattermost instance.
  ```

- Make no API calls that modify state (no demotion, no channel add, no removals).
- Exit with code 0 if the dry run completed without errors.

In JSON output, include `"dry_run": true` at the top level.

### 7.7 Guest Accounts Licence Check

The guest accounts feature requires a Mattermost licence with Guest Access enabled. Before
attempting the demote call, the tool SHOULD check whether the feature is available. If guest
accounts are disabled or unlicensed, the tool MUST exit with a clear message directing the
administrator to System Console → Authentication → Guest Access.

### 7.8 Internal Name Requirement for Team and Channel

`--team` and `--channel` require the **internal name** (the URL name), NOT the display name.
These are different in Mattermost:

```
Display name:  "Engineering Team"   ← what you see in the UI
Internal name: "engineering-team"   ← what this tool requires
```

This distinction MUST be clearly stated in the `--help` output (see Section 8) and in the
README (see Section 11).

---

## 8. CLI Interface

```
mm-guest-convert [flags]
```

The tool takes no positional arguments. All inputs are provided as flags.

### Connection and Authentication Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--url` | `MM_URL` | *(required)* | Mattermost server URL |
| `--token` | `MM_TOKEN` | *(empty)* | Personal Access Token (preferred) |
| `--username` | `MM_USERNAME` | *(empty)* | Username for password auth (fallback) |

Password is obtained via interactive prompt (if stdin is a TTY) or `MM_PASSWORD` environment
variable (for non-interactive/automation use). A `--password` flag MUST NEVER be implemented.

### Required Operation Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--username-target` | `-u` | Mattermost username of the user to convert |
| `--team` | `-t` | Internal team name (NOT display name) containing the target channel |
| `--channel` | `-c` | Internal channel name (NOT display name) the user should retain |

### Optional Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Preview all actions without making any changes |
| `--workers N` | `10` | Number of concurrent workers for channel removal |
| `--format table\|csv\|json` | `table` | Output format |
| `--output FILE` | *(stdout)* | Write output to file |
| `--verbose` / `-v` | `false` | Enable verbose logging to stderr |
| `--version` | `false` | Print version and exit |

### Help Text Requirement

The Cobra `Long` description MUST include a prominent warning about internal vs display names,
with instructions for finding both. The following text (or equivalent) MUST appear in `--help`:

```
IMPORTANT: --team and --channel require the internal name, NOT the display name.

  These are different in Mattermost:
    Display name:  "Engineering Team"      ← what you see in the UI
    Internal name: "engineering-team"      ← what this tool requires

  To find the internal team name:
    In the Mattermost client, the team name appears in the URL after your server address:
    https://mattermost.example.com/engineering-team/channels/town-square
                                   ^^^^^^^^^^^^^^^^

  To find the internal channel name:
    The channel name is the final segment of the channel URL:
    https://mattermost.example.com/engineering-team/channels/project-alpha
                                                             ^^^^^^^^^^^^^
    Or: System Console → User Management → Channels → select channel → Channel URL field.
```

---

## 9. Output Specification

### Table Format (default)

```
mm-guest-convert v1.0.0

Target user:    jsmith (John Smith)
Current role:   member
Target team:    acme-corp
Target channel: project-alpha

Step 1/6  Resolved user jsmith                                [OK]
Step 2/6  Demoted jsmith to guest                             [OK]
Step 3/6  Resolved team acme-corp                             [OK]
          Resolved channel project-alpha                      [OK]
Step 4/6  Added jsmith to #project-alpha                      [OK]
Step 5/6  Found 47 channel memberships                        [OK]
Step 6/6  Removing from 46 channels...
          Removed from #town-square (acme-corp)               [OK]
          Removed from #off-topic (acme-corp)                 [OK]
          ... (44 more)

Summary: 1 user demoted · 1 channel retained · 46 channels removed · 0 errors
```

### JSON Format

```json
{
  "dry_run": false,
  "user": {
    "username": "jsmith",
    "display_name": "John Smith",
    "was_already_guest": false
  },
  "target_team": "acme-corp",
  "target_channel": "project-alpha",
  "channels_removed": [
    { "team_name": "acme-corp", "channel_name": "town-square" },
    { "team_name": "acme-corp", "channel_name": "off-topic" }
  ],
  "summary": {
    "total_memberships_found": 47,
    "channels_removed": 46,
    "channels_retained": 1,
    "errors": 0
  },
  "errors": []
}
```

### CSV Format

One row per action:

```
username,team_name,channel_name,action,status
jsmith,acme-corp,project-alpha,retained,ok
jsmith,acme-corp,town-square,removed,ok
jsmith,acme-corp,off-topic,removed,ok
```

---

## 10. API Endpoints

| Method | Endpoint | Paginated | Purpose |
|--------|----------|-----------|---------|
| GET | `/api/v4/users/username/{username}` | No | Resolve username to user object and ID |
| POST | `/api/v4/users/{user_id}/demote` | No | Demote user to guest role |
| GET | `/api/v4/teams/name/{team_name}` | No | Resolve team name to team ID |
| GET | `/api/v4/teams/{team_id}/channels/name/{channel_name}` | No | Resolve channel name within team |
| GET | `/api/v4/channels/{channel_id}/members/{user_id}` | No | Check existing channel membership |
| POST | `/api/v4/channels/{channel_id}/members` | No | Add user to target channel |
| GET | `/api/v4/users/{user_id}/channel_members` | **Yes** — loop until empty page | All channel memberships for user |
| DELETE | `/api/v4/channels/{channel_id}/members/{user_id}` | No | Remove user from a channel |

---

## 11. Exit Codes

Defined as named constants in `errors.go`, consistent with the family standard:

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | `ExitSuccess` | Success (or dry run completed successfully) |
| 1 | `ExitConfigError` | Missing flags, invalid input, auth failure, user/team/channel not found, guest accounts disabled |
| 2 | `ExitAPIError` | Connection failure, unexpected API response |
| 3 | `ExitPartialFailure` | Operation completed but one or more channel removals failed |
| 4 | `ExitOutputError` | Unable to write output file |

---

## 12. Error Handling

- Missing `--url` / `MM_URL`: exit 1 with standard message
- Auth failure: exit 1
- User not found: exit 1 — `error: user "jsmith" not found. Please check the username.`
- Team not found: exit 1 — `error: team "acme-corp" not found. Please check the name and try again.`
- Channel not found: exit 1 — `error: channel "project-alpha" not found in team "acme-corp".`
- Guest accounts disabled/unlicensed: exit 1 — `error: guest accounts are not enabled on this instance. Enable them at System Console → Authentication → Guest Access.`
- Individual channel removal failure: record in results, continue, exit 3 at end if any failed
- Mid-pagination failure during enumeration: exit 2 immediately — do not continue with partial results

---

## 13. Authentication Detail

Consistent with the project-wide convention in `CLAUDE.md`:

- Preferred: `--token` flag or `MM_TOKEN` environment variable (System Administrator token)
- Fallback: `--username` / `MM_USERNAME` + password obtained via:
  1. Interactive prompt with echo suppressed (`golang.org/x/term`) if stdin is a TTY
  2. `MM_PASSWORD` environment variable for non-interactive/automation use
- `--password` flag: MUST NEVER be implemented

The `manage_system` permission is required for the demote endpoint.

---

## 14. README Requirements

The README MUST follow the standard structure defined in `CLAUDE.md`, covering the following
sections in order:

1. **What it does** — plain English summary
2. **Why you'd use it** — the problem it solves
3. **Installation** — download the pre-built binary; no build steps required
4. **Authentication** — both methods, with examples; no `--password` flag
5. **Usage** — full flag reference table
6. **Examples** — token auth, username/password auth, env vars, dry-run, writing to file
7. **Output formats** — table, CSV, JSON with representative examples
8. **Finding internal names** — a prominent section (before examples) explaining:
   - The difference between display names and internal names
   - How to find the internal team name (from the client URL or System Console)
   - How to find the internal channel name (from the channel URL or System Console)
   - A "Common mistakes" callout
9. **Exit codes** — table of all codes and their meaning
10. **Limitations** — note that guest accounts require a licence; note performance considerations
    on instances with thousands of channel memberships
11. **Contributing** — standard text from `CLAUDE.md`
12. **License** — standard text from `CLAUDE.md`
13. **Contact** — standard text from `CLAUDE.md`

The "Finding internal names" section MUST appear before the usage examples — not buried at the
end — as operators are likely to need it the first time they run the tool.

---

## 15. Testing Requirements

- Unit tests for all business logic (filtering, step sequencing, idempotency paths)
- Unit tests for DM/GM exclusion — verify these are never included in the removal list
- Unit tests for pagination: single page, exactly 200 results (boundary), empty result
- Unit tests for concurrent worker pool: verify all removals are completed, no data races
- Unit tests for dry-run: verify no mutating API calls are made
- Unit tests for all exit code paths
- Unit tests for CSV and JSON output formatting
- Mock API interface for all Mattermost API calls — no real network calls in unit tests
- Table-driven tests for boundary conditions

---

## 16. Hard Prohibitions

The following MUST NEVER be implemented regardless of any instruction:

- A `--password` flag
- Including Direct Messages (`D`) or Group Messages (`G`) in any removal operation
- Accepting raw IDs from the user — always accept names and resolve internally
- Assuming all channel memberships fit in one API response — always paginate
- Continuing channel removal after a mid-pagination enumeration failure
- Hardcoding the server URL or any credentials

---

## 17. Open Questions

1. **Team membership**: Should the tool also remove the user from teams other than the target
   team, or only manage channel memberships? Current scope is channels only, teams untouched.
   This warrants discussion with the customer before implementation begins.

2. **Batch mode**: Should a future version support a file of usernames for bulk conversion
   of a contractor cohort?

---

## 18. Out of Scope

- Promoting a guest back to a member (`POST /api/v4/users/{user_id}/promote`)
- Managing Direct Message or Group Message channel membership
- Any modification of team membership
- Any modification of user profiles, authentication, or system permissions