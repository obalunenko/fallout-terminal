# Implementation Plan: Player Sessions, Character Assignment, and Shared Terminal Control

> **SUPERSEDED LEGACY PLAYER TRANSPORT — HISTORICAL, NON-AUTHORITATIVE.**
> Any WebSocket or handwritten JSON player-transport description in this retained
> completed feature document has been replaced by the generated ConnectRPC contract in
> [`specs/005-connectrpc-protobuf-migration/contracts/public-player.md`](../005-connectrpc-protobuf-migration/contracts/public-player.md).

**Branch**: `004-player-sessions-control` | **Date**: 2026-08-12 | **Spec**: `specs/004-player-sessions-control/spec.md`

**Bugfix**: 2026-08-12 — BUG-001 Updated from bugfix patch

**Bugfix**: 2026-08-12 — BUG-002 Updated from bugfix patch

**Bugfix**: 2026-08-12 — BUG-003 Updated from bugfix patch

**Bugfix**: 2026-08-12 — BUG-004 Updated from bugfix patch

**Bugfix**: 2026-08-12 — BUG-005 Updated from bugfix patch

**Bugfix**: 2026-08-12 — BUG-006 Updated from bugfix patch

**Bugfix**: 2026-08-12 — BUG-007 Updated from bugfix patch

**Bugfix**: 2026-08-20 — BUG-008 Updated from bugfix patch

**Bugfix**: 2026-08-21 — BUG-009 Updated from bugfix patch

**Bugfix**: 2026-08-21 — BUG-010 Updated from bugfix patch

## Summary

Add process-local logical browser sessions, a game-master-managed character roster, broadcast-scoped exclusive claims, and one explicitly assigned controller while preserving the existing server-authoritative navigation and hacking rules. A single ordered coordinator will serialize browser presence, roster and assignment changes, controller changes, player actions, broadcast lifecycle, active-terminal switches, and puzzle-preservation decisions; it will emit detached revisioned projections to the Wails master and browser clients before another transition can overtake them. The browser will use a profile-scoped opaque recognition token, personalized player context, and correlated authoritative action results so refreshes and multiple tabs converge without optimistic shared mutation. ~~Durable version-1 session JSON remains unchanged~~ BUG-001 adds only an optional relative `playerConfig` reference to version-1 session JSON and stores reusable roster IDs and names in a separate versioned player-config file; all live coordination state remains excluded. `ForceHackSuccess` remains an exact private Wails-only operation. Under BUG-008, semantic terminal selection, paging, and preview join the ordered process-local live projection: only the active controller changes them, observers mirror them, and reassignment transfers presentation authority without resetting the shared display.

## Project Structure

