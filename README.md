# NodeWorld

NodeWorld is a local actor-based timed flow graph simulator. It requires Go 1.23 or later.

```sh
go run . --config examples/sample.yaml
go run . --config examples/sample.yaml --days 30
go run . --config examples/sample.yaml --days 30 --output result.json
go run . --config examples/sample.yaml --ws localhost:8080
```

The default config is `examples/sample.yaml`. The synthetic sample shows the YAML schema: nodes have unique IDs, optional labels, kinds and generic `meta`, and an initial integer `level`; links have existing `from` and `to` IDs, nonnegative integer `magnitude`, and `schedule_day` in the range 1 through `cycle_days`. The simulation defaults to a 30-day cycle and one day per second. Speeds must be between 0.001 and 1000 days per second. Use a `*.local.yaml` file for private configuration; these files are ignored by Git.

The simulation starts paused at day 0. Each step advances the day first, then processes the scheduled links in node configuration order and, for each node, link configuration order. After the last day of the cycle, the next day is 1. Transfers can make a level negative. A flow event reports the source level immediately after the debit and the target level immediately after the credit. Headless output is JSON containing the final `snapshot` and ordered `events`.

The REPL supports `step [n]`, `play [days_per_sec]`, `pause`, `status`, `node <id>` (including outgoing links), and `quit`. `step` also works while playing; it does not change the playback state. EOF exits the REPL.

With `--ws`, connect to `/ws` using a WebSocket client. The server runs for the lifetime of the REPL (or headless run). It sends `{"type":"snapshot","snapshot":...}` first, then `day_tick` and `flow_event` messages with `sim_day` and, for flows, `from`, `to`, `amount`, `source_level`, and `target_level`. Control messages are JSON:

```json
{"type":"step","days":2}
{"type":"resume"}
{"type":"pause"}
{"type":"set_speed","days_per_sec":5}
```

`step` defaults to one day and accepts up to 10000 days per command. Invalid commands produce an `error` message. Slow clients are disconnected if their event buffer fills; reconnect for a fresh snapshot. WebSocket connections use same-origin verification by default. There is no authentication, so bind to localhost unless access is restricted by a trusted network or proxy.

Run tests with `go test ./...`.
