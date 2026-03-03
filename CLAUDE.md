# CLAUDE.md — Mattermost Admin Utilities

## ⚠ Important: Purpose of This File

**This file is written for Claude Code. It is not user documentation, developer documentation,
or a project README.**

- This file exists solely to provide Claude Code with the context, conventions, and requirements
  it needs to work consistently and correctly across all tools in this family.
- Do NOT add user-facing or developer-facing documentation here.
- All project documentation — architecture notes, API references, contribution guides, design
  decisions, and anything intended for humans — MUST go in the `docs/` folder.
- The end-user guide for each tool lives in `README.md`. See the README Requirements section
  of this file for what that must contain.

---

This file provides project-wide context, conventions, and requirements for all tools in the
Mattermost Admin Utilities family. Read this file in full before writing any code. Every
convention described here MUST be followed consistently across all tools in the family.

---

## Project Context

This is one tool in a family of standalone command-line utilities for Mattermost System
Administrators. Each tool addresses a specific gap — something that cannot be done (or cannot
be done efficiently) through the Mattermost UI or `mmctl`.

The tools are used by real administrators in production environments, including regulated industries
such as defence, government, and financial services. They must be robust, clear in their output,
safe in their behaviour, and trivial to deploy.

The individual PRD for this tool lives at `.claude/prd/prd-<toolname>.md` and contains the full
functional specification. This file covers the shared patterns and conventions that apply to all
tools in the family.

### ⚠ Scope Warning

**Work only within this repository.** Do not search for, fetch, or reference any other tools in
this family (e.g. `mm-guest-audit`, `mm-plugin-audit`, `mm-ldap-sim`, `mm-bulk-archive`, etc.).
Those are separate projects in separate repositories and are entirely irrelevant to the task at
hand. All context needed to build this tool is contained within this repository — in this file
and in the PRD.

---

## Language and Build

- All tools MUST be written in **Go**
- Go version: use the most recent stable release
- No unnecessary third-party dependencies — prefer the standard library wherever possible
- Permitted third-party packages:
  - `golang.org/x/term` — for suppressed password prompts (required)
  - `github.com/mattermost/mattermost/server/public/model` — Mattermost API client (preferred over raw HTTP)
  - A CLI flag library if needed (e.g. `github.com/spf13/cobra` for tools with subcommands, standard `flag` package for simple tools)
  - A CSV library is not needed — use `encoding/csv` from the standard library
  - A JSON library is not needed — use `encoding/json` from the standard library

### Cross-Platform Compilation

Every tool MUST produce binaries for the following targets:

| OS      | Architecture | Output filename               |
|---------|-------------|-------------------------------|
| Linux   | amd64       | `<toolname>-linux-amd64`      |
| macOS   | amd64       | `<toolname>-darwin-amd64`     |
| macOS   | arm64       | `<toolname>-darwin-arm64`     |
| Windows | amd64       | `<toolname>-windows-amd64.exe`|

The `Makefile` MUST include a `build-all` target that produces all four binaries. Example:

```makefile
build-all:
	GOOS=linux   GOARCH=amd64 go build -o bin/$(BINARY)-linux-amd64 .
	GOOS=darwin  GOARCH=amd64 go build -o bin/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -o bin/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY)-windows-amd64.exe .
```

### Versioning

- Version MUST be embedded at build time using `-ldflags`
- A `--version` flag MUST be supported on every tool
- Example build with version:
  ```
  go build -ldflags="-X main.version=1.0.0" -o bin/<toolname> .
  ```

---

## Project Structure

Each tool MUST follow this standard layout:

```
<toolname>/
├── .claude/
│   └── prd/
│       └── prd-<toolname>.md    # The PRD for this specific tool
├── main.go              # Entry point, flag parsing, orchestration only — no business logic
├── client.go            # Mattermost API client initialisation and authentication
├── <feature>.go         # One or more files containing the core business logic
├── output.go            # All output formatting (table, CSV, JSON)
├── errors.go            # Exit codes as constants, error types
├── <feature>_test.go    # Unit tests alongside the code they test
├── Makefile             # build, build-all, test, lint targets
├── CLAUDE.md            # This file — project-wide conventions for Claude Code
└── README.md            # End-user documentation (see README requirements below)
```

