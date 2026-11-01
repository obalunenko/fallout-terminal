import { expect, test } from '@playwright/test';

import { switchTab } from './helpers/authoring-bar.mjs';
import { deviceList, deviceRows, filterCount } from './helpers/facility-editors.mjs';
import { forceState, playCard, selectDevice, setConditionActive } from './helpers/facility-play.mjs';
import { FIXTURE, fixtureSession, openOverseer, seedDemo } from './helpers/overseer-forms.mjs';

// User Story 5: spot changed and broken devices in the list
// (specs/041-facility-editors-language/spec.md, contracts/overseer-ui.md).

const GRID = 'power-grid-main';
const GRID_NAME = 'Резервная сеть Убежища 76';
const VENTILATION = 'ventilation-main';
const VENTILATION_NAME = 'Контур вентиляции теплицы';
const ELEVATOR = 'freight-elevator';
const SPORES = 'air-spore-bloom';
const NO_MATCH = 'Ни одно устройство не подходит под фильтр и поиск.';
const HIDDEN_SELECTION = 'Выбранное устройство скрыто фильтром';
const FILTERS = [['all', 'ВСЕ'], ['changed', 'ИЗМЕНЁННЫЕ'], ['faulty', 'С НЕИСПРАВНОСТЬЮ']];
const KIND_LABELS = {
  door: 'ДВЕРЬ', turret: 'ТУРЕЛЬ', 'power-grid': 'ЭЛЕКТРОСЕТЬ', reactor: 'РЕАКТОР', ventilation: 'ВЕНТИЛЯЦИЯ',
  alarm: 'СИГНАЛИЗАЦИЯ', 'robot-pod': 'РОБОТЕХНИЧЕСКАЯ КАПСУЛА', elevator: 'ЛИФТ', 'network-segment': 'СЕГМЕНТ СЕТИ',
};

function pageErrors(page) {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  return errors;
}

// openPlayedDemo seeds the demo with the play state it ships with: five
// devices away from their starting state and the spore fault active. The
// fixture's save starts the demo at its starting values, so the journey moves
// it there as play would.
async function openPlayedDemo(page, request) {
  const demo = await seedDemo(request);
  for (const device of demo.facility.devices) {
    if (device.currentStateId !== device.initialStateId) await forceState(request, device.id, device.currentStateId);
  }
  const forced = (await fixtureSession(request)).facility;
  for (const fault of demo.facility.conditions) {
    const live = forced.conditions.find(value => value.id === fault.id);
    if (live.currentActive !== fault.currentActive) await setConditionActive(request, fault.id, fault.currentActive);
  }
  const played = (await fixtureSession(request)).facility;
  expect(played.devices.map(device => [device.id, device.currentStateId]))
    .toEqual(demo.facility.devices.map(device => [device.id, device.currentStateId]));
  expect(played.conditions.map(fault => [fault.id, fault.currentActive]))
    .toEqual(demo.facility.conditions.map(fault => [fault.id, fault.currentActive]));
  await openOverseer(page);
  await expect(deviceList(page).rows).toHaveCount(demo.facility.devices.length);
  return played;
}

// expectedRows is the device list the facility implies, in authored order (FR-030).
function expectedRows(facility, keep = () => true) {
  return facility.devices.filter(keep).map(device => ({
    id: device.id,
    name: device.name,
    kind: device.kind === 'custom' ? 'ДРУГОЙ' : KIND_LABELS[device.kind],
    state: device.states.find(value => value.id === device.currentStateId).name,
    changed: device.currentStateId !== device.initialStateId,
    faults: activeFaults(facility, device.id),
  }));
}

function activeFaults(facility, deviceID) {
  return facility.conditions.filter(value => value.device?.deviceId === deviceID && value.currentActive).length;
}

async function listedIDs(page) {
  return deviceList(page).rows.evaluateAll(rows => rows.map(row => row.dataset.facilityId));
}

