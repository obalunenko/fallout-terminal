import { expect, test } from '@playwright/test';

const CRT_VIEWPORTS = [
  { name: 'compact', width: 360, height: 640 },
  { name: 'medium', width: 768, height: 720 },
  { name: 'large', width: 1440, height: 900 },
];

const HACK_FONT_LAYOUTS = [
  { name: 'normal', width: 1440, height: 900 },
  { name: 'compact-stacked', width: 360, height: 640 },
  { name: '200-percent-zoom-fallback', width: 512, height: 300, fallbackFont: true },
];

async function resetAndOpen({ page, request }) {
  const reset = await request.post('/__fixture/reset');
  expect(reset.status()).toBe(204);
  await page.goto('/');
  await expect(page.locator('#connOverlay')).toBeHidden();
}

async function assignPlayer(page) {
  const selection = page.locator('#characterSelect');
  if (await selection.isVisible()) {
    await page.locator('#characterOptions button:not([disabled])').first().click();
  }
  await expect(page.locator('#termList')).toBeVisible();
}

async function activateCRTFixture(request, state = 'content') {
  const response = await request.post(`/__fixture/local/crt/${state}`);
  expect(response.status(), await response.text()).toBe(204);
}

async function approveCRTCommand(page, request, { verifyInputLock = true } = {}) {
  await expect(page.locator('#termEntry')).toBeVisible();
  await expect(page.locator('#entryBody')).toHaveText('Выполняется запрос');
  await expect(page.locator('#termOutput')).toBeHidden();
  await expect(page.locator('#backBtn')).toBeHidden();
  if (verifyInputLock) {
    await page.keyboard.press('Enter');
    await page.keyboard.press('Backspace');
    await expect(page.locator('#entryBody')).toHaveText('Выполняется запрос');
  }

  const response = await request.post('/__fixture/local/crt/approve-command');
  expect(response.status()).toBe(204);
  await expect(page.locator('#entryBody')).toContainText('DIAGNOSTIC OUTPUT');
}

async function visibleHackPatternCell(page) {
  const seen = new Set();
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline) {
    const candidates = await page.locator('#hackColumns .hcell.filler').evaluateAll(cells =>
      cells
        .filter(cell => '([{<'.includes(cell.textContent || ''))
        .map(cell => ({ row: cell.dataset.row, offset: cell.dataset.offset })),
    );
    for (const coordinates of candidates) {
      const key = `${coordinates.row}:${coordinates.offset}`;
      if (seen.has(key)) continue;
      seen.add(key);
      const cell = page.locator(
        `#hackColumns .hcell.filler[data-row="${coordinates.row}"][data-offset="${coordinates.offset}"]`,
      );
      await cell.hover();
      try {
        await expect.poll(
          () => page.locator('#hackColumns .hcell.hi').count(),
          { timeout: 250 },
        ).toBeGreaterThan(1);
        return cell;
      } catch {
        // The authoritative projection confirmed this opener is not a pattern.
      }
    }
    await page.waitForTimeout(40);
  }
  throw new Error('authoritative hacking pattern highlight was not found');
}

async function observeHackRevealFonts(page) {
  await page.evaluate(() => {
    window.__hackRevealFontSamples = [];
    window.__hackRevealFontObserver?.disconnect();
    const sample = () => {
      const rows = document.querySelectorAll('#hackColumns .hack-row');
      if (!rows.length) return;
      window.__hackRevealFontSamples.push({
        count: rows.length,
        font: Number.parseFloat(getComputedStyle(rows[0]).fontSize),
      });
    };
    window.__hackRevealFontObserver = new MutationObserver(records => {
      if (records.some(record => Array.from(record.addedNodes)
        .some(node => node.nodeType === Node.ELEMENT_NODE &&
          (node.matches?.('.hack-row') || node.querySelector?.('.hack-row'))))) {
        requestAnimationFrame(sample);
      }
    });
    window.__hackRevealFontObserver.observe(document.querySelector('#hackColumns'), {
      childList: true,
      subtree: true,
    });
  });
}

function observePlayerMutations(page) {
  const mutations = [];
  page.on('request', request => {
    const procedure = new URL(request.url()).pathname.split('/').pop();
    if (['Navigate', 'Guess', 'ActivatePattern'].includes(procedure)) mutations.push(procedure);
  });
  return mutations;
}

async function freezeAnimations(page) {
  await page.evaluate(() => {
    for (const animation of document.getAnimations()) {
      animation.pause();
      animation.currentTime = 0;
    }
  });
}

async function expectScreenContained(page) {
  const result = await page.evaluate(() => {
    const screen = document.querySelector('#screen').getBoundingClientRect();
    return {
      documentScrolls: document.documentElement.scrollHeight > window.innerHeight + 1 ||
        document.documentElement.scrollWidth > window.innerWidth + 1,
      contained: screen.top >= -1 && screen.left >= -1 &&
        screen.right <= window.innerWidth + 1 && screen.bottom <= window.innerHeight + 1,
    };
  });
  expect(result).toEqual({ documentScrolls: false, contained: true });
}