```text
specs/004-player-sessions-control/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
└── contracts/
    ├── desktop-coordination.md
    └── player-websocket.md

app.go                                      # Validated Wails coordination commands and runtime status
app_test.go                                 # Bridge validation, event replay, and private-operation coverage
main.go                                     # Coordinator, player server, and master-event composition

internal/
├── domain/
│   ├── model.go                            # Runtime broadcast/session/roster projections beside durable models
│   └── model_test.go                       # Detached projections and unchanged version-1 JSON boundary
├── live/
│   ├── service.go                          # Terminal/nav/hack mechanics and exact private puzzle checkpoints
│   └── service_test.go                     # Preserve/discard/restore and unchanged gameplay rules
├── control/
│   ├── service.go                          # Single ordered process-local coordination aggregate
│   └── service_test.go                     # Claims, roles, lifetimes, authorization, ordering, concurrency
├── playerconfig/
│   ├── service.go                          # Native select/create, strict load, association, and atomic roster saves
│   └── service_test.go                     # Schema, portability, cancellation, failure, and restart coverage
├── player/
│   ├── client.go                           # Connection-to-logical-session dispatch context
│   ├── client_test.go                      # Queue and sender-context behavior
│   ├── http.go                             # Player assets, sound manifests, and static sound delivery
│   ├── http_test.go                        # Sound-manifest, asset-delivery, and HTTP security regressions
│   ├── protocol.go                         # Strict handshake, selection, action, context, and result envelopes
│   ├── protocol_test.go                    # Exact-field decoding and secret-free projection contracts
│   ├── server.go                           # Presence aggregation and selective/session-wide fanout
│   └── server_test.go                      # Multi-tab, reconnect, convergence, and selective delivery
├── platform/
│   └── assets_test.go                      # Player/master authority and UI-source contracts
└── testutil/testdata/protocol/             # Golden JSON for new player messages and projections

frontend/src/
├── desktop-api.js                          # Narrow coordination command/event facade
├── index.html                              # Player-config workflow, roster/session panel, and switch-decision dialog
├── master.css                              # Existing-aesthetic coordination and dialog styling
└── master.js                               # Authoritative master snapshot rendering and awaited commands

client/
├── index.html                              # Selection, waiting, role, and pending surfaces
├── client.css                              # Terminal-styled observer/read-only and selection states
├── client.js                               # Browser identity, personalized context, gating, and outcomes
├── sound.js                                # Web Audio loading and local one-shot playback
└── sounds/                                 # Retained hack-bad and hack-good player assets

tests/browser/
├── playwright.config.mjs                   # Run all player browser specifications
├── hacking-camouflage.spec.mjs             # Existing hacking interaction regression
└── player-sessions-control.spec.mjs         # Identity, selection, observer, pending, and switching journeys
```

**Structure Decision**: Add `internal/control` as the one outer transaction owner and keep `internal/live` focused on transport-independent terminal, navigation, and hacking mechanics. This avoids overloading the existing durable `internal/session` vocabulary, gives Wails and WebSocket callers the same ordering boundary, and retains the established `internal/player` and browser-only `client/` boundaries without a new runtime dependency. BUG-001 adds `internal/playerconfig` as the filesystem-backed authored-roster boundary; `internal/control` depends only on its narrow persistence interface so filesystem access does not leak into WebSocket or browser code.

## Constitution Check

The pre-research gate passes. The design stays within the modular-monolith boundaries, strengthens server authority, and deliberately keeps every new identity, claim, controller, connection, broadcast, and suspended-puzzle value outside version-1 session JSON.

| Principle | Assessment |
|---|---|
| I. Preserve Runtime Boundaries | PASS: `app.go` remains the validated private desktop facade; `internal/control` and `internal/live` own transport-independent coordination and gameplay; `internal/player` owns HTTP/WebSocket concerns; `client/` remains browser-only; and all affected surfaces are listed above. |
| II. Keep Shared State Server-Authoritative | PASS: every claim, control change, terminal action, and switch is validated and committed by one coordinator. Clients wait for revisioned server projections and correlated `ACTION_RESULT` messages and never apply canonical optimistic state. Secret hacking data stays private. |
| III. Protect Desktop and Public-Access Boundaries | PASS: the Wails facade exposes only the coordination methods in the desktop contract, validates identifiers and display names, and retains `ForceHackSuccess` only on the trusted master surface. No credential, filesystem, process, or desktop capability enters the player protocol. |
| IV. Preserve Session Data Compatibility | PASS: ~~roster entries,~~ logical sessions, browser recognition, presence, assignments, controller state, broadcast epochs, revisions, pending switches, and puzzle checkpoints are process-local. BUG-001 persists only authored roster IDs/names in a separate player config and adds an optional relative `playerConfig` reference to `domain.Session`; old version-1 files remain valid without migration. |
| V. Match Established Code Conventions | PASS: Go types and small integration interfaces follow existing packages; browser JavaScript, CSS, JSON, and uppercase message identifiers follow repository conventions; existing strict decoding and deterministic fakes are extended. No dependency is added. |

The post-design re-check uses the same assessments. `data-model.md` keeps runtime and durable entities separate, and both contracts preserve transport, privilege, and persistence boundaries; no Complexity Tracking table is required.

## Implementation Strategy

