# Architecture Review: Resolved Findings — `github.com/dioad/net`

_Reviewed: 2026-06-18 — See [open findings](./claude-review-architecture.md)_

---

## Resolved Findings

### F1. `authz.Listener.Accept()` returns a closed connection on rejection ✅ Resolved

- **File(s):** `authz/network_acl_listener.go:18-38`
- **Dimension(s):** Correctness
- **Priority:** High (P1)
- **Resolved in:** 336542d
- **Description:** When a connection was rejected, the method closed it but still returned `(c, nil)` to the caller. Any `net/http.Server` wrapping this listener received a dead connection with no error signal, causing spurious IO errors. The two sibling listeners (`ratelimit.Listener`, `prefixlist.Listener`) used the correct loop-and-continue pattern; `authz.Listener` was the odd one out.
- **Outcome:** Adopted the loop-and-continue pattern. Only authorised connections are returned. Complexity delta: `authz.Accept` 0→3.

---

### F2. `http/authz/ip.NewHandler` silently drops `NewNetworkACL` error → nil-deref panic ✅ Resolved

- **File(s):** `http/authz/ip/handler.go:20`
- **Dimension(s):** Correctness, Security
- **Priority:** High (P1)
- **Resolved in:** 71ac279
- **Description:** `NewHandler` discarded the error from `NewNetworkACL`. On malformed CIDR config, `authoriser` was nil, and the first HTTP request would panic with a nil-pointer dereference.
- **Outcome:** `NewHandler` now returns `(*Handler, error)`. Callers updated. Complexity delta: `ip.NewHandler` 0→1.

---

### F3+F21. `dns.DOHClient.Exchange` returns body-close error as function error; wrong URL passed ✅ Resolved

- **File(s):** `dns/doh_client.go`
- **Dimension(s):** Correctness
- **Priority:** High (P2)
- **Resolved in:** 7955418
- **Description:** `Exchange` returned the error from `resp.Body.Close()` as the function's result, discarding the actual exchange outcome. Separately, `c.URL.Host` was passed to `doh.NewRequest` instead of the full URL string.
- **Outcome:** Body close error no longer returned as the result. Full URL used in `doh.NewRequest`.

---

### F4. `my_ip.go` — `io.ReadAll` error shadowed by `netip.ParseAddr` assignment ✅ Resolved

- **File(s):** `my_ip.go:32-38`
- **Dimension(s):** Correctness
- **Priority:** Medium (P2)
- **Resolved in:** d9a5be0
- **Description:** If `io.ReadAll` failed, the error was overwritten by the `netip.ParseAddr` assignment, surfacing a misleading "invalid address" error instead of the true IO error.
- **Outcome:** `io.ReadAll` error is now checked before proceeding. Complexity delta: `getICanHazIP` 4→5.

---

### F5. `http.Client.Request` unconditionally sets `Content-Type: application/json` ✅ Resolved

- **File(s):** `http/client.go:53`
- **Dimension(s):** Correctness
- **Priority:** Medium (P2)
- **Resolved in:** 02363c7
- **Description:** The header was always set, overwriting any caller-provided `Content-Type` and being semantically incorrect for `GET`, `DELETE`, and `HEAD` requests.
- **Outcome:** `Content-Type` is only set when the request has a body (`req.Body != nil && req.ContentLength != 0`). Complexity delta: `Client.Request` 7→8.

---

### F6. `"DioadClient/VERSION"` literal placeholder in user-agent ✅ Resolved

- **File(s):** `http/client.go:45`
- **Dimension(s):** Correctness
- **Priority:** Medium (P2)
- **Resolved in:** d926d58
- **Description:** Every request identified the library version as the literal string `"VERSION"`.
- **Outcome:** `libraryUserAgent` now uses `debug.ReadBuildInfo()` to populate the real version.

---

### F7. Three near-identical listeners with divergent rejection logic — `GatingListener` extracted ✅ Resolved