async function expectStateContained(page, selector) {
  const state = page.locator(selector);
  await expect(state).toBeVisible();
  await expect.poll(() => state.evaluate(element => {
    const screen = document.querySelector('#screen').getBoundingClientRect();
    const bounds = element.getBoundingClientRect();
    return {
      insideScreen: bounds.top >= screen.top - 1 && bounds.left >= screen.left - 1 &&
        bounds.right <= screen.right + 1 && bounds.bottom <= screen.bottom + 1,
      insideViewport: bounds.top >= -1 && bounds.left >= -1 &&
        bounds.right <= window.innerWidth + 1 && bounds.bottom <= window.innerHeight + 1,
    };
  })).toEqual({ insideScreen: true, insideViewport: true });
  await expectScreenContained(page);
  return state;
}

async function expectControlReachable(page, selector) {
  const control = page.locator(selector).first();
  await expect(control).toBeVisible();
  await expect(control).toBeEnabled();
  expect(await control.evaluate(element => {
    const bounds = element.getBoundingClientRect();
    const target = document.elementFromPoint(bounds.left + bounds.width / 2, bounds.top + bounds.height / 2);
    return target === element || element.contains(target);
  })).toBe(true);
  return control;
}

async function showIdlePresentation(page) {
  await page.evaluate(() => {
    for (const id of [
      'characterSelect', 'assignedWaiting', 'normalHeader', 'hackHeader', 'termList',
      'termEntry', 'hackBoard', 'hackBlocked', 'termOutput', 'termPrompt', 'backBtn', 'pageNav',
    ]) {
      document.getElementById(id).hidden = true;
    }
    document.querySelector('#termIdle').hidden = false;
  });
}

async function installAudioFailure(page) {
  await page.addInitScript(() => {
    class FailedAudioContext {
      constructor() { throw new Error('fixture audio context unavailable'); }
    }
    class FailedAudio {
      paused = true;
      play() { return Promise.reject(new Error('fixture playback rejected')); }
      pause() {}
    }
    Object.defineProperty(window, 'AudioContext', { configurable: true, value: FailedAudioContext });
    Object.defineProperty(window, 'webkitAudioContext', { configurable: true, value: FailedAudioContext });
    Object.defineProperty(window, 'Audio', { configurable: true, value: FailedAudio });
  });
}

export {
  CRT_VIEWPORTS,
  activateCRTFixture,
  assignPlayer,
  expectScreenContained,
  freezeAnimations,
  installAudioFailure,
  observePlayerMutations,
  resetAndOpen,
};

