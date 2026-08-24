# Tasks: Player Sessions, Character Assignment, and Shared Terminal Control

> **SUPERSEDED LEGACY PLAYER TRANSPORT — HISTORICAL, NON-AUTHORITATIVE.**
> Any WebSocket or handwritten JSON player-transport description in this retained
> completed feature document has been replaced by the generated ConnectRPC contract in
> [`specs/005-connectrpc-protobuf-migration/contracts/public-player.md`](../005-connectrpc-protobuf-migration/contracts/public-player.md).

**Input**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, and `contracts/` in `specs/004-player-sessions-control/`

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

**Tests**: Required by the specification and constitution. Each user-story phase writes its focused tests before implementation; the final phase owns the complete Success Criteria suite run.

**Task format**: `T### [P?] [US#] Description · exact/file/path`

## Phase 1: Setup

Configure the existing browser-test project to discover the new player-session specification without adding a dependency.

**Wave 1 — setup:**

- [x] **T001** Broaden Playwright discovery from the single hacking file to all `*.spec.mjs` browser specifications while retaining one worker and the existing static player server · `tests/browser/playwright.config.mjs`

---

## Phase 2: Foundational — Shared Runtime and Protocol Infrastructure

This phase establishes the transport-independent types, strict wire shapes, sender-aware connection callback, coordinator transaction boundary, deterministic seams, and serialization guardrails that block every user story.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T002** [P] Prove runtime coordination projections detach deeply and that browser tokens, logical sessions, roster, claims, controller, revisions, switches, and puzzles never enter version-1 session JSON · `internal/domain/model_test.go`
- [x] **T003** [P] Specify deterministic opaque IDs, monotonic accepted-transition revisions, unchanged revisions on rejection, detached effects, and enqueue-before-unlock ordering for the new coordinator · `internal/control/service_test.go`
- [x] **T004** [P] Specify strict exact-field decoding and JSON envelopes for handshake, selection, request preconditions, personalized player state, action results, and revisioned terminal messages · `internal/player/protocol_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — independent (different files):**

- [x] **T005** [P] Add transport-independent runtime IDs, roles, phases, roster/session/broadcast models, terminal slots, action results, master snapshots, and personalized player projections beside the unchanged durable models · `internal/domain/model.go`
- [x] **T006** [P] Extend constants, typed messages, strict decoders, and secret-free revisioned envelope constructors to the frozen player WebSocket contract · `internal/player/protocol.go`
- [x] **T007** [P] Carry the originating `PlayerConnection` into decoded-message callbacks while retaining one reader, one bounded writer queue, and slow-client isolation · `internal/player/client.go`
- [x] **T008** [P] Add deterministic opaque-ID, clock-free revision, and ordered-effect fakes used by coordinator and server concurrency tests · `internal/testutil/fakes.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — coordinator foundation:**

- [x] **T009** Create `internal/control.Service` with one mutex, process runtime root, ID sources, monotonic revision, detached snapshot builders, bounded request-result records, and non-reentrant effect enqueueing · `internal/control/service.go`

**Checkpoint**: Foundational types, strict contracts, deterministic seams, and the one-owner transaction skeleton compile; no story behavior has been enabled yet.

---

## Phase 3: User Story 1 — Join as a Character (Priority: P1) 🎯 MVP

**Goal**: A new browser profile establishes one logical session, sees the game-master-defined roster, claims one available character, and becomes the first controller only when no controller exists.

**Independent Test**: Start a broadcast with two roster entries, connect one new browser profile, select one character, and verify the claim, personalized identity, active role, and current-terminal-or-waiting result.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T010** [P] [US1] Cover roster creation, fresh broadcast selection, 100 concurrent same-character claims, 100 concurrent different first assignments, one claim per session, one session per character, and exactly one initial controller · `internal/control/service_test.go`
- [x] **T011** [P] [US1] Cover exact `SESSION_HELLO`, `SESSION_WELCOME`, `CHARACTER_SELECT`, `PLAYER_STATE`, and `ACTION_RESULT` shapes, roster privacy, stale broadcast rejection, and unknown-field rejection · `internal/player/protocol_test.go`
- [x] **T012** [P] [US1] Cover first handshake, token issuance, fallback-name uniqueness, selection success/conflict, personalized roster refresh, and assigned terminal/waiting delivery over real sockets · `internal/player/server_test.go`
- [x] **T013** [P] [US1] Cover validated `AddCharacter`, `StartBroadcast`, coordination-status replay, detached master events, and failure without partial state · `app_test.go`
- [x] **T014** [P] [US1] Add browser tests for handshake gating, terminal-styled available/claimed selection, one pending selection, conflict recovery, escaped names, and progression after authoritative acceptance · `tests/browser/player-sessions-control.spec.mjs`
- [x] **T015** [P] [US1] Assert selection markup/style contracts, hidden-state layout, text-safe rendering, no browser token in URLs/markup, and no player `ForceHackSuccess` path · `internal/platform/assets_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — independent (different files):**

- [x] **T016** [P] [US1] Implement session creation/fallback names, process-local roster add, broadcast start, exclusive player selection, conflict effects, and atomic first-controller establishment · `internal/control/service.go`
- [x] **T017** [P] [US1] Add semantic character-selection, player-identity, role, and assigned-waiting regions without exposing a privileged operation · `client/index.html`
- [x] **T018** [P] [US1] Add responsive terminal-styled selection, claimed/available, identity, and waiting presentation within the existing bounded desktop/tablet layout · `client/client.css`
- [x] **T019** [P] [US1] Add the minimal game-master roster and broadcast controls needed to define characters and start the MVP broadcast · `frontend/src/index.html`
- [x] **T020** [P] [US1] Style the roster/broadcast controls within the existing desktop master layout and terminal aesthetic · `frontend/src/master.css`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent integrations (different files):**

- [x] **T021** [P] [US1] Add coordinator interfaces, validated roster/start commands, `coordinationState` runtime status, and `coordination-state` event emission to the Wails facade · `app.go`
- [x] **T022** [P] [US1] Require handshake before state/actions, bind connections to sessions, dispatch selection, send personalized state, and fan out the accepted claim to affected tabs · `internal/player/server.go`
- [x] **T023** [P] [US1] Implement welcome-gated connection state, token storage, selection rendering/submission, authoritative player-state application, and character-primary identity · `client/client.js`
- [x] **T024** [P] [US1] Add the exact roster/start methods plus `onCoordinationState` status-replay subscription to the narrow desktop facade · `frontend/src/desktop-api.js`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — independent composition (different files):**

- [x] **T025** [P] [US1] Render the authoritative roster/broadcast snapshot, await add/start results, surface conflicts, and avoid optimistic master runtime state · `frontend/src/master.js`
- [x] **T026** [P] [US1] Construct the coordinator with the existing live service, connect player/master effect sinks, and preserve lifecycle shutdown ownership · `main.go`

**Checkpoint**: User Story 1 is independently functional and testable: a game master can define a roster/start a broadcast, and concurrent players can establish exclusive identities and claims with one initial controller.

---

## Phase 4: User Story 2 — Control One Shared Terminal (Priority: P1)

**Goal**: Only the connected assigned controller can mutate the canonical terminal and its semantic presentation; ~~observers mirror state with local-only terminal feedback~~ under BUG-008 observers mirror the controller-owned selection, page, and preview while their own terminal pointer/keyboard/focus input is inert, and every request ends via a correlated authoritative result.

**Independent Test**: Connect one controller and two observers, exercise every navigation/hacking/presentation action from each, and verify only controller requests mutate state, observer pointer/keyboard/focus input changes neither its local terminal view nor canonical state, all clients converge on the controller's presentation, and every pending request resolves.

**BUG-006 deferral**: T029, T030, T034, and T037 were reopened for traceability and were re-completed through T150 after T148 reproduced the outcome-audio failure and T149 corrected the confirmed boundary; the original Phase 4 wave placement is historical and does not override the Phase 19 DAG.

**BUG-007 deferral**: T029, T030, and T037 were reopened again for traceability and were re-completed through T154 after T152 reproduced the complete sound-family failure and T153 corrected the confirmed production boundary; T034 remained completed BUG-006 work and received focused regression review in T153–T154.

**BUG-008 deferral**: T029, T030, T032, and T034 are reopened because the prior observer contract intentionally retained local selection/highlight behavior. Their correction and re-completion are governed by the Phase 22 DAG; prior BUG-003/BUG-006/BUG-007 completion remains historical evidence for gameplay and audio behavior only.

**BUG-009 deferral**: T029, T030, and T032 are reopened because their BUG-008 browser/asset/CSS coverage accepted presentation-only pending as a grey, locked shared-input state. Their correction and re-completion are governed by the Phase 23 DAG; T034 remains complete but requires focused review so gameplay/navigation pending stays blocking while `pendingPresentationAction` becomes visually non-blocking.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T027** [P] [US2] Cover controller/unassigned/observer/unknown/stale-terminal authorization, exact no-mutation and zero-RNG rejection, duplicate request fingerprints, request/reassignment ordering, and unchanged gameplay rules · `internal/control/service_test.go`
- [x] **T028** [P] [US2] ⚠️ Reopened (reopened — BUG-003): Cover 4–7 client convergence, initiating-socket action results, shared revision order, crafted observer rejection, duplicate one-use requests, slow-client isolation, and production-shaped active-controller hacking dispatch · `internal/player/server_test.go`
- [x] **T029** [P] [US2] ⚠️ Reopened (reopened — BUG-003; reopened — BUG-006; reopened — BUG-007; reopened — BUG-008; reopened — BUG-009): Cover visibly read-only observers, ~~local hover/focus/paging/preview~~ zero observer-local terminal-selection changes plus authoritative controller selection/page/preview mirroring, zero outbound observer actions, active-controller password/filler/pattern selection, controller pending state, accepted/rejected result completion without optimistic mutation, presentation-only pending with zero grey/locked controller flicker under delayed `SetPresentation`, and the complete exact-once `single`/`multiple`/`enter` plus authoritative bad/good/lockout audio matrix · `tests/browser/player-sessions-control.spec.mjs`
- [x] **T030** [P] [US2] ⚠️ Reopened (reopened — BUG-003; reopened — BUG-006; reopened — BUG-007; reopened — BUG-008; reopened — BUG-009): Assert every pointer/keyboard/focus/paging/back/guess/pattern path is role/pending gated before local terminal-presentation mutation, active controller targets remain actionable and normally styled while only presentation correlation is pending, observer controls and genuinely pending gameplay controls remain visibly inert, `pendingPresentationAction` cannot activate blocking shared-input styling, `ForceHackSuccess` and `HACK_ADMIN` remain absent from player assets/protocol, and all established hacking-screen sound manifests/assets plus native-decode verification remain packaged · `internal/platform/assets_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — independent prerequisites (different files):**

