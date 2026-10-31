import { expect, test } from '@playwright/test';
import { readFile } from 'node:fs/promises';

const DEMO_URL = new URL('../../sessions/demo.json', import.meta.url);
const INVENTORY_URL = new URL('../../sessions/demo-capability-paths.md', import.meta.url);
const PLAYER_CONFIG_URL = new URL('../../sessions/demo-players.json', import.meta.url);
const ROLE_FIXTURE = '/__fixture/facility-player-state';
const TOKEN_KEY = 'fallout-terminal.player-token';

async function loadDemoAssets() {
  const [demoRaw, inventory, playerConfigRaw] = await Promise.all([
    readFile(DEMO_URL, 'utf8'),
    readFile(INVENTORY_URL, 'utf8'),
    readFile(PLAYER_CONFIG_URL, 'utf8'),
  ]);
  return {
    demo: JSON.parse(demoRaw),
    demoRaw,
    inventory,
    playerConfig: JSON.parse(playerConfigRaw),
  };
}

function walkNodes(terminal) {
  const nodes = [];
  const visit = (node, route) => {
    const nextRoute = [...route, node.name];
    nodes.push({ node, route: nextRoute, terminal });
    for (const child of node.children ?? []) visit(child, nextRoute);
  };
  visit(terminal.root, []);
  return nodes;
}

function conditionEffectKind(effect) {
  return ['capabilityBlock', 'diagnosticPath', 'recordSubstitution', 'displayInstability']
    .find(kind => effect[kind] !== undefined);
}

function recoveryKind(recovery) {
  return ['transition', 'recoveryProgramId', 'privateOverseerAction']
    .find(kind => recovery[kind] !== undefined);
}

async function openParticipant(browser) {
  const context = await browser.newContext();
  await context.addInitScript(tokenKey => {
    localStorage.removeItem(tokenKey);
    HTMLMediaElement.prototype.play = () => Promise.resolve();
  }, TOKEN_KEY);
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
  if (await page.locator('#characterSelect').isVisible()) {
    await page.locator('#characterOptions button:not([disabled])').first().click();
  }
  return { context, page };
}

test('capability inventory resolves every player path and preserves terminal identity', async () => {
  const { demo, inventory, playerConfig } = await loadDemoAssets();
  expect(demo.version).toBe(1);
  expect(demo.playerConfig).toBe('demo-players.json');
  expect(playerConfig.version).toBe(1);
  expect(playerConfig.roster).toHaveLength(4);
  expect(demo.terminals.map(terminal => terminal.hackLevel).sort((left, right) => left - right))
    .toEqual([0, 1, 2, 3, 4, 5]);

  const terminalByID = new Map(demo.terminals.map(terminal => [terminal.id, terminal]));
  const groupByTerminalID = new Map();
  for (const group of demo.terminalGroups) {
    expect(group.terminalIds.length).toBeGreaterThan(0);
    expect(inventory).toContain(`\`${group.id}\``);
    for (const terminalID of group.terminalIds) {
      expect(terminalByID.has(terminalID)).toBe(true);
      expect(groupByTerminalID.has(terminalID)).toBe(false);
      groupByTerminalID.set(terminalID, group.id);
    }
  }
  expect(groupByTerminalID.size).toBe(demo.terminals.length);
  expect(demo.terminalGroups.find(group => group.id === 'vault-76-overseer-console')?.terminalIds)
    .toEqual(['t_demo1', 't_demo2']);
  for (const independentID of [
    'vault-76-greenhouse',
    'vault-76-freight-lift',
    'vault-76-outer-security',
    'atlas-76-relay',
  ]) {
    expect(demo.terminalGroups.find(group => group.id === independentID)?.terminalIds).toHaveLength(1);
  }

  const transitionEdges = new Map();
  for (const terminal of demo.terminals) {
    expect(inventory).toContain(`\`${terminal.id}\``);
    for (const { node, route } of walkNodes(terminal)) {
      if (node.id !== 'root') {
        expect(inventory, `inventory path for ${terminal.id}:${node.id}`).toContain(`\`${node.id}\``);
        expect(inventory, `inventory label for ${terminal.id}:${node.id}`).toContain(node.name);
      }
      expect(route.every(segment => segment.trim() !== '')).toBe(true);
      if (node.type === 'folder') expect(node.children?.length ?? 0).toBeGreaterThan(0);
      if (node.type === 'entry') {
        expect(Boolean(node.description?.trim()) || (node.blocks?.length ?? 0) > 0).toBe(true);
      }
      if (node.type !== 'command') continue;
      const configuredModes = [
        Boolean(node.stateChange),
        Boolean(node.terminalTransition),
        !node.stateChange && !node.terminalTransition,
      ].filter(Boolean);
      expect(configuredModes).toHaveLength(1);
      expect(Boolean(node.text?.trim()) || Boolean(node.terminalTransition)).toBe(true);
      if (!node.terminalTransition) continue;
      const targetID = node.terminalTransition.targetTerminalId;
      expect(terminalByID.has(targetID)).toBe(true);
      expect(groupByTerminalID.get(targetID)).toBe(groupByTerminalID.get(terminal.id));
      transitionEdges.set(`${terminal.id}:${targetID}`, node.id);
    }
  }
  expect(transitionEdges.get('t_demo1:t_demo2')).toBe('n_cmd_state_change_1');
  expect(transitionEdges.get('t_demo2:t_demo1')).toBe('n2_cmd_return');
  for (const role of ['Overseer', 'controller', 'observer']) expect(inventory).toContain(role);
  for (const escape of ['НАЗАД', 'Acknowledge', 'private Overseer recovery']) {
    expect(inventory).toContain(escape);
  }
});