1. Introduce transport-independent runtime types for logical sessions, roster entries, broadcast epochs, exclusive assignments, controller identity, terminal runtime slots, ordered revisions, master snapshots, personalized player context, and action results. ~~Keep the existing durable `Session` and `Terminal` JSON models unchanged~~ Keep `Terminal` unchanged and extend `Session` only with the optional relative `playerConfig` reference required by BUG-001; add explicit serialization regressions proving runtime coordination remains excluded.
2. Build `internal/control.Service` around one mutex and one monotonic revision. Route connection attach/detach, roster changes, assignment changes, control reassignment, broadcast lifecycle, terminal switching, and all player actions through this service. During each accepted or rejected command, construct detached effects and enqueue a non-reentrant publication callback before releasing the transaction so mutation order and publication order cannot diverge.
3. Retain `internal/live` as the terminal mechanics boundary, but allow the coordinator to own multiple private terminal runtime slots. Preserve moves the exact active private state into a suspended slot; discard removes it; cancel changes nothing. Reactivation restores the private puzzle exactly while applying the latest authored terminal content through the existing navigation revalidation path. Deleting an active or suspended terminal must use the same explicit discard decision rather than bypassing the switch guard.
4. ~~Separate process lifetime, roster lifetime, broadcast lifetime, and active-terminal lifetime. Process restart clears everything runtime-only; broadcast end clears claims, controller, active terminal, pending switch, and suspended puzzles but retains logical sessions, fallback names, and roster.~~ Under BUG-001, separate process lifetime, authored-roster lifetime, broadcast lifetime, and active-terminal lifetime. Process restart clears everything runtime-only; reopening the session reloads authored roster IDs/names from its active player config; broadcast end clears claims, controller, active terminal, pending switch, and suspended puzzles but retains logical sessions, fallback names, and the loaded roster; a new broadcast issues a new opaque `broadcastId`; and a running broadcast may intentionally have no active terminal.
5. Create logical sessions from a server-issued browser-profile recognition token rather than from raw socket order. Under a cross-tab initialization lock, the browser sends the stored token when one exists; otherwise it completes one token-issuing handshake and saves the returned opaque value in `localStorage` before another tab connects. The returned process-local logical `sessionId` is read-only. The server maps one valid token to one logical session for its process lifetime, counts connection membership, changes presence only on first attach/last detach, and replaces an unknown token after restart while creating a fresh session and fallback name.
6. Extend strict WebSocket decoding with `SESSION_HELLO` and `CHARACTER_SELECT`. Add `requestId`, `broadcastId`, and `terminalId` to every shared player action so stale or duplicate requests can be rejected before gameplay mutation. Keep the existing `NAV_ACTION`, `HACK_GUESS`, and `HACK_PATTERN` identifiers and their gameplay-specific fields.
7. Add a complete personalized `PLAYER_STATE` projection for identity, current assignment/role, broadcast, roster availability, and active-terminal identity. Continue using `TERMINAL_LIVE`, `TERMINAL_UPDATE`, `TERMINAL_CLEAR`, `NAV_STATE`, and `HACK_STATE` for canonical terminal content, but add the committed revision to their envelopes. Send role and assignment changes to every tab of the affected logical session without exposing connection details to players.
8. Add `ACTION_RESULT` for every selection or shared terminal request. The result correlates by `requestId`, identifies acceptance or a stable rejection reason, and carries the committed revision. The initiating tab remains pending until it has both the result and any required authoritative projection at that revision; a rejection with no canonical mutation releases pending immediately. Duplicate request IDs for one logical session return the original result and never repeat a mutation.
9. Authorize every navigation and hacking request inside the coordinator transaction: the connection must resolve to the current logical session, that session must hold a current-broadcast character assignment, it must be the controller, it must be connected, and `terminalId` must still be active. Only then invoke the unchanged `internal/live` navigation or hacking operation. Observer, unassigned, disconnected, stale, invalid, and inactive-terminal requests return rejection effects with no canonical gameplay, log, attempt, random, or outcome mutation.
10. Add validated Wails methods and one detached `coordination-state` event/status projection for roster CRUD, fallback-name changes, assignment/release/transfer, controller reassignment, broadcast start/end, terminal activation, and switch resolution. Update the master frontend to render that authoritative snapshot and await command results before changing visible runtime state. Preserve the existing `ForceHackSuccess` name, eligibility, and publication path. Under BUG-004, the visible end-broadcast control and its confirmation must be exercised through this complete facade/coordinator/publication path rather than inferred from isolated lifecycle layers.
11. Add an immersive character-selection view, assigned waiting view, active/observer badge, visibly read-only observer state, and pending state to the player frontend. ~~Keep hover, focus, paging, sound, and other local feedback available where harmless, but gate every shared send path in the UI and rely on the same server authorization for crafted requests.~~ Under BUG-008, gate observer pointer, focus, keyboard, paging, and preview handlers before they can change local terminal presentation; apply semantic selection/page/preview only from the current authoritative projection, while per-document audio eligibility, playback, connection overlays, and other non-terminal technical effects remain local. Under BUG-009, keep `pendingPresentationAction` as internal correlation/latest-wins state but exclude it from the blocking `.shared-input-pending` appearance so only observers and genuinely unavailable shared gameplay controls look locked. Retain server authorization for crafted requests. Active-terminal changes reuse the existing loading presentation and never require identity or character selection again.
12. Extend deterministic Go, asset-contract, and Playwright coverage. Include the specification's 100-trial claim/controller races, multi-tab last-close presence, reconnect/restart lifetimes, stale broadcast and terminal actions, action-versus-reassignment ordering, exact puzzle preservation, broadcast resets, all-tab convergence, authoritative pending completion, and proof that player assets and protocol expose no `ForceHackSuccess` path. BUG-008 adds observer-local-inertness, controller presentation convergence, reconnect snapshots, and presentation-action-versus-reassignment races to these gates. BUG-009 adds delayed-`SetPresentation` computed-style and class assertions proving zero false controller lock/flicker while retaining authoritative-only selection, observer read-only styling, and actual gameplay-pending blocking.
13. Add a version-1 `PlayerConfig` model containing only config name and ordered stable roster ID/name pairs. Add an optional relative `playerConfig` reference to the existing session model; resolve it against the active session directory and never encode an absolute path when establishing a new association. Under BUG-002, every clone between storage, App, and coordinator boundaries preserves a non-nil zero-length roster so the required empty JSON array cannot become an invalid missing/null value.
14. After session create/open, load the referenced player config automatically or present select-existing/create-new actions. Cancellation keeps terminal authoring available but leaves roster mutation and broadcast start disabled. Missing or invalid references surface a recoverable error and never partially replace the roster. The create-new regression path uses the real player-config service, session association, and coordinator installation together before it is considered successful.
15. Make player-config selection/replacement legal only with no active broadcast. Persist the relative session association before activating it, and atomically persist every candidate add/rename/unclaimed-delete before committing the matching coordinator revision. Failed session or player-config writes retain the prior association, roster, claims, controller, and terminal/puzzle state.
16. Add restart/reopen and compatibility coverage proving stable roster restoration, unchanged legacy session loading, safe missing/invalid/cancel paths, reuse of one player config from multiple sessions, and complete exclusion of browser recognition and runtime coordination values from both durable files.
17. Under BUG-005, add exact trusted `ResetFailedHack` handling through `internal/live`, the coordinator, Wails facade, master UI, and existing ordered player publication. Accept it only for the current failed active hacking puzzle; atomically replace that runtime with a fresh generation from the latest authored terminal settings; preserve the broadcast, active terminal, sessions, claims, controller, roster, other runtime slots, and durable state; reject stale or ineligible calls without mutation; and expose no player reset capability.
18. Under BUG-006, retain bad-guess and good/unlock audio as idempotent client-local effects of applying a newer authoritative hacking projection. Compare the previously applied puzzle state with each applicable newer accepted revisioned terminal projection—`TERMINAL_LIVE` on the composed live path and `HACK_STATE` where that focused projection is emitted—through one shared transition function; for each audio-enabled player document, invoke bad once for an attempt-decreasing wrong selection including final lockout, invoke good once for a new solved transition, and invoke neither from the click or `ACTION_RESULT` alone, rejected actions, initial/reconnect snapshots, stale/duplicate projections, or unchanged re-renders. Audio eligibility requires that document's own user activation and applicable loaded asset. Keep ineligible or failed playback non-fatal to canonical rendering and pending completion.
19. Under BUG-007, preserve the complete established hacking-screen audio matrix. Use `single` for a newly previewed individual filler target, `multiple` for a newly previewed password or unused pattern, `enter` for selection, and the BUG-006 `hack-bad`/`hack-good` mapping for authoritative outcomes, while retaining ambient and character-scroll as working controls. Trace every failing family through event dispatch, manifest readiness, asset fetch, native decode, eligible context state, source/gain/destination routing, and audible packaged output. Synthetic audio adapters remain useful diagnostics but cannot be the terminal proof of native playback.
20. Under BUG-008, add one semantic controller-presentation variant for menu selection, information-page position, and hacking preview to the active terminal runtime. Extend the current generated protobuf/ConnectRPC boundary with a typed presentation mutation carrying recognition, request, broadcast, terminal, context, and variant-specific target preconditions; include the complete presentation in `LiveTerminal` snapshots and applicable compound updates. Validate and order it in `internal/control.Service`, reject observer/stale/invalid requests without revision or local display mutation, revalidate it after navigation/content/puzzle changes, and apply it in `frontend/client/client.js` without optimistic selection. Do not restore legacy WebSocket messages or publish raw pointer coordinates, DOM focus, audio eligibility, or playback state.
21. Under BUG-009, separate presentation request correlation from blocking input presentation at the player-client boundary only. Continue tracking and superseding `pendingPresentationAction`, applying semantic highlight/page/preview solely from authoritative revisions, and using the existing generated request unchanged; do not toggle the blocking shared-input class, locked cursor, opacity reduction, or desaturation for presentation-only latency. Preserve the existing blocked appearance for observer and genuinely pending gameplay/navigation/command paths.
22. Under BUG-010, make high-frequency presentation dispatch bounded at the player-client boundary without changing the generated contract or coordinator. Keep one `SetPresentation` request in flight and one replaceable latest desired semantic presentation; after authoritative completion or rejection, dispatch only that latest target if it remains eligible for the current role, broadcast, terminal, and context. Clear or revalidate queued intent on reassignment, disconnect, terminal/context change, navigation revalidation, and teardown. Superseded unsent targets produce no request, revision, render, reveal restart, or preview cue, while accepted projections remain ordered, authoritative, and shared with observers.