- [x] **T031** [P] [US2] Refactor terminal/navigation/hacking operations behind a coordinator-callable runtime boundary while retaining all current password, likeness, attempt, pattern, log, and forced-success semantics · `internal/live/service.go`
- [x] **T032** [P] [US2] ⚠️ Reopened (reopened — BUG-008; reopened — BUG-009): Add observer/read-only and genuinely blocking shared-input-pending classes ~~without suppressing harmless local feedback~~ that preserve non-terminal document effects while making observer terminal selection, paging, and preview visibly inert, rendering only authoritative controller-owned highlights, and keeping presentation-only pending controls at normal cursor, opacity, and saturation · `frontend/client/client.css`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent canonical/client behavior (different files):**

- [x] **T033** [P] [US2] Implement request fingerprinting, assignment/controller/connected/terminal authorization, ordered live mutation, revisioned effects, and accepted/rejected cached action results · `internal/control/service.go`
- [x] **T034** [P] [US2] ⚠️ Reopened (reopened — BUG-003; reopened — BUG-006; reopened — BUG-008): Add request IDs and broadcast/terminal/context preconditions, gate gameplay and presentation paths by role/pending before local terminal mutation, keep the current controller's rendered targets actionable, apply revisioned snapshots including controller-owned presentation without a default-selection flash, clear pending only from authoritative results, and derive selection/preview and bad/good/lockout audio only from applicable newly applied authoritative transitions · `frontend/client/client.js`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — independent boundary integrations (different files):**

- [x] **T035** [P] [US2] ⚠️ Reopened (reopened — BUG-003): Dispatch active-controller sender-aware navigation/hacking commands through the coordinator, enqueue canonical fanout before per-request results, and reject all pre-handshake or unauthorized actions · `internal/player/server.go`
- [x] **T036** [P] [US2] Route exact private `ForceHackSuccess` through the same ordered coordinator revision without granting any player capability · `app.go`
- [x] **T037** [P] [US2] ⚠️ Reopened (reopened — BUG-003; reopened — BUG-006; reopened — BUG-007): Update the existing hacking browser fixture for hello/welcome, request preconditions, revisions, action results, explicit active-controller password/filler/pattern actionability, and the complete interaction/outcome audio matrix while preserving camouflage, no-optimistic-mutation, and no-reconnect-replay assertions · `tests/browser/hacking-camouflage.spec.mjs`

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — composition join:**

- [x] **T038** [US2] ⚠️ Reopened (reopened — BUG-003): Wire ordered terminal effects and the unchanged private hack-state event through the composed coordinator/player/master sinks, proving the active browser receives consistent authority and completion state · `main.go`

**Checkpoint**: User Story 2 is independently functional and testable: one controller drives unchanged canonical gameplay, observers remain local-only, and every shared request has an authoritative completion.

---

## Phase 5: User Story 3 — Reuse One Device Session (Priority: P1)

**Goal**: Refreshes, reopens, reconnects, and multiple tabs from one recognized browser profile reuse one process-local logical session and aggregate presence correctly.

**Independent Test**: Assign one browser profile, open at least three tabs, close/reopen them in varied order, and verify one logical identity/claim remains connected until the final tab closes.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T039** [P] [US3] Cover known/absent/unknown token attachment, unique replacement tokens, first/last connection presence transitions, stable fallback/claim/role, and no release on a new unrecognized session · `internal/control/service_test.go`
- [x] **T040** [P] [US3] Cover three-tab membership, one-close continuity, final-close presence, refresh/reconnect snapshots, different tokens/profiles, and stale token replacement over real sockets · `internal/player/server_test.go`
- [x] **T041** [P] [US3] Cover reload, reopen, three pages in one BrowserContext, another context, cleared storage, private-context-equivalent isolation, and first-use cross-tab handshake serialization · `tests/browser/player-sessions-control.spec.mjs`
- [x] **T042** [P] [US3] Prove recognition tokens are absent from HTTP URLs/query endpoints/loggable asset paths and existing same-host Origin/CSP/nosniff behavior is unchanged · `internal/player/http_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — canonical membership:**

- [x] **T043** [US3] Implement token lookup/replacement, connection-set attach/detach idempotency, aggregate presence effects, and stable logical-session reuse without claim release · `internal/control/service.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent transport/browser behavior (different files):**

- [x] **T044** [P] [US3] Track connection-to-session association through close, emit only first/last presence changes, and resend current personalized/canonical snapshots on reconnect · `internal/player/server.go`
- [x] **T045** [P] [US3] Serialize initial token issuance with Web Locks plus a storage fallback, reuse only the opaque token from `localStorage`, overwrite stale tokens, and keep the connection overlay until welcome · `client/client.js`

**Checkpoint**: User Story 3 is independently functional and testable: one recognized profile maps to one logical session across ordinary browser lifecycles and multiple tabs.

---

## Phase 6: User Story 4 — Manage Roster and Assignments (Priority: P1)

**Goal**: The game master can rename sessions, add/rename/delete roster entries, and assign/release/move claims without mutating terminal or puzzle state.

**Independent Test**: Exercise every roster and assignment correction, including claimed-delete refusal, and deep-compare canonical terminal/puzzle state before and after each operation.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T046** [P] [US4] Cover name validation/uniqueness, duplicate character names, rename stability, claimed-delete refusal, GM assign, release, move, inverse-index invariants, controller clearing, and byte-equivalent terminal/puzzle state · `internal/control/service_test.go`
- [x] **T047** [P] [US4] Cover validated roster/session/assignment Wails payloads, conflict errors with unchanged snapshots, detached events, and no calls into durable session saving · `app_test.go`
- [x] **T048** [P] [US4] Cover all-tab roster/assignment fanout, release-to-selection, rename propagation, transfer, claimed-delete refusal, and absence of claimant/connection data in player JSON · `internal/player/server_test.go`
- [x] **T049** [P] [US4] Cover live roster rename, claimed/available updates, release back to selection, character-primary/fallback-secondary identity, and no canonical terminal change · `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — independent canonical/markup/style work (different files):**

- [x] **T050** [P] [US4] Implement character rename/delete, logical-session rename, GM assign/release/move, inverse claim indexes, controller-clear rules, and detached roster/player/master effects · `internal/control/service.go`
- [x] **T051** [P] [US4] Add complete roster CRUD, logical-session labels, assignment/release/move controls, and accessible error/status regions to the coordination panel · `frontend/src/index.html`
- [x] **T052** [P] [US4] Style roster/session rows, claim states, correction controls, and responsive overflow within the existing master layout · `frontend/src/master.css`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent boundary/client integrations (different files):**

- [x] **T053** [P] [US4] Add validated `RenameCharacter`, `DeleteCharacter`, `RenameLogicalSession`, `AssignCharacter`, `ReleaseCharacter`, and `MoveCharacter` Wails methods · `app.go`
- [x] **T054** [P] [US4] Add exact lower-camel roster/session/assignment commands to the frozen desktop facade · `frontend/src/desktop-api.js`
- [x] **T055** [P] [US4] Route coordinator roster/assignment effects into complete per-session state for every affected tab without exposing private session data · `internal/player/server.go`
- [x] **T056** [P] [US4] Apply roster/identity/assignment changes by stable IDs, return released sessions to selection, and retain all canonical terminal mirrors · `client/client.js`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — game-master integration:**

- [x] **T057** [US4] Render and await all roster/session/claim corrections, show claimed-delete and conflict refusals, and retain authoritative terminal/puzzle UI state · `frontend/src/master.js`

**Checkpoint**: User Story 4 is independently functional and testable: the game master can repair names and claims without affecting gameplay state.

---

## Phase 7: User Story 5 — Reassign Terminal Control (Priority: P1)

**Goal**: The game master can atomically designate a connected assigned observer as controller, including during action races, without changing claims or gameplay state.

**Independent Test**: Reassign between two connected assigned sessions during navigation and hacking and verify ordered authority, all-tab role updates, and exact puzzle continuity.

**BUG-008 deferral**: T061 and T066 are reopened because their prior role-swap scope gated outbound shared sends while deliberately retaining each browser's local feedback. Phase 22 adds atomic semantic-presentation authority transfer and exact observer following.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T058** [P] [US5] Cover eligible/ineligible reassignment, 100 interleaved action-versus-reassignment trials, former-controller rejection, one controller invariant, and exact assignment/terminal/puzzle preservation · `internal/control/service_test.go`
- [x] **T059** [P] [US5] Cover validated `SetActiveController`, connected-assigned eligibility, refusal snapshots, and ordered master event emission · `app_test.go`
- [x] **T060** [P] [US5] Cover all-tab active/observer fanout and before/after reassignment action ordering over concurrent real sockets · `internal/player/server_test.go`
- [x] **T061** [P] [US5] ⚠️ Reopened (reopened — BUG-008): Cover live role swap, immediate former-controller terminal-input lock and authoritative following, immediate new-controller pointer/keyboard eligibility from the unchanged shared selection/page/preview, and unchanged displayed character/terminal/navigation/puzzle state · `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — canonical reassignment:**

- [x] **T062** [US5] Implement connected-assigned validation and atomic controller replacement in the same order as player actions while preserving claims and every terminal runtime field · `internal/control/service.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent boundary fanout (different files):**

- [x] **T063** [P] [US5] Add the validated `SetActiveController` Wails method and updated detached coordination status · `app.go`
- [x] **T064** [P] [US5] Add `setActiveController` to the narrow desktop facade · `frontend/src/desktop-api.js`
- [x] **T065** [P] [US5] Fan one reassignment revision to every connection of the former and new controller sessions before later action results · `internal/player/server.go`
- [x] **T066** [P] [US5] ⚠️ Reopened (reopened — BUG-008): Apply authoritative active/observer changes across tabs, immediately transfer gameplay and presentation authority, discard any former-controller local selection divergence, preserve the shared selection/page/preview and character identity, and render subsequent controller-owned presentation revisions · `frontend/client/client.js`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — game-master integration:**

- [x] **T067** [US5] Render eligible controller controls and active/observer status, await reassignment, and show refusal without optimistic role changes · `frontend/src/master.js`

**Checkpoint**: User Story 5 is independently functional and testable: controller changes are atomic, globally ordered, and gameplay-neutral.

---

## Phase 8: User Story 6 — Handle Controller Disconnects (Priority: P1)

**Goal**: Last-connection loss retains the controller and claim without promotion; reconnect restores control unless the game master reassigned it meanwhile.

**Independent Test**: Disconnect a multi-tab controller, reconnect before and after reassignment, and verify presence, role, claim, and puzzle continuity in player and master views.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T068** [P] [US6] Cover observer/controller last-close transitions, retained claim/controller, zero promotion, reconnect-before/after-reassignment roles, and byte-equivalent terminal/puzzle state · `internal/control/service_test.go`
- [x] **T069** [P] [US6] Cover multi-tab controller close order, final disconnect, no observer promotion, reconnect snapshots, and reassigned former-controller observer status · `internal/player/server_test.go`
- [x] **T070** [P] [US6] Cover master projection/event replay for a disconnected active session and unchanged claims/controller across transient connection changes · `app_test.go`
- [x] **T071** [P] [US6] Cover connection overlay/reconnect for unchanged controller and reassigned former controller without selection or canonical-state reset · `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — canonical disconnect semantics:**