test.describe('CRT visual shell', () => {
  for (const viewport of CRT_VIEWPORTS) {
    test(`${viewport.name} viewport keeps the shell aligned, contained, and interactive`, async ({ page, request }) => {
      await page.setViewportSize(viewport);
      await resetAndOpen({ page, request });
      await freezeAnimations(page);
      await expectScreenContained(page);

      const geometry = await page.evaluate(() => {
        const screen = document.querySelector('#screen').getBoundingClientRect();
        const scanlines = document.querySelector('.scanlines').getBoundingClientRect();
        const vignette = document.querySelector('.vignette').getBoundingClientRect();
        return {
          screen: [screen.x, screen.y, screen.width, screen.height],
          scanlines: [scanlines.x, scanlines.y, scanlines.width, scanlines.height],
          vignette: [vignette.x, vignette.y, vignette.width, vignette.height],
        };
      });
      const insideBorder = [
        geometry.screen[0] + 2,
        geometry.screen[1] + 2,
        geometry.screen[2] - 4,
        geometry.screen[3] - 4,
      ];
      expect(geometry.scanlines).toEqual(insideBorder);
      expect(geometry.vignette).toEqual(insideBorder);
      await expect(page.locator('.scanlines')).toHaveAttribute('aria-hidden', 'true');
      await expect(page.locator('.vignette')).toHaveAttribute('aria-hidden', 'true');

      const option = page.locator('#characterOptions button:not([disabled])').first();
      await expect(option).toBeVisible();
      const hitTarget = await option.evaluate(element => {
        const bounds = element.getBoundingClientRect();
        return document.elementFromPoint(bounds.left + bounds.width / 2, bounds.top + bounds.height / 2)?.className;
      });
      expect(hitTarget).toContain('character-option');

      const colors = await page.locator('#screen').evaluate(element => {
        const style = getComputedStyle(element);
        return { background: style.backgroundColor, foreground: style.color, border: style.borderColor };
      });
      expect(colors).toEqual({
        background: 'rgb(2, 10, 2)',
        foreground: 'rgb(87, 255, 110)',
        border: 'rgb(12, 46, 12)',
      });
    });
  }

  test('connection status dominates without replacing the underlying selection state', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await page.locator('#connOverlay').evaluate(element => element.classList.remove('hidden'));
    await expect(page.locator('#connOverlay')).toBeVisible();
    await expect(page.locator('#characterSelect')).toBeVisible();
    const topmost = await page.evaluate(() => {
      const bounds = document.querySelector('#connOverlay').getBoundingClientRect();
      return document.elementFromPoint(bounds.width / 2, bounds.height / 2)?.closest('#connOverlay')?.id;
    });
    expect(topmost).toBe('connOverlay');
  });

  for (const viewport of CRT_VIEWPORTS) {
    test(`${viewport.name} viewport keeps all nine presentation states contained and operable`, async ({ page, request }) => {
      await page.setViewportSize(viewport);
      await resetAndOpen({ page, request });

      await expectStateContained(page, '#characterSelect');
      await expectControlReachable(page, '#characterOptions button:not([disabled])');

      await page.locator('#connOverlay').evaluate(element => element.classList.remove('hidden'));
      await expect(page.locator('#connOverlay')).toBeVisible();
      await expect(page.locator('#connText')).toBeVisible();
      await expectScreenContained(page);
      await page.locator('#connOverlay').evaluate(element => element.classList.add('hidden'));

      await showIdlePresentation(page);
      await expectStateContained(page, '#termIdle');

      await resetAndOpen({ page, request });
      await assignPlayer(page);
      await activateCRTFixture(request, 'waiting');
      await expectStateContained(page, '#assignedWaiting');

      await activateCRTFixture(request, 'content');
      await expect(page.locator('#termList')).toBeVisible();
      await page.keyboard.press('Shift');
      await expect(page.locator('.term-row')).toHaveCount(25);
      await expectStateContained(page, '.term-row.sel');

      await page.locator('.term-row', { hasText: 'LONG RECORD' }).click();
      await expect(page.locator('#termEntry')).toBeVisible();
      await page.keyboard.press('Shift');
      await expectStateContained(page, '#termEntry');
      await expect(page.locator('#entryBody')).toContainText('ROBCO RECORD LINE');
      const nextRecordPage = await expectControlReachable(page, '#pageNext');
      const recordPage = await page.locator('#pageIndicator').textContent();
      await nextRecordPage.click();
      await expect(page.locator('#pageIndicator')).not.toHaveText(recordPage);
      await expectStateContained(page, '#termEntry');

      await page.locator('#backBtn').click();
      await expect(page.locator('#termList')).toBeVisible();
      await page.keyboard.press('Shift');
      const command = page.locator('.term-row', { hasText: 'RUN DIAGNOSTIC' });
      await expect(command).toBeVisible();
      await command.click();
      await approveCRTCommand(page, request);
      await page.keyboard.press('Shift');
      await expectStateContained(page, '#termEntry');
      await expect(page.locator('#entryBody')).toContainText('DIAGNOSTIC OUTPUT');
      await expectControlReachable(page, '#pageNext');

      await activateCRTFixture(request, 'hacking');
      await expectStateContained(page, '#hackBoard');
      await expectControlReachable(page, '.hcell.word');

      await activateCRTFixture(request, 'blocked');
      await expect(page.locator('#hackBoard')).toBeVisible();
      for (let attempt = 0; attempt < 16 && !(await page.locator('#hackBlocked').isVisible()); attempt += 1) {
        await page.locator('.hcell.filler').nth(attempt).click();
        await page.waitForTimeout(30);
      }
      await expectStateContained(page, '#hackBlocked');
    });
  }

  test('historical focus and active colors remain exact', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    const option = page.locator('#characterOptions button:not([disabled])').first();
    await option.focus();
    await expect(option).toHaveCSS('border-color', 'rgb(216, 255, 184)');
    await expect(option).toHaveCSS('background-color', 'rgba(87, 255, 110, 0.16)');
    await option.hover();
    await page.mouse.down();
    await expect(option).toHaveCSS('background-color', 'rgb(87, 255, 110)');
    await expect(option).toHaveCSS('color', 'rgb(2, 16, 2)');
    await page.mouse.up();
  });

  for (const viewport of CRT_VIEWPORTS) {
    test(`${viewport.name} historical color states match approved snapshots`, async ({ page, request }) => {
      await page.setViewportSize(viewport);
      await resetAndOpen({ page, request });
      const option = page.locator('#characterOptions button:not([disabled])').first();
      await option.focus();
      await expect(option).toHaveScreenshot(`${viewport.name}-focused-character-option.png`, { animations: 'disabled' });

      await option.hover();
      await page.mouse.down();
      await expect(option).toHaveScreenshot(`${viewport.name}-active-character-option.png`, { animations: 'disabled' });
      await page.mouse.move(0, 0);
      await page.mouse.up();

      await option.click();
      await expect(page.locator('#termList')).toBeVisible();
      await activateCRTFixture(request, 'content');
      const selected = page.locator('.term-row.sel').first();
      await expect(selected).toBeVisible();
      await expect(selected).toHaveScreenshot(`${viewport.name}-selected-terminal-row.png`, { animations: 'disabled' });

      await activateCRTFixture(request, 'hacking');
      const hackingTarget = page.locator('.hcell.word').first();
      await hackingTarget.hover();
      const highlight = page.locator('.hcell.hi').first();
      await expect(highlight).toBeVisible();
      await expect(highlight).toHaveCSS('background-color', 'rgb(87, 255, 110)');
      const stableHighlight = page.locator('#hackInputPreview');
      await stableHighlight.evaluate(element => {
        element.classList.add('hcell', 'hi');
        element.textContent = 'TEST';
      });
      await expect(stableHighlight).toHaveScreenshot(`${viewport.name}-hacking-hover.png`, { animations: 'disabled' });
    });
  }
});

