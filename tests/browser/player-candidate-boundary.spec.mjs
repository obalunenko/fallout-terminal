import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const moduleURL = relativePath => `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(relativePath, import.meta.url))}`;

test.use({ bypassCSP: true });

test('Player decoded-network boundary accepts snapshots and rejects invalid ordering', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async url => {
    const module = await import(url);
    const controller = module.createPlayerProjectionController();
    const invalid = controller.applySnapshot({ recognitionHandle: 'handle', revision: 1n });
    const stateAfterInvalid = { revision: controller.state.revision, playerState: controller.state.playerState };
    const valid = controller.applySnapshot({
      playerState: { fallbackName: 'P1' },
      recognitionHandle: 'handle',
      revision: 1n,
      terminalPresentation: { presentation: { case: 'noLiveTerminal', value: {} } },
    });
    const stale = controller.applyUpdate({
      playerState: { fallbackName: 'STALE' },
      revision: 0n,
      terminalPresentation: { presentation: { case: 'noLiveTerminal', value: {} } },
    });
    return { fallback: controller.state.playerState?.fallbackName, invalid, stale, stateAfterInvalid, valid };
  }, moduleURL('../../frontend/client/src/composables/usePlayerProjection.ts'));
  expect(result).toEqual({
    fallback: 'P1',
    invalid: false,
    stale: false,
    stateAfterInvalid: { playerState: null, revision: 0 },
    valid: true,
  });
});

test('Player storage boundary accepts valid records and rejects malformed values', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async url => {
    const module = await import(url);
    const values = new Map();
    const storage = {
      get length() { return values.size; },
      clear() { values.clear(); },
      getItem(key) { return values.get(key) ?? null; },
      key(index) { return [...values.keys()][index] ?? null; },
      removeItem(key) { values.delete(key); },
      setItem(key, value) { values.set(key, String(value)); },
    };
    const eventTarget = { addEventListener() {}, removeEventListener() {} };
    const adapter = module.createRecognitionStorage({ eventTarget, storage });
    values.set(module.PLAYER_SESSION_INIT_LEASE_KEY, '{bad');
    const invalid = adapter.readLease();
    const validWrite = adapter.writeLease({ expiresAt: 100, owner: 'owner', token: 'token', version: 1 });
    return { invalid, valid: adapter.readLease(), validWrite };
  }, moduleURL('../../frontend/client/src/adapters/recognition-storage.ts'));
  expect(result).toEqual({
    invalid: null,
    valid: { expiresAt: 100, owner: 'owner', token: 'token', version: 1 },
    validWrite: true,
  });
});

test('Player DOM action boundary accepts authorized IDs and rejects unauthorized input', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async url => {
    const module = await import(url);
    let authorized = false;
    let calls = 0;
    const rpc = {
      navigate: async input => { calls += 1; return { accepted: true, reason: 1, requestId: input.requestId, revision: 0n }; },
      selectCharacter: async input => { calls += 1; return { accepted: true, reason: 1, requestId: input.requestId, revision: 0n }; },
    };
    const controller = module.createPlayerActionsController({
      authorize: () => authorized,
      requestIDFactory: () => 'request',
      rpc,
    });
    const action = {
      input: { broadcastId: 'broadcast', characterId: 'character', recognitionHandle: 'handle' },
      kind: 'selectCharacter',
    };
    const rejected = await controller.begin(action);
    const rejectedState = JSON.parse(JSON.stringify(controller.state));
    authorized = true;
    const accepted = await controller.begin(action);
    return { accepted, calls, rejected, rejectedState };
  }, moduleURL('../../frontend/client/src/composables/usePlayerActions.ts'));
  expect(result).toEqual({
    accepted: true,
    calls: 1,
    rejected: false,
    rejectedState: { error: '', pending: null, revision: 0 },
  });
});