- [x] **T072** [US6] Preserve claim/controller on final detach, prohibit automatic promotion, and emit presence-only effects ordered with reassignments and actions · `internal/control/service.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent projections (different files):**

- [x] **T073** [P] [US6] Publish last-close presence and reconnect role snapshots without clearing or regenerating terminal state · `internal/player/server.go`
- [x] **T074** [P] [US6] Restore the welcomed authoritative role after reconnect and never infer promotion from connection loss · `client/client.js`
- [x] **T075** [P] [US6] Keep disconnected sessions visible, mark a disconnected controller distinctly, and offer reassignment without changing its claim · `frontend/src/master.js`

**Checkpoint**: User Story 6 is independently functional and testable: network interruption never elects a controller or changes gameplay, and reconnect behavior follows explicit reassignment history.

---

## Phase 9: User Story 7 — Follow the Active Terminal (Priority: P1)

**Goal**: One broadcast-wide active terminal, or none, is authoritative; all assigned players follow switches while sessions, claims, and controller remain unchanged.

**Independent Test**: Switch among configured terminals at least ten times with active and observer sessions, include a no-terminal waiting interval and a late assignment, and verify convergence plus stale-terminal rejection.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T076** [P] [US7] Cover checkpointable terminal runtime creation/update, navigation revalidation, fresh puzzle generation, detached public state, and no reset on active content update · `internal/live/service_test.go`
- [x] **T077** [P] [US7] Cover broadcast-wide active terminal/no-terminal state, direct completed-puzzle switches, session/claim/controller preservation, late-assignee current-terminal join, and stale/inactive terminal rejection · `internal/control/service_test.go`
- [x] **T078** [P] [US7] Cover validated terminal activation/clear/update bridge results, authoritative status, and no optimistic live-terminal mutation · `app_test.go`
- [x] **T079** [P] [US7] Cover ordered revisioned `TERMINAL_LIVE`/`TERMINAL_CLEAR` fanout, ten switches, late assignment, reconnect, and stale-terminal action results · `internal/player/server_test.go`
- [x] **T080** [P] [US7] Cover terminal reveal transition, assigned waiting, ten automatic switches, late selection into the current terminal, and stable identity/character/role · `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — live runtime boundary:**

- [x] **T081** [US7] Add private terminal runtime create/update/project helpers that preserve existing navigation/hacking state and support coordinator-owned active/suspended slots · `internal/live/service.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — canonical terminal selection:**

- [x] **T082** [US7] Implement direct active-terminal activation/clear, runtime-slot ownership, late-assignee snapshots, stale-terminal guards, and identity/claim/controller preservation · `internal/control/service.go`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — independent boundary/client integrations (different files):**

- [x] **T083** [P] [US7] Replace set/clear conflation with validated `RequestTerminalActivation`, `RequestTerminalClear`, and ordered `UpdateLiveTerminal` Wails methods · `app.go`
- [x] **T084** [P] [US7] Add request-terminal-activation/clear methods while retaining update-live and exact private forced-success methods · `frontend/src/desktop-api.js`
- [x] **T085** [P] [US7] Emit revisioned terminal live/update/clear effects selectively to assigned sessions and reject actions for inactive terminals · `internal/player/server.go`
- [x] **T086** [P] [US7] Apply active-terminal revisions through the existing reveal presentation, show assigned waiting when null, and retain identity/assignment/role through switches · `client/client.js`

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — game-master integration:**

- [x] **T087** [US7] Drive make-active and clear-active controls from coordinator status, await results, and distinguish no active terminal from broadcast end · `frontend/src/master.js`

**Checkpoint**: User Story 7 is independently functional and testable: every assigned player follows the single selected terminal or waiting state without losing broadcast identity or authority.

---

## Phase 10: User Story 8 — Decide an Unfinished Puzzle's Fate (Priority: P1)

**Goal**: Switching away from an unfinished puzzle requires preserve, discard, or cancel; preservation restores the exact private puzzle and inactive terminals reject actions.

**Independent Test**: Exercise all three decisions, switch back after preserve/discard, compare every puzzle field, and verify the source remains active while the decision is pending or cancelled.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T088** [P] [US8] Deep-compare secret word, generation, board, attempts, candidates, removed duds, patterns, log, outcome, and navigation across private suspend/reactivate; verify discard regenerates and content refresh revalidates navigation · `internal/live/service_test.go`
- [x] **T089** [P] [US8] Cover decision-required detection, opaque switch IDs, preserve/discard/cancel, stale decision refusal, continued source actions while pending, active/preserved deletion guard, and inactive action rejection · `internal/control/service_test.go`
- [x] **T090** [P] [US8] Cover switch result shapes, decision validation, stale refusal, detached status, and exact `ForceHackSuccess` eligibility while a decision is pending · `app_test.go`
- [x] **T091** [P] [US8] Cover preserve/discard/cancel player fanout, no premature terminal clear, exact restored public puzzle, and inactive-terminal crafted-request rejection · `internal/player/server_test.go`
- [x] **T092** [P] [US8] Cover unchanged source display while pending/cancelled, preserve restore, discard fresh puzzle, and no identity/assignment/role changes across the decision · `tests/browser/player-sessions-control.spec.mjs`
- [x] **T093** [P] [US8] Assert accessible preserve/discard/cancel dialog markup, destructive-action emphasis, hidden-state behavior, and no player exposure of switch or private puzzle controls · `internal/platform/assets_test.go`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — exact live checkpoints:**

- [x] **T094** [US8] Implement exact private suspend/reactivate/discard helpers, retain checkpoint generation level, apply latest name/tree/intro, and revalidate navigation without exposing private state · `internal/live/service.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — canonical switch transaction:**

- [x] **T095** [US8] Implement pending switch tokens, preserve/discard/cancel resolution, stale/source/broadcast checks, deletion guard, and ordered source/target effects · `internal/control/service.go`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — independent game-master surfaces (different files):**

- [x] **T096** [P] [US8] Add validated `ResolveTerminalSwitch` and switch-result payloads while keeping `ForceHackSuccess` exact and private · `app.go`
- [x] **T097** [P] [US8] Add `resolveTerminalSwitch` and normalized switch-command results to the desktop facade · `frontend/src/desktop-api.js`
- [x] **T098** [P] [US8] Add the blocking preserve/discard/cancel dialog and stale/error status region to the master document · `frontend/src/index.html`
- [x] **T099** [P] [US8] Style the switch dialog, destructive discard option, focus states, and responsive bounds in the master aesthetic · `frontend/src/master.css`

**⟶ Wait for Wave 4 to finish, then:**

**Wave 5 — game-master integration:**

- [x] **T100** [US8] Await decision-required activation/clear results, drive preserve/discard/cancel resolution, keep the source authoritative while pending, and guard active/preserved terminal deletion · `frontend/src/master.js`

**Checkpoint**: User Story 8 is independently functional and testable: no unfinished puzzle can be silently lost, altered, or acted on while inactive.

---

## Phase 11: User Story 9 — End and Restart Broadcast Lifetimes (Priority: P2)

**Goal**: Broadcast end clears claims/control/runtime terminals while retaining process sessions/roster; the next broadcast requires selection, and process restart restores no runtime identity or ownership.

**Independent Test**: End a populated broadcast, start another in the same process, then restart the application and verify each required lifetime boundary plus unchanged durable terminal data.

### Tests

**Wave 1 — independent (different files; write failing tests first):**

