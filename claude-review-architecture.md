# Architecture & Engineering Review

**Date:** 2026-06-18  
**Reviewer:** Claude Sonnet 4.6  
**Scope:** Full codebase — `github.com/dioad/net`  
**Go version:** 1.26 (go.mod)

---

## Executive Summary

The library is well-structured in aggregate: it provides useful networking primitives (TLS, rate limiting, authz, DNS, SMTP record parsing, HTTP server/client) with reasonable test coverage and idiomatic Go style in most places. The most significant issues are a **correctness bug in `authz.Listener.Accept()`** that silently hands a closed connection to the caller, a **panic-on-bad-config** path in the IP authz HTTP handler, and a pattern of **parallel implementations without shared abstraction** (three listener types solving the same rejection problem in slightly different ways). The dependency on the **unmaintained `weaveworks/common`** library is the main supply-chain risk.

Hexagonal architecture thinking reveals that the `authz/prefixlist` package couples domain logic too tightly to HTTP infrastructure, the `http.Server` is a borderline god-object, and several key types lack port interfaces that would reduce coupling and improve testability.

---

## Findings

### Priority Ranking Table

| # | Finding | Severity | Dimensions |
|---|---------|----------|------------|
| 1 | `authz.Listener.Accept()` returns closed conn on rejection | **P1 — Bug** | Correctness |
| 2 | `http/authz/ip.NewHandler` drops `NewNetworkACL` error → nil deref panic | **P1 — Bug** | Correctness, Security |
| 3 | `dns.DOHClient.Exchange` returns `resp.Body.Close()` as the function error | **P2 — Bug** | Correctness |
| 4 | `my_ip.go`: `io.ReadAll` error shadowed by `netip.ParseAddr` assignment | **P2 — Bug** | Correctness |
| 5 | `http.Client.Request` unconditionally sets `Content-Type: application/json` | **P2 — API** | Correctness |
| 6 | `"DioadClient/VERSION"` literal version placeholder | **P2 — API** | Correctness |
| 7 | Three near-identical listeners with divergent reject logic; only `authz.Listener` is broken | **P2 — Design** | Correctness, Maintainability |
| 8 | `ratelimit.RateLimiter` — 8 overlapping constructors vs. functional-options pattern | **P3 — Design** | Maintainability, Consistency |
| 9 | `WithBodySizeLimiterLogger` is a no-op kept for "API compatibility" | **P3 — API** | Maintainability |
| 10 | `http/rate_limiter.go`: rate-limit counter registered on `prometheus.DefaultRegisterer`, not the server's private registry | **P3 — Design** | Consistency |
| 11 | `CachingFetcher.lastHeaders` written outside the lock | **P3 — Concurrency** | Correctness (fragility) |
| 12 | `CachingFetcher.fetchJSON` and `FetchTextLines` each create a fresh `*http.Client` per fetch | **P3 — Performance** | Maintainability |
| 13 | `WithCORS` silently discards the error from `CORSHandler` | **P3 — API** | Correctness |
| 14 | `metrics.Listener.acceptedCount` is not protected by a mutex | **P3 — Concurrency** | Correctness (fragility) |
| 15 | `http.Server.Mux` is an exported field — callers can bypass the middleware chain | **P3 — Design** | Maintainability, Security |
| 16 | `weaveworks/common` is archived/unmaintained; leaks through the public API | **P3 — Dependencies** | Maintainability, Security |
| 17 | `authz/prefixlist`: HTTP infrastructure coupled inside the domain package | **P4 — Architecture** | Maintainability, Testability |
| 18 | `NetworkACL` has no port interface; all call-sites are coupled to the concrete type | **P4 — Architecture** | Maintainability, Testability |
| 19 | `http.Server` Config uses `Enable*` feature flags — composition-resistant god-object | **P4 — Architecture** | Maintainability |
| 20 | `http/resource/log_level.go`: duplicate route registrations (`GET /{$}` and `GET /`) | **P4 — Minor** | Correctness |
| 21 | `dns.DOHClient` passes `c.URL.Host` instead of the full URL string to `doh.NewRequest` | **P4 — API** | Correctness |
| 22 | `http.Client` validates config at call time instead of construction time | **P4 — Design** | Maintainability |
| 23 | `EnforceTLSHandler.EnforceTLS` is an exported mutable field with no constructor | **P4 — Design** | Maintainability |
| 24 | `comment rot` in `http/resource/log_level.go` (large commented-out `Duration` type) | **P5 — Quality** | Maintainability |
| 25 | `my_ip.go`: service URLs use `http://` (plaintext) not `https://` | **P5 — Security** | Security |

