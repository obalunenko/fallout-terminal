import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const typewriterModulePath = fileURLToPath(new URL(
  '../../frontend/client/src/composables/useTypewriterReveal.ts',
  import.meta.url,
));
const expectedAssertion = '40ms clock reveal cancels once, consumes repeat once, does not replay after unmount';
const typewriterModuleURL = `http://127.0.0.1:34120/@fs${typewriterModulePath}`;
const crtShellModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/components/CrtShell.vue',
  import.meta.url,
))}`;

test.use({ bypassCSP: true });

test(expectedAssertion, async () => {
  if (!existsSync(typewriterModulePath)) {
    process.stderr.write(`AssertionError: ${expectedAssertion}\n`);
    throw new Error('Player candidate typewriter/CRT lifecycle is not implemented');
  }
});

test('typewriter reveal preserves identity repeat cues and releases its clock', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const { createTypewriterRevealController } = await import(source);
    const cancelled = [];
    const completed = [];
    const cues = [];
    const delays = [];
    const timers = new Map();
    const target = new EventTarget();
    let now = 0;
    let nextTimer = 1;
    const controller = createTypewriterRevealController({
      clock: {
        clearTimeout(handle) { timers.delete(handle); },
        now: () => now,
        setTimeout(callback, delay) { const handle = nextTimer++; delays.push(delay); timers.set(handle, callback); return handle; },
      },
      onCancel: identity => cancelled.push(identity),
      onComplete: identity => completed.push(identity),
      onCue: (index, identity) => cues.push(`${identity}:${index}`),
      target,
    });
    const advance = milliseconds => {
      now += milliseconds;
      const pending = [...timers.values()];
      timers.clear();
      for (const callback of pending) callback();
    };
    const first = controller.start('alpha', 3, true);
    advance(40);
    const key = new KeyboardEvent('keydown', { cancelable: true, code: 'Enter', key: 'Enter' });
    target.dispatchEvent(key);
    const repeat = new KeyboardEvent('keydown', { cancelable: true, code: 'Enter', key: 'Enter', repeat: true });
    target.dispatchEvent(repeat);
    const suppressed = controller.start('alpha', 3, true);
    controller.start('beta', 2, true);
    controller.start('gamma', 2, true);
    const pendingBeforeDispose = timers.size;
    controller.dispose();
    const afterDispose = controller.start('delta', 2, true);
    const detachedKey = new KeyboardEvent('keydown', { cancelable: true, key: 'Escape' });
    target.dispatchEvent(detachedKey);
    return {
      afterDispose,
      cancelled,
      completed,
      cues,
      delays,
      detachedConsumed: detachedKey.defaultPrevented,
      first,
      keyConsumed: key.defaultPrevented,
      pendingAfterDispose: timers.size,
      pendingBeforeDispose,
      repeatConsumed: repeat.defaultPrevented,
      state: controller.state,
      suppressed,
    };
  }, typewriterModuleURL);

  expect(result).toEqual({
    afterDispose: false,
    cancelled: ['beta', 'gamma'],
    completed: ['alpha'],
    cues: ['alpha:0', 'alpha:1', 'beta:0', 'gamma:0'],
    delays: [40, 40, 40, 40],
    detachedConsumed: false,
    first: true,
    keyConsumed: true,
    pendingAfterDispose: 0,
    pendingBeforeDispose: 1,
    repeatConsumed: true,
    state: { identity: 'gamma', phase: 'cancelled', total: 2, visible: 1 },
    suppressed: false,
  });
});

test('CRT effects preserve classes timing and clear on context change', async ({ page }) => {
  await page.setViewportSize({ width: 640, height: 360 });
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const shell = await import(source);
    const compiled = await (await fetch(source)).text();
    const runtimePath = compiled.match(/from "([^"]*\/node_modules\/\.vite\/deps\/vue\.js\?v=[^"]+)"/u)?.[1];
    if (runtimePath === undefined) throw new Error('compiled CRT shell Vue runtime was not found');
    const { createApp, h, nextTick, ref } = await import(new URL(runtimePath, location.origin).href);
    const host = document.createElement('div');
    host.id = 'crtEffectsFixture';
    host.style.cssText = 'position:fixed;inset:0';
    document.body.append(host);
    const context = ref('context-a');
    const revealing = ref(false);
    const app = createApp({
      render: () => h(shell.default, {
        contextKey: context.value,
        revealing: revealing.value,
      }, { default: () => h('p', { id: 'crtContent' }, 'READY') }),
    });
    app.mount(host);
    revealing.value = true;
    await nextTick();
    const presenting = host.querySelectorAll('.crt-presenting').length;
    const screen = host.querySelector('#screen').getBoundingClientRect();
    const scanlines = host.querySelector('.scanlines').getBoundingClientRect();
    context.value = 'context-b';
    await nextTick();
    const clearedOnContext = host.querySelectorAll('.crt-presenting').length;
    revealing.value = false;
    await nextTick();
    revealing.value = true;
    await nextTick();
    const restarted = host.querySelectorAll('.crt-presenting').length;
    const animation = getComputedStyle(host.querySelector('#screen')).animationName;
    app.unmount();
    const retained = host.querySelectorAll('.scanlines, .vignette, .crt-presenting').length;
    host.remove();
    return {
      animation,
      clearedOnContext,
      overlaysMatch: scanlines.left >= screen.left && scanlines.top >= screen.top &&
        screen.right - scanlines.right <= 4 && screen.bottom - scanlines.bottom <= 4,
      presenting,
      restarted,
      retained,
    };
  }, crtShellModuleURL);

  expect(result).toEqual({
    animation: 'flicker',
    clearedOnContext: 0,
    overlaysMatch: true,
    presenting: 2,
    restarted: 2,
    retained: 0,
  });
});
