# bluebird

An agent-friendly command-line client for [Bluebird](https://bluebird.caozc.top), a weather-discovery service that finds "good weather moments" — sunset glow, crystal-clear skies, starry nights, rainbows, and nine other photogenic atmospheric scenes — rather than just reporting temperature and rain chance.

`bluebird` is designed for two audiences at once:

- **Humans**, via readable table/text output and interactive prompts.
- **AI agents / LLM tool-use**, via a stable `--json` envelope, fine-grained exit codes, `--dry-run` for every write operation, and offline `describe`/`doctor`/`version` commands for self-diagnosis.

It talks to the public Bluebird API at `https://bluebird.caozc.top` by default. No account signup flow is bundled — you provide your own bearer token or a JWT signing secret via environment variables (see [Authentication](#authentication)).

## Install

**Homebrew (macOS/Linux):**

```sh
brew tap x0c/tap
brew install --cask x0c/tap/bluebird
```

**One-line install script:**

```sh
curl -fsSL https://raw.githubusercontent.com/x0c/bluebird-cli/main/install.sh | bash
```

**Go install:**

```sh
go install github.com/x0c/bluebird-cli/cmd/bluebird@latest
```

## Quick start

```sh
# Check configuration and connectivity (works offline with --no-probe)
bluebird doctor

# What's the sky doing right now near a place name?
bluebird conditions now --place "Hangzhou"

# Look ahead 3 days for recommendable windows
bluebird conditions windows --place "Hangzhou" --hours 72

# Manage a persistent watch (location × time × scene subscription)
bluebird watch create --name "West Lake sunset" --place "杭州西湖" --scenes sunset_glow
bluebird watch overview
bluebird watch delete <id>
```

Add `--json` for machine-readable output (or just pipe the output — non-TTY stdout auto-switches to JSON).

## Command reference

```
bluebird conditions now|windows [--place <name> | --lat --lon [--elevation]] [--hours N]
bluebird place search <query>
bluebird watch list|overview|get|outlook|climate <id>
bluebird watch create|update|delete   [--dry-run]
bluebird astro calendar --place <name> [--days N]
bluebird air current|latest|stations
bluebird doctor [--no-probe]
bluebird describe [command]
bluebird version
```

Run `bluebird describe --json` for a machine-readable dump of every command, flag, exit code, and required environment variable — generated directly from the command tree, so it never drifts from actual behavior.

## Output contract

Every command supports `--json`, which is also auto-enabled whenever stdout is not a terminal (piped or redirected). The envelope shape is stable across releases (fields are only ever added, never removed or repurposed):

```json
{
  "ok": true,
  "data": { "...": "..." },
  "error": null,
  "meta": { "duration_ms": 123, "version": "0.1.1" }
}
```

On failure, `data` is `null` and `error` is populated — and this is still written to **stdout** as valid JSON, alongside a non-zero exit code:

```json
{
  "ok": false,
  "data": null,
  "error": {
    "code": "place_ambiguous",
    "message": "地名「中山」有多个匹配结果，无法确定唯一地点",
    "hint": "从候选中选一个精确坐标，或加 --first 取第一个",
    "next_commands": ["--lat 22.52 --lon 113.39 (中山市, 广东省, 中国)", "..."],
    "retryable": false,
    "details": { "candidates": [ "..." ] }
  },
  "meta": { "duration_ms": 448, "version": "0.1.1" }
}
```

`error.message` is Chinese (matching the upstream API and the CLI's primary user base); `error.code` is a stable English short code safe to branch on.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success (including idempotent "unchanged") |
| 1 | General failure (5xx / network error / upstream rate limit / feature disabled) |
| 2 | Usage error (bad CLI arguments, or backend 400) |
| 3 | Not found (404, zero place-search results, no location on file) |
| 4 | Auth failure (401/403, or no credentials configured) |
| 5 | Conflict (ambiguous place name, or a watch with the same name+location already exists with different fields) |
| 6 | Timeout |

### Idempotent writes

`watch create` is declarative: creating a watch with the same name and location (rounded to ~110m) twice converges to `"status": "unchanged"` on the second call instead of erroring or duplicating. `watch delete` on an already-deleted ID likewise converges to `"unchanged"`. Every write command supports `--dry-run`, which performs zero writes and reports what *would* happen with the same data shape as a real run.

## Authentication

Credentials are read from environment variables only — never from flags, never written to disk, never echoed in logs or error output.

| Variable | Purpose |
|---|---|
| `BLUEBIRD_TOKEN` | A ready-made bearer token, used as-is |
| `BLUEBIRD_JWT_SECRET` | HS256 signing secret; the CLI self-signs a short-lived (5 min) token |
| `BLUEBIRD_USER_ID` | Subject (user UUID) for the self-signed token |
| `BLUEBIRD_JWT_ISSUER` | Overrides the default token issuer |
| `BLUEBIRD_BASE_URL` | Overrides the default `https://bluebird.caozc.top` |

`BLUEBIRD_TOKEN` takes priority if set; otherwise `BLUEBIRD_JWT_SECRET` + `BLUEBIRD_USER_ID` are used to self-sign a token. Run `bluebird doctor` to check which path is active and whether the backend is reachable — it never prints the secret itself, only "configured" / "not configured".

## Using it from an agent

`bluebird` is built to be driven by an LLM tool-use loop: deterministic JSON, self-describing (`describe`), self-diagnosing (`doctor`), and safe to retry (idempotent writes, `--dry-run`). A typical integration adds the binary to an agent's allowed command list and points it at `bluebird describe --json` once to learn the full command surface, then calls subcommands with `--json` as needed.

## Development

```sh
go build ./...
go vet ./...
go test -race ./...
golangci-lint run ./...
```

`scripts/acceptance.sh` runs the full behavioral acceptance checklist (non-interactive behavior, envelope validity, exit codes, idempotency, dry-run safety) against a real Bluebird backend — see the script header for usage.

## License

MIT — see [LICENSE](LICENSE).
