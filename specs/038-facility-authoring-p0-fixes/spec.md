# Feature Specification: Facility Authoring P0 Fixes

**Feature Branch**: `038-facility-authoring-p0-fixes`

**Created**: 2026-10-29

**Status**: Draft

**Input**: User description: "Facility authoring P0 fixes. Fix the four P0 defects found by the real-application UX review of features 035/036 (review page: https://claude.ai/artifact/7JTskVJn9xKELD4esDS78f; evidence captured in the packaged macOS app built from commit 4d5984b with a copy of sessions/demo.json): F-01 Cyrillic names produce identical stable IDs; F-02 labels of hidden fields stay visible; F-03 device actions silently appear and silently disappear; F-04 a fault cannot be authored as active at game start. Keep portable session JSON version 1 compatibility and existing authored IDs, update the demo capability-to-path inventory if an authoring path changes, cover each fix with browser tests, and branch from 035-facility-state-system because feature 035 is not yet merged into develop."

## Clarifications

### Session 2026-10-29

- Q: What happens when a command's device action exists only in the unsaved facility draft and the author changes that command's mode under ТЕРМИНАЛЫ? → A: Block the mode change and apply for that command while the facility draft has unsaved changes; explain why and offer to save or discard the draft first.
- Q: How is the keep-or-remove choice presented when a command with a device action changes mode? → A: As an inline notice in the command form directly below the mode selector, listing the affected transitions with two buttons; applying stays disabled until the author chooses, and selecting the original mode again dismisses the notice.
- Q: Which transliteration scheme turns Russian names into identifiers? → A: The practical slug scheme used in web addresses: й→y, ё→yo, ю→yu, я→ya, х→kh, ц→ts, щ→shch, ы→y, ъ and ь omitted (for example «Ядро» → yadro, «Южный шлюз» → yuzhnyy-shlyuz, «Дверь в убежище» → dver-v-ubezhishche).
- Q: Do identifier fields move into a collapsed technical section in this feature? → A: No. Identifier fields keep their current position and stay editable; only the suggested values and the row-level validation change. Collapsing identifiers and re-laying out the device dialog belong to the later editors feature (review finding F-10).

## User Scenarios & Testing *(mandatory)*

All four stories fix defects that the review classified as P0: each one either loses authored
data, blocks the guide's own example, or forces manual JSON editing. Each story is independently
deliverable and verifiable.

### User Story 1 - Change a command's mode without losing its device action (Priority: P1)

As an Overseer, when I change how a command behaves, I am told what will happen to the device
transitions attached to it and decide myself; and when I attach transitions to an ordinary command,
I write the texts players will see instead of the system inventing them.

**Why this priority**: Today a single mode change followed by **ПРИМЕНИТЬ** silently deletes the
command's device action, while the form keeps showing it. This is silent loss of authored data.

**Independent Test**: In the demo, open **ТЕРМИНАЛЫ → Терминал смотрителя — Убежище 76 → ПРОТОКОЛ
ВЫХОДА → Открыть выход и подать сигнал**, switch its mode to **ОБЫЧНАЯ КОМАНДА**, and verify that an
explicit choice is required before anything is applied. Separately, attach a device action to an
ordinary command from **ОБЪЕКТЫ** and verify that the after-execution name and confirmation text are
requested from the author.

**Acceptance Scenarios**:

1. **Given** a state-changing command with a device action, **When** I switch its mode to ordinary
   or to terminal transition, **Then** an inline notice directly below the mode selector names the
   device transitions that would be lost and offers exactly two outcomes: keep the state-changing
   mode, or remove the device action and switch.
2. **Given** that choice is pending, **When** I try to apply the form, **Then** applying is not
   possible and the form says what has to be decided first.
3. **Given** I chose to keep the state-changing mode, **When** the form updates, **Then** the
   original mode, texts, and device action are unchanged and nothing has been saved.
4. **Given** I chose to remove the device action, **When** I apply the form, **Then** the command
   is saved in the new mode without a device action, and the facility relationship lists no longer
   show that action.
5. **Given** a command in a mode that cannot carry a device action, **When** I view its form,
   **Then** no device-action block is shown for it.
6. **Given** an ordinary command, **When** I attach a device action to it, **Then** the editor asks
   me for the name after execution and the confirmation text, both empty and required, and the
   command becomes state-changing only after I confirm with both filled in.
7. **Given** the facility draft has unsaved changes, **When** I try to change the mode of a command
   whose device action is affected, **Then** the change and apply are blocked, the form explains
   that the facility draft must be saved or discarded first, and it offers both actions.

---

### User Story 2 - Create the guide's door with Russian names (Priority: P1)