test('inventory covers every facility mode, prerequisite, effect, and recovery shape', async () => {
  const { demo, inventory } = await loadDemoAssets();
  const facility = demo.facility;
  expect(facility).toBeDefined();

  expect(new Set(facility.devices.map(device => device.kind))).toEqual(new Set([
    'alarm',
    'custom',
    'door',
    'elevator',
    'network-segment',
    'power-grid',
    'reactor',
    'robot-pod',
    'turret',
    'ventilation',
  ]));
  expect(facility.devices.find(device => device.kind === 'custom')?.customKind).toBe('water-purifier');
  expect(facility.devices.filter(device => device.initialStateId !== device.currentStateId).length)
    .toBeGreaterThan(0);

  const deviceByID = new Map(facility.devices.map(device => [device.id, device]));
  const conditionByID = new Map(facility.conditions.map(condition => [condition.id, condition]));
  const recoveryProgramByID = new Map(facility.recoveryPrograms.map(program => [program.id, program]));
  const allNodes = demo.terminals.flatMap(walkNodes);
  const nodeByTerminalAndID = new Map(allNodes.map(({ terminal, node }) => [`${terminal.id}:${node.id}`, node]));
  const blockByTerminalAndID = new Map(allNodes.flatMap(({ terminal, node }) =>
    (node.blocks ?? []).map(block => [`${terminal.id}:${block.id}`, block])));

  for (const device of facility.devices) {
    expect(inventory).toContain(`\`${device.id}\``);
    const stateIDs = new Set(device.states.map(state => state.id));
    expect(stateIDs.has(device.initialStateId)).toBe(true);
    expect(stateIDs.has(device.currentStateId)).toBe(true);
    for (const transition of device.transitions ?? []) {
      expect(stateIDs.has(transition.sourceStateId)).toBe(true);
      expect(stateIDs.has(transition.destinationStateId)).toBe(true);
      for (const effect of transition.conditionEffects ?? []) {
        expect(conditionByID.has(effect.conditionId)).toBe(true);
      }
    }
  }

  const effectKinds = new Set();
  const blockedCapabilities = new Set();
  const recoveryKinds = new Set();
  for (const condition of facility.conditions) {
    expect(inventory).toContain(`\`${condition.id}\``);
    // A fault may have no stored recovery entry (FR-020); the demo's entries
    // are unchanged, so every shape is still present below.
    for (const effect of condition.effects) {
      const kind = conditionEffectKind(effect);
      expect(kind).toBeDefined();
      effectKinds.add(kind);
      if (kind === 'capabilityBlock') blockedCapabilities.add(effect.capabilityBlock.capability);
      if (kind === 'diagnosticPath') {
        expect(nodeByTerminalAndID.has(`${effect.diagnosticPath.terminalId}:${effect.diagnosticPath.nodeId}`))
          .toBe(true);
      }
      if (kind === 'recordSubstitution') {
        expect(blockByTerminalAndID.has(`${effect.recordSubstitution.terminalId}:${effect.recordSubstitution.blockId}`))
          .toBe(true);
      }
    }
    for (const recovery of condition.recovery ?? []) {
      const kind = recoveryKind(recovery);
      expect(kind).toBeDefined();
      recoveryKinds.add(kind);
      if (kind === 'transition') expect(deviceByID.has(recovery.transition.deviceId)).toBe(true);
      if (kind === 'recoveryProgramId') expect(recoveryProgramByID.has(recovery.recoveryProgramId)).toBe(true);
    }
  }
  expect(effectKinds).toEqual(new Set([
    'capabilityBlock',
    'diagnosticPath',
    'displayInstability',
    'recordSubstitution',
  ]));
  expect(blockedCapabilities).toEqual(new Set([
    'execute-command',
    'hack',
    'run-recovery-program',
    'terminal-transition',
    'view-entry',
  ]));
  expect(recoveryKinds).toEqual(new Set([
    'privateOverseerAction',
    'recoveryProgramId',
    'transition',
  ]));
  for (const program of facility.recoveryPrograms) {
    expect(inventory).toContain(`\`${program.id}\``);
    expect(program.transitions.length).toBeGreaterThan(0);
    for (const transition of program.transitions) expect(deviceByID.has(transition.deviceId)).toBe(true);
  }
  for (const marker of [
    'facilityNameVariants',
    'facilityTextVariants',
    'visibleWhen',
    'availableWhen',
    'ПРЕДПРОСМОТР',
    'СБРОСИТЬ К НАЧАЛЬНОМУ…',
    'СБРОСИТЬ СЦЕНУ',
    // Private recovery is the Overseer's fault switch, listed in every fault dialog.
    'private Overseer recovery',
    'Смотритель вручную',
    'КАК УСТРАНЯЕТСЯ',
  ]) {
    expect(inventory, marker).toContain(marker);
  }
});

