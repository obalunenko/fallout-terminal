# Contract: Overseer UI

Identifiers the journeys and helpers rely on. Existing ids keep their meaning.

## Device sidebar

| Identifier | Contract |
|---|---|
| `#facilityTerminalFaults` | Stays a region named «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ». Gains `data-collapsed="true"` when collapsed. |
| `#facilityTerminalFaultsHeading` | Text stays exactly `НЕИСПРАВНОСТИ ТЕРМИНАЛОВ`. It holds `#btnToggleTerminalFaults`. |
| `#btnToggleTerminalFaults` (new) | Button with `aria-expanded` and `aria-controls="facilityTerminalFaultList"`; its text is the heading text. |
| `#facilityTerminalFaultsActiveCount` (new) | Next to the heading: `АКТИВНО N`. Visible in both states. |
| `#facilityTerminalFaultList`, `#facilityTerminalFaultsEmpty`, `#facilityTerminalFaultStatus` | Hidden while collapsed; unchanged otherwise. |
| `#facilityDeviceList` | Takes the sidebar height the group leaves. Every row and focus outline inside it is drawn whole. |

## Workspace

| Identifier | Contract |
|---|---|
| `#facilityStatus` | Keeps no `hidden` attribute, so the live region stays announced. While it has no text and `#facilityValidation` is hidden, their `.facility-feedback` wrapper is not displayed and takes no height. |

## Fault dialog

| Identifier | Contract |
|---|---|
| `#facilityConditionInitialWorking`, `#facilityConditionInitialActive` | Stay radio inputs that tests `.check()`. Drawn as Overseer segmented controls with the labels `ИСПРАВНО` and `НЕИСПРАВНОСТЬ АКТИВНА` under the legend `В НАЧАЛЕ ИГРЫ И ПОСЛЕ СБРОСА`. |

## Device dialog

| Identifier | Contract |
|---|---|
| `#btnApplyFacilityDeviceTemplate` | `aria-disabled="true"` with the reason `ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ` as soon as the dialog opens on a device whose template cells are all filled. |

## Terminal tree and rows

| Element | Contract |
|---|---|
| Tree row kind tag | Rendered with a space before the name, for example `[КОМАНДА] Название`. |
| `facilityRowActions` buttons | Visible text `ИЗМЕНИТЬ` and `УДАЛИТЬ`. `aria-label` stays `Изменить: <name>` and `Удалить: <name>`. |

## Feed

See [facility-feed-refusal.md](facility-feed-refusal.md).