// expectFilters checks each filter's text, count, and pressed state (FR-031).
async function expectFilters(page, counts, pressed) {
  const list = deviceList(page);
  for (const [filter, label] of FILTERS) {
    const button = list.filter(filter);
    await expect(button, filter).toHaveText(`${label} ${counts[filter]}`);
    await expect(button, filter).toHaveAttribute('data-count', String(counts[filter]));
    await expect(button.locator('span.facility-filter-count'), filter).toHaveText(String(counts[filter]));
    await expect(button, filter).toHaveAttribute('aria-pressed', String(filter === pressed));
  }
}

async function pressFilter(page, filter) {
  await deviceList(page).filter(filter).click();
  await expect(deviceList(page).filter(filter)).toHaveAttribute('aria-pressed', 'true');
}

// expectRow checks one row's badge and fault mark against the session (FR-030).
async function expectRow(page, id, state, changed, faults) {
  const row = deviceList(page).row(id);
  await expect(row.locator('span.facility-state-badge'), id).toHaveText(state);
  await expect(row.locator('span.facility-state-badge'), id).toHaveAttribute('data-changed', String(changed));
  await expect(row, id).toHaveAttribute('data-active-faults', String(faults));
  if (faults) {
    await expect(row.locator('span.facility-fault-mark'), id).toHaveText(`● ${faults}`);
    await expect(row.locator('span.facility-fault-mark'), id).toHaveAttribute('aria-label', `Активных неисправностей: ${faults}`);
  } else {
    await expect(row.locator('.facility-fault-mark'), id).toHaveCount(0);
  }
}

// rowGeometry measures each row's text and badge against its content box.
async function rowGeometry(page) {
  return deviceList(page).rows.evaluateAll(rows => rows.map(row => {
    const style = getComputedStyle(row);
    const box = row.getBoundingClientRect();
    const left = box.left + parseFloat(style.borderLeftWidth) + parseFloat(style.paddingLeft);
    const right = box.right - parseFloat(style.borderRightWidth) - parseFloat(style.paddingRight);
    // The glyphs, not the boxes, show where the text starts.
    const glyphs = element => {
      const range = document.createRange();
      range.selectNodeContents(element);
      return range.getBoundingClientRect();
    };
    const text = row.querySelector(':scope > .facility-device-row-text');
    const name = glyphs(text.querySelector(':scope > strong'));
    const kind = glyphs(text.querySelector(':scope > .facility-device-row-kind'));
    const badge = row.querySelector(':scope > .facility-state-badge').getBoundingClientRect();
    return {
      id: row.dataset.facilityId,
      title: row.hasAttribute('title'),
      parts: [...row.children].map(child => child.className),
      textParts: [...text.children].map(child => (child.tagName === 'STRONG' ? 'STRONG' : child.className)),
      nameStart: name.left - left,
      kindStart: kind.left - left,
      nameAboveKind: name.bottom <= kind.top + 0.5,
      badgeAfterText: badge.left >= text.getBoundingClientRect().right - 0.5,
      end: right - row.lastElementChild.getBoundingClientRect().right,
    };
  }));
}

// rowColors applies the contract's color rules to each badge border and fault mark.
async function rowColors(page) {
  return deviceList(page).rows.evaluateAll(rows => rows.map(row => {
    const channels = value => (value.match(/[\d.]+/g) || []).map(Number);
    const badge = getComputedStyle(row.querySelector('.facility-state-badge'));
    const [r, g, b, alpha = 1] = channels(badge.borderTopColor);
    const shown = parseFloat(badge.borderTopWidth) > 0 && badge.borderTopStyle !== 'none' && alpha > 0;
    const mark = row.querySelector('.facility-fault-mark');
    let red = null;
    if (mark) {
      const [mr, mg, mb] = channels(getComputedStyle(mark).color);
      red = mr > mg && mr > mb;
    }
    return { id: row.dataset.facilityId, yellowFrame: shown && r >= 2 * b && g >= 2 * b, red };
  }));
}

async function playerCommand(request, terminalId, commandId) {
  const response = await request.post(`${FIXTURE}/player-command`, {
    data: { terminalId, commandId, decision: 'approve' },
  });
  expect(response.ok(), commandId).toBe(true);
}

