# Research: Unified Facility Authoring

All decisions below resolve the Technical Context; no `NEEDS CLARIFICATION` remains. Product
decisions come from `spec.md` Clarifications; this file records how the code realizes them.

## Existing behavior confirmed

| Behavior | Evidence |
|---|---|
| Terminal autosave sends the whole session and is latest-wins with no base-revision check. | `internal/session/service.go:402` (`Save`), `frontend/overseer/src/overseer.js:1543` (`autosave`) |
| Terminal autosave refuses any facility or binding change ("requires the dedicated facility operation"). | `internal/session/service.go:1073` (`preserveCanonicalFacility`), `:1126` (`validateFacilityBindingsUnchanged`) |
| Facility saves are stale-checked against the durable session revision **and** the facility revision; command executions advance both, terminal autosaves advance the first. This is F-05. | `internal/session/facility.go:86–96`, `internal/control/facility.go:409` |
| A facility save rejects any pending player command that has a device action. | `internal/control/facility.go:453–466` |
| A facility save refreshes every broadcast terminal from the durable tree, so it also publishes unpublished terminal edits. Terminal edits otherwise wait for «ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ». | `internal/control/facility.go:1378` (`refreshFacilityAuthoringRuntimes`), `frontend/overseer/src/overseer.js:6809` (`btnPublish`) |
| `UpdateRuntime` keeps hacking state and revalidates navigation when a tree changes. | `internal/live/service.go:164` |
| Player precedence: name after execution < state-based name; initial text < text after execution < state-based text < diagnostic substitution; diagnostic forced visibility overrides `visibleWhen`; `availableWhen` sets availability, and an active diagnostic capability block rejects the command regardless. | `internal/live/facility.go:405–449`, `internal/control/facility.go:1062`, `internal/control/service.go:2706` |
| Whole-facility reset invalidates a pending device action; command reset clears a pending request for the reset command. | `internal/control/facility.go:713`, `internal/control/service.go:1649` |
| Durable mutations can roll back on write failure. | `internal/session/service.go:551` (`mutateCommandStates`, `rollbackOnFailure`) |
| Preview returns one hypothetical projection and the Overseer compares it with itself. This is F-08. | `app.go:2177`, `internal/control/facility.go:65`, `frontend/overseer/src/overseer.js:2686–2700` |
| A live session-state event replaces the Overseer's whole session, and `renderAll` re-renders forms. | `frontend/overseer/src/overseer.js:570–585` |
| The private desktop contract, bindings and digests are pinned by scripts. | `scripts/wails-bindings-check.sh:34–58`, `scripts/wails-v3-contract-check.sh:198–215`, `scripts/state-changing-reset-native-smoke.sh:198` |
| The facility browser fixture fakes save and preview with a simplified projection; other fixtures compose the real `control` and `live` services. | `tests/browser/fixture-server/main.go:322`, `:399`, `:2822–2827` |

## R1. One authoring save operation

**Decision**: Replace the Wails methods `SaveSession` and `SaveFacilityAuthoring` with one private
operation, `SaveAuthoring`. It routes through the coordinator (`control.Service.SaveAuthoring`) into
the session store (`session.Service.SaveAuthoring`). The request carries the complete authored
session, the expected authoring revision (R2) and a correlation ID. The store:

1. rejects with `STALE_REVISION` when the expected authoring revision differs from the current one;
2. merges canonical live values into the candidate: terminal command states (the existing logic in
   `mergeCanonicalSessionState`), device `currentStateId`, fault `currentActive`
   (`protectFacilityAuthoringCurrentValues`), terminal groups (`mergeCanonicalTerminalGroups`), and
   the facility revision;
3. validates the whole candidate with `verifySessionContract`, `validateAuthoredTransitionChanges`
   and `domain.ValidateFacilityAuthoringCandidate`;
4. writes once, synchronously, through the existing serialized writer
   (`commitFacilityCandidateLocked`), and increments the facility revision only if the facility
   changed.

The coordinator then installs the facility, refreshes every broadcast terminal (R5), applies the
pending-request rule (R4), reprojects, and publishes.

`Service.Save`, `preserveCanonicalFacility`, `validateFacilityBindingsUnchanged`, the session-level
`SaveFacilityAuthoring`, and their Wails, adapter and test code are removed (Principle VII).

**Rationale**: One server path means one validation, one revision rule and one publication rule for
every edit (FR-001, FR-007). The existing helpers already enforce the live-value protection the
clarified save needs.

