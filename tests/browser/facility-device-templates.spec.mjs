import { expect, test } from '@playwright/test';

import { blockedReason, expectSaved, undo } from './helpers/authoring-bar.mjs';
import {
  deviceDialog, deviceList, filterCount, openDeviceDialog, stateChips,
} from './helpers/facility-editors.mjs';
import { demoSession, fixtureSession, openOverseer, seedDemo, seedSession } from './helpers/overseer-forms.mjs';

// User Story 3: start a device from its type's template
// (specs/041-facility-editors-language/spec.md, contracts/overseer-ui.md «Device dialog» → «Template»).

const APPLIED = (states, transitions) => `ИЗ ШАБЛОНА ДОБАВЛЕНО: СОСТОЯНИЙ — ${states}, ПЕРЕХОДОВ — ${transitions}`;
const NOTHING_TO_ADD = 'ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ';

// Each template is a starting state, a second state, and one transition each way (FR-009).
function template(kind, label, [first, second], [forward, back]) {
  return {
    kind,
    label,
    initial: first,
    states: [first, second],
    transitions: [{ name: forward, from: first, to: second }, { name: back, from: second, to: first }],
  };
}

const TEMPLATES = [
  template('door', 'ДВЕРЬ', ['Заперта', 'Открыта'], ['Открыть', 'Запереть']),
  template('turret', 'ТУРЕЛЬ', ['Безопасный режим', 'Сопровождение'], ['Включить сопровождение', 'Вернуть безопасный режим']),
  template('power-grid', 'ЭЛЕКТРОСЕТЬ', ['Нет питания', 'В работе'], ['Запустить', 'Отключить']),
  template('reactor', 'РЕАКТОР', ['Аварийная остановка', 'Дежурный режим'], ['Перевести в дежурный режим', 'Остановить']),
  template('ventilation', 'ВЕНТИЛЯЦИЯ', ['Загрязнён', 'Очищен'], ['Продуть', 'Смоделировать загрязнение']),
  template('alarm', 'СИГНАЛИЗАЦИЯ', ['Тишина', 'Эвакуация'], ['Подать сигнал', 'Отключить сигнал']),
  template('robot-pod', 'РОБОТЕХНИЧЕСКАЯ КАПСУЛА', ['Зарядка', 'Патруль'], ['Отправить в патруль', 'Вернуть на зарядку']),
  template('elevator', 'ЛИФТ', ['Нижний уровень', 'Верхний уровень'], ['Поднять кабину', 'Опустить кабину']),
  template('network-segment', 'СЕГМЕНТ СЕТИ', ['Изолирована', 'Подключена'], ['Подключить', 'Изолировать']),
];
const DOOR = TEMPLATES[0];
const ELEVATOR = TEMPLATES.find(value => value.kind === 'elevator');

const summary = ({ states, transitions }) => `${states.join(', ')} · ${transitions.map(value => value.name).join(', ')}`;
const byPair = (left, right) => `${left.from} → ${left.to}: ${left.name}`.localeCompare(`${right.from} → ${right.to}: ${right.name}`);

// A graph as state names with the star, and transitions by name and state names.
function templateGraph({ initial, states, transitions }) {
  return {
    states: states.map(name => ({ name, initial: name === initial })),
    transitions: [...transitions].sort(byPair),
  };
}

function savedGraph(device) {
  const names = new Map(device.states.map(value => [value.id, value.name]));
  return {
    states: device.states.map(value => ({ name: value.name, initial: value.id === device.initialStateId })),
    transitions: device.transitions.map(value => ({
      name: value.name, from: names.get(value.sourceStateId), to: names.get(value.destinationStateId),
    })).sort(byPair),
  };
}

// The open device dialog's draft, read from the chips and the matrix cells.
async function draftGraph(page) {
  const chips = await stateChips(page);
  const names = new Map(chips.map(chip => [chip.id, chip.name]));
  const cells = await deviceDialog(page).cells.evaluateAll(buttons => buttons.map(button => ({
    from: button.dataset.from,
    to: button.dataset.to,
    names: [...button.querySelectorAll('.facility-transition-cell-name')].map(name => name.textContent),
  })));
  return {
    states: chips.map(({ name, initial }) => ({ name, initial })),
    transitions: cells.flatMap(cell => cell.names.map(name => ({
      name, from: names.get(cell.from), to: names.get(cell.to),
    }))).sort(byPair),
  };
}

// Polls the fixture session until a device matches, and returns it.
async function savedDevice(request, match) {
  let device;
  await expect.poll(async () => {
    device = (await fixtureSession(request)).facility.devices.find(match);
    return device !== undefined;
  }).toBe(true);
  return device;
}

function pageErrors(page) {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  return errors;
}

