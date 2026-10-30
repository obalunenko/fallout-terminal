# Feature Specification: Unified Facility Authoring

**Feature Branch**: `039-unified-facility-authoring`

**Created**: 2026-10-29

**Status**: Draft

**Input**: User description: "Unified facility authoring model. Implement review findings F-05, F-06, and F-08 from the real-application UX review of the Overseer facility workspace (review page: https://claude.ai/artifact/7JTskVJn9xKELD4esDS78f), building on feature 038. F-05: terminal edits autosave while facility edits stage in a private draft that goes stale after any terminal autosave; expected one save model, one vocabulary, a shared save status on both tabs, no stale-draft failure, no unexplained disabled resets. F-06: legacy command state change and facility device states describe the same thing in two places with hidden precedence; expected one place per command listing everything it changes and what players see, visible precedence, an Overseer tree that shows what players see, and one reset for the whole scene. F-08: preview preselects the current state, reads only the saved configuration, and marks nothing; expected a different preselected state, side-by-side comparison, named and marked differences, and a preview of what the author is editing. Keep session JSON version 1 compatibility, server-authoritative state with revision checks, demo updates per Principle IX, browser tests for every story; stack on 038-facility-authoring-p0-fixes."

## Clarifications

### Session 2026-10-29

- Q: Which save model do terminal and facility authoring share? → A: Autosave every applied edit, as terminal edits already do, with an undo history that reverses recent authoring edits; there is no draft.
- Q: What happens to legacy command state change (name after execution, block after execution)? → A: Keep its data model and player behavior; present it together with device effects and display rules in the unified lists. No migration.
- Q: Does a live-state change (player action, approved command) reject an authoring save in flight? → A: No; the edit applies on top of the current live state. Only a change to the authored configuration itself rejects a save, with the reason and a one-action «Применить снова».
- Q: What else does «Сбросить сцену» reset during a live broadcast? → A: Nothing; only devices, faults, and executed commands reset. Players stay on their current terminal and keep the return route and hacking progress; the confirmation says so.
- Q: Do the command form's rule lists also cover rules that hide a command or make it unavailable? → A: Yes; state-based visibility and availability and diagnostic overrides are listed with their conditions, and the one players experience is marked under the existing precedence.
- Q: Is there a redo for edits that «Отменить» reversed? → A: Yes; «Повторить» reapplies reversed edits, newest-reversed first, each saved like any other edit, under the same depth and rules as «Отменить». A new edit clears the redo history.
- Q: What happens to unapplied form or dialog input when the Overseer switches tabs? → A: It is kept and shown again on return, and both tabs' save status names it as not yet applied.
- Q: When do players see an applied authoring edit on a live terminal? → A: At once, for terminal and facility edits alike, including undo and redo; hacking progress and navigation are kept, and «ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ» is removed.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Edit both tabs without losing work (Priority: P1)

As an Overseer, I edit terminals and the facility in any order and never lose work because of the
other tab. I always know whether my work is saved, from either tab, and the same words mean the same
action everywhere.

**Why this priority**: Today a single autosaved terminal edit makes unsaved facility work stale and
forces the author to discard it. This is the most expensive failure left in authoring, and feature
038 could only fence it off.

**Independent Test**: With a copy of the demo open, rename a device on ОБЪЕКТЫ, then edit a command on
ТЕРМИНАЛЫ, then return to ОБЪЕКТЫ. Both edits are saved, the save status reads the same on both
tabs, and no "session changed" message appears. Press «Отменить» twice: the command edit and then
the device rename are reversed, newest first. Press «Повторить» once: the device rename returns.

**Acceptance Scenarios**:

1. **Given** an applied edit on ОБЪЕКТЫ, **When** I apply an edit to a command on ТЕРМИНАЛЫ,
   **Then** both edits survive and neither tab reports a stale or discarded draft.
2. **Given** any moment of authoring, **When** I look at either tab, **Then** the same save status
   is shown: the last saved time, any save in progress, any input not yet applied by name, and the
   last save error if any.
3. **Given** any facility dialog that edits data, **When** I confirm it, **Then** the confirm action
   reads «ГОТОВО» like every other such dialog, and no other save wording exists in the workspace.
