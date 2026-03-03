# mm-guest-convert

## What It Does

`mm-guest-convert` converts a regular Mattermost member to a guest account and restricts their
channel access to a single specified channel. It automates the full workflow in a single,
idempotent command: demote the user to guest, ensure they are a member of the target channel,
and remove them from every other channel across all teams on the instance. Direct Message and
Group Message channels are left untouched.

## Why You'd Use It

When Mattermost demotes a user to guest via the API or System Console, the guest retains all of
their existing channel memberships. Administrators must then manually remove the user from every
other channel — which is impractical when a user belongs to dozens or hundreds of channels.

This tool solves that problem by performing the entire conversion in one command. It is designed
for System Administrators who need to convert existing member accounts to guest accounts and
restrict their access to a specific channel — for example, when onboarding external contractors
or restricting partner access.

## Installation

Download the pre-built binary for your platform from the
[Releases](https://github.com/jlandells/mm-guest-convert/releases) page.

| Platform            | Filename                            |
|---------------------|-------------------------------------|
| Linux (amd64)       | `mm-guest-convert_linux_amd64`      |
| Linux (arm64)       | `mm-guest-convert_linux_arm64`      |
| macOS (Apple)       | `mm-guest-convert_macos_apple`      |
| macOS (Intel)       | `mm-guest-convert_macos_intel`      |
| Windows             | `mm-guest-convert_windows.exe`      |

On Linux and macOS, make the binary executable after downloading:

```bash
chmod +x mm-guest-convert-*
```

No other installation steps are required.

## Authentication

The tool requires System Administrator credentials to perform the conversion. Two authentication
methods are supported.

### Personal Access Token (Recommended)

Pass your token via the `--token` flag or the `MM_TOKEN` environment variable:

```bash
mm-guest-convert --url https://mattermost.example.com --token YOUR_TOKEN \
  -u jsmith -t acme-corp -c project-alpha
```

The token must belong to a System Administrator account. Personal Access Tokens can be created
in **System Console > Integrations > Integration Management**, or per-user in
**Profile > Security > Personal Access Tokens**.

### Username and Password

If Personal Access Tokens are disabled on your instance, use username/password authentication:

```bash
mm-guest-convert --url https://mattermost.example.com --username admin \
  -u jsmith -t acme-corp -c project-alpha
```

You will be prompted to enter your password securely (input is hidden). For non-interactive or
automation scenarios, set the `MM_PASSWORD` environment variable instead:

```bash
export MM_PASSWORD="your-password"
mm-guest-convert --url https://mattermost.example.com --username admin \
  -u jsmith -t acme-corp -c project-alpha
```

> **Note:** There is no `--password` flag. Passwords passed as CLI flags are visible in shell
> history and process listings, which is a security risk.

## Finding Internal Names

`--team` and `--channel` require the **internal name** (also called the URL name), **not** the
display name. These are different in Mattermost:

```
Display name:  "Engineering Team"      ← what you see in the UI
Internal name: "engineering-team"      ← what this tool requires
```

### Finding the Internal Team Name

The team name appears in the URL after your server address:

```
https://mattermost.example.com/engineering-team/channels/town-square
                               ^^^^^^^^^^^^^^^^
```

Or: **System Console > User Management > Teams** — the team URL shows the internal name.

### Finding the Internal Channel Name

The channel name is the final segment of the channel URL:

```
https://mattermost.example.com/engineering-team/channels/project-alpha
                                                         ^^^^^^^^^^^^^
```

Or: **System Console > User Management > Channels** — select the channel and look at the
**Channel URL** field.

### Common Mistakes

- Using `"Engineering Team"` instead of `"engineering-team"` — display names contain spaces and
  capital letters; internal names are lowercase with hyphens.
- Using the channel display name `"General"` instead of the internal name `"town-square"` — the
  default channel's display name and internal name are different on many instances.

## Usage

```
mm-guest-convert [flags]
```

### Connection and Authentication Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--url` | `MM_URL` | *(required)* | Mattermost server URL |
| `--token` | `MM_TOKEN` | *(empty)* | Personal Access Token (preferred) |
| `--username` | `MM_USERNAME` | *(empty)* | Username for password auth (fallback) |

### Required Operation Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--username-target` | `-u` | Mattermost username of the user to convert |
| `--team` | `-t` | Internal team name containing the target channel (required unless `--keep-all-channels`) |
| `--channel` | `-c` | Internal channel name the user should retain (required unless `--keep-all-channels`) |

> **Note:** `--team` and `--channel` are **mutually exclusive** with `--keep-all-channels`.
> Using them together will produce an error. Either specify `--team` and `--channel` to restrict
> the user to a single channel, or use `--keep-all-channels` to demote without changing channel
> memberships — but not both.

### Optional Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--keep-all-channels` | `false` | Only demote the user to guest; do not remove any channel memberships (mutually exclusive with `--team` and `--channel`) |
| `--restrict-to-team` | `false` | Remove user from all teams except the target team, then clean up channels (mutually exclusive with `--keep-all-channels`) |
| `--dry-run` | `false` | Preview all actions without making any changes |
| `--workers` | `10` | Number of concurrent workers for channel removal |
| `--format` | `table` | Output format: `table`, `csv`, `json` |
| `--output` | *(stdout)* | Write output to a file |
| `--verbose` / `-v` | `false` | Enable verbose logging to stderr |
| `--version` | | Print version and exit |

## Examples

### Basic Run with Token Auth

```bash
mm-guest-convert --url https://mattermost.example.com --token YOUR_TOKEN \
  -u jsmith -t acme-corp -c project-alpha
```

### Basic Run with Username/Password Auth

```bash
mm-guest-convert --url https://mattermost.example.com --username admin \
  -u jsmith -t acme-corp -c project-alpha
Password: ********
```

### Using Environment Variables

```bash
export MM_URL=https://mattermost.example.com
export MM_TOKEN=YOUR_TOKEN

mm-guest-convert -u jsmith -t acme-corp -c project-alpha
```

### Dry-Run Mode

Preview what would happen without making any changes:

```bash
mm-guest-convert --url https://mattermost.example.com --token YOUR_TOKEN \
  -u jsmith -t acme-corp -c project-alpha --dry-run
```

### Demote Without Removing Channels

Demote a user to guest but leave all their existing channel memberships intact:

```bash
mm-guest-convert --url https://mattermost.example.com --token YOUR_TOKEN \
  -u jsmith --keep-all-channels
```

This is useful when you want to restrict a user's role without changing their channel access.
The `--team` and `--channel` flags are not required (and cannot be used) with
`--keep-all-channels`.

### Restrict to a Single Team

Demote the user to guest, remove them from all teams except the target team, and restrict their
channel access to a single channel:

```bash
mm-guest-convert --url https://mattermost.example.com --token YOUR_TOKEN \
  -u jsmith -t acme-corp -c project-alpha --restrict-to-team
```

This is useful when a user belongs to multiple teams and you want to fully isolate them to a
single team and channel in one command. Removing a team membership implicitly removes all of
that team's channel memberships, so this approach is more efficient on large instances.

### Writing Output to a File

```bash
mm-guest-convert --url https://mattermost.example.com --token YOUR_TOKEN \
  -u jsmith -t acme-corp -c project-alpha --format csv --output report.csv
```

### JSON Output

```bash
mm-guest-convert --url https://mattermost.example.com --token YOUR_TOKEN \
  -u jsmith -t acme-corp -c project-alpha --format json
```

## Output Formats

### Table (Default)

Human-readable, step-by-step output:

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

### CSV

One row per action, suitable for import into spreadsheets:

```csv
username,team_name,channel_name,action,status
jsmith,acme-corp,project-alpha,retained,ok
jsmith,acme-corp,town-square,removed,ok
jsmith,acme-corp,off-topic,removed,ok
```

### JSON

Structured output for scripting and automation:

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

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (or dry run completed successfully) |
| 1 | Configuration error — missing flags, invalid input, auth failure, user/team/channel not found, guest accounts disabled |
| 2 | API error — connection failure, unexpected server response, mid-enumeration failure |
| 3 | Partial failure — operation completed but one or more channel removals failed |
| 4 | Output error — unable to write to the specified output file |

## Limitations

- **Guest accounts require a licence.** The Guest Access feature must be enabled on your
  Mattermost instance. If it is not, the tool will exit with a clear error directing you to
  **System Console > Authentication > Guest Access**.
- **Performance on very large instances.** If a user belongs to thousands of channels, the
  enumeration phase (fetching channel details) may take some time. The `--workers` flag controls
  the number of concurrent removal operations (default: 10). Increase it for faster processing
  on instances that can handle higher API concurrency, or decrease it if you encounter rate
  limiting.
- **Team memberships are not modified by default.** Without `--restrict-to-team`, the tool only
  manages channel memberships. The user will remain a member of any teams they were previously
  on. Use `--restrict-to-team` to also remove the user from all teams except the target team.
- **DM and GM channels are excluded.** Direct Message and Group Message channels are silently
  skipped and never included in any removal operation.

## Integration Testing

To test the tool end-to-end against a local Mattermost instance:

1. Start a local Mattermost server (e.g. via Docker).
2. Create a test user, team, and channels. Add the test user to several channels.
3. Create a Personal Access Token for a System Administrator account.
4. Run the tool in dry-run mode first to verify the plan:
   ```bash
   ./mm-guest-convert --url http://localhost:8065 --token YOUR_TOKEN \
     -u testuser -t test-team -c target-channel --dry-run
   ```
5. Once satisfied, run without `--dry-run` to execute the conversion.
6. Verify in the System Console that the user is now a guest and is only a member of the
   target channel (plus any DM/GM channels).

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
