# Research: Facility Editors and Language

Each entry records a design choice this plan settles. File references point at the code the choice copies or changes.

## R1. Clearing by hand uses the 040 switch, and the recovery path goes away

**Decision**: «Смотритель вручную» is feature 040's fault switch: `runFacilityFaultSwitch(id, false)`, which calls `SetFacilityConditionActive`. The change has three parts:

- **Validation**: `validateConditions` (`internal/domain/validate.go` 862) stops requiring a clearing recovery entry. `validateRecoveryReference` keeps checking every entry that exists, so `privateOverseerAction: false` and dangling references stay invalid.
- **Removed**: the private recovery path goes end to end.
  - `RecoverFacilityCondition` in `internal/control/facility.go` 444, with `privateFacilityRecovery` and `conditionAllowsRecovery`.
  - The recovery branch of `session.Service.ApplyWorldAction`: `applyConditionRecoveryLocked`, `resolveConditionRecovery`, and the `WorldActionRequest` fields `RecoveryConditionID` and `Recovery`.
  - `App.RecoverFacilityCondition` and its payload, contract converters and desktop method.
  - `RecoverFacilityConditionRequest` in `desktop.proto`, plus the regenerated bindings.
  - `desktopAPI.recoverFacilityCondition`.
  - The «ВЫБРАННОЕ УСЛОВИЕ» panel with «ВОССТАНОВИТЬ УСЛОВИЕ», its confirmation dialog, and its block reason «ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ НЕ НАСТРОЕНО».
- **Baseline**: the compatibility baseline is regenerated.

**Rationale**: The clarification says the Overseer can always clear any fault by hand. The 040 switch already does that for any fault scope (`internal/session/facility.go` 185), and the recovery path is gated by exactly the setting that now means nothing. The only caller of the session recovery branch is control `RecoverFacilityCondition`, so it becomes dead code. Principle VII requires removing it.