---

## Detailed Findings

---

### P1 — Bug

#### Finding 1: `authz.Listener.Accept()` returns a closed connection on rejection

**File:** `authz/network_acl_listener.go:18–38`

```go
func (l *Listener) Accept() (net.Conn, error) {
    c, err := l.Listener.Accept()
    // ...
    if !authorised {
        l.Logger.Warn()...
        err = c.Close()
        // error from Close is logged and then...
    }
    return c, nil  // ← closed connection returned with nil error
}
```

When a connection is rejected the method closes it but still returns `(c, nil)` to the caller. Any `net.http.Server` wrapping this listener will receive a dead connection with no error signal, attempt to use it, log confusing IO errors, and waste a goroutine. The method never loops — it accepts exactly one connection per call, so the server advances correctly, but each rejected connection causes spurious errors instead of being silently discarded.

**Contrast with the correct pattern** used in both sibling listeners:

```go
// ratelimit/listener.go — correct loop-and-continue pattern
func (l *Listener) Accept() (net.Conn, error) {
    for {
        conn, err := l.Listener.Accept()
        if err != nil { return nil, err }
        if !l.RateLimiter.Allow(principal) {
            conn.Close()
            continue   // ← discard and try next
        }
        return conn, nil
    }
}
```

`prefixlist.Listener` uses the same loop pattern. `authz.Listener` is the odd one out (see **Finding 7** for the structural root cause).

**Fix:** Adopt the loop-and-continue pattern. Return only authorised connections.

---

#### Finding 2: `http/authz/ip.NewHandler` silently drops `NewNetworkACL` error

**File:** `http/authz/ip/handler.go:20`

```go
func NewHandler(cfg authz.NetworkACLConfig) *Handler {
    authoriser, _ := authz.NewNetworkACL(cfg)   // ← error discarded
    return &Handler{Authoriser: authoriser}
}
```

`NewNetworkACL` returns an error when any configured CIDR is malformed. When that happens `authoriser` is `nil`. The first incoming HTTP request then panics:

```
panic: runtime error: invalid memory address or nil pointer dereference
    authz.(*NetworkACL).AuthoriseFromString(...)
```

Because `NewHandler` returns a value (not an error), the only practical fix is to panic eagerly at construction time — or better, change the signature to return `(*Handler, error)` and let callers decide.

**Fix:**
```go
func NewHandler(cfg authz.NetworkACLConfig) (*Handler, error) {
    authoriser, err := authz.NewNetworkACL(cfg)
    if err != nil {
        return nil, fmt.Errorf("ip.NewHandler: invalid ACL config: %w", err)
    }
    return &Handler{Authoriser: authoriser}, nil
}
```

---

### P2 — Correctness


#### Finding 4: `my_ip.go` — `io.ReadAll` error shadowed

**File:** `my_ip.go:32–38`

```go
ipBytes, err := io.ReadAll(resp.Body)          // err assigned here
ipString := strings.TrimRight(string(ipBytes), "\n")

addr, err := netip.ParseAddr(ipString)          // err re-assigned, original lost
```

If `io.ReadAll` fails, `ipBytes` may be partial, `ipString` will be garbage, and the error surfaced will say "invalid address" rather than the true IO error. The fix is straightforward: check `err` from `ReadAll` before proceeding.

---

#### Finding 5: `http.Client.Request` unconditionally sets `Content-Type: application/json`