test.describe('CRT motion and reveal lifecycle', () => {
  test('flicker and hard-step indicators expose the exact historical animation model', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    const model = await page.evaluate(() => {
      const read = selector => {
        const animation = document.querySelector(selector).getAnimations()[0];
        const timing = animation.effect.getTiming();
        return {
          duration: timing.duration,
          iterations: timing.iterations,
          frames: animation.effect.getKeyframes().map(frame => ({
            offset: frame.computedOffset,
            opacity: frame.opacity,
            borderColor: frame.borderColor,
          })),
        };
      };
      return { screen: read('#screen'), blink: read('#termPrompt .blink') };
    });
    expect(model.screen.duration).toBe(6000);
    expect(model.screen.iterations).toBe(Infinity);
    expect(model.screen.frames.map(({ offset, opacity }) => ({ offset, opacity }))).toEqual([
      { offset: 0, opacity: '1' },
      { offset: 0.96, opacity: '1' },
      { offset: 0.97, opacity: '0.92' },
      { offset: 0.98, opacity: '1' },
      { offset: 0.99, opacity: '0.96' },
      { offset: 1, opacity: '1' },
    ]);
    expect(model.blink.duration).toBe(1000);
    expect(model.blink.frames.map(({ offset, opacity }) => ({ offset, opacity }))).toEqual([
      { offset: 0, opacity: '1' },
      { offset: 0.49, opacity: '1' },
      { offset: 0.5, opacity: '0' },
      { offset: 1, opacity: '0' },
    ]);
  });

  test('25 rows reveal in order within 1.2 seconds and unchanged updates do not replay', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    const started = Date.now();
    await activateCRTFixture(request, 'content');
    await expect.poll(
      () => page.locator('.term-row').count(),
      { timeout: 1200, intervals: [40] },
    ).toBe(25);
    expect(Date.now() - started).toBeLessThanOrEqual(1200);
    const labels = await page.locator('.term-row').allTextContents();
    expect(labels.slice(0, 3)).toEqual(['> ARCHIVE 01', '> ARCHIVE 02', '> ARCHIVE 03']);
    expect(labels.at(-1)).toContain('<img data-crt-injected');

    await page.evaluate(() => {
      window.__crtFirstStableRow = document.querySelector('.term-row');
      window.__crtListMutations = { added: 0, removed: 0 };
      window.__crtListObserver = new MutationObserver(records => {
        for (const record of records) {
          window.__crtListMutations.added += record.addedNodes.length;
          window.__crtListMutations.removed += record.removedNodes.length;
        }
      });
      window.__crtListObserver.observe(document.querySelector('#termList'), { childList: true });
    });

    await activateCRTFixture(request, 'unchanged');
    await page.waitForTimeout(120);
    await expect(page.locator('.term-row')).toHaveCount(25);
    expect(await page.evaluate(() => window.__crtListMutations)).toEqual({ added: 0, removed: 0 });

    const disconnect = await request.post('/__fixture/local/disconnect');
    expect(disconnect.status()).toBe(204);
    await expect(page.locator('#connOverlay')).toBeVisible();
    await expect(page.locator('#connOverlay')).toBeHidden({ timeout: 5000 });
    await expect(page.locator('.term-row')).toHaveCount(25);
    expect(await page.evaluate(() => ({
      sameFirstRow: document.querySelector('.term-row') === window.__crtFirstStableRow,
      mutations: window.__crtListMutations,
    }))).toEqual({ sameFirstRow: true, mutations: { added: 0, removed: 0 } });
  });

  test('replacement cancels stale rows and layout-only work does not replay content', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'content');
    await expect.poll(() => page.locator('.term-row').count()).toBeGreaterThan(1);
    await activateCRTFixture(request, 'replacement');
    await expect(page.locator('.term-row')).toHaveCount(3);
    await page.waitForTimeout(250);
    expect(await page.locator('.term-row').allTextContents()).toEqual([
      '> REPLACEMENT ALPHA',
      '> REPLACEMENT BETA',
      '> REPLACEMENT GAMMA',
    ]);

    await page.setViewportSize({ width: 768, height: 720 });
    await page.waitForTimeout(80);
    await expect(page.locator('.term-row')).toHaveCount(3);
  });

  test('pagination renders an already-opened page immediately', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'content');
    await expect(page.locator('.term-row', { hasText: 'LONG RECORD' })).toBeVisible();
    await page.locator('.term-row', { hasText: 'LONG RECORD' }).click();
    await expect(page.locator('#pageNext')).toBeVisible();
    const firstPage = await page.locator('#pageIndicator').textContent();
    await page.locator('#pageNext').click();
    await expect(page.locator('#pageIndicator')).not.toHaveText(firstPage);
    expect(await page.locator('#entryBody > div').count()).toBeGreaterThan(2);
    await page.setViewportSize({ width: 720, height: 640 });
    await page.waitForTimeout(80);
    expect(await page.locator('#entryBody > div').count()).toBeGreaterThan(2);
  });
});