test('inventory demonstrates unified authoring on the demo command and door', async () => {
  const { demo, inventory } = await loadDemoAssets();
  // The paths name demo assets that exist.
  const nodeNames = new Set();
  const visit = node => {
    nodeNames.add(node.name);
    for (const child of node.children ?? []) visit(child);
  };
  for (const terminal of demo.terminals) visit(terminal.root);
  const deviceNames = new Set(demo.facility.devices.map(device => device.name));
  for (const name of ['Открыть выход и подать сигнал', 'СВОДКА ЭВАКУАЦИИ', 'НАРУЖНЫЙ МАРШРУТ ПОДТВЕРЖДЁН']) {
    expect(nodeNames, name).toContain(name);
    expect(inventory).toContain(name);
  }
  expect(deviceNames).toContain('Гермодверь Убежища 76');
  expect(inventory).toContain('Гермодверь Убежища 76');

  for (const marker of [
    // Cross-tab autosave with undo and redo.
    'ГОТОВО', 'ОТМЕНИТЬ', 'ПОВТОРИТЬ', 'ПРИМЕНИТЬ СНОВА', 'СОХРАНЕНО', 'НЕ ПРИМЕНЕНО',
    // Both command lists and the marked winner.
    'ПРИ ОДОБРЕНИИ', 'ЧТО ВИДЯТ ИГРОКИ', 'ВИДЯТ ИГРОКИ', 'ПЕРЕКРЫВАЕТ', 'СКРЫТО ОТ ИГРОКОВ', 'НЕДОСТУПНО',
    // Scene reset from the shared bar.
    'СБРОСИТЬ СЦЕНУ', 'СЦЕНА УЖЕ В НАЧАЛЬНОМ СОСТОЯНИИ',
    // Preview of differences.
    'СЕЙЧАС', 'ЕСЛИ', 'ИЗМЕНЕНИЙ: 3', 'ПЕРЕИМЕНУЕТСЯ', 'ТЕКСТ ИЗМЕНИТСЯ', 'ПОЯВИТСЯ',
  ]) {
    expect(inventory, marker).toContain(marker);
  }
  for (const removed of [
    'ПРИМЕНИТЬ К ЧЕРНОВИКУ', 'СОХРАНИТЬ ИЗМЕНЕНИЯ СЕССИИ', 'СОХРАНИТЬ ЧЕРНОВИК', 'ОТМЕНИТЬ ИЗМЕНЕНИЯ',
    'ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ', 'СБРОСИТЬ ВЕСЬ ОБЪЕКТ', 'СБРОСИТЬ ВСЕ СОСТОЯНИЯ',
  ]) {
    expect(inventory, removed).not.toContain(removed);
  }
  expect(inventory).not.toMatch(/черновик|draft/i);
});