**File:** `http/client.go:53`

```go
req.Header.Set("Content-Type", "application/json")
```

This overwrites any `Content-Type` the caller may have set and is semantically incorrect for `GET`, `DELETE`, and `HEAD` requests that carry no body. It will confuse some servers.

**Fix:** Only set the header when `req.Body != nil && req.ContentLength != 0`.

---

#### Finding 6: `"DioadClient/VERSION"` literal placeholder

**File:** `http/client.go:45`

```go
libraryUserAgent := "DioadClient/VERSION"
```

`VERSION` is a raw string, not a substituted build variable. Every request identifies the library version as `"VERSION"`. This should be replaced with a real version string (build tag, `debug.ReadBuildInfo`, or a package-level constant managed by the release process).

**Fix:** Use a similar pattern to `sdk/agentregistry/client.go` from `github.com/dioad/connect-control` to populate the version string in the user agent using `buildinfo`.

---

#### Finding 7: Three listeners with divergent rejection logic — structural root cause of Finding 1

**Files:** `authz/network_acl_listener.go`, `ratelimit/listener.go`, `authz/prefixlist/listener.go`

All three implement the same "reject-and-gate" contract on `net.Listener.Accept()`, yet each is written independently. The two newer ones are correct; the oldest (`authz.Listener`) is buggy. Parallel implementations without a shared abstraction guarantee divergence.

**The pattern is simple enough to extract:**

```go
// A gating listener that applies a predicate to each accepted connection.
// Rejected connections are closed and the loop continues.
type GatingListener struct {
    net.Listener
    Gate func(net.Conn) bool
}

func (g *GatingListener) Accept() (net.Conn, error) {
    for {
        c, err := g.Listener.Accept()
        if err != nil { return nil, err }
        if g.Gate(c) { return c, nil }
        c.Close()
    }
}
```

Each existing listener becomes a thin wrapper that provides its `Gate` function. This also eliminates duplicated `Accept`, `Close`, and `Addr` forwarding.

---

### P3 — Maintainability / Design

#### Finding 8: `ratelimit.RateLimiter` — 8 constructors for 3 conceptual axes

**File:** `ratelimit/rate_limiter.go:63–221`

The 8 constructors (`NewRateLimiter`, `NewRateLimiterWithContext`, `NewRateLimiterWithConfig`, `NewRateLimiterWithContextAndConfig`, `NewRateLimiterWithSource`, `NewRateLimiterWithSourceAndContext`, `NewRateLimiterWithSourceAndConfig`, `NewRateLimiterWithSourceContextAndConfig`) encode three orthogonal choices:

- Static limits vs. dynamic source
- Default config vs. custom cleanup intervals
- With or without a parent context

The [functional options pattern](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis) (already used in `http.RateLimiter` in the same repo) collapses this to one constructor and `O(N)` option functions:

```go
func NewRateLimiter(opts ...Option) *RateLimiter { ... }

func WithContext(ctx context.Context) Option { ... }
func WithLimitSource(s RateLimitSource) Option { ... }
func WithStaticLimits(rps float64, burst int) Option { ... }
func WithCleanupConfig(interval, ttl time.Duration) Option { ... }
```

This is a direct consistency issue: the HTTP-layer `RateLimiter` uses functional options; the underlying `ratelimit.RateLimiter` does not.

**Fix:** Deprecate the 8 constructors and replace with an option function

---

#### Finding 9: `WithBodySizeLimiterLogger` is a no-op

**File:** `http/body_size.go:26–28`

```go
// Deprecated: BodySizeLimiter now uses the request-scoped zerolog context logger...
func WithBodySizeLimiterLogger(_ zerolog.Logger) BodySizeLimiterOpt {
    return func(_ *BodySizeLimiter) {}
}
```

This option was retained for "API compatibility" but accepts and discards a logger silently. Callers get no error and no indication that the option is ignored. Given this is a library (not a binary), a clean break — removing the option — is preferable to keeping a misleading no-op. If removing it breaks callers, the `// Deprecated` comment is the right approach, but the godoc should clearly state "this option is a no-op and will be removed in a future release."