- [x] **T101** [P] [US9] Cover end/start lifetime cleanup, retained sessions/fallback names/roster, cleared claims/controller/active/suspended/request caches, fresh broadcast ID, and fresh initial controller · `internal/control/service_test.go`
- [x] **T102** [P] [US9] Compare version-1 encoding before/after complete runtime activity and prove no runtime coordination or puzzle field is persisted and durable unlocked-terminal behavior is unchanged · `internal/domain/model_test.go`
- [x] **T103** [P] [US9] Cover end-broadcast player clear/context, same-process reselection, stale old-broadcast requests, and unknown prior-process token replacement with no restored state · `internal/player/server_test.go`
- [x] **T104** [P] [US9] Cover validated `EndBroadcast`, second `StartBroadcast`, shutdown cleanup, status replay, and no durable terminal deletion/mutation · `app_test.go`
- [x] **T105** [P] [US9] Cover end-to-selection/waiting, second-broadcast reselection, stale pending completion, and fresh welcome after simulated process restart · `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Implementation

**Wave 2 — canonical lifetime cleanup:**

- [x] **T106** [US9] Implement atomic broadcast end/start cleanup and process shutdown disposal while retaining only process-scoped sessions/fallback names/roster between broadcasts · `internal/control/service.go`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — independent boundary/client integrations (different files):**

- [x] **T107** [P] [US9] Add validated `EndBroadcast`, second-start behavior, shutdown coordinator cleanup, and unchanged durable-session bridge separation · `app.go`
- [x] **T108** [P] [US9] Add `endBroadcast` and retained `startBroadcast` commands to the desktop facade · `frontend/src/desktop-api.js`
- [x] **T109** [P] [US9] Publish terminal clear and fresh personalized contexts on end/start, reject stale broadcast IDs, and replace tokens unknown to a fresh process · `internal/player/server.go`
- [x] **T110** [P] [US9] Clear pending/canonical mirrors on broadcast end, retain only server-confirmed identity display, and require new selection for every new broadcast · `client/client.js`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — game-master integration:**

- [x] **T111** [US9] ⚠️ Reopened (reopened — BUG-004): Drive broadcast end/start from authoritative coordination status, retain roster/session labels, clear live controls, and never mutate configured durable terminals; ensure the visible end-broadcast control can confirm and invoke the authoritative command exactly once · `frontend/src/master.js`

**Checkpoint**: User Story 9 is independently functional and testable: broadcast and process boundaries clear exactly the intended runtime state and preserve exactly the intended authored/process state.

---

## Phase 12: Polish and Cross-Cutting Validation

Finalize operational documentation, enforce security/presentation contracts across the complete surface, and run the baseline SC-001 through SC-022 validation suite here; later bugfix and convergence phases rerun the affected and full gates after subsequent changes.

**Wave 1 — independent (different files):**

- [x] **T112** [P] Document the game-master roster/assignment/controller/switch workflow, browser-profile recognition scope, broadcast lifetimes, and unchanged private `ForceHackSuccess` boundary · `README.md`
- [x] **T113** [P] Consolidate cross-cutting asset assertions for restrictive CSP, escaped names, token secrecy, observer local-only behavior, pending resolution, responsive layouts, and absence of every player forced-success path · `internal/platform/assets_test.go`

**⟶ Wait for Wave 1 to finish, then:**

**Wave 2 — automated Success Criteria validation:**

- [x] **T114** ⚠️ Reopened (reopened — BUG-004): Run `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`, `npm --prefix frontend run build`, and `npm --prefix tests/browser test`; record each result and map SC-001 through SC-022 to evidence, then add the composed end-broadcast evidence required for SC-029 · `specs/004-player-sessions-control/validation.md`

**⟶ Wait for Wave 2 to finish, then:**

**Wave 3 — interactive/package gates:**

- [x] **T115** Run affected `wails dev` master/player journeys and a clean macOS `wails build`, recording unavailable signing/notarization credentials rather than claiming those release-only gates · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: All nine stories, cross-cutting boundaries, SC-001 through SC-022, browser interactions, concurrency/race behavior, and package-level smoke gates are verified or explicitly reported unavailable.

---

## Phase 13: BUG-001 — Persist Reusable Player Rosters (User Stories 4 and 9)

**Goal**: A session can reference a separately reusable player-config JSON file so authored roster IDs/names survive restart and reopen, while logical sessions, presence, claims, controller state, broadcasts, terminals, and puzzles retain their existing runtime lifetimes.

**Independent Test**: Create or open a session, create/select a player config, add and rename characters, restart, reopen the session, and verify automatic stable-roster restoration with zero restored runtime coordination state. Repeat with legacy sessions, cancellation, missing/invalid configs, failed writes, and two sessions sharing one player config.

**Task disposition**: No completed pre-BUG-001 task was falsely completed against the prior specification, so no task is reopened. The following tasks supersede the process-local-roster portions of T002, T016, T047, and T101–T112 without erasing their historical completion.

**BUG-002 correction disposition**: T116, T118, T119, T123, T124, T125, and T129 are reopened because their completed coverage did not distinguish an empty roster array from nil across the real create-and-install workflow. T127 remains complete but receives regression review when the reopened browser and integration checks run.

### Tests

**Wave 1 — independent failing coverage (different files):**

- [x] **T116** [P] [US4] ⚠️ Reopened (reopened — BUG-002): Specify version-1 player-config JSON, optional relative session association, legacy omission, strict validation, stable ordered roster IDs/names, atomic create/save, shared-config reuse, filesystem failures, and explicit preservation of a non-nil empty roster through result cloning and JSON `[]` encoding · `internal/playerconfig/service_test.go`, `internal/domain/model_test.go`
- [x] **T117** [P] [US4] Prove referenced roster load/replacement is allowed only without a broadcast, CRUD requires an active store, persistence succeeds before publication, and failed persistence leaves roster/claims/controller/terminal/puzzle snapshots unchanged · `internal/control/service_test.go`
- [x] **T118** [P] [US4] ⚠️ Reopened (reopened — BUG-002): Cover referenced auto-load, native create/open cancellation, relative association save, missing/invalid recovery, unchanged prior association on failure, status replay, validated roster CRUD, and the real empty-roster `Create → association → coordinator install → first add` chain through the Wails facade · `app_test.go`, `internal/session/service_test.go`
- [x] **T119** [P] [US4] ⚠️ Reopened (reopened — BUG-002): Cover the post-session select-or-create journey, successful activation of a newly created empty config, automatic reopen restoration, disabled roster/broadcast controls without an active config, visible recoverable errors, stable names after restart, and no restored claims/controller · `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Durable model and master surfaces

**Wave 2 — independent prerequisites (different files):**

- [x] **T120** [P] [US4] Add `PlayerConfig`, durable roster-entry encoding, active player-config metadata, validation limits, and the optional `Session.playerConfig` relative reference without changing terminal JSON or runtime-only projections · `internal/domain/model.go`, `internal/domain/json.go`, `internal/domain/validate.go`
- [x] **T121** [P] [US4] Add accessible select-existing/create-new player-config actions, loaded-config status, recoverable error state, and disabled roster/broadcast presentation within the existing master layout · `frontend/src/index.html`, `frontend/src/master.css`, `internal/platform/assets_test.go`
- [x] **T122** [P] [US4] Add exact `loadReferencedPlayerConfig`, `newPlayerConfig`, and `openPlayerConfig` methods plus normalized results to the frozen desktop facade · `frontend/src/desktop-api.js`

**⟶ Wait for Wave 2 to finish, then:**

### Persistence and canonical coordination

**Wave 3 — player-config storage:**

- [x] **T123** [US4] ⚠️ Reopened (reopened — BUG-002): Implement native select/create, strict load, relative-reference resolution, deterministic config naming, atomic complete-file replacement, and roster cloning that preserves a non-nil empty array without exposing filesystem capability to the player surface · `internal/playerconfig/service.go`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — coordinator persistence boundary:**

- [x] **T124** [US4] ⚠️ Reopened (reopened — BUG-002): Add the narrow roster-store interface, no-broadcast roster installation that accepts a non-nil empty roster while rejecting nil, active-config requirement, and save-before-commit add/rename/unclaimed-delete ordering with no-mutation failures · `internal/control/service.go`

**⟶ Wait for Wave 4 to finish, then:**

### Boundary integration and restoration

**Wave 5 — independent integrations (different files):**

- [x] **T125** [P] [US4] ⚠️ Reopened (reopened — BUG-002): Compose player-config storage with the active session and coordinator; implement referenced load, create/open association including a valid empty roster, recoverable cancellation/errors, status projection, and durable roster CRUD through validated App methods · `app.go`, `main.go`, `internal/session/service.go`
- [x] **T126** [P] [US9] Publish a newly loaded roster to connected unassigned sessions while restoring no prior browser identity, claim, controller, broadcast, terminal runtime, or puzzle · `internal/player/server.go`

**⟶ Wait for Wave 5 to finish, then:**

**Wave 6 — game-master integration:**

- [x] **T127** [US4] Run referenced auto-load after session create/open, present select/create fallback and retries, render active config status, gate roster/broadcast controls, and await durable roster commands without optimistic state · `frontend/src/master.js`

**⟶ Wait for Wave 6 to finish, then:**

### Documentation and validation

**Wave 7 — documentation and full verification:**

- [x] **T128** [P] Document player-config format, session-relative association, reuse, file recovery, runtime exclusions, and the updated game-master workflow · `README.md`, `specs/004-player-sessions-control/contracts/desktop-coordination.md`
- [x] **T129** ⚠️ Reopened (reopened — BUG-002): Run formatting, vet, full and race Go suites, frontend build, browser suite, the real newly-created-empty-config Wails journey, and package build; append SC-023 through SC-027 evidence and BUG-001/BUG-002 results · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: BUG-001 is independently complete when reopened sessions restore the exact authored roster from their player config without manual re-entry, all failure paths are non-partial, and no transient coordination or puzzle value appears in either durable file.

---

## Phase 14: BUG-003 — Restore Active-Controller Hacking Selection (User Story 2)

**Goal**: The current connected, assigned active controller can select password candidates, filler cells, and unused special patterns through the production-composed player path, while observers remain read-only and all pending input resolves from authoritative results.

**Independent Test**: Start the real player server with the production coordinator and hacking runtime, connect one active controller and one observer in browsers, activate an unfinished puzzle, select one target from every hacking category, and verify exactly-once accepted mutations, shared convergence, pending completion, and zero observer shared actions.

**Task disposition**: T028–T030, T034–T035, T037, and T038 are reopened because their completed layer checks and fake-WebSocket journeys did not establish the production-composed active-controller path. T027 and T033 remain complete but require regression review against the reproduced failure before the correction is accepted.

**BUG-006 deferral**: T132 and T133 were reopened for traceability and were re-completed through T150 and T151 respectively after T148–T149; their original BUG-003 wave placement is historical and does not override the Phase 19 DAG.

**BUG-007 deferral**: T132 and T133 were reopened again for traceability and were re-completed through T154 and T155 respectively after T152–T153; their original BUG-003/BUG-006 wave placement is historical and does not override the Phase 20 DAG.

### Reproduction and failing coverage

**Wave 1 — production-composed regression first:**

- [x] **T130** [US2] Add a real coordinator/player-server browser fixture and a failing journey from handshake, character assignment, and terminal activation through active-controller password, filler, and unused-pattern selection; assert exact request fields, observer silence, canonical mutation, convergence, and pending completion · `tests/browser/fixture-server/main.go`, `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/playwright.config.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Correction

**Wave 2 — isolate and repair the first broken production boundary:**

- [x] **T131** [US2] Trace the T130 journey across rendered target handling, role/phase and terminal-mirror gating, sender/session binding, coordinator authorization, runtime application, revisioned projection, and `ACTION_RESULT`; implement the smallest confirmed correction while preserving observer rejection and the private `ForceHackSuccess` boundary · `client/client.js`, `internal/player/server.go`, `internal/control/service.go`, `main.go`

**⟶ Wait for Wave 2 to finish, then:**

### Regression and validation

**Wave 3 — focused regressions:**

- [x] **T132** [US2] ⚠️ Reopened (reopened — BUG-006; reopened — BUG-007): Complete the reopened T028–T030, T034–T035, T037, and T038 checks; add focused regressions for the corrected action and complete audio-family boundaries, and confirm T027/T033 authorization invariants still reject observers, stale terminals, and duplicate actions without mutation or outcome-audio replay · `internal/control/service_test.go`, `internal/player/server_test.go`, `internal/platform/assets_test.go`, `tests/browser/hacking-camouflage.spec.mjs`, `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — complete BUG-003 verification:**

- [x] **T133** [US2] ⚠️ Reopened (reopened — BUG-006; reopened — BUG-007): Run formatting, vet, full and race Go suites, frontend build, browser suite, a native composed active-controller hacking journey including all established interaction and outcome cues, and package build; append SC-028/SC-032/SC-033 and BUG-003/BUG-006/BUG-007 evidence without weakening SC-007, SC-021, or SC-022 · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: BUG-003 is complete only when a real active browser can select every hacking target category through the composed server-authoritative path, all assigned views converge, pending input resolves, and observers remain unable to emit shared actions.

---

## Dependencies & Execution Order

### Requirement Traceability

| Requirement range | Primary task coverage |
|---|---|
| FR-001–FR-018 | T002–T016, T039–T045, T101–T126 |
| FR-019–FR-052 | T010–T027, T039–T075, T116–T129 |
| FR-053–FR-075 | T027–T038, T058–T066, T130–T146 |
| FR-076–FR-099 | T076–T115, T135–T146, T156 |
| FR-100–FR-115 | T116–T129, T134 |
| FR-116–FR-121 | T139–T144 |
| FR-122–FR-125 | T029–T037, T132–T155 |
| FR-126–FR-129 | T029–T037, T132–T155 |
| FR-130–FR-138 | T029–T030, T032, T034, T061, T066, T158–T166 |
| FR-139–FR-140 | T029–T030, T032, T164–T170 |
| FR-141–FR-143 | T160, T164–T165, T168–T174 |
| SC-001–SC-022 | T010–T115, T145–T147 |
| SC-023–SC-027 | T116–T129, T134 |
| SC-028–SC-033 | T130–T155 |
| SC-034–SC-036 | T158–T166 |
| SC-037 | T167–T170 |
| SC-038 | T171–T174 |