test('inventory reaches every play control on the device card (FR-027)', async () => {
  const { demo, inventory } = await loadDemoAssets();
  const devices = new Map(demo.facility.devices.map(device => [device.id, device]));
  const reactor = devices.get('reactor-main');
  expect(reactor.states).toContainEqual({ id: 'critical-overheat', name: 'Критический перегрев' });
  // Forced change is the only way into the overheat state.
  for (const device of demo.facility.devices) {
    for (const transition of device.transitions ?? []) expect(transition.destinationStateId).not.toBe('critical-overheat');
  }
  const variants = [];
  const visit = node => {
    for (const block of node.blocks ?? []) {
      for (const variant of block.facilityTextVariants ?? []) variants.push({ block: block.id, ...variant });
    }
    for (const child of node.children ?? []) visit(child);
  };
  visit(demo.terminals.find(terminal => terminal.id === 't_demo1').root);
  expect(variants).toContainEqual({
    block: 'b_reactor_status',
    when: { deviceId: 'reactor-main', stateId: 'critical-overheat' },
    text: 'РЕАКТОР: КРИТИЧЕСКИЙ ПЕРЕГРЕВ',
  });
  // The ventilation row shows one active fault on a fresh copy.
  const ventilation = devices.get('ventilation-main');
  const activeVentilationFaults = demo.facility.conditions
    .filter(condition => condition.device?.deviceId === 'ventilation-main' && condition.currentActive);
  expect(ventilation.states.find(state => state.id === ventilation.currentStateId).name).toBe('Загрязнён');
  expect(activeVentilationFaults).toHaveLength(1);
  expect(devices.get('power-grid-main').currentStateId).toBe('online');
  expect(devices.get('reactor-main').currentStateId).toBe('standby');

  for (const marker of [
    'ВО ВРЕМЯ ИГРЫ', 'ПОСЛЕДНИЕ ИЗМЕНЕНИЯ', 'Открыть гермодверь → Открыта', 'НЕ ВЫПОЛНЕНО: Резервная сеть Убежища 76 = В работе',
    'ПРИНУДИТЕЛЬНО В ДРУГОЕ СОСТОЯНИЕ…', 'Критический перегрев', 'ПЕРЕВЕСТИ', 'РЕАКТОР: КРИТИЧЕСКИЙ ПЕРЕГРЕВ',
    'Контроллер гермодвери не отвечает', 'запрос игрока отклонён: Смотрителем',
    // The device row: a state badge, then the fault mark (FR-030).
    'the badge **Загрязнён** and the fault mark **● 1**',
    'СБРОСИТЬ К НАЧАЛЬНОМУ…', 'ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', 'КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ 1 ИЗ 5: УСТРОЙСТВО',
    '1. Устройство', 'ПОКАЗАТЬ', 'НЕИСПРАВНОСТИ',
  ]) {
    expect(inventory, marker).toContain(marker);
  }
  for (const removed of ['РЕДАКТИРОВАТЬ ГРАФ', 'СБРОСИТЬ УСТРОЙСТВО', 'open by default', 'ДИАГНОСТИЧЕСКИЕ УСЛОВИЯ']) {
    expect(inventory, removed).not.toContain(removed);
  }
});

