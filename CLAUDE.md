# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Portal is a command-line file transfer tool (Go). Two peers (`sender`/`receiver`) exchange a
human-readable password (e.g. `1-inertia-elliptical-celestial`), perform a PAKE2 key exchange
through a self-hosted "rendezvous" relay server, and then transfer files through that relay.
The relay never sees file contents or the plaintext password.

This is a personal fork of the upstream project, trimmed down for a single-user, self-hosted
deployment: the original P2P direct-connection path (sender/receiver probing for a LAN direct
connection before falling back to the relay) and the gzip compression of the file payload have
both been removed — see "Deviations from upstream" below.

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

CI (`.github/workflows/ci.yml`) runs `lint`, `build`, `build-wasm`, and `test` (unit tests only,
`make test`, no Docker) on every push. Go version pinned to 1.26.x. `.github/workflows/release.yml`
builds and publishes GitHub releases (Linux/macOS, amd64/arm64 only) via `.goreleaser.yml` on
`v*.*.*` tags.

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
2. **transfer protocol** — once a secure channel exists, moves the actual payload through the
   relay (`ReceiverHandshake` → `SenderHandshake` → `ReceiverRequestPayload` → payload bytes →
   `SenderPayloadSent`/`ReceiverPayloadAck` → `SenderClosing`/`ReceiverClosingAck`).

### Connection wrapping (`internal/conn`)

`Conn` is the base interface (`Read`/`Write` over a websocket, see `WS`). Two typed wrappers add
protocol-aware `ReadMsg`/`WriteMsg` helpers with expected-type validation:
- `Rendezvous` — plaintext JSON messages to/from the relay.
- `Transfer` — same, but encrypted/decrypted via `crypt.go` using the PAKE-derived session key
  (`TransferFromSession`).

### Relay server (`internal/rendezvous`)

`Server` (server.go) wires up an `http.Server` + `gorilla/mux` router (routes.go) and holds
`Mailboxes` (mailbox.go) — a `sync.Map` pairing a sender and receiver by their hashed password,
used to relay every message between the two, from the handshake through the entire file
transfer. `handlers.go` implements `/establish-sender` and `/establish-receiver`; `id.go`
allocates the numeric ID that seeds the generated password. Serves an embedded landing page
(`templates/`) and a `/version` endpoint checked by clients against `internal/semver` for
compatibility.

### Clients (`internal/sender`, `internal/receiver`)

Each package is a single file implementing the same three-step sequence, called from
`internal/portal`: `ConnectRendezvous` (dial the relay, get an ID, generate+hash the password) →
`SecureConnection` (PAKE2 exchange, derive session key/salt) → `Transfer`/`Receive` (the
payload, always relayed through the rendezvous connection).

### `internal/portal`

Public `Send`/`Receive` functions that merge a partial `Config` over `defaultConfig` and drive
the client packages above. This is the shared entry point used by both the CLI and the WASM
build — role-specific protocol logic belongs in `internal/sender`/`internal/receiver`, not here.

### Supporting packages

- `internal/password` — generates/validates the human-readable password from `data/words.go`.
- `internal/file` — tars (uncompressed) files/directories for send (`PackFiles`, resolving
  symlinks), and streams-unpacks on receive (`Unpacker`/`Committer`), optionally prompting
  before overwriting existing files.
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
reusing `internal/portal`/`internal/receiver` as-is — no platform-specific build tags remain in
those packages since the transfer path is relay-only on every platform.

## Deviations from upstream

This fork intentionally removed two things present in the original project:

- **P2P direct transfer**: upstream had the sender spin up a local websocket server advertising
  its LAN IP/port, and the receiver would probe it with a short backoff before falling back to
  the relay. Since this fork always runs against a self-hosted relay that both peers can reach
  but that can't reach each other directly, that probing was pure added latency. It's gone from
  the transfer protocol, the clients, and the TUI (no more "direct vs relayed" messaging).
- **gzip compression**: upstream tarred *and* gzip-compressed (`pgzip`, parallel gzip) the
  payload before sending. Compression cost was dominating transfer time for already-compressed
  files (video, archives, etc.), so `internal/file.PackFiles`/`Unpacker` now only tar — no
  compression layer — preserving directory structure and multi-file support without the CPU
  overhead.
