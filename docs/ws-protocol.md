# WebSocket JSON protocol

## Overview and endpoint

NodeWorld exposes a live stream of a shared simulation and accepts control
commands over a WebSocket. The server is **disabled by default**. Start it with,
for example:

```sh
go run . --config examples/sample.yaml --ws localhost:8080
```

Connect to `ws://localhost:8080/ws`. `--ws` is a TCP listen address, not just a
port; there is no default port. The server exists only for the lifetime of the
REPL or headless run (`--days`), so a headless run may finish before a client can
connect. Add `--serve` (which requires `--ws` and cannot be combined with
`--days` or `--output`) for a persistent server that ignores stdin and runs
until SIGINT or SIGTERM; the simulation starts paused and clients can step,
resume, reset and replay it. The implementation uses the WebSocket library's default same-origin
verification and has no authentication. Bind to localhost unless access is
restricted by a trusted network or proxy.

On connection, the server subscribes the client and sends a `snapshot` first.
It then streams events as they occur. All clients control and observe the same
simulation. There is no event history or replay: reconnecting gives a fresh
snapshot. A reset broadcasts a new snapshot to all subscribers after their
previously queued simulation events.

Each subscription buffers 256 events. If its buffer remains full for one second,
the subscriber is removed and the connection closes; reconnect for a snapshot.
Writes have a 10-second timeout, incoming messages are limited to 4096 bytes, and
connections close when the server shuts down.

## Message format

Each WebSocket message contains one JSON value. Server messages are objects
with a string `type` and type-specific fields at the top level; there is no
common `payload` wrapper. The event struct uses `omitempty` for all fields
except `type`, so fields not relevant to an event are absent. Flow amount and
level fields are pointers in Go and are included even when their value is zero.
Clients send JSON command objects with a `type` and, where needed, parameters.

## Server-to-client messages

The examples below use a two-node simulation with a one-unit flow on day 1.

### `snapshot`

Sent first on connection and broadcast on reset:

```json
{
  "type": "snapshot",
  "snapshot": {
    "sim_day": 0,
    "elapsed_days": 0,
    "running": false,
    "days_per_sec": 1,
    "cycle_days": 30,
    "nodes": [
      {"id": "a", "label": "A", "kind": "source", "level": 1, "meta": {"x": 0, "y": 0}},
      {"id": "b", "label": "B", "kind": "sink", "level": 0}
    ],
    "links": [
      {"from": "a", "to": "b", "magnitude": 1, "schedule_day": 1}
    ]
  }
}
```

| Snapshot field | JSON type | Meaning |
| --- | --- | --- |
| `sim_day` | integer | Current day in the cycle; 0 initially or after reset, otherwise 1 through `cycle_days`. |
| `elapsed_days` | integer (int64) | Total days advanced since start or reset. |
| `running` | boolean | Whether timed playback is running. |
| `days_per_sec` | number | Playback speed in simulation days per second. |
| `cycle_days` | integer | Positive number of days in a cycle. |
| `nodes` | array of objects | Current nodes in configuration order; can be empty. |
| `links` | array of objects | Configured links in configuration order; can be empty. |

Every node has `id` (nonempty unique string), `label` (string), `kind`
(string), and `level` (int64 integer). `label` and `kind` are always serialized
but can be empty strings. Optional `meta` is a JSON-compatible object with
string keys; it is omitted when empty or absent. Each link has `from` and `to`
(node ID strings), `magnitude` (nonnegative int64 integer), and `schedule_day`
(integer from 1 through `cycle_days`).

### `day_tick`

```json
{"type":"day_tick","sim_day":1}
```

The day advances before scheduled flows are processed. After `cycle_days`, the
next day is 1. `sim_day` is the current cycle day, not the total elapsed days.

### `flow_event`

```json
{"type":"flow_event","sim_day":1,"from":"a","to":"b","amount":1,"source_level":0,"target_level":1}
```

`from` and `to` are node IDs; `amount` is the link's integer `magnitude`.
`source_level` is the source's int64 level immediately after the debit, and
`target_level` is the target's int64 level immediately after the credit. Levels
can be negative. For a self-link, these describe the same node after each
successive operation. Each tick precedes its flows, which are emitted in node
configuration order and then outgoing link configuration order.

### `error`

```json
{"type":"error","message":"days must be between 1 and 10000"}
```

`message` is a string. Malformed JSON, incompatible non-null field types,
unknown commands, and invalid command parameters produce an error for the
sending client. JSON `null` for a scalar command field leaves the field
unchanged (normally its zero value) instead of producing a type error. Error
text from JSON decoding depends on the input. A transfer that would overflow
an int64 level broadcasts an error such as:

```json
{"type":"error","message":"level overflow on link \"a\" -> \"b\" at day 1"}
```

The tick and any earlier flows for that day have already been emitted and
applied; the failing flow is not applied, further steps in that request stop,
and playback pauses. Simulation errors are broadcast to all subscribers.

## Client-to-server commands

Commands act on the shared simulation. Successful commands have no direct
acknowledgement; `step` generates ticks and flows, and `reset` broadcasts a
snapshot. `pause`, `resume`, and `set_speed` do not themselves broadcast a
status update.

### `pause`

```json
{"type":"pause"}
```

Stops timed playback.

### `resume`

```json
{"type":"resume"}
```

Starts timed playback at the current speed.

### `step`

```json
{"type":"step","days":2}
```

Advances the simulation by `days` (an integer from 1 through 10000). Omitted,
null, or zero `days` defaults to 1; negative values or values above 10000 produce
`{"type":"error","message":"days must be between 1 and 10000"}`. Stepping
also works during playback and does not change its running state unless a
simulation error occurs.

### `set_speed`

```json
{"type":"set_speed","days_per_sec":5}
```

Sets the playback speed without changing whether playback is running.
`days_per_sec` must be a finite JSON number between 0.001 and 1000 inclusive.
An omitted, null, or zero value is invalid. Invalid speeds produce
`{"type":"error","message":"days_per_sec must be between 0.001 and 1000"}`.

### `reset`

```json
{"type":"reset"}
```

Pauses playback, resets `sim_day` and `elapsed_days` to 0, restores initial node
levels, and preserves the current speed. All subscribers receive the resulting
snapshot.

Unknown or missing `type` produces an error such as
`{"type":"error","message":"unknown command \"play\""}`. The REPL's `play`,
`status`, `node`, `help`, and `quit` commands are not WebSocket commands.
Unrecognized JSON object fields are ignored. Command errors and simulation
events are sent asynchronously, without request IDs or a guaranteed ordering
between the two streams.

## Layout and frontend considerations

There are no dedicated or guaranteed layout, coordinate, geometry, projection,
or link metadata fields. Node `meta` is optional and its contents have no
enforced spatial schema; the sample configuration uses `{"x":0,"y":0}` for
one node, but other nodes may omit it. A frontend must provide a fallback layout
for an abstract graph, 2D map, or 3D globe. Agree on coordinate names, types,
units, coordinate system, and any link geometry before relying on metadata.

There is no protocol version, request ID, explicit command acknowledgement,
status-change event, or snapshot-request command. A client cannot infer
`running`, speed, or total elapsed days reliably from every event alone;
reconnection or reset provides a snapshot, although reset changes state.
The Go int64 values may exceed JavaScript's safe integer range; a frontend
contract should standardize their representation if full precision is needed.