// FR-044 needs one reachable route per editor case. The cases come from the
// demo data, so a demo edit that drops one fails here.
test('inventory reaches every facility editor case from the demo (FR-044)', async () => {
  const { demo, inventory } = await loadDemoAssets();
  const { devices, conditions, recoveryPrograms } = demo.facility;
  const deviceByID = new Map(devices.map(device => [device.id, device]));
  const stateName = (device, stateID) => device.states.find(state => state.id === stateID).name;
  const transitionOf = request => deviceByID.get(request.deviceId).transitions
    .find(transition => transition.id === request.transitionId);

  // Templates: applying one to a new door, and one with nothing left to add.
  for (const marker of [
    'ПРИМЕНИТЬ ШАБЛОН', 'ШАБЛОН: ДВЕРЬ', 'ИЗ ШАБЛОНА ДОБАВЛЕНО: СОСТОЯНИЙ — 2, ПЕРЕХОДОВ — 2',
    'ШАБЛОН: ТУРЕЛЬ', 'ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ',
  ]) {
    expect(inventory, marker).toContain(marker);
  }

  // A matrix cell with two transitions.
  const sharedCells = devices.flatMap(device => {
    const cells = Map.groupBy(device.transitions ?? [],
      transition => `${transition.sourceStateId}→${transition.destinationStateId}`);
    return [...cells.values()].filter(cell => cell.length > 1).map(cell => ({ device, cell }));
  });
  expect(sharedCells.length).toBeGreaterThan(0);
  for (const { device, cell } of sharedCells) {
    const heading = `ПЕРЕХОД: ${stateName(device, cell[0].sourceStateId)} → ${stateName(device, cell[0].destinationStateId)}`;
    expect(inventory, heading).toContain(heading);
    for (const transition of cell) {
      expect(inventory, transition.name).toContain(transition.name);
      expect(inventory).toContain(`\`${device.id}/${transition.id}\``);
    }
  }

  // Each display rule type, each stored somewhere in the demo.
  const nodes = demo.terminals.flatMap(walkNodes).map(({ node }) => node);
  const ruleTypes = {
    'ТЕКСТ ЗАПИСИ': nodes.some(node => node.blocks?.some(block => block.facilityTextVariants?.length)),
    'НАЗВАНИЕ В МЕНЮ': nodes.some(node => node.facilityNameVariants?.length),
    'ПОКАЗАТЬ ЭЛЕМЕНТ': nodes.some(node => node.visibleWhen),
    'РАЗРЕШИТЬ КОМАНДУ': nodes.some(node => node.availableWhen),
  };
  for (const [tab, stored] of Object.entries(ruleTypes)) {
    expect(stored, tab).toBe(true);
    expect(inventory, tab).toContain(`Display rule: **${tab}**`);
  }

  // The command action dialog opened from the command form.
  for (const marker of ['ДЕЙСТВИЕ КОМАНДЫ → ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ', 'ЧТО ЗАПУСТИТЬ']) {
    expect(inventory, marker).toContain(marker);
  }

  // Faults with and without a transition that switches them on.
  const activating = new Map();
  for (const device of devices) {
    for (const transition of device.transitions ?? []) {
      for (const effect of transition.conditionEffects ?? []) {
        if (effect.active) activating.set(effect.conditionId, `${device.id}/${transition.id}`);
      }
    }
  }
  const unswitched = conditions.filter(condition => !activating.has(condition.id));
  expect(activating.size).toBeGreaterThan(0);
  expect(unswitched.length).toBeGreaterThan(0);
  for (const [conditionID, transition] of activating) {
    expect(inventory, conditionID).toContain(`| \`${conditionID}\` |`);
    expect(inventory, transition).toContain(`\`${transition}\``);
  }
  expect(inventory).toContain('Ни один переход не включает эту неисправность. Сейчас её можно включить только вручную.');
  expect(unswitched.map(condition => condition.id)).toContain('door-offline');

  // A recovery program with at least two transitions.
  const multiStep = recoveryPrograms.filter(program => program.transitions.length > 1);
  expect(multiStep.length).toBeGreaterThan(0);
  for (const program of multiStep) {
    expect(inventory).toContain(`«${program.name}»`);
    for (const request of program.transitions) {
      expect(inventory).toContain(`\`${request.deviceId}/${request.transitionId}\``);
      expect(inventory).toContain(`**${deviceByID.get(request.deviceId).name} / ${transitionOf(request).name}**`);
    }
  }

  // Terminal faults in their own group.
  expect(inventory).toContain('ОБЪЕКТЫ → НЕИСПРАВНОСТИ ТЕРМИНАЛОВ');
  const terminalName = new Map(demo.terminals.map(terminal => [terminal.id, terminal.name]));
  for (const fault of conditions.filter(condition => condition.terminal)) {
    const row = `**${fault.name}** · **${fault.currentActive ? 'АКТИВНА' : 'НЕАКТИВНА'}** · `
      + `**ТЕРМИНАЛ: ${terminalName.get(fault.terminal.terminalId)}**`;
    expect(inventory, fault.id).toContain(row);
  }

  // Every dependency destination.
  for (const destination of ['К УСТРОЙСТВУ', 'К НЕИСПРАВНОСТИ', 'К ПРОГРАММЕ', 'К КОМАНДЕ', 'К ПРАВИЛУ ОТОБРАЖЕНИЯ']) {
    expect(inventory, destination).toContain(`| **${destination}** |`);
  }

  // Each list filter with its nonzero demo count.
  const activeFaults = deviceID => conditions
    .filter(condition => condition.device?.deviceId === deviceID && condition.currentActive).length;
  const counts = {
    'ВСЕ': devices.length,
    'ИЗМЕНЁННЫЕ': devices.filter(device => device.currentStateId !== device.initialStateId).length,
    'С НЕИСПРАВНОСТЬЮ': devices.filter(device => activeFaults(device.id) > 0).length,
  };
  expect(counts).toEqual({ 'ВСЕ': 10, 'ИЗМЕНЁННЫЕ': 5, 'С НЕИСПРАВНОСТЬЮ': 1 });
  for (const [filter, count] of Object.entries(counts)) {
    expect(inventory, filter).toContain(`ОБЪЕКТЫ → ${filter} ${count}`);
  }

  // The 041 names replace these everywhere in the inventory.
  for (const removed of [
    'Загрязнён · ● 1', 'ШАГ 1 ИЗ 5: ОБЪЕКТ', '1. Объект', 'ОБЪЕКТЫ → НЕИСПРАВНОСТИ →',
    'ПРЕДПРОСМОТР УСЛОВИЯ', 'ВОССТАНОВИТЬ УСЛОВИЕ', 'ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ', 'Operator recovery',
    'display-rule editor', 'ЗАВИСИМОСТИ → ОТКРЫТЬ', 'Область неисправности', '→ ОТКРЫТЬ**',
    'ДЕЙСТВИЕ УСТРОЙСТВ', 'Технические идентификаторы', 'ПРЕДУСЛОВИЯ И ЭФФЕКТЫ',
    'АКТИВИРОВАТЬ НЕИСПРАВНОСТЬ', 'СНЯТЬ НЕИСПРАВНОСТЬ', 'confirmed private Overseer recovery',
  ]) {
    expect(inventory, removed).not.toContain(removed);
  }
});