`CLAUDE.md` lives in the repo root so that Claude Code picks it up automatically when working
in the project. The `.claude/prd/` folder is the conventional home for PRDs and any other
Claude Code context documents — it keeps them clearly separated from the actual project files
while making their purpose immediately obvious.

`main.go` should be thin — flag parsing, validation, calling into the business logic, and handling
the top-level error. Business logic MUST live in separate files so it can be unit tested without
instantiating a full CLI context.

---

## Connection and Authentication

### Server URL

- MUST be provided via `--url` flag or `MM_URL` environment variable
- There is no sensible default — the tool MUST fail immediately with a clear error if neither is provided
- Error message: `error: server URL is required. Use --url or set the MM_URL environment variable.`
- The URL should be normalised on input: strip any trailing slash

### Authentication

Every tool MUST support two authentication methods, resolved in the following order:

#### Option 1 — Personal Access Token (preferred)

```
--token TOKEN    or    MM_TOKEN environment variable
```

- This is the recommended method and should be documented as such in the README
- The token must belong to an account with System Administrator role
- Use as a Bearer token: `Authorization: Bearer <token>`

#### Option 2 — Username and Password (fallback)

```
--username USERNAME    or    MM_USERNAME environment variable
```

Password MUST be obtained by one of the following methods — in this order of preference:

1. **Interactive prompt** — if stdin is a TTY, prompt for the password using `golang.org/x/term`:
   ```go
   fmt.Print("Password: ")
   passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
   fmt.Println() // move to next line after input
   ```
   This suppresses echo so the password is never visible on screen.

2. **`MM_PASSWORD` environment variable** — for non-interactive/automation use only.
   Document clearly in the README that this is for automation scenarios and should not be
   used in interactive sessions.

**A `--password` flag MUST NEVER be implemented.** Passwords passed as CLI flags appear in
shell history, `ps` output, and system logs. If asked to add one, decline and explain why.

#### Authentication Implementation

On startup, the tool should:
1. Check for `--token` / `MM_TOKEN` → if found, use token auth
2. Check for `--username` / `MM_USERNAME` → if found, obtain password and use session auth
3. If neither is provided, exit with:
   `error: authentication required. Use --token (or MM_TOKEN) for token auth, or --username (or MM_USERNAME) for password auth.`

---

## Standard Flags

Every tool MUST implement the following flags consistently. Do not rename them, do not use
shorthand alternatives as the primary flag name.

| Flag | Env Var | Type | Default | Description |
|------|---------|------|---------|-------------|
| `--url` | `MM_URL` | string | *(required)* | Mattermost server URL |
| `--token` | `MM_TOKEN` | string | *(empty)* | Personal Access Token |
| `--username` | `MM_USERNAME` | string | *(empty)* | Username for password auth |
| `--format` | *(none)* | string | `table` | Output format: `table`, `csv`, `json` |
| `--output` | *(none)* | string | *(stdout)* | Write output to this file path |
| `--verbose` / `-v` | *(none)* | bool | `false` | Enable verbose logging to stderr |
| `--version` | *(none)* | bool | `false` | Print version and exit |

Tools with destructive operations MUST also implement:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | bool | `false` | Preview mode — no changes made |