4. **Given** a reset, preview, or private recovery control, **When** it cannot run right now,
   **Then** the control itself says why; otherwise it runs.
5. **Given** a command with a device action, **When** I edit it on ТЕРМИНАЛЫ, **Then** no draft
   guard blocks the form, because no stale draft can exist.
6. **Given** a save the server rejects, **When** the rejection arrives, **Then** I see the reason
   and my edit is not silently lost.
7. **Given** several applied edits on both tabs, **When** I press «Отменить» repeatedly, **Then**
   the edits are reversed newest first, each reversal is saved like any other edit, and live state
   (device states, faults, executed commands) is not reversed.
8. **Given** reversed edits, **When** I press «Повторить», **Then** they are reapplied
   newest-reversed first and saved like any other edit; **When** I instead apply a new edit,
   **Then** «Повторить» has nothing left to reapply.

---

### User Story 2 - See everything a command changes and what players will see (Priority: P1)

As an Overseer, I open a command and see in one place everything it changes when approved and
every rule that decides what players then read, with the winning rule marked. The terminal tree
shows command names the way players see them, and one action returns the whole scene to its start.

**Why this priority**: The demo's own «Открыть выход и подать сигнал» sets a completed name
«Выход открыт» and also has a door-state name «Выход уже открыт»; players see the second, the
Overseer tree shows the first, and nothing explains why. Authors cannot predict the scene.

**Independent Test**: Open «Открыть выход и подать сигнал» in a copy of the demo. The form lists both door and
alarm transitions, the completed name, and the rewritten block 2 of «Сводка эвакуации»; it lists the
door-state name and block 3 rules and marks «Выход уже открыт» as what players see once the door is
open. Run «Сбросить сцену» and confirm that devices, faults, and executed commands of every
terminal are back at their starting values.

**Acceptance Scenarios**:

1. **Given** a state-changing command, **When** I open it, **Then** one list names every change it
   makes on approval: device transitions or recovery program, the name after execution, and any
   entry block it rewrites.
2. **Given** that command, **When** I read the second list, **Then** it names every rule that
   decides this command's name, visibility, and availability for players and the text of every
   block it rewrites, including rules authored on ОБЪЕКТЫ, each with its condition.
3. **Given** two rules that target the same name, block, visibility, or availability, **When** both
   can apply, **Then** the list marks which one players experience and which one it overrides.
4. **Given** the terminal tree, **When** a device state or an executed command changes a command's
   name for players, **Then** the tree shows that same name.
5. **Given** a scene with changed devices, active faults, and executed commands on several
   terminals, **When** I confirm «Сбросить сцену», **Then** all of them return to their starting
   values in one step, and the confirmation listed what would change.
6. **Given** a failure while resetting the scene, **When** it is reported, **Then** nothing was
   reset.

---

### User Story 3 - Preview shows the difference (Priority: P2)

As an Overseer, I open a preview and immediately see what would change for players, side by side,
for the configuration I am editing right now.

**Why this priority**: Preview is the main way to check a scene without running it, but its first
screen always compares a state with itself and it ignores the edit the author just made.

**Independent Test**: Select «Гермодверь Убежища 76», open the preview on «Терминал смотрителя —
Убежище 76». It preselects «Открыта», shows «Заперта» and «Открыта» side by side, and lists block 3
of «Сводка эвакуации», the name «Выход уже открыт», and the appearing «Наружный маршрут
подтверждён». Change a door text rule and reopen: the preview shows the new text.

**Acceptance Scenarios**:

1. **Given** a device with several states, **When** I open its preview, **Then** the first state
   different from the current one is preselected; for a fault, the opposite activity is preselected.
2. **Given** a device with one state, **When** I open its preview, **Then** the dialog says there is
   nothing to compare.
3. **Given** a preview, **When** it resolves, **Then** the current and hypothetical views stand side
   by side.
4. **Given** differences, **When** the preview resolves, **Then** a summary counts them and names
   each element whose name or text changes, that appears, that disappears, or that becomes available
   or unavailable, and the tree marks each one with the same words.
5. **Given** an edit I just applied, **When** I open the preview, **Then** it reflects that edit.
6. **Given** any preview, **When** I close it, **Then** players, live state, and revisions are
   unchanged.