function templateOffer(page) {
  return {
    label: page.locator('#facilityDeviceTemplateLabel'),
    summary: page.locator('#facilityDeviceTemplateSummary'),
    status: page.locator('#facilityDeviceStatus'),
  };
}

async function openNewDevice(page) {
  await page.locator('#btnAddFacilityDevice').click();
  const dialog = deviceDialog(page);
  await expect(dialog.dialog).toBeVisible();
  await expect(dialog.name).toBeFocused();
  return dialog;
}

test('a named door takes three actions: «ДВЕРЬ», «ПРИМЕНИТЬ ШАБЛОН», «ГОТОВО» (SC-003, AS1, FR-009, FR-010)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await seedDemo(request);
  await openOverseer(page);
  await expect.poll(() => filterCount(page, 'all')).toBe(10);
  const dialog = await openNewDevice(page);
  const offer = templateOffer(page);
  await expect(dialog.template).toBeHidden();
  await dialog.name.fill('Шлюз теплицы');

  // 1. Pick «ДВЕРЬ». The offer names the template, and nothing is added yet (FR-010).
  await dialog.kind.selectOption({ label: 'ДВЕРЬ' });
  await expect(dialog.template).toBeVisible();
  await expect(offer.label).toHaveText('ШАБЛОН: ДВЕРЬ');
  await expect(dialog.templateSummary).toHaveText('Заперта, Открыта · Открыть, Запереть');
  await expect(dialog.applyTemplate).toHaveText('ПРИМЕНИТЬ ШАБЛОН');
  await expect(dialog.applyTemplate).not.toHaveAttribute('aria-disabled', 'true');
  await expect(dialog.chips).toHaveCount(0);
  await expect(offer.status).toHaveText('');

  // 2. Press «ПРИМЕНИТЬ ШАБЛОН»: both states, the star, and both transitions.
  await dialog.applyTemplate.click();
  await expect(offer.status).toHaveText(APPLIED(2, 2));
  expect(await draftGraph(page)).toEqual(templateGraph(DOOR));
  await expect(dialog.chips.first()).toHaveAttribute('aria-label', 'Заперта, начальное состояние');
  expect(await blockedReason(dialog.applyTemplate)).toBe(NOTHING_TO_ADD);

  // 3. Press «ГОТОВО».
  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  const saved = await savedDevice(request, device => device.name === 'Шлюз теплицы');
  await expectSaved(page);
  expect(saved).toMatchObject({
    id: 'device-shlyuz-teplitsy', kind: 'door', initialStateId: 'zaperta', currentStateId: 'zaperta',
  });
  expect(savedGraph(saved)).toEqual(templateGraph(DOOR));
  // Template IDs come from the names, as for typed ones.
  expect(saved.states.map(value => value.id)).toEqual(['zaperta', 'otkryta']);
  expect(saved.transitions.map(value => value.id)).toEqual(['otkryt', 'zaperet']);
  // The template itself is never stored.
  expect(JSON.stringify(saved)).not.toMatch(/template|шаблон/i);
  await expect(deviceList(page).row('device-shlyuz-teplitsy')).toBeVisible();
  await expect.poll(() => filterCount(page, 'all')).toBe(11);

  // Apply and «ГОТОВО» are one history entry.
  await expect(page.locator('#btnUndoReason')).toHaveText('ОТМЕНИТЬ: Устройство «Шлюз теплицы»');
  await undo(page);
  await expect.poll(async () => (await fixtureSession(request)).facility.devices
    .some(device => device.id === 'device-shlyuz-teplitsy')).toBe(false);
  await expect.poll(() => filterCount(page, 'all')).toBe(10);
  expect(errors).toEqual([]);
});

test('every type except «ДРУГОЙ» names its template before applying it, then saves what it named (FR-009, FR-010, independent test)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await seedDemo(request);
  await openOverseer(page);
  const offer = templateOffer(page);

  for (const [index, value] of TEMPLATES.entries()) {
    const name = `Учебное устройство ${index + 1}`;
    const dialog = await openNewDevice(page);
    await dialog.name.fill(name);
    await dialog.kind.selectOption(value.kind);
    await expect(dialog.template, value.kind).toBeVisible();
    await expect(offer.label, value.kind).toHaveText(`ШАБЛОН: ${value.label}`);
    await expect(dialog.templateSummary, value.kind).toHaveText(summary(value));
    await expect(dialog.chips, value.kind).toHaveCount(0);

    await dialog.applyTemplate.click();
    await expect(offer.status, value.kind).toHaveText(APPLIED(2, 2));
    expect(await draftGraph(page), value.kind).toEqual(templateGraph(value));
    expect(await blockedReason(dialog.applyTemplate), value.kind).toBe(NOTHING_TO_ADD);

    await dialog.save.click();
    await expect(dialog.dialog).toBeHidden();
    const saved = await savedDevice(request, device => device.name === name);
    await expectSaved(page);
    expect(saved.kind, value.kind).toBe(value.kind);
    expect(savedGraph(saved), value.kind).toEqual(templateGraph(value));
    // At least two states, a starting state, and a way back for every transition (FR-009).
    expect(saved.states.length, value.kind).toBeGreaterThanOrEqual(2);
    expect(saved.states.map(state => state.id), value.kind).toContain(saved.initialStateId);
    for (const transition of saved.transitions) {
      expect(saved.transitions.some(other => other.sourceStateId === transition.destinationStateId
        && other.destinationStateId === transition.sourceStateId), `${value.kind}: ${transition.name}`).toBe(true);
    }
  }
  await expect.poll(() => filterCount(page, 'all')).toBe(10 + TEMPLATES.length);
  expect(errors).toEqual([]);
});