## BUG-001 Complexity Notes

- **Two-file consistency**: a player config may be created successfully before its reference can be saved into the session. The file is retained as a recoverable standalone config, while the application keeps the previous active association and roster until both sides of the association succeed.
- **Portability**: session references are normalized relative paths. Moving the session without its referenced config produces a recoverable missing-config state rather than blocking terminal authoring or silently creating an empty roster.
- **Shared player configs**: several sessions may reference one file. Writes remain atomic at the player-config boundary, and another session observes changes on its next load; concurrent multi-process editing and live external-file watching remain outside this bugfix.
- **Runtime separation**: loading durable roster IDs/names reconstructs only authored entries. Availability starts unclaimed, and no browser token, logical session, presence, assignment, controller, broadcast, terminal runtime, or puzzle is restored.
- **Empty-array representation — BUG-002**: Go distinguishes a non-nil empty slice from nil even though both have length zero. Player-config cloning must preserve that distinction: an application-created empty roster is valid and encodes as `[]`, while a missing or `null` roster remains invalid under strict decoding.

## BUG-003 Integration Verification Note

- **Composed active-controller hacking path**: Isolated browser fixtures, coordinator calls, and socket seams do not prove that one production browser session receives a mutually consistent `active`/`controlling` player state and active-terminal mirror, emits `HACK_GUESS` and `HACK_PATTERN` from rendered targets, reaches the authorized runtime mutation, receives the corresponding canonical projection and `ACTION_RESULT`, and releases pending input. BUG-003 requires this complete path to be exercised with the real player WebSocket server and coordinator while retaining observer read-only behavior and the private `ForceHackSuccess` boundary.