test('the demo list shows each name over its type, frames the five changed badges, and marks the active fault (AS1, AS2, FR-030, SC-006)', async ({ page, request }) => {
  const errors = pageErrors(page);
  const facility = await openPlayedDemo(page, request);
  const list = deviceList(page);

  // The sidebar holds the search, the filters, the hidden-selection line, the list, and the terminal faults in order.
  expect(await page.locator('#facilityDeviceSidebar').evaluate(sidebar => [...sidebar.children]
    .map(child => child.id || `${child.tagName.toLowerCase()}${child.htmlFor ? `[for=${child.htmlFor}]` : ''}`)))
    .toEqual([
      'header', 'label[for=facilityDeviceSearch]', 'facilityDeviceSearch', 'facilityDeviceFilters',
      'facilityDeviceHiddenSelection', 'facilityDeviceList', 'facilityTerminalFaults',
    ]);
  await expect(page.getByRole('searchbox', { name: 'Поиск устройств', exact: true })).toHaveAttribute('placeholder', 'Название или состояние');
  const filters = page.getByRole('group', { name: 'Фильтр устройств', exact: true });
  await expect(filters).toHaveAttribute('id', 'facilityDeviceFilters');
  await expect(filters.getByRole('button')).toHaveText(['ВСЕ 10', 'ИЗМЕНЁННЫЕ 5', 'С НЕИСПРАВНОСТЬЮ 1']);
  await expectFilters(page, { all: 10, changed: 5, faulty: 1 }, 'all');
  await expect(page.getByRole('group', { name: 'Устройства локации', exact: true })).toHaveAttribute('id', 'facilityDeviceList');
  await expect(list.hiddenSelection).toBeHidden();

  // Every row reads name, type, and current state, in authored order.
  const expected = expectedRows(facility);
  expect(await deviceRows(page)).toEqual(expected);
  expect(expected.filter(row => row.changed).map(row => row.id))
    .toEqual([GRID, 'reactor-main', 'robot-pod-atrium', ELEVATOR, 'water-purifier-76']);
  expect(expected.filter(row => row.faults).map(row => [row.id, row.faults])).toEqual([[VENTILATION, 1]]);
  for (const row of expected) await expectRow(page, row.id, row.state, row.changed, row.faults);
  await expect(list.row(GRID).locator('.facility-device-row-kind')).toHaveText('ЭЛЕКТРОСЕТЬ');
  await expect(list.row('water-purifier-76').locator('.facility-device-row-kind')).toHaveText('ДРУГОЙ');

  // The name and type start at the left content edge, the name above the type,
  // and the badge and mark end at the right one.
  for (const row of await rowGeometry(page)) {
    const faults = expected.find(value => value.id === row.id).faults;
    expect(row.parts, row.id).toEqual(['facility-device-row-text', 'facility-state-badge', ...(faults ? ['facility-fault-mark'] : [])]);
    expect(row.textParts, row.id).toEqual(['STRONG', 'facility-device-row-kind']);
    expect(row.title, `${row.id} reveals no ID on hover`).toBe(false);
    expect(Math.abs(row.nameStart), `${row.id} name start`).toBeLessThanOrEqual(1);
    expect(Math.abs(row.kindStart), `${row.id} type start`).toBeLessThanOrEqual(1);
    expect(row.nameAboveKind, `${row.id} name above type`).toBe(true);
    expect(row.badgeAfterText, `${row.id} badge right of the text`).toBe(true);
    expect(Math.abs(row.end), `${row.id} right end`).toBeLessThanOrEqual(1);
  }

  // Changed badges have a yellow frame, unchanged ones do not, and the fault mark is red.
  expect(await rowColors(page)).toEqual(expected.map(row => ({
    id: row.id, yellowFrame: row.changed, red: row.faults ? true : null,
  })));
  await expect(list.row(VENTILATION).locator('.facility-fault-mark')).toHaveText('● 1');
  expect(errors).toEqual([]);
});

