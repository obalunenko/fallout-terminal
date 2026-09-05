# Implementation Plan: Shared Facility State System

## Summary

Add one optional, session-wide facility aggregate that keeps authored device graphs, deterministic diagnostic conditions, recovery programs, current values, and a persistent facility revision in the existing version-1 session. Extend state-changing commands so their established approval can commit the immutable command snapshot and every requested facility transition through one synchronous atomic session mutation, then rebuild authoritative terminal projections from the saved result. Add typed private authoring, preview, repair, reset, recovery, and failure contracts while exposing only resolved content, command availability, safe visual-effect flags, and the existing access-error flow to players.

## Project Structure

The feature stays within the existing modular-monolith boundaries and adds focused files inside the packages that already own each concern.

```text
.
├── app.go
├── app_contract.go
├── app_contract_test.go
├── app_test.go
├── desktop_service.go
├── main.go
├── internal/
│   ├── control/
│   │   ├── errors.go
│   │   ├── facility.go                 # new: facility transactions, results, audit facts
│   │   ├── facility_test.go            # new
│   │   ├── service.go
│   │   └── service_test.go
│   ├── domain/
│   │   ├── facility.go                 # new: facility definitions, state, evaluator inputs
│   │   ├── facility_test.go            # new
│   │   ├── json.go
│   │   ├── json_test.go
│   │   ├── model.go
│   │   ├── model_test.go
│   │   ├── validate.go
│   │   └── validate_test.go
│   ├── live/
│   │   ├── facility.go                 # new: pure effective-tree and condition projection
│   │   ├── facility_test.go            # new
│   │   ├── service.go
│   │   └── service_test.go
│   ├── nav/
│   │   ├── nav.go
│   │   └── nav_test.go
│   ├── player/
│   │   ├── adapter.go
│   │   ├── adapter_test.go
│   │   └── public_stream_test.go
│   ├── session/
│   │   ├── contract.go
│   │   ├── contract_test.go
│   │   ├── facility.go                 # new: atomic facility/world-action persistence
│   │   ├── facility_test.go            # new
│   │   ├── service.go
│   │   └── service_test.go
│   └── testutil/testdata/
│       ├── session-v1-facility.json    # new
│       └── session-v1-state-changing.json
├── proto/
│   ├── fallout/terminal/persistence/v1/session.proto
│   ├── fallout/terminal/player/v1/terminal.proto
│   ├── fallout/terminal/private/v1/coordination.proto
│   ├── fallout/terminal/private/v1/desktop.proto
│   ├── schema-revision.txt
│   └── compatibility-baseline.binpb
├── internal/gen/                       # regenerated from changed protobuf schemas
├── frontend/
│   ├── client/
│   │   ├── client.js
│   │   ├── client.css
│   │   └── gen/                        # regenerated player contract
│   └── overseer/
│       ├── bindings/                   # regenerated Wails models/methods
│       └── src/
│           ├── desktop-api.js
│           ├── index.html
│           ├── overseer.css
│           └── overseer.js
├── tests/browser/
│   ├── demo-session.spec.mjs           # new: executable demo capability inventory
│   ├── facility-authoring.spec.mjs     # new
│   ├── facility-player-state.spec.mjs  # new
│   ├── facility-diagnostics.spec.mjs   # new
│   ├── facility-lifecycle.spec.mjs     # new
│   ├── fixtures/desktop-bindings.js
│   └── fixture-server/main.go
├── sessions/
│   ├── demo.json                       # expanded canonical product showcase
│   └── demo-capability-paths.md         # capability-to-route acceptance inventory
└── scripts/
    └── wails-bindings-check.sh
```

**Structure Decision**: Keep canonical types and validation in `internal/domain`, durability in `internal/session`, transaction ordering in `internal/control`, and detached effective presentation in `internal/live`; use focused facility files inside those packages instead of creating a second state-owning service.

## Constitution Check

The design passes the pre-research gate and the same assessment remains valid after the Phase 1 data model and contracts.

| Constitutional principle | Assessment | Design evidence |
|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | Domain, session, control, navigation, and live logic remain Wails-independent; only the root desktop adapter and Overseer bindings expose trusted operations. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | Persistence, private desktop results, pending summaries, player availability, and visual effects are schema-defined and adapted explicitly. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | Player commands remain generated unary requests; Go revalidates and commits world actions, and complete server-streamed projections carry the result. |
| IV. Separate Public and Private Capabilities | PASS | Players receive resolved content and bounded presentation flags; facility graphs, resets, previews, repairs, raw failures, and master recovery remain private. |
| V. Evolve Schemas Safely and Reproducibly | PASS | Changes are additive in existing v1 packages, use enum zero values and oneofs where required, preserve field numbers, and regenerate through pinned workflows. |
| VI. Preserve Portable Session JSON Version 1 | PASS | Facility data is optional, absent data stays absent, JSON remains authoritative, unknown fields remain preserved, and existing fields and behaviors retain their shape. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | The change extends existing approval, persistence, streaming, and logging paths and introduces no temporary duplicate protocol or runtime. |
| VIII. Make Player Activity Observable to the Overseer | PASS | Every semantic player request and authoritative outcome, including facility actions and recovery, emits correlated retained evidence without exposing authored content or becoming gameplay state. |
| IX. Make Demo Sessions Complete, Diegetic, and Coherent | PASS | The version-1 demo forms one in-world scenario, covers all shipped session-driven modes through documented reachable routes, models each group as one terminal installation, and has executable route, role, compatibility, dead-end, and warning checks. |
| Dependency Rules | PASS | Package ownership and one-way control-to-session calls remain intact; generated protobuf values stay at boundaries rather than owning mutable domain state. |
| Secret and Credential Governance | PASS | Facility records contain safe stable IDs and categories; authored content, labels, secrets, and raw dependency errors are excluded from retained audit fields. |
| Go Development Tool Modules | PASS | No new dependency or unpinned tool is planned; protobuf and Wails generation continue through repository-owned tooling. |
| Testing and Quality Gates | PASS | The plan covers package tests, contract compatibility, browser journeys, race validation, generated drift, and the repository's required Go and Node workflows. |