test('Player navigation boundary accepts valid modes and rejects invalid states', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async url => {
    const module = await import(url);
    const controller = module.createTerminalNavigationController();
    const invalid = controller.apply({ mode: 2, path: ['root'] }, 1, false);
    const stateAfterInvalid = controller.state;
    const valid = controller.apply({ mode: 1, path: ['root'] }, 1, false);
    return { invalid, stateAfterInvalid, valid, validState: controller.state };
  }, moduleURL('../../frontend/client/src/composables/useTerminalNavigation.ts'));
  expect(result).toEqual({
    invalid: false,
    stateAfterInvalid: null,
    valid: true,
    validState: {
      commandNodeID: null,
      mode: 'list',
      path: ['root'],
      pending: false,
      revision: 1,
      viewEntryID: null,
    },
  });
});

test('Player pointer boundary accepts current geometry and rejects unauthorized input', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async url => {
    const { createHackingPointerController } = await import(url);
    const root = document.createElement('div');
    root.dataset.hackingContext = 'context-a';
    root.innerHTML = '<span class="hcell word" data-target="word-a">WORD</span>';
    document.body.append(root);
    let authorized = false;
    const actions = [];
    const controller = createHackingPointerController({
      authorize: () => authorized,
      contextKey: () => 'context-a',
      onActivate: target => actions.push(target.action),
      onPreview() {},
      patterns: () => [],
    });
    controller.setRoot(root);
    const cell = root.querySelector('.hcell');
    cell.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    const rejectedActions = actions.length;
    authorized = true;
    cell.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    controller.dispose();
    root.remove();
    return { actions, rejectedActions };
  }, moduleURL('../../frontend/client/src/composables/useHackingPointer.ts'));
  expect(result).toEqual({ actions: [{ kind: 'word', wordID: 'word-a' }], rejectedActions: 0 });
});

test('Player keyboard boundary accepts controller input and rejects observer mutation', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async url => {
    const { createTerminalKeyboardController } = await import(url);
    const target = new EventTarget();
    const values = [];
    let authorized = false;
    const controller = createTerminalKeyboardController({
      authorize: () => authorized,
      contextKey: () => 'context-a',
      onActivate() {}, onBack() {}, onMenuIndex() {}, onPageIndex() {},
      onTyped: value => values.push(value),
      state: () => ({ blocked: false, contextKey: 'context-a', hackingComplete: false, menuCount: 0,
        menuIndex: 0, mode: 'hacking', pageCount: 0, pageIndex: 0, typed: '' }),
      target,
    });
    const rejected = new KeyboardEvent('keydown', { cancelable: true, key: 'A' });
    target.dispatchEvent(rejected);
    authorized = true;
    const accepted = new KeyboardEvent('keydown', { cancelable: true, key: 'A' });
    target.dispatchEvent(accepted);
    controller.dispose();
    return { accepted: accepted.defaultPrevented, rejected: rejected.defaultPrevented, values };
  }, moduleURL('../../frontend/client/src/composables/useTerminalKeyboard.ts'));
  expect(result).toEqual({ accepted: true, rejected: true, values: ['A'] });
});

test('Player sound manifest boundary accepts safe assets and rejects unsafe paths', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async url => {
    const { safeSoundAssetURL } = await import(url);
    return {
      safe: safeSoundAssetURL('enter', 'sounds/enter/ui_hacking_charenter_01.wav', 'https://terminal.example'),
      unsafe: safeSoundAssetURL('enter', 'sounds/enter/../secret.wav', 'https://terminal.example'),
    };
  }, moduleURL('../../frontend/client/src/adapters/sound-manifest.ts'));
  expect(result).toEqual({ safe: '/sounds/enter/ui_hacking_charenter_01.wav', unsafe: null });
});

test('Player uplink boundary accepts correlated secure envelopes and rejects stale input', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async url => {
    const { validatePresentationResult } = await import(url);
    const expected = { clientInstanceID: 'client-a', generation: 2, requestIDs: new Set(['request-a']) };
    const action = { accepted: true, reason: 1, requestId: 'request-a', revision: 4n };
    return {
      accepted: validatePresentationResult({ clientInstanceId: 'client-a', payload: { case: 'action', value: action }, uplinkGeneration: 2n }, expected)?.kind,
      stale: validatePresentationResult({ clientInstanceId: 'client-a', payload: { case: 'action', value: action }, uplinkGeneration: 1n }, expected),
    };
  }, moduleURL('../../frontend/client/src/adapters/presentation-uplink-transport.ts'));
  expect(result).toEqual({ accepted: 'action', stale: null });
});
