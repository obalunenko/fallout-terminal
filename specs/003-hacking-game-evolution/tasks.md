# Tasks: Phase 1 Generation-Bound Hacking Patterns

> **SUPERSEDED LEGACY PLAYER TRANSPORT — HISTORICAL, NON-AUTHORITATIVE.**
> Any WebSocket or handwritten JSON player-transport description in this retained
> completed feature document has been replaced by the generated ConnectRPC contract in
> [`specs/005-connectrpc-protobuf-migration/contracts/public-player.md`](../005-connectrpc-protobuf-migration/contracts/public-player.md).

**Input**: `spec.md`, `planning-handoff.md`, `plan.md`, `research.md`, `data-model.md`, and `contracts/hacking-interface.md`

**Bugfix**: 2026-08-11 — BUG-001 Updated from bugfix patch; artifact consistency and verification coverage remediated after analysis.

**Bugfix**: 2026-08-28 — BUG-002 Updated from bugfix patch.

**Bugfix**: 2026-08-28 — BUG-003 Updated from bugfix patch.

Task IDs continue at `T027` so the prior completed task journal cannot mark this corrective task set complete accidentally.

## Phase 1: Setup

The corrective implementation uses the existing Go module, WebSocket server, browser assets, deterministic randomness seam, and test infrastructure. No package, dependency, generated binding, directory, or tooling setup is required.

## Phase 2: Foundational Identity and Projection Model

This phase establishes the generation-bound identity and minimal public shape required by every story.

**Wave 1:**

- [x] **T027** Replace coordinate-only pattern models with `GenerationID`, comparable generation/row/start/end identity, private discovery metadata, and minimal detached public `id`/`row`/`start`/`end`/`used` fields · `internal/domain/model.go`

**Checkpoint**: Canonical and public types compile with no public `column` or `pair`, and all later story work can target the complete generation-bound identity.

## Phase 3: User Story 1 — Solve Without Player Cheats (P1)

**Goal**: Preserve the removal of every player-accessible cheat and retain ordinary password/filler behavior while the model changes.

**Independent Test**: Exercise normal candidate and filler guesses, the removed administrator command, and bundled player surfaces; verify unchanged attempts and no force-success or bulk-dud path.

### Tests

**Wave 1 — independent (different files):**

- [x] **T028** [P] [US1] Update and strengthen ordinary candidate, filler-click, removed `SUCCESS`, and terminal-state regression cases against the generation-bound model · `internal/hack/hack_test.go`
- [x] **T029** [P] [US1] Strengthen bundled-player checks for removed administrator inputs, query/keyboard/global cheat paths, and unchanged ordinary cell dispatch · `internal/platform/assets_test.go`

**Checkpoint**: User Story 1 is independently verified; the corrective work has not restored any player cheat or changed ordinary attempt rules.

## Phase 4: User Story 2 — Discover and Use Special Patterns (P1)

**Goal**: Publish only final boards with `3–6` discovered patterns and apply the exact weighted outcome mapping with one required outcome draw per accepted activation.

**Independent Test**: Generate 1,000 final boards, cover all four pairs, drive all 100 equiprobable outcome values, exercise no-dud fallback, and interact through row-based inclusive browser coordinates.

### Tests

**Wave 1 — independent (different files):**

- [x] **T030** [P] [US2] Add failing final-board regeneration, `3–6` discovery, 100-value `80/20` mapping, accepted no-dud RNG consumption, rejected zero-RNG, secret-preservation, restoration, and minimal-projection tests · `internal/hack/hack_test.go`
- [x] **T031** [P] [US2] ⚠️ Reopened — Add failing bundled-player assertions for ~~row-local inclusive hover/click lookup~~ opening-coordinate-only pattern lookup, inclusive whole-span highlighting from that anchor, ordinary individual lookup at every non-opening filler offset, opaque `patternId` echo, used-state handling, and absence of public `column`/`pair` dependencies · `internal/platform/assets_test.go` (reopened — BUG-001)

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2:**

