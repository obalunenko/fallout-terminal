# Trust, Runtime, and Evidence Boundaries

## Application dependency boundaries

```text
Overseer Vue components/composables
        ↓ typed DesktopPort only
frontend/overseer/src/adapters/desktop-api.ts
        ↓ generated-service alias + @wailsio/runtime
generator-owned Wails bindings / named events
        ↓ private Wails transport
existing Go adapters, validation, authorization, persistence, services

Player Vue components/composables
        ↓ typed Player transport/projection ports only
frontend/client/src/adapters/player-rpc.ts
        ↓ generated public TypeScript + ConnectRPC
existing public ConnectRPC handlers
        ↓ existing Go validation/authorization/state owners
canonical server-authoritative state
```

The Player graph must have no direct or indirect path to `frontend/overseer`, `@wailsio/runtime`, Wails bindings, native capabilities, filesystem APIs, privileged types, Overseer state, or a shared cross-boundary store. The applications share only the npm install/lock boundary, exact compiler/build tooling, and capability-neutral `frontend/tsconfig.base.json` compiler policy. Authored environment, global, transport, view-state, Wails, ConnectRPC, component, and composable declarations stay inside their owning application; type-only imports do not create an allowed cross-boundary application contract.

Wave c preserves FR-045 by establishing two independently compiling Vue shells. Its Player exception is capability-neutral and exhaustive: strict participation through `frontend/tsconfig.base.json` and `frontend/client/tsconfig.json`; Player-owned declarations and ports; empty `frontend/client/src/App.vue`; `frontend/client/src/mount.ts`; isolated `frontend/client/test-fixtures/index.html` and `frontend/client/test-fixtures/candidate-main.ts`; and dependency/boundary policy fixtures. Wave c must not implement Player business behavior, render or replace production Player DOM, call ConnectRPC as application behavior, consume Wails or any privileged API, or select the candidate for production. Player feature migration begins only after the complete wave-e Overseer exit gate—including legacy deletion, temporary-mechanism absence, browser/visual parity, native/binding/resource/package evidence, and ownership sign-off—passes. Wave h remains the only atomic Player production cutover.

## Task-local evidence and RED/GREEN contract

Every implementation task has a locally executable completion check at the point it runs. That check exercises the exact production and test files created or changed by the task and cannot name a Taskfile target, checker, fixture, package script, or generated artifact introduced later. Broader integration, parity, and wave-exit tasks add evidence but cannot be the sole verification of an earlier implementation task.

A test-first RED task is complete only when it records: the expected failing assertion; the accepted failure signature demonstrating absent product behavior; rejection signatures for missing dependencies, configuration, routes, fixtures, browser startup, or other infrastructure failures; the exact RED evidence path; and the explicit later GREEN task. RED evidence is not a prerequisite that demands the test pass. The GREEN task must rerun the same exact assertion and record the passing result.

Tasks are split by one coherent UI, workflow, resource owner, or independently testable behavior. Each slice names exact production files, exact test files, an immediate local verification, matching legacy deletion where applicable, cleanup evidence, the exact ownership-record row, and temporary-mechanism impact. Integration and wave-exit gates are separate. `[P]` is permitted only for disjoint exact files with no shared generated output, lockfile, `Taskfile.yml` section, manifest, entrypoint, `specs/033-frontend-vue-typescript-migration/migration-ownership.md` row, evidence record, or visual baseline, and parallel branches join before shared integration.

## Preserved Player RPC contract

| RPC | Cardinality | Preserved client responsibility |
|---|---|---|
| `Subscribe` | unary request → server stream | Include opaque recognition handle/client instance, require complete snapshot first, cancel replaced streams, reconnect after current fixed delay, reject stale generations/revisions. |
| `SelectCharacter` | unary | Send typed request, correlate `ActionResult`, and wait for applicable authoritative revision before considering canonical state complete. |
| `Navigate` | unary | Preserve command/navigation request validation, authority, pending-state exclusion, typed rejection, and authoritative convergence. |
| `Guess` | unary | Preserve target identity, controller authority, attempts/outcomes, typed rejection, and no optimistic canonical hack mutation. |
| `ActivatePattern` | unary | Preserve pattern identity, controller authority, typed rejection, and authoritative convergence. |
| `SetPresentation` | unary | Preserve functionally equivalent fallback for unsupported/failed client streaming and context-keyed transient presentation. |
| `PresentationUplink` | client stream → unary response | Preserve capability probe, open handshake, targeted ready/results through `Subscribe`, latest-value backpressure, cancellation, generation/request correlation, timeout/retry, and fallback. |
| `SoundManifest` | unary | Preserve exact categories, same-origin asset validation, allowed filenames/formats, and optional failure isolation. |