### Edge Cases

- A player command is approved, or another Overseer action changes live state, while I am editing:
  my authoring edits are kept and live state is shown as it now is.
- I switch tabs while a form or dialog has unapplied input: the input is there when I return, and
  both tabs' save status names it as not yet applied until I confirm or cancel it.
- A save is rejected because the authored configuration moved on (stale revision): the reason is
  shown and «Применить снова» reapplies the same edit on top of the newer session in one action,
  without retyping it. Live-state changes alone never cause this rejection.
- «Отменить» or «Повторить» when the edit no longer applies, for example its entity was deleted by
  a later edit that was itself undone or its reference became invalid: the control says why and the
  history skips nothing silently.
- Opening another session clears the undo and redo history.
- A command uses only legacy state change, or only device transitions: both lists still describe
  it completely.
- A block is rewritten by a command and also has a state-based text variant: the list shows both and
  marks the state-based text as the one players see while its condition holds.
- A diagnostic substitution currently replaces a block: the list shows it as overriding every other
  text for that block.
- «Сбросить сцену» while a player request is pending approval: the confirmation says the pending
  request will be resolved by the existing rules, and the scene resets only after confirmation.
- «Сбросить сцену» during a live broadcast: players stay on their current terminal with their return
  route and hacking progress intact, and see the reset device states and command names there.
- A terminal-transition command: the command lists only its transition; it has no device action.
- A session saved before this feature opens and saves with unchanged player-visible behavior.

## Requirements *(mandatory)*

### Functional Requirements

**One save model (User Story 1)**

- **FR-001**: Terminal and facility authoring MUST use one save model: every applied edit is saved
  immediately, as terminal edits are today. There is no facility draft.
- **FR-001a**: One undo history MUST cover authoring edits from both tabs. «Отменить» MUST reverse
  the most recent unreversed edit as a new save with the usual revision checks, MUST keep at least
  the 20 most recent edits, MUST never reverse live state (device states, faults, executed
  commands), and MUST say on the control why an edit cannot be reversed. Opening another session
  clears the history.
- **FR-001b**: «Повторить» MUST reapply reversed edits, newest-reversed first, as new saves with the
  usual revision checks, under the same depth and live-state rules as «Отменить», and MUST say on
  the control why an edit cannot be reapplied. Applying a new edit or opening another session MUST
  clear the redo history.
- **FR-001c**: Every saved authoring edit, undo, and redo MUST reach players on live terminals at
  once, for terminal and facility edits alike, keeping their hacking progress and navigation.
  «ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ» MUST be removed.
- **FR-002**: Applying an edit on one tab MUST NOT invalidate, discard, or hide unapplied input on
  the other tab. Switching tabs MUST keep unapplied form and dialog input and show it again on
  return. Modal dialogs prevent switching tabs while open, so their input is never left behind.
- **FR-003**: Both tabs MUST show the same save status: the last saved time, any save in progress,
  each form or dialog with unapplied input by name, and the last save error with its reason.
- **FR-004**: Facility dialogs that edit data MUST confirm with «ГОТОВО» and cancel with «ОТМЕНА».
  Dialogs that confirm a live operation (reset, recovery, scene reset) keep their verb label. The
  labels
  «Применить к черновику», «Сохранить изменения сессии», «Сохранить черновик сценария», «Сохранить
  черновик и открыть терминалы», and «Отменить изменения» MUST disappear.
- **FR-005**: Reset, preview, and private-recovery controls MUST either run or state on the control
  why they cannot run now.
- **FR-006**: The command-form draft guard introduced by feature 038 MUST be removed.
- **FR-007**: Every save MUST keep server revision checks against the authored configuration.
  Live-state changes (device states, faults, executed commands) made by players, approved commands,
  or Overseer resets MUST NOT reject an authoring save; the edit MUST apply on top of the current
  live state. A save rejected because the authored configuration changed MUST show its reason and
  MUST offer «Применить снова», which reapplies the same edit in one action.

**One description of what a command does (User Story 2)**

- **FR-008**: A state-changing command's form MUST list, as one list, every change the command makes
  on approval: device transitions or recovery program, the name after execution, and any entry block
  it rewrites with the new text.
