# Implementation Plan: Phase 1 Generation-Bound Hacking Patterns

> **SUPERSEDED LEGACY PLAYER TRANSPORT — HISTORICAL, NON-AUTHORITATIVE.**
> Any WebSocket or handwritten JSON player-transport description in this retained
> completed feature document has been replaced by the generated ConnectRPC contract in
> [`specs/005-connectrpc-protobuf-migration/contracts/public-player.md`](../005-connectrpc-protobuf-migration/contracts/public-player.md).

**Branch**: `feature/003-hacking-game-evolution` | **Date**: 2026-08-11 | **Spec**: `specs/003-hacking-game-evolution/spec.md` | **Handoff**: `specs/003-hacking-game-evolution/planning-handoff.md`

**Bugfix**: 2026-08-11 — BUG-001 Updated from bugfix patch; artifact consistency and verification coverage remediated after analysis.

**Bugfix**: 2026-08-28 — BUG-002 Updated from bugfix patch.

**Bugfix**: 2026-08-28 — BUG-003 Updated from bugfix patch.

## Summary

Correct the existing server-authoritative special-pattern implementation so every public identity is bound to one runtime puzzle generation and one rendered-row coordinate pair, every accepted activation follows the mandated weighted-selection and atomic publication order, and every rejected request leaves both canonical state and the outcome random source untouched. Board generation now camouflages valid patterns among candidate words, ordinary punctuation, non-empty pattern interiors, alphabetic-interrupted spans, and ~~inert~~ individually selectable delimiter decoys before the complete rendered board is analyzed. The unchanged production discovery algorithm remains the sole authority for the initial `3–6` count and the public projection; publication additionally requires the final board to satisfy the decoy, non-empty-interior, alphabetic-interruption, and exact occupied-row distribution gates. Ordinary word guesses and filler-symbol behavior remain unchanged; ~~delimiter decoys are inert~~ BUG-001 makes every delimiter individually selectable unless the exact cell is a current valid pattern's opening coordinate; and `ForceHackSuccess` stays exclusively behind the existing private desktop/Wails boundary. BUG-002 additionally makes an authoritative solved snapshot invalidate player-facing action and presentation results from its superseded hacking context so a late rejection cannot overwrite or outlive the shared success flow.

## Constitution Check

The pre-research gate passes. The Phase 1 corrections remain inside the existing modular-monolith boundaries and introduce no dependency, persistence, role, or presentation expansion.

| Principle | Assessment |
|---|---|
| I. Preserve Runtime Boundaries | PASS: discovery, generation-bound identity, outcome selection, and used history remain transport-independent in `internal/hack`; `internal/live` owns serialization and the atomic publication commit; `internal/player` owns strict WebSocket decoding and fanout; `client/` remains browser-only; and the existing Wails game-master path remains separate. |
| II. Keep Shared State Server-Authoritative | PASS: `HACK_PATTERN` remains an untrusted request. The live-service mutex validates generation and current coordinates, marks used state, selects and applies one outcome, creates a detached projection, and commits one ordered broadcast before releasing the transition. Browsers perform no optimistic mutation. |
| III. Protect Desktop and Public-Access Boundaries | PASS: strict unknown-field rejection is retained, the public pattern shape is reduced, and no player route, message, global, DOM control, shortcut, or query parameter gains access to `ForceHackSuccess`. |
| IV. Preserve Session Data Compatibility | PASS: generation IDs, patterns, used history, removed duds, attempts, logs, and outcomes remain process-local. Version-1 session JSON retains only the existing durable terminal `hackLevel`; no puzzle seed or unlocked state is added. |
| V. Match Established Code Conventions | PASS: the work extends existing Go aggregates, injectable seams, mutex-protected services, uppercase snake-case messages, camelCase JSON, and browser JavaScript conventions. No runtime dependency is added. One isolated browser-test development dependency is permitted only for executable player interaction coverage, must be exactly pinned, and must be locked reproducibly. |

The post-design re-check uses the same assessments: the data model and contract below introduce no constitutional violation, so no Complexity Tracking table is required.

## Project Structure