### Phase dependencies

1. **Phase 1: Setup** enables discovery of new browser tests.
2. **Phase 2: Foundational** depends on Setup and blocks every story.
3. **Phase 3: US1** depends on Foundational and is the MVP identity/selection slice.
4. **Phase 4: US2** depends on US1 identities/roles and establishes authoritative shared-action control.
5. **Phase 5: US3** depends on US1 handshake/session state and hardens recognition and multi-tab presence.
6. **Phase 6: US4** depends on US1 claims and US2 ordered effects to add corrections safely.
7. **Phase 7: US5** depends on US2 authorization and US4 assignment operations.
8. **Phase 8: US6** depends on US3 presence and US5 reassignment semantics.
9. **Phase 9: US7** depends on US2 action preconditions and introduces broadcast-wide active-terminal state.
10. **Phase 10: US8** depends on US7 runtime slots/switching and adds unfinished-puzzle decisions.
11. **Phase 11: US9** depends on every process/broadcast/runtime aggregate that it clears.
12. **Phase 12: Polish** depends on all story checkpoints and owns the single complete validation run.
13. **Phase 13: BUG-001** depends on the completed session, roster, coordinator, desktop-facade, and restart paths from US1, US4, US9, and Polish. Within the phase: Wave 1 tests precede Wave 2 prerequisites; T123 depends on T120; T124 depends on T117 and T123; T125 depends on T118, T120, T123, and T124; T126 depends on T117 and T124; T127 depends on T119, T121, T122, and T125; T128–T129 depend on all implementation tasks.
14. **BUG-002 correction** reuses the Phase 13 DAG: reopened T116/T118/T119 establish regression coverage; T123 then fixes service cloning; T124 fixes coordinator installation; T125 completes the real integration; T127 receives regression review after T119 and T125; and T129 reruns only after every reopened implementation/test task passes.
15. **Phase 14: BUG-003** depends on the completed US1 identity path and the US2 authorization/runtime boundaries. T130 must reproduce the failure first; T131 then repairs only the confirmed boundary; T132 completes every reopened task and focused regression; T133 runs only after T130–T132 pass.
16. **Phase 15: Convergence** depends on Phase 14 completion. T134 resolves the FR-105 and US4/AC10 cancellation-presentation contradiction and its browser regression before any later bugfix relies on the converged player-config workflow.
17. **Phase 16: BUG-004** depends on the completed US9 lifecycle layers and Phase 15 convergence. T135 must reproduce the game-master interaction failure first; T136 then repairs only the confirmed boundary; T137 completes reopened T111 and runs regression review for T104–T110/T115; T138 completes reopened T114 and full validation only after T135–T137 pass.
18. **Phase 17: BUG-005** depends on the completed US2 authoritative hacking path, US7 terminal runtime ownership, US8 private discard helpers, and Phase 16 composition. T139–T141 add independent failing coverage; T142 implements the canonical runtime replacement after those tests fail; T143 completes the trusted master-to-player composition after T142; T144 runs only after T139–T143 pass.
19. **Phase 18: Convergence** depends on Phase 17 completion. T145 restores assigned-session reconnect projection first; T146 then completes the coordinator-owned private `ForceHackSuccess` composition; T147 applies the final scoped JavaScript convention enforcement after the behavioral convergence tasks.
20. **Phase 19: BUG-006** depends on the completed US2/BUG-003 authoritative hacking path and Phase 18 convergence. T148 must reproduce and localize the missing outcome playback first; T149 then repairs only the confirmed client/audio boundary; T150 completes the reopened T029/T030/T034/T037/T132 regressions; T151 completes reopened T133 and full verification only after T148–T150 pass.
21. **Phase 20: BUG-007** depends on the BUG-006 outcome mapping and the complete US2 interaction path. T152 must reproduce every required hacking-screen family with the native browser audio stack and localize the first failing production boundary; T153 then repairs only that confirmed boundary; T154 completes reopened T029/T030/T037/T132/T148–T150 plus T034 regression review; T155 completes reopened T133/T151 and full packaged verification only after T152–T154 pass.
22. **Phase 21: Convergence** depends on Phase 20 completion. T156 runs after T155 to remove the remaining production coordinator bypass while preserving the authoritative activation, clear, update, and private forced-success boundaries. T157 runs only after T156 and is the final constitution-mandated validation gate.
23. **Phase 22: BUG-008** depends on the generated ConnectRPC cutover, the completed US2 authority path, US5 reassignment, assigned-session reconnect convergence, and Phase 21. T158–T160 establish independent failing coordinator, contract, and browser coverage; T161 defines the additive generated model/contract; T162 implements canonical presentation ownership after T161; T163 and T164 integrate transport and browser behavior after T162; T165 completes all reopened task scopes and focused regressions; T166 runs only after T158–T165 pass.
24. **Phase 23: BUG-009** depends on BUG-008's generated authoritative presentation and latest-wins browser correlation. T167 first reproduces the delayed-`SetPresentation` visual lock; T168 separates presentation correlation from blocking styling; T169 completes all reopened scopes and regression review after T168; T170 runs only after T167–T169 pass.
25. **Phase 24: BUG-010** depends on BUG-009's non-blocking authoritative presentation path. T171 first reproduces hacking-hover request buildup and stale replay; T172 implements one-in-flight/one-latest bounded dispatch; T173 completes reopened T164/T165/T168/T169 and focused T160 review after T172; T174 completes reopened T170 and full verification only after T171–T173 pass.

### Wave restatement

- **Setup**: Playwright discovery completes before any new browser specification is relied on.
- **Foundational**: failing tests → independent domain/protocol/connection/fake work → coordinator skeleton.
- **US1**: failing claim/handshake/UI tests → independent canonical/markup/style work → boundary integrations → master/composition join.
- **US2**: failing authorization/convergence/UI tests → live/CSS prerequisites → coordinator/client behavior → server/App/browser-fixture integrations → composition join.
- **US3**: failing membership/storage/security tests → canonical membership → server/browser reconnect behavior.
- **US4**: failing CRUD/correction tests → canonical/markup/style work → bridge/server/client integrations → master join.
- **US5**: failing reassignment/race tests → canonical reassignment → boundary fanout → master join.
- **US6**: failing disconnect/reconnect tests → canonical detach semantics → server/client/master projections.
- **US7**: failing live/control/bridge/fanout/browser tests → live runtime helpers → canonical terminal selection → boundary integrations → master join.
- **US8**: failing preservation/decision/UI tests → exact private checkpoints → canonical switch transaction → master boundary/markup/style → dialog integration.
- **US9**: failing lifetime/persistence/restart tests → canonical cleanup → boundary/client integrations → master join.
- **Polish**: independent documentation/security contracts → automated suites and SC evidence → interactive/package gates.
- **BUG-001**: failing storage/coordinator/bridge/browser tests → durable model and master prerequisites → player-config storage → save-before-commit coordination → App/player restoration integrations → master workflow → documentation and complete verification.
- **BUG-002**: reopened empty-array unit/bridge/browser tests → player-config clone correction → coordinator empty-roster installation correction → real App integration → T127 regression review → complete verification rerun.
- **BUG-003**: production-composed failing browser journey → confirmed-boundary correction → reopened layer and interaction regressions → full suites, native journey, and SC-028 evidence.
- **Phase 15 Convergence**: BUG-003 completion → T134 cancellation-presentation correction and browser regression → converged player-config workflow required by BUG-004.
- **BUG-004**: production-shaped failing game-master click journey → confirmed-boundary correction → reopened master integration and lifecycle regression review → full suites, native click-through journey, and SC-029 evidence.
- **BUG-005**: failing runtime/coordinator, private-boundary/fanout, and production-shaped browser journeys → canonical failed-runtime replacement → trusted master-to-player composition → full suites, native retry journey, and SC-030/SC-031 evidence.
- **Phase 18 Convergence**: BUG-005 completion → T145 assigned-session reconnect projection → T146 coordinator-owned private forced-success composition → T147 scoped JavaScript convention enforcement.
- **BUG-006**: observable Web Audio failing journey → confirmed client/audio-boundary correction → reopened interaction/asset/idempotence regressions → full suites, packaged native wrong/unlock/lockout journey, and SC-032 evidence.
- **BUG-007**: native production-audio family matrix with ambient/charscroll controls → confirmed event/manifest/decode/context/routing/output correction → reopened interaction/outcome regressions → full suites, directly monitored packaged playback, and SC-033 evidence.
- **Phase 21 Convergence**: BUG-007 completion → remove the legacy `SetLiveTerminal` and `ClearLiveTerminal` production bindings → migrate focused tests to authoritative activation/clear commands → verify the legacy bindings are unavailable → run and record the final full validation suite.
- **BUG-008**: independent failing coordinator/contract/browser coverage → additive semantic-presentation model and generated ConnectRPC contract → coordinator ownership/revalidation → parallel transport and browser integration → reopened role/presentation regressions → full suites, reassignment/reconnect matrix, native smoke, and SC-034–SC-036 evidence.
- **BUG-009**: delayed-`SetPresentation` browser/style reproduction → client-only pending-state/style separation → reopened browser/asset/CSS/BUG-008 integration regressions → full suites, output-visible packaged latency journey, and SC-037 evidence.
- **BUG-010**: delayed first `SetPresentation` plus rapid hacking-grid sweep → one-in-flight/one-latest client coalescing → reopened presentation/audio/rapid-supersession regressions and role/context invalidation review → full suites, output-visible packaged rapid-hover journey, and SC-038 evidence.

Tasks tagged `[P]` are parallel only inside their declared wave. Same-file work in later phases remains sequential even when conceptually independent, preserving one writer and the story checkpoint order.

---

## Phase 15: Convergence

- [x] T134 [US4] Keep player-config cancellation informational by clearing and hiding the assertive `playerConfigError` alert while preserving terminal authoring and disabled roster/broadcast controls, and add a browser regression for the complete cancellation state per FR-105 and US4/AC10 (contradicts) · `frontend/src/master.js`, `tests/browser/player-sessions-control.spec.mjs`

---

## Phase 16: BUG-004 — Restore Game-Master Broadcast Termination (User Story 9)

**Goal**: The visible game-master end-broadcast control reliably ends the current broadcast through the authoritative composition and removes the live terminal from every connected player while retaining the required process, authored-roster, and durable-terminal state.

**Independent Test**: Start a populated broadcast with connected active and observer players, activate and confirm `ЗАВЕРШИТЬ ТРАНСЛЯЦИЮ`, and verify one end command, authoritative master no-broadcast state, terminal clear/no-broadcast context for every player, cleared claims/controller/runtime, and retained sessions/fallback names/roster/configured terminals.

