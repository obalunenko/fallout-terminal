import { readFileSync } from 'node:fs';
import { readFile } from 'node:fs/promises';

import { expect, test } from '@playwright/test';

import { switchTab } from './helpers/authoring-bar.mjs';
import { expectRussianMessage, renderedError, RULE_TABS } from './helpers/facility-editors.mjs';
import { selectDevice } from './helpers/facility-play.mjs';
import { FIXTURE, facilityState, fixtureSession, openOverseer, seedDemo, seedSession } from './helpers/overseer-forms.mjs';

// User Story 6: one name for each thing, and links that say where they lead
// (specs/041-facility-editors-language/spec.md, contracts/overseer-ui.md,
// docs/overseer-glossary.md).

const GLOSSARY_URL = new URL('../../docs/overseer-glossary.md', import.meta.url);
const DEMO = JSON.parse(readFileSync(new URL('../../sessions/demo.json', import.meta.url), 'utf8'));
// The served Overseer sources. The fixture page adds an import map line after
// <head>; it is dropped so hits keep their source line numbers.
const SOURCES = [
  ['index.html', `${FIXTURE}/overseer`, text => text.replace(/(?<=<head>)\n<script type="importmap">[^\n]*<\/script>/, '')],
  ['overseer.js', '/__fixture/overseer.js', text => text],
  ['desktop-api.js', '/__fixture/desktop-api.js', text => text],
];
const EDIT_DEVICE = 'ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ';
const DEVELOPER_DETAILS = 'подробности для разработчика';
// The 040 guide labels that FR-036 replaces with EDIT_DEVICE.
const OLD_DEVICE_EDIT_LABELS = [
  'НАСТРОИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', 'ДОБАВИТЬ ПЕРЕХОД В УСТРОЙСТВО',
  'ИЗМЕНИТЬ ПЕРЕХОДЫ И ПРЕДУСЛОВИЯ', 'ДОБАВИТЬ ПЕРЕХОД ДЛЯ ПРОГРАММЫ',
];
const DESTINATION_LABELS = {
  device: 'К УСТРОЙСТВУ',
  condition: 'К НЕИСПРАВНОСТИ',
  program: 'К ПРОГРАММЕ',
  command: 'К КОМАНДЕ',
  rule: 'К ПРАВИЛУ ОТОБРАЖЕНИЯ',
};
const RULE_PROPERTIES = {
  'name-variant': 'menu-name',
  'entry-content-variant': 'entry-content',
  visibility: 'visibility',
  availability: 'availability',
};

// fencedBlock returns the non-empty lines of the glossary's ```name block.
function fencedBlock(markdown, name) {
  const block = new RegExp(`^\`\`\`${name}\\n([\\s\\S]*?)^\`\`\`$`, 'm').exec(markdown);
  expect(block, `${name} block in the glossary`).not.toBeNull();
  const lines = block[1].split('\n').filter(line => line !== '');
  expect(lines.length, `${name} lines`).toBeGreaterThan(0);
  return lines;
}

// readVocabulary compiles the glossary's machine part as the glossary
// specifies: each banned line with the flags iu (plus g to list every hit),
// and each allowed phrase as a literal that counts only where it stands
// alone, longest phrase first.
async function readVocabulary() {
  const markdown = await readFile(GLOSSARY_URL, 'utf8');
  const banned = fencedBlock(markdown, 'vocabulary-banned').map(source => {
    expect(source, 'banned patterns never use \\b').not.toContain('\\b');
    return new RegExp(source, 'giu');
  });
  const allowed = fencedBlock(markdown, 'vocabulary-allowed')
    .toSorted((left, right) => right.length - left.length)
    .map(phrase => new RegExp(`(?<![\\p{L}\\p{N}\\s])${RegExp.escape(phrase)}(?![\\p{L}\\p{N}\\s])`, 'gu'));
  return { banned, allowed };
}

// vocabularyHits removes the allowed phrases from text, then reports every
// banned match as «<file>:<line>: <match>».
function vocabularyHits(file, text, { banned, allowed }) {
  const scanned = allowed.reduce((value, phrase) => value.replace(phrase, ''), text);
  const hits = [];
  for (const pattern of banned) {
    for (const match of scanned.matchAll(pattern)) {
      hits.push(`${file}:${scanned.slice(0, match.index).split('\n').length}: ${match[0]}`);
    }
  }
  return hits;
}