test('«ИЗМЕНЁННЫЕ» lists the five changed devices, and search for «В работе» leaves the reserve grid (AS3, FR-031, SC-006)', async ({ page, request }) => {
  const errors = pageErrors(page);
  const facility = await openPlayedDemo(page, request);
  const list = deviceList(page);
  const counts = { all: 10, changed: 5, faulty: 1 };

  await pressFilter(page, 'changed');
  await expectFilters(page, counts, 'changed');
  const changed = expectedRows(facility, device => device.currentStateId !== device.initialStateId);
  expect(await deviceRows(page)).toEqual(changed);
  expect(changed).toHaveLength(5);

  // Search and filter combine; the counts stay over the whole session.
  await list.search.fill('В работе');
  await expect(list.rows).toHaveCount(1);
  expect(await deviceRows(page)).toEqual(expectedRows(facility, device => device.id === GRID));
  await expect(list.row(GRID).locator('strong')).toHaveText(GRID_NAME);
  await expectFilters(page, counts, 'changed');
  expect(await filterCount(page, 'changed')).toBe(5);

  // «С НЕИСПРАВНОСТЬЮ» lists exactly the devices with an active fault.
  await list.search.fill('');
  await pressFilter(page, 'faulty');
  await expectFilters(page, counts, 'faulty');
  expect(await listedIDs(page)).toEqual(facility.devices.filter(device => activeFaults(facility, device.id)).map(device => device.id));
  expect(await listedIDs(page)).toEqual([VENTILATION]);
  await pressFilter(page, 'all');
  await expect(list.rows).toHaveCount(10);
  await expectFilters(page, counts, 'all');
  expect(errors).toEqual([]);
});

test('search matches names and current states, trimmed and in any case, but no longer device IDs (FR-031)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openPlayedDemo(page, request);
  const list = deviceList(page);
  for (const [query, ids] of [
    ['Контур вентиляции', [VENTILATION]],
    ['  контур ВЕНТИЛЯЦИИ  ', [VENTILATION]],
    ['Загрязнён', [VENTILATION]],
    ['в работе', [GRID]],
    ['Верхний уровень', [ELEVATOR]],
    // Only the current state counts: the lift is at its upper level.
    ['Нижний уровень', []],
    // IDs and kind values are not searched any more.
    [VENTILATION, []],
    ['power-grid', []],
    [ELEVATOR, []],
  ]) {
    await list.search.fill(query);
    await expect.poll(() => listedIDs(page), query).toEqual(ids);
    if (!ids.length) await expect(page.locator('#facilityDeviceList p.facility-empty'), query).toHaveText(NO_MATCH);
    // Search never changes the counts.
    await expectFilters(page, { all: 10, changed: 5, faulty: 1 }, 'all');
  }
  await list.search.fill('');
  await expect(list.rows).toHaveCount(10);
  expect(errors).toEqual([]);
});

test('rows, badges, marks, and counts follow card transitions, fault switches, and player commands (FR-032)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openPlayedDemo(page, request);
  const list = deviceList(page);
  await selectDevice(page, VENTILATION);
  const card = playCard(page);

  // The card's fault switch clears and restores the spore fault.
  await card.faultSwitch(SPORES).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ СНЯТА');
  await expectRow(page, VENTILATION, 'Загрязнён', false, 0);
  await expectFilters(page, { all: 10, changed: 5, faulty: 0 }, 'all');
  await card.faultSwitch(SPORES).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  await expectRow(page, VENTILATION, 'Загрязнён', false, 1);
  await expectFilters(page, { all: 10, changed: 5, faulty: 1 }, 'all');

  // A card transition moves the loop off its starting state and clears its fault.
  await card.run('purge').click();
  await expect(card.status).toHaveText('ПЕРЕХОД ВЫПОЛНЕН');
  await expectRow(page, VENTILATION, 'Очищен', true, 0);
  await expectFilters(page, { all: 10, changed: 6, faulty: 0 }, 'all');
  expect((await rowColors(page)).find(row => row.id === VENTILATION)).toEqual({ id: VENTILATION, yellowFrame: true, red: null });
  await card.run('contaminate').click();
  await expect(card.status).toHaveText('ПЕРЕХОД ВЫПОЛНЕН');
  await expectRow(page, VENTILATION, 'Загрязнён', false, 1);
  await expectFilters(page, { all: 10, changed: 5, faulty: 1 }, 'all');

  // A player command lowers the lift back to its starting level; it leaves «ИЗМЕНЁННЫЕ» live.
  await pressFilter(page, 'changed');
  expect(await listedIDs(page)).toContain(ELEVATOR);
  await playerCommand(request, 't_demo_hack_2', 'n4_cmd_call_lift');
  await expect(list.row(ELEVATOR)).toHaveCount(0);
  await expectFilters(page, { all: 10, changed: 4, faulty: 1 }, 'changed');
  await pressFilter(page, 'all');
  await expectRow(page, ELEVATOR, 'Нижний уровень', false, 0);
  await expect(list.row(ELEVATOR)).toHaveAttribute('data-current-state-id', 'lower');

  // The reserve grid's own fault gives it a mark and joins «С НЕИСПРАВНОСТЬЮ».
  await selectDevice(page, GRID);
  await card.faultSwitch('grid-unpowered').click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  await expectRow(page, GRID, 'В работе', true, 1);
  await expectFilters(page, { all: 10, changed: 4, faulty: 2 }, 'all');
  expect((await rowColors(page)).find(row => row.id === GRID)).toEqual({ id: GRID, yellowFrame: true, red: true });
  await pressFilter(page, 'faulty');
  expect(await listedIDs(page)).toEqual([GRID, VENTILATION]);
  expect(errors).toEqual([]);
});