- **File(s):** `authz/network_acl_listener.go`, `ratelimit/listener.go`, `authz/prefixlist/listener.go`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** Medium (P2)
- **Resolved in:** 9a7d29e
- **Description:** Three independent listener implementations solved the same reject-and-gate contract, guaranteeing divergence. The oldest (`authz.Listener`) was buggy while the two newer ones were correct.
- **Outcome:** Extracted a `GatingListener` abstraction with the loop-and-continue pattern. `authz.Listener` delegates to it. Complexity delta: `authz.Listener` methods 2→1.

---

### F8. `ratelimit.RateLimiter` — 8 overlapping constructors ✅ Resolved

- **File(s):** `ratelimit/rate_limiter.go:63-221`
- **Dimension(s):** Maintainability
- **Priority:** Medium (P3)
- **Resolved in:** f79944d
- **Description:** Eight constructors encoded three orthogonal choices (static vs. dynamic limits, default vs. custom config, with or without context), inconsistent with the HTTP-layer `RateLimiter` which already used functional options.
- **Outcome:** Added `NewRateLimiterWithOptions` constructor with functional options. Old constructors deprecated.

---

### F9. `WithBodySizeLimiterLogger` is a no-op ✅ Resolved

- **File(s):** `http/body_size.go:26-28`
- **Dimension(s):** Maintainability
- **Priority:** Medium (P3)
- **Resolved in:** d6317a5
- **Description:** The option silently accepted and discarded a logger, giving callers no indication it was ignored.
- **Outcome:** `WithBodySizeLimiterLogger` deleted.

---

### F10. HTTP rate-limit counter registered on `prometheus.DefaultRegisterer` ✅ Resolved

- **File(s):** `http/rate_limiter.go:20-32`
- **Dimension(s):** Maintainability
- **Priority:** Medium (P3)
- **Resolved in:** f27fecd
- **Description:** The rate-limit counter bypassed the server's private Prometheus registry and registered globally, creating accidental coupling.
- **Outcome:** Counter is now per-instance. `WithRateLimiterRegistry` option added.

---

### F11. `CachingFetcher.lastHeaders` written outside the lock ✅ Resolved

- **File(s):** `authz/prefixlist/cache.go:216`
- **Dimension(s):** Correctness
- **Priority:** Medium (P3)
- **Resolved in:** c3c7e18
- **Description:** `lastHeaders` was written without holding `f.mu`, making the access formally racy against concurrent reads from `calculateExpiry`.
- **Outcome:** `lastHeaders` field removed; headers returned as a value from `doFetch`/`fetchJSON` and passed to `calculateExpiry`. No shared state needed.

---

### F12. New `*http.Client` created per fetch ✅ Resolved

- **File(s):** `authz/prefixlist/cache.go:207`, `authz/prefixlist/utils.go:66`
- **Dimension(s):** Maintainability
- **Priority:** Medium (P3)
- **Resolved in:** 85c64fb
- **Description:** Both `CachingFetcher.fetchJSON` and `FetchTextLines` created a fresh `*http.Client` per invocation, bypassing connection-pool reuse.
- **Outcome:** Shared `defaultFetchClient` created once; no per-call `*http.Client` allocation.

---

### F13. `WithCORS` silently discards error from `CORSHandler` ✅ Resolved

- **File(s):** `http/server.go:158-160`
- **Dimension(s):** Correctness
- **Priority:** Medium (P3)
- **Resolved in:** 98e5326
- **Description:** `WithCORS` discarded the error return from `CORSHandler`, silently swallowing any CORS misconfiguration.
- **Outcome:** Error return removed from `CORSHandler`; `WithCORS` no longer needs to discard it.

---

### F14. `metrics.Listener.acceptedCount` is not protected ✅ Resolved

- **File(s):** `metrics/listener.go:38`
- **Dimension(s):** Correctness
- **Priority:** Medium (P3)
- **Resolved in:** e091417
- **Description:** `l.acceptedCount += 1` was not thread-safe; `AcceptedCount()` and `ResetMetrics()` could be called concurrently from a monitoring goroutine.
- **Outcome:** `acceptedCount` changed to `atomic.Int64`. Complexity delta: `Accept` 3→3 (unchanged).

---

