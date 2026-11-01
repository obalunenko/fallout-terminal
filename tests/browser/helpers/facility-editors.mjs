import { expect } from '@playwright/test';

import { selectDevice } from './facility-play.mjs';

// Journey helpers for the facility editors
// (specs/041-facility-editors-language/contracts/overseer-ui.md).

// Opens the selected device's dialog through «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ».
export async function openDeviceDialog(page, deviceId) {
  await selectDevice(page, deviceId);
  await page.locator('#btnEditFacilityDevice').click();
  const dialog = deviceDialog(page);
  await expect(dialog.dialog).toBeVisible();
  return dialog;
}

export function deviceDialog(page) {
  const dialog = page.locator('#facilityDeviceDialog');
  const matrix = page.locator('#facilityTransitionMatrix');
  return {
    dialog,
    name: page.locator('#facilityDeviceName'),
    kind: page.locator('#facilityDeviceKind'),
    template: page.locator('#facilityDeviceTemplate'),
    templateSummary: page.locator('#facilityDeviceTemplateSummary'),
    applyTemplate: page.locator('#btnApplyFacilityDeviceTemplate'),
    chips: page.locator('#facilityDeviceStateChips button.facility-state-chip'),
    chip: id => page.locator(`#facilityDeviceStateChips button.facility-state-chip[data-state-id="${id}"]`),
    addState: page.locator('#btnAddFacilityState'),
    stateEditor: page.locator('#facilityStateEditor'),
    stateName: page.locator('#facilityStateEditorName'),
    matrix,
    cells: matrix.locator('button.facility-transition-cell'),
    cell: (from, to) => matrix.locator(`button.facility-transition-cell[data-from="${from}"][data-to="${to}"]`),
    unavailable: (from, to) => matrix.locator(`td.facility-transition-cell-unavailable[data-from="${from}"][data-to="${to}"]`),
    panel: page.locator('#facilityTransitionPanel'),
    panelHeading: page.locator('#facilityTransitionPanelHeading'),
    addCellTransition: page.locator('#btnAddFacilityCellTransition'),
    technical: page.locator('#facilityDeviceTechnical'),
    error: page.locator('#facilityDeviceError'),
    save: page.locator('#btnSaveFacilityDevice'),
    cancel: dialog.locator('.facility-dialog-actions').getByRole('button', { name: 'ОТМЕНА', exact: true }),
  };
}

// The starting state's ID and every chip's name, in draft order.
export async function stateChips(page) {
  return page.locator('#facilityDeviceStateChips button.facility-state-chip').evaluateAll(chips => chips.map(chip => ({
    id: chip.dataset.stateId,
    initial: chip.dataset.initial === 'true',
    name: chip.querySelector('.facility-state-chip-name')?.textContent ?? '',
  })));
}

// Every off-diagonal matrix cell with its transition count and visible text.
export async function matrixCells(page) {
  return page.locator('#facilityTransitionMatrix button.facility-transition-cell').evaluateAll(cells => cells.map(cell => ({
    from: cell.dataset.from,
    to: cell.dataset.to,
    count: Number(cell.dataset.count),
    text: cell.textContent.trim(),
  })));
}

export async function openTransitionCell(page, from, to) {
  const dialog = deviceDialog(page);
  await dialog.cell(from, to).click();
  await expect(dialog.panel).toBeVisible();
  return dialog.panel;
}

export function faultDialog(page) {
  const dialog = page.locator('#facilityConditionDialog');
  return {
    dialog,
    title: page.locator('#facilityConditionDialogTitle'),
    name: page.locator('#facilityConditionName'),
    category: page.locator('#facilityConditionCategory'),
    scope: page.locator('#facilityConditionScope'),
    capability: page.locator('#facilityConditionCapability'),
    appears: page.locator('#facilityFaultAppearsList li.facility-fault-way'),
    appearsWarning: page.locator('#facilityFaultAppearsWarning'),
    addActivation: page.locator('#btnAddFaultActivation'),
    activationPicker: page.locator('#facilityFaultActivationPicker'),
    clears: page.locator('#facilityFaultClearsList li'),
    clearingPicker: page.locator('#facilityFaultClearingPicker'),
    addProgram: page.locator('#btnAddFaultRecoveryProgram'),
    technical: page.locator('#facilityConditionTechnical'),
    error: page.locator('#facilityConditionError'),
    save: page.locator('#btnSaveFacilityCondition'),
    cancel: dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }),
  };
}

