# tug

[![Sponsor](https://img.shields.io/badge/Sponsor-❤-ea4aaa?style=flat-square&logo=github)](https://github.com/sponsors/mickamy)

Docker Compose with auto-routing. HTTP services get `*.localhost` URLs via Traefik, TCP services (databases, etc.) get
deterministic port mappings. Works great with git worktrees.

![demo](docs/demo.gif)

## The problem

When running multiple Docker Compose projects (or the same project in multiple worktrees), port conflicts are
inevitable. You end up manually juggling port numbers or stopping one project to start another.

## How tug solves it

```bash
tug up
```

tug wraps Docker Compose and automatically:

- Routes **HTTP services** through Traefik at `http://<service>.<project>.localhost`
- Assigns **deterministic host ports** to TCP services (PostgreSQL, Redis, etc.) using a hash of the project + service
  name — no more conflicts
- Passes through any other command to `docker compose` as-is

Two projects, same ports, zero conflicts:

```
~/examples/app-a $ tug ps
SERVICE    TYPE   URL/PORT                              STATUS
api        http   http://api.app-a.localhost             running
web        http   http://web.app-a.localhost             running
postgres   tcp    localhost:19315 → 5432                 running
redis      tcp    localhost:29042 → 6379                 running

~/examples/app-b $ tug ps
SERVICE    TYPE   URL/PORT                              STATUS
api        http   http://api.app-b.localhost             running
postgres   tcp    localhost:54817 → 5432                 running
redis      tcp    localhost:38291 → 6379                 running
```

## Install

### Homebrew

```bash
brew install mickamy/tap/tug
```

### Download binary

Grab the latest release from [GitHub Releases](https://github.com/mickamy/tug/releases) and place it in your `$PATH`.

### Go

```bash
go install github.com/mickamy/tug@latest
```

### Build from source

```bash
make install
```

### Requirements

- Docker (or Podman) with Compose v2.24+
- Port 80 available for Traefik

## Usage

```bash
tug up                    # start services with auto-routing
tug down                  # stop services and clean up
tug ps                    # show URLs and port mappings
tug ps --json             # machine-readable output
tug logs -f api           # passthrough to docker compose
tug exec db psql          # passthrough to docker compose
```

### Flags

```
-f, --file       Specify compose file (default: auto-detect)
--override       Layer an additional override file (repeatable)
--version, -v    Print version
-h, --help       Show help
```

### Override files

Layer additional compose files on top of the base, just like `docker compose -f`:

```bash
tug --override staging.yaml up
tug --override a.yaml --override b.yaml up
```

tug merges them via `docker compose config`, then generates its own routing override on top.

## Configuration

Create a `.tug.yaml` in your project root (or `~/.config/tug.yaml` for global defaults). Project-local config takes
priority.

### Custom commands

```yaml
command:
  compose: "podman compose"  # default: "docker compose"
  runtime: "podman"          # default: "docker"
```

### Per-service kind override

By default, tug detects TCP services by well-known ports (5432, 3306, 6379, etc.) and treats everything else as HTTP.
Override this per service:

```yaml
services:
  my-grpc-server:
    kind: tcp
```

### Per-port kind override

For services that expose both HTTP and TCP ports (e.g., a SQL proxy with a web UI and a gRPC port):

```yaml
services:
  sql-tap:
    ports:
      8081: http   # web UI → Traefik
      9091: tcp    # gRPC → deterministic port
```

Priority: per-port config > per-service `kind` > well-known port detection > HTTP default.

## How it works

1. **Parse** the compose file (and any `--override` files)
2. **Classify** each port as HTTP or TCP
3. **Generate** `.tug/override.yaml` with Traefik labels and port remappings
4. **Start Traefik** if not already running (shared across all projects)
5. **Run** `docker compose -f <base> -f .tug/override.yaml up`

The generated `.tug/` directory is in `.gitignore` — it's ephemeral.

### Deterministic ports

TCP ports are mapped using FNV-1a hash of `project + service + container port`, landing in the 10000–60000 range. The
same project always gets the same ports, even across machines.

### Traefik

tug runs a single shared Traefik instance (`tug-traefik` container) that routes all HTTP services. It's created on first
`tug up` and persists across projects. `tug down` does not stop Traefik since other projects may be using it.

## License

[MIT](./LICENSE)