As an Overseer writing in Russian, I can name devices, states, transitions, faults, and recovery
programs in Russian and save them on the first attempt, with readable technical identifiers that I
never have to type.

**Why this priority**: The guide's own example ("Дверь в убежище", "Заперта", "Открыта") cannot be
saved today; the first scene every new author builds fails with a generic error.

**Independent Test**: Create a new door device named "Дверь в убежище" with states "Заперта" and
"Открыта" and a transition "Открыть дверь", without touching any identifier field, then save the
session and reopen it.

**Acceptance Scenarios**:

1. **Given** a new device, **When** I type "Дверь в убежище", **Then** its suggested identifier is
   `device-dver-v-ubezhishche`, and states named "Заперта" and "Открыта" get `zaperta` and
   `otkryta`.
2. **Given** two rows whose different names produce the same identifier (for example «Открыта» and
   «Открыта!»), **When** the identifiers are suggested, **Then** each one is unique within its scope,
   with a numeric suffix added to the later one (`otkryta` and `otkryta-2`).
3. **Given** a name with no letters or digits that can be represented, **When** an identifier is
   suggested, **Then** it falls back to a numbered identifier such as `state-1`.
4. **Given** an existing device, state, transition, fault, or recovery program, **When** I rename
   it, **Then** its identifier does not change.
5. **Given** a row with a missing name or a conflicting identifier, **When** I apply the dialog,
   **Then** the message appears next to that exact row, names the problem, and keyboard focus moves
   to the offending field.

---

### User Story 3 - Start a scene with a fault already active (Priority: P1)

As an Overseer, I can decide whether a fault is already present when the game starts and after a
reset, and I can see both the current and the starting activity of every fault.

**Why this priority**: Scenes that begin with a broken system are common, and the demo itself uses
them, but today they can only be produced by editing the session file by hand.

**Independent Test**: Create a fault, mark it active at game start, save, and verify that it is
active now and that both device reset and whole-facility reset return it to active. Then open an
existing demo fault and verify that its starting activity is shown and editable.

**Acceptance Scenarios**:

1. **Given** the fault dialog, **When** I create or edit a fault, **Then** it offers an explicit
   choice for game start and reset: working, or fault active.
2. **Given** a new fault marked active at game start, **When** the session is saved, **Then** the
   fault is active immediately and stays active after a device or facility reset.
3. **Given** an existing fault, **When** I change its starting activity, **Then** only the reset
   target changes; its current activity during play stays as it was.
4. **Given** the fault list or a selected fault, **When** I look at it, **Then** I can tell its
   current activity and its starting activity apart.

---

### User Story 4 - Forms show only the fields that apply (Priority: P1)

As an Overseer, every form shows a label only together with the control it describes, so I never
wonder whether a field failed to load.

**Why this priority**: Orphan labels appear in the binding and fault dialogs on the most common
authoring path and make the forms look broken.

**Independent Test**: Open every Overseer dialog and switch through every mode it offers; verify
that no visible label is left without its visible control.

**Acceptance Scenarios**:

1. **Given** the binding dialog in any of its five kinds, **When** a field does not apply, **Then**
   neither the field nor its label is visible.
2. **Given** the fault dialog in device or terminal scope, **When** a field does not apply, **Then**
   neither the field nor its label is visible.
3. **Given** any other Overseer form that hides fields, **When** a field is hidden, **Then** its
   label is hidden as well.

### Edge Cases

- A name mixes Russian and Latin letters, digits, punctuation, or emoji: the identifier keeps the
  representable letters and digits in order, separated by single hyphens.
- A name uses Cyrillic letters outside the Russian alphabet (for example і, ї, є, ґ, ў) or Latin
  letters with diacritics: they produce a readable Latin equivalent rather than being dropped.
- A very long name: the suggested identifier stays within the existing 256-character limit and
  stays unique.
- The author edits an identifier by hand: automatic suggestions for that row stop, as today.
- The device action shown in the command form exists only in the unsaved facility draft: the mode
  change and apply stay blocked until the draft is saved or discarded; nothing in the command or
  the draft changes meanwhile.
- A command whose execution is already recorded keeps its mode locked until its state is reset, as
  today; no new prompt appears for it.
- The author selects the original mode again instead of using either button: the inline notice
  disappears and nothing has changed.
- A session saved before this feature, without a starting activity for some faults, opens with
  those faults treated as working at game start, and saving it does not alter other faults.
- Applying a dialog with several invalid rows: the first invalid row in reading order receives
  focus, and every invalid row shows its own message.

## Requirements *(mandatory)*

### Functional Requirements

**Device actions and command modes (User Story 1)**

- **FR-001**: The command form MUST NOT remove a command's device action as a side effect of a mode
  change without an explicit author decision.