- **FR-009**: The form MUST list every rule that decides, for players, that command's name,
  visibility, and availability and the text of each block it rewrites, wherever the rule was
  authored, each with its condition.
- **FR-010**: Where several rules can set the same name, block text, visibility, or availability,
  the list MUST mark the one players experience under the stated condition, following the existing
  precedence: for names, a state-based name overrides the name after execution; for blocks, initial
  text < text after execution < state-based text < diagnostic substitution; for visibility, a
  diagnostic that forces the command visible overrides its state-based visibility; for
  availability, an active diagnostic capability block overrides state-based availability.
- **FR-011**: The Overseer terminal tree MUST show each node's name as players currently see it.
- **FR-012**: Legacy command state change MUST keep its data model and player behavior. The unified
  lists present it next to device effects and display rules; no data is migrated.
- **FR-013**: A rule edited from the command form and from ОБЪЕКТЫ MUST be the same rule: a change in
  one place appears in the other without a separate save.
- **FR-014**: One action «Сбросить сцену» MUST return every device, fault, and executed command of
  every terminal to its starting value as one step, after a confirmation that counts what will
  change; if it fails, nothing is reset. It MUST NOT change players' current terminal, the return
  route between terminals, or hacking progress, and the confirmation MUST say that these are kept.
- **FR-015**: Single-device reset and single-command reset MUST remain available. Whole-facility
  reset and per-terminal «Сбросить все состояния» MUST be replaced by «Сбросить сцену».

**Preview (User Story 3)**

- **FR-016**: Device preview MUST preselect the first state that differs from the current one; fault
  preview MUST preselect the opposite activity. A device with one state MUST show that there is
  nothing to compare.
- **FR-017**: Preview MUST show the current and hypothetical views side by side.
- **FR-018**: Preview MUST summarize the differences by kind and name each element whose name or
  text changes, that appears, that disappears, or that becomes available or unavailable, and MUST
  mark each one in the tree with the same words.
- **FR-019**: Preview MUST reflect every edit the author has applied.
- **FR-020**: Preview MUST NOT change players, live state, or revisions.

**Compatibility and demo**

- **FR-021**: Session JSON version 1 files MUST open and save without changing player-visible
  behavior.
- **FR-022**: The demo capability-to-path inventory and the demo assets MUST describe and exercise
  the save model, the command lists, «Сбросить сцену», and the new preview.

### Impacted Application Surfaces *(mandatory)*

- **Composition and Wails bridge (`main.go`, `app.go`)**: Affected — one private authoring save
  replaces the terminal and facility saves; scene reset and the player-view explanation are new
  private operations; superseded methods are removed.
- **Domain and canonical state (`internal/domain/`, `internal/nav/`, `internal/hack/`,
  `internal/live/`, `internal/control/`)**: Affected — scene reset spans facility values and command
  execution states in one revision; projection rules are reused for the Overseer tree and lists.
- **Persistence (`internal/session/`, `internal/playerconfig/`, `sessions/`)**: Affected in code,
  not in shape — `internal/session/` gains the unified save, a runtime-only authoring revision, and
  scene reset; the session JSON shape does not change (FR-012, FR-021);
  `sessions/demo-capability-paths.md` and, if a path needs it, `sessions/demo.json` change.
- **Player transport (`internal/player/`)**: Not affected — players receive the same projections.
- **Platform and public access (`internal/platform/`, `internal/tunnel/`)**: Not affected.
- **Application update and release identity (`internal/update/`, `internal/version/`,
  `wails_updater*.go`)**: Not affected.
- **Overseer UI (`frontend/overseer/src/`)**: Affected — autosave of facility edits, undo and redo
  history, shared save status, dialog vocabulary, command lists, tree names, scene reset, preview.
- **Player UI (`frontend/client/`)**: Not affected.
- **Tests and fixtures (`internal/**/*_test.go`, `tests/browser/`, `internal/testutil/`)**: Affected —
  browser journeys for each story; Go tests for new private operations; fixture server support.
- **Build and packaging (`go.mod`, `frontend/`, `build/`, `scripts/`)**: Not affected beyond normal
  builds.

