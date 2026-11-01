# Research: Facility Authoring P0 Fixes

All decisions are frontend-only. The persistence schema, Go domain, Wails bridge, and player
protocol already support every required value; see "Existing behavior confirmed" at the end.

## R1 — Readable identifier suggestion (F-01, FR-008…FR-012)

**Decision**: Replace `stableFacilityID` with one pure helper in `frontend/overseer/src/overseer.js`
that (1) lower-cases with `String.prototype.toLowerCase` (locale-independent), (2) transliterates
Cyrillic letters with the fixed FR-008 table **before** any Unicode normalization, (3) applies
`normalize('NFKD')` and removes combining marks (`\p{M}`) to reduce Latin diacritics, (4) replaces
every run of characters outside `[a-z0-9]` with one hyphen and trims hyphens, (5) applies the
entity's prefix rule, (6) resolves collisions with `-2`, `-3`, … against the final values in scope,
and (7) truncates the stem so that prefix, stem, and suffix stay within 256 characters. When the
stem is empty, the fallback is `<prefix>-1`, `<prefix>-2`, …

**Rationale**: The current helper runs `NFKD` first, which would split `й` and `ё` into base letter
plus combining mark before transliteration, and it filters `[a-z0-9]` only, which empties every
Cyrillic name. Collisions are tested against `state-state` while the stored set contains already
stripped values, so duplicates are never found. Doing transliteration first and comparing the
final string fixes both defects at the root instead of special-casing states.

**Prefix rules kept from features 035/036**: devices `device-…`, faults `condition-…`, recovery
programs `program-…` keep the prefix in the stored ID; states and transitions store the stem
without prefix, and their numbered fallback is `state-1` / `transition-1`.

**Alternatives considered**:
- npm transliteration/slug packages: rejected; a 40-entry table does not justify a new runtime
  dependency under the constitution's dependency rules, and package tables differ from FR-008.
- ICAO Doc 9303 and GOST 7.79-2000 B: rejected in clarification as less readable for Russian
  authors.
- Hash-based or random IDs: rejected; authors and logs need readable identifiers.

## R2 — Row-level validation (F-01, FR-013)

