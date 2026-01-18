# Security Issue: HMAC Timestamp Validation Allows Future Timestamps

## Summary

The HMAC authentication timestamp validation logic in `http/auth/hmac/handler.go` currently allows timestamps that are in the future (within the `MaxTimestampDiff` window). This creates a vulnerability to pre-signed replay attacks where an attacker can capture a request with a future timestamp and replay it later within the allowed time window.

## Current Implementation

In `http/auth/hmac/handler.go` (lines 73-80):

```go
now := time.Now().Unix()
diff := now - timestamp
if diff < 0 {
    diff = -diff
}
if diff > int64(a.cfg.MaxTimestampDiff.Seconds()) {
    return r.Context(), errors.New("request timestamp expired or too far in the future")
}
```

The code uses absolute difference (`diff < 0` then `diff = -diff`), which means it treats future timestamps the same as past timestamps - both are allowed as long as they're within `MaxTimestampDiff` of the current time.

## Security Vulnerability

### Attack Vector: Pre-signed Request Replay

**Scenario:**
1. An attacker with temporary access to valid credentials creates an authenticated request with a timestamp set 5 minutes in the future (assuming default `MaxTimestampDiff` of 5 minutes)
2. The attacker captures this signed request
3. Even after losing access to the credentials, the attacker can replay this request for up to 10 minutes total:
   - The request is valid from creation time until the future timestamp (5 minutes)
   - Then valid from the future timestamp for another `MaxTimestampDiff` period (5 more minutes)

### Example Attack

**Setup:**
- `MaxTimestampDiff` = 5 minutes (default)
- Current server time: 2026-01-18 12:00:00 UTC
- Attacker creates request at: 2026-01-18 12:00:00 UTC
- Request timestamp: 2026-01-18 12:05:00 UTC (5 minutes in the future)

**Attack Timeline:**

| Time | Status | Reason |
|------|--------|--------|
| 12:00:00 | ✅ Valid | Timestamp is 5 min future, within MaxTimestampDiff |
| 12:01:00 | ✅ Valid | Timestamp is 4 min future, within MaxTimestampDiff |
| 12:05:00 | ✅ Valid | Timestamp matches server time exactly |
| 12:09:59 | ✅ Valid | Timestamp is 4:59 old, within MaxTimestampDiff |
| 12:10:00 | ❌ Invalid | Timestamp is 5 min old, exceeds MaxTimestampDiff |

**Result:** The attacker has a 10-minute window to replay the request, instead of the intended 5-minute window.

### Real-World Impact

1. **Extended Replay Window:** Doubles the effective replay window (10 minutes instead of 5 minutes with default settings)
2. **Credential Theft Mitigation Failure:** If credentials are compromised and then revoked/rotated, pre-signed requests with future timestamps remain valid longer than expected
3. **Audit Trail Confusion:** Requests with future timestamps may confuse log analysis and incident response
4. **Rate Limiting Bypass:** An attacker could pre-generate multiple requests with staggered future timestamps to bypass rate limits

## Recommended Solution

Reject timestamps that are too far in the future. The validation should be asymmetric:

```go
now := time.Now().Unix()
diff := now - timestamp

// Reject timestamps too far in the past
if diff > int64(a.cfg.MaxTimestampDiff.Seconds()) {
    return r.Context(), errors.New("request timestamp expired")
}

// Reject timestamps too far in the future (much stricter window)
// Allow small clock skew (e.g., 30 seconds) but not the full MaxTimestampDiff
if diff < -30 { // 30 seconds clock skew allowance
    return r.Context(), errors.New("request timestamp too far in the future")
}
```

### Alternative: Configurable Future Tolerance

Add a separate configuration parameter for future timestamp tolerance:

```go
type CommonConfig struct {
    // ... existing fields ...
    MaxTimestampDiff time.Duration `mapstructure:"max-timestamp-diff"`
    // Maximum allowed time difference for future timestamps (default: 30s)
    // This should be smaller than MaxTimestampDiff to prevent pre-signed attacks
    MaxFutureTimestampDiff time.Duration `mapstructure:"max-future-timestamp-diff"`
}
```

Then in validation:

```go
now := time.Now().Unix()
diff := now - timestamp

if diff > int64(a.cfg.MaxTimestampDiff.Seconds()) {
    return r.Context(), errors.New("request timestamp expired")
}

maxFuture := int64(30) // default 30 seconds
if a.cfg.MaxFutureTimestampDiff > 0 {
    maxFuture = int64(a.cfg.MaxFutureTimestampDiff.Seconds())
}

if diff < -maxFuture {
    return r.Context(), errors.New("request timestamp too far in the future")
}
```

## Considerations

1. **Clock Skew:** Some allowance for future timestamps is necessary due to clock skew between client and server. A reasonable default is 30-60 seconds.
2. **Backward Compatibility:** This change would reject previously valid requests with far-future timestamps. Consider:
   - Adding the feature behind a configuration flag initially
   - Logging warnings before enforcing rejection
   - Documenting the security improvement in release notes
3. **Testing:** Ensure integration tests cover:
   - Requests with slightly future timestamps (within clock skew) - should succeed
   - Requests with far-future timestamps - should fail
   - Requests with past timestamps within MaxTimestampDiff - should succeed
   - Requests with past timestamps beyond MaxTimestampDiff - should fail

## References

- File: `http/auth/hmac/handler.go`, lines 73-80
- Original PR: #189
- Review Comment: https://github.com/dioad/net/pull/189#discussion_r2702819309

## Labels

- `security`
- `enhancement`
- `http/auth/hmac`