**Task disposition**: T111 and T114 are reopened because the completed master integration and automated Success Criteria run did not exercise the actual end-broadcast control. T104–T110 remain complete but require focused regression review across their App, facade, coordinator, server, and player-client boundaries. T115 remains complete because it explicitly reported that deeper native interaction was unavailable; BUG-004 adds the missing click-through gate rather than erasing that record.

### Reproduction and failing coverage

**Wave 1 — production-shaped regression first:**

- [x] **T135** [US9] Reproduce the failure from a visible, enabled `#btnEndBroadcast`; deterministically accept confirmation and identify whether button state/hit testing, confirmation, desktop invocation, authoritative result application, coordinator transition, or player fanout is the first broken boundary, while asserting exactly one attempted end command · `frontend/src/master.js`, `frontend/src/index.html`, `frontend/src/master.css`, `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Correction

**Wave 2 — repair only the reproduced boundary:**

- [x] **T136** [US9] Implement the smallest correction proven by T135 so the enabled end-broadcast control confirms accessibly, invokes `desktopAPI.endBroadcast()` exactly once, awaits its authoritative result, and leaves failures visible without weakening broadcast-state guards or retained-state rules · `frontend/src/master.js`, `frontend/src/index.html`, `frontend/src/master.css`, `frontend/src/desktop-api.js`, `app.go`

**⟶ Wait for Wave 2 to finish, then:**

### Regression and validation

**Wave 3 — complete reopened integration and cross-boundary regressions:**

- [x] **T137** [US9] Complete reopened T111 and review T104–T110/T115 against the reproduced failure; prove the corrected master action reaches the coordinator once, publishes no-broadcast context plus terminal clear to active and observer players, clears broadcast-scoped state, and retains logical sessions, fallback names, authored roster, configured terminals, and durable unlocked state · `app_test.go`, `internal/control/service_test.go`, `internal/player/server_test.go`, `internal/platform/assets_test.go`, `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — complete BUG-004 verification:**

- [x] **T138** [US9] Complete reopened T114 by running formatting, vet, full and race Go suites, frontend build, complete browser suite, a native Wails end-broadcast click-through journey, and package build; append SC-029 and BUG-004 evidence without weakening SC-018/SC-019 or prior bugfix gates · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: BUG-004 is complete only when the real game-master control ends the broadcast once, every connected player leaves the active terminal from authoritative state, required retained state is unchanged, and the complete automated/native verification evidence is recorded.

---

## Phase 17: BUG-005 — Retry a Failed Hacking Puzzle (User Story 8)

**Goal**: The game master can replace the failed puzzle of the current active terminal with one fresh shared puzzle while keeping the broadcast, terminal selection, character assignments, controller, sessions, roster, other runtime slots, and durable terminal state intact.

**Independent Test**: Start a populated broadcast with active and observer players, exhaust the active terminal's attempts, activate `ПОВТОРИТЬ ВЗЛОМ`, and verify exactly one fresh generation with full attempts reaches every assigned player while identity, ownership, and durable state remain unchanged and stale-generation actions are rejected.

**Task disposition**: No completed task is falsely complete against its written scope. T088–T100 correctly cover unfinished-puzzle switching, T101–T111 correctly cover whole-broadcast restart, and T114/T115 retain their historical validation record. BUG-005 adds the previously unowned failed same-terminal retry flow and requires regression review of those boundaries without erasing their completion.

### Failing coverage

**Wave 1 — independent tests first (different files):**

- [x] **T139** [P] [US8] Add failing live/coordinator tests that reach a failed active puzzle, accept exactly one `ResetFailedHack`, create a new generation with full attempts from the latest authored settings, preserve the broadcast/active terminal/sessions/assignments/controller/roster/other runtimes/durable unlock state, reject every ineligible or stale call without mutation, and serialize duplicate retry plus old-generation action races · `internal/live/service_test.go`, `internal/control/service_test.go`
- [x] **T140** [P] [US8] Add failing App, player-server, and asset-contract tests for exact private `ResetFailedHack` validation, one ordered fresh terminal/hack fanout to active and observer sessions, detached authoritative results/errors, retained master state, stale-generation rejection, and zero player WebSocket/DOM/global/keyboard/query/public-endpoint exposure · `app_test.go`, `internal/player/server_test.go`, `internal/platform/assets_test.go`
- [x] **T141** [P] [US8] Add a failing production-shaped browser journey that exhausts attempts, observes the player blocked state and visible enabled `ПОВТОРИТЬ ВЗЛОМ` master control, invokes it exactly once, and proves active/observer convergence on one fresh board with unchanged broadcast, terminal, character, and role state · `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Canonical correction

**Wave 2 — replace only the failed active runtime:**

- [x] **T142** [US8] Implement a fresh-runtime helper and coordinator-owned `ResetFailedHack` transition that validates the current failed active puzzle, atomically discards its private generation and creates a replacement from the latest authored terminal settings, preserves every unrelated runtime field, publishes one ordered revision, and rejects stale/duplicate/ineligible transitions without mutation · `internal/live/service.go`, `internal/control/service.go`

**⟶ Wait for Wave 2 to finish, then:**

### Trusted composition

**Wave 3 — master boundary, presentation, and player publication:**

- [x] **T143** [US8] Define and implement exact `ResetFailedHack` through the desktop coordination contract, validated App method, lower-camel facade, composed effect sinks, and accessible blocked-state `ПОВТОРИТЬ ВЗЛОМ` control with awaited pending/error handling; deliver the fresh canonical projection through existing player envelopes without adding any player command or broadening `ForceHackSuccess` · `specs/004-player-sessions-control/contracts/desktop-coordination.md`, `app.go`, `frontend/src/desktop-api.js`, `frontend/src/index.html`, `frontend/src/master.css`, `frontend/src/master.js`, `internal/player/server.go`, `client/client.js`, `main.go`

**⟶ Wait for Wave 3 to finish, then:**

### Regression and validation

**Wave 4 — complete BUG-005 verification:**

- [x] **T144** [US8] Complete T139–T143, review T088–T115 against the new retry boundary, run formatting, vet, full and race Go suites, frontend build, complete browser suite, a native Wails failed-puzzle retry journey, and package build; append SC-030/SC-031 and BUG-005 evidence without weakening lockout, unfinished-puzzle preservation, broadcast lifetime, or prior bugfix gates · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: BUG-005 is complete only when the real game-master retry control creates exactly one fresh puzzle for the same active terminal, every assigned player converges, stale-generation actions cannot mutate it, no player reset path exists, and all broadcast/session/assignment/controller/roster/runtime/durable invariants remain unchanged.

---

## Phase 18: Convergence

- [x] T145 Deliver the coordinator-owned active terminal snapshot to every recognized assigned reconnect and new tab, and add a production-composed WebSocket/browser regression proving role, assignment, puzzle, and current-terminal resume without regeneration per FR-005, FR-074, US3/AC1–AC2 (partial) · `internal/control/service.go`, `internal/player/server.go`, `main.go`, `internal/player/server_test.go`, `tests/browser/player-sessions-control.spec.mjs`
- [x] T146 Route exact private `ForceHackSuccess` through the coordinator-owned current active terminal runtime, publish one ordered sanitized solved projection to all assigned players, and add a production-composed regression while preserving player-side absence and failed-puzzle ineligibility per FR-095, FR-099, SC-021, plan: Implementation Strategy 10 (contradicts) · `internal/control/service.go`, `internal/live/service.go`, `app.go`, `main.go`, `internal/control/service_test.go`, `internal/player/server_test.go`, `app_test.go`
- [x] T147 Normalize tab-indented scoped browser source to the required two-space convention and enforce that convention for frontend/client JavaScript per Constitution V, plan: Constitution Check V (partial) · `frontend/src/desktop-api.js`, `frontend/src/master.js`, `internal/platform/assets_test.go`

---

## Phase 19: BUG-006 — Restore Authoritative Hacking Outcome Audio (User Story 2)

**Goal**: Every audio-enabled continuously connected assigned player invokes the established bad cue for an ordinary wrong password selection and final lockout, and the established good cue for a successful unlock, exactly once when the corresponding newer authoritative hacking state is applied.

**Independent Test**: Start the production-composed player server with active and observer browsers, enable audio with a separate gesture in every tested player document, and use three fresh puzzle cases for an ordinary wrong candidate, a correct candidate, and a final-attempt wrong candidate. Observe one bad, good, and bad Web Audio invocation respectively on each audio-enabled continuously connected assigned view while click-only, rejected, duplicate/stale, reconnect, and unchanged-render paths remain silent.

**Task disposition**: T029, T030, T034, T037, T132, and T133 are reopened because their completed browser, asset, client-state, and full-validation scopes did not observe Web Audio outcome playback. T020 in `specs/hacking-game/tasks.md` remains historical evidence of the required behavior; BUG-006 restores its regressed result in the active feature rather than rewriting that completed dependency artifact.

**BUG-007 deferral**: T148–T151 were reopened because their synthetic source-start and headless packaged evidence did not establish audible production playback and because T148 assumed cursor/click audio was working. They were re-completed through T152–T155; the original Phase 19 wave placement is historical and does not override the Phase 20 DAG.

### Reproduction and failing coverage

**Wave 1 — observable browser regression first:**

- [x] **T148** [US2] ⚠️ Reopened (reopened — BUG-007): Add failing production-shaped active-and-observer coverage with an observable sound adapter or `AudioContext` decode/source-start spy and colocated sound-manifest/static-asset HTTP assertions; explicitly enable audio in every tested player document, distinguish working cursor audio from missing outcome audio, use separate fresh puzzles for ordinary wrong, newly solved, and final lockout, and expect one bad, good, and bad cue while click-only, rejected, stale/duplicate, reconnect, and unchanged-render paths expect none · `internal/player/http_test.go`, `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/hacking-camouflage.spec.mjs`

**⟶ Wait for Wave 1 to finish, then:**

### Correction

**Wave 2 — repair only the first reproduced audio boundary:**

- [x] **T149** [US2] ⚠️ Reopened (reopened — BUG-007): Trace T148 through accepted revision filtering, previous-to-next hacking-state comparison, `playHackBad`/`playHackGood`, folder readiness, Web Audio decode/resume/source start, and packaged sound serving; implement the smallest confirmed correction with exact-once transition guards and non-fatal playback failure, without playing optimistically or changing canonical/pending behavior · `client/client.js`, `client/sound.js`, `internal/player/http.go`

**⟶ Wait for Wave 2 to finish, then:**

### Regression and validation

**Wave 3 — complete reopened interaction and asset coverage:**

- [x] **T150** [US2] ⚠️ Reopened (reopened — BUG-007): Complete reopened T029, T030, T034, T037, and T132; retain hover/cursor sounds, assert bad/good HTTP assets and runtime playback are observable, give each tested active and observer document its own enabling gesture, verify exact-once cues across separate fresh wrong/unlock/lockout puzzles, and prove initial/reconnect snapshots, repeated render, rejected actions, stale/duplicate revisions, `ACTION_RESULT` alone, ineligible audio, and playback failures cause no replay, canonical mutation, or pending-input stall · `client/client.js`, `client/sound.js`, `internal/player/http_test.go`, `internal/platform/assets_test.go`, `tests/browser/hacking-camouflage.spec.mjs`, `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — complete BUG-006 verification:**

