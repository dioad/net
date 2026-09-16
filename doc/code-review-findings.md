# Production Code Review Findings — `github.com/dioad/net`

_Reviewed: 2026-09-16 — branch `master`_
_Status: **complete** — all 29 findings fixed as of 2026-09-17._

This reviewed **production code**, not test code. Existing tests were used
as a lens on intended behaviour (what a covering test implies the function
should do), not as the review target — a test asserting the wrong thing
successfully is a production-code finding, not a test-quality one. That's a
different pass from `doc/test-quality-findings.md`, which grades the test
suite's own assertion strength via mutation testing.

29 findings came out of the original pass (whole-repo, three parallel
reviewers plus direct verification of every finding before it was
reported). All 29 are now fixed, one conventional commit per finding, each
verified with a test that failed against the pre-fix code and passed after
(noted inline where that wasn't practical — a couple of items are
inspection-only or docs-only).

## Correctness

| Finding | Commit |
|---|---|
| SPF `Qualifier` silently dropped for all mechanisms except `all` (`smtp/spf/record.go:176`) | `71e4049` |
| Silent 255-byte truncation in `spf`/`dmarc`/`mtasts`/`tlsrpt` `String()` (shared `smtp/internal/txtchunk` helper added) | `f646dc2` |
| `MultiProvider.Contains` always false until `Prefixes()` is called (`authz/prefixlist/multiprovider.go:77`) | `c898fe3` |
| `CloseWrite` fallback never fires the `onClose` callback (`conn_closer.go:51`) | `ae58da4` |
| `SaveTLSCertificateToFile` leaks a file descriptor on error paths (`tls/certificate.go:109`) — inspection-only, a leaked fd isn't observable from a plain unit test | `bc91b05` |
| Duplicate ALPN protocols for ACME tls-alpn-01 (`tls/server_config.go:121`) | `c384f53` |
| `parseParams` rejects unrecognized DKIM tags, violating RFC 6376 3.2 (`smtp/dkim/record.go:106`) | `6485b59` |
| MTA-STS record separator differs by `Version` path (`smtp/mtasts/record.go:26`) | `41f70c3` |
| `Duration()` returns a bogus negative value before any I/O (`metrics/conn.go:1`) | `976185c` |
| `mergeResponseData` drops `PublicMessage` for non-map `Data()` payloads — decided: log a warning rather than build wire-format merge logic for a combination with zero precedent among this package's ~40 real call sites (`http/json/response.go:179`) | `3164426` |
| `NewPersistentCookieStore` silently session-only on zero `MaxAge` — decided: default to 30 days (`http/cookies.go:19`) | `80bbf8f` |
| `Sscanf` int parsing silently accepts trailing garbage (`http/http_marshal.go:368`) | `611d63b` |
| Liveness handler inconsistent with readiness/status — stopped at the first failure found while ranging a map (`http/health.go:38`) | `68d094f` |
| Middleware cardinality guard misses its own unmatched-route fallback (`http/metrics.go:99`) | `10bb89f` |
| `ReverseIP` silently no-ops for non-IPv4 input (`dns/ip.go:15`) | `121b326` |
| `Get()` loses request coalescing when a fetch fails — thundering herd (`httpcache/fetcher.go:126`) | `fdae505` |
| `HTTPHandler` freezes the MTA-STS policy at construction (`smtp/mtasts/handler.go:1`) | `aee1ece` |
| `FormatPolicy` doesn't enforce RFC 8461's `mx` requirement (`smtp/mtasts/policy.go:1`) — no signature change needed, already returned `(string, error)` | `03ddf15` |
| `formatRUA` has a stale unimplemented-encoding TODO — comma/`!` in rua URIs weren't percent-encoded per RFC 8460 3 (`smtp/tlsrpt/record.go:20`) | `bb9b56e` |

## Concurrency

| Finding | Commit |
|---|---|
| `LimitSource` is an unsynchronized exported field — data race confirmed under `-race` before the fix (`ratelimit/rate_limiter.go:54`) | `8e9cb05` |

## Least surprise / API design

