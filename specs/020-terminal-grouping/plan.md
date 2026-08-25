# Implementation Plan: Terminal Grouping

## Summary

Add durable, ordered terminal groups as the only high-level representation of terminals and give the Overseer create, rename, dissolve, reorder, merge/split, and terminal-move controls. Legacy and newly created terminals normalize into singleton groups, while destructive group edits use a structured impact dialog and one private compare-and-replace guarded by both session and coordination revisions. The session catalog enforces same-group navigation, and the coordinator seeds the backward route from members preceding the first active terminal so a middle-start broadcast can traverse the whole group with the existing approval lifecycle.

**Bugfix**: 2026-08-25 — BUG-001 Updated from bugfix patch using `bugs/assets/BUG-001-terminal-list-ux-mockup.png` as the visual implementation reference.

**Bugfix**: 2026-08-25 — BUG-002 Updated from bugfix patch and reconciled with the traced production and integration boundaries.

## Project Structure

```text
proto/fallout/terminal/
├── persistence/v1/session.proto          # TerminalGroup and Session.terminal_groups
└── private/v1/desktop.proto              # revisioned replace-groups request/result

internal/
├── domain/
│   ├── model.go                          # durable groups, candidate values, runtime route metadata
│   ├── json.go                           # JSON-v1 presence detection and unknown-field preservation
│   ├── validate.go                       # exact-one membership, names, order, and link validation
│   ├── model_test.go                     # JSON/clone and runtime metadata coverage
│   └── validate_test.go                  # canonical, legacy, and strict candidate matrices
├── session/
│   ├── contract.go                       # persistence protobuf adapters
│   ├── contract_test.go                  # protobuf field and round-trip assertions
│   ├── service.go                        # normalization, revisioned group replace, group-aware catalog
│   └── service_test.go                   # generic-save guards and compare-and-replace coverage
└── control/
    ├── service.go                        # runtime revision guard, initial route prefix, approval revalidation
    └── service_test.go                   # group races, middle starts, approval and route coverage

main.go                                   # control-to-session TerminalGroupStore adapter
app.go                                    # trusted application command and result publication
app_contract.go                           # private protobuf boundary adapters
desktop_service.go                        # narrow Wails service method
app_test.go                               # orchestration, revision, and publication coverage
app_contract_test.go                      # descriptor/adapter and public-leak checks
desktop_service_test.go                   # generated desktop method routing

frontend/overseer/src/
├── index.html                            # high-level group manager and impact-confirmation dialog
├── overseer.js                           # CRUD/move drafts, diff summary, filtered destinations, refresh
├── overseer.css                          # group tree, controls, confirmation, and validation states
└── desktop-api.js                        # normalized revisioned group command/result

frontend/overseer/bindings/               # regenerated Wails bindings; never hand-edited
internal/gen/                             # regenerated Go protobuf contracts; never hand-edited
frontend/client/gen/                      # regenerated ECMAScript contracts; never hand-edited

tests/browser/
├── fixture-server/main.go                # grouped navigation and authoring fixture
├── terminal-grouping.spec.mjs            # CRUD, confirmation, compatibility, middle-start journeys
├── terminal-navigation.spec.mjs          # approval and route regressions
└── desktop-api.spec.mjs                  # session/group payload detachment and normalization

sessions/demo.json                        # explicit group for the authored demo transition
README.md                                 # grouping and middle-start behavior
```

**Structure Decision**: Keep canonical group data and normalization in `internal/domain`/`internal/session`, runtime compare-and-commit guards in `internal/control`, private orchestration in the existing root desktop boundary, and confirmation presentation in the Overseer frontend without creating a second navigation or persistence service.

## Constitution Check

| Principle | Before Research | After Design | Assessment |
|---|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | PASS | Domain, session, and control remain Wails-independent; only the existing narrow desktop service exposes group authoring. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | PASS | Durable groups and the revisioned private request/result are defined in versioned protobuf schemas before adapters or UI code. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | PASS | Go derives group diffs, owns both revision checks, eligibility, seeded route state, approval, and public projections; player unary actions and the server stream remain unchanged. |
| IV. Separate Public and Private Capabilities | PASS | PASS | Group management is private and Overseer-only; public players receive only the existing terminal-navigation projection. |
| V. Evolve Schemas Safely and Reproducibly | PASS | PASS | New fields use unused numbers, generated files are regenerated by pinned tools, and Buf/generation/breaking checks are required. |
| VI. Preserve Portable Session JSON Version 1 | PASS | PASS | `terminalGroups` is additive; legacy absence normalizes in memory without an open-time rewrite, unknown fields survive, and runtime/confirmation state is not persisted. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | PASS | One canonical group mutation path is introduced; generic saves cannot become a parallel unguarded group writer. |

No constitution violation or complexity exception is required.

## Phase 0: Research

The design decisions and rejected alternatives are recorded in [research.md](./research.md). The load-bearing conclusions are to normalize legacy terminals into singleton groups, interpret group deletion as content-preserving dissolution, use one two-revision compare-and-replace after a structured confirmation, preserve the existing public navigation protocol, and seed only the fresh broadcast's initial backward prefix.