**Fix:** Remove `WithBodySizeLimiterLogger`

---

#### Finding 10: HTTP rate-limit counter registered on `prometheus.DefaultRegisterer`

**File:** `http/rate_limiter.go:20–32`

```go
var (
    rateLimitCounter     *prometheus.CounterVec
    rateLimitCounterOnce sync.Once
)

func getRateLimitCounter() *prometheus.CounterVec {
    rateLimitCounterOnce.Do(func() {
        rateLimitCounter = prometheus.NewCounterVec(...)
        prometheus.DefaultRegisterer.MustRegister(rateLimitCounter)  // ← default registry
    })
    return rateLimitCounter
}
```

The `http.Server` creates a **private** `prometheus.Registry` and a `MetricSet` specifically to avoid polluting the default registry. The rate-limiter counter bypasses this entirely and registers globally. Two consequences:

1. In tests that create multiple `*Server` instances, the first instance's metric registration succeeds; subsequent ones have no conflict (because it's a `sync.Once`), but the metric is shared across all server instances without any label to distinguish them.
2. The metric doesn't appear under the server's private `/metrics` endpoint unless `prometheus.DefaultGatherer` is in `metricsGatherers` (which it is by default, but this is accidental coupling).

**Fix:** Pass the server's `prometheus.Registerer` when constructing an HTTP `RateLimiter`, mirroring how `MetricSet` is handled.

---

#### Finding 11: `CachingFetcher.lastHeaders` written outside the lock

**File:** `authz/prefixlist/cache.go:216`

```go
func (f *CachingFetcher[T]) fetchJSON(ctx context.Context) (T, error) {
    // ...
    f.lastHeaders = resp.Header   // ← written without holding f.mu
    // ...
}
```

`fetchJSON` is called from both the synchronous path (`Get`) and the background goroutine (`backgroundRefresh`). In `Get`, the lock is released before `doFetch` is called. `lastHeaders` is then read in `calculateExpiry` (called from within the lock). While the `refreshing` flag provides logical serialisation, the absence of a lock on the write makes the access formally racy and fragile against future changes.

**Fix:** Make `lastHeaders` a local return value from `doFetch`/`fetchJSON` and pass it to `calculateExpiry`. No shared state required.

---

#### Finding 12: New `*http.Client` created per fetch

**Files:** `authz/prefixlist/cache.go:207`, `authz/prefixlist/utils.go:66`

```go
client := &http.Client{Timeout: 30 * time.Second}   // new allocation every call
```

Both `CachingFetcher.fetchJSON` and `FetchTextLines` create a fresh `*http.Client` on each invocation, bypassing connection-pool reuse. `http.Client` is documented as safe for concurrent use and intended to be shared. A package-level (or fetcher-level) client with appropriate timeouts should be used instead.

**Fix:** Use a fetcher level `http.Client`

---

#### Finding 13: `WithCORS` silently discards error from `CORSHandler`

**File:** `http/server.go:158–160`

```go
func WithCORS(options cors.Options) ServerOption {
    return func(s *Server) {
        // ...
        handler, _ := CORSHandler(options)   // ← error discarded
        s.Use(handler)
    }
}
```

`CORSHandler` returns an error in its signature but `WithCORS` discards it. The `s.Use(nil)` call is safe (nil middlewares are filtered), so this fails silently rather than panicking — but any misconfiguration in `cors.Options` is invisible.

---

#### Finding 14: `metrics.Listener.acceptedCount` is not protected

**File:** `metrics/listener.go:38`

```go
func (l *Listener) Accept() (net.Conn, error) {
    // ...
    l.acceptedCount += 1   // not thread-safe
    // ...
}
```

`Accept` is typically called from a single serve-loop goroutine, but `AcceptedCount()` and `ResetMetrics()` are on the public API and may be called from a monitoring goroutine concurrently. Use `sync/atomic` or a mutex to guard this counter.