The message was added in 035, which is not in `develop`, so no shipped consumer depends on it. Regenerating the baseline follows what 035 and 039 did on this stack (Principle V's "explicit, documented migration").

**Alternatives considered**:
- Keeping the recovery method and panel with new wording was rejected. That would leave two controls that clear a fault, one of them gated by a setting the author can no longer see.
- Writing `privateOverseerAction: true` silently into every new fault, to satisfy the old rule, was rejected. It keeps a meaningless value alive and still fails for an existing fault that lacks one.

## R2. «Как появляется» and «Как устраняется» are derived from transition effects

**Decision**: Both lists are computed from `transition.conditionEffects`. «Как появляется» lists `active: true` effects. «Как устраняется» lists `active: false` effects, plus every recovery program that contains such a transition, plus «Смотритель вручную».

Adding or removing an entry edits the effect on that transition. `condition.recovery[]` becomes stored data the UI never adds to. An edit that would invalidate a stored entry drops that entry in the same edit. Examples are removing the clearing effect a `transition` entry relies on, or deleting a referenced program. Entries with `privateOverseerAction` are never touched.

**Rationale**: Effects are what runs during play, so they are the truth the author needs. `recovery[]` holds a second, partial copy. In the demo, `network-isolated` lists СЕТЬ-76 but not `connect`. A `transition` entry also needs the hidden `recovery` flag, which the UI cannot set. Pruning only what an edit invalidates keeps saved files valid without rewriting data the author never touched.

**Alternatives considered**:
- Keeping `recovery[]` in sync on every add was rejected. It would require setting `transition.recovery` from the fault dialog, and it would keep two stores describing one thing (F-06's problem).
- Ignoring `recovery[]` entirely was rejected. A stale entry fails `transitionClearsCondition` on save.

## R3. One fault edit is one save

**Decision**: The fault dialog's «Готово» writes the condition and every touched transition's `conditionEffects` in one `commitFacilityEdit` `mutate`. The mutate finds each target by ID, so it replays on newer sessions. The client hides combinations the domain refuses:

- One transition that both activates and clears a fault.
- A repeated effect.

**Rationale**: It gives one «Отменить» entry and one revision check for what the author sees as one change. `commitAuthoringEdit` already replays the mutate on newer sessions, so editing other devices' transitions is safe.

**Alternatives considered**: Saving each transition edit separately was rejected. It would produce several undo entries and partial states.

## R4. A program created from a fault is its own edit

**Decision**: «+ Программа восстановления…» opens the program dialog modally over the fault dialog. The program's list is prefilled with the first transition that clears the fault in the fault dialog's current draft, or left empty. The program's «Готово» saves it as its own edit. Its «Отмена» saves nothing. Either way focus returns to the fault dialog, whose draft is untouched, and its lists are re-derived against the new session. Any change saves through `commitFacilityEdit` as today, so the program list follows the domain rules:

- One transition per device.
- No contradicting fault effects.

**Rationale**: Programs already save on their own. Native `showModal` stacking works and keeps the fault draft in the DOM. The program shows under «Как устраняется» because it contains a clearing transition (R2), so the fault needs no link to it.

**Alternatives considered**: Saving the fault first and then opening the program was rejected, because it commits a half-edited fault.

## R5. One «ГДЕ» list across terminals, filtered by rule type

**Decision**:
- **«ГДЕ»**: the separate «ТЕРМИНАЛ» select goes away. «ГДЕ» lists every element of every terminal as «<terminal> › <folder> › … › <element>». The labels are built from a walk like `entryBlockFolderCatalog` (`overseer.js` 2272), and option values index a catalog built when the dialog opens.
- **Elements per type**:
  - «Текст записи»: records only.
  - «Разрешить команду»: commands only.
  - «Название в меню»: any element except a root.
  - «Показать элемент»: any element except a root.
- **«БЛОК»**: «<n> · «<opening text>»», built from `normalizedEntryBlockPreview` (2525).
- **Tabs**: they copy `facilityDetailTabs` (3565) and its keyboard handler (5309).
- **Existing rule**: its type is fixed. The other tabs get `aria-disabled` with the reason «Чтобы сменить тип, создайте новое правило».
- **Editing a variant**: it replaces the variant in place instead of removing it and pushing a new one, so its position stays.

**Rationale**: The spec's sentence puts the terminal inside «ГДЕ». The filtered list prevents the domain errors for `availableWhen` outside commands and `visibleWhen` on a root. Replacing in place keeps the variant order stable.

**Alternatives considered**: Keeping a terminal select ahead of «ГДЕ» was rejected, because it breaks the sentence order and starts the form with the terminal again (F-11).

## R6. Rule conflicts are checked in the client before save

**Decision**: Before «Готово», the dialog checks the session against three rules:

- A name or text variant list driven by another device: «must use one controlling device».
- An existing variant for the same element, device and state: «repeats state».
- An existing `visibleWhen` or `availableWhen` on the element. Today `node.visibleWhen = equality` overwrites it without a word.

The check reads `facilityBindingCatalog`. On a hit, the dialog names the existing rule, offers «Открыть правило», and does not save.

**Rationale**: These rules live in `validateTextVariants` (`internal/domain/validate.go` 1103) and come back as English text, or as a silent overwrite for visibility and availability (FR-028). The server stays authoritative.

**Alternatives considered**: Relying on the server error was rejected, because the error is in English and names no rule.

## R7. «Действие команды» is its own dialog

**Decision**: A new `#facilityCommandActionDialog` takes over the action editor from the binding dialog:

- `appendFacilityActionRequest`, `validateFacilityActionRows` and `readFacilityActionRows` (4768-4820).
- The program select.
- «НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ» and «ЗАПРОС ПОДТВЕРЖДЕНИЯ», which show only when the command has no `stateChange` yet.

The command is fixed when the dialog opens from the command form. It is picked from a commands-only «КОМАНДА» list when the dialog opens from the «Что делают игроки» tab or the scenario guide. «ЧТО ЗАПУСТИТЬ» is a two-option segment: «ПЕРЕХОДЫ» or «ПРОГРАММА ВОССТАНОВЛЕНИЯ». A program-backed action opens this dialog with the program selected, no longer the program dialog. The command form's «ДЕЙСТВИЕ УСТРОЙСТВ» button stays enabled for every command.

**Rationale**: Today a program-backed action skips straight to the program dialog. The player texts can only be reached from «ДОБАВИТЬ КОМАНДУ» and the guide (reader findings). FR-029 needs one place with both choices.

**Alternatives considered**: Editing the action inline in the command form was rejected. The form already holds two 039 lists, and the action editor needs the program and texts too.

## R8. The device dialog keeps one draft

**Decision**:
- **Chips**: states are buttons in `#facilityDeviceStateChips`, and the starting state shows «★». Choosing a chip opens an inline state editor with the name, «СДЕЛАТЬ НАЧАЛЬНЫМ» and «УДАЛИТЬ».
- **Matrix**: `#facilityTransitionMatrix` is a `<table>` with one row per source state, one column per destination state, and unavailable diagonal cells. Each cell is a `<button>` with `data-from` and `data-to`. Choosing a cell opens `#facilityTransitionPanel` under the matrix, in the same form. Choosing a cell with several transitions lists them first.
- **Transition panel fields**:
  - The name.
  - Source and destination selects.
  - Preconditions, whose device list leaves out the transition's own device and any device already used, as the domain requires (`validate.go` ~823).
  - Read-only fault effects with «К НЕИСПРАВНОСТИ». The link is blocked with «Сначала нажмите «Готово»» while the draft has unsaved changes.
- **Identifiers**: every identifier input moves into one `<details>` «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ» at the bottom. `setFacilityFeedback` already opens enclosing `<details>` and focuses the field, so FR-007's expand-and-focus needs no new code.
- **Originals**: every row keeps `facilityOriginal` and is read with a spread, as `readFacilityDeviceDraft` does now (4158). That keeps `transition.recovery` and unknown fields.
- **Starting state**: the first state of a new device is starred. `#facilityDeviceInitialState` is removed.

**Rationale**: It reuses the existing draft reader, validation and save unchanged. Only the markup and the row builders change. A panel inside the form keeps one «Готово» and one undo entry.

**Alternatives considered**: A separate transition dialog was rejected, because it would split the draft and add a second save for one device.

## R9. Templates are a frontend table

**Decision**: `FACILITY_DEVICE_TEMPLATES` in `overseer.js` maps each of the nine non-custom kinds to its states, starting state and transitions. The names follow the demo device of the same kind (`contracts/overseer-ui.md`, Templates). Applying a template works like this:

- It adds a state when no draft state has the same trimmed, case-insensitive name, the same check as `facilityDeviceRowProblems` (4205).
- It adds a transition when no draft transition has the same name between the same pair of states.
- It sets the starting state only when the draft has none.
- It generates IDs through `suggestFacilityID`.

**Rationale**: Templates are editor suggestions and never stored (Key Entities). A table next to `CYRILLIC_TO_LATIN` is the smallest home for them.

**Alternatives considered**:
- Defining templates in Go or proto was rejected: no consumer outside the editor needs them.
- Applying a template on type change was rejected (FR-012).

## R10. State deletion checks uses in the client

**Decision**: «УДАЛИТЬ» on a state chip refuses the starting state and the current state, giving the reason. Otherwise it collects the uses:

- This device's transitions that start or end in the state.
- Outside the device:
  - Other devices' preconditions.
  - Display rules with that device and state.
  - Command actions, programs, and fault `recovery[]` entries that reference any of those transitions.

Outside uses block the deletion and are listed with their destination buttons (R13). Otherwise a confirmation names the transitions that go with the state.

**Rationale**: The domain's `facilityEntityHasInboundReference` already refuses these cases on save, in English. FR-008 needs the list before «Готово». The draft lives only in the client, so the check has to run there.

**Alternatives considered**: Calling the Go state-scoped dependency report was rejected. It sees only the saved session, not the draft.

## R11. The device list is rebuilt per render, with filters in module state

**Decision**:
- **Rows**: `facilityDeviceRow` renders a left-aligned `<button>`. It holds the name, the type from `facilityDeviceKindLabel` under it, a `.facility-state-badge` on the right with `data-changed` when `currentStateId !== initialStateId`, and a red `.facility-fault-mark` with the count. `data-current-state-id`, `data-active-faults`, `data-facility-selected` and `aria-pressed` stay. `row.title = id` is removed from `facilityListRow`, so internal IDs no longer show on hover.
- **Search and filters**: search matches the name and the current state name, no longer the ID. `#facilityDeviceFilters` holds three toggle buttons with `aria-pressed` and counts over the whole session. The filter lives in a module `let` next to `selectedFacilityDeviceID`.
- **Hidden selection**: when the filter hides the selected device, a line «Выбранное устройство скрыто фильтром» sits above the list.
- **Terminal faults**: «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ» is a `<ul id="facilityTerminalFaultList">` after `#facilityDeviceList`. Its `<li>` rows have `data-facility-kind="terminal-condition"` and two buttons: open the fault dialog, and a `role="switch"` copied from `renderFacilityPlayFaults` (3415). Results go to `#facilityTerminalFaultStatus` next to the group. Today's `#facilityPlayStatus` sits inside the hidden device editor when no device is selected.

**Rationale**: `renderAll` already rebuilds the list on every state change, which gives FR-032 for free. A different `data-facility-kind` keeps `rowByFacilityIdentity` and the tests that count device rows unambiguous. Interactive controls cannot nest inside a row `<button>`.

**Alternatives considered**: Persisting the filter was rejected. The spec says current run only, and 040 keeps the guide choice the same way.

## R12. Errors pass through one helper and one catalog

**Decision**:
- **Helper**: `renderOverseerError(sink, result, area)` replaces every `result?.error || '<fallback>'` sink, about 28 sites. It writes the Russian message and appends a `<details>` «подробности для разработчика» holding the original text.
- **Specific messages**: they come from `OVERSEER_ERROR_MESSAGES` in `overseer.js`, keyed by area and by the exact Go text. The keys cover the 13 `failure(...)` texts in `internal/playerconfig/service.go` and the `sessionFailure`/`saveFailure` texts in `internal/session/service.go`. Facility failures keep their code-keyed table, `facilityFailureMessages` (5888), reworded to the glossary.
- **General messages**: every other area has one per-area Russian message with a next step, for example «Не удалось изменить публичный доступ. Повторите попытку. Если ошибка повторится, сохраните журнал.»
- **Sync tests**: two Go tests read `frontend/overseer/src/overseer.js` and fail when a Go failure text has no catalog entry: `internal/playerconfig/messages_test.go` and `internal/session/service_messages_test.go`.
- **Validation problems**: `facilityIssueMessage` (3651) names entities by kind label and by name looked up in the session. Raw kinds and IDs go under the disclosure. The facility authoring validator returns only `invalid-configuration` (`internal/domain/facility.go` 583), so the client checks in R6, R8 and R10 catch the common cases first. The rest gets the general facility message.

**Rationale**: The Go texts stay English for logs and developers, and the UI owns the language. Keying on the text needs no contract change. The sync tests stop the two sides from drifting apart.

**Alternatives considered**:
- Translating the Go messages was rejected. It mixes languages in logs and still leaves the other areas untranslated.
- Adding error codes to every result was rejected. It means a contract change across 50+ call sites for the same outcome.

## R13. Dependency links are described in the client

**Decision**:
- **Description**: `describeFacilityDependency(dependency, session)` builds the sentence and the place from the session. Examples are «Команда запускает переход «Открыть гермодверь»» and «Терминал смотрителя — Убежище 76 › Открыть выход и подать сигнал». The index in `property` is read with a regex (`\[(\d+)\]`) to find the equality or request.
- **Transitions**: they are resolved by matching `${device.id}/${transition.id}` against `sourceId` over all devices. The ID is no longer split, because IDs may contain `/`.
- **Buttons per kind**:

  | Kind | Button |
  |---|---|
  | `transition-precondition` | «К УСТРОЙСТВУ» |
  | `recovery-reference`, `diagnostic-scope` | «К НЕИСПРАВНОСТИ» |
  | `recovery-program-transition` | «К ПРОГРАММЕ» |
  | `command-action` | «К КОМАНДЕ» |
  | `name-variant`, `entry-content-variant`, `visibility`, `availability` | «К ПРАВИЛУ ОТОБРАЖЕНИЯ» |

- **Targets**: «К УСТРОЙСТВУ» opens the device dialog with that transition's panel. «К ПРАВИЛУ ОТОБРАЖЕНИЯ» opens the rule dialog on the matching catalog entry. An `entry-content-variant` source is a block ID, resolved through `entryBlockLocation` (2256).
- **Technical path**: `facilityTechnicalDetails(parent, text, summary)` gains the summary parameter and renders the path under «подробности для разработчика».
- The Go report and the proto stay unchanged.

**Rationale**: Every fact the sentence needs is already in the session. Adding fields to the private report would only move the same lookup into Go.

**Alternatives considered**: Listing a device's own fault effects in its report was rejected. Those links now live in the fault dialog (R2), and the spec does not ask for them here.

## R14. The glossary is a document and a browser test

**Decision**: `docs/overseer-glossary.md` lists the glossary terms with their meaning and banned synonyms. `tests/browser/facility-vocabulary.spec.mjs` fetches the served `index.html`, `overseer.js` and `desktop-api.js` and fails on any banned pattern. It uses Unicode-aware boundaries (`(?<!\p{L})…(?!\p{L})`, flag `u`), because `\b` does not work on Cyrillic. The banned patterns are:

- «объект» in the singular and oblique forms.
- «диагностическ… услови…».
- A standalone «услови…», which lets «предусловие» pass.
- «Восстановить условие».
- «Восстановление оператором».

Two strings are allowed: the tab «ОБЪЕКТЫ» and the heading «ОБЪЕКТЫ И СОСТОЯНИЯ». The 039 `REMOVED_COPY` test stays.

**Rationale**: Scanning the sources rather than the rendered page avoids false hits from authored session content, such as the demo node «СИСТЕМЫ ОБЪЕКТА». The scan covers the whole Overseer, not only the facility tab.

**Alternatives considered**: Scanning the rendered page was rejected because of the authored-content false positives.

## R15. Case convention

**Decision**: Controls, labels, dialog titles and statuses render in upper case, as the existing source does. Hints, warnings and error messages stay in sentence case. Authored names keep their case. `contracts/overseer-ui.md` maps every verbatim spec string to its rendered form, as 040's contract does.

**Rationale**: The Overseer has no CSS `text-transform`. All copy is literal in the source, and the existing hints («Неисправности не настроены для выбранного устройства.») are sentence case.

**Alternatives considered**: Rendering every string in upper case was rejected, because sentence-length warnings and error text become hard to read.

## R16. Demo additions

**Decision**: `sessions/demo.json` gains two things:

- A second reactor transition from «Аварийная остановка» to «Дежурный режим». It is «Пуск от резервной сети», with the precondition «Резервная сеть Убежища 76 = В работе», so the matrix shows a cell with two transitions.
- A second transition in one recovery program. Its source state must hold whenever the program's command can run, so the existing recovery command still succeeds.

Stored `recovery[]` values stay as they are. The inventory replaces every old route and marker with the new names, including «Загрязнён · ● 1». It adds rows for templates, the matrix, each rule type, the action dialog, the terminal-fault group, each link destination, and the filters. `internal/platform/assets_test.go` stops requiring a recovery entry for every condition.

**Rationale**: FR-044 needs reachable paths for both cases, and the demo has neither. A precondition on the second reactor transition keeps it different in play from the first.

**Alternatives considered**: Demonstrating the two-transition cell only in the browser fixture was rejected (Principle IX).
