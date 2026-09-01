import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const pointerModulePath = fileURLToPath(new URL(
  '../../frontend/client/src/composables/useHackingPointer.ts',
  import.meta.url,
));
const expectedAssertion = 'candidate groups semantic targets across cells, rejects stale pointer context, restores focus and removes fit probe';
const hackingSessionModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/useHackingSession.ts',
  import.meta.url,
))}`;
const keyboardModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/useTerminalKeyboard.ts',
  import.meta.url,
))}`;
const hackingSurfaceModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/HackingSurface.vue',
  import.meta.url,
))}`;
const hackingAttemptsModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/HackingAttempts.vue',
  import.meta.url,
))}`;
const hackingLogModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/HackingLog.vue',
  import.meta.url,
))}`;
const hackingInputModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/HackingInputPreview.vue',
  import.meta.url,
))}`;
const hackingBlockedModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/HackingBlocked.vue',
  import.meta.url,
))}`;

test.use({ bypassCSP: true });

test('authoritative hacking state rejects stale actions without optimistic mutation', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async moduleURL => {
    const module = await import(moduleURL);
    const requests = [];
    const rpc = {
      async activatePattern(input) { requests.push(input); return { accepted: true, reason: 1, requestId: input.requestId, revision: 6n }; },
      async guess(input) { requests.push(input); return { accepted: true, reason: 1, requestId: input.requestId, revision: 6n }; },
    };
    const controller = module.createHackingSessionController({
      authorize: contextKey => contextKey === 'hack-context',
      broadcastID: () => 'broadcast',
      recognitionHandle: () => 'handle',
      requestIDFactory: () => 'guess-request',
      rpc,
      terminalID: () => 'terminal',
    });
    const hack = {
      attemptsLeft: 4,
      attemptsMax: 4,
      columns: [{
        addresses: ['0xF000'],
        text: '..CIPHER....',
        words: [{ id: 'word-a', length: 6, start: 2 }],
      }],
      failed: false,
      level: 3,
      log: [],
      patterns: [{ end: 3, patternId: 'pattern-a', row: 0, start: 0, used: false }],
      solved: false,
      wordLength: 6,
    };
    const initial = controller.apply(hack, 5, 'hack-context');
    const invalid = controller.apply({ ...hack, attemptsLeft: 5 }, 6, 'hack-context');
    const unauthorized = await controller.begin({ kind: 'word', wordID: 'word-a' }, 'stale-context');
    const accepted = await controller.begin({ kind: 'word', wordID: 'word-a' }, 'hack-context');
    const beforeAuthoritative = JSON.parse(JSON.stringify(controller.state));
    const stale = controller.apply({ ...hack, attemptsLeft: 0, solved: true }, 4, 'hack-context');
    const converged = controller.apply({ ...hack, attemptsLeft: 3 }, 6, 'hack-context');
    const afterAuthoritative = JSON.parse(JSON.stringify(controller.state));
    controller.dispose();
    return {
      accepted,
      afterAuthoritative,
      beforeAuthoritative,
      converged,
      initial,
      invalid,
      requestCount: requests.length,
      stale,
      unauthorized,
    };
  }, hackingSessionModuleURL);

  expect(observation.initial).toBe(true);
  expect(observation.invalid).toBe(false);
  expect(observation.unauthorized).toBe(false);
  expect(observation.accepted).toBe(true);
  expect(observation.requestCount).toBe(1);
  expect(observation.beforeAuthoritative.hack.attemptsLeft).toBe(4);
  expect(observation.beforeAuthoritative.pending.acceptedRevision).toBe(6);
  expect(observation.stale).toBe(false);
  expect(observation.converged).toBe(true);
  expect(observation.afterAuthoritative.hack.attemptsLeft).toBe(3);
  expect(observation.afterAuthoritative.pending).toBeNull();
});

test('terminal keyboard obeys current context authority and removes its listener', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const { createTerminalKeyboardController } = await import(source);
    const calls = [];
    let context = 'hack-1';
    let state = {
      blocked: false,
      contextKey: 'hack-1',
      hackingComplete: false,
      menuCount: 3,
      menuIndex: 1,
      mode: 'hacking',
      pageCount: 3,
      pageIndex: 1,
      typed: 'AB',
    };
    const controller = createTerminalKeyboardController({
      authorize: key => key === 'hack-1',
      contextKey: () => context,
      onActivate: () => calls.push(['activate']),
      onBack: () => calls.push(['back']),
      onMenuIndex: index => calls.push(['menu', index]),
      onPageIndex: index => calls.push(['page', index]),
      onTyped: value => { calls.push(['typed', value]); state = { ...state, typed: value }; },
      state: () => state,
    });
    const press = (key, target = document) => {
      const event = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key });
      target.dispatchEvent(event);
      return event.defaultPrevented;
    };

    const typed = press('C');
    const erased = press('Backspace');
    const input = document.createElement('input');
    document.body.append(input);
    const editableIgnored = press('Z', input);
    context = 'stale';
    const staleConsumed = press('D');
    context = 'hack-1';
    state = { ...state, mode: 'menu' };
    const moved = press('ArrowDown');
    state = { ...state, blocked: true };
    const blockedConsumed = press('Enter');
    controller.dispose();
    state = { ...state, blocked: false };
    const afterDispose = press('Enter');
    input.remove();
    return { afterDispose, blockedConsumed, calls, editableIgnored, erased, moved, staleConsumed, typed };
  }, keyboardModuleURL);

  expect(result).toEqual({
    afterDispose: false,
    blockedConsumed: true,
    calls: [['typed', 'ABC'], ['typed', 'AB'], ['menu', 2]],
    editableIgnored: false,
    erased: true,
    moved: true,
    staleConsumed: true,
    typed: true,
  });
});

test('hacking surface preserves DOM geometry input and stable keys', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  await page.evaluate(async urls => {
    const [attempts, surface] = await Promise.all([import(urls.attempts), import(urls.surface)]);
    const compiled = await (await fetch(urls.surface)).text();
    const runtimePath = compiled.match(/from "([^"]*\/node_modules\/\.vite\/deps\/vue\.js\?v=[^"]+)"/u)?.[1];
    if (runtimePath === undefined) throw new Error('compiled hacking surface Vue runtime was not found');
    const { createApp, h, nextTick, ref } = await import(new URL(runtimePath, location.origin).href);
    const host = document.createElement('div');
    host.id = 'hackingSurfaceFixture';
    document.body.append(host);
    const events = [];
    const hack = ref({
      attemptsLeft: 4,
      attemptsMax: 4,
      columns: [{
        addresses: ['0xF000', '0xF00C'],
        text: '..CIPHER....' + '..........()',
        words: [{ id: 'word-a', length: 6, start: 2 }],
      }],
      failed: false,
      level: 3,
      log: [],
      patterns: [{ end: 11, id: 'pattern-a', row: 1, start: 10, used: false }],
      solved: false,
      wordLength: 6,
    });
    const app = createApp({
      render: () => h('div', [
        h(attempts.default, { attemptsLeft: hack.value.attemptsLeft, attemptsMax: hack.value.attemptsMax }),
        h(surface.default, {
          canControl: true,
          contextKey: 'hack-context',
          hack: hack.value,
          highlightedKey: '',
          onActivate: action => events.push(action),
        }),
      ]),
    });
    app.mount(host);
    window.__hackingSurfaceFixture = {
      events,
      release: () => app.unmount(),
      updateAttempts: async () => { hack.value = { ...hack.value, attemptsLeft: 3 }; await nextTick(); },
    };
  }, { attempts: hackingAttemptsModuleURL, surface: hackingSurfaceModuleURL });

  const fixture = page.locator('#hackingSurfaceFixture');
  await expect(fixture.locator('#attemptsLine .atsq')).toHaveCount(4);
  await expect(fixture.locator('.hack-row[data-hack-row="0:0"] .hack-addr')).toHaveText('0xF000');
  await expect(fixture.locator('.hcell.word[data-target="word-a"]')).toHaveText('CIPHER');
  await expect(fixture.locator('.hcell.filler[data-column="0"][data-character="0"]')).toHaveAttribute('data-row', '0');
  const stable = await page.evaluate(async () => {
    const before = document.querySelector('#hackingSurfaceFixture .hack-row[data-hack-row="0:0"]');
    await window.__hackingSurfaceFixture.updateAttempts();
    return before === document.querySelector('#hackingSurfaceFixture .hack-row[data-hack-row="0:0"]');
  });
  expect(stable).toBe(true);
  await fixture.locator('.hcell.word[data-target="word-a"]').dispatchEvent('click');
  expect(await page.evaluate(() => window.__hackingSurfaceFixture.events)).toEqual([{ kind: 'word', wordID: 'word-a' }]);
  await page.evaluate(() => window.__hackingSurfaceFixture.release());
  await expect(fixture.locator('*')).toHaveCount(0);
  await expect(page.locator('[data-hacking-fit-probe]')).toHaveCount(0);
});

test('hacking output preserves copy accessibility and authoritative blocking', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async urls => {
    const [log, input, blocked] = await Promise.all([
      import(urls.log),
      import(urls.input),
      import(urls.blocked),
    ]);
    const compiled = await (await fetch(urls.log)).text();
    const runtimePath = compiled.match(/from "([^"]*\/node_modules\/\.vite\/deps\/vue\.js\?v=[^"]+)"/u)?.[1];
    if (runtimePath === undefined) throw new Error('compiled hacking output Vue runtime was not found');
    const { createApp, h, nextTick, ref } = await import(new URL(runtimePath, location.origin).href);
    const host = document.createElement('div');
    host.id = 'hackingOutputFixture';
    document.body.append(host);
    const typed = ref('');
    const hover = ref('CIPHER');
    const failed = ref(false);
    const lines = ref(['ENTRY DENIED', '<img src=x onerror="window.__unsafe=true">']);
    const outside = document.createElement('button');
    outside.textContent = 'outside';
    document.body.append(outside);
    const app = createApp({
      render: () => h('div', [
        h(log.default, { lines: lines.value }),
        h(input.default, { blocked: failed.value, hoverText: hover.value, typed: typed.value }),
        h(blocked.default, { visible: failed.value }),
      ]),
    });
    app.mount(host);
    outside.focus();
    const focusedBefore = document.activeElement === outside;
    typed.value = 'ROOT';
    await nextTick();
    const typedPreview = host.querySelector('#hackInputPreview')?.textContent;
    failed.value = true;
    await nextTick();
    const output = {
      blockedCopy: [...host.querySelectorAll('#hackBlocked > div')].map(element => element.textContent),
      focusedBefore,
      focusPreserved: document.activeElement === outside,
      hasUnsafeElement: host.querySelector('#hackLog img') !== null,
      lineCopy: [...host.querySelectorAll('#hackLog > div')].map(element => element.textContent),
      live: host.querySelector('#hackLog')?.getAttribute('aria-live'),
      previewBlocked: host.querySelector('#hackInputPreview')?.textContent,
      previewDisabled: host.querySelector('.hack-input-line')?.getAttribute('aria-disabled'),
      typedPreview,
      unsafeExecuted: window.__unsafe === true,
    };
    app.unmount();
    host.remove();
    outside.remove();
    return output;
  }, { blocked: hackingBlockedModuleURL, input: hackingInputModuleURL, log: hackingLogModuleURL });

  expect(result).toEqual({
    blockedCopy: ['Вход заблокирован.', 'Обратитесь к администратору.'],
    focusedBefore: true,
    focusPreserved: true,
    hasUnsafeElement: false,
    lineCopy: ['ENTRY DENIED', '<img src=x onerror="window.__unsafe=true">'],
    live: 'polite',
    previewBlocked: '',
    previewDisabled: 'true',
    typedPreview: 'ROOT',
    unsafeExecuted: false,
  });
});

test(expectedAssertion, async () => {
  if (!existsSync(pointerModulePath)) {
    process.stderr.write(`AssertionError: ${expectedAssertion}\n`);
    throw new Error('Player candidate hacking pointer/geometry lifecycle is not implemented');
  }
});