**Alternatives considered**: Keeping both operations and adding an authoring revision to each. Rejected:
two save models would remain, and the binding split would still force facility-only saves. Merging
live values in the Overseer instead. Rejected by Principle III.

## R2. Authoring revision

**Decision**: Add a runtime-only `AuthoringRevision` to `session.ActiveSession`. It is set to 1 on
create, open and demo copy. It increments when an accepted `SaveAuthoring` changes the document, and
when another authored mutation changes it (`ReplaceTerminalGroups`, `AssociatePlayerConfig`). Command
executions, world actions, recoveries and resets never touch it. Results of operations that can change
it return it. The session-state event (`runtime.proto`) is unchanged, because the events it carries
never change the authoring revision.

**Rationale**: This realizes the clarification "live-state changes never reject an authoring save"
with an exact check. There is a single Overseer process, so a counter is enough, and it never enters
JSON (Principle VI).

**Alternatives considered**: Reusing the facility or session revision. Rejected: live changes advance
both, which is the F-05 cause. A content hash of the authored projection. Rejected: it needs identical
canonicalization in Go and JS for no extra safety. Persisting the counter. Rejected: Principle VI, and
the value has no meaning across runs.

## R3. Overseer save pipeline and «ПРИМЕНИТЬ СНОВА»

**Decision**: Every form and dialog commit goes through one helper,
`commitAuthoringEdit({ label, scope, mutate })`. `mutate(session)` is a pure function of the
already-captured input values. The helper:

1. applies `mutate` to the working copy;
2. records an undo entry with normalized before and after snapshots (R9);
3. enqueues a save.

One save is in flight at a time. Each edit is its own request, sent in order; later edits wait while
one is in flight, and each keeps its own undo entry. The outcome decides what happens next:

- **Success**: the working copy adopts the canonical session, plus any still-queued local edits.
- **Validation failure**: only the rejected edit reverts. The working copy is rebuilt from the
  canonical session by re-running the `mutate` of each later queued edit. The reason is shown, the
  rejected edit's undo entry is dropped, and its dialog or form reopens with the author's input.
- **`STALE_REVISION`**: the working copy adopts the returned session, and «ПРИМЕНИТЬ СНОВА» re-runs
  the retained `mutate` against it in one action.

**Rationale**: Re-running the same pure `mutate` reapplies "the same edit without retyping" (FR-007)
without a JSON diff or merge engine. Serializing saves keeps the authoring revision exact. One
request per edit means a rejection never takes earlier or later edits with it (SC-001).

**Alternatives considered**: Structural JSON patches with a three-way merge. Rejected: complex, and
only needed for a rejection that serialization makes rare. Silent automatic retry. Rejected by the
clarification.

## R4. Pending player request during an authoring save

**Decision**: When an accepted save changes the facility while a command with a device action is
pending, the coordinator recomputes `pendingFacilityAction` for that command from the new session. If
the `ActionFingerprint` and the expected source states are unchanged, it moves the pending request's
`ExpectedFacilityRevision` to the new revision. Otherwise it rejects the request with the existing
stale path and records it in the existing audit. Pending requests without a device action are
unaffected, as today.

**Rationale**: Under autosave, the current reject-always rule would turn unrelated Overseer edits into
player rejections, the inverse of the clarified "live never rejects" rule. The fingerprint already
exists for exactly this comparison.

**Alternatives considered**: Always rejecting (current behavior, now too frequent). Never rejecting.
Rejected: it is unsafe when the command's own action changed.

## R5. Every saved edit goes live

**Decision**: `SaveAuthoring` (including undo and redo saves) refreshes every broadcast terminal
runtime through `refreshFacilityAuthoringRuntimes` and `UpdateRuntime`, which keep hacking state and
navigation. The following are removed: `#btnPublish` «ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ», the settings form's
separate `updateLiveTerminal` call, and the Wails `UpdateLiveTerminal` method with its
`LiveTerminalUpdateRequest` adapter. The coordinator's `UpdateLiveTerminal` is removed too, along
with the legacy-fake fallback that calls it (`app.go:3346`); the fixture and the remaining refresh
path use `RefreshActiveTerminal`. «ПЕРЕПРИМЕНИТЬ НАСТРОЙКИ» stays, because a hack level change
still requires reactivation.

