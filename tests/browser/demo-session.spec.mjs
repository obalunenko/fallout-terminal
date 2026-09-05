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
    expect(condition.recovery.length).toBeGreaterThan(0);
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
    for (const recovery of condition.recovery) {
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
    'СБРОСИТЬ УСТРОЙСТВО',
    'СБРОСИТЬ ВЕСЬ ОБЪЕКТ',
  ]) {
    expect(inventory).toContain(marker);
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