test('player-visible demo remains one in-world narrative without project warnings', async () => {
  const { demo } = await loadDemoAssets();
  const visibleText = [];
  for (const terminal of demo.terminals) {
    visibleText.push(terminal.name, terminal.introText);
    for (const { node } of walkNodes(terminal)) {
      visibleText.push(node.name, node.text, node.description);
      for (const block of node.blocks ?? []) {
        visibleText.push(block.initialText);
        for (const variant of block.facilityTextVariants ?? []) visibleText.push(variant.text);
      }
      for (const variant of node.facilityNameVariants ?? []) visibleText.push(variant.text);
      if (node.stateChange) {
        visibleText.push(node.stateChange.completedName, node.stateChange.confirmationText);
        visibleText.push(node.stateChange.entryContentChange?.completedText);
      }
    }
    for (const state of Object.values(terminal.commandStates ?? {})) {
      visibleText.push(state.completedName, state.resultText, state.entryContentChange?.completedText);
    }
  }
  for (const device of demo.facility.devices) {
    visibleText.push(device.name);
    for (const state of device.states) visibleText.push(state.name);
    for (const transition of device.transitions ?? []) visibleText.push(transition.name);
  }
  for (const condition of demo.facility.conditions) visibleText.push(condition.name);
  for (const program of demo.facility.recoveryPrograms) visibleText.push(program.name);

  const narrative = visibleText.filter(Boolean).join('\n');
  expect(narrative).not.toMatch(/licen[cs]e|copyright|intellectual[ -]property|hobby[ -]project|fan[ -]project|attribution/iu);
  expect(narrative).not.toMatch(/лиценз|авторск|интеллектуальн|хобби|любительск|предупрежден/iu);
  expect(narrative).not.toMatch(/\b(?:TODO|fixture|sample|example|feature flag)\b/iu);
  expect(narrative).toContain('УБЕЖИЩЕ 76');
  expect(narrative).toContain('РС-04');
  expect(narrative).toContain('АТЛАС-76');
});