// The rows of «КАК ПОЯВЛЯЕТСЯ» or «КАК УСТРАНЯЕТСЯ» as { kind, deviceId, transitionId, programId, text }.
export async function faultWays(list) {
  return list.evaluateAll(rows => rows.map(row => ({
    kind: row.dataset.kind,
    deviceId: row.dataset.deviceId ?? '',
    transitionId: row.dataset.transitionId ?? '',
    programId: row.dataset.programId ?? '',
    text: row.textContent.trim(),
  })));
}

export function programDialog(page) {
  const dialog = page.locator('#facilityRecoveryProgramDialog');
  return {
    dialog,
    name: page.locator('#facilityRecoveryProgramName'),
    rows: page.locator('#facilityProgramTransitions .facility-action-request-row'),
    addTransition: page.locator('#btnAddFacilityProgramTransition'),
    commandsWarning: page.locator('#facilityProgramCommandsWarning'),
    error: page.locator('#facilityRecoveryProgramError'),
    save: page.locator('#btnSaveFacilityRecoveryProgram'),
    cancel: dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }),
  };
}

export const RULE_TABS = {
  'entry-content': '#facilityRuleTabEntryContent',
  'menu-name': '#facilityRuleTabMenuName',
  visibility: '#facilityRuleTabVisibility',
  availability: '#facilityRuleTabAvailability',
};

export function ruleDialog(page) {
  const dialog = page.locator('#facilityBindingDialog');
  return {
    dialog,
    title: page.locator('#facilityBindingDialogTitle'),
    tabs: page.locator('#facilityRuleTabs [role="tab"]'),
    tab: kind => page.locator(RULE_TABS[kind]),
    sentence: page.locator('#facilityRuleSentence'),
    clause: name => page.locator(`#facilityRuleSentence .facility-rule-clause[data-clause="${name}"]`),
    device: page.locator('#facilityBindingDevice'),
    state: page.locator('#facilityBindingState'),
    where: page.locator('#facilityBindingNode'),
    block: page.locator('#facilityBindingBlock'),
    text: page.locator('#facilityBindingText'),
    otherStates: page.locator('#facilityRuleOtherStates'),
    conflict: page.locator('#facilityRuleConflict'),
    error: page.locator('#facilityBindingError'),
    save: page.locator('#btnSaveFacilityBinding'),
    cancel: dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }),
  };
}

// Picks a «ГДЕ» option by its terminal and node IDs, since option values are opaque indexes.
export async function chooseWhere(page, terminalId, nodeId) {
  const option = page.locator(`#facilityBindingNode option[data-terminal-id="${terminalId}"][data-node-id="${nodeId}"]`);
  await expect(option).toHaveCount(1);
  await page.locator('#facilityBindingNode').selectOption(await option.getAttribute('value'));
}

// Fills «КОГДА / ГДЕ / БЛОК / ТЕКСТ» on the active tab; omitted parts stay as they are.
export async function fillRule(page, { deviceId, stateId, terminalId, nodeId, block, text } = {}) {
  const rule = ruleDialog(page);
  if (deviceId) await rule.device.selectOption(deviceId);
  if (stateId) await rule.state.selectOption(stateId);
  if (terminalId && nodeId) await chooseWhere(page, terminalId, nodeId);
  if (block !== undefined) await rule.block.selectOption(block);
  if (text !== undefined) await rule.text.fill(text);
  return rule;
}