| Finding | Commit |
|---|---|
| Silent TLS config-arm precedence, no operator guard — now logs a warning naming which arms are set and which wins (`tls/server_config.go:81`) | `04f5b01` |
| `NoContent` accepted a `Data()`/`PublicMessage()` option and would write a body on a status HTTP requires to have none — decided: log-and-ignore, not panic or error return (`http/json/response.go:1`) | `b530771` |
| `Deny(net *net.IPNet)` parameter shadows `net` package (`authz/network_acl.go:67`) | `0d79783` |
| `AddMapToHTTPHeader` clones instead of mutating — renamed to `MergedHTTPHeader` (`http/headers.go:12`) | `31757b9` |
| `Client.Request` forced `Content-Type: application/json` on every non-empty body regardless of caller intent (`http/client.go:38`) | `c62b7f5` |
| Negative rps/burst silently clamped with no warning (`ratelimit/rate_limiter.go:84`) | `da30a52` |
| `RateLimiter`'s zero value was not safe to use — panicked on first `Allow()`/`Stop()` (`ratelimit/rate_limiter.go:42`) | `0b54d93` |

## Resolved design decisions

Three findings needed an explicit decision from the repo owner before they
could be implemented; all three are now decided and fixed.

### `http/cookies.go` — `Base64EncryptionKey` wiring

**Decision:** encryption stays optional (keeps cookie contents inspectable
for local debugging) and must support key rotation.

`CookieConfig.Base64AuthenticationKey`/`Base64EncryptionKey` became an
ordered `[]CookieKeyPair` (`KeyPairs`), matching
`gorilla/sessions.NewCookieStore`'s native `(auth, encryption)` key-pair
rotation mechanism — confirmed by reading `securecookie.CodecsFromPairs`
that the encryption slot can be `nil` for any pair, not only the last.
Breaking config-shape change, but since `Base64EncryptionKey` was already
silently unused, no deployment relied on the old shape actually encrypting
anything (confirmed no sibling `dioad` repo uses this package's
`CookieConfig` at all). Fixed in `fa34b40`.

### `http/cookies.go` — `NewPersistentCookieStore` `MaxAge` default

**Decision:** use a sensible default — 30 days when `MaxAge` is left at
its zero value. Fixed in `80bbf8f`.

### `http/client_ip.go` — trusted-proxy design for `GetClientIP`

**Decision:** address the fly.io deployment first, then generalize so the
same mechanism covers direct internet traffic and other reverse proxies
(AWS/Azure application gateways, Cloudflare, etc).

Research (see commit `cacc0b2` and `728750e` for full detail and sources):
fly.io doesn't publish a stable edge IP range, so a classic CIDR-based
trusted-proxy allowlist isn't practical there specifically — but
`Fly-Client-IP` is trustworthy by **topology** (Fly Proxy is the sole
author of that header, not a value copied from client input), not by
validating a source address. Separately, confirmed against MDN's
`X-Forwarded-For` reference that Fly's append-to-the-right behavior is the
standard convention applied correctly, and that `GetClientIP`'s
leftmost-entry parsing was the specific anti-pattern the standard warns
against for security-relevant use.

Implemented as `ClientIPResolver` (`daac4af`) with two pluggable trust
modes:

- `WithTrustedHeader` / `NewFlyClientIPResolver` — trusts one named header
  verbatim, for platforms whose edge sets a header this process can only
  receive by going through that edge (fly.io's `Fly-Client-IP`).
- `WithTrustedProxyCIDRs` — the general RFC 7239-style mechanism: requires
  the direct peer (`RemoteAddr`) to itself be in the trusted set before
  consulting `X-Forwarded-For` at all, then walks the header from the
  right, stopping at the first untrusted entry. For AWS ALB, Azure
  Application Gateway, Cloudflare, self-hosted nginx, and similar
  platforms that do publish stable ranges.

`GetClientIP` itself is unchanged — still available, still documented as
unsafe, for callers that explicitly want it. `ClientIPPrincipalFunc` is no
longer `http.NewRateLimiter`'s silent default: `NewRateLimiter` now panics
unless a `PrincipalFunc` is configured via `WithPrincipalFunc` (confirmed
via workspace-wide grep that every real caller already passes one
explicitly, so this closes the gap for a future caller rather than
breaking an existing one).