- [x] **T151** [US2] ⚠️ Reopened (reopened — BUG-007): Complete reopened T133 by running formatting, vet, full and race Go suites, frontend build, complete browser suite, three packaged native active/observer audio cases using fresh puzzles and a separate enabling gesture in every tested document, and package build; append BUG-006 and SC-032 evidence without weakening SC-007, SC-014, SC-022, SC-028, or prior bugfix gates · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: BUG-006 is complete only when three fresh packaged puzzle cases invoke exactly one bad, good, and bad cue at the production Web Audio boundary for the corresponding newly applied authoritative wrong, unlock, and lockout transitions in every explicitly audio-enabled tested document; all replay/no-op paths remain silent; and ineligible or failed playback cannot affect canonical state or pending completion.

---

## Phase 20: BUG-007 — Restore Complete Hacking-Screen Audio (User Story 2)

**Goal**: In an explicitly audio-enabled packaged player document, the password-hacking screen audibly retains individual-target preview, grouped-target preview, entry, wrong/lockout, and unlock cues in addition to the already working CRT ambience and character-scroll sounds, without changing canonical gameplay or player authority.

**Independent Test**: Use one output-capable production browser served by the packaged player, perform a separate enabling gesture, and exercise individual filler hover, password and unused-pattern hover, target selection, ordinary wrong guess, correct unlock, and final lockout. Directly monitor audible output and native decode/routing for `single`, `multiple`, `enter`, `hack-bad`, and `hack-good`, with ambient and character-scroll as working controls; repeat outcome cases in active and observer documents to retain BUG-006 exact-once/no-replay behavior.

**Task disposition**: T029, T030, T037, T132, T133, and T148–T151 are reopened because their completed interaction, asset, browser, correction, and packaged-validation scopes relied on synthetic `AudioContext` success or assumed cursor audio was working. T034 remains completed but receives focused review because authoritative outcome detection may still be implicated. Historical T020 in `specs/hacking-game/tasks.md` remains the original complete-audio contract rather than being rewritten.

### Reproduction and failing coverage

**Wave 1 — native production-audio matrix first:**

- [x] **T152** [US2] Reproduce BUG-007 in an output-capable real browser served by the packaged player without replacing the native `AudioContext`; after one explicit enabling gesture, exercise `ambient`, `charscroll`, `single`, `multiple`, `enter`, `hack-bad`, and `hack-good`, record DOM/public-function dispatch, manifest and asset readiness, native decode, context state, source/gain/destination routing, source start, and directly monitored audible output, and identify the first failing boundary for every silent family · `client/client.js`, `client/sound.js`, `internal/player/http_test.go`, `tests/browser/hacking-camouflage.spec.mjs`, `tests/browser/player-sessions-control.spec.mjs`, `specs/004-player-sessions-control/validation.md`

**⟶ Wait for Wave 1 to finish, then:**

### Correction

**Wave 2 — repair only the reproduced production boundary:**

- [x] **T153** [US2] Trace T152 across target-preview deduplication, selection dispatch, authoritative revision comparison, sound-folder readiness, asset selection/fetch, native decoding, per-document eligibility/resume timing, and source/gain/destination routing; implement the smallest confirmed correction for all affected families while preserving ambient/charscroll controls, observer local-only behavior, exact-once outcome guards, canonical state, and pending completion · `client/client.js`, `client/sound.js`, `internal/player/http.go`

**⟶ Wait for Wave 2 to finish, then:**

### Regression and validation

**Wave 3 — complete reopened interaction, asset, and outcome coverage:**

- [x] **T154** [US2] Complete reopened T029, T030, T037, T132, and T148–T150 and review T034 against the reproduced boundary; retain synthetic adapter tests only as unit-level diagnostics, add native-decode/output-capable coverage for `single`, `multiple`, `enter`, `hack-bad`, and `hack-good`, keep ambient/charscroll as controls, and prove observer silence on the wire, exact-once authoritative outcomes, no replay, and non-fatal audio failure · `client/client.js`, `client/sound.js`, `internal/player/http_test.go`, `internal/platform/assets_test.go`, `tests/browser/hacking-camouflage.spec.mjs`, `tests/browser/player-sessions-control.spec.mjs`

**⟶ Wait for Wave 3 to finish, then:**

**Wave 4 — complete BUG-007 verification:**

- [x] **T155** [US2] Complete reopened T133 and T151 by running formatting, vet, full and race Go suites, frontend build, complete browser suite, clean Wails package build, and a directly monitored output-capable packaged-player journey for the complete sound-family matrix; append BUG-007 and SC-033 evidence without weakening BUG-006/SC-032, player authority, canonical convergence, pending-input, or prior bugfix gates · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: BUG-007 is complete only when the packaged password-hacking screen audibly produces `single`, `multiple`, `enter`, `hack-bad`, and `hack-good` for their specified real interactions after explicit document eligibility, retains ambient and character-scroll, preserves exact-once/no-replay authoritative outcomes, and records native decode/routing plus directly monitored output rather than relying only on synthetic source starts.

## Phase 21: Convergence

- [x] T156 CRITICAL Eliminate the production terminal-state coordinator bypass by removing `SetLiveTerminal` and `ClearLiveTerminal` from the Wails-bound App and desktop facade (or moving compatibility helpers behind an unbound test-only seam), migrate remaining focused tests to `RequestTerminalActivation` and `RequestTerminalClear`, and assert that the legacy binding names are unavailable while `UpdateLiveTerminal` and exact private `ForceHackSuccess` remain per Constitution II, Constitution III, plan: Implementation Strategy 2/10, T083–T084 (contradicts) · `app.go`, `app_test.go`, `frontend/src/desktop-api.js`, `internal/platform/assets_test.go`
- [x] T157 Run `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`, `npm --prefix frontend run build`, and `npm --prefix tests/browser test`; run the affected `wails dev` boundary smoke and a clean `wails build`; record all post-T156 results and unavailable release-only checks in `specs/004-player-sessions-control/validation.md`

---

## Phase 22: BUG-008 — Make Observer Presentation Fully Read-Only (User Stories 2 and 5)

**Goal**: Only the current connected controller can change semantic terminal selection, information-page position, or hacking preview; every observer mirrors that authoritative presentation in real time and its own terminal pointer, focus, and keyboard input is inert. Reassignment transfers both gameplay and presentation authority without resetting the shared view.

**Independent Test**: Start the production-composed generated ConnectRPC player with one controller and two observers. Move menu selection by pointer and keyboard, change information pages, and preview hacking targets as the controller; verify every observer converges in revision order. Repeat those inputs in observer documents and verify zero local terminal-selection change, zero outbound mutation, and zero canonical revision. Reassign control in both orderings with a racing presentation action, then open a new tab and reconnect the former controller; verify immediate lock/unlock, unchanged presentation, and first-snapshot convergence.

**Task disposition**: T029, T030, T032, T034, T061, and T066 are reopened because their completed behavior either explicitly preserved observer-local terminal feedback or proved only outbound shared-action gating. T027, T033, T058, T060, T062, T065, and T145 remain complete but require focused regression review for authorization, ordering, role fanout, and reconnect invariants. Historical WebSocket tasks T028, T035, and T038 remain completed records; BUG-008 modifies only the active generated ConnectRPC boundary through new tasks below.

### Reproduction and failing coverage

**Wave 1 — independent tests first (different files):**

- [x] **T158** [P] [US2] Add failing live/coordinator tests for semantic `none`/menu/page/hacking-preview presentation, current-context validation, exact observer/former-controller/disconnected/stale/invalid no-mutation behavior, deterministic revalidation after navigation/content/puzzle changes, ordered fanout, replay handling, and at least 100 presentation-action-versus-reassignment interleavings · `internal/live/service_test.go`, `internal/control/service_test.go`
- [x] **T159** [P] [US2] Add failing protobuf descriptor, adapter, Connect handler, stream, and limit tests for one typed controller-presentation mutation; require recognition/request/broadcast/terminal/context preconditions, exclusive semantic variants, complete snapshot/update projection, strict rejection, generated Go/ECMAScript parity, late/new-tab/reconnect delivery, and zero legacy WebSocket dependency · `internal/player/adapter_test.go`, `internal/player/handler_test.go`, `internal/player/public_stream_test.go`, `internal/platform/assets_test.go`
- [x] **T160** [P] [US5] Add a failing production-composed browser matrix proving controller pointer/keyboard menu selection, paging, and hacking preview converge to two observers; 100 observer pointer/focus/key/control attempts cause zero local terminal-view change and zero RPC; reassignment immediately locks the former controller and unlocks the new controller from unchanged presentation; new-tab/reconnect snapshots show no divergent default-selection flash · `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/connectrpc-player.spec.mjs`, `tests/browser/fixture-server/main.go`

**⟶ Wait for Wave 1 to finish, then:**

### Generated model and contract

**Wave 2 — additive protobuf-first presentation model:**

- [x] **T161** [US2] Add detached `ControllerTerminalPresentation` runtime/public models and the additive generated protobuf/ConnectRPC mutation plus live/snapshot/update projection for semantic menu target, page ordinal, and hacking target/pattern; reserve no existing field, enforce bounded context/target values, regenerate Go and ECMAScript outputs, update the active public-player contract, and keep every value process-local · `internal/domain/model.go`, `proto/fallout/terminal/player/v1/player.proto`, `proto/fallout/terminal/player/v1/terminal.proto`, `internal/gen/fallout/terminal/player/v1/`, `frontend/client/gen/fallout/terminal/player/v1/`, `specs/005-connectrpc-protobuf-migration/contracts/public-player.md`

**⟶ Wait for T161 to finish, then:**

### Canonical ownership and revalidation

**Wave 3 — coordinator-owned presentation transition:**

- [x] **T162** [US2] Store semantic presentation in the active terminal runtime; authorize it only for the current connected controller under the coordinator order; validate context/target/page, deduplicate requests, publish one complete revisioned projection, preserve it across reassignment, include it in reconnect snapshots, and revalidate/reset it after navigation, content, terminal, pagination, hacking-generation, target, or outcome changes without affecting gameplay randomness or durable data · `internal/domain/model.go`, `internal/live/service.go`, `internal/control/service.go`

**⟶ Wait for T162 to finish, then:**

### Transport and browser integration

**Wave 4 — independent boundary and player-client work:**