export function commandActionDialog(page) {
  const dialog = page.locator('#facilityCommandActionDialog');
  return {
    dialog,
    title: page.locator('#facilityCommandActionDialogTitle'),
    command: page.locator('#facilityCommandActionCommand'),
    commandFixed: page.locator('#facilityCommandActionCommandFixed'),
    modeTransitions: page.locator('#facilityCommandActionModeTransitions'),
    modeProgram: page.locator('#facilityCommandActionModeProgram'),
    rows: page.locator('#facilityCommandActionTransitions .facility-action-request-row'),
    addTransition: page.locator('#btnAddFacilityCommandActionTransition'),
    program: page.locator('#facilityCommandActionProgram'),
    completedName: page.locator('#facilityCommandActionCompletedName'),
    confirmationText: page.locator('#facilityCommandActionConfirmationText'),
    error: page.locator('#facilityCommandActionError'),
    save: page.locator('#btnSaveFacilityCommandAction'),
    cancel: dialog.getByRole('button', { name: 'ОТМЕНА', exact: true }),
  };
}

export function deviceList(page) {
  return {
    search: page.locator('#facilityDeviceSearch'),
    filter: name => page.locator(`#facilityDeviceFilters button.facility-device-filter[data-filter="${name}"]`),
    rows: page.locator('#facilityDeviceList button[data-facility-kind="device"]'),
    row: id => page.locator(`#facilityDeviceList button[data-facility-kind="device"][data-facility-id="${id}"]`),
    hiddenSelection: page.locator('#facilityDeviceHiddenSelection'),
    reset: page.locator('#btnFacilityDeviceFilterReset'),
    terminalFaults: page.locator('#facilityTerminalFaults li.facility-terminal-fault'),
    terminalFault: id => page.locator(`#facilityTerminalFaults li.facility-terminal-fault[data-condition-id="${id}"]`),
    terminalFaultSwitch: id => page.locator(`#facilityTerminalFaults li.facility-terminal-fault[data-condition-id="${id}"] button.facility-play-fault-switch`),
    terminalFaultOpen: id => page.locator(`#facilityTerminalFaults li.facility-terminal-fault[data-condition-id="${id}"] button.facility-terminal-fault-open`),
    terminalFaultStatus: page.locator('#facilityTerminalFaultStatus'),
  };
}

// Each visible device row as { id, name, kind, state, changed, faults }.
export async function deviceRows(page) {
  return page.locator('#facilityDeviceList button[data-facility-kind="device"]').evaluateAll(rows => rows.map(row => ({
    id: row.dataset.facilityId,
    name: row.querySelector('strong')?.textContent ?? '',
    kind: row.querySelector('.facility-device-row-kind')?.textContent ?? '',
    state: row.querySelector('.facility-state-badge')?.textContent ?? '',
    changed: row.querySelector('.facility-state-badge')?.dataset.changed === 'true',
    faults: Number(row.dataset.activeFaults ?? 0),
  })));
}

// The count shown on a filter button, read from its trailing number.
export async function filterCount(page, name) {
  const text = await deviceList(page).filter(name).textContent();
  return Number(text.trim().match(/(\d+)$/)?.[1] ?? Number.NaN);
}

// A rendered error sink as { area, key, message, original }; original is '' without a disclosure.
export async function renderedError(sink) {
  await expect(sink).toBeVisible();
  return sink.evaluate(element => ({
    area: element.dataset.errorArea ?? '',
    key: element.dataset.errorKey ?? '',
    message: element.querySelector('.overseer-error-message')?.textContent ?? '',
    original: element.querySelector('.overseer-error-details .overseer-error-original')?.textContent ?? '',
    summary: element.querySelector('.overseer-error-details summary')?.textContent ?? '',
  }));
}

// SC-008: no Latin letters outside «…» segments of a visible message.
export function expectRussianMessage(message) {
  expect(message.replace(/«[^»]*»/g, '')).not.toMatch(/[A-Za-z]/);
}