test('the vocabulary check finds no banned wording in the served Overseer sources (AS1, FR-035, SC-007)', async ({ request }) => {
  const vocabulary = await readVocabulary();
  const hits = [];
  for (const [file, path, served] of SOURCES) {
    const response = await request.get(path);
    expect(response.ok(), path).toBe(true);
    const text = served(await response.text());
    expect(text.length, path).toBeGreaterThan(1000);
    hits.push(...vocabularyHits(file, text, vocabulary));
  }
  expect(hits).toEqual([]);
});

test('the vocabulary check rejects the old wordings and accepts «предусловие», the tab, and the heading (FR-035)', async () => {
  const vocabulary = await readVocabulary();
  // Wordings this feature renamed or removed, and the allowed phrases where
  // they do not stand alone.
  const rejected = [
    'УСТРОЙСТВО ОБЪЕКТА', 'ЗАВИСИМОСТИ ОБЪЕКТА', 'ПРИВЯЗКА ОБЪЕКТА', 'ПРЕДПРОСМОТР ОБЪЕКТА',
    'Сведения об объекте', 'Устройства объекта', 'Состояние объектов', 'ОЖИДАЕМАЯ РЕВИЗИЯ ОБЪЕКТА',
    'СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ. ПОВТОРИТЕ ЗАПРОС', 'ДЕЙСТВИЕ ОБЪЕКТА НАСТРОЕНО НЕКОРРЕКТНО',
    'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ', 'Редактирование диагностического условия', 'ДИАГНОСТИЧЕСКИЕ УСЛОВИЯ',
    'ДОБАВИТЬ УСЛОВИЕ', 'ОБЛАСТЬ УСЛОВИЯ', 'УКАЖИТЕ НАЗВАНИЕ УСЛОВИЯ', 'ВОССТАНОВИТЬ УСЛОВИЕ',
    'УСЛОВИЕ ВОССТАНОВЛЕНО', 'связанные условия вернутся', 'ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ',
    'Восстановление оператором', 'ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ НЕ НАСТРОЕНО',
    'ОБЪЕКТЫ УБЕЖИЩА', '// the binding dialog that ОБЪЕКТЫ uses', '«объекты»',
  ];
  for (const sample of rejected) {
    expect(vocabularyHits('sample', sample, vocabulary).length, sample).toBeGreaterThan(0);
  }
  // Every banned line bites on at least one old wording.
  for (const pattern of vocabulary.banned) {
    expect(rejected.some(sample => sample.match(pattern)), String(pattern)).toBe(true);
  }
  const accepted = [
    '<button id="facilityWorkspaceTab" type="button" role="tab">ОБЪЕКТЫ</button>',
    '<h1>ОБЪЕКТЫ И СОСТОЯНИЯ</h1>', "openOverseer(page, 'ОБЪЕКТЫ')", '// the «ОБЪЕКТЫ И СОСТОЯНИЯ» heading',
    'предусловие', '+ ПРЕДУСЛОВИЕ', 'ПРЕДУСЛОВИЯ', 'БЕЗ ПРЕДУСЛОВИЙ', '1 предусловие · снимает 1 неисправность',
    'УСТРОЙСТВО', 'НЕИСПРАВНОСТЬ', 'Сведения о локации', 'ПРАВИЛО ОТОБРАЖЕНИЯ', 'Смотритель вручную',
    DEVELOPER_DETAILS, 'DiagnosticCondition', 'condition.privateOverseerAction',
  ];
  for (const sample of accepted) expect(vocabularyHits('sample', sample, vocabulary), sample).toEqual([]);
  expect(vocabularyHits('sample.js', 'const ok = 1;\nbutton.textContent = \'ДОБАВИТЬ УСЛОВИЕ\';', vocabulary))
    .toEqual(['sample.js:2: УСЛОВИЕ']);
});

function nodeTrail(node, id, names = []) {
  if (node.id === id) return { node, names };
  for (const child of node.children ?? []) {
    const found = nodeTrail(child, id, [...names, child.name]);
    if (found) return found;
  }
  return null;
}