- **FR-002**: When a command that has a device action is switched to a mode that cannot carry one,
  the form MUST show an inline notice directly below the mode selector that lists the affected device
  transitions or recovery program by readable name and offers exactly two outcomes: keep the
  state-changing mode, or remove the device action. Selecting the original mode again MUST dismiss
  the notice without changes.
- **FR-003**: While that decision is pending, the form MUST prevent applying and MUST state what
  remains to be decided.
- **FR-004**: The device-action block MUST be shown only for command modes that support a device
  action.
- **FR-005**: Attaching a device action to an ordinary command MUST ask the author for the name
  after execution and the confirmation text, MUST require both, and MUST NOT prefill them from the
  command's original name or result text.
- **FR-006**: Choosing to keep or remove the device action MUST NOT save anything by itself; the
  existing apply and save rules decide when a change is stored.

- **FR-007**: While the facility draft has unsaved changes, the command form MUST block a mode change
  and applying for any command whose device action is present in the draft, MUST explain that the
  draft has to be saved or discarded first, and MUST offer both actions; it MUST NOT apply the
  command and leave the draft stale.

**Readable identifiers (User Story 2)**

- **FR-008**: Suggested identifiers for devices, states, transitions, faults, and recovery programs
  MUST transliterate Cyrillic letters with this fixed table and MUST reduce Latin letters with
  diacritics to their base letters: а a, б b, в v, г g, д d, е e, ё yo, ж zh, з z, и i, й y, к k,
  л l, м m, н n, о o, п p, р r, с s, т t, у u, ф f, х kh, ц ts, ч ch, ш sh, щ shch, ъ omitted,
  ы y, ь omitted, э e, ю yu, я ya; Ukrainian and Belarusian letters і i, ї yi, є ye, ґ g, ў u.
  Upper-case letters follow the same table.
- **FR-009**: Suggested identifiers MUST contain only lowercase Latin letters, digits, and single
  hyphens, MUST keep the existing type prefix rules for each entity, and MUST NOT exceed 256
  characters.
- **FR-010**: When nothing representable remains, the suggestion MUST fall back to a numbered
  identifier for that entity type.
- **FR-011**: Uniqueness MUST be checked against the final identifier values in the same scope
  (all devices; all faults; all recovery programs; states and transitions within one device), and a
  numeric suffix MUST resolve any collision.
- **FR-012**: Identifiers of entities that already exist MUST NOT change when they are renamed or
  edited.