```text
specs/003-hacking-game-evolution/
├── spec.md                                  # Normative clarified requirements
├── planning-handoff.md                     # Mandatory no-loss planning guardrails
├── plan.md                                  # Corrective implementation plan
├── research.md                              # Superseding design decisions and rejected alternatives
├── data-model.md                            # Generation, identity, state, projection, and transition model
└── contracts/
    └── hacking-interface.md               # Strict `HACK_PATTERN` and public-state contract

internal/
├── domain/
│   ├── model.go                             # Generation-bound private identity and minimal public pattern model
│   └── model_test.go                        # Version-1 JSON remains free of runtime hacking state
├── hack/
│   ├── hack.go                              # Camouflaged generation, final-board gates, unchanged discovery, outcomes
│   └── hack_test.go                         # 1,000-board camouflage gate, ~~decoy inertness~~ delimiter selection, dynamic rediscovery
├── live/
│   ├── service.go                           # Generation issuance and nine-step mutex-protected activation/publication
│   └── service_test.go                      # Stale-generation, duplicate, ordering, projection-isolation coverage
├── player/
│   ├── protocol.go                          # Strict generation-bearing opaque `patternId` input
│   ├── protocol_test.go                     # Exact-field and public-envelope contract coverage
│   ├── server.go                            # Publication callback committed during the live transition
│   └── server_test.go                       # Multi-client convergence, rejection silence, reconnect, stale puzzle tests
├── platform/
│   └── assets_test.go                       # Valid-pattern, delimiter-decoy, styling, and player-authority contracts
└── testutil/testdata/protocol/
    └── hack-state.json                      # Minimal generation-bound public pattern fixture

client/client.js                              # Opening-anchor pattern mapping, opaque submission, individual delimiter input
client/client.css                             # Static styling parity for valid and decoy delimiters
tests/browser/
├── package.json                              # Isolated, exactly pinned browser-test development dependency
├── package-lock.json                         # Reproducible browser-test dependency lock
├── playwright.config.mjs                    # Player-board interaction test configuration
└── hacking-camouflage.spec.mjs              # Executable hover/focus/click and dispatch coverage
app_test.go                                   # Detached public pattern fixtures and private GM solve regression
frontend/src/index.html                       # Existing private GM solve control, verified unchanged
frontend/src/master.js                        # Existing Wails invocation and eligibility, verified unchanged
```

**Structure Decision**: Correct the established `internal/hack` → `internal/live` → `internal/player` → `client/` path in place. Keep generation and discovery in the hacking domain, serialization and publication commit in the live service, protocol ownership in the player server, and the trusted Wails override in its existing separate boundary; add no runtime package, route, persisted field, or role model. Add one isolated, exactly pinned and locked browser-test development dependency because the existing Go asset-contract tests cannot execute player hover, focus, click, computed-style, or WebSocket-dispatch behavior.

## Implementation Strategy

1. Replace coordinate-only string identities with a private comparable identity containing generation, flattened rendered-row ordinal, and row-local inclusive start/end offsets. Issue a new non-persisted generation ID for every fresh puzzle without consuming the pattern-outcome random source.
2. Make board construction an attempt inside a regeneration loop. Place candidate words, ordinary punctuation, valid-pattern construction aids, at least one non-empty non-alphabetic pattern interior, alphabetic-interrupted word spans, and standalone delimiter-decoy candidates through the same normal board-row space before validation. Do not reserve rows or contiguous regions by content type.
3. Run the existing production discovery function unchanged on the complete final rendered board. Derive the valid pattern set and every accidental pattern from that result, then classify standalone ~~inert~~ individually selectable decoys and alphabetic-interrupted spans. For candidate words, valid-pattern endpoints, and standalone delimiter decoys, calculate each category's inclusive minimum-to-maximum occupied-row interval. Publish only when discovery returns `3–6`; the standalone-decoy count is at least the valid-pattern count; at least one valid pattern has a non-empty non-alphabetic interior; at least one potential span is alphabetic-interrupted; each of the three categories occupies at least two rows; their intervals overlap pairwise; and ordinary punctuation or filler remains in at least two rows. Otherwise regenerate the whole board.
4. Change pattern activation so every accepted request is marked used, consumes exactly one `Intn(100)` outcome value, maps `0..79` to dud removal and `80..99` to restoration, then applies restoration as the fallback if a selected dud removal has no target. A dud-selection draw remains separate and occurs only when removal has an eligible target.
5. Reduce public pattern JSON to opaque `id`, `row`, inclusive `start`/`end`, and `used`. Recompute current spans from canonical board text after mutation and retain complete private used history so a rediscovered coordinate pair stays unavailable. Never project a standalone, mismatched, alphabetic-interrupted, later-compatible-but-unselected, or otherwise invalid delimiter.
6. Keep strict `HACK_PATTERN` decoding with one opaque `patternId`. The ID contains or resolves to the complete generation-bound identity; the browser treats it as opaque and uses the explicit row coordinates only for rendering. Candidate cells inside invalid spans continue through ordinary word guessing. ~~Delimiter cells outside all projected valid pattern ranges are inert in both browser dispatch and canonical filler-target handling.~~ **BUG-001 correction**: only the cell at a current pattern's `start` resolves to pattern handling; an unused start sends `HACK_PATTERN`, a used start retains existing unavailable behavior, and every other filler cell, including standalone delimiters and non-opening cells within a pattern span, retains individual `HACK_GUESS` dispatch and canonical filler-target handling.
7. Commit the detached `HACK_STATE` publication callback as step nine while the live-service mutex is still held. The callback may only enqueue the detached payload to the player fanout and game-master event paths and must not call back into live state; actual socket writes remain owned by `internal/player`.
8. Extend deterministic domain tests so the 1,000-board gate checks every final-board camouflage invariant, including accidental discoveries and projection exclusivity. Add focused fixtures for non-empty and empty pairs, unmatched and mismatched delimiters, first-closer decoys, ~~inert delimiter input~~ ordinary individual delimiter input, word-interrupted candidates, and dud-created rediscovery without modifying discovery semantics.
9. Extend bundled asset-contract tests so the player source retains valid-only pattern lookup, candidate-word dispatch, ~~inert decoy dispatch~~ opening-anchor-only pattern dispatch plus ordinary individual delimiter dispatch, and no persistent validity-dependent class or style. Include `client/client.css` in the governed surface and retain ordinary filler-click assertions as behavioral sentinels.
10. Add an isolated executable browser suite that loads a controlled player board and intercepts outbound WebSocket messages. Verify ~~valid-pattern hover/focus/click~~ whole-span hover/focus/click only from the valid opening symbol, candidate selection inside an alphabetic-interrupted span, ~~inert standalone decoys~~ individual standalone-delimiter selection, non-opening pattern-cell selection, equal pre-interaction computed styles, no optimistic mutation, and activation only after a server snapshot publishes a dud-created pattern. Pin the browser-test development dependency exactly and commit its lockfile; add no production dependency.
11. On an authoritative transition of the active hacking generation to solved, clear pending shared-action and presentation UI state plus transient rejection notices belonging to the superseded hacking context. Retain enough correlated request/context identity to ignore a matching late rejection without hiding unrelated current rejections. Test both orderings around the solved snapshot, the active controller and observers, and streamed presentation plus unary fallback paths.