**Fix:** Use `sync/atomic` to modify / read `acceptedCount`
---

#### Finding 15: `http.Server.Mux` is an exported field

**File:** `http/server.go:67`

The exported `Mux *http.ServeMux` field allows callers to register handlers that bypass the server's middleware chain (logging, metrics, request IDs). It also makes it impossible to replace the mux implementation without a breaking API change.

`AddHandler` and `AddHandlerFunc` are the idiomatic accessors already provided. The exported field should be unexported, or at minimum documented with the caveat that direct registration bypasses middleware.

**Fix:** Make `Mux` an unexported field.

---

#### Finding 16: `weaveworks/common` is archived and leaks through the public API

**File:** `http/server.go:178`

```go
func WithTelemetryInstrument(i middleware.Instrument) ServerOption {
```

`middleware.Instrument` is from `github.com/weaveworks/common`, a project whose GitHub repository is archived/unmaintained. This type appears in a public `ServerOption` function, meaning consumers of this library are indirectly coupled to an unmaintained transitive dependency. The full import chain is visible in `go.mod`, which also pulls in Jaeger, logrus, and protobuf via this dependency.

**Recommendation:** Define a local `Instrument` interface that wraps what is actually needed, and implement an adapter from `weaveworks/common` behind that interface. This decouples the API surface from the external type.

---

### P4 — Architecture / Hexagonal Thinking

#### Finding 17: `authz/prefixlist` couples domain logic to HTTP infrastructure

**Package:** `authz/prefixlist`

The package contains:
- A domain abstraction (`Provider` interface, prefix matching logic)
- HTTP fetching infrastructure (`CachingFetcher`, `HTTPJSONProvider`, `HTTPTextProvider`, `FetchTextLines`)
- Multiple cloud-provider adapters (AWS, GitHub, Cloudflare, Atlassian, Fastly, GitLab, Hetzner, Google)

In hexagonal terms, the domain (IP prefix matching) should be independent of how prefixes are sourced. Currently, `CachingFetcher` (an infrastructure concern) sits inside the domain package. This makes it impossible to test prefix matching without also dealing with HTTP, and it prevents using the fetcher for other purposes.

**Suggested split:**

```
authz/
  prefixlist/              ← domain: Provider interface, MultiProvider, ACL integration
  prefixlist/provider/     ← adapters: HTTPJSONProvider, CachingFetcher, cloud providers
```

Or: promote `CachingFetcher` to a shared `cache/http` package — it is entirely generic (`CachingFetcher[T any]`) and has no dependency on prefix-list concepts.

**Fix:** Move `CachingFetcher` into a shared package.

---

#### Finding 18: `NetworkACL` has no port interface

The `prefixlist.Provider` interface is a well-designed secondary port. `authz.NetworkACL` has no equivalent. All three listeners hold a `*authz.NetworkACL` concrete pointer, all HTTP handlers hold a `*authz.NetworkACL`, and tests must construct the real type.

Introducing an `Authoriser` interface:

```go
type Authoriser interface {
    Authorise(addr *net.TCPAddr) bool
    AuthoriseFromString(addr string) (bool, error)
    AuthoriseConn(c net.Conn) (bool, error)
}
```

would let the gating listener, HTTP handler, and tests all depend on the interface rather than the implementation — enabling simpler test doubles and future alternative implementations (e.g., a dynamic or externally-sourced ACL).

---


#### Finding 20: `http/resource/log_level.go` — duplicate route patterns

**File:** `http/resource/log_level.go:122–125`

```go
mux.HandleFunc("GET /{$}", dr.GetIndex())
mux.HandleFunc("GET /", dr.GetIndex())
mux.HandleFunc("POST /{$}", dr.PostIndex())
mux.HandleFunc("POST /", dr.PostIndex())
```