## BUG-004 Integration Verification Note

- **Composed game-master end-broadcast path**: Coordinator, App, desktop-facade, server-fanout, and player-client tests do not prove that the production game-master button is enabled, accepts confirmation, invokes `EndBroadcast` exactly once, applies the returned authoritative master snapshot, and drives every connected player to no-broadcast state with its terminal cleared. BUG-004 requires the first failing boundary to be reproduced before correction and the complete master-to-player path to pass afterward; `window.confirm` is an investigation point, not a presumed cause.

## BUG-005 Complexity and Integration Notes

- **Runtime replacement boundary**: Retry deletes only the failed active terminal runtime and creates its replacement inside the same coordinator transaction. It is not terminal reactivation, unfinished-puzzle switch discard, `ForceHackSuccess`, or broadcast restart.
- **Eligibility and ordering**: `ResetFailedHack` is eligible only for the same current failed active puzzle. Duplicate master invocations, a terminal/broadcast change, and player actions racing the retry serialize under the coordinator; discarded-generation targets can never be interpreted against the replacement puzzle.
- **Retained state**: The transition changes the active runtime generation, attempts, board, log, outcome, and gated navigation only. Broadcast identity, terminal selection, sessions, presence, assignments, controller, roster, other runtime slots, authored configuration, and durable unlocked state remain unchanged.
- **Composed private path**: Unit tests for puzzle generation and generic player fanout do not prove that the blocked master control invokes one trusted command and that every assigned browser converges on the same fresh state. The production-shaped master-to-player journey and explicit absence of every player reset path are required.