- [x] **T032** [US2] Implement generation-aware row discovery, final-board regeneration through the production scanner, mandatory weighted outcome draw before fallback, dud selection, and minimal detached current-pattern projection · `internal/hack/hack.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [x] **T033** [US2] ⚠️ Reopened — Replace column-based pattern interaction with ~~rendered-row and row-local inclusive offsets~~ rendered-row coordinates where only `start` resolves to the pattern, while `start` through `end` remains the inclusive highlight span and every non-opening filler offset retains ordinary individual interaction; continue to send the opaque server-issued `patternId` without optimistic mutation · `client/client.js` (reopened — BUG-001)

**Checkpoint**: User Story 2 is independently functional and testable across generation, weighted effects, projection, and browser interaction.

## Phase 5: User Story 3 — Discover Stacked and Dynamic Patterns (P1)

**Goal**: Keep shared-closer openings independent and preserve permanent coordinate-pair used history across dynamic discovery changes.

**Independent Test**: Mutate controlled boards so one opening changes its first compatible closer and a previously used pair disappears and reappears; verify new identity availability and old identity unavailability.

### Tests

**Wave 1:**

- [x] **T034** [US3] Add failing stacked shared-closer, first-compatible-close, same-row, alphabetic-interior, changed-closer, disappeared/reappeared used-pair, and post-dud dynamic projection cases · `internal/hack/hack_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2:**

- [x] **T035** [US3] Complete row-local scanning and private complete-identity history so changed coordinate pairs are new and rediscovered used pairs remain unavailable · `internal/hack/hack.go`

**Checkpoint**: User Story 3 is independently functional and testable for stacked, changed, newly created, and rediscovered patterns.

## Phase 6: User Story 4 — Share One Atomic Puzzle State (P1)

**Goal**: Reject stale and duplicate generation-bound requests without RNG or mutation, publish accepted transitions in the mandated mutex order, and converge connected/reconnecting clients on detached state.

**Independent Test**: Race duplicate requests, delay an ID across two puzzle generations with coincident coordinates, mutate returned snapshots, and reconnect a client; verify one acceptance, one outcome draw, one ordered publication, and current state convergence.

### Tests

**Wave 1 — independent (different files):**

- [x] **T036** [P] [US4] Add failing deterministic generation-ID, stale-generation, duplicate zero-RNG, exact one-publication, callback-order, terminal-state, fresh-set, and detached-projection live-service tests · `internal/live/service_test.go`
- [x] **T037** [P] [US4] Update strict `HACK_PATTERN` decoder and envelope tests for opaque generation-bearing IDs, missing/unknown/invalid fields, and minimal public pattern JSON · `internal/player/protocol_test.go`
- [x] **T038** [P] [US4] Add failing multi-client duplicate, stale-generation, no-broadcast rejection, accepted ordered fanout, reconnect-current-state, and process-local reset cases · `internal/player/server_test.go`
- [x] **T039** [P] [US4] Update public-pattern fixtures and deepen returned-projection mutation tests across runtime status and game-master events · `app_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2:**

- [x] **T040** [US4] Issue collision-resistant runtime generation IDs independently of gameplay RNG and execute validation, used marking, outcome, mutation, rediscovery, detached projection, and one publication callback under the live mutex · `internal/live/service.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent (different files):**

- [x] **T041** [P] [US4] Retain strict `HACK_PATTERN` decoding while documenting and carrying the opaque generation-bearing `patternId` with no coordinate-only assumptions · `internal/player/protocol.go`
- [x] **T042** [P] [US4] Replace the golden public pattern object with `id`, `row`, inclusive `start`/`end`, and `used` only · `internal/testutil/testdata/protocol/hack-state.json`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4:**

- [x] **T043** [US4] Supply the non-reentrant publication callback, enqueue exactly one accepted `HACK_STATE` plus detached game-master notification under the live transition, and suppress every rejected publication · `internal/player/server.go`

**Checkpoint**: User Story 4 is independently functional and testable under concurrency, stale delivery, projection mutation, and reconnect.

## Phase 7: User Story 5 — Let the Game Master Resolve the Puzzle Privately (P1)

**Goal**: Preserve `ForceHackSuccess` only through the private desktop/Wails boundary after the public model and publication changes.

**Independent Test**: Invoke the trusted control for an eligible puzzle, verify unchanged attempts and normal shared success, reject ineligible states, and inspect every player surface for equivalent authority.

### Tests

**Wave 1 — independent (different files):**

- [x] **T044** [P] [US5] Update generation-bound public fixtures and verify eligible/ineligible `ForceHackSuccess`, unchanged attempts, detached events, and existing shared success publication · `app_test.go`
- [x] **T045** [P] [US5] Verify the private master control and Wails call remain intact while WebSocket messages, browser globals, DOM controls, keyboard shortcuts, query parameters, and player assets expose no equivalent · `internal/platform/assets_test.go`

**Checkpoint**: User Story 5 is independently verified; only the trusted game-master boundary can force success.

## Phase 8: Polish and Success-Criteria Validation

**Wave 1:**

- [x] **T046** Add a version-1 encode/decode regression proving generation IDs, patterns, used history, removed duds, attempts, outcomes, unlocked state, and puzzle seeds never enter persisted session JSON · `internal/domain/model_test.go`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2:**

