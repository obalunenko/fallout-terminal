# Data Model: Facility Authoring P0 Fixes

No persistent field, protobuf message, or Go model changes. This document records the authoring
rules the Overseer enforces on existing data and the transient form state it introduces.

## Facility entity identifier (existing)

| Entity | Stored form | Scope of uniqueness | Fallback |
|---|---|---|---|
| Device | `device-<stem>` | all devices in the facility | `device-1`, `device-2`, … |
| State | `<stem>` | states of one device | `state-1`, … |
| Transition | `<stem>` | transitions of one device | `transition-1`, … |
| Fault (condition) | `condition-<stem>` | all faults | `condition-1`, … |
| Recovery program | `program-<stem>` | all recovery programs | `program-1`, … |

Rules:
- `<stem>` = lower-case name → FR-008 transliteration → diacritics removed → runs outside
  `[a-z0-9]` collapsed to `-` → leading/trailing `-` trimmed.
- Suggested only while the identifier field has not been edited by hand and only for entities that
  do not exist yet (`dataset.manual` semantics unchanged).
- A collision appends `-2`, `-3`, … to the final value; the total length never exceeds 256.
- Existing identifiers are never rewritten (FR-012, FR-021).

State names stay unique within a device after trimming and ignoring case (existing server rule).

Examples: «Дверь в убежище» → `device-dver-v-ubezhishche`; «Заперта» → `zaperta`; «Открыта» →
`otkryta`; «Открыта!» next to it → `otkryta-2`; «Ядро» → `yadro`; «Южный шлюз» → `yuzhnyy-shlyuz`;
«★★★» → `state-1`.

## Command (existing) and device action

| Field | Meaning | Rule in this feature |
|---|---|---|
| mode | ordinary / state-changing / terminal transition | Leaving state-changing with a device action requires an explicit keep/remove decision. |
| `stateChange.completedName` | name after execution | Required; never copied from the original name by the binding editor. |
| `stateChange.confirmationText` | approval request text | Required; never copied from the result text by the binding editor. |
| `stateChange.facilityAction` | transitions or recovery program | Exists only on state-changing commands; removed only by an explicit decision. |

## Fault (existing)

| Field | Meaning | Rule in this feature |
|---|---|---|
| `initialActive` | activity at game start and after reset | Authorable in the fault dialog. |
| `currentActive` | activity now | New fault: equals `initialActive`. Existing fault: unchanged by editing `initialActive`. |

## Transient Overseer form state (new, not persisted)

| State | Owner | Lifecycle |
|---|---|---|
| Pending mode decision (`none` / `awaiting` / `remove`) | command form in `renderNodeForm` | Created when the selector leaves state-changing with an action; cleared by keep, by reselecting the original mode, by apply, or by re-render. |
| Draft guard active | command form | True while `facilityDraftDirty` and the command has an effective action; cleared after the draft is saved or discarded. |
| Row validation messages | device dialog rows | Created on apply; cleared when the row changes or the dialog reopens. |
