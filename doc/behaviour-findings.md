# Behaviour Findings — `github.com/dioad/net`

_Method: use the existing test suite as a lens on the application code's actual
behaviour, not as the subject of review. For each test, the question is not
"does it pass" but "is the behaviour it pins down correct, least-surprising,
and intuitive to a downstream Go consumer who has read only the godoc."_

_Scope note: `claude-review-architecture.md.original` already covers static
structural/API findings (all 29 marked fixed as of `5dfe169`). This doc does
not re-litigate those; it looks for *behavioural* defects that only show up by
reading what a passing test actually asserts._

_Started: 2026-09-18 — branch `master`_

_Examined and clear (a real behavioural check was done; no defect found):
`http/client_ip.go` (`ClientIPResolver` trust-mode tests), `authz/network_acl.go`
(allow/deny precedence), `http/authz/ip` (panic-on-bad-config fix holds up),
`http/cookies.go` (key rotation, encryption-optional), `http/json/response.go`
(`NoContent`/`PublicMessage` drop handling), `ratelimit/listener.go`,
`authz/prefixlist/multiprovider.go` (stale-cache-on-partial-failure — looks
safe given the fetcher's own stale fallback, but not exhaustively proven),
`authz/gating_listener.go` + `authz/network_acl_listener.go` (original
review's Finding 1/7 fix holds up), `doneConn.CloseWrite` (fallback-to-full-close
is explicitly and adequately documented, not a hidden surprise),
`smtp/internal/txtchunk`, `smtp/mtasts/{policy,record}` (long-value chunking
is not lossy), `dns/record_test.go`, `dns/tlsa_record_test.go` (prefix/dot
formatting, no case-sensitivity contradiction found), `tls/server_config.go`
(multi-arm warning added in `04f5b01` is honest about its own precedence)._

_Not examined this pass — a follow-up sweep should start here:
`http/*_marshal.go`, `http/server.go`, `http/logging.go`, `http/metrics.go`,
`http/tls_error_filter.go`, `http/request_id.go`, `http/body_size.go`,
`metrics/` package, remainder of `tls/` (`auto_cert`, `cert`, `certificate`,
`dns01`, `self_signed`), `dns/doh_client.go`, `dns/ip.go`,
`dns/refreshable_content.go`, `dns/auto_refresh.go` (skimmed only),
`smtp/dkim`, `smtp/dmarc`, `smtp/mx`, `smtp/tlsrpt`,
`authz/prefixlist/{factory,integration,providers}_test.go` (network-hitting
provider tests, `factory.go` error-propagation on construction)._

## B1. `authz.IsPrincipalAuthorised` fails open with no rules configured — inconsistent with its sibling `NetworkACL`

**Status:** Resolved in `dioad/net`. `IsPrincipalAuthorised` now takes an explicit `allowByDefault bool` parameter mirroring `NetworkACL.AllowByDefault`; `PrincipalACLConfig` gained a matching `AllowByDefault` field (`mapstructure:"allow-by-default"`, zero value `false`/deny). `authz/principal_acl_test.go` was rewritten to cover both `allowByDefault` values for the "no rules configured" cases.

Cross-repo consequence, not yet resolved: `dioad/auth`'s `http/authz/principal.Handler.AuthRequest` (the only other in-workspace caller) was updated to pass `true` explicitly, preserving its existing allow-all-when-unconfigured behaviour exactly rather than silently flipping it in this change. That preservation surfaced a concrete, real instance of the same fail-open pattern one layer up: `dioad/connect`'s `authmiddleware.New` builds an effective ACL that is empty (and therefore allow-all under `auth`'s current `allowByDefault: true`) whenever a tunnel has no `tunnelPrincipal` and no explicit ACL configured — filed as [dioad/connect#490](https://github.com/dioad/connect/issues/490). Whether `auth`/`connect` should switch to deny-by-default is an operator decision outside this repo's scope, tracked in that issue rather than left dangling here.

**Files:** `authz/principal_acl.go:4-24`, pinned by `authz/principal_acl_test.go:15` (`"no acl"` case, `expected: true`)

```go
func IsPrincipalAuthorised(user string, allowList []string, denyList []string) bool {
	principalAuthorised := true   // <- fail-open zero value
	if len(allowList) > 0 {
		principalAuthorised = false
		...
```

The test's `"no acl"` case (`allowList: nil, denyList: nil, expected: true`) is passing and accurately documents the actual behaviour: with no rules configured at all, **every principal is authorised**. That is the correct implementation of what the code does — but the code does the wrong thing.

The same package's other ACL type, `NetworkACL`, treats "no rules configured" as a decision the caller must make explicitly via `AllowByDefault` (defaulting to `false`/deny — see `TestAuthoriserDenyByDefault` in `authz/network_acl_test.go:62`). `IsPrincipalAuthorised` has no equivalent knob: it hardcodes allow-all as its only unconfigured behaviour. A downstream caller who has used `NetworkACL` in the same package and reads `IsPrincipalAuthorised`'s one-line godoc ("checks if a user is authorised based on allow and deny lists") has every reason to expect the same deny-by-default posture. Instead, forgetting to populate `allowList`/`denyList` (e.g. a config-loading bug that leaves both nil) silently authorises everyone — a fail-open default is exactly the "surprising" case CLAUDE.md's "make the zero value useful" principle warns against for access-control code.

`gograph query IsPrincipalAuthorised` shows zero production call sites inside this repo — it is only exercised by its own test, so the blast radius is entirely in downstream consumers (`dioad/auth`, `dioad/cli`, etc.) that this repo cannot see.

**Suggested fix:** add an explicit `allowByDefault bool` parameter (or wrap in a small config struct mirroring `NetworkACLConfig`) so "no rules" is a caller decision, not a hardcoded allow-all. At minimum, flip the hardcoded default to deny-when-unconfigured to match `NetworkACL`'s posture, and update the test's `"no acl"`/`"empty allow"`/`"empty deny"` cases to `expected: false` (or add a new parameter to the table for the default value).

---

## B2. `ratelimit.NewRateLimiterWithSource` silently defaults unmatched principals to deny-everything, contradicting its own doc comment

**Status:** Resolved via the warn-don't-change-behaviour approach (matching the precedent set by `04f5b01`'s TLS multi-arm warning, rather than changing the deny-on-zero-value semantics). `Allow` now logs a warning exactly once per `RateLimiter` when a `limitSource` is configured, returns `ok=false` for a principal, and no static fallback limits were set. `GetLimit`'s doc comment now states the zero-value-fallback risk explicitly. Covered by `TestRateLimiter_WarnsWhenSourceFallbackIsZeroValue` in `ratelimit/rate_limiter_test.go`.

**Files:** `ratelimit/rate_limiter.go:156-196` (`NewRateLimiterWithSource`, `NewRateLimiterWithSourceAndConfig`) and `ratelimit/rate_limiter.go:271-306` (`NewRateLimiterWithOptions` — "the preferred constructor" per its own doc comment — defaults `requestsPerSec: 0, burst: 0` at line 276 and never requires `WithRateLimiterStaticLimits` alongside `WithRateLimiterSource`), doc comment at `ratelimit/rate_limiter.go:26` (`RateLimitSource.GetLimit`); pinned by `ratelimit/rate_limiter_test.go:320-356` (`TestRateLimiter_WithSourceAndFallback`)

`RateLimitSource.GetLimit`'s doc comment promises: *"If it returns ok=false, the default limits of the RateLimiter will be used."* That reads as "there is a sensible fallback." In fact, `NewRateLimiterWithSource`/`NewRateLimiterWithSourceAndConfig` never set `requestsPerSecond`/`burst` on the constructed `RateLimiter` at all — they stay at the Go zero value, `0.0`/`0`. `rate.NewLimiter(0, 0)` denies every request unconditionally. So "the default limits" for any principal the source doesn't recognize is silent, total denial — with no warning logged (contrast `clampNonNegativeLimits`, which *does* warn when static limits are explicitly negative; an implicit zero-value static limit warns about nothing).

The test that exercises the fallback path, `TestRateLimiter_WithSourceAndFallback`, only gets a working (5 rps) fallback by *not* using `NewRateLimiterWithSource` at all — it constructs via the deprecated `NewRateLimiterWithConfig(5, 5, ...)` and then calls `SetLimitSource(source)` afterward, with this comment left in place by whoever wrote it:

```go
// Create a rate limiter with fallback limits first, then we'll set source
// This approach is not ideal but demonstrates fallback behavior
// In production, consider having the source always return ok=true
```

That comment is itself an admission that the natural, name-matching constructor for this exact use case (`NewRateLimiterWithSource`) doesn't give you a usable fallback — you have to know to route around it via a different, deprecated constructor plus a separate method call. A downstream caller who reads `NewRateLimiterWithSource`'s doc ("creates a new rate limiter with a custom rate limit source") and the `GetLimit` doc's promise of a "default limit" has no reason to expect that every principal outside their source's coverage is silently blocked with no log line pointing at why.

**Suggested fix:** either make `NewRateLimiterWithSource`/`NewRateLimiterWithSourceAndConfig`/`NewRateLimiterWithOptions` require (or default to) a non-zero static fallback whenever a `limitSource` is configured without explicit static limits, or have `Allow` log a warning the first time it falls through to a zero-value static limit while a `limitSource` is configured — mirroring the existing `clampNonNegativeLimits` warning for the equivalent misconfiguration on the static-limits path. Fix the currently-recommended `NewRateLimiterWithOptions` path, not just the deprecated `NewRateLimiterWithSource` constructors — both share the same defect.

---

## B3. `httpcache.CachingFetcher.Get` returns the `CacheResultStale` sentinel under two contradictory error conventions

**Status:** Resolved by picking the convention `TestCachingFetcher_ReturnStale` already pinned: `CacheResultStale` is now always returned with a nil error on every path (`Get`'s doc comment states this explicitly). The two contradicting branches (blocking-refresh and woken-waiter) were changed to return `nil` instead of the underlying fetch error. A new `LastError()` accessor exposes the swallowed error for callers that want to know why data is stale (mirroring `GetCachedData`/`GetCacheInfo`). Both previously-untested branches now have dedicated tests: `TestCachingFetcher_BlockingRefreshFailure_StaleDataHasNilError` and `TestCachingFetcher_ConcurrentAccess_WaiterSeesStaleWithNilError` in `httpcache/fetcher_test.go`.

**Files:** `httpcache/fetcher.go:129-153` (woken-waiter path), `httpcache/fetcher.go:160-179` (blocking-refresh path); pinned by `httpcache/fetcher_test.go:100-104` (`TestCachingFetcher_ReturnStale`)

`TestCachingFetcher_ReturnStale` pins the contract for `CacheResultStale` explicitly, including a comment stating the rule:

```go
data2, result2, err2 := fetcher.Get(ctx)
assert.Equal(t, CacheResultStale, result2)
require.NotNil(t, data2)
assert.Equal(t, 1, data2.Count) // Stale data
assert.NoError(t, err2)         // No error returned with stale data
```

That's the `ReturnStale: true` immediate-return path (`fetcher.go:110-122`), which never even attempts a synchronous fetch, so of course there's no error to report. But the *other* two code paths that also return `CacheResultStale` don't follow this rule — they return valid stale data **and** a non-nil error at the same time:

- `fetcher.go:166-173` — blocking path (`ReturnStale: false`), synchronous refetch fails, prior data exists: `return result, CacheResultStale, err`.
- `fetcher.go:135-144` — a caller that waited on another goroutine's in-flight refresh, which failed: `return data, result, err` with `result = CacheResultStale`.

Neither of these branches has its own test — `fetcher_test.go` never constructs a fetcher with `ReturnStale: false` that has *both* prior successful data and a subsequent failed refresh — so nothing currently exercises the contradiction directly.

This repo's own bundled `authz/prefixlist` providers (`aws.go:59`, `github.go:50`, `google.go:58`, `cloudflare.go:37`, `fastly.go:35`) all happen to hardcode `ReturnStale: true`, so they don't hit the blocking-path branch and are not exposed to this in practice today. But `NewHTTPJSONProvider`/`NewHTTPTextProvider` (`authz/prefixlist/http_provider.go:26`, `:68`) are exported and take a caller-supplied `httpcache.CacheConfig` — and `ReturnStale: false` is that struct's zero value, so any downstream caller who constructs one without explicitly opting into `ReturnStale: true` (a very easy thing to omit; it's not called out as required anywhere) lands on exactly the contradicted path. That caller's `Prefixes`/`Contains` already assume the `ReturnStale: true` convention (nil error whenever there's usable data) unconditionally, and get it wrong for their own `ReturnStale: false` fetcher:

```go
// authz/prefixlist/http_provider.go:38-42
func (p *HTTPJSONProvider[T]) Prefixes(ctx context.Context) ([]netip.Prefix, error) {
	data, _, err := p.fetcher.Get(ctx)
	if err != nil {
		return nil, err            // discards valid stale `data` whenever err != nil
	}
	return p.transform(data)
}
```

```go
// authz/prefixlist/http_provider.go:47-51
func (p *HTTPJSONProvider[T]) Contains(addr netip.Addr) bool {
	prefixes, err := p.Prefixes(context.Background())
	if err != nil {
		return false                // "IP is in the allowed list" silently becomes false
	}
	...
```

So a transient origin failure on a `ReturnStale: false`-configured `CachingFetcher` — with perfectly good, only-slightly-stale data cached — makes an `HTTPJSONProvider`/`HTTPTextProvider` built on it (by any caller who didn't think to override the zero-value `ReturnStale`) turn `Contains` into `false` instead of using the cached list, because `Get` handed back `(staleData, CacheResultStale, err)` and `Prefixes`/`Contains` followed the idiomatic (and, for the `ReturnStale: true` path this repo's own providers happen to use, *correct*) `if err != nil { return }` pattern. This is the same class of bug as the original review's Finding 3 (`DOHClient.Exchange` returning a valid answer alongside a non-nil error) — a working document elsewhere in this repo showing the project already knows this pattern is a trap, yet it still exists here, one layer down, under the same sentinel value with a test that documents the *opposite* contract for a sibling code path.

**Suggested fix:** pick one convention for `CacheResultStale` and apply it on every path that returns it: either always accompany stale data with `err == nil` (log the underlying fetch error instead, the way `f.lastError` already records it for introspection via `GetCacheInfo`), or update the doc comment on `Get` to state explicitly that `CacheResultStale` may carry a non-nil `err` and callers must check `data`/`result` before treating `err != nil` as "no usable data." Either way, add a test that exercises `ReturnStale: false` with prior successful data followed by a failed refresh, so the chosen convention is actually pinned.

---