## BUG-006 Integration Verification Note

- **Authoritative transition boundary**: Cursor sounds prove only that some local audio works. Outcome cues must be derived from the difference between the previously applied hacking state and one newer accepted projection, not from pointer selection or an `ACTION_RESULT` without the corresponding canonical state.
- **Idempotence and recovery**: Duplicate or stale revisions, unchanged rendering, initial terminal delivery, and reconnect restoration must establish or retain the comparison baseline without replaying historical bad, good, or lockout cues.
- **Observable playback boundary**: Asset presence and an `HTMLMediaElement.play` stub do not exercise the Web Audio buffer path used by the one-shot hacking cues. Browser coverage must observe the actual sound adapter or `AudioContext` decode/source-start boundary, explicitly enable audio in every tested player document, and use separate fresh puzzle cases for ordinary wrong, unlock, and final lockout. The packaged native player journey must prove the retained `hack-bad` and `hack-good` assets are audible after each tested document's own gesture.

## BUG-007 Integration Verification Note

- **Complete family boundary**: The working CRT loop and character-scroll cue do not prove `single`, `multiple`, `enter`, `hack-bad`, or `hack-good`. Exercise each family through its real hacking-screen event and keep the two working families as controls in the same output-capable document.
- **Native decode and output boundary**: Existing browser fixtures replace `AudioContext.decodeAudioData()` and source output with unconditional synthetic success. Those fixtures may localize event and adapter invocation, but BUG-007 completion requires the production sound adapter, native browser decoder, real destination routing, and directly monitored audible output from the packaged player.
- **Authority boundary**: Preview and entry cues remain client-local for harmless interaction. Bad/good cues remain derived only from newly applied authoritative hacking transitions, and audio failure must never change observer authorization, canonical state, pending completion, or replay guards.

## BUG-008 Complexity and Integration Notes