All paths remain `/fallout.terminal.player.v1.PlayerService/<RPC>`, binary Connect encoding remains enabled, and no public health, reflection, status, administration, native, or private method is added. Existing Go request-size/origin/authentication/authorization rules and Connect error mapping remain authoritative.

## Preserved state and lifecycle rules

- Every new physical `Subscribe` stream begins with one complete personalized snapshot; deltas before a snapshot are invalid.
- Applicable revisions strictly increase. Older snapshots, updates, action results, events, update/public-access snapshots, and stale async completions cannot replace newer state.
- Go remains the canonical owner of player role, controller authority, navigation, command state, hacking outcomes, terminal transitions, sessions, persistence, public access, and updates.
- One logical Player session is coordinated across qualifying tabs; each connected tab retains its own physical stream. Storage values are untrusted and recognition handles remain opaque.
- Controller-only local presentation is transient, context-keyed, non-authoritative, bounded, and reconciled with server updates; observers remain read-only.
- Client-streaming presentation is optional. Backpressure is latest-value, cancellation is explicit, and any unsupported or failed path returns to `SetPresentation` without losing basic operation.
- Overseer command/result and named-event adapters preserve listener-before-getter ordering, exact-once release, correlation/idempotency, tuple/revision comparisons, confirmation atomicity, and stale completion suppression.
- Session and player-configuration JSON retain their exact version-1 fields, defaults, compatible unknown-field behavior, and business meaning.

## Persistence compatibility evidence

`task frontend:compatibility:check` is the production-fidelity FR-023/SC-007 evidence gate in Overseer wave e and final wave i. Its reviewed fixture set contains one current and one legacy version-1 session, one current and one legacy player configuration, compatible unknown fields in both document types, and established cross-file player-configuration references. Task generation records the exact repository fixture paths and reuses the existing version-1 Go codecs where suitable instead of defining another persistence representation.

Each fixture opens through the migrated Overseer application boundary, renders and permits an edit without changing established meaning, saves, reopens, and compares supported fields, defaults, references, compatible unknown fields, and location. Loss, silent normalization, relocation, or business-meaning change fails the gate. Browser evidence proves the migrated application-boundary journey; existing Go codec tests separately retain persistence-format authority.

## External runtime validation

Generated decoding or Wails construction proves transport structure only. Trusted application state is created only after boundary validation.

| Boundary | Required validation |
|---|---|
| `localStorage` and storage events | Key, record shape, version/expiry, string IDs, safe integers, ownership token, malformed/denied storage fallback |
| Wails method results and named events | Non-null records, exact strings/booleans/safe integers, enum allowlists, optional fields, revision/generation applicability, error defaults, deep detachment |
| ConnectRPC decoded values | Semantic identifiers, unique roster entries, revision applicability, active broadcast/terminal/context match, typed action correlation, public-only meaning |
| DOM/form/pointer/keyboard input | Actual target kind, trimmed strings, ranges, IDs, modes, filler coordinate syntax, focusability, current authoritative context |
| Clipboard | Non-empty value, supported native function, promise outcome/failure; secrets remain transient and are cleared immediately |
| Sound manifests/assets | Exact category prefix, safe single filename, approved extension, same-origin URL, optional fetch/decode/play failure isolation |
| Presentation streaming | Secure-context capability, stream generation, open/ready/result correlation, response envelope/end-stream validity, cancellation, timeout, context applicability |

`task frontend:boundary:check` makes these obligations measurable through the reviewed `tests/browser/fixtures/frontend-boundary-manifest.json`. Each entry names the boundary class, fixture identifier, owning adapter/composable, expected accept or reject result, trusted projection or no-state-change outcome, applicable migration wave, and focused test file. The gate fails if a manifest entry lacks a test mapping, rejects every explicitly defined invalid fixture before trusted state mutation, and accepts every explicitly defined valid fixture. Desktop/Wails entries belong primarily to waves c/e, Player storage/network/input entries to wave f, pointer/sound/stream entries to wave g, production Player completion to wave h, and the complete manifest to wave i. This is exhaustive only for the reviewed manifest population, never for every theoretically possible invalid value.

## Imperative Vue-owned seams and cleanup

Imperative code is limited to these narrow owners:

| Seam | Vue owner | Acquisition | Cleanup requirement |
|---|---|---|---|
| Web Audio | `useTerminalSound` | Gesture listeners, `AudioContext`, decoded buffers, `Audio`, BufferSource/Gain nodes, fetch controllers | Remove listeners; abort fetches; stop/disconnect nodes; pause/clear ambient audio; close context; suppress late promises |
| Focus | `useDialogFocus`, `useFocusRestoration`, or focused directives | Element refs, opener ref, focus key | Restore only to connected valid opener; cancel queued focus on unmount; no global descendant queries |
| Element measurement | `usePaginationMeasurement` | Element refs, `ResizeObserver`, font readiness, animation frame | Disconnect observers; cancel frames; ignore late font completion; restore any temporary content |
| CRT/typewriter timing | `useTypewriterReveal` | Timers, performance clock, reveal generation, key listeners | Clear timers; remove listeners; cancel/complete active generation once; prevent replay after unmount/context change |
| Hacking-board fitting | `useHackingBoardFit` | Owned hidden probe, measurements, observers/frames | Strip IDs/inert probe; always remove probe; disconnect/cancel; never mutate outside owned root |
| Pointer geometry | `useHackingPointer` | Owned cell refs, pointer/focus delegates, hover timer | Remove listeners; clear timers/local targets; preserve latest authoritative context |
| Presentation streaming | `usePresentationUplink` | AbortControllers, request stream/iterator, mailbox, ready/result/retry timers | Abort/cancel/return iterator; clear mailbox/timers; invalidate generation; ignore late completion; preserve unary fallback |

Subscriptions, document/window listeners, timers, animation frames, observers, audio resources, AbortControllers, streams, and temporary nodes are registered immediately with the owning Vue scope and released in `onScopeDispose`/`onUnmounted` as appropriate.

The exact creation owner, owning file and selector/root/entry/config, permitted scope, expiry wave, unconditional removal task, and executable absence check for every temporary compiler config, candidate document/entry, Vite selection, Playwright selection, staging/leaf root, callback bridge, legacy script tag, listener, timer, observer, stream, and audio owner is governed by `specs/033-frontend-vue-typescript-migration/migration-ownership.md` §Temporary mechanism register. No task may create an unregistered temporary mechanism.

## Browser versus native evidence

The typed fake `DesktopPort` is permanent test-only browser evidence infrastructure outside production source and bundles. It remains after the temporary candidate/test entrypoint is deleted in wave e, is never embedded or packaged, is checked against the production `DesktopPort` contract, and cannot be cited as Wails/native evidence.

| Evidence class | Producer | What it proves | What it must not claim |
|---|---|---|---|
| Browser functional/visual | Existing `tests/browser/*.mjs`, unchanged selectors/snapshots, Player production bundle, Overseer production SFCs with typed fake `DesktopPort` | DOM semantics, accessibility, copy, focus, keyboard/pointer, timing, geometry, audio observations, transport fixtures, revisions, application behavior | Real Wails binding generation, native dialogs/clipboard, embedded startup, packaged resources, secure store, matching-host runtime |
| Wails binding integrity | Pinned clean generator plus `scripts/wails-bindings-check.sh` | Exact generated JS/JSDoc/declarations, 39 methods, seven events, deterministic output, capability separation | Vue DOM behavior or packaged startup |
| Native/runtime | Task/buildtool, Wails startup/resource tests, native smoke scripts | Separate embeds, binding availability, window startup, native adapter behavior, resources, native accessibility where run | Complete browser visual parity unless the browser suite also ran |
| Package content | Matching-host `task package` and governed inspection scripts | Supported archive/executable/resource inventory and identity for that target | Signing/notarization, credentials, or unexecuted UI journeys |
| Conditional external/signing | Explicit opt-in/manual workflows | Only the real provider/signing/notarization behavior actually executed | A PASS when prerequisites were absent; absent checks are `NOT RUN` |

## Final forbidden-state boundary

Scan handwritten production application source under `frontend/client` and `frontend/overseer/src` for:

- `.js` application modules;
- legacy bootstrap names and script tags;
- candidate/staging entrypoints or temporary mount switches;
- mixed/cross-owned DOM queries and mutations;
- `allowJs` and `checkJs`;
- broad `any`;
- `@ts-nocheck`;
- blanket assertions and assertion chains used to bypass checking;
- unexplained `@ts-ignore`, `@ts-expect-error`, or lint/compiler suppressions.

Applicable scans exclude `frontend/overseer/bindings/**`, dependency directories, Vite/build/package output, and `tests/browser/*.mjs`. Exclusions must be path-exact and tested with positive and negative self-test fixtures so a forbidden authored file cannot hide behind a generated filename.
