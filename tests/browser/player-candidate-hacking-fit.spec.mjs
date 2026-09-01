import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const moduleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/useHackingBoardFit.ts',
  import.meta.url,
))}`;

test('hacking board fit restores probes and disconnects observers and frames', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const { createHackingBoardFitController } = await import(source);
    const board = document.createElement('section');
    board.id = 'hackBoard';
    board.className = 'hack-board';
    board.style.cssText = 'width:640px;height:280px';
    board.innerHTML = `
      <div id="hackColumns" class="hack-columns">
        <div id="hackColumn" class="hack-col">
          <div id="hackRow" class="hack-row"><span class="hack-addr">0xF000</span><span class="hack-cells">${'<span class="hcell">.</span>'.repeat(12)}</span></div>
        </div>
      </div>
      <aside id="hackLogPanel" class="hack-log-panel"><div class="hack-log"><div>ENTRY</div></div><p class="hack-input-line">&gt;</p></aside>`;
    document.body.append(board);

    let callback;
    let nextFrame = 1;
    let observerCallback;
    const cancelled = [];
    const disconnected = [];
    const frames = new Map();
    const observed = [];
    const probes = [];
    const mutations = new MutationObserver(records => {
      for (const record of records) {
        for (const node of record.addedNodes) {
          if (node instanceof HTMLElement && node.dataset.hackingFitProbe === 'true') {
            probes.push({
              ariaHidden: node.getAttribute('aria-hidden'),
              idCount: node.querySelectorAll('[id]').length + (node.id === '' ? 0 : 1),
              inert: node.inert,
            });
          }
        }
      }
    });
    mutations.observe(document.body, { childList: true });
    const controller = createHackingBoardFitController({
      frameScheduler: {
        cancelAnimationFrame(handle) { cancelled.push(handle); frames.delete(handle); },
        requestAnimationFrame(frameCallback) { const handle = nextFrame++; frames.set(handle, frameCallback); return handle; },
      },
      observerFactory(resizeCallback) {
        observerCallback = resizeCallback;
        return {
          disconnect() { disconnected.push(true); },
          observe(target) { observed.push(target.id); },
        };
      },
    }, value => { callback = value; });

    controller.setBoard(board);
    observerCallback([], {});
    const pendingAfterResize = [...frames.keys()];
    const measure = frames.get(pendingAfterResize[0]);
    frames.delete(pendingAfterResize[0]);
    measure(performance.now());
    await Promise.resolve();
    const applied = {
      compact: board.classList.contains('hack-compact'),
      font: board.style.getPropertyValue('--hack-row-font'),
      stacked: board.classList.contains('hack-stacked'),
      tight: board.classList.contains('hack-tight'),
    };
    const leakedAfterMeasure = document.querySelectorAll('[data-hacking-fit-probe]').length;
    controller.schedule();
    const pendingBeforeDispose = [...frames.keys()];
    controller.dispose();
    mutations.disconnect();
    const result = {
      applied,
      cancelled,
      callback,
      disconnected: disconnected.length,
      leakedAfterMeasure,
      observed,
      pendingAfterDispose: frames.size,
      pendingBeforeDispose,
      probes,
      restored: {
        compact: board.classList.contains('hack-compact'),
        font: board.style.getPropertyValue('--hack-row-font'),
        stacked: board.classList.contains('hack-stacked'),
        tight: board.classList.contains('hack-tight'),
      },
    };
    board.remove();
    return result;
  }, moduleURL);

  expect(result.observed).toEqual(['hackBoard']);
  expect(result.cancelled).toEqual([1, 3]);
  expect(result.pendingBeforeDispose).toEqual([3]);
  expect(result.pendingAfterDispose).toBe(0);
  expect(result.disconnected).toBe(1);
  expect(result.probes).toEqual([{ ariaHidden: 'true', idCount: 0, inert: true }]);
  expect(result.leakedAfterMeasure).toBe(0);
  expect(result.callback).toMatchObject({
    compact: result.applied.compact,
    stacked: result.applied.stacked,
    tight: result.applied.tight,
  });
  expect(Number.parseFloat(result.applied.font)).toBeCloseTo(result.callback.fontSize, 4);
  expect(result.applied.compact).toBe(true);
  expect(result.applied.stacked).toBe(true);
  expect(result.restored).toEqual({ compact: false, font: '', stacked: false, tight: false });
});