- **Semantic-state boundary**: Synchronize stable terminal meaning—selected menu node, page ordinal, or current hacking target/pattern—not pointer coordinates, CSS hover, browser focus, viewport geometry, reveal progress, or other DOM implementation detail. Each client renders the semantic state within its responsive layout.
- **Current transport boundary**: The public player boundary is generated protobuf over ConnectRPC. Additive request/projection fields, strict limits, adapters, generated Go/ECMAScript outputs, descriptor drift checks, and the active public contract must move together; the superseded handwritten WebSocket vocabulary remains historical only.
- **Authority and ordering**: Presentation movement is lightweight but authoritative. It uses current session/broadcast/terminal/context checks and the same coordinator order as reassignment and gameplay; observer, former-controller, stale-context, duplicate, and invalid-target requests produce no revision or display mutation.
- **Reassignment and recovery**: The presentation lives with the active terminal runtime rather than a browser session. Reassignment preserves it while atomically changing who may update it; complete snapshots give late, reopened, and reconnected observer tabs the current state before they render a default highlight.
- **Audio boundary**: Audio eligibility and playback remain per document. Applicable menu-focus and hacking-preview cues are derived from a newer authoritative presentation transition, never from observer-local pointer/focus input; failures remain non-fatal and non-canonical.
- **Backpressure and revalidation**: Rapid pointer movement must remain bounded by the existing transport and stream rules without allowing an older selection to overtake a newer accepted revision. Navigation, content publication, pagination changes, hacking generation/outcome changes, and removed targets revalidate or reset the semantic selection deterministically.

## BUG-009 Complexity and Integration Notes

- **Pending-state split**: `pendingPresentationAction` remains necessary for request correlation, accepted-revision completion, stale-result suppression, and latest-wins movement. It is not evidence that controller input is unavailable and therefore must not drive the same CSS class as `pendingSharedAction`, command execution, or terminal navigation.
- **No optimistic fallback**: Removing the grey/locked flash must not move semantic highlighting back to local pointer/focus state. The current authoritative selection remains visible until the next applicable revision arrives.
- **Regression boundary**: Observer documents retain their read-only cursor and styling, and genuinely blocked gameplay/navigation/command operations retain their pending treatment. The correction is limited to presentation-only latency in an active/controlling document and requires no protobuf, coordinator, runtime, persistence, or private-operation change.

## BUG-010 Complexity and Integration Notes

- **Bounded client backpressure**: `pendingPresentationAction` represents the one dispatched mutation awaiting result/projection completion. A separate latest-desired slot may be replaced freely but never creates concurrent presentation RPCs; it drains at most once per completed request and skips a value already authoritative or in flight.
- **Authority and invalidation**: Queued intent is not canonical and cannot outlive the controller/broadcast/terminal/context that produced it. Reassignment, disconnect, navigation, puzzle regeneration/outcome, target removal, or terminal teardown clears or revalidates it before any follow-up dispatch.
- **Visual and audio exactness**: Only accepted authoritative projections may change the hacking highlight or invoke `single`/`multiple`; unsent intermediate hover samples remain invisible and silent. Observer convergence and BUG-009's normal active-controller styling remain unchanged.

## Verification Gates