- [x] **T047** Run `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and `npm --prefix frontend run build`; fix only feature-caused failures and verify the pre-camouflage definitions of SC-001 through SC-013. The amended SC-003 and SC-004 remain unverified until Phase 10 · `.`

## Dependencies & Execution Order

- Setup adds no work. Foundational T027 blocks every user-story phase.
- User Story 1 protects existing behavior before User Story 2 changes discovery/outcomes; User Story 3 then extends the same hacking engine; User Story 4 integrates generation, concurrency, projection, protocol, and publication; User Story 5 verifies the separate trusted boundary; Polish closes persistence and full-suite gates.
- Phase 3 Wave 1 is independent after T027. Phase 4 Wave 1 blocks T032, which blocks T033. Phase 5 T034 blocks T035. Phase 6 Wave 1 blocks T040, which blocks Wave 3, which blocks T043. Phase 7 Wave 1 follows the public integration. Phase 8 T046 blocks the final T047 validation.
- Tasks tagged `[P]` touch different files within their wave and may be completed in any order; every explicit join must complete before the next wave begins.

## Phase 9: Convergence

- [x] **T048** ⚠️ Reopened — Make browser special-pattern hover and click resolution row-local and ~~inclusive across every offset from `start` through `end`~~ pattern-aware only at `start`; retain inclusive whole-span highlighting from that opening anchor, make all non-opening filler offsets individually selectable, and correct the asset-contract regression · `client/client.js`, `internal/platform/assets_test.go` per plan: client hover/click mapping and T031/T033 (partial) (reopened — BUG-001)

## Phase 10: Board Camouflage and Delimiter Decoys

**Goal**: Camouflage the unchanged special-pattern discovery rules among words, ordinary filler, non-empty pattern interiors, word-interrupted spans, and ~~inert~~ individually selectable delimiter decoys, then publish only complete boards that pass every final-board gate.

**Independent Test**: Generate 1,000 publishable boards and prove each has `3–6` production-discovered patterns, decoy parity, a non-empty valid interior, an alphabetic-interrupted span, at least two occupied rows per candidate-word, valid-endpoint, and standalone-decoy category, pairwise-overlapping occupied-row intervals, ordinary filler in at least two rows, accidental-pattern accounting, and a valid-only public projection; then execute valid-pattern, candidate-word, delimiter-decoy, computed-style, and dud-created rediscovery interactions in a real browser context.

**Verification status**: SC-003, SC-004, SC-014, and SC-016–SC-019 are verified by completed corrective tasks T049–T056 and the reopened final verification gate T054. T057 recorded the constitution-mandated Electron smoke as unavailable because the post-cutover repository has no root `package.json`; no smoke-test pass is claimed. T047 predates the camouflage and BUG-001 interaction amendments and remains only a historical baseline.

### Tests

**Wave 1 — independent (different files):**

- [x] **T049** [P] [US6] ⚠️ Reopened — Extend generator and domain regressions with the 1,000-board camouflage gate; exact occupied-row counts and pairwise interval overlap; adjacent-empty and non-empty patterns; unmatched, mismatched, and first-closer decoys; accidental-pattern accounting; valid-only projection; ~~inert direct delimiter targets~~ ordinary individual delimiter-target logging and attempt behavior; ordinary word selection inside invalid spans; and post-dud rediscovery while preserving the existing scanner fixtures unchanged · `internal/hack/hack_test.go` (reopened — BUG-001)
- [x] **T050** [P] [US6] ⚠️ Reopened — Extend bundled-player asset and style contracts for opening-only valid-pattern lookup, candidate-word dispatch, ~~inert delimiter dispatch~~ ordinary individual delimiter and non-opening pattern-cell dispatch, unchanged filler behavior, no persistent validity-dependent class, and static styling parity across the governed player stylesheet · `internal/platform/assets_test.go`, `client/client.css` (reopened — BUG-001)
- [x] **T051** [P] [US6] ⚠️ Reopened — ~~Add~~ Update the isolated, exactly pinned and locked executable browser-test harness plus controlled board fixtures covering ~~valid-pattern hover/focus/click~~ whole-pattern hover/focus/click only from `start`, ordinary individual selection for non-opening pattern cells, ordinary candidate selection inside an alphabetic-interrupted span, ~~standalone delimiter no-op behavior~~ standalone delimiter selection, outbound message capture, equal pre-interaction computed styles, no optimistic mutation, and activation only after a server snapshot publishes a dud-created pattern · `tests/browser/package.json`, `tests/browser/package-lock.json`, `tests/browser/playwright.config.mjs`, `tests/browser/hacking-camouflage.spec.mjs` (reopened — BUG-001)

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2:**

- [x] **T052** [US6] Replace bracket-free isolated-pair construction with normal-row camouflage placement, including a non-empty valid interior, alphabetic-interrupted candidate span, and standalone delimiter-decoy candidates; run unchanged production discovery on the complete board; compute occupied-row counts and pairwise interval overlap; and regenerate unless every pattern-count, decoy, interior, interruption, distribution, and projection gate passes · `internal/hack/hack.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3:**