test('documented controller and observer roles converge through the live browser journey', async ({ browser, request }) => {
  const { inventory } = await loadDemoAssets();
  expect(inventory).toContain('controller');
  expect(inventory).toContain('observer');

  const reset = await request.post(`${ROLE_FIXTURE}/reset`, { data: { scenario: 'ready' } });
  expect(reset.status()).toBe(204);
  const overseerContext = await browser.newContext();
  const overseer = await overseerContext.newPage();
  let controller;
  let observer;
  try {
    await overseer.goto(`${ROLE_FIXTURE}/overseer`);
    await overseer.getByRole('button', { name: 'ОТКРЫТЬ СЕССИЮ' }).click();
    await expect(overseer.locator('#mainLayout')).toBeVisible();
    controller = await openParticipant(browser);
    observer = await openParticipant(browser);
    await expect(controller.page.locator('#roleBadge')).toContainText('АКТИВЕН');
    await expect(observer.page.locator('#roleBadge')).toContainText('НАБЛЮДАТЕЛЬ');

    await controller.page.locator('.term-row', { hasText: 'OPEN SECURITY DOOR' }).click();
    const decision = overseer.getByRole('dialog', { name: 'ПОДТВЕРЖДЕНИЕ КОМАНДЫ' });
    await expect(decision).toBeVisible();
    for (const participant of [controller, observer]) {
      await expect(participant.page.locator('#entryBody')).toContainText('Выполняется запрос');
    }
    await decision.getByRole('button', { name: 'ПОДТВЕРДИТЬ' }).click();
    for (const participant of [controller, observer]) {
      await expect(participant.page.locator('.term-row', { hasText: 'SECURITY DOOR OPEN' })).toBeVisible();
      await expect(participant.page.locator('.term-row')).not.toHaveCount(0);
      await expect(participant.page.locator('#backBtn')).toBeHidden();
    }
  } finally {
    await controller?.context.close();
    await observer?.context.close();
    await overseerContext.close();
  }
});