With Go 1.22+ enhanced `ServeMux`, `GET /{$}` matches only the exact root path, while `GET /` is a catch-all. Both registrations are active, so an unmatched sub-path would be handled by `GetIndex` (via the catch-all). This is likely unintentional — the resource probably only wants to respond to exactly `GET /`. The `GET /` catch-all should be removed, and unmatched paths should return 404.

**Fix:** Remove the catchall

---


#### Finding 22: `http.Client` validates config at call time

**File:** `http/client.go:22–37`

`checkConfig` is called on every `Request` and `ResolveRelativeRequestPath` call. Config validation is an invariant that should be enforced at construction time (`NewClient`). If the config is bad, failing at call time means the error is triggered during an otherwise-valid request, often far from where the bad config was supplied.

**Fix:** Enforce config validation at construction time

---

#### Finding 23: `EnforceTLSHandler.EnforceTLS` is an exported mutable field

**File:** `http/enforce_tls_handler.go`

```go
type EnforceTLSHandler struct {
    EnforceTLS bool
}
```

There is no constructor. The exported field can be mutated after the handler is wrapped into a middleware chain, which would silently change security enforcement at runtime. A constructor (`NewEnforceTLSHandler(enforce bool) *EnforceTLSHandler`) plus an unexported field is the safer pattern.

---

### P5 — Minor Quality

#### Finding 24: Comment rot in `http/resource/log_level.go`

**File:** `http/resource/log_level.go:13–35`

A large block of commented-out Go code (a `Duration` type with JSON marshal/unmarshal) has been left in place. It adds noise to an otherwise clean file and should be removed (the purpose is presumably captured in git history).

---

#### Finding 25: `my_ip.go` uses plaintext HTTP

**File:** `my_ip.go:14–17`

```go
IPv4ICanHazIP = "http://ipv4.icanhazip.com"
IPv6ICanHazIP = "http://ipv6.icanhazip.com"
```

These endpoints are used to determine the host's public IP address. Using plaintext HTTP means the response can be tampered with in transit (e.g., a MitM substituting a different IP). `https://` should be used, and the TODO comment notes a planned migration to owned infrastructure — that migration should also enforce TLS.

**Fix:** Migrate to `https://` protocol.

---

## Hexagonal Architecture: Summary Assessment

| Concern | Current State | Recommended Direction |
|---------|--------------|----------------------|
| IP prefix matching | Domain + HTTP infra mixed in `authz/prefixlist` | Separate `Provider` interface from HTTP adapters |
| ACL enforcement | Concrete `*NetworkACL` throughout | Add `Authoriser` interface; implement adapters |
| Rate limiting | `RateLimitSource` is a good port; no interface for the limiter itself | Consider a `Limiter` interface for HTTP middleware decoupling |
| Telemetry / metrics | `weaveworks/common.Instrument` in public API | Define local `Instrument` interface; adapter behind it |
| HTTP instrumentation | Mixed into `Server` via `Config.Enable*` | Move to functional options; make features composable |
| DNS over HTTPS | `DOHClient` is a concrete type with no abstraction | Add `Exchanger` interface mirroring `net.Resolver` patterns |
| Connection lifecycle | `DoneConn` / `RawConn` / `connWithCloser` three-layer wrap | Consider adopting `net.Conn` + channel signalling without the `RawConn` unwrapping chain |

The strongest area architecturally is the `authz/prefixlist.Provider` + `MultiProvider` design — clean interface, composable, easily testable. The weakest area is `http.Server`, which owns too many concerns. Treating each `Enable*` flag as a candidate for extraction is the highest-leverage architectural refactor available.

---

## Dependencies of Note

| Dependency | Status | Risk |
|-----------|--------|------|
| `github.com/weaveworks/common` | Archived/unmaintained | Public API coupling; transitive Jaeger/logrus/grpc |
| `github.com/gorilla/mux` | Indirect only | Low — not used directly |
| `github.com/gorilla/handlers` | Active | Low — used for access log only |
| `github.com/gorilla/sessions` | Active | Low — cookie store |
| `github.com/pires/go-proxyproto` | Active | Low |
| `github.com/coredns/coredns` | Active | Medium — large dependency for a single DoH helper |