test('«ДРУГОЙ» and an unchosen type offer no template (AS3, FR-009)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  const dialog = await openNewDevice(page);
  await expect(dialog.kind).toHaveValue('');
  await expect(dialog.template).toBeHidden();
  await dialog.kind.selectOption({ label: 'ДРУГОЙ' });
  await expect(dialog.template).toBeHidden();
  await dialog.kind.selectOption({ label: 'ДВЕРЬ' });
  await expect(dialog.template).toBeVisible();
  await dialog.kind.selectOption({ label: 'ДРУГОЙ' });
  await expect(dialog.template).toBeHidden();
  await expect(dialog.applyTemplate).toBeHidden();
  await expect(dialog.chips).toHaveCount(0);
  await dialog.cancel.click();
  await expect(dialog.dialog).toBeHidden();

  // A saved «ДРУГОЙ» device offers none either.
  await openDeviceDialog(page, 'water-purifier-76');
  await expect(dialog.kind).toHaveValue('custom');
  await expect(dialog.template).toBeHidden();
});

test('applying adds only what is missing, renames and removes nothing, and keeps the star (AS2, FR-011)', async ({ page, request }) => {
  const errors = pageErrors(page);
  const lock = {
    id: 'greenhouse-lock', name: 'Шлюз теплицы', kind: 'door',
    initialStateId: 'quarantine', currentStateId: 'quarantine',
    states: [{ id: 'quarantine', name: 'Карантин' }, { id: 'sealed', name: 'Заперта' }],
    transitions: [
      { id: 'declare-quarantine', name: 'Объявить карантин', sourceStateId: 'sealed', destinationStateId: 'quarantine' },
      // The template's name between another pair does not count as present.
      { id: 'zaperet', name: 'Запереть', sourceStateId: 'quarantine', destinationStateId: 'sealed' },
    ],
  };
  const demo = await demoSession();
  demo.facility.devices.push(lock);
  await seedDemo(request);
  await seedSession(request, demo);
  await openOverseer(page);
  const offer = templateOffer(page);

  const dialog = await openDeviceDialog(page, lock.id);
  await expect(offer.label).toHaveText('ШАБЛОН: ДВЕРЬ');
  await expect(dialog.templateSummary).toHaveText(summary(DOOR));
  expect(await draftGraph(page)).toEqual(savedGraph(lock));
  await dialog.applyTemplate.click();
  await expect(offer.status).toHaveText(APPLIED(1, 2));
  const added = [
    { name: 'Открыть', from: 'Заперта', to: 'Открыта' },
    { name: 'Запереть', from: 'Открыта', to: 'Заперта' },
  ];
  expect(await draftGraph(page)).toEqual({
    states: [...savedGraph(lock).states, { name: 'Открыта', initial: false }],
    transitions: [...savedGraph(lock).transitions, ...added].sort(byPair),
  });
  expect(await blockedReason(dialog.applyTemplate)).toBe(NOTHING_TO_ADD);

  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  const saved = await savedDevice(request, device => device.id === lock.id && device.states.length === 3);
  await expectSaved(page);
  expect(saved).toMatchObject({ name: lock.name, initialStateId: 'quarantine', currentStateId: 'quarantine' });
  expect(saved.states.slice(0, 2)).toEqual(lock.states);
  expect(saved.transitions.slice(0, 2)).toMatchObject(lock.transitions);
  expect(saved.transitions).toHaveLength(4);
  const ids = [...saved.states, ...saved.transitions].map(value => value.id);
  expect(new Set(ids).size).toBe(ids.length);

  // The demo door's renamed transitions stay, and the template's join them in their cells.
  const door = (await fixtureSession(request)).facility.devices.find(device => device.id === 'vault-door-main');
  await openDeviceDialog(page, door.id);
  const before = await draftGraph(page);
  expect(before).toEqual(savedGraph(door));
  await dialog.applyTemplate.click();
  await expect(offer.status).toHaveText(APPLIED(0, 2));
  expect(await draftGraph(page)).toEqual({
    states: before.states,
    transitions: [...before.transitions, ...DOOR.transitions].sort(byPair),
  });
  const pair = DOOR.transitions.map(({ from, to }) => [from, to]);
  const chips = new Map((await stateChips(page)).map(chip => [chip.name, chip.id]));
  for (const [from, to] of pair) {
    await expect(dialog.cell(chips.get(from), chips.get(to))).toHaveAttribute('data-count', '2');
  }
  // Applying stores nothing until «ГОТОВО».
  await dialog.cancel.click();
  await expect(dialog.dialog).toBeHidden();
  expect((await fixtureSession(request)).facility.devices.find(device => device.id === door.id)).toEqual(door);
  expect(errors).toEqual([]);
});

