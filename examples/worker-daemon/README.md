# Worker Daemon

This example is a daemon with no listener: a worker that does a unit of work on an interval, run by
[`dioad/lifecycle`](https://github.com/dioad/lifecycle). It stops cleanly on `SIGTERM`, and exits non-zero if the
worker fails.

## Running the Example

```bash
go run github.com/dioad/net/examples/worker-daemon
```

Stop it with Ctrl+C or `SIGTERM`. It finishes the unit of work in progress and exits with status 0.

| Flag | Default | Meaning |
|---|---|---|
| `-interval` | `1s` | How often the worker does a unit of work |
| `-fail-after` | `0` | Fail after this many units of work, to show a failure ending the process (0 never fails) |

```bash
go run github.com/dioad/net/examples/worker-daemon -fail-after 3   # exits with status 1
```

## What It Demonstrates

- A `lifecycle.Service` that is not an HTTP server: `Start` launches the loop and returns, and `Shutdown` stops it within the context it is given
- `Failed`, which makes the worker's failure end the process, with `Shutdown` still called
- A loop context that is not derived from `Start`'s context, which is done as soon as `SIGTERM` arrives, before the drain
- Structured logging with zerolog and a `service` field

## Code

See [main.go](main.go) for the complete executable example.