### BUG-002 Edge-Case Note

The new edge case spans the existing authoritative snapshot and correlated result channels but adds no new authority, protocol, persistence, dependency, or long-lived state owner. Browser reconciliation uses existing request and hacking-context identity, so the Constitution Check still requires no Complexity Tracking table.

### BUG-003 Implementation-Drift Correction

Keep strategy 11 and the existing authority boundaries, but treat the ended actionable hacking generation as the reconciliation lifetime even when the controller presentation is absent, changes in the solved update, or was already advanced. Capture enough pre-transition request and generation identity to suppress only results from that ended context across both full live-terminal snapshots and hacking-only projections; do not make cleanup depend solely on the current or immediately previous presentation key. First reproduce and record the production ordering without sensitive session data, then extend the controlled browser coverage. Final verification must drive the actual private Overseer success control after the player's first incorrect password selection; the fixture force-success endpoint remains useful for deterministic ordering tests but cannot substitute for that native smoke.

This correction adds no authority, protocol, persistence, dependency, or cross-process state. Any additional client marker is transient reconciliation state scoped to the current broadcast, terminal, and puzzle generation, so no Complexity Tracking table is required.

## Verification Gates

- `gofmt -l .` reports no changed Go files.
- `go vet ./...` succeeds.
- `go test ./...` succeeds, including the controlled 100-value probability mapping; a 1,000-board final-render gate proving `3–6` discovered patterns, decoy parity, a non-empty valid interior, an alphabetic-interrupted span, mixed row distribution, accidental-pattern accounting, and valid-only projection on every board; stale-generation rejection; zero-RNG rejection paths; atomic duplicate publication; reconnect convergence; detached projection mutation; and version-1 session compatibility.
- `go test -race ./...` succeeds for concurrent duplicate pattern activation and publication ordering.
- `npm --prefix frontend run build` succeeds without adding or changing a player-accessible solve path.
- `npm start` completes the Electron behavioral smoke journey: the player hacking board loads, ordinary guesses and BUG-001 delimiter interactions work, no player-accessible force-success path exists, and the private game-master `ForceHackSuccess` flow remains operational without a stale rejection after an unsuccessful guess. BUG-003 requires selecting the first incorrect password and immediately invoking the real private Overseer control while checking both the hacking surface and resulting terminal menu. If this check is unavailable, record the reason and do not claim it passed.
- Go asset-contract tests confirm opening-coordinate-only pattern lookup, opaque generation-bound submission, ~~inert-decoy dispatch guards~~ ordinary individual delimiter dispatch, unchanged candidate and filler paths, static CSS parity constraints, and absence of `ForceHackSuccess` from all player assets.
- `npm --prefix tests/browser test` succeeds against controlled board fixtures, executing opening-symbol whole-pattern hover/focus/click, non-opening and standalone-symbol individual interaction, outbound message capture, computed-style parity, no optimistic mutation, dud-created pattern activation after the next server snapshot, and BUG-002 forced-success reconciliation with correlated results delivered on both sides of the solved snapshot through streamed and unary presentation paths. BUG-003 additionally covers full live-terminal and hacking-only solved publications with absent, changed, and already-advanced presentation context.