- [x] **T053** [US6] ⚠️ Reopened — ~~Make rendered delimiter cells outside all current valid pattern ranges inert~~ Preserve ordinary canonical filler-target handling and browser `HACK_GUESS` dispatch for standalone delimiters and every non-opening filler cell within a pattern span, while only a current unused pattern's `start` sends `HACK_PATTERN` and a used `start` retains existing unavailable behavior; retain ordinary candidate and filler behavior and change the governed stylesheet only if static-parity tests expose a difference · `internal/hack/hack.go`, `client/client.js`, `client/client.css` (reopened — BUG-001)

## Updated Dependencies & Execution Order

- ~~Phase 10 starts from the completed T027–T048 baseline. T049, T050, and T051 may run in parallel; all three block T052. T052 blocks T053, and T053 blocks the final T054 verification gate.~~ BUG-001 reopens the affected interaction work after completed T052: `T031 → T050`, T049, and T051 form the corrective test wave; all block T033, which blocks T048, then T053. T049 and T051 may run in parallel with the T031 → T050 chain because they touch different files.
- The existing discovery, identity, activation, probability, concurrency, projection, reconnect, private-control, and persistence rules remain prerequisites and are not reopened except where a new regression explicitly proves they remain unchanged.
- Phase 11 closes the remaining coverage before final verification: T055 blocks T056 because both update `internal/hack/hack_test.go`; T057 may run independently after T053; T055, T056, and T057 all block the reopened T054 verification gate.
- BUG-002 starts from the completed T054–T057 baseline without reopening those historically valid tasks: T058 adds the failing cross-channel regression, T059 implements reconciliation against that regression, and both block T060 final verification.
- BUG-003 preserves T058–T060 as historical BUG-002 work and adds T062–T064 for the uncovered production-equivalent ordering and context shapes. T062 blocks T063; T063 blocks T064, whose native smoke must use the actual private Overseer control.

## Phase 11: Convergence

- [x] **T055** Add table-driven domain and executable browser fixtures for unmatched, mismatched, word-interrupted, and later-compatible-but-unselected delimiter targets; verify individual hover/preview behavior, ordinary `HACK_GUESS` logging and attempt consumption, and zero `HACK_PATTERN` requests per SC-018 and US6/AC6 · `internal/hack/hack_test.go`, `tests/browser/hacking-camouflage.spec.mjs`
- [x] **T056** [US6] Extend the publishable-board generator tests across every configured hacking difficulty and prove that each uses the same inclusive `3–6` initial production-discovered-pattern limit, with no difficulty-specific count branch · `internal/hack/hack_test.go`
- [x] **T057** Run and record the constitution-mandated `npm start` Electron behavioral smoke and rollback check, including player hacking interactions and the private game-master success flow; if unavailable, record the reason and do not claim it passed · `.`

**⟶ Wait for T055–T057 to finish, then:**

### Final Verification

- [x] **T054** ⚠️ Reopened — Run formatting, static analysis, full tests, race tests, browser syntax checks, `npm --prefix tests/browser test`, the frontend build, and explicit ~~SC-003/SC-004/SC-014–SC-017~~ SC-003/SC-004/SC-014/SC-016–SC-019 verification; confirm the 1,000-board gate, opening-only pattern activation, individual delimiter/non-opening-symbol selection, executable browser interactions, unchanged discovery fixtures, difficulty-invariant `3–6` generation, and an accurately recorded Electron smoke result without claiming unavailable checks passed · `.` (reopened — BUG-001)

**Checkpoint**: User Story 6 is independently functional and the board reveals pattern validity only through ~~normal valid-pattern interaction~~ whole-span interaction from an unused valid pattern's opening coordinate, never through construction grouping, public decoy metadata, or static styling. ~~Delimiter decoys have no side effects.~~ Under BUG-001 they retain ordinary individual filler effects without gaining pattern identity.

## Phase 12: BUG-002 Authoritative Hack-Outcome Reconciliation

**Goal**: Preserve the authoritative shared success flow when a trusted `ForceHackSuccess` transition races a player guess or hacking-presentation result, and prevent notices from the superseded hacking context from surviving on either the hacking surface or terminal menu.