**Decision**: Device-dialog validation collects problems per state and transition row, marks each
offending control with `aria-invalid="true"` and an adjacent row message linked through
`aria-describedby`, keeps the dialog-level alert as a short summary ("Исправьте отмеченные строки:
N"), and focuses the first offending control in reading order. Fault and recovery-program dialogs
are single-entity forms; their existing field-focused messages stay, with wording that names the
field.

**Rationale**: Today one generic message is shown and focus goes to the first identifier field even
when a later row is wrong. Per-row messages meet FR-013 without changing the dialog structure that
feature 041 will redesign.

**Alternatives considered**: Inline-only messages without a dialog summary (rejected: the dialog's
existing `role="alert"` region is how screen readers learn about the failure); live validation on
every keystroke (rejected: noisy while typing and outside P0 scope).

## R3 — Hidden controls never leave visible labels (F-02, FR-019, FR-020)

**Decision**: Add `[hidden] { display: none !important; }` at the top of
`frontend/overseer/src/overseer.css`. Existing component-level `[hidden]` rules remain harmless.

**Rationale**: `.field-label-inline { display: block; }` and similar author rules outrank the
user-agent `[hidden]` rule. A packaged build with exactly this line was verified in the real
application: the binding dialog (all kinds) and the fault dialog (both scopes) showed no orphan
labels. One global rule fixes every current and future form instead of patching each selector.

**Guard**: A browser test opens every Overseer dialog and switches through every mode; for every
visible `label`, `legend`, and `.field-label-inline`, it asserts that the control it describes
(`for` target, wrapped control, or next sibling control) is visible.

**Alternatives considered**: Per-selector `:not([hidden])` fixes (rejected: incomplete by
construction); toggling `style.display` in JS (rejected: duplicates state and fights the existing
`hidden` usage across the file).

## R4 — Mode change with an existing device action (F-03, FR-001…FR-004, FR-006)

**Decision**: In `renderNodeForm`, the command's effective device action is the draft action when
a facility draft exists, otherwise the saved action. When the mode selector moves away from
`ИЗМЕНЯЕТ СОСТОЯНИЕ` and an effective action exists, an inline notice below the selector lists the
action by readable summary (reusing `facilityCommandActionSummary`) with **ОСТАВИТЬ «ИЗМЕНЯЕТ
СОСТОЯНИЕ»** and **УДАЛИТЬ ДЕЙСТВИЕ И ПЕРЕКЛЮЧИТЬ**. Until one is chosen, **ПРИМЕНИТЬ** is disabled
and the notice states why. "Keep" restores the selector; selecting the original mode again also
dismisses the notice. "Remove" only records the decision; the existing apply path then saves the
command without `stateChange`. The device-action block renders only while the selector is on the
state-changing mode; for a command not yet saved in that mode, the block shows a hint that the mode
must be applied first and its add button is disabled.

**Rationale**: Matches the clarified inline presentation, keeps saving rules unchanged (FR-006),
and closes the silent-loss path at `overseer.js:6237–6290`.

**Alternatives considered**: Modal confirmation (rejected in clarification); preserving
`facilityAction` on an ordinary command (rejected: ordinary commands are not approved, so the action
could never run and server validation ties actions to state-changing commands).

## R5 — Dirty facility draft guard in the command form (F-03, FR-007)

**Decision**: When `facilityDraftDirty` is true and the command has an effective device action, the
command form disables the mode selector and **ПРИМЕНИТЬ**, and shows a notice with **СОХРАНИТЬ
ЧЕРНОВИК ОБЪЕКТОВ** (existing `saveFacilityCandidate(facilityDraft)`) and **ОТМЕНИТЬ ЧЕРНОВИК**
(the existing discard behavior, extracted from the `btnDiscardFacility` handler into one shared
function). After either succeeds, the form re-renders unlocked.

**Rationale**: Applying the command form autosaves the session and advances its revision, which
makes a dirty facility draft stale. Blocking only the affected commands follows the existing rule
that preview, reset, and private recovery require a clean draft, and leaves unification of the two
save models to feature 039.

## R6 — Attaching an action to an ordinary command (F-03, FR-005)

**Decision**: The binding dialog's command-action fieldset gains **НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ** and
**ЗАПРОС ПОДТВЕРЖДЕНИЯ**, empty with example placeholders, visible and required only when the
selected node is a command without `stateChange` in the draft. Submitting writes
`stateChange = { completedName, confirmationText, facilityAction }`. The `stateChange ||= {…}`
fallback that copied the name and result text is removed.

**Rationale**: Server validation (`internal/domain/validate.go:1482–1487`) requires both texts for a
state-changing command, which is why the fallback invented them. Asking the author satisfies the
same rule without inventing player-visible text.

## R7 — Starting activity of faults (F-04, FR-014…FR-018)

**Decision**: The fault dialog gains a radio group **В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА** with
**ИСПРАВНО** and **НЕИСПРАВНОСТЬ АКТИВНА**, bound to `initialActive`. For a new fault the draft also
sets `currentActive` to the same value. Fault rows show `СЕЙЧАС: … · В НАЧАЛЕ: …`; the selected
fault summary uses the same two values.

**Rationale**: Authoring save already sets a new condition's current activity from its initial
activity and keeps existing conditions' current activity (`internal/control/facility.go:1278–1284`;
the browser fixture applies the same protection). Device and facility resets already restore
`initialActive` (`internal/control/facility.go:826–847`). Only the authoring control is missing.

## R8 — Tests and documentation

**Decision**: Add one Playwright spec per story (`tests/browser/facility-p0-command-actions.spec.mjs`,
`facility-p0-identifiers.spec.mjs`, `facility-p0-fault-activity.spec.mjs`,
`facility-p0-visible-fields.spec.mjs`) sharing `tests/browser/helpers/overseer-forms.mjs`; update existing journeys only where FR-005 now requires texts (for example the
story-guide recovery journey if its target command is ordinary). Update
`sessions/demo-capability-paths.md` in "Prepare a story with the in-app guide" and "Inspect and
maintain existing scenes". `sessions/demo.json` needs no data change.

## Existing behavior confirmed

| Need | Where it already exists |
|---|---|
| Fault starting activity persisted | `proto/fallout/terminal/persistence/v1/session.proto:133`, `internal/domain/facility.go:191` |
| New fault current = starting activity on save | `internal/control/facility.go:1278–1284` |
| Resets restore starting activity | `internal/control/facility.go:826–847` |
| State-changing commands need both texts | `internal/domain/validate.go:1482–1487` |
| Existing tests type identifiers explicitly | `tests/browser/facility-authoring.spec.mjs:36`, `facility-authoring-ux.spec.mjs:556,730` |