### BUG-002 Edge-Case Tracking

A legacy transition whose endpoints normalize into separate singleton groups is an existing dormant link, not a conflict introduced by a later repair candidate. Moving the target into the source terminal's existing singleton group must carry one complete resultant group set unchanged through the Overseer draft, desktop facade, private protobuf route, coordinator, and session compare-and-replace. Authored-link validation must build membership from that resultant set. Diagnostic coverage must trace every production boundary; if the production path already preserves the candidate, it must record that result and correct the first stale integration projection found so later eligibility uses the accepted canonical session.

## Phase 1: Design and Contracts

The complete entities, validation rules, and state transitions are defined in [data-model.md](./data-model.md). Contract impact is split into:

- [session-v1.md](./contracts/session-v1.md) for persistence protobuf and portable JSON compatibility;
- [private-overseer.md](./contracts/private-overseer.md) for the trusted revisioned whole-set mutation and confirmation behavior;
- [public-player.md](./contracts/public-player.md) for unchanged player RPC cardinality and the group-constrained approval behavior carried by existing messages.

### Implementation Sequence

1. Add `TerminalGroup` and the revisioned private request/result to protobuf schemas; regenerate contracts and prove stable existing field numbers and public/private separation.
2. Add domain cloning, normalized unique-name and exact-one membership validation, deterministic singleton normalization for legacy/new terminals, content-preserving dissolution, and complete candidate diff helpers.
3. Extend session adapters and the catalog with detached group snapshots. Preserve canonical memberships during generic saves, normalize terminal create/delete atomically, and add a synchronous expected-session-revision compare-and-replace for the coordinator.
4. Add the coordinator's group store seam, expected-coordination-revision guard, active pending/route validation, initial-route metadata, first-activation prefix seeding, forward/return eligibility checks, and approval-time revalidation.
5. Expose the private mutation through the root application contract and regenerated Wails bindings; after durability, advance/publish canonical session and coordination revisions and return both projections on success or stale rejection.
6. Replace the flat terminal list with high-level group presentation and add create/rename/dissolve/move/reorder controls. Build structured destructive impact, cancel with zero calls, disable duplicate submission, and refresh from canonical results; keep rename-only edits confirmation-free.
7. Restrict transition destination options to the edited terminal's group, update the demo and documentation, then complete unit, contract, browser, race, generation, build, and packaged acceptance gates.
8. Refine the terminal organization panel to match the BUG-001 mockup: use a wider responsive sidebar, independently collapsible group cards, readable wrapping names and member counts, and one target-specific contextual menu per group or terminal. Reuse the existing mutation handlers and confirmation dialogs, preserve selection and disclosure state across unrelated renders, and keep destructive menu entries visually separated.
9. Add the BUG-002 legacy repair matrix: normalize a no-group A to B session, move B into A's existing singleton group, assert the reviewed candidate at each private boundary, persist and reopen it without command or content loss, and prove A to B becomes same-group eligible while genuinely cross-group candidates remain rejected. Trace rather than assume a production defect; when production preserves the candidate, refresh the first stale integration projection from the accepted canonical session.

## Verification Strategy

| Surface | Required evidence |
|---|---|
| Domain and JSON | Table-driven validation for duplicate IDs/names, empty groups, duplicate/missing/multiply assigned members, exact-one coverage, deterministic singleton normalization, strict authored-link grouping, order, unknown-field preservation, and detached cloning. |
| Protobuf and adapters | Descriptor assertions for `Session.terminal_groups = 5`; protobuf-aware round trips for empty, populated, and legacy sessions; clean pinned regeneration and Buf compatibility gates. |
| Session service | Generic saves preserve memberships of existing terminals while atomically handling terminal create/delete; expected-revision group replacement is atomic, preserves command states/extras, rejects cross-group links, and returns detached canonical snapshots. |
| Coordinator | Both revision mismatches, pending/route mutation guards, fresh starts at first/middle/last members, seeded reverse prefix, C→B→A→B→C→D, exact-one approval, stale group/link/order rejection, manual activation cleanup, and reconnect behavior. |
| Private boundary and Overseer UI | Generated two-revision request/result routing; create/rename/dissolve/reorder/move; impact contents; cancel/close zero call; double-submit guard; stale canonical refresh; normalized duplicate-name feedback; no player access; independently collapsible group cards; readable names/member counts; and target-specific contextual menus with separated destructive actions. |
| Browser acceptance | Overseer plus controller and two observers; destructive accept/cancel/stale/retry journeys; dissolution into singletons; singleton-delete rejection; reconnect during pending; middle start; cross-group attempts; legacy normalization; direct activation regression; and BUG-001 layout, disclosure, focus, and menu reachability at 1280×720 and 1600×900. |
| Legacy transition repair | Domain acceptance of the resultant membership set; exact candidate preservation through session/application/private boundaries; move-target browser confirmation; empty-source removal; save/reopen command preservation; same-group eligibility; and regression rejection for candidates that truly leave authored endpoints split. |
| Repository gates | `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`, frontend clean builds, browser tests, proto format/lint/generation/breaking checks, Wails binding drift check, owned build, and package smoke when the local macOS environment is available. |
