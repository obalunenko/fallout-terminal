# Overseer UI Contract

The existing `saveFacilityAuthoring`, `inspectFacilityDependencies`, `previewFacility`, `resetFacilityDevice`, `resetFacility`, and recovery operations remain unchanged. No new serialized contract is introduced.

## Workspace

- Retain `facilityWorkspace`, `facilityDeviceList`, `facilityBindingList`, `facilityConditionList`, `facilityRecoveryProgramList`, `facilitySelectionEditor`, `facilityConditionSelectionEditor`.
- Add `facilityDeviceSearch` (label: «Поиск устройств»), `facilityDetailPanel`, `facilityDeviceSummary`, `facilitySelectionEmpty`, `btnClearFacilitySelection`, `btnDiscardFacility`.
- Sidebar contains devices; detail panel contains selection controls then related bindings/faults/programs. No forced focus to device name after selection. Search retains the selected detail.
- Save label: «СОХРАНИТЬ ИЗМЕНЕНИЯ СЕССИИ». Dialog submit labels: «ПРИМЕНИТЬ К ЧЕРНОВИКУ». Selected-name staging uses the same wording. Discard: «ОТМЕНИТЬ ИЗМЕНЕНИЯ».
- Form footers use `.facility-dialog-actions`; transition advanced details use `.facility-transition-references`; selected status and reference summaries are readable text with technical IDs in details/title.

## Relationships and command context

- Binding/program rows have explicit edit/remove buttons with contextual accessible names.
- Command properties expose a facility action section with summary and an edit action opening the existing binding dialog preselected to that command.
- Dependencies resolve block IDs to their owning entry and focus the appropriate editor; composite transition IDs open that device's graph at the referenced transition. Conditions and programs open their editor. Unsupported/missing targets display an error rather than a dead button.
- Reset confirmation names scope and current → initial values. Dirty drafts must be saved/discarded before live reset/recovery.

## Preview

- Add `facilityPreviewContext` and `facilityPreviewChanges`. Explain saved configuration and no effect on players.
- Resolve automatically on opening/selection changes; retain Refresh for retry.
- `.facility-preview-changed`, `.facility-preview-unavailable`, and a hidden-items summary convey effects with text as well as color.
- Dirty drafts must be saved/discarded first. Ignore stale responses after close or selection change.

## Accessibility and regression expectations

Use associated labels, semantic controls, focus-visible outlines, and live status/errors. Desktop detail panel and list are independently scrollable; narrow layout stacks them without horizontal overflow. Sticky dialog footers must not cover fields. Technical IDs remain inspectable without dominating visible option labels.