test.describe('CRT hacking code reveal', () => {
  const hackRowCount = 32;

  for (const layout of HACK_FONT_LAYOUTS) {
    test(`${layout.name} reveal keeps one complete-board row font from first paint to completion`, async ({ page, request }) => {
      await page.setViewportSize({ width: layout.width, height: layout.height });
      await resetAndOpen({ page, request });
      await assignPlayer(page);
      if (layout.fallbackFont) {
        await page.evaluate(() => { document.body.style.fontFamily = "'Courier New', monospace"; });
      }
      await page.evaluate(() => document.fonts.ready);
      await observeHackRevealFonts(page);

      await activateCRTFixture(request, 'hacking');
      await expect(page.locator('#hackBoard')).toBeVisible();
      await expect(page.locator('.hack-row')).toHaveCount(hackRowCount, { timeout: 2000 });
      await page.waitForTimeout(80);

      const samples = await page.evaluate(() => window.__hackRevealFontSamples);
      expect(samples.length).toBeGreaterThanOrEqual(4);
      expect(samples[0].count).toBeLessThan(hackRowCount);
      expect(new Set(samples.map(sample => sample.font.toFixed(3))).size).toBe(1);
    });
  }

  test('skip completion retains the precomputed hacking row font', async ({ page, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'hacking');
    await expect.poll(() => page.locator('.hack-row').count()).toBeGreaterThan(1);

    const initialFont = await page.locator('.hack-row').first().evaluate(row =>
      Number.parseFloat(getComputedStyle(row).fontSize));
    await page.keyboard.press('Shift');
    await expect(page.locator('.hack-row')).toHaveCount(hackRowCount);
    await page.waitForTimeout(80);
    const completedFont = await page.locator('.hack-row').first().evaluate(row =>
      Number.parseFloat(getComputedStyle(row).fontSize));

    expect(completedFont).toBeCloseTo(initialFont, 3);
  });

  test('viewport, orientation, and active-font refits use queued rows without replay', async ({ page, request }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'hacking');
    await expect.poll(() => page.locator('.hack-row').count()).toBeGreaterThan(1);
    expect(await page.locator('.hack-row').count()).toBeLessThan(hackRowCount);
    await page.evaluate(() => {
      window.__hackRefitFirstRow = document.querySelector('#hackColumns .hack-row');
    });

    await page.setViewportSize({ width: 640, height: 720 });
    await page.evaluate(async () => {
      document.body.style.fontFamily = "'Courier New', monospace";
      await document.fonts.ready;
      window.dispatchEvent(new Event('resize'));
    });
    await page.waitForTimeout(100);
    expect(await page.locator('.hack-row').count()).toBeLessThan(hackRowCount);
    const partialFont = await page.locator('.hack-row').first().evaluate(row =>
      Number.parseFloat(getComputedStyle(row).fontSize));

    await page.keyboard.press('Shift');
    await expect(page.locator('.hack-row')).toHaveCount(hackRowCount);
    await page.waitForTimeout(80);
    const completed = await page.evaluate(() => ({
      font: Number.parseFloat(getComputedStyle(document.querySelector('#hackColumns .hack-row')).fontSize),
      sameFirstRow: document.querySelector('#hackColumns .hack-row') === window.__hackRefitFirstRow,
    }));
    expect(completed.font).toBeCloseTo(partialFont, 3);
    expect(completed.sameFirstRow).toBe(true);
  });

  test('new hacking generations reveal complete rows in deterministic 40ms order', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'hacking');
    await expect(page.locator('#hackBoard')).toBeVisible();
    await expect.poll(() => page.locator('.hack-row').count()).toBeGreaterThan(0);

    const initialCount = await page.locator('.hack-row').count();
    expect(initialCount).toBeLessThan(hackRowCount);
    const initialAddresses = await page.locator('.hack-addr').allTextContents();
    expect(await page.evaluate(() => {
      const rows = Array.from(document.querySelectorAll('#hackColumns .hack-row'));
      const renderedCharacters = rows.reduce((total, row) =>
        total + row.querySelector('.hack-cells').textContent.length, 0);
      return renderedCharacters === rows.length * 12 &&
        rows.every(row => row.querySelector('.hack-addr') && row.querySelectorAll('.hcell').length > 0);
    })).toBe(true);

    await page.evaluate(() => {
      window.__hackRevealTimes = [];
      window.__hackRevealObserver = new MutationObserver(records => {
        for (const record of records) {
          for (const node of record.addedNodes) {
            if (node.nodeType === Node.ELEMENT_NODE && node.classList.contains('hack-row')) {
              window.__hackRevealTimes.push(performance.now());
            }
          }
        }
      });
      window.__hackRevealObserver.observe(document.querySelector('#hackColumns'), {
        childList: true,
        subtree: true,
      });
    });

    await expect(page.locator('.hack-row')).toHaveCount(hackRowCount, { timeout: 2000 });
    const finalAddresses = await page.locator('.hack-addr').allTextContents();
    expect(initialAddresses).toEqual(finalAddresses.slice(0, initialAddresses.length));
    const timings = await page.evaluate(() => window.__hackRevealTimes);
    expect(timings.length).toBeGreaterThanOrEqual(4);
    const intervals = timings.slice(1).map((time, index) => time - timings[index]);
    const averageInterval = intervals.reduce((total, interval) => total + interval, 0) / intervals.length;
    expect(averageInterval).toBeGreaterThanOrEqual(20);
    expect(averageInterval).toBeLessThanOrEqual(90);
  });

  test('same hacking identity survives updates, reconnect, viewport, and fitting without replay', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'hacking');
    await expect.poll(() => page.locator('.hack-row').count()).toBeGreaterThan(1);

    await page.evaluate(() => {
      window.__hackFirstStableRow = document.querySelector('#hackColumns .hack-row');
      window.__hackRemovedRows = 0;
      window.__hackStableObserver = new MutationObserver(records => {
        for (const record of records) {
          window.__hackRemovedRows += Array.from(record.removedNodes)
            .filter(node => node.nodeType === Node.ELEMENT_NODE &&
              (node.classList.contains('hack-row') || node.querySelector?.('.hack-row'))).length;
        }
      });
      window.__hackStableObserver.observe(document.querySelector('#hackColumns'), {
        childList: true,
        subtree: true,
      });
    });

    await activateCRTFixture(request, 'hacking-unchanged');
    await page.waitForTimeout(120);
    await page.setViewportSize({ width: 720, height: 640 });
    await page.waitForTimeout(100);
    expect(await page.evaluate(() => ({
      sameFirstRow: document.querySelector('#hackColumns .hack-row') === window.__hackFirstStableRow,
      removedRows: window.__hackRemovedRows,
    }))).toEqual({ sameFirstRow: true, removedRows: 0 });

    const disconnect = await request.post('/__fixture/local/disconnect');
    expect(disconnect.status()).toBe(204);
    await expect(page.locator('#connOverlay')).toBeVisible();
    await expect(page.locator('#connOverlay')).toBeHidden({ timeout: 5000 });
    await expect(page.locator('.hack-row')).toHaveCount(hackRowCount);
    expect(await page.evaluate(() => ({
      sameFirstRow: document.querySelector('#hackColumns .hack-row') === window.__hackFirstStableRow,
      removedRows: window.__hackRemovedRows,
    }))).toEqual({ sameFirstRow: true, removedRows: 0 });
  });

  test('replacement generations cancel stale rows before starting one fresh reveal', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'hacking');
    await expect.poll(() => page.locator('.hack-row').count()).toBeGreaterThan(1);
    await page.evaluate(() => {
      window.__oldHackRows = Array.from(document.querySelectorAll('#hackColumns .hack-row'));
    });

    await activateCRTFixture(request, 'hacking-replacement');
    await expect.poll(() => page.locator('.hack-row').count()).toBeGreaterThan(0);
    expect(await page.locator('.hack-row').count()).toBeLessThan(hackRowCount);
    expect(await page.evaluate(() => window.__oldHackRows.every(row => !row.isConnected))).toBe(true);

    await page.evaluate(() => {
      window.__replacementRemovedRows = 0;
      window.__replacementObserver = new MutationObserver(records => {
        for (const record of records) {
          window.__replacementRemovedRows += Array.from(record.removedNodes)
            .filter(node => node.nodeType === Node.ELEMENT_NODE &&
              (node.classList.contains('hack-row') || node.querySelector?.('.hack-row'))).length;
        }
      });
      window.__replacementObserver.observe(document.querySelector('#hackColumns'), {
        childList: true,
        subtree: true,
      });
    });
    await expect(page.locator('.hack-row')).toHaveCount(hackRowCount, { timeout: 2000 });
    await page.waitForTimeout(120);
    expect(await page.evaluate(() => window.__replacementRemovedRows)).toBe(0);
  });

  test('removing a dud from a revealed row reconciles that row without replaying the board', async ({ page, request }) => {
    const mutations = observePlayerMutations(page);
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'hacking');
    await expect(page.locator('.hack-row')).toHaveCount(hackRowCount, { timeout: 2000 });
    const patternCell = await visibleHackPatternCell(page);

    await page.evaluate(() => {
      window.__dudRows = Array.from(document.querySelectorAll('#hackColumns .hack-row'));
      window.__dudRowText = window.__dudRows.map(row => row.textContent);
      window.__dudRemovedRows = 0;
      window.__dudAddedRows = 0;
      window.__dudObserver = new MutationObserver(records => {
        for (const record of records) {
          for (const node of record.removedNodes) {
            if (node.nodeType === Node.ELEMENT_NODE &&
                (node.classList.contains('hack-row') || node.querySelector?.('.hack-row'))) {
              window.__dudRemovedRows++;
            }
          }
          for (const node of record.addedNodes) {
            if (node.nodeType === Node.ELEMENT_NODE &&
                (node.classList.contains('hack-row') || node.querySelector?.('.hack-row'))) {
              window.__dudAddedRows++;
            }
          }
        }
      });
      window.__dudObserver.observe(document.querySelector('#hackColumns'), {
        childList: true,
        subtree: true,
      });
    });

    const armed = await request.post('/__fixture/local/crt/hacking-dud/revealed');
    expect(armed.status()).toBe(204);
    await patternCell.click();
    await expect(page.locator('#hackLog')).toContainText('Ложное слово удалено.');
    await expect.poll(async () => page.evaluate(() =>
      Array.from(document.querySelectorAll('#hackColumns .hack-row'))
        .filter((row, index) => row.textContent !== window.__dudRowText[index]).length,
    )).toBeGreaterThan(0);

    expect(await page.evaluate(() => {
      const current = Array.from(document.querySelectorAll('#hackColumns .hack-row'));
      return {
        count: current.length,
        sameRows: current.every((row, index) => row === window.__dudRows[index]),
        removedRows: window.__dudRemovedRows,
        addedRows: window.__dudAddedRows,
      };
    })).toEqual({ count: hackRowCount, sameRows: true, removedRows: 0, addedRows: 0 });
    await expect(page.locator('#hackColumns .hcell.hi')).toHaveCount(0);
    expect(mutations).toEqual(['ActivatePattern']);

    await page.locator('#hackHeader').hover();
    await page.keyboard.press('b');
    await expect(page.locator('#hackInputPreview')).toHaveText('b');
  });

  test('removing a dud from a pending row preserves reveal cadence and existing interaction', async ({ page, request }) => {
    const mutations = observePlayerMutations(page);
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'hacking');
    await expect.poll(() => page.locator('.hack-row').count()).toBeGreaterThan(1);
    const patternCell = await visibleHackPatternCell(page);
    const initialCount = await page.locator('.hack-row').count();
    expect(initialCount).toBeLessThan(hackRowCount);

    await page.evaluate(() => {
      window.__pendingDudRows = Array.from(document.querySelectorAll('#hackColumns .hack-row'));
      window.__pendingDudRemovedRows = 0;
      window.__pendingDudAddedTimes = [];
      window.__pendingDudObserver = new MutationObserver(records => {
        for (const record of records) {
          for (const node of record.removedNodes) {
            if (node.nodeType === Node.ELEMENT_NODE &&
                (node.classList.contains('hack-row') || node.querySelector?.('.hack-row'))) {
              window.__pendingDudRemovedRows++;
            }
          }
          for (const node of record.addedNodes) {
            if (node.nodeType === Node.ELEMENT_NODE && node.classList.contains('hack-row')) {
              window.__pendingDudAddedTimes.push(performance.now());
            }
          }
        }
      });
      window.__pendingDudObserver.observe(document.querySelector('#hackColumns'), {
        childList: true,
        subtree: true,
      });
    });

    const armed = await request.post('/__fixture/local/crt/hacking-dud/pending');
    expect(armed.status()).toBe(204);
    await patternCell.click();
    await expect(page.locator('#hackLog')).toContainText('Ложное слово удалено.');
    expect(await page.locator('.hack-row').count()).toBeLessThan(hackRowCount);
    expect(await page.evaluate(() => ({
      initialRowsStayConnected: window.__pendingDudRows.every((row, index) =>
        row.isConnected && document.querySelectorAll('#hackColumns .hack-row')[index] === row),
      removedRows: window.__pendingDudRemovedRows,
    }))).toEqual({ initialRowsStayConnected: true, removedRows: 0 });

    await expect(page.locator('.hack-row')).toHaveCount(hackRowCount, { timeout: 2000 });
    const reveal = await page.evaluate(() => ({
      initialRowsStayConnected: window.__pendingDudRows.every((row, index) =>
        document.querySelectorAll('#hackColumns .hack-row')[index] === row),
      removedRows: window.__pendingDudRemovedRows,
      times: window.__pendingDudAddedTimes,
    }));
    expect(reveal.initialRowsStayConnected).toBe(true);
    expect(reveal.removedRows).toBe(0);
    expect(reveal.times.length).toBeGreaterThanOrEqual(4);
    const intervals = reveal.times.slice(1).map((time, index) => time - reveal.times[index]);
    const averageInterval = intervals.reduce((total, interval) => total + interval, 0) / intervals.length;
    expect(averageInterval).toBeGreaterThanOrEqual(20);
    expect(averageInterval).toBeLessThanOrEqual(90);
    await expect(page.locator('#hackColumns .hcell.hi')).toHaveCount(0);
    expect(mutations).toEqual(['ActivatePattern']);

    await page.locator('#hackHeader').hover();
    await page.keyboard.press('b');
    await expect(page.locator('#hackInputPreview')).toHaveText('b');
  });

  test('a hacking key completes the board without acting and later input works normally', async ({ page, request }) => {
    const mutations = observePlayerMutations(page);
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'hacking');
    await expect.poll(() => page.locator('.hack-row').count()).toBeGreaterThan(1);
    expect(await page.locator('.hack-row').count()).toBeLessThan(hackRowCount);
    await page.locator('#hackHeader').hover();
    await expect(page.locator('#hackInputPreview')).toHaveText('');

    const started = Date.now();
    await page.keyboard.down('a');
    await expect(page.locator('.hack-row')).toHaveCount(hackRowCount);
    expect(Date.now() - started).toBeLessThanOrEqual(100);
    await expect(page.locator('#hackInputPreview')).toHaveText('');
    expect(mutations).toEqual([]);

    await page.evaluate(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', {
        key: 'a', code: 'KeyA', repeat: true, bubbles: true, cancelable: true,
      }));
    });
    await page.waitForTimeout(40);
    await expect(page.locator('#hackInputPreview')).toHaveText('');
    expect(mutations).toEqual([]);

    await page.keyboard.up('a');
    await page.keyboard.press('b');
    await expect(page.locator('#hackInputPreview')).toHaveText('b');
    expect(mutations).toEqual([]);
  });
});

