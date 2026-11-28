# NodeWorld

[![CI](https://github.com/its-the-vibe/NodeWorld/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/its-the-vibe/NodeWorld/actions/workflows/ci.yaml)

NodeWorld is a local actor-based timed flow graph simulator. It requires Go 1.27.4 or later.

```sh
go run . --config examples/sample.yaml
go run . --config examples/sample.yaml --days 30
go run . --config examples/sample.yaml --days 30 --output result.json
go run . --config examples/sample.yaml --ws localhost:8080
go run . --config examples/sample.yaml --ws localhost:8080 --serve
```

The default config is `examples/sample.yaml`. The synthetic sample shows the YAML schema: nodes have unique IDs, optional labels, kinds and generic `meta`, and an initial integer `level`; links have existing `from` and `to` IDs, nonnegative integer `magnitude`, and `schedule_day` in the range 1 through `cycle_days`. The simulation defaults to a 30-day cycle and one day per second. Speeds must be between 0.001 and 1000 days per second. Use a `*.local.yaml` file for private configuration; these files are ignored by Git.

## Docker

Build the image and run the sample interactively or headlessly:

```sh
docker build -t nodeworld .
docker run --rm -it nodeworld
docker run --rm nodeworld --days 30
```

To use your own configuration, mount it read-only and pass its container path:

```sh
docker run --rm -v "$(pwd)/config.local.yaml:/config.yaml:ro" nodeworld --config /config.yaml --days 30
```

For a persistent WebSocket server, listen on all container interfaces and publish the port only on the host's loopback interface:

```sh
docker run --rm -p 127.0.0.1:8080:8080 nodeworld --ws 0.0.0.0:8080 --serve
```

The server has no authentication; restrict access to trusted clients and networks.

The simulation starts paused at day 0. Each step advances the day first, then processes the scheduled links in node configuration order and, for each node, link configuration order. After the last day of the cycle, the next day is 1. Transfers can make a level negative. A transfer that would overflow an int64 level is rejected; earlier transfers in the day remain applied, and playback pauses on an overflow. A flow event reports the source level immediately after the debit and the target level immediately after the credit. Headless output is JSON containing the final `snapshot` and ordered `events`. Metadata must be JSON-compatible.

The REPL supports `step [n]`, `play [days_per_sec]`, `pause`, `reset`, `status`, `node <id>` (including outgoing links), `help [command]`, and `quit`. Use `help` to list commands or `help <command>` for details. `step` also works while playing; it does not change the playback state. `reset` restores day 0 and initial node levels, pauses playback, and preserves the current speed. EOF exits the REPL.

With `--ws`, connect to `/ws` using a WebSocket client. The server runs for the lifetime of the REPL (or headless run). It sends `{"type":"snapshot","snapshot":...}` first, then `day_tick` and `flow_event` messages with `sim_day` and, for flows, `from`, `to`, `amount`, `source_level`, and `target_level`. A reset command returns the simulation to day 0, restores initial node levels, pauses playback, and preserves the current speed; all subscribers receive a fresh snapshot after any previously queued events. Control messages are JSON:

```json
{"type":"step","days":2}
{"type":"resume"}
{"type":"pause"}
{"type":"reset"}
{"type":"set_speed","days_per_sec":5}
```

`step` defaults to one day and accepts up to 10000 days per command. Invalid commands and overflow produce an `error` message. Slow clients are disconnected if their event buffer remains full for one second; reconnect for a fresh snapshot. WebSocket connections use same-origin verification by default. There is no authentication, so bind to localhost unless access is restricted by a trusted network or proxy.

`--serve` (requires `--ws`) runs a persistent WebSocket server with no REPL: it does not read stdin, so it keeps running after stdin closes, and exits cleanly on SIGINT or SIGTERM. The simulation starts paused at day 0; clients can step, play, reset and replay it, and reconnecting clients receive the current snapshot. `--serve` without `--ws`, or combined with `--days` or `--output`, is an error. Without `--serve`, REPL and `--days` behavior is unchanged.

Run `make build`, `make test`, `make vet`, or `make fmt` for common development tasks.
Use `make run ARGS="--days 30"` to pass arguments to the simulator.
