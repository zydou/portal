# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Portal is a command-line file transfer tool (Go). Two peers (`sender`/`receiver`) exchange a
human-readable password (e.g. `1-inertia-elliptical-celestial`), perform a PAKE2 key exchange
through a public "rendezvous" relay server, and then transfer files either directly (P2P, e.g.
same LAN) or through the relay if direct connection isn't possible. The relay never sees file
contents or the plaintext password.

## Common commands

```bash
# Build the CLI (PORTAL_VERSION is a semver string, injected via -ldflags; required)
PORTAL_VERSION=v1.x.x make build

# Build the WASM receiver used by the relay's web page
make build-wasm

# Lint (matches CI: golangci-lint with misspell enabled)
make lint

# Unit tests (race detector, -short, coverage)
make test

# Full test suite including Docker-based e2e tests (requires Docker; builds the relay image first)
make test-e2e

# Run a single test
go test ./internal/semver/... -run TestSomething -v -race

# Build/run the relay server in Docker
make image   # docker build --tag rendezvous:latest
make serve   # image + docker run -dp 8080:8080
```

CI (`.github/workflows/ci.yml`) runs `lint`, `build`, `build-wasm`, and `test` on every push;
`test-e2e` runs instead of `test` for pull requests. Go version pinned to 1.20.x.

## Architecture

The codebase is split into a **protocol layer** (wire messages only, no logic) and packages that
implement the two roles that speak that protocol: the **rendezvous relay server** and the
**sender/receiver clients**. `internal/portal` is the thin public API tying client-side steps
together; `cmd/portal` and `cmd/wasm` are the two entry points that consume it.

### Protocol layer (`protocol/rendezvous`, `protocol/transfer`)

Pure message-type definitions shared by client and server code, each with a `Msg`/`Payload`
struct and an `Error{Expected, Got}` for mismatched message types. Two distinct protocols run
back-to-back over the same websocket connection:

1. **rendezvous protocol** — establishes the connection through the relay and performs the PAKE2
   handshake (password hashing, PAKE bytes, salt exchange) to derive a shared session key.
2. **transfer protocol** — once a secure channel exists, negotiates direct-vs-relay transfer and
   moves the actual payload.

### Connection wrapping (`internal/conn`)

`Conn` is the base interface (`Read`/`Write` over a websocket, see `WS`). Two typed wrappers add
protocol-aware `ReadMsg`/`WriteMsg` helpers with expected-type validation:
- `Rendezvous` — plaintext JSON messages to/from the relay.
- `Transfer` — same, but encrypted/decrypted via `crypt.go` using the PAKE-derived session key
  (`TransferFromSession`) or a raw key (`TransferFromKey`, used once a direct connection exists).

### Relay server (`internal/rendezvous`)

`Server` (server.go) wires up an `http.Server` + `gorilla/mux` router (routes.go) and holds
`Mailboxes` (mailbox.go) — a `sync.Map` pairing a sender and receiver by their hashed password,
used to relay messages between the two during the handshake and, if direct transfer isn't
possible, for the whole transfer. `handlers.go` implements `/establish-sender` and
`/establish-receiver`; `id.go` allocates the numeric ID that seeds the generated password.
Serves an embedded landing page (`templates/`) and a `/version` endpoint checked by clients
against `internal/semver` for compatibility.

### Clients (`internal/sender`, `internal/receiver`)

Each package mirrors the same three-step sequence, called from `internal/portal`:
`ConnectRendezvous` (dial the relay, get an ID, generate+hash the password) →
`SecureConnection` (PAKE2 exchange, derive session key/salt) → `Transfer`/`Receive` (the payload).

Direct-vs-relay is negotiated in the transfer step: the sender always spins up a local
websocket server (`sender/server.go`, started in `transfer.go`) advertising its LAN IP/port;
the receiver tries to dial it directly with a short linear backoff (`probeSender` in
`receive.go`) and falls back to relaying through the rendezvous connection if that fails.

**Build-tag platform split**: `transfer.go`/`receive.go` (tag `//go:build !js`) implement the
native, direct-capable path; `transfer_wasm.go`/`receive_wasm.go` implement the `GOOS=js` (WASM,
browser) path, which is always relay-only since a browser can't open a listening TCP server.
When touching sender/receiver transfer logic, check both variants stay consistent.

### `internal/portal`

Public `Send`/`Receive` functions that merge a partial `Config` over `defaultConfig` and drive
the client packages above. This is the shared entry point used by both the CLI and the WASM
build — role-specific protocol logic belongs in `internal/sender`/`internal/receiver`, not here.

### Supporting packages

- `internal/password` — generates/validates the human-readable password from `data/words.go`.
- `internal/file` — tars + parallel-gzips (`pgzip`) files/directories for send (`PackFiles`,
  resolving symlinks), and streams-unpacks on receive (`Unpacker`/`Committer`), optionally
  prompting before overwriting existing files.
- `internal/semver` — version comparison used to reject transfers between incompatible major
  versions of sender/receiver/relay.
- `internal/logger` — `zap`-based logger and HTTP middleware for the relay server.

### CLI (`cmd/portal`)

Cobra command tree built in `main.go` (`send`, `receive`, `serve`, `version`, `config`), backed
by a Viper config (`cmd/portal/config`) stored at `$HOME/.config/portal/config.yml` and merged
with defaults defined there. `cmd/portal/tui` holds the Bubble Tea UI (separate sender/receiver
views, a shared file table and transfer-progress component); the `-s/--tui-style` flag switches
between `rich` and `raw` rendering.

### WASM (`cmd/wasm`)

Builds a browser-side receiver (`GOOS=js GOARCH=wasm`) served from the relay's landing page,
reusing `internal/portal`/`internal/receiver` with the WASM-tagged transfer files.