test.describe('authoritative terminal presentation effects', () => {
  test('display instability preserves authored content and stops on replacement or teardown', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'display-unstable');

    const screen = page.locator('#screen');
    await expect(screen).toHaveAttribute('data-presentation-effect', 'display-unstable');
    await page.keyboard.press('Shift');
    await expect(page.locator('.term-row')).toHaveCount(25);
    await page.waitForTimeout(200);
    const authoredRows = await page.locator('.term-row').allTextContents();

    await page.evaluate(() => {
      window.__unstableContentMutations = 0;
      window.__unstableContentObserver = new MutationObserver(records => {
        window.__unstableContentMutations += records.reduce(
          (count, record) => count + record.addedNodes.length + record.removedNodes.length,
          0,
        );
      });
      window.__unstableContentObserver.observe(document.querySelector('#termBody'), {
        childList: true,
        subtree: true,
      });
      window.__unstableAnimations = document.querySelector('#screen').getAnimations({ subtree: true })
        .filter(animation => animation.animationName?.startsWith('facility-'));
    });
    await expect.poll(() => page.evaluate(() => window.__unstableAnimations.length)).toBe(2);
    await page.waitForTimeout(800);
    expect(await page.locator('.term-row').allTextContents()).toEqual(authoredRows);
    expect(await page.evaluate(() => window.__unstableContentMutations)).toBe(0);

    await activateCRTFixture(request, 'display-stable');
    await expect(screen).not.toHaveAttribute('data-presentation-effect');
    await expect.poll(() => page.evaluate(() =>
      document.querySelector('#screen').getAnimations({ subtree: true })
        .filter(animation => animation.animationName?.startsWith('facility-')).length,
    )).toBe(0);
    expect(await page.evaluate(() =>
      window.__unstableAnimations.every(animation => animation.playState === 'idle'),
    )).toBe(true);

    await activateCRTFixture(request, 'display-unstable');
    await expect(screen).toHaveAttribute('data-presentation-effect', 'display-unstable');
    const teardown = await request.post('/__fixture/local/crt/waiting');
    expect(teardown.status()).toBe(204);
    await expect(screen).not.toHaveAttribute('data-presentation-effect');
    await expect(page.locator('#assignedWaiting')).toBeVisible();
  });
});