// blockOwner finds the record that holds blockID and the block's index.
function blockOwner(node, blockID) {
  const index = node.blocks?.findIndex(block => block.id === blockID) ?? -1;
  if (index >= 0) return { entry: node, index };
  for (const child of node.children ?? []) {
    const found = blockOwner(child, blockID);
    if (found) return found;
  }
  return null;
}

// expectedLink words one dependency of the inspected device from session as
// the contract's «Dependency dialog» table does. source is the name the
// button's aria-label names; open says where its button leads.
function expectedLink(dependency, session) {
  const { devices, conditions, recoveryPrograms } = session.facility;
  const device = id => devices.find(value => value.id === id);
  const stateName = (owner, id) => owner.states.find(value => value.id === id).name;
  const inspected = device(dependency.targetId);
  const equality = when => `${inspected.name} = ${stateName(inspected, when.stateId)}`;
  const transitionName = request => device(request.deviceId).transitions
    .find(value => value.id === request.transitionId).name;
  const index = Number(/\[(\d+)\]/.exec(dependency.property)?.[1] ?? 0);
  const terminal = session.terminals.find(value => value.id === dependency.terminalId);
  const place = id => [terminal.name, ...nodeTrail(terminal.root, id).names].join(' › ');
  const link = (destination, sentence, where, source, open) => ({ destination, sentence, place: where, source, open });
  switch (dependency.kind) {
    case 'transition-precondition': {
      const owner = devices.find(value => value.transitions?.some(item => `${value.id}/${item.id}` === dependency.sourceId));
      const transition = owner.transitions.find(item => `${owner.id}/${item.id}` === dependency.sourceId);
      return link('device', `Переход «${transition.name}» устройства «${owner.name}» требует: ${equality(transition.preconditions[index])}.`,
        `${owner.name} › ${transition.name}`, transition.name, { owner, transition });
    }
    case 'recovery-reference': {
      const fault = conditions.find(value => value.id === dependency.sourceId);
      return link('condition', `Неисправность «${fault.name}» устраняется переходом «${transitionName(fault.recovery[index].transition)}».`,
        fault.name, fault.name, { fault });
    }
    case 'diagnostic-scope': {
      const fault = conditions.find(value => value.id === dependency.sourceId);
      return link('condition', `Неисправность «${fault.name}» действует на это устройство.`, fault.name, fault.name, { fault });
    }
    case 'recovery-program-transition': {
      const program = recoveryPrograms.find(value => value.id === dependency.sourceId);
      return link('program', `Программа восстановления «${program.name}» запускает переход «${transitionName(program.transitions[index])}».`,
        program.name, program.name, { program });
    }
    case 'command-action': {
      const { node } = nodeTrail(terminal.root, dependency.sourceId);
      const request = node.stateChange.facilityAction.transitions.transitions[index];
      return link('command', `Команда «${node.name}» запускает переход «${transitionName(request)}».`,
        place(node.id), node.name, { terminal, node });
    }
    case 'entry-content-variant': {
      const { entry, index: block } = blockOwner(terminal.root, dependency.sourceId);
      const variant = entry.blocks[block].facilityTextVariants[index];
      return link('rule', `Текст записи «${entry.name}» (блок ${block + 1}) меняется, когда ${equality(variant.when)}.`,
        `${place(entry.id)} › блок ${block + 1}`, entry.name, { terminal, node: entry, blockId: dependency.sourceId, variant });
    }
    default: {
      const { node } = nodeTrail(terminal.root, dependency.sourceId);
      const variant = {
        'name-variant': () => node.facilityNameVariants[index],
        visibility: () => ({ when: node.visibleWhen }),
        availability: () => ({ when: node.availableWhen }),
      }[dependency.kind]();
      const sentence = {
        'name-variant': `Название «${node.name}» в меню меняется, когда ${equality(variant.when)}.`,
        visibility: `Элемент «${node.name}» виден, только когда ${equality(variant.when)}.`,
        availability: `Команду «${node.name}» можно запросить, только когда ${equality(variant.when)}.`,
      }[dependency.kind];
      return link('rule', sentence, place(node.id), node.name, { terminal, node, variant });
    }
  }
}

