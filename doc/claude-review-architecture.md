# Architecture Review: `github.com/dioad/net`

_Reviewed: 2026-06-18 — branch `master`_

---

## Executive Summary

The library is well-structured and provides useful networking primitives (TLS, rate limiting, authz, DNS, SMTP record parsing, HTTP server/client) with reasonable test coverage. All 25 findings from the initial review were addressed. Two architectural findings were deferred to GitHub issues pending larger refactors.

---

## Findings

### 1. `authz/prefixlist`: HTTP infrastructure coupled inside the domain package

- **File(s):** `httpcache/fetcher.go`, `httpcache/text.go`
- **Dimension(s):** Architecture, Maintainability
- **Priority:** Low
- **Status:** Resolved
- **Description:** The `authz/prefixlist` package mixed domain logic (IP prefix matching, `Provider` interface) with HTTP infrastructure (`CachingFetcher`, cloud-provider adapters). `CachingFetcher` and `FetchTextLines` have been extracted to a new `httpcache` package (`github.com/dioad/net/httpcache`), allowing prefix matching to be tested without HTTP and enabling reuse of the generic fetcher elsewhere. The package was named `httpcache` rather than `cache/http` to avoid shadowing the `net/http` standard library package.

---

### 2. `http.Server` Config uses `Enable*` feature flags — composition-resistant

- **File(s):** `http/server.go`
- **Dimension(s):** Architecture, Maintainability
- **Priority:** Low
- **Status:** Open — GitHub issue #299
- **Description:** `ServerConfig` uses boolean `Enable*` flags (`EnableMetrics`, `EnableTelemetry`, etc.) to opt into features, making the server a borderline god-object. Treating each `Enable*` flag as a candidate for extraction into functional options would make features composable and independently testable.
- **Recommended fix:** Replace `Enable*` flags with functional options. See GitHub issue #299 for tracking.

---

## Priority Table

| # | Priority | Status | Finding | File(s) |
|---|----------|--------|---------|---------|
| 1 | Low | Resolved | `authz/prefixlist` HTTP infrastructure extracted to `httpcache` | `httpcache/fetcher.go`, `httpcache/text.go` |
| 2 | Low | Open (Issue #299) | `http.Server` `Enable*` flags — composition-resistant | `http/server.go` |
