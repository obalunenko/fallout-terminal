import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const identityModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/usePlayerIdentity.ts',
  import.meta.url,
))}`;
const authorityModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/usePlayerAuthority.ts',
  import.meta.url,
))}`;
const vueModuleURL = 'http://127.0.0.1:34120/@fs' + fileURLToPath(new URL(
  '../../frontend/node_modules/vue/dist/vue.esm-browser.js',
  import.meta.url,
));
const characterSelectionModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/CharacterSelection.vue',
  import.meta.url,
))}`;
const assignedWaitingModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/AssignedWaiting.vue',
  import.meta.url,
))}`;
const playerStatusLineModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/PlayerStatusLine.vue',
  import.meta.url,
))}`;

test.use({ bypassCSP: true });

test('player identity rejects invalid roster and stale assignment', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async moduleURL => {
    const module = await import(moduleURL);
    const controller = module.createPlayerIdentityController();
    const state = {
      assignedCharacter: { characterId: 'character-a', displayName: 'Mara' },
      fallbackName: 'PLAYER 1',
      logicalSessionId: 'logical-session',
      phase: 4,
      role: 2,
      roster: [
        { availability: 2, characterId: 'character-a', displayName: 'Mara' },
        { availability: 1, characterId: 'character-b', displayName: 'Viktor' },
      ],
    };
    const accepted = controller.apply('opaque-handle', 10, state);
    const trusted = JSON.parse(JSON.stringify(controller.state));
    const duplicateRoster = controller.apply('opaque-handle', 11, {
      ...state,
      roster: [state.roster[0], state.roster[0]],
    });
    const mismatchedAssignment = controller.apply('opaque-handle', 11, {
      ...state,
      assignedCharacter: { characterId: 'character-a', displayName: 'Wrong' },
    });
    const malformedHandle = controller.apply(' opaque-handle', 11, state);
    const staleAssignment = controller.apply('opaque-handle', 9, {
      ...state,
      assignedCharacter: { characterId: 'character-b', displayName: 'Viktor' },
    });
    const unchanged = JSON.parse(JSON.stringify(controller.state));
    controller.dispose();
    const afterDispose = controller.apply('opaque-handle', 12, state);
    return {
      accepted,
      afterDispose,
      duplicateRoster,
      malformedHandle,
      mismatchedAssignment,
      staleAssignment,
      trusted,
      unchanged,
    };
  }, identityModuleURL);

  expect(observation.accepted).toBe(true);
  expect(observation.trusted).toEqual({
    assigned: { id: 'character-a', name: 'Mara' },
    fallbackName: 'PLAYER 1',
    logicalSessionID: 'logical-session',
    phase: 'controlling',
    recognitionHandle: 'opaque-handle',
    revision: 10,
    role: 'active',
    roster: [
      { id: 'character-a', name: 'Mara', status: 'claimed' },
      { id: 'character-b', name: 'Viktor', status: 'available' },
    ],
  });
  expect(observation.unchanged).toEqual(observation.trusted);
  expect({
    afterDispose: observation.afterDispose,
    duplicateRoster: observation.duplicateRoster,
    malformedHandle: observation.malformedHandle,
    mismatchedAssignment: observation.mismatchedAssignment,
    staleAssignment: observation.staleAssignment,
  }).toEqual({
    afterDispose: false,
    duplicateRoster: false,
    malformedHandle: false,
    mismatchedAssignment: false,
    staleAssignment: false,
  });
});

test('character selection preserves DOM keyboard focus and typed events', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const selected = await page.evaluate(async urls => {
    const [{ createApp, h }, selection, waiting, status] = await Promise.all([
      import(urls.vue),
      import(urls.selection),
      import(urls.waiting),
      import(urls.status),
    ]);
    const host = document.createElement('div');
    host.id = 'identityFixture';
    document.body.append(host);
    const events = [];
    const identity = {
      assigned: { id: 'character-a', name: 'Mara' },
      fallbackName: 'PLAYER 12',
      logicalSessionID: 'logical',
      phase: 'selecting',
      recognitionHandle: 'handle',
      revision: 10,
      role: 'active',
      roster: [
        { id: 'character-a', name: 'Mara', status: 'available' },
        { id: 'character-b', name: 'Viktor', status: 'claimed' },
      ],
    };
    const app = createApp({
      render: () => h('div', [
        h(selection.default, {
          onSelect: characterID => events.push(characterID),
          roster: identity.roster,
        }),
        h(waiting.default),
        h(status.default, { identity }),
      ]),
    });
    app.mount(host);
    window.__identityFixture = { events, release: () => app.unmount() };
    return true;
  }, {
    selection: characterSelectionModuleURL,
    status: playerStatusLineModuleURL,
    vue: vueModuleURL,
    waiting: assignedWaitingModuleURL,
  });
  expect(selected).toBe(true);

  const available = page.locator('#identityFixture .character-option[data-character-id="character-a"]');
  const claimed = page.locator('#identityFixture .character-option[data-character-id="character-b"]');
  await expect(available).toHaveAttribute('data-status', 'available');
  await expect(claimed).toBeDisabled();
  await available.focus();
  await expect(available).toBeFocused();
  await available.press('Enter');
  expect(await page.evaluate(() => window.__identityFixture.events)).toEqual(['character-a']);
  await expect(page.locator('#identityFixture #assignedWaiting')).toContainText('ПЕРСОНАЖ НАЗНАЧЕН');
  await expect(page.locator('#identityFixture #playerFallbackName')).toHaveText('P12');
  await expect(page.locator('#identityFixture #playerCharacterName')).toHaveText('Mara');
  await expect(page.locator('#identityFixture #roleBadge')).toHaveText('АКТИВЕН');
  await expect(page.locator('#identityFixture #roleBadge')).toHaveAttribute('data-role', 'active');

  await page.evaluate(() => window.__identityFixture.release());
  await expect(page.locator('#identityFixture').locator('*')).toHaveCount(0);
});

test('controller and observer authority stays context-keyed and non-optimistic', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async moduleURL => {
    const module = await import(moduleURL);
    const controller = module.createPlayerAuthorityController();
    const identity = {
      assigned: { id: 'character-a', name: 'Mara' },
      fallbackName: 'P1',
      logicalSessionID: 'logical',
      phase: 'controlling',
      recognitionHandle: 'handle',
      revision: 10,
      role: 'active',
      roster: [],
    };
    const active = controller.apply({
      activeTerminalID: 'terminal-a',
      broadcastID: 'broadcast-a',
      contextKey: 'hack:terminal-a:round-1',
      identity,
      presentedTerminalID: 'terminal-a',
    });
    const wrongContextFeedback = controller.showTransientFeedback('hack:terminal-a:old', 'STALE');
    const feedback = controller.showTransientFeedback('hack:terminal-a:round-1', 'ДЕЙСТВИЕ ОТКЛОНЕНО');
    const activeState = { ...controller.state };
    const observer = controller.apply({
      activeTerminalID: 'terminal-a',
      broadcastID: 'broadcast-a',
      contextKey: 'hack:terminal-a:round-1',
      identity: { ...identity, revision: 11, role: 'observer', phase: 'observing' },
      presentedTerminalID: 'terminal-a',
    });
    const observerFeedback = controller.showTransientFeedback('hack:terminal-a:round-1', 'NOT ALLOWED');
    const observerState = { ...controller.state };
    const staleAuthority = controller.apply({
      activeTerminalID: 'terminal-a',
      broadcastID: 'broadcast-a',
      contextKey: 'hack:terminal-a:round-1',
      identity,
      presentedTerminalID: 'terminal-a',
    });
    controller.dispose();
    return {
      active,
      activeState,
      feedback,
      observer,
      observerFeedback,
      observerState,
      staleAuthority,
      wrongContextFeedback,
    };
  }, authorityModuleURL);

  expect(observation).toEqual({
    active: true,
    activeState: {
      canControl: true,
      contextKey: 'hack:terminal-a:round-1',
      observerReadOnly: false,
      revision: 10,
      transientFeedback: 'ДЕЙСТВИЕ ОТКЛОНЕНО',
    },
    feedback: true,
    observer: true,
    observerFeedback: false,
    observerState: {
      canControl: false,
      contextKey: 'hack:terminal-a:round-1',
      observerReadOnly: true,
      revision: 11,
      transientFeedback: '',
    },
    staleAuthority: false,
    wrongContextFeedback: false,
  });
});