test('the selected device leaves «С НЕИСПРАВНОСТЬЮ» when its last fault clears, and its details stay open (AS4, FR-032, FR-033)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openPlayedDemo(page, request);
  const list = deviceList(page);
  await pressFilter(page, 'faulty');
  await selectDevice(page, VENTILATION);
  await expect(list.row(VENTILATION)).toHaveAttribute('aria-pressed', 'true');
  await expect(list.hiddenSelection).toBeHidden();

  const card = playCard(page);
  await card.faultSwitch(SPORES).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ СНЯТА');
  await expect(list.row(VENTILATION)).toHaveCount(0);
  await expectFilters(page, { all: 10, changed: 5, faulty: 0 }, 'faulty');
  await expect(list.hiddenSelection).toBeVisible();
  await expect(list.hiddenSelection).toHaveText(HIDDEN_SELECTION);
  await expect(page.locator('#facilitySelectionEditor')).toBeVisible();
  await expect(page.locator('#facilityDeviceHeaderName')).toHaveText(VENTILATION_NAME);
  await expect(card.card).toBeVisible();
  await expect(card.faultSwitch(SPORES)).toHaveAttribute('aria-checked', 'false');

  // The fault returns from the still-open card, and so does the row.
  await card.faultSwitch(SPORES).click();
  await expect(card.status).toHaveText('НЕИСПРАВНОСТЬ АКТИВИРОВАНА');
  await expect(list.row(VENTILATION)).toHaveAttribute('aria-pressed', 'true');
  await expectFilters(page, { all: 10, changed: 5, faulty: 1 }, 'faulty');
  await expect(list.hiddenSelection).toBeHidden();
  expect(errors).toEqual([]);
});

