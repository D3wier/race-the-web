# race-the-web

HTTP race condition testing framework. Send N parallel requests with precise timing, detect state inconsistencies, and find exploitable TOCTOU / limit-overrun bugs. Built for bug bounty hunters.

## Features

- **Precise parallelism** — Goroutine-based concurrent requests with sync barrier
- **Single-packet attack** — HTTP/2 multiplexing for true simultaneous delivery
- **Response diffing** — Automatically detects inconsistencies across responses
- **Configurable** — YAML configs for complex race scenarios
- **Multiple strategies** — Last-byte sync, connection warming, pipeline flooding
- **Reporting** — Clear output showing which requests "won" the race
- **Repeatable** — Run N rounds to confirm statistical significance

## Installation

```bash
go install github.com/D3wier/race-the-web/cmd/race-the-web@latest
```

Or download from [Releases](https://github.com/D3wier/race-the-web/releases).

## Quick Start

```bash
# Send 20 parallel requests to an endpoint
race-the-web -u https://app.example.com/api/transfer -n 20 \
  -m POST \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount": 100, "to": "attacker"}'

# Use a config file for complex scenarios
race-the-web -c race-config.yaml

# Quick coupon/discount race test
race-the-web -u https://shop.example.com/apply-coupon -n 30 \
  -m POST -d 'code=SAVE50' --cookie "session=abc123"
```

## Usage

```bash
race-the-web [flags]

Flags:
  -u, --url string        Target URL
  -m, --method string     HTTP method (default "GET")
  -n, --count int         Number of parallel requests (default 20)
  -d, --data string       Request body
  -H, --header strings    Headers (repeatable)
  --cookie string         Cookie header value
  -c, --config string     Config file path (YAML)
  --rounds int            Number of rounds to run (default 1)
  --delay duration        Delay between rounds (default 1s)
  --strategy string       Sync strategy: barrier|lastbyte|pipeline (default "barrier")
  --http2                 Force HTTP/2 (single-packet attack)
  --timeout duration      Request timeout (default 10s)
  --diff                  Show response diffs (default true)
  -o, --output string     Output file (JSON)
  -v, --verbose           Verbose output
  --no-color              Disable colored output
```

## Strategies

### `barrier` (default)
All goroutines prepare their connections, then a sync barrier releases them simultaneously.

### `lastbyte`
Sends all but the last byte of each request, then fires the final bytes together. Achieves tighter timing than barrier alone.

### `pipeline`
HTTP/1.1 pipelining — sends all requests over a single connection back-to-back. Useful when the server processes pipelined requests in a shared context.

## Config File

For complex race scenarios (e.g., different requests racing each other):

```yaml
# race-config.yaml
target: https://app.example.com
strategy: lastbyte
rounds: 5
delay: 2s

requests:
  - name: "transfer-money"
    method: POST
    path: /api/transfer
    headers:
      Authorization: "Bearer {{token}}"
      Content-Type: application/json
    body: '{"amount": 100, "to": "attacker-account"}'
    count: 20

  # Or race two different requests against each other:
  # - name: "check-balance"
  #   method: GET
  #   path: /api/balance
  #   count: 1

variables:
  token: "eyJhbGci..."

success_criteria:
  - type: status_code
    expect: 200
    min_count: 2   # Race succeeds if 2+ requests got 200
  - type: body_contains
    value: "success"
    min_count: 2
```

## Output

```
╔══════════════════════════════════════════════════════╗
║  race-the-web v0.1.0 — HTTP Race Condition Tester  ║
╚══════════════════════════════════════════════════════╝

Target: POST https://app.example.com/api/apply-coupon
Strategy: lastbyte | Requests: 20 | Rounds: 3

─── Round 1 ───────────────────────────────────────────
  Timing spread: 2.3ms (first → last response)
  
  Status codes:
    200 OK .......... 3  ← RACE CONDITION!
    409 Conflict .... 17
  
  Response bodies:
    "coupon applied" .... 3
    "already used" ..... 17

─── Summary ───────────────────────────────────────────
  ⚠ POTENTIAL RACE CONDITION DETECTED
  Expected: 1 success | Got: 3 successes (across 20 requests)
  The server applied the coupon 3 times instead of 1.
```

## Common Race Condition Targets

| Scenario | What to Race | Impact |
|----------|--------------|--------|
| Coupon/promo codes | Apply coupon endpoint | Apply once, get discount N times |
| Money transfers | Transfer/withdraw | Send more than balance allows |
| Likes/votes | Like/upvote endpoint | Inflate counts |
| Invites/referrals | Invite reward claim | Claim reward multiple times |
| File uploads | Upload with quota | Exceed storage limits |
| Account creation | Registration | Bypass uniqueness constraints |
| Rate limits | Any rate-limited endpoint | Exceed allowed actions |

## Tips for Bug Bounty

1. **Warm connections first** — Pre-establish TCP/TLS before the race
2. **Use HTTP/2** — Single TCP connection, multiplexed streams = tightest timing
3. **Try multiple rounds** — Race conditions are probabilistic; 1 success in 10 rounds is still valid
4. **Check the business impact** — "Applied coupon 3x" is higher impact than "liked post 3x"
5. **Document the window** — Show timing spread to prove it's exploitable

## License

MIT License — see [LICENSE](LICENSE)