**Rationale**: FR-001c. The existing facility path already behaves this way; making it the rule
removes the leak instead of special-casing it.

**Alternatives considered**: Recorded in the spec Clarifications, where the user chose "all edits go
live".

## R6. Live changes never touch authored content in the Overseer

**Decision**: The `onSessionState` handler, and any canonical session returned by reset or recovery
operations, merges **only live values** (command states, device current states, fault activity,
facility revision, durable revision) into both the working copy and the last saved snapshot, through
`mergeLiveValues(target, source)`. Then it re-renders with unapplied input restored (R7).

**Rationale**: This is the edge case "my authoring edits are kept and live state is shown as it now
is". Replacing the session would drop queued edits.

**Alternatives considered**: Replacing the session, then re-applying queued `mutate` functions.
Rejected: it is equivalent, but slower and harder to reason about.

## R7. Unapplied input survives tab switches and re-renders

**Decision**: A registry `unappliedInputs`, keyed by surface and entity, covers the terminal node
form, the terminal settings form and the device detail form. It captures field values on the first
`input` or `change` event, with a human label (the node, terminal or device name). Rendering the same
entity restores the values from the registry. Apply («ГОТОВО» or the form's apply action) and cancel
clear the entry. Switching tabs never clears it. The shared save status lists the labels. Modal
facility dialogs need no entry, because a modal dialog prevents switching tabs while it is open.

**Rationale**: FR-002 and FR-003 as clarified. Today `renderAll` silently drops this input.

**Alternatives considered**: Not re-rendering forms while they are dirty. Rejected: live changes
must still update the visible state.

## R8. One save status and one set of authoring controls on both tabs

**Decision**: Move `#saveStatus` out of the terminal sidebar into a new shared `#authoringBar` next to
the workspace tabs. The bar also holds «ОТМЕНИТЬ», «ПОВТОРИТЬ», «ПРИМЕНИТЬ СНОВА» and
«СБРОСИТЬ СЦЕНУ». The status shows, in priority order: the save error with its reason, then a save in
progress, then the names of unapplied input, then the last saved time. Save wording disappears from
`#facilityStatus`.

**Rationale**: FR-003 and SC-002 are guaranteed by construction, because there is one element.

**Alternatives considered**: A mirrored status on each tab. Rejected: it can drift and needs a parity
test.

## R9. Undo and redo

**Decision**: History lives in the Overseer. Each entry has a label plus normalized `before` and
`after` snapshots. Normalization strips live values and terminal groups; consecutive entries share
snapshot objects. The depth is 20, as FR-001a requires.

- **Undo** of the newest entry is allowed only when the current normalized authored state equals
  `entry.after`. Otherwise the control says «СЕССИЯ ИЗМЕНИЛАСЬ ПОСЛЕ ЭТОЙ ПРАВКИ». When allowed, it
  commits `entry.before` through the same save pipeline (R3); the server re-applies current live
  values, so live state is never reversed. The entry then moves to the redo stack.
- **Redo** is symmetric.
- A new edit clears the redo stack. Opening, creating or copying a session clears both stacks.
- A server rejection of an undo or redo leaves the stacks unchanged and puts the server's reason on
  the control (for example, a deleted state that is currently active).

**Rationale**: For a single linear writer, snapshot undo is exact. The equality precondition stops
the history from ever skipping an edit silently.

**Alternatives considered**: Server-side history. Rejected: the history is a presentation concern
and is cleared on open. Inverse patches. Rejected: snapshots are simpler, and demo sessions are about
38 KiB.

## R10. What players see, explained by the server

**Decision**: Add a pure function to `internal/live` that builds a presentation explanation for every
terminal of a session from the same helpers `applyFacilityProjection` uses
(`facilityEqualityMatches`, `matchingFacilityText`, `diagnosticProjection`). For every node it
returns:

- the name, visibility and availability players see now;
- for each of name, visibility and availability: the ordered rules, each with its kind, precedence,
  condition, value, `conditionHolds` and `playersSee`;
- the same rule list for the text of each block.

`applyFacilityProjection` is refactored to call the same per-target resolvers, so projection and
explanation cannot diverge. A new read-only Wails method, `DescribePresentation`, returns the
explanation for the canonical session. It does not change revisions.

**Rationale**: SC-004 requires that the marked winner matches what players see. Sharing the resolver
code makes that structural, and Go tests can assert it. The Overseer only renders.

**Alternatives considered**: Re-implementing precedence in `overseer.js`. Rejected: drift risk.
Embedding the explanation in every save result. Rejected: it inflates unrelated contracts.

## R11. Command form lists and tree names

**Decision**: The command form shows two lists:

- **«ПРИ ОДОБРЕНИИ»** (FR-008): device transitions or the recovery program, the name after execution,
  and the rewritten block with its new text. It is built from the authored command in the Overseer.
- **«ЧТО ВИДЯТ ИГРОКИ»** (FR-009, FR-010): the explanation rules for the command's name, visibility
  and availability and for each block the command rewrites. Rules are grouped by target and ordered
  by precedence. The current winner is marked «ВИДЯТ ИГРОКИ»; a higher-precedence rule shows
  «ПЕРЕКРЫВАЕТ …» while its condition holds. Each rule opens its editor: binding rules open the same
  binding dialog that ОБЪЕКТЫ uses. Because the data lives on the node, an edit made in either place
  is the same edit (FR-013).

The terminal tree renders each node's `playerName` and marks «СКРЫТО ОТ ИГРОКОВ» and «НЕДОСТУПНО»
(FR-011). The authored name stays in the form. The Overseer fetches the explanation after open, after
each accepted save, reset or recovery, and after live events. The newest response, by revision, wins.

**Rationale**: Effects are authored facts and need no server round trip. Rules need parity, so they
come from R10. Reusing the binding dialog avoids a second editor.

**Alternatives considered**: Inline rule editing inside the list. Deferred to features 040 and 041
(editor redesign is out of scope).

## R12. Scene reset

**Decision**: Add a new private operation, `ResetScene`, which takes the expected session revision,
the expected facility revision and a correlation ID. The session store builds one candidate: every
device at its initial state, every fault at its initial activity (the existing whole-facility reset
logic), and every terminal's `commandStates` cleared. It writes the candidate once with
`rollbackOnFailure`; the authoring revision is unchanged.

The coordinator installs the new facility only after a durable success. For each broadcast terminal
it then clears reset command presentation (`clearResetCommandPresentation` with terminal-wide
scope), replaces command states, and reprojects. It invalidates a pending device action
(`invalidatePendingFacilityAction`) and clears a pending request for a reset command; any other
pending request stays pending. It never touches the active terminal, the return route or hacking.

The operation emits one audit record with one correlation ID, the prior and resulting session and
facility revisions, and counts. The Overseer computes the confirmation counts from its session
(changed devices, changed faults, executed commands per terminal, pending request). The expected
revisions guarantee that the counts are still true when the reset applies. A stale result reopens
the confirmation with fresh counts.

The Wails `ResetFacility` and `ResetTerminalCommandStates` methods and all their layers are removed.
`ResetFacilityDevice` and `ResetCommandState` remain (FR-015).

**Rationale**: FR-014 needs one write and one revision, so that "if it fails, nothing is reset".
Composing two existing resets would leave a half-reset scene after a failure.

**Alternatives considered**: Calling the two existing resets in sequence with compensation.
Rejected: compensation can itself fail.

## R13. Preview

**Decision**: Extend `PreviewFacility` additively:

- **Request**: add `expected_authoring_revision`. The Overseer sends it only after the save queue is
  idle. A mismatch returns `STALE_REVISION`, and the Overseer silently retries once the queue drains
  (FR-019).
- **Result**: add `current` (the projection under current values, from the same snapshot) and
  `differences`. Differences are computed in `internal/live` from per-block projections, with these
  kinds: name changed, text changed, appears, disappears, becomes available, becomes unavailable.
  Each carries the node, an optional block, and the before and after values.

The Overseer:

- preselects the first authored state different from the current one, or the opposite activity for a
  fault;
- shows «НЕЧЕГО СРАВНИВАТЬ» without a request for a one-state device;
- renders both projections side by side, with a summary counted by kind;
- marks each differing tree item with the same words as the summary.

Preview stays read-only (FR-020).

**Rationale**: Computing the diff in Go keeps it testable and identical for the fixture and the app.
Additive fields keep the existing preview tests valid.

**Alternatives considered**: Making two preview calls and diffing in JS. Rejected: two revisions
could differ, and the diff would be duplicated logic.

## R14. Dialog vocabulary and explained controls

**Decision**:

- Every facility dialog's submit becomes «ГОТОВО» and cancel stays «ОТМЕНА».
- The device detail form's apply becomes «ГОТОВО».
- `#btnSaveFacility`, `#btnDiscardFacility` and `#btnResetFacility`, the guide actions «СОХРАНИТЬ
  ЧЕРНОВИК СЦЕНАРИЯ» and «СОХРАНИТЬ ЧЕРНОВИК И ОТКРЫТЬ ТЕРМИНАЛЫ» (the latter becomes «ОТКРЫТЬ
  ТЕРМИНАЛЫ»), the draft explanation text, and the per-terminal «СБРОСИТЬ ВСЕ СОСТОЯНИЯ» are removed.
- The command-form draft guard (`#commandDraftGuard`) and all `facilityDraft*` state are removed.

For reset, preview and private-recovery controls, a helper `controlBlockReason(control)` returns
`null` or one reason, from this set: save in progress, read-only bundled demo, no open session,
nothing to reset, only one state, recovery not authored for the Overseer, and a coordination command
already running. A blocked control gets `aria-disabled="true"`, stays focusable, and shows the reason
in an `aria-describedby` element.

**Rationale**: FR-004, FR-005 and FR-006. `aria-disabled` keeps the reason reachable by keyboard and
screen readers, which `disabled` does not.

## R15. Protobuf evolution and cutover

**Decision**: All new messages and fields in `proto/fallout/terminal/private/v1/desktop.proto` are
additive. The superseded private messages (`SaveSessionResult`, `SaveFacilityAuthoringRequest`,
`ResetFacilityRequest`, `ResetTerminalCommandStatesRequest`, `LiveTerminalUpdateRequest`) are deleted
under a documented migration (`contracts/private-desktop.md`, "Migration"), as Principle V permits.

The private desktop contract has no out-of-process consumer: bindings are generated in the same
build. The cutover then:

- regenerates code with `task proto:generate`;
- refreshes `proto/compatibility-baseline.binpb` from the new schema;
- updates the pinned digests in `scripts/wails-v3-contract-check.sh`;
- updates the method allowlist and signatures in `scripts/wails-bindings-check.sh`;
- moves `scripts/state-changing-reset-native-smoke.sh` to `ResetCommandState` and `ResetScene`.

**Rationale**: Principle VII forbids keeping dead transports. Principle V allows a breaking change
with an explicit migration, and nothing outside the process can observe the removal.

**Alternatives considered**: Keeping the deleted messages with `deprecated = true`. Rejected: that
leaves dead generated contract surface, which Principle VII forbids. A `private.v2` package.
Rejected: it would version the whole desktop contract for five unused messages.

## R16. Tests and fixtures

**Decision**:

- **Go**: table tests in `internal/session` and `internal/control` for `SaveAuthoring` (accepted
  after live changes, rejected after authored changes, live values protected, pending rebase or
  reject, publication) and `ResetScene` (atomic rollback, revision checks, pending handling, audit).
  `internal/live` tests cover explanation and projection parity over the demo and all fixture
  sessions, and preview differences.
- **Race**: `task test:race` with authoring saves concurrent with world actions and `ResetScene`.
- **Browser**: new specs `facility-unified-save.spec.mjs` (US1), `facility-command-lists.spec.mjs`
  (US2) and `facility-preview-diff.spec.mjs` (US3). The fake facility-authoring fixture gains
  `save-authoring`, `reset-scene` and `describe` routes that call the exported `internal/live`
  functions, so preview and explanation behave as in the app. Player-parity journeys
  (SC-004, US2 scenario 4) run on the `facility-player-state` fixture with the real coordinator.
  Existing specs that use the removed labels, `#btnPublish` or draft buttons are updated in the same
  change.

**Rationale**: Each story is tested independently against the real projection code.

## R17. Demo and documentation

**Decision**: `sessions/demo.json` stays unchanged: it already has the command, entry, door and
precedence conflict that the stories use, and its starting values differ from its current values, so
«СБРОСИТЬ СЦЕНУ» is visible. `sessions/demo-capability-paths.md` gains these paths: cross-tab autosave
with undo and redo, the command lists on «Открыть выход и подать сигнал», scene reset from the shared
bar, and the preview from «Гермодверь Убежища 76». The authoring guide copy stops mentioning drafts.
`tests/browser/demo-session.spec.mjs` asserts the new inventory entries.

**Rationale**: Principle IX requires a demonstration path for each changed behavior. No new
session-driven capability is added.