// openDependencies opens «ЗАВИСИМОСТИ» for deviceID and returns the report
// the dialog renders.
async function openDependencies(page, deviceID) {
  await selectDevice(page, deviceID);
  const [response] = await Promise.all([
    page.waitForResponse(answer => answer.url().endsWith(`${FIXTURE}/inspect`)),
    page.locator('#btnInspectFacilityDependencies').click(),
  ]);
  const { report } = await response.json();
  const dialog = page.locator('#facilityDependencyDialog');
  await expect(dialog).toBeVisible();
  await expect(page.locator('#facilityDependencyDialogTitle')).toHaveText('ЗАВИСИМОСТИ УСТРОЙСТВА');
  const dependencies = report?.dependencies ?? [];
  await expect(page.locator('#facilityDependencyStatus'))
    .toHaveText(dependencies.length ? `НАЙДЕНО ССЫЛОК: ${dependencies.length}` : 'ССЫЛКИ НЕ НАЙДЕНЫ');
  return { dialog, rows: page.locator('#facilityDependencyList .facility-dependency-row'), dependencies };
}

// expectDestination checks that a followed button opened link's place, then
// leaves it without saving.
async function expectDestination(page, link) {
  const cancel = dialog => page.locator(`#${dialog}`).locator('.facility-dialog-actions')
    .getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
  switch (link.destination) {
    case 'device': {
      await expect(page.locator('#facilityDeviceDialog')).toBeVisible();
      await expect(page.locator('#facilityDeviceID')).toHaveValue(link.open.owner.id);
      await expect(page.locator('#facilityTransitionPanel')).toBeVisible();
      const name = page.locator('#facilityTransitionPanel .facility-transition-row:not([hidden]) .facility-transition-name');
      await expect(name).toHaveValue(link.open.transition.name);
      await expect(name).toBeFocused();
      await cancel('facilityDeviceDialog');
      return;
    }
    case 'condition':
      await expect(page.locator('#facilityConditionDialog')).toBeVisible();
      await expect(page.locator('#facilityConditionID')).toHaveValue(link.open.fault.id);
      await expect(page.locator('#facilityConditionName')).toHaveValue(link.open.fault.name);
      await cancel('facilityConditionDialog');
      return;
    case 'program':
      await expect(page.locator('#facilityRecoveryProgramDialog')).toBeVisible();
      await expect(page.locator('#facilityRecoveryProgramID')).toHaveValue(link.open.program.id);
      await expect(page.locator('#facilityRecoveryProgramName')).toHaveValue(link.open.program.name);
      await cancel('facilityRecoveryProgramDialog');
      return;
    case 'command':
      await expect(page.getByRole('tab', { name: 'ТЕРМИНАЛЫ', exact: true })).toHaveAttribute('aria-selected', 'true');
      await expect(page.locator('#fldName')).toHaveValue(link.open.node.name);
      await expect(page.locator('#btnEditCommandFacilityAction')).toBeFocused();
      await switchTab(page, 'ОБЪЕКТЫ');
      return;
    default: {
      const { terminal, node, blockId, variant } = link.open;
      await expect(page.locator('#facilityBindingDialog')).toBeVisible();
      await expect(page.locator(RULE_TABS[link.property])).toHaveAttribute('aria-selected', 'true');
      const where = page.locator('#facilityBindingNode option:checked');
      await expect(where).toHaveAttribute('data-terminal-id', terminal.id);
      await expect(where).toHaveAttribute('data-node-id', node.id);
      await expect(page.locator('#facilityBindingDevice')).toHaveValue(variant.when.deviceId);
      await expect(page.locator('#facilityBindingState')).toHaveValue(variant.when.stateId);
      if (blockId) await expect(page.locator('#facilityBindingBlock')).toHaveValue(blockId);
      if (variant.text !== undefined) await expect(page.locator('#facilityBindingText')).toHaveValue(variant.text);
      await cancel('facilityBindingDialog');
    }
  }
}