### State and Contract Requirements *(include when applicable)*

- **Session/player-config compatibility**: Version 1 shape unchanged.
- **Wails bridge and event contract**: One private authoring save replaces the terminal and
  facility saves; new private scene-reset and player-view explanation operations use revision checks
  and the existing failure codes; the preview operation is extended. Authoring saves check the
  authored-configuration revision, so live-state revisions alone never make them stale. Details:
  `contracts/private-desktop.md`.
- **Protobuf/ConnectRPC contract**: The private desktop contract gains the authoring-save,
  scene-reset, explanation, and preview-difference messages and drops the superseded save and reset
  messages under a documented migration; the public player contract does not change.
- **Reconnect and multi-tab behavior**: Players reconnecting after a scene reset receive the reset
  scene through existing publication.
- **Runtime-state lifecycle**: Scene reset clears facility live values and command execution states
  together in one revision and leaves broadcast state (current terminal, return route, hacking
  progress) unchanged.

### Security and Privacy Requirements *(include when applicable)*

- New operations stay private to the Overseer; no player capability is added.
- Retained activity records for scene reset follow the existing correlation and redaction rules.

### Verification Requirements *(mandatory)*

- **Go tests**: Scene reset atomicity and revision checks; an authoring save accepted after a
  live-state change and rejected after an authored-configuration change; projection parity between
  players and the Overseer tree.
- **Race testing**: Scene reset concurrent with player requests and autosave.
- **Browser tests**: Cross-tab editing without loss, unapplied input kept across tab switches, shared
  status, and undo and redo across tabs; dialog vocabulary; command lists with precedence on the
  demo command, including visibility and availability; tree names; scene reset with players kept on
  their terminal; preview preselection, side-by-side, named differences, and preview of a
  just-applied edit.
- **Interactive verification**: Packaged app on a demo copy repeating each story's independent test.
- **Packaging/release verification**: `task package` launches and opens the demo.

### Key Entities *(include if feature involves data)*

- **Authoring edit**: One applied change from a terminal form or a facility dialog; the unit of
  saving, undo, and redo.
- **Command effect**: Something a command changes on approval: a device transition, a recovery
  program, its name after execution, or a rewritten entry block.
- **Display rule**: Something that decides what players read: initial text, text after execution,
  state-based name or text, state-based visibility or availability, diagnostic substitution,
  diagnostic forced visibility, or diagnostic capability block; has a condition and a precedence.
- **Scene**: All devices, faults, and command execution states of the session's terminals; has
  starting values that «Сбросить сцену» restores. Players' current terminal, return route, and
  hacking progress are broadcast state, not part of the scene.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a cross-tab editing session of at least ten alternating edits, zero edits are lost
  and zero stale-draft messages appear.
- **SC-002**: The save status reads identically on both tabs at every point of that session.
- **SC-003**: Exactly one confirm label («ГОТОВО») is used across all facility dialogs that edit
  data.
- **SC-004**: For every demo command, the command form names 100% of its effects and of the rules
  that decide what players see, and the marked winning rule matches what a player actually sees.
- **SC-005**: One confirmed «Сбросить сцену» returns the demo scene to its starting values with one
  action; a forced failure leaves it fully unchanged.
- **SC-006**: Every device preview opens on a state different from the current one and names each
  difference players would see.
- **SC-007**: Existing sessions keep byte-identical player-visible projections after open and save.

## Assumptions

- The precedence rules are those players already experience; this feature makes them visible rather
  than changing them.
- Single-command reset («Сбросить состояние») and single-device reset keep their current behavior.
- Undo and redo cover authoring only; they never reverse or reapply live state changes made by
  players or by approved commands.
- Players see each saved authoring edit immediately (FR-001c). Before this feature only facility
  saves did; terminal edits on the live terminal waited for «ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ».
- Work stacks on `038-facility-authoring-p0-fixes`, which stacks on `035-facility-state-system`;
  none of these are merged into `develop` yet.

## Out of Scope

- Manual transitions during play, layout and list redesign, editors, and terminology (review
  findings F-07, F-09 to F-14), planned as features 040 and 041.
- Changing the precedence rules players experience.
- Multi-user collaborative editing.