test('a name that differs only in case counts as present (FR-011, edge case)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await seedDemo(request);
  await openOverseer(page);
  const offer = templateOffer(page);
  const dialog = await openNewDevice(page);
  await dialog.name.fill('Шлюз склада');
  for (const name of ['заперта', 'ОТКРЫТА']) {
    await dialog.addState.click();
    await expect(dialog.stateEditor).toBeVisible();
    await dialog.stateName.fill(name);
  }
  await dialog.kind.selectOption({ label: 'ДВЕРЬ' });
  await expect(dialog.applyTemplate).not.toHaveAttribute('aria-disabled', 'true');

  // The empty cell suggests the template's name for the pair, which the author retypes in upper case.
  const [locked, open] = await stateChips(page);
  await dialog.cell(locked.id, open.id).click();
  await expect(dialog.panel).toBeVisible();
  const transitionName = dialog.panel.locator('.facility-transition-row:not([hidden]) input.facility-transition-name');
  await expect(transitionName).toHaveValue('Открыть');
  await transitionName.fill('ОТКРЫТЬ');

  await dialog.applyTemplate.click();
  await expect(offer.status).toHaveText(APPLIED(0, 1));
  const expected = {
    states: [{ name: 'заперта', initial: true }, { name: 'ОТКРЫТА', initial: false }],
    transitions: [
      { name: 'ОТКРЫТЬ', from: 'заперта', to: 'ОТКРЫТА' },
      { name: 'Запереть', from: 'ОТКРЫТА', to: 'заперта' },
    ].sort(byPair),
  };
  expect(await draftGraph(page)).toEqual(expected);
  expect(await blockedReason(dialog.applyTemplate)).toBe(NOTHING_TO_ADD);

  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  const saved = await savedDevice(request, device => device.name === 'Шлюз склада');
  await expectSaved(page);
  expect(savedGraph(saved)).toEqual(expected);
  expect(errors).toEqual([]);
});

test('changing the type changes only the offer, never the states or transitions (AS4, FR-012)', async ({ page, request }) => {
  const errors = pageErrors(page);
  await seedDemo(request);
  const turret = (await fixtureSession(request)).facility.devices.find(device => device.id === 'security-turret');
  await openOverseer(page);
  const offer = templateOffer(page);
  const dialog = await openDeviceDialog(page, turret.id);
  const graph = savedGraph(turret);
  expect(await draftGraph(page)).toEqual(graph);

  // The turret already has its whole template, so the press does nothing.
  await expect(offer.label).toHaveText('ШАБЛОН: ТУРЕЛЬ');
  expect(await blockedReason(dialog.applyTemplate)).toBe(NOTHING_TO_ADD);
  await dialog.applyTemplate.dispatchEvent('click');
  await expect(offer.status).toHaveText('');
  expect(await draftGraph(page)).toEqual(graph);

  // A new type offers its own template and applies nothing by itself.
  await dialog.kind.selectOption({ label: 'ЛИФТ' });
  await expect(offer.label).toHaveText('ШАБЛОН: ЛИФТ');
  await expect(dialog.templateSummary).toHaveText(summary(ELEVATOR));
  await expect(dialog.applyTemplate).not.toHaveAttribute('aria-disabled', 'true');
  await expect(offer.status).toHaveText('');
  expect(await draftGraph(page)).toEqual(graph);
  await dialog.kind.selectOption({ label: 'ДРУГОЙ' });
  await expect(dialog.template).toBeHidden();
  expect(await draftGraph(page)).toEqual(graph);

  await dialog.kind.selectOption({ label: 'ЛИФТ' });
  await dialog.save.click();
  await expect(dialog.dialog).toBeHidden();
  const saved = await savedDevice(request, device => device.id === turret.id && device.kind === 'elevator');
  await expectSaved(page);
  expect(saved).toMatchObject({
    name: turret.name, initialStateId: turret.initialStateId, currentStateId: turret.currentStateId,
  });
  expect(saved.states).toEqual(turret.states);
  expect(saved.transitions).toMatchObject(turret.transitions);
  expect(savedGraph(saved)).toEqual(graph);
  expect(errors).toEqual([]);
});