### F15. `http.Server.Mux` is an exported field ✅ Resolved

- **File(s):** `http/server.go:67`
- **Dimension(s):** Maintainability
- **Priority:** Medium (P3)
- **Resolved in:** 3ee2aa0
- **Description:** The exported `Mux` field allowed callers to register handlers that bypass the server's middleware chain (logging, metrics, request IDs).
- **Outcome:** `Mux` unexported to `mux`. Callers use `AddHandler`/`AddHandlerFunc`.

---

### F16. `weaveworks/common` is archived and leaks through the public API ✅ Resolved

- **File(s):** `http/server.go:178`
- **Dimension(s):** Maintainability, Security
- **Priority:** Medium (P3)
- **Resolved in:** 47d992a
- **Description:** `WithTelemetryInstrument` exposed `middleware.Instrument` from the archived/unmaintained `weaveworks/common` in a public function, coupling consumers to an unmaintained transitive dependency.
- **Outcome:** Defined a local `Instrument` interface. `weaveworks/common` removed from the public API surface.

---

### F18. `NetworkACL` has no port interface ✅ Resolved

- **File(s):** `authz/`, `http/authz/ip/`
- **Dimension(s):** Architecture
- **Priority:** Low (P4)
- **Resolved in:** fed318b
- **Description:** All listeners and HTTP handlers held `*authz.NetworkACL` concrete pointers. Tests had to construct the real type.
- **Outcome:** Added `Authoriser` interface with `Authorise`, `AuthoriseFromString`, and `AuthoriseConn` methods. `Listener` and `ip.Handler` now depend on the interface.

---

### F20. `http/resource/log_level.go` — duplicate route patterns ✅ Resolved

- **File(s):** `http/resource/log_level.go:122-125`
- **Dimension(s):** Correctness
- **Priority:** Low (P4)
- **Resolved in:** d356686
- **Description:** Both `GET /{$}` (exact root) and `GET /` (catch-all) were registered, meaning unmatched sub-paths were handled by `GetIndex` via the catch-all. Likely unintentional.
- **Outcome:** Catch-all `GET /` and `POST /` routes removed. Unmatched paths now return 404.

---

### F22. `http.Client` validates config at call time instead of construction time ✅ Resolved

- **File(s):** `http/client.go:22-37`
- **Dimension(s):** Maintainability
- **Priority:** Low (P4)
- **Resolved in:** d41bf02
- **Description:** `checkConfig` was called on every `Request` invocation. A bad config produced an error far from where it was supplied.
- **Outcome:** Validation moved to `NewClient`. `NewClient` returns `(*Client, error)`. `checkConfig` deleted. Complexity delta: `NewClient` 0→3.

---

### F23. `EnforceTLSHandler.EnforceTLS` is an exported mutable field ✅ Resolved

- **File(s):** `http/enforce_tls_handler.go`
- **Dimension(s):** Maintainability
- **Priority:** Low (P4)
- **Resolved in:** 247dd9f
- **Description:** `EnforceTLS bool` could be mutated after the handler was wrapped into a middleware chain, silently changing security enforcement at runtime.
- **Outcome:** Added `NewEnforceTLSHandler(enforce bool) *EnforceTLSHandler` constructor. `EnforceTLS` field unexported.

---

### F24. Comment rot — large commented-out `Duration` block ✅ Resolved

- **File(s):** `http/resource/log_level.go:13-35`
- **Dimension(s):** Maintainability
- **Priority:** Low (P5)
- **Resolved in:** 118c26c
- **Description:** A large block of commented-out Go code (a `Duration` type) was left in place with no explanation.
- **Outcome:** Deleted.

---

### F25. `my_ip.go` uses plaintext HTTP ✅ Resolved

- **File(s):** `my_ip.go:14-17`
- **Dimension(s):** Security
- **Priority:** Low (P5)
- **Resolved in:** b7defc7
- **Description:** `http://ipv4.icanhazip.com` and `http://ipv6.icanhazip.com` used plaintext HTTP, allowing a MitM to substitute a different IP address.
- **Outcome:** Migrated to `https://`.