test('a filter and search that match nothing say so and offer «ВСЕ», and a hidden selection is named (AS5, FR-033)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openPlayedDemo(page, request);
  const list = deviceList(page);

  // Without a selection, a narrowed list hides no selected device.
  await pressFilter(page, 'faulty');
  await expect(list.rows).toHaveCount(1);
  await expect(list.hiddenSelection).toBeHidden();

  // The search hides the selected device; its details stay open.
  await pressFilter(page, 'all');
  await selectDevice(page, GRID);
  await list.search.fill('Загрязнён');
  await expect.poll(() => listedIDs(page)).toEqual([VENTILATION]);
  await expect(list.hiddenSelection).toHaveText(HIDDEN_SELECTION);
  await expect(list.hiddenSelection).toBeVisible();
  await expect(page.locator('#facilityDeviceHeaderName')).toHaveText(GRID_NAME);
  await expect(playCard(page).card).toBeVisible();

  // Filter and search combine to nothing: the list says so and offers «ВСЕ».
  await pressFilter(page, 'changed');
  await expect(list.rows).toHaveCount(0);
  const empty = page.locator('#facilityDeviceList > p.facility-empty');
  await expect(empty).toHaveText(NO_MATCH);
  await expect(list.reset).toHaveText('ВСЕ');
  await expect(page.locator('#facilityDeviceList').getByRole('button', { name: 'ВСЕ', exact: true })).toHaveAttribute('id', 'btnFacilityDeviceFilterReset');
  await expect(page.locator('#facilityDeviceList')).not.toContainText('Добавьте первое устройство');
  await expect(list.hiddenSelection).toBeVisible();
  await expectFilters(page, { all: 10, changed: 5, faulty: 1 }, 'changed');

  // «ВСЕ» presses «ВСЕ», clears the search, and focuses the «ВСЕ» filter.
  await list.reset.click();
  await expectFilters(page, { all: 10, changed: 5, faulty: 1 }, 'all');
  await expect(list.search).toHaveValue('');
  await expect(list.filter('all')).toBeFocused();
  await expect(list.rows).toHaveCount(10);
  await expect(empty).toHaveCount(0);
  await expect(list.reset).toHaveCount(0);
  await expect(list.hiddenSelection).toBeHidden();
  await expect(list.row(GRID)).toHaveAttribute('aria-pressed', 'true');
  expect(errors).toEqual([]);
});

test('the filter choice survives tab switches and session changes, and a restart starts at «ВСЕ» (FR-033)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await openPlayedDemo(page, request);
  const list = deviceList(page);
  await pressFilter(page, 'changed');
  await expect(list.rows).toHaveCount(5);

  await switchTab(page, 'ТЕРМИНАЛЫ');
  await switchTab(page, 'ОБЪЕКТЫ');
  await expectFilters(page, { all: 10, changed: 5, faulty: 1 }, 'changed');
  await expect(list.rows).toHaveCount(5);

  // A player's write is a session change; the filter stays and recounts.
  await playerCommand(request, 't_demo_hack_2', 'n4_cmd_call_lift');
  await expectFilters(page, { all: 10, changed: 4, faulty: 1 }, 'changed');
  await expect(list.rows).toHaveCount(4);

  // Nothing is stored: a new run starts at «ВСЕ».
  await openOverseer(page);
  await expectFilters(page, { all: 10, changed: 4, faulty: 1 }, 'all');
  await expect(list.rows).toHaveCount(10);
  expect(errors).toEqual([]);
});

test('filters and search never narrow «НЕИСПРАВНОСТИ ТЕРМИНАЛОВ», and its switches leave the device counts alone (FR-021, FR-031)', async ({ page, request }) => {
  const errors = pageErrors(page);
  const facility = await openPlayedDemo(page, request);
  const list = deviceList(page);
  const terminalFaults = facility.conditions.filter(value => value.terminal).map(value => value.id);
  expect(terminalFaults).toHaveLength(4);
  const group = page.locator('#facilityTerminalFaults');
  await expect(page.getByRole('region', { name: 'НЕИСПРАВНОСТИ ТЕРМИНАЛОВ', exact: true })).toHaveAttribute('id', 'facilityTerminalFaults');
  const listedFaults = () => list.terminalFaults.evaluateAll(items => items.map(item => item.dataset.conditionId));

  for (const [filter, query] of [['all', ''], ['changed', ''], ['faulty', ''], ['faulty', 'В работе'], ['all', 'Закрытый канал']]) {
    await pressFilter(page, filter);
    await list.search.fill(query);
    await expect(group, `${filter} «${query}»`).toBeVisible();
    expect(await listedFaults(), `${filter} «${query}»`).toEqual(terminalFaults);
  }
  // «Закрытый канал» names a terminal fault, not a device.
  await expect(list.rows).toHaveCount(0);
  await expect(page.locator('#facilityDeviceList > p.facility-empty')).toHaveText(NO_MATCH);

  // A terminal fault's switch works from the group and is not a device fault.
  await list.reset.click();
  await expect(list.terminalFault('network-isolated')).toHaveAttribute('data-active', 'true');
  await list.terminalFaultSwitch('network-isolated').click();
  await expect(list.terminalFaultStatus).toHaveText('НЕИСПРАВНОСТЬ СНЯТА');
  await expect(list.terminalFault('network-isolated')).toHaveAttribute('data-active', 'false');
  await expectFilters(page, { all: 10, changed: 5, faulty: 1 }, 'all');
  expect(await deviceRows(page)).toEqual(expectedRows((await fixtureSession(request)).facility));
  expect(errors).toEqual([]);
});

