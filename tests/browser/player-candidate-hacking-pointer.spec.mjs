import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const moduleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/useHackingPointer.ts',
  import.meta.url,
))}`;

test('hacking pointer validates current geometry and releases hover resources', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const { createHackingPointerController } = await import(source);
    const root = document.createElement('div');
    root.dataset.hackingContext = 'hack-1';
    root.innerHTML = `
      <button class="hcell word" data-target="word-a">CI</button>
      <button class="hcell word" data-target="word-a">PHER</button>
      <button class="hcell filler" data-target="0:6" data-column="0" data-character="6" data-row="0" data-offset="6">(</button>
      <button class="hcell filler" data-target="0:7" data-column="0" data-character="7" data-row="0" data-offset="7">)</button>`;
    document.body.append(root);
    const activations = [];
    const clears = [];
    const previews = [];
    const timers = new Map();
    let nextTimer = 1;
    let context = 'hack-1';
    const controller = createHackingPointerController({
      authorize: key => key === context,
      clock: {
        clearTimeout(handle) { clears.push(handle); timers.delete(handle); },
        setTimeout(callback) { const handle = nextTimer++; timers.set(handle, callback); return handle; },
      },
      contextKey: () => context,
      onActivate: (target, key) => activations.push({ action: target.action, key }),
      onPreview: (target, key) => previews.push({ cellCount: target?.cells.length ?? 0, key, target: target?.key ?? '', text: target?.text ?? '' }),
      patterns: () => [{ end: 7, id: 'pattern-a', row: 0, start: 6, used: false }],
    });
    controller.setRoot(root);

    const words = root.querySelectorAll('.word');
    words[0].dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
    words[1].dispatchEvent(new MouseEvent('mouseout', { bubbles: true, relatedTarget: words[0] }));
    words[0].focus();
    const identity = controller.captureFocus();
    const replacement = words[0].cloneNode(true);
    words[0].replaceWith(replacement);
    const restored = controller.restoreFocus(identity);
    replacement.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    const patternCell = root.querySelector('[data-character="6"]');
    patternCell.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
    patternCell.dispatchEvent(new MouseEvent('mouseout', { bubbles: true }));
    const pendingBeforeContextChange = [...timers.keys()];
    context = 'hack-2';
    patternCell.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    for (const callback of timers.values()) callback();
    timers.clear();

    controller.setRoot(null);
    replacement.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
    controller.setRoot(root);
    root.dataset.hackingContext = 'hack-2';
    controller.dispose();
    patternCell.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    const output = {
      activations,
      clears,
      pendingAfterDispose: timers.size,
      pendingBeforeContextChange,
      previews,
      restored,
    };
    root.remove();
    return output;
  }, moduleURL);

  expect(result.previews[0]).toEqual({ cellCount: 2, key: 'hack-1', target: 'word:word-a', text: 'CIPHER' });
  expect(result.previews[1]).toEqual({ cellCount: 2, key: 'hack-1', target: 'pattern:pattern-a', text: '()' });
  expect(result.activations).toEqual([{ action: { kind: 'word', wordID: 'word-a' }, key: 'hack-1' }]);
  expect(result.restored).toBe(true);
  expect(result.clears).toContain(1);
  expect(result.pendingBeforeContextChange).toEqual([2]);
  expect(result.pendingAfterDispose).toBe(0);
});