for (const { id: deviceID, name: deviceName } of DEMO.facility.devices) {
  test(`every dependency of «${deviceName}» reads as a sentence, hides its technical path, and its button leads to its destination (AS2, FR-037)`, async ({ page, request }) => {
    // Each button is followed from a freshly opened dialog.
    test.slow();
    await seedDemo(request);
    const session = await fixtureSession(request);
    await openOverseer(page);
    const { rows, dependencies } = await openDependencies(page, deviceID);
    await expect(page.locator('#facilityDependencyDialogDescription'))
      .toHaveText(`Что в локации зависит от устройства «${deviceName}».`);
    // Every demo device is named by at least one display rule.
    expect(dependencies.length).toBeGreaterThan(0);
    await expect(rows).toHaveCount(dependencies.length);
    const links = dependencies.map(dependency => ({
      ...expectedLink(dependency, session), property: RULE_PROPERTIES[dependency.kind],
    }));

    for (const [index, dependency] of dependencies.entries()) {
      const row = rows.nth(index);
      const link = links[index];
      const context = `${dependency.kind} ${dependency.sourceId}`;
      await expect(row, context).toHaveAttribute('role', 'listitem');
      await expect(row, context).toHaveAttribute('data-kind', dependency.kind);
      await expect(row, context).toHaveAttribute('data-destination', link.destination);
      await expect(row.locator('.facility-dependency-sentence'), context).toHaveText(link.sentence);
      await expect(row.locator('.facility-dependency-place'), context).toHaveText(link.place);
      const button = row.locator('button.facility-dependency-navigation');
      await expect(button, context).toHaveText(DESTINATION_LABELS[link.destination]);
      await expect(button, context).toHaveAttribute('aria-label', `${DESTINATION_LABELS[link.destination]}: ${link.source}`);
      await expect(row.locator('.facility-dependency-kind, .facility-dependency-property'), context).toHaveCount(0);
      await expect(row.getByRole('button', { name: 'ОТКРЫТЬ', exact: true }), context).toHaveCount(0);

      // No technical path is visible until the disclosure opens.
      const details = row.locator('details.facility-technical-details');
      const path = `${dependency.kind} · ${dependency.sourceId} → ${dependency.targetId} · ${dependency.property}`
        + (dependency.terminalId ? ` · терминал ${dependency.terminalId}` : '');
      await expect(details, context).not.toHaveAttribute('open', '');
      await expect(details.locator('summary'), context).toHaveText(DEVELOPER_DETAILS);
      await expect(details.locator('p'), context).toBeHidden();
      const visible = await row.evaluate(element => element.innerText);
      for (const raw of [dependency.kind, dependency.property, dependency.sourceId, dependency.targetId]) {
        expect(visible, `${context}: ${raw}`).not.toContain(raw);
      }
      expectRussianMessage(link.sentence);
      await details.locator('summary').click();
      await expect(details.locator('p'), context).toBeVisible();
      await expect(details.locator('p'), context).toHaveText(path);
    }

    await page.locator('#btnCloseFacilityDependencies').click();
    await expect(page.locator('#facilityDependencyDialog')).toBeHidden();

    // Every destination button opens its place.
    for (const [index, link] of links.entries()) {
      const { dialog, rows: current } = await openDependencies(page, deviceID);
      await current.nth(index).locator('button.facility-dependency-navigation').click();
      await expect(dialog, link.sentence).toBeHidden();
      await expectDestination(page, link);
    }
  });
}

