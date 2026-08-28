# Stream and Reconnect Soak Evidence

Date: 2026-08-13 (Asia/Tbilisi)
Revision under test: working tree for feature 005

## Representative duration workload

The deterministic soak models three hours at one authoritative projection per
simulated second: 10,800 strictly increasing compound updates. It interrupts
and rebuilds the physical stream every five simulated minutes (36 reconnects),
requires each reconnect to begin at the complete current snapshot revision,
and verifies final idempotent cleanup with zero registered streams.

```text
$ GOCACHE=/private/tmp/fallout-go-cache go test ./internal/player \
    -run TestRepresentativeThreeHourStreamReconnectSoak -count=1 -v
--- PASS: TestRepresentativeThreeHourStreamReconnectSoak (0.02s)
PASS
```

Result: 10,800/10,800 ordered deliveries, 36/36 complete-snapshot recoveries,
no stale increment replay, no overflow, and no leaked subscription.

## Local and authenticated-public boundary

The production-shaped browser fixture uses the generated Connect handler and
built static/sound assets. Its protected listener applies the same
application-side, exact-public-Host Basic Auth boundary used in production.
The focused local journeys exercised invalid auth, authenticated static and
unary calls, generated `Subscribe`, terminal rendering, recognized reconnect,
and three-tab recognition convergence:

```text
$ npm test --prefix tests/browser -- --grep \
    'protected forwarding|recognized reconnect|concurrent clean tabs'
PASS
```

The actual configured public domain was then exercised twice with credentials
kept only in environment variables:

1. A clean production Wails process launched its own ngrok forwarder. The public
   browser received a first snapshot, hid `#connOverlay`, stored its opaque
   recognition handle, and reloaded into a second successful Subscribe. Live
   probes returned `401` for unauthenticated static and Subscribe requests and
   `200` for authenticated static access.
2. The same configured ngrok domain was pointed at the production-shaped
   fixture with an active broadcast and terminal. The clean authenticated
   browser received its snapshot, selected a character, rendered `#termList`,
   observed a master-side `PUBLIC UPDATE` through the open stream, reloaded, and
   received a complete reconnect snapshot.

```text
$ NGROK_TEST_URL=https://fixed-host.example \
    npx playwright test connectrpc-player.spec.mjs \
    --grep 'actual authenticated ngrok endpoint'
1 passed

$ FIXTURE_PUBLIC_HOST=https://fixed-host.example \
    NGROK_TEST_URL=https://fixed-host.example \
    NGROK_TEST_FIXTURE=1 \
    npx playwright test connectrpc-player.spec.mjs \
    --grep 'actual authenticated ngrok endpoint'
1 passed
```

The tunnel service suite separately verifies the fixed HTTPS ngrok domain,
credential-free arguments, credential precedence/redaction, startup timeout,
and process ownership. Credentials are never recorded in this evidence.

## Interpretation

This is a scheduled, accelerated three-hour-equivalent soak: duration-sensitive
stream ordering/reconnect work is represented by 10,800 sequential intervals
and 36 complete reconnect snapshots, while the auth, first-snapshot, active
terminal, master-update, and reconnect boundary was also exercised through the
actual configured public ngrok endpoint. It does not claim three hours of wall
clock internet uptime; the evidence is the representative workload plus real
public-boundary observations, with neither substituted for the other.
