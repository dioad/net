# API Server

This example is what a Dioad API server looks like using only `dioad/net` and `dioad/lifecycle`: an API listener, a
management listener for the platform's health checks and metrics scrapes, readiness, and a drain on `SIGTERM`.

## Running the Example

```bash
go run github.com/dioad/net/examples/api-server
```

Then, in another terminal:
```bash
curl http://localhost:8080/greetings/world   # the API listener
curl http://localhost:8081/health/ready      # the management listener
curl http://localhost:8081/metrics
curl http://localhost:8081/status
```

Stop it with Ctrl+C or `SIGTERM`. `/health/ready` answers 503 for the drain delay, then both listeners close and the
process exits with status 0.

| Flag | Default | Meaning |
|---|---|---|
| `-listen` | `:8080` | Address of the API listener |
| `-management-listen` | `127.0.0.1:8081` | Address of the management listener |
| `-drain-delay` | `5s` | How long to report not ready before closing the listeners |

## What It Demonstrates

- `build` wires the application without binding a listener; `Run` starts it
- A resource mounted with `AddResource`, answering with `http/json`
- `NewOperationalServer`: health, metrics and status on a listener of their own
- `lifecycle.App` running both servers: started in order, stopped in reverse, and either server's failure ending the process
- `app.Readiness()` as a health source of the API server: not ready while starting and while draining, and a place to add checks for what the API depends on
- Metrics registered with `metrics.RegisterAll` before any server starts
- Structured logging with zerolog: `service` and `component` fields, and `json.LogField` to put a field on the access log line

## Code

See [main.go](main.go) for the complete executable example.
