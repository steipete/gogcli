# 📮 gog — Google in your terminal

Minimal Google CLI in Go for:

- Gmail
- Calendar
- Drive
- Contacts (People API)
- Tasks
- People (profile / `people/me`)

## Install / Build

Install via Homebrew (tap):

- `brew install steipete/tap/gogcli`

Build locally:

- `make`

Run:

- `./bin/gog --help`

## Setup (OAuth)

Before adding an account you need OAuth2 credentials from Google Cloud Console:

1. Create a project (or select an existing one): https://console.cloud.google.com/projectcreate
2. Enable the APIs you need:
   - Gmail API: https://console.cloud.google.com/apis/api/gmail.googleapis.com
   - Google Calendar API: https://console.cloud.google.com/apis/api/calendar-json.googleapis.com
   - Google Drive API: https://console.cloud.google.com/apis/api/drive.googleapis.com
   - People API (Contacts): https://console.cloud.google.com/apis/api/people.googleapis.com
   - Google Tasks API: https://console.cloud.google.com/apis/api/tasks.googleapis.com
3. Set app name / branding (OAuth consent screen): https://console.cloud.google.com/auth/branding
4. If your app is in “Testing”, add test users (all Google accounts you’ll use with `gog`): https://console.cloud.google.com/auth/audience
5. Create an OAuth client: https://console.cloud.google.com/auth/clients
   - Click “Create Client”
   - Application type: “Desktop app”
   - Download the JSON file (usually named like `client_secret_....apps.googleusercontent.com.json`)

Then:

- Store the downloaded client JSON (no renaming required):
  - `gog auth credentials ~/Downloads/client_secret_....json`
- Authorize your account (refresh token stored in OS keychain via `github.com/99designs/keyring`):
  - `gog auth add you@gmail.com`

Notes:

- If no OS keychain backend is available (e.g. Linux/WSL/container), keyring can fall back to an encrypted on-disk store and may prompt for a password; for non-interactive runs set `GOG_KEYRING_PASSWORD`.
- Default is `--services all` (gmail, calendar, drive, contacts, tasks).
- To request fewer scopes: `gog auth add you@gmail.com --services drive,calendar`.
- If you add services later and Google doesn’t return a refresh token, re-run with `--force-consent`.
- `gog auth add ...` overwrites the stored token for that email.

## Accounts

Most API commands require an account selection:

- `--account you@gmail.com`
- or set `GOG_ACCOUNT=you@gmail.com` to avoid repeating the flag.

List configured accounts:

- `gog auth list`

## Output (Parseable)

- `--output=text` (default): plain text on stdout (lists are tab-separated).
- `--output=json`: JSON on stdout (best for scripting).
- Human-facing hints/progress go to stderr.
- Colors are enabled only in rich TTY output and are disabled automatically for JSON.

Useful pattern:

- `gog --output=json ... | jq .`

If you use `pnpm`, see the shortcut section for `pnpm -s` (silent) to keep stdout clean.

## Examples

Drive:

- `gog drive ls --max 20`
- `gog drive search "invoice" --max 20`
- `gog drive get <fileId>`
- `gog drive download <fileId>`
- `gog drive upload ./path/to/file --folder <folderId>`

Calendar:

- `gog calendar calendars`
- `gog calendar events <calendarId> --from 2025-12-08T00:00:00+01:00 --to 2025-12-15T00:00:00+01:00 --max 250`
- `gog calendar event <calendarId> <eventId>`
- `gog calendar respond <calendarId> <eventId> --status accepted`

Gmail:

- `gog gmail search 'newer_than:7d' --max 10`
- `gog gmail thread <threadId>`
- `gog gmail get <messageId> --format metadata`
- `gog gmail attachment <messageId> <attachmentId> --out ./attachment.bin`
- `gog gmail labels list`
- `gog gmail labels get INBOX --output=json` (includes counts)
- `gog gmail send --to a@b.com --subject "Hi" --body "Plain fallback" --body-html "<p>Hello</p>"`

Contacts:

- `gog contacts list --max 50`
- `gog contacts search "Ada" --max 50`
- `gog contacts get people/...`
- `gog contacts other list --max 50`

Tasks:

- `gog tasks lists --max 50`
- `gog tasks lists create <title>`
- `gog tasks list <tasklistId> --max 50`
- `gog tasks add <tasklistId> --title "Task title"`
- `gog tasks update <tasklistId> <taskId> --title "New title"`
- `gog tasks done <tasklistId> <taskId>`
- `gog tasks undo <tasklistId> <taskId>`
- `gog tasks delete <tasklistId> <taskId>`
- `gog tasks clear <tasklistId>`

Workspace directory (requires Google Workspace account; `@gmail.com` won’t work):

- `gog contacts directory list --max 50`
- `gog contacts directory search "Jane" --max 50`

People:

- `gog people me`

## Environment

- `GOG_ACCOUNT=you@gmail.com` (used if `--account` is omitted)
- `GOG_COLOR=auto|always|never` (default `auto`)
- `GOG_OUTPUT=text|json` (default `text`)

## Development

Pinned tools (installed into `.tools/`):

- Format: `make fmt` (goimports + gofumpt)
- Lint: `make lint` (golangci-lint)
- Test: `make test`

CI runs format checks, tests, and lint on push/PR.

### `pnpm gog` shortcut

Build + run in one step:

- `pnpm gog auth add you@gmail.com`

For clean stdout when scripting:

- `pnpm -s gog --output=json gmail search "from:me" | jq .`

## Credits

This project is inspired by Mario Zechner’s original CLIs:

- [`gmcli`](https://github.com/badlogic/gmcli)
- [`gccli`](https://github.com/badlogic/gccli)
- [`gdcli`](https://github.com/badlogic/gdcli)