- **FR-013**: Validation in the device, fault, and recovery-program dialogs MUST attach each message
  to the row or field it concerns, MUST name the problem there (including a state name that repeats
  another state's name of the same device, ignoring case), and MUST move keyboard focus to the first
  offending field.

**Starting activity of faults (User Story 3)**

- **FR-014**: The fault dialog MUST offer an explicit choice of the fault's activity at game start
  and after reset: working, or fault active.
- **FR-015**: For a new fault, the chosen starting activity MUST also become its current activity.
- **FR-016**: For an existing fault, changing the starting activity MUST NOT change its current
  activity.
- **FR-017**: The fault list and the selected-fault view MUST show the current activity and the
  starting activity as separate, readable values.
- **FR-018**: Device reset and whole-facility reset MUST continue to restore each affected fault to
  its starting activity.

**Visible fields (User Story 4)**

- **FR-019**: In every Overseer form, a hidden control MUST NOT leave its label, legend, or help
  text visible.
- **FR-020**: The binding dialog MUST show, for each of its five kinds, only the fields that kind
  uses; the fault dialog MUST show only the fields of the selected scope.

**Compatibility and documentation**

- **FR-021**: Portable session JSON version 1 files MUST open and save without changes to existing
  identifiers, command modes, device actions, or fault activity.
- **FR-022**: The demo capability-to-path inventory MUST describe the new authoring routes for the
  mode-change decision, the texts requested when attaching an action to an ordinary command, and
  the starting activity of faults.

### Impacted Application Surfaces *(mandatory)*

- **Composition and Wails bridge (`main.go`, `app.go`)**: Not affected — no new bound methods or
  events; existing facility save and reset operations are reused.
- **Domain and canonical state (`internal/domain/`, `internal/nav/`, `internal/hack/`,
  `internal/live/`, `internal/control/`)**: Not affected — fault starting activity, command modes,
  and device actions already exist in the model with their current validation and reset semantics.
- **Persistence (`internal/session/`, `internal/playerconfig/`, `sessions/`)**: Documentation only —
  the session JSON shape does not change; `sessions/demo-capability-paths.md` gains the new
  authoring routes. `sessions/demo.json` already contains faults that start active and needs no
  data change.
- **Player transport (`internal/player/`)**: Not affected.
- **Platform and public access (`internal/platform/`, `internal/tunnel/`)**: Not affected.
- **Application update and release identity (`internal/update/`, `internal/version/`,
  `wails_updater*.go`)**: Not affected.
- **Overseer UI (`frontend/overseer/src/`)**: Affected — identifier suggestion and row validation in
  facility dialogs, command form mode change, binding dialog for command actions, fault dialog and
  fault views, and the shared rule that hides labels of hidden controls.
- **Player UI (`frontend/client/`)**: Not affected — player-visible behavior changes only through
  authored data.
- **Tests and fixtures (`internal/**/*_test.go`, `tests/browser/`, `internal/testutil/`)**: Affected —
  browser journeys for each story; fixture sessions may gain a command with a device action in
  ordinary mode only if needed to reach a scenario.
- **Build and packaging (`go.mod`, `frontend/`, `build/`, `scripts/`)**: Not affected beyond the
  normal Overseer asset build.

### State and Contract Requirements *(include when applicable)*

- **Session/player-config compatibility**: Version 1 unchanged. A fault without a stored starting
  activity keeps the current default (working). Existing identifiers are never rewritten.
- **Wails bridge and event contract**: N/A — no new methods, events, or payloads.
- **Protobuf/ConnectRPC contract**: N/A — the starting activity of faults already exists in the
  persistence schema; no field is added or renumbered.
- **Reconnect and multi-tab behavior**: N/A — authoring-only changes; players receive authored
  results through existing publication.
- **HTTP/static contract**: N/A.
- **Runtime-state lifecycle**: Unchanged. A new fault's current activity equals its chosen starting
  activity when the session is saved; resets restore starting activity as today.

### Verification Requirements *(mandatory)*

- **Go tests**: N/A unless planning finds server-side validation that rejects a newly authorable
  combination; any such change needs tests in the owning package.
- **Race testing**: N/A — no concurrent service changes.
- **Browser tests**: Journeys under `tests/browser/` for: Russian-named door with transliterated,
  unique identifiers and row-level validation; mode change with keep/remove decisions and blocked
  apply; attaching an action to an ordinary command with required texts; fault starting activity
  for new and existing faults including resets; a sweep of every Overseer dialog and mode asserting
  that no visible label lacks a visible control.
- **Interactive verification**: In the packaged or `task dev` Overseer with a copy of
  `sessions/demo.json`, repeat each story's independent test and confirm the player view of
  "Открыть выход и подать сигнал" is unchanged when its device action is kept.
- **Packaging/release verification**: `task package` builds and launches; no other packaging change.

### Key Entities *(include if feature involves data)*

- **Facility entity identifier**: Stable technical identity of a device, state, transition, fault,
  or recovery program. Suggested once from the name when the entity is created and never rewritten
  afterwards.
- **Command**: Menu item with a mode (ordinary, state-changing, terminal transition), player texts,
  and optionally a device action that runs transitions or a recovery program after approval.
- **Device action**: The transitions or recovery program a state-changing command requests.
- **Fault (diagnostic condition)**: An obstruction scoped to a device or terminal, with a current
  activity and a starting activity used at game start and on reset.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Following the guide's example with Russian names, an author saves a door device with
  two states and one transition on the first attempt, without opening or editing any identifier
  field.
- **SC-002**: Across every Overseer dialog and every mode it offers, zero visible labels remain
  without their visible control.
- **SC-003**: In every scenario that changes a command's mode or attaches a device action, zero
  device actions or player texts are removed or created without an explicit author decision.
- **SC-004**: An author can make a scene start with a fault already active, entirely from the
  Overseer, and both the current and starting activity of every fault are readable in one view.
- **SC-005**: The demo session and existing fixture sessions open and save with byte-identical
  identifiers, command modes, device actions, and fault activities.

## Assumptions

- The transliteration table in FR-008 is the practical slug scheme chosen during clarification; the
  examples in this specification are exact outputs of that table.
- State names within one device must stay unique after trimming and ignoring case, as server validation
  already requires; the device dialog reports a duplicate name on its row before saving.
- Work continues on a branch from `035-facility-state-system`; feature 035 is not yet merged into
  `develop`, which deviates from the constitution's branching rule and is recorded in the plan.

## Out of Scope

- The remaining review findings F-05 to F-14 (single save model, single state system, manual
  transitions during play, preview, layout, editors, terminology, device list), which are planned as
  separate features.
- Renaming or migrating identifiers of existing entities.
- Moving, collapsing, or re-laying out identifier fields in facility dialogs (review finding F-10).
- Renaming «диагностическое условие» to «неисправность» in the Overseer copy (review finding F-13).
- Changing which command modes can carry a device action.
- Player UI changes.