- [x] **T163** [P] [US2] Implement strict generated request adaptation, Connect handler dispatch, bounded replay fingerprinting, correlated results, and revisioned stream fanout for controller presentation; reject observer/former-controller/stale/invalid requests without revision and preserve slow-stream isolation and complete snapshot recovery · `internal/player/adapter.go`, `internal/player/handler.go`, `internal/player/limits.go`, `internal/player/server.go`, `internal/player/stream.go`
- [x] **T164** [P] [US2] ⚠️ Reopened (reopened — BUG-009; reopened — BUG-010): Replace local `selIndex`, paging, and hacking-preview ownership with authoritative semantic presentation application; send pointer/keyboard presentation requests only while active/controlling, keep presentation request correlation/latest-wins handling visually non-blocking for the controller, bound high-frequency dispatch to one in-flight request plus one replaceable latest desired target, make observer handlers inert before DOM highlight/audio changes, transfer authority on role revisions without reset or stale queued intent, render snapshots before default selection, and derive applicable menu/preview cues only from accepted authoritative changes while preserving local audio eligibility and responsive layout · `frontend/client/client.js`, `frontend/client/client.css`

**⟶ Wait for Wave 4 to finish, then:**

### Regression and validation

**Wave 5 — complete reopened task scopes:**

- [x] **T165** [US5] ⚠️ Reopened (reopened — BUG-009; reopened — BUG-010): Complete reopened T029, T030, T032, T034, T061, and T066 and review T027/T033/T058/T060/T062/T065/T145; prove exact observer local inertness, current-controller actionability without a presentation-latency locked-state flash, role-swap ordering, semantic convergence, reconnect recovery, stale-target and stale-queued-intent rejection, rapid hacking-hover backpressure, exact preview audio without superseded-target replay, no optimistic transition, and no player access to private game-master operations · `internal/control/service_test.go`, `internal/player/adapter_test.go`, `internal/player/handler_test.go`, `internal/player/public_stream_test.go`, `internal/platform/assets_test.go`, `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/connectrpc-player.spec.mjs`

**⟶ Wait for T165 to finish, then:**

**Wave 6 — complete BUG-008 verification:**

- [x] **T166** [US2] ⚠️ Reopened (reopened — BUG-009): Run protobuf generation/drift/breaking checks, formatting, vet, full and race Go suites, frontend build, complete browser suite, an output-capable native controller/observer/reassignment/reconnect journey including visible presentation-latency stability, and a clean Wails build; append BUG-008/BUG-009 plus SC-034–SC-037 evidence and confirm the earlier local-highlight clauses are superseded without weakening gameplay pending, observer read-only styling, audio, responsive layout, ConnectRPC, persistence, or private-operation gates · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: BUG-008 is complete only when observers cannot change any terminal selection/page/preview locally, every accepted controller presentation movement reaches all assigned views through the generated authoritative path, reassignment transfers authority without resetting the shared view, reconnect starts from the current presentation, and all reopened and full verification gates pass.

---

## Phase 23: BUG-009 — Keep Controller Presentation Visually Responsive (User Story 2)

**Goal**: An active controller retains normal actionable terminal styling while authoritative `SetPresentation` correlation is pending; observers and genuinely blocked gameplay/navigation/command paths keep their established locked styling, and semantic selection remains authoritative and non-optimistic.

**Independent Test**: Delay generated `SetPresentation` responses in a production-composed controller/observer browser pair. Perform at least 100 pointer and keyboard menu movements while inspecting screen classes and computed cursor/opacity/filter values; verify zero controller grey/desaturated/locked flash, continued latest-wins movement eligibility, authoritative-only semantic selection, observer convergence/read-only styling, and unchanged blocking behavior for a separately delayed gameplay/navigation request.

**Task disposition**: T029, T030, T032, T164, T165, and T166 are reopened because their completed browser, asset, CSS, client-integration, regression, and visual-validation scopes accepted `pendingPresentationAction` as a blocking `.shared-input-pending` state. T034 remains complete but requires focused regression review so its authoritative completion rules and genuinely blocking gameplay/navigation pending behavior remain unchanged. No protobuf, coordinator, runtime, persistence, or private-operation task is reopened.

### Reproduction and failing coverage

**Wave 1 — reproduce the first failing client boundary:**

- [x] **T167** [US2] Add a failing production-composed Playwright and asset-contract regression that delays `SetPresentation`, exercises pointer and keyboard menu movement, and proves the active controller currently gains `.shared-input-pending`, `cursor: not-allowed`, reduced opacity, and desaturation even though a newer presentation movement remains eligible; pair it with observer and delayed navigation controls that retain their intended locked state · `tests/browser/player-sessions-control.spec.mjs`, `internal/platform/assets_test.go`

**⟶ Wait for T167 to finish, then:**

### Client correction

**Wave 2 — separate correlation from blocking presentation:**

- [x] **T168** [US2] ⚠️ Reopened (reopened — BUG-010): Keep `pendingPresentationAction` as the single in-flight correlation, add one replaceable latest desired presentation, drain it only after authoritative completion or rejection when still eligible, suppress duplicate and superseded unsent targets, preserve stale-result suppression and authoritative-only highlight application, and exclude presentation-only pending from `.shared-input-pending` and its locked cursor/opacity/saturation rules; retain observer read-only and genuinely pending gameplay/navigation/command styling unchanged · `frontend/client/client.js`, `frontend/client/client.css`

**⟶ Wait for T168 to finish, then:**

### Regression and validation

**Wave 3 — complete reopened scopes and focused review:**

- [x] **T169** [US2] ⚠️ Reopened (reopened — BUG-010): Complete reopened T029, T030, T032, T164, T165, and T168 plus focused T034 review; run repeated delayed presentation movements, a 100-target hacking-grid sweep, bounded request concurrency/count, final-target-only queued dispatch, authoritative convergence, observer inertness/read-only appearance, role/context invalidation, reconnect, preview audio without intermediate replay, and delayed gameplay/navigation pending cases with no optimistic semantic mutation or private-player capability drift · `frontend/client/client.js`, `frontend/client/client.css`, `internal/platform/assets_test.go`, `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/connectrpc-player.spec.mjs`

**⟶ Wait for T169 to finish, then:**

**Wave 4 — complete BUG-009 verification:**

- [x] **T170** [US2] ⚠️ Reopened (reopened — BUG-010): Complete reopened T166 by running formatting, vet, full and race Go suites, frontend build, complete browser suite, protocol/binding drift gates, clean Wails package, and an output-visible packaged controller/observer journey with delayed/repeated presentation movement, a rapid hacking-target sweep, preview-audio observation, and a delayed gameplay control; append BUG-009/BUG-010 and SC-037/SC-038 evidence without weakening BUG-008 ordering/convergence, observer read-only styling, gameplay pending, audio, responsive layout, persistence, or private-operation gates · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: BUG-009 is complete only when presentation-only latency never makes an active controller look locked or unavailable, newer controller movements remain eligible, semantic selection still changes only from authoritative revisions, and observer plus genuinely pending gameplay states remain visibly and behaviorally locked as specified.

---

## Phase 24: BUG-010 — Coalesce Rapid Hacking Presentation (User Story 2)

**Goal**: Rapid active-controller movement across hacking targets remains authoritative and shared without dispatching one concurrent `SetPresentation` request per transient target or replaying a stale highlight, reveal/selection animation, or preview cue after the pointer has moved on.

**Independent Test**: In a production-composed active-and-observer browser pair, hold the first generated `SetPresentation` request and sweep at least 100 distinct filler, password, and unused-pattern targets. Verify exactly one request is in flight, only the final desired target is sent after release, skipped targets produce no revision/highlight/reveal restart/audio, both views converge on the final target, and reassignment, context replacement, rejection, disconnect, and teardown discard stale queued intent.

**Task disposition**: T164, T165, T168, T169, and T170 are reopened because their completed latest-wins, browser-integration, audio, rapid-supersession, and output-visible validation scopes accepted immediate dispatch of every distinct transient hover target. T160 remains complete but requires focused review against the bounded-dispatch browser matrix. No protobuf, coordinator, runtime, persistence, CSS-only, or private-operation task is reopened.

### Reproduction and failing coverage

**Wave 1 — reproduce the first failing client boundary:**

- [x] **T171** [US2] Add a failing production-composed Playwright regression that delays the first `SetPresentation`, rapidly traverses at least 100 filler/password/pattern targets, and records concurrent request count, payload order, authoritative projections, selected-target/reveal restarts, and `single`/`multiple` cues; prove the current client sends multiple requests and replays intermediate targets after the pointer reaches its final target · `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/connectrpc-player.spec.mjs`

**⟶ Wait for T171 to fail for the reported reason, then:**

### Client backpressure correction

**Wave 2 — bounded latest-target dispatch:**

- [x] **T172** [US2] Implement one-in-flight/one-latest presentation dispatch: keep the dispatched request correlated until result plus required projection completion, replace only one queued desired semantic target during latency, skip authoritative/in-flight/queued duplicates, drain at most the newest still-applicable target after completion or rejection, and clear/revalidate queued intent on role, broadcast, terminal, context, navigation, puzzle, disconnect, or teardown changes without changing the generated RPC, coordinator ordering, authoritative-only rendering, BUG-009 styling, or observer inertness · `frontend/client/client.js`

**⟶ Wait for T172 and the focused T171 regression to pass, then:**

### Regression and reopened scopes

**Wave 3 — complete reopened tasks and focused review:**

- [x] **T173** [US2] Complete reopened T164, T165, T168, and T169 plus focused T160 review; prove bounded rapid hover for filler/password/pattern/none transitions, final-target active-and-observer convergence, exact preview audio, no stale selected-target/reveal replay, rejection recovery, reassignment/context/disconnect invalidation, reconnect snapshots, keyboard/menu/page presentation, delayed gameplay blocking, and zero optimistic or private-capability drift · `frontend/client/client.js`, `internal/platform/assets_test.go`, `tests/browser/player-sessions-control.spec.mjs`, `tests/browser/connectrpc-player.spec.mjs`

**⟶ Wait for T171–T173 to pass, then:**

### Complete BUG-010 verification

**Wave 4 — full validation and evidence:**

- [x] **T174** [US2] Complete reopened T170 by running formatting, vet, full and race Go suites, frontend build, complete browser suite, protocol/binding drift gates, clean Wails package, and an output-visible packaged controller/observer rapid hacking-hover journey; append BUG-010 and SC-038 evidence including one-in-flight/one-final-follow-up counts and zero stale highlight/reveal/audio replay without weakening BUG-008/BUG-009, gameplay pending, observer styling, audio, responsive layout, persistence, or private-operation gates · `specs/004-player-sessions-control/validation.md`

**Checkpoint**: BUG-010 is complete only when rapid hacking hover keeps at most one presentation request in flight and one replaceable latest target, superseded unsent targets create no revision/render/reveal/audio effect, the final still-applicable target converges authoritatively on active and observer views, stale queued intent cannot survive authority/context changes, and all reopened and full verification gates pass.