- `gofmt -l .` reports no Go source paths.
- `go vet ./...` succeeds.
- `go test ./...` succeeds, including 100 concurrent same-character trials, 100 concurrent first-controller trials, detached projections, exact no-mutation rejection comparisons, process/broadcast/terminal lifetime coverage, and unchanged version-1 JSON.
- `go test -race ./...` succeeds for concurrent claims, first-controller assignment, connection membership, action/reassignment ordering, duplicate requests, and ordered publication.
- `npm --prefix frontend run build` succeeds with the generated-binding fallback intact.
- `npm --prefix tests/browser test` succeeds for selection, multi-tab identity, observer read-only behavior, accepted/rejected pending outcomes, loading transitions, terminal switches, and existing hacking camouflage interactions.
- `wails dev` passes the private game-master journeys for roster/session management, claim corrections, controller reassignment, disconnected-controller status, preserve/discard/cancel, broadcast end/restart, and the unchanged `ForceHackSuccess` control. If unavailable, report the reason rather than claiming success.
- A clean `wails build` remains the packaging gate for a self-contained macOS application; signing/notarization checks remain release-only when credentials are available.
- Player-config service tests cover create/open/cancel, relative association, malformed and unsupported data, duplicate IDs, atomic write failure, reuse from multiple session files, and BUG-002 preservation of a non-nil empty roster through result cloning.
- A real BUG-002 integration test covers `Create → session association → coordinator install → first roster add`, proving the newly created config is active before the first mutation and still rejects a missing or `null` roster.
- Restart/reopen browser and bridge journeys prove authored roster restoration without restored logical sessions, claims, controller, terminal runtime, or puzzles.
- A production-composed BUG-003 browser journey covers active-controller password, filler, and unused-pattern selection through the real WebSocket/coordinator path, exactly-once canonical mutation and convergence, authoritative pending completion, and zero observer shared actions.
- A production-shaped BUG-004 game-master journey activates and confirms the visible end-broadcast control, observes exactly one `EndBroadcast` call through the real facade/coordinator boundary, and proves authoritative no-broadcast context plus terminal clear reaches every connected player without changing retained process/authored/durable state.
- A production-shaped BUG-005 journey fails the current puzzle, activates the visible `ПОВТОРИТЬ ВЗЛОМ` control, observes exactly one `ResetFailedHack` call, and proves all assigned players receive one fresh generation with full attempts while broadcast identity, active terminal, assignments, controller, sessions, roster, other runtimes, and durable state remain unchanged.
- Concurrent retry/stale-action coverage proves exactly one replacement generation, no mutation from discarded-generation targets, and no player asset or protocol path to `ResetFailedHack`.
- A production-shaped BUG-006 active-and-observer journey observes the Web Audio boundary across separate fresh ordinary-wrong, newly-solved, and final-lockout puzzle cases; every tested player document receives its own enabling gesture and records exactly one bad, good, and bad cue invocation respectively, with zero outcome cues from click-only, rejected, stale, duplicate, reconnect, or re-render paths.
- A packaged native BUG-006 player journey confirms that `hack-bad` and `hack-good` folders load and decode after each tested document's own user gesture and that ineligible or failed playback remains non-fatal to authoritative rendering and pending completion.
- A BUG-007 production-browser matrix uses the native audio implementation after one explicit enabling gesture and exercises individual filler preview, password/pattern preview, actionable selection, ordinary wrong, unlock, and final lockout. It verifies `single`, `multiple`, `enter`, `hack-bad`, and `hack-good` independently while ambient and character-scroll remain working controls.
- A packaged BUG-007 player journey records successful manifest delivery, native decode, connected destination routing, and directly monitored audible output for every required hacking-screen family. A fake decoder, source-start counter, or playback observer alone is insufficient evidence.
- A production-composed BUG-008 controller/observer matrix proves observer pointer, focus, keyboard, paging, back, activation, and hacking-preview input causes zero local terminal-selection change and zero outbound mutation, while accepted controller presentation changes converge through generated ConnectRPC on every assigned client in revision order.
- BUG-008 reassignment/reconnect coverage runs at least 100 presentation-action-versus-reassignment interleavings and late/new-tab/reconnect cases, proving immediate old-controller lock, new-controller unlock from unchanged presentation, stale-context rejection, and first-snapshot restoration without a divergent default-highlight flash.
- A BUG-009 browser latency matrix delays generated `SetPresentation` responses across at least 100 controller pointer/keyboard movements and asserts stable active cursor, opacity, saturation, and absence of blocking/read-only classes while newer movements remain eligible and only authoritative revisions change semantic selection; paired observer and gameplay-pending cases retain their locked appearance and behavior.
- A BUG-010 browser backpressure matrix holds the first generated `SetPresentation` request, sweeps at least 100 distinct hacking targets, and proves one in-flight request plus at most one final-target follow-up, zero intermediate canonical revisions/highlight or reveal restarts/preview cues, final active-and-observer convergence, and stale queued-intent disposal across reassignment, context change, rejection, disconnect, and teardown.
- Review confirms ~~there are no new version-1 session fields~~ the only new version-1 session field is the optional relative `playerConfig` reference, there is no player path to `ForceHackSuccess`, no private connection data in player projections or either durable file, and no optimistic canonical browser transition.