// fullyVisibleRows counts the device rows drawn whole inside the list's
// visible box, borders included.
async function fullyVisibleRows(page) {
  return page.locator('#facilityDeviceList').evaluate(list => {
    const box = list.getBoundingClientRect();
    const top = box.top + list.clientTop;
    const bottom = top + list.clientHeight;
    return [...list.querySelectorAll('button[data-facility-kind="device"]')]
      .map(row => row.getBoundingClientRect())
      .filter(rect => rect.top >= top && rect.bottom <= bottom).length;
  });
}

test('the terminal-fault group collapses with its active count and frees room for seven rows at 1440×900 (042 FR-015, FR-016, SC-005)', async ({ page, request }) => {
  await page.setViewportSize({ width: 1440, height: 869 });
  await seedDemo(request);
  await openOverseer(page);
  const toggle = page.locator('#btnToggleTerminalFaults');
  const count = page.locator('#facilityTerminalFaultsActiveCount');
  const active = (await fixtureSession(request)).facility.conditions
    .filter(condition => condition.terminal && condition.currentActive).length;
  await expect(page.locator('#facilityTerminalFaultsHeading')).toHaveText('НЕИСПРАВНОСТИ ТЕРМИНАЛОВ');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  await expect(count).toHaveText(`АКТИВНО ${active}`);

  await toggle.click();
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await expect(page.locator('#facilityTerminalFaults')).toHaveAttribute('data-collapsed', 'true');
  await expect(page.locator('#facilityTerminalFaultList')).toBeHidden();
  await expect(count).toHaveText(`АКТИВНО ${active}`);
  expect(await fullyVisibleRows(page)).toBeGreaterThanOrEqual(7);

  // The choice holds for the run and returns to expanded after a restart.
  await switchTab(page, 'ТЕРМИНАЛЫ');
  await switchTab(page, 'ОБЪЕКТЫ');
  await selectDevice(page, VENTILATION);
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await page.reload();
  await openOverseer(page);
  await expect(page.locator('#btnToggleTerminalFaults')).toHaveAttribute('aria-expanded', 'true');
});

test('a filtered row and a focused row are drawn whole (042 FR-017, FR-022, SC-006)', async ({ page, request }) => {
  await page.setViewportSize({ width: 1440, height: 869 });
  await seedDemo(request);
  await openOverseer(page);
  const list = deviceList(page);
  await list.search.fill(GRID_NAME);
  await expect(list.rows).toHaveCount(1);
  expect(await fullyVisibleRows(page)).toBe(1);

  await list.search.fill('');
  await expect(list.rows).toHaveCount(10);
  await list.search.click();
  for (let step = 0; step < 6; step += 1) {
    if (await page.evaluate(() => document.activeElement?.dataset?.facilityKind === 'device')) break;
    await page.keyboard.press('Tab');
  }
  const clipped = await page.evaluate(() => {
    const row = document.activeElement;
    if (row?.dataset?.facilityKind !== 'device') return 'no device row focused';
    const list = document.getElementById('facilityDeviceList');
    const style = getComputedStyle(row);
    const extent = parseFloat(style.outlineWidth) + parseFloat(style.outlineOffset);
    const rect = row.getBoundingClientRect();
    const box = list.getBoundingClientRect();
    const left = box.left + list.clientLeft;
    const top = box.top + list.clientTop;
    return rect.left - extent >= left && rect.right + extent <= left + list.clientWidth && rect.top - extent >= top
      ? '' : `outline ${extent}px clipped`;
  });
  expect(clipped).toBe('');
});

