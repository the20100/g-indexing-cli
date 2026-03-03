# g-indexing — Google Indexing API CLI

A fast, agent-friendly CLI for the [Google Indexing API v3](https://developers.google.com/search/apis/indexing-api/v3/using-api).

Notifies Google Search when your pages are **created/updated** or **deleted**, and queries indexing notification status.

Outputs **JSON when piped** (for scripting/agents) and **human-readable tables** in a terminal.

> **Note:** The Indexing API is currently restricted to **job postings** and **livestream event pages**, or pages on properties verified in Search Console where the service account is an owner.

---

## Install

```bash
git clone https://github.com/the20100/g-indexing-cli
cd g-indexing-cli
go build -o g-indexing .
mv g-indexing /usr/local/bin/
```

## Update

```bash
g-indexing update
```

---

## Authentication

Two auth methods are supported.

### Option A — Service Account (recommended for automation)

1. Create a service account in [Google Cloud Console](https://console.cloud.google.com/iam-admin/serviceaccounts)
2. Download the JSON key file
3. In Search Console, add the service account email as an **Owner** of each property

```bash
g-indexing auth setup --service-account /path/to/sa.json

# Or set the env var (no config file needed)
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa.json
```

### Option B — OAuth2 (for manual/interactive use)

1. Create OAuth2 credentials (Desktop app) at [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Add `http://localhost:8080` as an authorized redirect URI
3. Download `credentials.json`

```bash
g-indexing auth setup --credentials /path/to/credentials.json
# or
g-indexing auth setup --client-id <id> --client-secret <secret>

# On a remote server (VPS) where no browser is available:
g-indexing auth setup --credentials /path/to/credentials.json --no-browser
```

With `--no-browser`: the CLI prints the OAuth URL. Open it in a local browser, authorize, then copy the full redirect URL from the address bar and paste it into the terminal (the page will fail to load — that's expected).

Config stored at:
- macOS: `~/Library/Application Support/g-indexing/config.json`
- Linux: `~/.config/g-indexing/config.json`

---

## Commands

### `g-indexing info`
Show binary location, config path, and auth status.

### `g-indexing auth setup / status / logout`
Manage authentication (service account or OAuth2).

---

### `g-indexing notify update <url> [url2 ...]`
Notify Google that one or more URLs have been **created or updated**.

```bash
g-indexing notify update https://example.com/new-page
g-indexing notify update https://example.com/p1 https://example.com/p2
g-indexing notify update https://example.com/page --json
```

### `g-indexing notify delete <url> [url2 ...]`
Notify Google that one or more URLs have been **permanently removed**.

```bash
g-indexing notify delete https://example.com/old-page
g-indexing notify delete https://example.com/p1 https://example.com/p2
```

### `g-indexing notify batch <file>`
Batch notify from a file (or stdin with `-`).

**File format** (one entry per line):
```
# Comments and blank lines are ignored
update https://example.com/new-page
delete https://example.com/removed-page
https://example.com/another-page       # bare URL defaults to update
```

```bash
g-indexing notify batch urls.txt
g-indexing notify batch urls.txt --concurrency 10
echo "update https://example.com/page" | g-indexing notify batch -
```

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--concurrency` | `5` | Parallel API requests (1–20) |

---

### `g-indexing metadata get <url>`
Get the latest indexing notification metadata for a URL.

```bash
g-indexing metadata get https://example.com/page
g-indexing metadata get https://example.com/page --json
```

Returns the timestamp and type of the most recent `URL_UPDATED` and `URL_DELETED` notifications sent for that URL.

> Only works for URLs previously notified via the Indexing API.

---

## Global Flags

| Flag | Description |
|---|---|
| `--json` | Force JSON output |
| `--pretty` | Force pretty-printed JSON (implies `--json`) |

---

## Tips

- **Quota**: the Indexing API allows ~200 requests/day and 100 requests/minute by default. Use `--concurrency` conservatively on large batches.
- **Service account ownership**: the service account email must be added as a **verified owner** (not just a user) in Search Console for each property.
- **Batch from CI**: combine with `GOOGLE_APPLICATION_CREDENTIALS` for zero-config use in CI pipelines.
  ```bash
  export GOOGLE_APPLICATION_CREDENTIALS=/secrets/sa.json
  g-indexing notify batch new-urls.txt --concurrency 8 --json | jq '.[] | select(.success == false)'
  ```
- **Piped output**: JSON is emitted automatically when stdout is not a TTY.