**Independent Test**: Submit an unsuccessful guess, delay a correlated shared-action or presentation result, force the active puzzle to success, and deliver the result on each side of the solved snapshot; verify the active controller and observers retain the solved outcome and see no stale or raw rejection through the hacking-to-menu transition.

### Tests

- [x] **T058** [US5] Add deterministic executable browser regressions for unsuccessful-guess then `ForceHackSuccess` ordering, including a delayed shared-action result, streamed hacking-presentation result, unary fallback result, delivery before and after the solved snapshot, active-controller and observer rendering, and zero stale `invalid-action` notice on the hacking surface or subsequent terminal menu · `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/terminal-navigation.spec.mjs`, `tests/browser/fixture-server/main.go`

**⟶ Wait for T058 to finish, then:**

### Implementation

- [x] **T059** [US5] Reconcile authoritative solved snapshots in the player client by invalidating pending shared-action and presentation UI state plus transient notices from the superseded hacking context, consuming matching late results without suppressing unrelated current rejections, and preserving the existing success delay, cue, and terminal-menu transition · `frontend/client/client.js`

**⟶ Wait for T059 to finish, then:**

### Final Verification

- [x] **T060** Verify US5/AC5, FR-089–FR-090, and SC-020 with the focused browser journeys; if Go files changed, run `go fix ./...` and review its edits; then run `task fmt:check`, `task vet`, `task test`, `task test:race`, `task frontend:build`, and `task browser:test`, and record the native behavioral smoke accurately without claiming unavailable checks passed · `.`

**Checkpoint**: A successful trusted solve remains the one player-facing outcome across correlated result orderings, and rejection notices remain visible only for unrelated current actions.

## Phase 13: Convergence

**Depends on:** all prior phases.

**Wave 1 — Restore mutex-protected pattern publication:**

- [x] **T061** [US4] Invoke the detached enqueue-only `ApplyHackPattern` publication callback before releasing the canonical live-service mutex, and add deterministic concurrency coverage proving a blocked callback prevents a later accepted pattern transition and publication from overtaking it while rejected requests still publish nothing and callbacks remain non-reentrant, per FR-047, plan strategy 7, research Decision 6, and T040/T043 (contradicts) · `internal/live/service.go`, `internal/live/service_test.go`

**Checkpoint**: Every accepted legacy pattern activation commits its detached publication in canonical mutation order before the live-service mutex is released, without performing transport writes or re-entering live state.

## Phase 14: BUG-003 Production Forced-Success Reconciliation

**Depends on:** T058 and T061.

**Goal**: Reproduce the remaining production ordering, close the presentation-context blind spot in BUG-002 coverage, and prove that the real private Overseer success flow cannot leak `invalid-action` across the hacking-to-menu transition.

**Independent Test**: Select the first incorrect password, invoke the real private Overseer success control immediately, and verify the active player and observers show only the shared solved outcome on the hacking surface and subsequent menu. Repeat the production-equivalent ordering deterministically for full live-terminal and hacking-only solved publications with absent, changed, and already-advanced presentation context.

### Tests and Production-Ordering Evidence

- [x] **T062** [US5] Capture the real private-control event ordering and non-sensitive request, broadcast, terminal, puzzle-generation, presentation-context, solved-publication, and action-result identities; add a failing production-equivalent regression for the first incorrect password followed by immediate trusted success, covering full live-terminal and hacking-only publications with absent, changed, and already-advanced presentation context and asserting zero raw or stale notice before and after menu return · `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/terminal-navigation.spec.mjs`, `tests/browser/fixture-server/main.go`, `frontend/client/client.js`

**⟶ Wait for T062 to finish, then:**

### Implementation

- [x] **T063** [US5] Bind solved-snapshot reconciliation to the ended broadcast/terminal/puzzle generation rather than only the current presentation kind, handling full live-terminal and hacking-only projections with absent, changed, or already-advanced presentation context while preserving unrelated current rejections and the existing success transition · `frontend/client/client.js`

**⟶ Wait for T063 to finish, then:**

### Final Verification

- [x] **T064** Verify US5/AC5, FR-089–FR-090, and SC-020 with the focused browser journeys and repository gates from T060; perform and record a native replay in which the player selects the first incorrect password and the game master immediately invokes the actual private Overseer success control, asserting zero rejection notice on the hacking surface and terminal menu, and do not treat fixture-only force success as equivalent evidence · `.`

**Checkpoint**: The fixture regression matches the captured production ordering, and the native Overseer replay is the final acceptance evidence rather than an unperformed follow-up.
