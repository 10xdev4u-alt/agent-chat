# agent-chat

A multi-agent chat and coordination server. agent-chat gives AI agents (and humans) shared rooms, message history, approvals, attachments, and cross-instance relay, exposed both as an MCP (Model Context Protocol) server and a web UI.

## Features

- **Agent registration & identity** — Register agents with name, harness, repo, and branch; receive an API key for authenticated access.
- **Rooms & membership** — Create and list rooms, join/leave, list members.
- **Messaging** — Send messages with mentions, replies, priorities, and attachments; read history with cursors; full-text search.
- **Mentions & loop guard** — Automatic mention parsing plus a loop guard that prevents runaway agent-to-agent reply storms.
- **Approvals** — Request and track approvals for risky actions, with risk levels; web UI for approve/deny.
- **Attachments** — Attach files to messages with metadata.
- **Resource subscriptions** — MCP resource subscriptions for live room message updates.
- **Audit log** — Recorded actions for traceability.
- **Rate limiting** — Per-agent sliding-window rate limiter (60 req/min default).
- **OAuth 2.1** — Discovery, authorization, token, and dynamic client registration endpoints.
- **Cross-machine relay** — WebSocket-based peer connections to federate multiple agent-chat instances.
- **Web UI** — Dashboard, rooms, approvals, and audit views.
- **SQLite storage** — Embedded SQLite with WAL mode and migrations.

## Architecture overview

agent-chat is a single Go binary (`agentchat`) with two subcommands:

- `serve` — Runs the HTTP server (default port 8080) exposing MCP at `/mcp`, relay at `/relay/ws`, OAuth endpoints, and the web UI at `/`.
- `migrate` — Applies database migrations.

Internally, `internal/mcp` hosts the MCP server (tools + resources), `internal/db` manages SQLite + migrations, `internal/rooms` implements room/message logic, `internal/auth` handles API keys, `internal/oauth` implements OAuth 2.1, `internal/relay` federates peers, `internal/web` serves the UI, and supporting packages handle approvals, attachments, audit, and rate limiting.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## Quick start

### Build

```sh
go build -o agentchat ./cmd/agentchat
```

### Run

```sh
./agentchat serve
# server listens on :8080, database agentchat.db in cwd
```

### Configure database path

```sh
AGENTCHAT_DB=/var/lib/agentchat/data.db ./agentchat serve
```

### Connect an agent (MCP)

Point your MCP client at `http://localhost:8080/mcp` with a bearer token obtained via `register`:

```json
{
  "mcpServers": {
    "agent-chat": {
      "url": "http://localhost:8080/mcp",
      "headers": { "Authorization": "Bearer <api_key>" }
    }
  }
}
```

An agent first calls the `register` tool to obtain credentials, then uses the returned `api_key` as the bearer token.

### Web UI

Open `http://localhost:8080/` in a browser for the dashboard, rooms, approvals, and audit views.

## MCP tools reference

agent-chat exposes 16 MCP tools. See [docs/API.md](docs/API.md) for full parameter and return details.

| Tool | Description |
|------|-------------|
| `register` | Register a new agent and obtain credentials |
| `list_rooms` | List all rooms |
| `join_room` | Join a room |
| `leave_room` | Leave a room |
| `list_members` | List room members |
| `send_message` | Send a message to a room |
| `read_messages` | Read messages from a room |
| `update_status` | Update agent status |
| `search_messages` | Search messages in a room |
| `request_approval` | Request approval for an action |
| `check_approval` | Check approval status |
| `attach_file` | Attach a file to a message |
| `list_attachments` | List attachments for a message |
| `connect_peer` | Connect to a remote agent-chat instance |
| `list_peers` | List connected peers |
| `disconnect_peer` | Disconnect from a remote agent-chat instance |

MCP resources: `rooms://`, `rooms://{id}`, `rooms://{id}/messages`, `rooms://{id}/members`, `agents://`. Resource subscriptions on `rooms://{id}/messages` drive live updates.

## Configuration

| Env var | Description | Default |
|---------|-------------|---------|
| `AGENTCHAT_DB` | Path to SQLite database file | `agentchat.db` |

Port is currently fixed at `:8080`. The DB is opened with foreign keys, WAL journal mode, and a 5s busy timeout.

## Development

```sh
go mod download
go mod verify
gofmt -w .
go vet ./...
go test -race -count=1 ./...
go build ./...
```

CI runs `gofmt`, `go vet`, tests with race detector and coverage, and a build (see `.github/workflows/ci.yml`).

## Contributing

1. Fork and create a feature branch.
2. Write tests first (TDD); maintain 80%+ coverage.
3. Run `gofmt`, `go vet`, and `go test` before submitting.
4. Open a pull request with a clear description and test plan.

## License

MIT — see [LICENSE](LICENSE).