Tools that accept a team name MUST implement:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--team` | string | *(empty)* | Team name (resolved to ID internally) |

---

## Human-Readable Names — Never Require IDs

Administrators must never be required to supply raw internal IDs (team IDs, channel IDs, user IDs).
These are opaque, difficult to find, and error-prone to type. The tool MUST resolve human-readable
names to IDs internally via the API.

- Team names → resolved via `GET /api/v4/teams/name/{name}`
- Channel names → resolved via `GET /api/v4/teams/{team_id}/channels/name/{name}`
- Usernames → resolved via `GET /api/v4/users/username/{username}`

If a name cannot be resolved, the tool MUST exit with a clear, specific error:
```
error: team "Engineering" not found. Please check the name and try again.
```

Raw IDs MAY be accepted as a convenience for advanced users (e.g. in CSV input), but MUST NOT
be required.

---

## Output Conventions

### Streams

- **All data output** (results, reports, lists) → **stdout**
- **All log output** (verbose messages, progress, warnings) → **stderr**
- **All error messages** → **stderr**

This separation means output can be piped or redirected cleanly without log noise contaminating
the data.

### Formats

All reporting tools MUST support three output formats via `--format`:

#### `table` (default)

Human-readable, aligned columns. Use this for interactive use. Truncate very long values with
a suffix (e.g. `general, dev-backend (+3 more)`). Always include a summary line or section at
the end of the output.

#### `csv`

- Use `encoding/csv` from the standard library
- Always include a header row
- Use pipe (`|`) as a separator within multi-value fields (e.g. a list of teams within a single cell)
- Dates in ISO 8601 format (`2025-11-01T09:00:00Z`)
- Booleans as `true` / `false`
- Empty/null values as empty string (not `null`, not `N/A`)

#### `json`

- Use `encoding/json` from the standard library
- Output must be valid JSON — always verify with `jq` during development
- Use `json.MarshalIndent` for human-readable output (2-space indent)
- Dates in ISO 8601 format
- Null values as JSON `null`, not empty string
- Always include a top-level `summary` object where the tool produces aggregate counts,
  alongside the `items` / `users` / `results` array

### Dry-Run Labelling

When `--dry-run` is active, ALL output MUST be clearly labelled. In table mode, print a prominent
header:

```
⚠  DRY RUN — no changes have been made to your Mattermost instance.
```

In JSON mode, include `"dry_run": true` at the top level of the output object.

---

## Error Handling

### Exit Codes

Every tool MUST define its exit codes as named constants in `errors.go`:

```go
const (
    ExitSuccess        = 0
    ExitConfigError    = 1  // missing flags, invalid input, auth failure
    ExitAPIError       = 2  // connection failure, unexpected API response
    ExitPartialFailure = 3  // operation completed but with some failures (where applicable)
    ExitOutputError    = 4  // unable to write output file
)
```

Additional tool-specific exit codes (e.g. exit code 5 for parse failure in mm-ldap-sim) should be
added to `errors.go` with a descriptive constant name and a comment.

### Mid-Run Failures

For tools that process a list of items (users, channels, etc.):

- A failure on one item MUST NOT abort the entire run
- Failed items MUST be recorded in the results report with their error
- After processing all items, if any failures occurred, exit with `ExitPartialFailure` (3)
- Always print a summary at the end showing how many succeeded, failed, and were skipped
- Log individual item failures to stderr with `--verbose` active; suppress them without it

### API Errors

- 401 Unauthorized → `error: authentication failed. Check your token or credentials.`
- 403 Forbidden → `error: permission denied. This operation requires a System Administrator account.`
- 404 Not Found → context-specific message (e.g. `error: team "X" not found`)
- 5xx Server Error → `error: the Mattermost server returned an unexpected error (HTTP {status}). Check server logs for details.`
- Connection failure → `error: unable to connect to {url}. Check the URL and network connectivity.`

---

## API Usage

### Pagination

**All API calls that return lists MUST be paginated.** Never assume all results fit in one response.
The standard Mattermost pattern is `?page=0&per_page=200` — use `per_page=200` (the maximum) to
minimise the number of round trips.

Standard pagination loop:

```go
page := 0
perPage := 200
for {
    items, err := getPage(page, perPage)
    if err != nil {
        return err
    }
    results = append(results, items...)
    if len(items) < perPage {
        break // last page
    }
    page++
}
```

### Rate Limiting

- For tools making many sequential API calls (e.g. one per user), include a small delay between
  calls where appropriate
- For bulk-operation tools (e.g. mm-bulk-archive), implement a `--delay-ms` flag (default: 100ms)
- This is courteous to large production instances and avoids triggering rate limiting

### Authentication Headers

- Token auth: `Authorization: Bearer <token>`
- Session auth: use the session token returned by `POST /api/v4/users/login`, passed as
  `Authorization: Bearer <session_token>`

---

## Automated Testing

Testing is not optional. Every tool MUST have a meaningful test suite. The following requirements
apply to all tools in this family.

### Unit Tests

Unit tests MUST be written for all business logic. "Business logic" means anything that is not
simply wiring — flag parsing, output formatting, calculations, filtering, and decision-making
all require tests.

Specifically, unit tests MUST cover:

- **All filtering and flagging logic** — e.g. inactivity threshold calculations, version comparison,
  criteria evaluation. Test boundary conditions explicitly (e.g. exactly N days ago, not just
  clearly inside or outside the threshold).
- **Name resolution error paths** — what happens when a team name, channel name, or username
  cannot be resolved. The tool must handle these correctly; verify it does.
- **Output formatting** — CSV header rows, JSON structure, field ordering, handling of null/empty
  values. Do not assume the formatter works; test it with known input and verify the output.
- **Pagination logic** — verify that the pagination loop correctly handles: a single page of results,
  exactly `per_page` results (boundary — must fetch the next page), and an empty result set.
- **Exit code logic** — verify that the correct exit code is returned for each scenario, including
  partial failures and tool-specific codes.
- **Sensitive field handling** (mm-config-diff only) — verify that every field in the redaction
  list is correctly redacted, and that the catch-all pattern works for fields containing
  `Password`, `Secret`, `Salt`, `Key`, `Token`.

### Mocked API Responses

Unit tests MUST NOT make real network calls. All Mattermost API calls must be mockable.

Structure the code so that API interaction is abstracted behind an interface:

```go
type MattermostClient interface {
    GetUsers(page, perPage int) ([]*model.User, error)
    GetTeamByName(name string) (*model.Team, error)
    // etc.
}
```

In tests, provide a mock implementation of this interface that returns pre-defined fixture data.
This makes tests fast, deterministic, and runnable without a Mattermost instance.

Fixture data should be defined as Go structs (or small JSON files loaded at test time) in a
`testdata/` directory. Fixtures should be realistic — use plausible usernames, timestamps, and
values rather than `"test"` and `"foo"`.

### Test File Conventions

- Test files live alongside the code they test: `output_test.go` tests `output.go`
- Use the standard `testing` package — no third-party test frameworks
- Table-driven tests are strongly preferred for functions with multiple input/output cases:

```go
func TestInactivityCheck(t *testing.T) {
    tests := []struct {
        name          string
        lastLoginDays int
        threshold     int
        expectInactive bool
    }{
        {"active user, well within threshold", 5, 30, false},
        {"exactly at threshold", 30, 30, true},
        {"one day over threshold", 31, 30, true},
        {"never logged in", -1, 30, true},  // -1 represents null
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := isInactive(tt.lastLoginDays, tt.threshold)
            if result != tt.expectInactive {
                t.Errorf("isInactive(%d, %d) = %v, want %v",
                    tt.lastLoginDays, tt.threshold, result, tt.expectInactive)
            }
        })
    }
}
```

### Integration Test Notes

Full integration tests (against a real Mattermost instance) are not required as part of the
automated suite, but the README MUST include an "Integration Testing" section describing how
to run the tool manually against a local Mattermost instance for end-to-end verification.
A `docker-compose.yml` for spinning up a local Mattermost instance for testing purposes is
a welcome addition but not mandatory.

### Running Tests

The `Makefile` MUST include:

```makefile
test:
	go test ./... -v

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out
```

---

## README Requirements

The README is an **end-user guide**. The target reader is a Mattermost System Administrator
who wants to download the tool and use it. They do not have Go installed, they do not want to
build anything, and they should not need to. Do NOT include build instructions in the README —
this implies that the user must compile the tool themselves, which is contrary to the entire
point of distributing Go binaries.

Every tool MUST have a `README.md` covering the following sections, in this order:

1. **What it does** — one paragraph, plain English, no jargon
2. **Why you'd use it** — the problem it solves and who it's for
3. **Installation** — download the pre-built binary for your platform from the Releases page;
   no other steps required. Include a table of available platforms and their filenames. Remind
   the user to make the binary executable on Linux/macOS (`chmod +x`).
4. **Authentication** — explain both methods (Personal Access Token and username/password),
   with clear examples of each. Note that Personal Access Tokens may be disabled on some
   instances and that username/password is provided for those cases.
5. **Usage** — full flag reference table covering every supported flag, its environment
   variable equivalent (where applicable), default value, and description.
6. **Examples** — at minimum:
   - Basic run with token auth
   - Basic run with username/password auth (noting that the password will be prompted)
   - Using `MM_URL` and `MM_TOKEN` environment variables instead of flags
   - Writing output to a file: `--format csv --output report.csv`
   - Any tool-specific examples relevant to that tool (dry-run, team scoping, etc.)
7. **Output formats** — describe all three formats (table, CSV, JSON) with representative
   example output for each
8. **Exit codes** — table of all exit codes and their meaning, so the tool can be used
   reliably in scripts and pipelines
9. **Limitations** — any known limitations or caveats. This section is especially important
   for `mm-ldap-sim` (wrapper limitations, no group sync coverage) and `mm-plugin-audit`
   (Marketplace unreachable in air-gapped environments)
10. **Contributing** — use the standard text below
11. **License** — use the standard text below
12. **Contact** — use the standard text below

### Standard Footer Content

The following three sections MUST be included verbatim at the end of every README:

---

## Contributing

We welcome contributions from the community! Whether it's a bug report, a feature suggestion,
or a pull request, your input is valuable to us. Please feel free to contribute in the
following ways:
- **Issues and Pull Requests**: For specific questions, issues, or suggestions for improvements,
  open an issue or a pull request in this repository.
- **Mattermost Community**: Join the discussion in the
  [Integrations and Apps](https://community.mattermost.com/core/channels/integrations) channel
  on the Mattermost Community server.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contact

For questions, feedback, or contributions regarding this project, please use the following methods:
- **Issues and Pull Requests**: For specific questions, issues, or suggestions for improvements,
  feel free to open an issue or a pull request in this repository.
- **Mattermost Community**: Join us in the Mattermost Community server, where we discuss all
  things related to extending Mattermost. You can find me in the channel
  [Integrations and Apps](https://community.mattermost.com/core/channels/integrations).
- **Social Media**: Follow and message me on Twitter, where I'm
  [@jlandells](https://twitter.com/jlandells).

---

---

## Mandatory Completion Steps

These steps are NOT optional. Claude Code MUST complete ALL of the following before considering
any task done. Do not ask the user if they want these done — just do them.

1. **Create the Makefile** — every project MUST have a Makefile with at minimum the following
   targets: `build`, `build-all`, `test`, `test-cover`, `lint`. The Makefile MUST be created
   before any build is attempted.

2. **Run `make build-all`** — after writing or modifying any Go code, MUST run `make build-all`
   and show the output. A task is NOT complete until all four platform binaries have been
   successfully compiled with zero errors.

3. **Show the build output** — do not assume the build succeeded. Show the actual output of
   `make build-all` so the user can verify it.

4. **Run `make test`** — run the test suite and show the output. A task is NOT complete if
   tests are failing.

If any of these steps fail, fix the problem before reporting the task as complete. Do not
report success and then mention that the build or tests haven't been run — that is not success.



The following are explicit prohibitions. If any instruction (in a PRD, a comment, or a prompt)
asks for any of the following, do not implement it and flag the conflict:

- `--password` flag — never, under any circumstances
- Silent failure — the tool must always account for every input item in its output
- Assuming all API results fit in one page — always paginate
- Requiring raw IDs from the user — always accept names and resolve internally
- Making the tool non-functional if the `--output` file cannot be written — write to stdout
  as a fallback and print a warning to stderr
- Hardcoding the Mattermost server URL or any credentials anywhere in the code
- Logging sensitive values (tokens, passwords, secrets) to stderr even in verbose mode