test.describe('CRT reveal skip', () => {
  test('the reveal key completes within 100ms, is consumed with its repeats, and the next press acts normally', async ({ page, request }) => {
    const mutations = observePlayerMutations(page);
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'content');
    await expect.poll(() => page.locator('.term-row').count()).toBeGreaterThan(1);
    expect(await page.locator('.term-row').count()).toBeLessThan(25);

    const started = Date.now();
    await page.keyboard.down('Enter');
    await expect(page.locator('.term-row')).toHaveCount(25);
    expect(Date.now() - started).toBeLessThanOrEqual(100);
    await expect(page.locator('#termList')).toBeVisible();
    expect(mutations).toEqual([]);

    await page.evaluate(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', {
        key: 'Enter', code: 'Enter', repeat: true, bubbles: true, cancelable: true,
      }));
    });
    await page.waitForTimeout(50);
    await expect(page.locator('#termList')).toBeVisible();
    expect(mutations).toEqual([]);

    await page.keyboard.up('Enter');
    await page.keyboard.press('Enter');
    await expect(page.locator('#termEntry')).toBeVisible();
    await expect.poll(() => mutations.filter(value => value === 'Navigate').length).toBe(1);
  });

  test('record reveal skips while approved command pages and persistent effects continue', async ({ page, request }) => {
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'content');
    await page.keyboard.press('Shift');
    await expect(page.locator('.term-row')).toHaveCount(25);

    await page.locator('.term-row', { hasText: 'LONG RECORD' }).click();
    await expect.poll(() => page.locator('#entryBody > div').count()).toBeGreaterThan(1);
    const recordBefore = await page.locator('#entryBody > div').count();
    await page.keyboard.press('Escape');
    await expect(page.locator('#termEntry')).toBeVisible();
    expect(await page.locator('#entryBody > div').count()).toBeGreaterThan(recordBefore);
    const recordPageSize = await page.locator('#entryBody > div').count();

    await page.keyboard.press('Escape');
    await expect(page.locator('#termList')).toBeVisible();
    await page.locator('.term-row', { hasText: 'LONG RECORD' }).click();
    await page.waitForTimeout(80);
    expect(await page.locator('#entryBody > div').count()).toBeLessThan(recordPageSize);
    await page.keyboard.press('Escape');
    await page.keyboard.press('Escape');
    await expect(page.locator('#termList')).toBeVisible();

    await page.locator('.term-row', { hasText: 'RUN DIAGNOSTIC' }).click();
    await approveCRTCommand(page, request, { verifyInputLock: false });
    await expect.poll(() => page.locator('#entryBody > div').count()).toBeGreaterThan(1);
    const indicatorBefore = await page.locator('#pageIndicator').textContent();
    await page.keyboard.press('PageDown');
    await expect(page.locator('#pageIndicator')).not.toHaveText(indicatorBefore);

    const effects = await page.evaluate(() => ({
      flicker: document.querySelector('#screen').getAnimations()[0]?.playState,
      scanlines: getComputedStyle(document.querySelector('.scanlines')).display,
      vignette: getComputedStyle(document.querySelector('.vignette')).display,
    }));
    expect(effects).toEqual({ flicker: 'running', scanlines: 'block', vignette: 'block' });
  });

  test('markup-like authored content stays literal and audio failure cannot block skipping', async ({ page, request }) => {
    await installAudioFailure(page);
    const pageErrors = [];
    page.on('pageerror', error => pageErrors.push(error.message));
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'content');
    await page.keyboard.press('a');
    await expect(page.locator('.term-row')).toHaveCount(25);
    await expect(page.locator('img[data-crt-injected]')).toHaveCount(0);
    expect(await page.evaluate(() => window.__crtInjected || false)).toBe(false);

    await page.locator('.term-row', { hasText: '<img data-crt-injected' }).click();
    await page.keyboard.press('b');
    await expect(page.locator('#entryBody')).toContainText('<script>window.__crtInjected=true</script>');
    await expect(page.locator('#entryBody script')).toHaveCount(0);
    expect(await page.evaluate(() => window.__crtInjected || false)).toBe(false);
    expect(pageErrors).toEqual([]);
  });

  test('operating-system reduced-motion emulation does not disable CRT motion or progressive reveal', async ({ page, request }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await resetAndOpen({ page, request });
    await assignPlayer(page);
    await activateCRTFixture(request, 'content');
    await page.waitForTimeout(80);
    expect(await page.locator('.term-row').count()).toBeLessThan(25);
    const animations = await page.evaluate(() => ({
      flicker: document.querySelector('#screen').getAnimations()[0]?.playState,
      blink: document.querySelector('#termPrompt .blink').getAnimations()[0]?.playState,
    }));
    expect(animations).toEqual({ flicker: 'running', blink: 'running' });
  });
});