test('the door\'s links name the terminal and the command, and each button names its destination (AS2, FR-037)', async ({ page, request }) => {
  await seedDemo(request);
  await openOverseer(page);
  const { rows } = await openDependencies(page, 'vault-door-main');
  const command = page.locator('#facilityDependencyList .facility-dependency-row[data-destination="command"]');
  await expect(command).toHaveCount(1);
  await expect(command.locator('.facility-dependency-sentence'))
    .toHaveText('Команда «Открыть выход и подать сигнал» запускает переход «Открыть гермодверь».');
  await expect(command.locator('.facility-dependency-place'))
    .toHaveText('Терминал смотрителя — Убежище 76 › ПРОТОКОЛ ВЫХОДА › Открыть выход и подать сигнал');
  await expect(command.getByRole('button', { name: 'К КОМАНДЕ: Открыть выход и подать сигнал', exact: true }))
    .toHaveText('К КОМАНДЕ');
  const fault = page.locator('#facilityDependencyList .facility-dependency-row[data-kind="diagnostic-scope"]');
  await expect(fault.locator('.facility-dependency-sentence'))
    .toHaveText('Неисправность «Контроллер гермодвери не отвечает» действует на это устройство.');
  await expect(fault.locator('button.facility-dependency-navigation')).toHaveText('К НЕИСПРАВНОСТИ');
  // Each button reads one of the five destinations, and no path shows.
  const labels = await rows.locator('button.facility-dependency-navigation').allTextContents();
  expect(labels.length).toBeGreaterThan(0);
  for (const label of labels) expect(Object.values(DESTINATION_LABELS)).toContain(label);
  expect(new Set(labels)).toEqual(new Set(['К НЕИСПРАВНОСТИ', 'К КОМАНДЕ', 'К ПРАВИЛУ ОТОБРАЖЕНИЯ']));
  await expect(page.locator('#facilityDependencyList details.facility-technical-details[open]')).toHaveCount(0);
  expect(await page.locator('#facilityDependencyList').evaluate(element => element.innerText)).not.toMatch(/vault-door-main|n_cmd_/);
});

test('a facility validation problem names the entity by name in Russian and keeps kinds and IDs under «подробности для разработчика» (AS3, FR-038)', async ({ page, request }) => {
  const demo = await seedDemo(request);
  await openOverseer(page);
  const door = demo.facility.devices.find(device => device.id === 'vault-door-main');
  const fault = demo.facility.conditions.find(condition => condition.id === 'door-offline');
  const program = demo.facility.recoveryPrograms.find(value => value.id === 'air-recovery-76');
  const cases = [
    [{ entityKind: 'device', entityId: door.id }, `устройство «${door.name}»`],
    [{ entityKind: 'device-state', entityId: door.states[1].id, referenceKind: 'owner', referenceId: door.id },
      `состояние «${door.states[1].name}» устройства «${door.name}»`],
    [{ entityKind: 'device-transition', entityId: door.transitions[0].id, referenceKind: 'owner', referenceId: door.id },
      `переход «${door.transitions[0].name}» устройства «${door.name}»`],
    [{ entityKind: 'condition', entityId: fault.id }, `неисправность «${fault.name}»`],
    [{ entityKind: 'recovery-program', entityId: program.id }, `программа восстановления «${program.name}»`],
  ];
  const conflict = 'Правка противоречит текущим данным локации. Проверьте связанные элементы и повторите.';
  const dialog = page.locator('#facilityDeviceDialog');
  for (const [issue, named] of cases) {
    // The session store rejects the next save with this issue.
    await page.route(`**${FIXTURE}/save-authoring`, async route => {
      const { correlationId } = route.request().postDataJSON();
      const state = await facilityState(request);
      await route.fulfill({ json: {
        ok: false, changed: false, correlationId, failure: 'conflict',
        issues: [{ code: 'conflict', ...issue }],
        sessionRevision: state.sessionRevision, authoringRevision: state.authoringRevision,
        previousFacilityRevision: state.facility.revision, resultingFacilityRevision: state.facility.revision,
        session: await fixtureSession(request),
      } });
    }, { times: 1 });
    await selectDevice(page, door.id);
    await page.locator('#btnEditFacilityDevice').click();
    await page.locator('#facilityDeviceName').fill('Гермодверь 76');
    await page.locator('#btnSaveFacilityDevice').click();

    const message = `${conflict} Затронуто: ${named}.`;
    const error = await renderedError(page.locator('#facilityDeviceError'));
    expect(error).toMatchObject({ area: 'facility', key: 'conflict', message, summary: DEVELOPER_DETAILS });
    expectRussianMessage(error.message);
    const raw = [`failure: conflict`, `${issue.entityKind} ${issue.entityId}`, issue.referenceId && `owner ${issue.referenceId}`]
      .filter(Boolean).join(' · ');
    expect(error.original).toBe(raw);
    // Only the message is visible; the kind and the IDs wait in the disclosure.
    const details = page.locator('#facilityDeviceError .overseer-error-details');
    await expect(details).not.toHaveAttribute('open', '');
    await expect(details.locator('.overseer-error-original')).toBeHidden();
    const visible = await page.locator('#facilityDeviceError').evaluate(element => element.innerText);
    for (const internal of [issue.entityKind, issue.entityId, 'conflict']) expect(visible, internal).not.toContain(internal);
    await expect(page.locator('#saveStatus')).toHaveText(`ОШИБКА СОХРАНЕНИЯ: ${message}`);
    await details.locator('summary').click();
    await expect(details.locator('.overseer-error-original')).toBeVisible();

    await dialog.locator('.facility-dialog-actions').getByRole('button', { name: 'ОТМЕНА', exact: true }).click();
    await expect(dialog).toBeHidden();
  }
  expect((await fixtureSession(request)).facility.devices.find(device => device.id === door.id).name).toBe(door.name);
});