The `coredns/coredns` dependency is a notably heavy import (full DNS server) for a package that only uses `plugin/pkg/doh`. A direct implementation of the DoH wire format would be smaller and more maintainable.

---

## Outcomes

All 25 findings have been addressed. The table below records the commit SHA, gocognit complexity delta, and outcome for each finding.

| # | Commit | Complexity (before → after) | Outcome |
|---|--------|----------------------------|---------|
| F1 | `336542d` | `authz.Accept`: 0 → 3 (loop added; inherent) | Fixed: loop-and-continue pattern; no closed conn returned |
| F2 | `71ac279` | `ip.NewHandler`: 0 → 1 | Fixed: `NewHandler` returns `(*Handler, error)`; tests updated |
| F3+F21 | `7955418` | `DOHClient.Exchange`: unchanged | Fixed: body close error no longer returned as result; full URL used |
| F4 | `d9a5be0` | `getICanHazIP`: 4 → 5 (extra guard; inherent) | Fixed: `io.ReadAll` error checked before use |
| F5 | `02363c7` | `Client.Request`: 7 → 8 (conditional; inherent) | Fixed: `Content-Type` only set when request has a body |
| F6 | `d926d58` | no function change | Fixed: `libraryUserAgent` var uses `debug.ReadBuildInfo()`; no more `"VERSION"` literal |
| F7 | `9a7d29e` | `authz.Listener` methods: 2 → 1 (delegated) | Fixed: `GatingListener` abstraction extracted; `authz.Listener` delegates |
| F8 | `f79944d` | no change to existing functions | Fixed: `NewRateLimiterWithOptions` added; old constructors deprecated |
| F9 | `d6317a5` | n/a (deletion) | Fixed: `WithBodySizeLimiterLogger` no-op deleted |
| F10 | `f27fecd` | no change | Fixed: counter is per-instance; `WithRateLimiterRegistry` option added |
| F11 | `c3c7e18` | `doFetch`/`backgroundRefresh`: unchanged | Fixed: `lastHeaders` field removed; headers returned as value |
| F12 | `85c64fb` | `fetchJSON`: 5 → 5, `FetchTextLines`: 3 → 3 | Fixed: `defaultFetchClient` shared; no per-call `*http.Client` allocation |
| F13 | `98e5326` | `CORSHandler`: 0 → 0 | Fixed: error return removed; `WithCORS` no longer discards silently |
| F14 | `e091417` | `Accept`: 3 → 3 | Fixed: `acceptedCount` is `atomic.Int64`; no data races |
| F15 | `3ee2aa0` | no change | Fixed: `Mux` → `mux`; callers use `AddHandler`/`AddHandlerFunc` |
| F16 | `47d992a` | no change | Fixed: local `Instrument` interface defined; `weaveworks/common` removed from public API |
| F17 | Issue [#298](https://github.com/dioad/net/issues/298) | — | Deferred: GitHub issue created for moving `CachingFetcher` to shared package |
| F18 | `fed318b` | no change | Fixed: `Authoriser` interface added; `Listener` and `ip.Handler` use interface |
| F19 | Issue [#299](https://github.com/dioad/net/issues/299) | — | Deferred: GitHub issue created for replacing `Enable*` flags with functional options |
| F20 | `d356686` | n/a | Fixed: catch-all `GET /` and `POST /` routes removed |
| F21 | (combined with F3) | — | Fixed: full URL used in `doh.NewRequest` |
| F22 | `d41bf02` | `NewClient`: 0 → 3; `checkConfig` deleted | Fixed: validation at construction time; `NewClient` returns `(*Client, error)` |
| F23 | `247dd9f` | no change | Fixed: `NewEnforceTLSHandler` constructor added; `EnforceTLS` field unexported |
| F24 | `118c26c` | n/a | Fixed: commented-out `Duration` block deleted |
| F25 | `b7defc7` | n/a | Fixed: `http://` → `https://` for icanhazip.com URLs |