## Implementation Phases

### Phase 1 - Schema and domain foundation

Add the optional facility aggregate, device/state/transition definitions, typed equality preconditions and bindings, diagnostic conditions, recovery programs, facility actions on state-changing commands, persistent current values, and facility revision. Implement deep clones, deterministic indexes, bounded validation, JSON-v1 unknown-field preservation, and explicit protobuf adapters before any runtime behavior consumes the model.

### Phase 2 - Atomic durable world actions

Generalize the trusted command-state mutation into one world-action store operation. Under the established session lock order, it re-resolves the command and facility graph, compares the expected facility revision, validates all transition sources and preconditions against one pre-state, builds the completed command snapshot plus simultaneous destinations and condition effects, advances the facility revision once, writes one complete document atomically, and returns a detached canonical session. Use the same candidate-and-write primitive for private recovery and single-device or whole-facility reset; no-op resets do not write or advance revisions.

### Phase 3 - Coordination, approval, and lifecycle ordering

Extend pending command execution with a safe facility-action snapshot and correlation data. Keep the coordinator lock across the one-way durable store call, install only validated returned state, invalidate stale or repeated decisions, and map typed failures onto the existing rejected player presentation. Store one shared detached facility snapshot outside broadcast-owned runtime data, hydrate or replace it on every successful session activation before player snapshots resume, and invalidate approvals when the session or facility revision changes.

### Phase 4 - Effective projection and navigation safety

Extend the live service's pure tree projection to apply legacy command snapshots, device-state bindings, and diagnostic overrides in the defined order. Project invisible nodes out, mark unavailable commands through the additive public field, derive blocked capabilities and safe visual effects, and use the same evaluator during command selection. After every facility-driven projection change, repair an invalid open entry or selection to the nearest valid authoritative navigation state while preserving Back and master recovery.

### Phase 5 - Private authoring and operations

Add a revision-aware private facility-authoring boundary that accepts one complete candidate, returns typed validation and dependency issues, and persists graph edits or reference repairs together. Build Overseer dialogs and panels for devices, states, transitions, equality bindings, conditions, recovery programs, dependency inspection, preview, individual reset, whole-facility reset, and private recovery. Draft and preview state stays local or detached until an explicit apply; canonical session and coordination events reconcile every successful mutation.

### Phase 6 - Player and diagnostic presentation

Render unavailable commands distinctly and prevent local activation, while retaining server validation. Apply bounded display-instability classes only from authoritative effect flags, cancel obsolete effects on newer revisions, and never alter content or world state from animation callbacks. Reuse the current pending, access-error, acknowledgement, pagination, reveal, controller, and observer behavior for facility-backed commands and condition-driven records.

### Phase 7 - Correlated retained diagnostics

Extend closed audit events with facility request, decision, transition, failure, recovery, and reset categories. Carry the existing request correlation plus sorted safe device IDs, action category, reset scope, failure enum, and prior/resulting facility revisions through the current retained-log sink. Keep labels, transition prose, record contents, secrets, and raw errors out, and add tests proving logs are never consulted by load or recovery.

### Phase 8 - Compatibility and verification

Exercise legacy version-1 sessions with no facility, sessions with command snapshots and EntryContent blocks, group moves, terminal transitions, hacking, roles, broadcast stop/start, application restart, simulated update handoff, session replacement, reconnect, storage failure, and high-contention duplicate/stale requests. Regenerate contracts through the pinned workflows, review every generated change, run `go fix ./...` before final formatting, then run `task vet`, `task test`, `task test:race`, the protobuf and Wails checks, frontend builds, and focused Playwright journeys under the `.nvmrc` runtime.

## Key Ordering and Failure Rules

- The coordinator owns the only process mutation transaction; the session service owns the only durable JSON replacement.
- Control calls session in one direction while holding its transaction lock; session never calls back into control.
- Approval captures intent but does not grant stale authority. The current graph, current states, command identity, and facility revision are checked again immediately before persistence.
- All requested transition sources and preconditions are evaluated against one immutable pre-state, then every destination and condition effect is installed simultaneously in the candidate.
- Durable success precedes runtime installation and publication. Durable failure leaves the coordinator clone unaccepted and returns a typed result.
- A durable commit remains canonical if a later stream or UI publication fails; reconnect and subsequent session-state delivery converge from the saved document.
- Facility definitions and current values are never cloned into terminal groups. Active and suspended terminals project from the one coordinator facility snapshot.
- Logs are append-only diagnostic evidence and cannot drive load, replay, conflict resolution, reset, or recovery.

## Validation Strategy

Package tests will prove graph validation, deterministic evaluation, deep-copy isolation, JSON-v1 compatibility, unknown-field retention, atomic write rollback, exact revision increments, one-write multi-device changes, pending invalidation, request replay, navigation repair, and redacted audits. Contract tests will verify additive protobuf descriptors, enum zero values, oneof/presence rules, explicit round trips, compatibility baselines, generated output, and the narrow Wails allowlist. Browser journeys will exercise complete authoring, approval/rejection, multi-terminal projection, deterministic faults and recovery, reset, group moves, reconnect, lifecycle restoration, accessibility, and the always-available escape path.
