# Production Code Review Findings — `github.com/dioad/net`

_Reviewed: 2026-09-16 — branch `master`_

This reviews **production code**, not test code. Existing tests were used as
a lens on intended behaviour (what a covering test implies the function
should do), not as the review target — a test asserting the wrong thing
successfully is a production-code finding, not a test-quality one. That's a
different pass from `doc/test-quality-findings.md`, which grades the test
suite's own assertion strength via mutation testing.

29 findings came out of the original pass (whole-repo, three parallel
reviewers plus direct verification of every finding before it was reported).
The 7 most severe were fixed this session; this document is the remaining
22, ranked, for whoever picks this up next.

## Already fixed this session

| # | Finding | Commit |
|---|---|---|
| 1 | SPF `Qualifier` silently dropped for all mechanisms except `all` | `71e4049` |
| 2 | `MultiProvider.Contains` always false until `Prefixes()` is called | `c898fe3` |
| 3 | `CloseWrite` fallback never fires the `onClose` callback | `ae58da4` |
| 4 | Silent 255-byte truncation in `spf`/`dmarc`/`mtasts`/`tlsrpt` `String()` | `f646dc2` |
| 6 | `SaveTLSCertificateToFile` leaks a file descriptor on error paths | `bc91b05` |
| 7 | Duplicate ALPN protocols for ACME tls-alpn-01 | `c384f53` |
| 5 | `GetClientIP` trusts spoofable proxy headers | `1d1f5f7` (**docs only** — see [Deferred design decisions](#deferred-design-decisions) below; still open, needs more thought from the owner before the actual mitigation is designed) |

## How to work this list

Same discipline as the fixed items above and as `test-quality-findings.md`:
one conventional commit per finding, write or strengthen a test that fails
against the current code and passes after the fix (where the bug is
observable from a test at all — a couple of items below are noted as
inspection-only), then run the repo's pre-completion checks
(`go build .`, `go fmt ./...`, `go vet ./...`, `go test -race ./...`) before
committing.

---

## Correctness

### `smtp/dkim/record.go:106` — `parseParams` rejects unrecognized DKIM tags
RFC 6376 §3.2 requires unrecognized tags on a DKIM key record to be ignored,
not rejected; `parseParams` returns an error for any tag outside its
`validParams` allowlist.

**Failure scenario:** `FromRecordFile` fails outright on any fully
RFC-legal DKIM key record that includes common tags like `h=`, `t=`, `s=`,
`g=`, or `n=` — real-world DKIM records commonly carry these.

**Fix shape:** drop the allowlist rejection; skip tags not in the map
instead of erroring. No API signature change.

### `smtp/mtasts/record.go:26` — MTA-STS record separator differs by `Version` path
The empty-`Version` branch writes `"v=STSv1; "` (with semicolon) but the
explicit-`Version` branch writes `fmt.Sprintf("v=%s ", r.Version)` (no
semicolon) — an RFC 8461 §3.1-noncompliant separator whenever `Version` is
set explicitly.

**Failure scenario:** `Record{Version:"STSv1", ID:"x"}.String()` produces
`"v=STSv1 id=x"` (missing the required semicolon) while the default-`Version`
case correctly produces `"v=STSv1; id=x"` for the same semantic record.

**Fix shape:** make both branches emit the same separator; likely simplest
to build the version token once and always append `"; "`.

### `metrics/conn.go:1` — `Duration()` returns a bogus negative value before I/O
`endTime` starts as the zero `time.Time` while `startTime` is set at
construction, so `Duration()` computes `zero.Sub(now)` — a large negative
duration — until the first byte transfers.

**Failure scenario:** a connection accepted and closed immediately with zero
bytes transferred logs a garbage multi-century negative duration via
`NewConnWithLogger`'s unconditional close-time `Duration()` log, instead of
~0.

**Fix shape:** `Duration()` should treat a zero `endTime` as "not yet
measured" — e.g. return `0` or `time.Since(startTime)` — rather than
subtracting the zero value literally.

### `http/json/response.go:179` — `mergeResponseData` drops message for non-map data
`mergeResponseData`'s `map[string]any` type assertion fails for any ordinary
struct payload, and the function returns `data` unchanged, discarding the
`PublicMessage` instead of erroring or embedding it.

**Failure scenario:** `r.OK(Data(myStruct), PublicMessage("done"))` where
`myStruct` is a normal Go struct (the overwhelmingly common case) never
surfaces `"done"` anywhere in the response, with no error or warning that
the option was ignored.

**Fix shape:** needs a decision on the wire shape for a struct payload plus
a message (wrapper envelope vs. reflection-based field injection) — worth
a short design note before implementing, not a pure bug fix.

### `http/cookies.go:19` — `NewPersistentCookieStore` silently session-only on zero `MaxAge`
`store.MaxAge(config.MaxAge)` with the Go zero value (`0`) sets
`Options.MaxAge=0`, meaning "no Max-Age attribute" (session-only) — the
opposite of what a function named `NewPersistentCookieStore` promises —
with no validation rejecting a zero/negative `MaxAge`.

**Failure scenario:** a caller constructs `CookieConfig` without setting
`MaxAge` (an easy omission, since nothing requires it) and gets
session-only cookies despite calling the "persistent" constructor.

**Status: decided, ready to implement.** See
[Deferred design decisions](#deferred-design-decisions) for the chosen
default and rationale.

### `http/http_marshal.go:368` — `Sscanf` int parsing silently accepts trailing garbage
`unmarshalIntField`/`unmarshalUintField` use
`fmt.Sscanf(values[0], "%d", &n)`, which — unlike `strconv.ParseInt` —
succeeds without requiring the whole string to match the verb.

**Failure scenario:** a query or header int field submitted as
`"5; DROP TABLE"`, `"10ms"`, or `"100<script>"` silently parses as `5`,
`10`, `100` respectively instead of being rejected as invalid input, for
both `UnmarshalQuery*` and `UnmarshalHeader*`.

**Fix shape:** replace with `strconv.ParseInt`/`ParseUint`, which fails
closed on trailing characters. Small, self-contained fix.

### `http/health.go:38` — liveness handler inconsistent with readiness/status
`aggregateReadinessHandler` and `aggregateStatusHandler` continue past a
failing resource and report every failure in the response;
`aggregateLivenessHandler` breaks after the first failure found while
ranging a map (Go map iteration order is randomized per run) and returns
only `{"live": false}` with no detail.

**Failure scenario:** with two simultaneously-unhealthy resources, which one
is reported varies run to run, and an operator debugging a liveness failure
gets strictly less diagnostic information than the analogous
readiness/status failure would provide.

**Fix shape:** make liveness collect and report all failures like its
siblings, or explicitly document why liveness intentionally short-circuits
(if that's deliberate for latency reasons) and make the short-circuit
deterministic rather than map-order-dependent.

### `http/metrics.go:99` — cardinality guard misses its own stated threat
The doc comment says this design prevents high-cardinality Prometheus
series from raw URL paths, but `route` falls back to the raw `r.URL.Path`
whenever `mux==nil` or the request doesn't match a registered pattern —
exactly the traffic shape of adversarial scanning (every 404).

**Failure scenario:** a scanner probing thousands of distinct nonexistent
paths produces thousands of distinct Prometheus label values via the
unmatched-route fallback, the exact cardinality blowup the middleware's own
doc comment claims to prevent.

**Fix shape:** fall back to a fixed label (e.g. `"unmatched"`) instead of
the raw path when there's no registered-pattern match.

### `dns/ip.go:15` — `ReverseIP` silently no-ops for non-IPv4 input
The doc comment claims to return "the reverse DNS notation for an IP
address" with no IPv4-only qualifier, but `ReverseIP` returns `("", nil)`
for any non-IPv4 input, and its only caller (`BlocklistLookupAddr`) doesn't
check for the empty result.

**Failure scenario:** `BlocklistLookupAddr` on an IPv6 address proceeds to
query `net.LookupHost(".zen.spamhaus.org")` — a malformed hostname — and the
lookup fails, so an IPv6 blocklist check silently and always reports "not
listed" rather than actually checking or erroring loudly.

**Fix shape:** either implement IPv6 reverse notation (`ip6.arpa`) or have
`BlocklistLookupAddr` reject/error on a non-IPv4 address instead of
proceeding with an empty prefix.

### `httpcache/fetcher.go:126` — `Get()` loses request coalescing when a fetch fails
Waiters woken from `refreshCond.Wait()` check `cachedData == nil` and,
seeing it still nil after a failed fetch, each independently issue their
own `doFetch` instead of re-checking `f.refreshing` and waiting again.

**Failure scenario:** a failing origin causes every waiting caller to
independently re-fetch simultaneously — a thundering herd against the
origin — exactly when coalescing matters most (an already-struggling
upstream).

**Fix shape:** loop on the wait condition (re-check `f.refreshing` before
falling through to a fresh fetch) rather than treating "still nil" as
license for every waiter to fetch.

### `smtp/mtasts/handler.go:1` — `HTTPHandler` freezes the MTA-STS policy at construction
The handler renders `*Policy` to a string once when constructed and serves
that frozen string forever, even though it's handed a pointer the caller
may reasonably expect to mutate and have reflected on subsequent requests.

**Failure scenario:** a caller updates the `*Policy` the handler was
constructed with (e.g. rotating MX hosts) expecting the running HTTP
endpoint to reflect it; every request keeps serving the original policy
text indefinitely.

**Fix shape:** render on each request instead of at construction, or make
the pointer-vs-snapshot contract explicit in the constructor's doc comment
if the freeze is intentional.

### `smtp/mtasts/policy.go:1` — `FormatPolicy` doesn't enforce RFC 8461's `mx` requirement
`FormatPolicy` has no error return and performs no validation, so a
`Policy` with an empty `MX` slice silently produces a spec-invalid MTA-STS
policy document.

**Failure scenario:** a misconfigured `Policy{MX: nil}` serializes to a
policy body missing any `mx` line, which is invalid per RFC 8461 but is
published without any error surfaced to the caller.

**Fix shape:** this changes `FormatPolicy`'s signature to return an error —
worth confirming callers can accommodate that before doing it as part of
the same commit that adds the validation.

### `smtp/tlsrpt/record.go:20` — `formatRUA` has a stale unimplemented-encoding TODO
A comment says "Need to encode" but no encoding is performed before joining
`rua` URIs with commas per RFC 8460.

**Failure scenario:** an `https://` rua URI containing a comma in its query
string is joined into the comma-separated rua list unescaped, corrupting
the list into what looks like two separate URIs to any RFC-compliant
parser.

**Fix shape:** percent-encode the comma (and any other RFC 8460 delimiter
characters) in `https://` URIs before joining, or reject URIs containing an
unescaped comma at config time.

---

## Concurrency

### `ratelimit/rate_limiter.go:54` — `LimitSource` is an unsynchronized exported field
`LimitSource` is read without a lock on every `Allow()` call (line ~308) but
is a plain exported field with no synchronized setter; its own doc comment
calls it a source for "dynamic rate limits," implying runtime updates are
expected, yet nothing protects concurrent mutation against concurrent
reads.

**Failure scenario:** a caller swaps `rl.LimitSource = newSource` at
runtime (the natural way to hot-reload limits for something documented as
"dynamic") concurrently with in-flight `Allow()` calls from other
goroutines — a data race that `go test -race` would catch if any test
exercised it, which none currently do.

**Fix shape:** either make `LimitSource` unexported with a synchronized
setter method, or wrap reads/writes with the existing mutex if one is
already on the struct. Write the concurrent-mutation test first — it
should fail under `-race` against current code.

---

## Least surprise / API design

### `tls/server_config.go:81` — silent TLS config-arm precedence, no operator guard
When more than one of ACME/SelfSigned/LocalConfig is non-zero-valued in a
`ServerConfig`, the first in fixed precedence order wins with no error or
warning logged — documented in a comment, but the code itself does nothing
to protect an operator from a likely-accidental misconfiguration.

**Failure scenario:** an operator switches from ACME to a local dev cert by
adding `LocalConfig` but forgets to remove the old ACME section; the server
silently keeps using ACME (making live network calls) with zero indication
that the `LocalConfig` they just added is being ignored.

**Fix shape:** log a warning (at minimum) when more than one arm is
non-zero-valued, naming which one won. A hard error is also defensible but
is a behavior change worth confirming with the user first.

### `http/json/response.go:1` — `NoContent` accepts a `Data()` option
The option system for building responses doesn't guard against
semantically invalid combinations — `NoContent(Data(x))` is representable
and would attempt to write a body on a status code that HTTP requires to
have none.

**Failure scenario:** a caller mistakenly passes `Data(...)` to `NoContent`
(easy to do since both take `...Option`) and gets a 204 response carrying a
body, which violates HTTP semantics and confuses well-behaved clients that
assume 204 has none.

**Fix shape:** have `NoContent` reject (panic, error, or log-and-ignore —
needs a decision) an incompatible `Data()` option rather than silently
emitting an invalid response.

### `authz/network_acl.go:67` — `Deny(net *net.IPNet)` parameter shadows `net` package
The parameter is named `net`, shadowing the imported `net` package within
the function body; harmless today since the body never references the
package, but inconsistent with the sibling `Allow(n *net.IPNet)` two
methods above, which correctly avoids the shadow.

**Failure scenario:** a future edit to `Deny` that needs `net.ParseCIDR` or
similar will either fail to compile with a confusing error or, in a worse
case, silently resolve to an unintended identifier if one is ever
introduced with a matching shape.

**Fix shape:** rename the parameter to `n`, matching `Allow`. Trivial,
one-line, no behavior change.

### `http/headers.go:12` — `AddMapToHTTPHeader` clones instead of mutating
Both the function name ("Add...To...") and doc comment describe adding
entries to an existing `http.Header`, but the implementation clones the
header and returns a new one without mutating the argument.

**Failure scenario:** a future caller writes `AddMapToHTTPHeader(h, extra)`
without capturing the return value — which the name strongly implies is
safe, the same way `append()` is often mis-called — and silently gets no
effect at all.

**Fix shape:** either mutate `h` in place (matching the name) or rename to
something like `MergedHTTPHeader` that documents the clone-and-return
contract. A naming fix is lower-risk than changing mutation semantics for
any existing callers relying on the clone.

### `http/client.go:38` — `Client.Request` forces `Content-Type: application/json`
The client is documented as "a generic HTTP client" yet `Request` silently
labels every non-empty body as JSON regardless of actual content, requiring
callers sending form data, XML, or binary payloads to remember to override
it afterward via `RequestModifier`.

**Failure scenario:** a caller sends a non-JSON body through this "generic"
client without overriding the header and the request is mislabeled
`Content-Type: application/json` to the receiving server, which may reject
or misinterpret it.

**Fix shape:** only default the header when unset by the caller (check
first via a `RequestModifier` or an explicit content-type parameter),
rather than always stamping it.

### `ratelimit/rate_limiter.go:84` — negative rps/burst silently clamped with no warning
`NewRateLimiterWithConfig` and its siblings clamp caller-supplied negative
`requestsPerSecond`/`burst` values to `0` in-place with no error return and
no log line, silently turning an invalid configuration into a limiter that
blocks everything.

**Failure scenario:** an operator passes a negative rps due to a
config-parsing bug upstream (e.g. a bad default or unit conversion) and
gets a fully-functional-looking `RateLimiter` that silently rejects all
traffic, with nothing in logs pointing at the actual misconfiguration.

**Fix shape:** log a warning (the limiter already takes a logger) when
clamping occurs, naming the field and the value that was rejected.

### `ratelimit/rate_limiter.go:42` — `RateLimiter`'s zero value is not safe to use
`var rl ratelimit.RateLimiter` (the exported type's zero value) has a nil
`limiters` map and a nil `cancel` func; calling `Allow()` on it panics with
"assignment to entry in nil map" rather than a clear, actionable error, and
`Stop()` would panic on the nil `cancel`.

**Failure scenario:** a caller unfamiliar with the required constructor
pattern writes `var rl ratelimit.RateLimiter` (a natural first attempt for
an exported struct type) and gets a low-level nil-map panic on the first
`Allow()` call instead of a clear signal that a constructor is required.

**Fix shape:** either make the zero value safe (lazy-init the map on first
`Allow()`, no-op `Stop()` when `cancel` is nil) or add a doc comment on the
type making the "must use a constructor" contract explicit — the Go
convention (per this repo's own style guide: "make the zero value useful")
favors the former if it's not too invasive.

---

## Deferred design decisions

These items needed a decision from whoever owns this repo's API surface
before implementation. Two are now decided; one is still open pending
further thought from the owner.

### `http/client_ip.go` — trusted-proxy allowlist for `GetClientIP`

**Status: still open — needs more thought, not ready to implement.**
(Owner note, 2026-09-17: one of the intended deployments sits behind the
fly.io platform proxy, and it wasn't clear whether a trusted-proxy source
IP is even something that can be determined programmatically for that
case. Researched below; the finding changes the shape of the eventual fix
but doesn't settle it.)

**Research: what fly.io actually guarantees.**

- Fly Proxy sets `Fly-Client-IP` to "the IP address of the client from the
  perspective of Fly Proxy" — this is Fly's own direct observation of the
  TCP peer it accepted a connection from, not a value copied out of any
  client-supplied header. A client cannot set or override it, because it
  isn't derived from client input at all. [Fly Docs — Request headers]
- Fly Proxy **appends** to `X-Forwarded-For` rather than replacing it —
  and this is Fly following the standard convention correctly, not a
  Fly-specific quirk. Per MDN's `X-Forwarded-For` reference: "the
  rightmost IP address is the IP address of the most recent proxy and the
  leftmost IP address is the address of the originating client (assuming
  well-behaved client and proxies)," and "any security-related use of
  X-Forwarded-For... must only use IP addresses added by a trusted proxy
  ... [l]eftmost (untrusted) values must only be used for cases where
  there is no negative impact from using spoofed values." `GetClientIP`
  currently takes the *first* (leftmost) entry unconditionally — the
  specific anti-pattern that guidance warns produces "rate-limiter
  avoidance, access-control bypass" — so this isn't only a fly.io
  mismatch, it's wrong against the general standard whenever XFF is used
  for anything security-relevant, which is exactly how `ClientIPPrincipalFunc`
  and the access-log field both use it today. [MDN — X-Forwarded-For;
  Fly Docs — Request headers]
- A Fly staffer confirmed that `X-Forwarded-Port` — and, per the same
  answer, headers other than `X-Forwarded-For` generally — do **not** get
  the append treatment: clients can fully overwrite them. The `Forwarded`
  header isn't addressed explicitly in Fly's docs; treat it as spoofable
  like `X-Forwarded-Port` until proven otherwise. [Fly community: "Why are
  clients allowed to spoof the X-Forwarded-Port header?"]
- Fly does **not** publish a stable/static IP range for its edge proxy —
  fly-proxy sits on an anycast network, and a community request for a
  published range has stood unanswered. A classic CIDR-based
  trusted-proxy allowlist (RFC 7239-style: trust these source IPs, walk
  `X-Forwarded-For` from the right until leaving the trusted set) **is not
  practical to build against fly.io specifically** — there's no stable
  range to allowlist against. [Fly community: "Request: Fly IP ranges"]

**What this means:** "hard to know the IP programmatically" turns out to
be true for the CIDR-allowlist approach, but not fatal — fly.io offers a
different trust model that sidesteps needing an IP range at all.
`Fly-Client-IP` is trustworthy by **topology**, not by validating a source
address: as long as the app is only reachable by going through fly-proxy
(the normal fly.io deployment shape — Fly Machines aren't directly
Internet-routable otherwise), fly-proxy is the sole author of that header
for that hop, so there's nothing to spoof around. That guarantee breaks if
another CDN or proxy is placed in front of fly.io, in which case
`Fly-Client-IP` would reflect that intermediate hop instead of the real
client — a caveat worth keeping in mind if that topology is ever in scope.

**Recommended shape** (not yet a final decision — this is what the
research points toward, still needs owner sign-off): make the trust model
pluggable rather than hard-coding one platform's convention into
`GetClientIP`:

- A `TrustedHeader` mode: trust one named header verbatim (e.g.
  `Fly-Client-IP`), for platforms whose edge sets a header the app can
  only receive by going through that edge — no IP-range knowledge needed.
- A `TrustedProxyCIDRs` mode: RFC-7239-style, walk `X-Forwarded-For` from
  the right against a caller-supplied CIDR set, for platforms that *do*
  publish stable ranges (AWS ALB, GCP LB, self-hosted nginx, etc.) — not a
  fit for fly.io today, but worth keeping as the general-purpose option
  for other deployments of this package.
- Keep today's "trust the first `X-Forwarded-For` entry unconditionally"
  behavior available, but opt-in and clearly labeled unsafe, rather than
  the default `GetClientIP` falls back to.

Still open: which mode(s) to build first, the exact config surface, and
whether `ClientIPPrincipalFunc` (the rate limiter's default principal)
should require one of the safe modes to be configured rather than
defaulting to the unsafe one.

Sources:
- [X-Forwarded-For · MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/X-Forwarded-For)
- [Request headers · Fly Docs](https://fly.io/docs/networking/request-headers/)
- [Why are clients allowed to spoof the X-Forwarded-Port header? — Fly.io community](https://community.fly.io/t/why-are-clients-allowed-to-spoof-the-x-forwarded-port-header/3278)
- [Request: Fly IP ranges — Fly.io community](https://community.fly.io/t/request-fly-ip-ranges/127)

### `http/cookies.go` — `Base64EncryptionKey` wiring

**Status: decided, ready to implement.** (Owner decision, 2026-09-17:
encryption stays optional, to keep cookie contents inspectable for local
debugging; key rotation must be supported.)

`gorilla/sessions.NewCookieStore(keyPairs ...[]byte)` already supports
both requirements natively, per its own doc comment: keys are passed as an
ordered sequence of `(auth, encryption)` pairs; the encryption key in a
pair may be nil/omitted (optional); and passing multiple pairs is exactly
gorilla's rotation mechanism — the first pair signs/encrypts new cookies,
and older pairs are still tried when decoding a cookie issued before a
rotation.

That means `CookieConfig` needs to grow from a single `(auth, enc)` string
pair to an **ordered list** of pairs:

```go
type CookieKeyPair struct {
	Base64AuthenticationKey string `mapstructure:"base64-authentication-key"`
	Base64EncryptionKey     string `mapstructure:"base64-encryption-key"` // optional
}

type CookieConfig struct {
	// KeyPairs is ordered newest-first: the first pair signs/encrypts new
	// cookies, and every pair is tried when decoding, so old pairs can be
	// kept around during a rotation window and then dropped.
	KeyPairs []CookieKeyPair `mapstructure:"key-pairs"`
	MaxAge   int             `mapstructure:"max-age"`
	Domain   string          `mapstructure:"domain"`
}
```

`NewSessionCookieStore` would decode each pair's keys and build the
`keyPairs ...[]byte` slice gorilla expects, passing `nil` for a pair's
encryption key when `Base64EncryptionKey` is unset.

This is a breaking config-shape change (single fields become a list), but
since `Base64EncryptionKey` was already silently unused, no existing
deployment relies on the old shape actually encrypting anything — this is
a reasonable point to make the change rather than bolting rotation onto
the single-pair shape and breaking it again later.

### `http/cookies.go` — `NewPersistentCookieStore` `MaxAge` default

**Status: decided, ready to implement.** (Owner decision, 2026-09-17: use
a sensible default.)

When `config.MaxAge` is `0` (unset), default to **30 days**
(`30 * 24 * 60 * 60` = `2592000` seconds) instead of passing the zero
value through to `store.MaxAge()`, which currently produces session-only
cookies — the opposite of what "persistent" promises. 30 days is a common
default lifetime for persistent session cookies and a reasonable balance
for this package's likely use case (long-lived auth sessions) without
being indefinite; a caller that wants a different lifetime still sets
`MaxAge` explicitly, this only changes what happens when it's left at the
Go zero value.

```go
// DefaultPersistentCookieMaxAge is used by NewPersistentCookieStore when
// CookieConfig.MaxAge is left at its zero value.
const DefaultPersistentCookieMaxAge = 30 * 24 * 60 * 60 // 30 days, in seconds

func NewPersistentCookieStore(config CookieConfig) (*sessions.CookieStore, error) {
	store, err := NewSessionCookieStore(config)
	if err != nil {
		return nil, err
	}

	maxAge := config.MaxAge
	if maxAge == 0 {
		maxAge = DefaultPersistentCookieMaxAge
	}
	store.MaxAge(maxAge)

	return store, nil
}
```

A caller that explicitly wants the cookie deleted immediately (gorilla's
documented meaning for `MaxAge < 0`) is unaffected — only the unset-zero
case changes.