// guideControls opens the guide on step and returns its action buttons' labels.
async function guideControls(page, step) {
  const guide = page.locator('#facilityStoryGuide');
  if (!await guide.evaluate(element => element.open)) await page.locator('#facilityStoryGuideSummary').click();
  await expect(guide).toHaveAttribute('open', '');
  await guide.locator(`[data-story-step="${step}"]`).click();
  return page.locator('#facilityStoryActions button').allTextContents();
}

test('every control that opens an existing device\'s dialog reads «ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ» (AS4, FR-036)', async ({ page, request }) => {
  // Every guide control of two devices on five steps is tried.
  test.slow();
  const demo = await seedDemo(request);
  // A device with no transitions shows the edit control on steps 2 and 4 too.
  demo.facility.devices.push({
    id: 'vocabulary-hatch', name: 'Люк без переходов', kind: 'door', initialStateId: 'closed', currentStateId: 'closed',
    states: [{ id: 'closed', name: 'Закрыт' }, { id: 'open', name: 'Открыт' }], transitions: [],
  });
  await seedSession(request, demo);
  await openOverseer(page);
  const before = await facilityState(request);
  const deviceDialog = page.locator('#facilityDeviceDialog');
  const openers = [];

  for (const deviceID of ['vault-door-main', 'vocabulary-hatch']) {
    await selectDevice(page, deviceID);
    await expect(page.locator('#btnEditFacilityDevice')).toHaveText(EDIT_DEVICE);
    await page.locator('#btnEditFacilityDevice').click();
    await expect(deviceDialog).toBeVisible();
    await expect(page.locator('#facilityDeviceID')).toHaveValue(deviceID);
    await page.locator('#btnCancelFacilityDevice').click();

    for (let step = 0; step < 5; step++) {
      const labels = await guideControls(page, step);
      let editors = 0;
      for (const [index, label] of labels.entries()) {
        if (label === 'ОТКРЫТЬ ТЕРМИНАЛЫ') continue;
        await page.locator('#facilityStoryActions button').nth(index).click();
        const existing = await deviceDialog.evaluate(element => element.open)
          && await page.locator('#facilityDeviceID').evaluate(input => input.readOnly);
        const context = `${deviceID}, step ${step + 1}: ${label}`;
        if (existing) {
          editors++;
          openers.push(label);
          expect(label, context).toBe(EDIT_DEVICE);
          await expect(page.locator('#facilityDeviceID'), context).toHaveValue(deviceID);
        } else {
          expect(label, context).not.toBe(EDIT_DEVICE);
        }
        await page.locator('dialog[open] .facility-dialog-actions')
          .getByRole('button', { name: /^(ОТМЕНА|ЗАКРЫТЬ)$/ }).click();
        await expect(page.locator('dialog.facility-dialog[open]'), context).toHaveCount(0);
        await guideControls(page, step);
      }
      expect(editors, `${deviceID}, step ${step + 1}`).toBeLessThanOrEqual(1);
    }
  }
  // Steps 1 and 2 of both devices and step 4 of the hatch offer the control.
  expect(openers).toHaveLength(5);
  for (const [, path] of SOURCES) {
    const text = await (await request.get(path)).text();
    for (const old of OLD_DEVICE_EDIT_LABELS) expect(text, `${old} in ${path}`).not.toContain(old);
  }
  expect((await facilityState(request)).saveCalls).toBe(before.saveCalls);
});
