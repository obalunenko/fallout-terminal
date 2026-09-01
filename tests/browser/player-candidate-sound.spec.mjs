import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const soundModulePath = fileURLToPath(new URL(
  '../../frontend/client/src/composables/useTerminalSound.ts',
  import.meta.url,
));
const expectedAssertion = 'gesture unlock preserves cue volumes and unmount aborts/stops/disconnects/closes audio';
const manifestModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/adapters/sound-manifest.ts',
  import.meta.url,
))}`;
const soundModuleURL = `http://127.0.0.1:34120/@fs${soundModulePath}`;

test.use({ bypassCSP: true });

test(expectedAssertion, async () => {
  if (!existsSync(soundModulePath)) {
    process.stderr.write(`AssertionError: ${expectedAssertion}\n`);
    throw new Error('Player candidate sound lifecycle is not implemented');
  }
});

test('sound manifest accepts exact shipped assets and rejects unsafe values', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const { createSoundManifestAdapter, safeSoundAssetURL, soundCategoryByFolder } = await import(source);
    const calls = [];
    const adapter = createSoundManifestAdapter({
      async soundManifest(input, options) {
        calls.push({ aborted: options.signal?.aborted ?? false, category: input.category });
        return {
          assets: [
            'sounds/single/ui_hacking_charsingle_01.wav',
            'sounds/single/ui_hacking_charsingle_01.wav',
            'sounds/multiple/ui_hacking_charmultiple_01.wav',
            'sounds/single/nested/file.wav',
            'sounds/single/file.exe',
            'https://evil.example/sounds/single/file.wav',
            'sounds/single/../file.wav',
          ],
        };
      },
    }, 'https://terminal.example');
    const controller = new AbortController();
    const assets = await adapter.load('single', controller.signal);
    const isolated = await createSoundManifestAdapter({ async soundManifest() { throw new Error('optional unavailable'); } })
      .load('ambient');
    return {
      assets,
      calls,
      categories: Object.keys(soundCategoryByFolder).sort(),
      isolated,
      rejects: [
        safeSoundAssetURL('enter', 'sounds/enter/file.wav', 'file:///tmp'),
        safeSoundAssetURL('enter', 'sounds/enter/a/b.wav', 'https://terminal.example'),
        safeSoundAssetURL('enter', 'sounds/hack-good/file.wav', 'https://terminal.example'),
        safeSoundAssetURL('enter', 'sounds/enter/file.svg', 'https://terminal.example'),
      ],
    };
  }, manifestModuleURL);

  expect(result).toEqual({
    assets: ['/sounds/single/ui_hacking_charsingle_01.wav'],
    calls: [{ aborted: false, category: 5 }],
    categories: ['ambient', 'charscroll', 'enter', 'hack-bad', 'hack-good', 'menu-focus', 'multiple', 'single'],
    isolated: [],
    rejects: [null, null, null, null],
  });
});

test('terminal sound preserves cue volumes and releases audio resources', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const { createTerminalSoundController } = await import(source);
    const events = [];
    const gains = [];
    const gestureTarget = new EventTarget();
    const sources = [];
    const ambient = {
      loop: false,
      pause() { events.push('ambient:pause'); },
      play() { events.push('ambient:play'); return Promise.resolve(); },
      volume: 0,
    };
    const context = {
      destination: {},
      state: 'suspended',
      close() { events.push('context:close'); return Promise.resolve(); },
      createBufferSource() {
        const sourceNode = {
          buffer: null,
          connect() { events.push('source:connect'); },
          disconnect() { events.push('source:disconnect'); },
          onended: null,
          start() { events.push('source:start'); },
          stop() { events.push('source:stop'); },
        };
        sources.push(sourceNode);
        return sourceNode;
      },
      createGain() {
        const gain = { connect() { events.push('gain:connect'); }, disconnect() { events.push('gain:disconnect'); }, gain: { value: 0 } };
        gains.push(gain);
        return gain;
      },
      decodeAudioData() { events.push('decode'); return Promise.resolve({}); },
      resume() { events.push('context:resume'); this.state = 'running'; return Promise.resolve(); },
    };
    let contextCreations = 0;
    const pendingControllers = [];
    let blockFetch = false;
    const manifest = {
      load(folder, signal) {
        events.push(`manifest:${folder}:${signal?.aborted ?? false}`);
        return Promise.resolve([`/sounds/${folder}/asset.wav`]);
      },
    };
    const controller = createTerminalSoundController({
      ambientFactory() { events.push('ambient:create'); return ambient; },
      audioContextFactory() { contextCreations += 1; return context; },
      fetcher(url, { signal }) {
        events.push(`fetch:${url}`);
        if (!blockFetch) return Promise.resolve({ ok: true, arrayBuffer: () => Promise.resolve(new ArrayBuffer(1)) });
        return new Promise((resolve, reject) => {
          pendingControllers.push(signal);
          signal.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')), { once: true });
        });
      },
      gestureTarget,
      manifest,
      onPlayback: (cue, url, volume) => events.push(`play:${cue}:${url}:${volume}`),
      random: () => 0,
    });
    gestureTarget.dispatchEvent(new Event('pointerdown'));
    await Promise.resolve();
    const enter = await controller.play('enter');
    const good = await controller.play('hack-good', 12);
    const duplicate = await controller.play('hack-good', 12);
    const ambientActive = await controller.setAmbientActive(true);
    blockFetch = true;
    const pending = controller.play('single');
    await Promise.resolve();
    await Promise.resolve();
    await controller.dispose();
    const pendingResult = await pending;
    const creationsBeforeDetachedGesture = contextCreations;
    gestureTarget.dispatchEvent(new Event('keydown'));
    return {
      ambient: { loop: ambient.loop, volume: ambient.volume },
      ambientActive,
      contextCreations,
      creationsBeforeDetachedGesture,
      duplicate,
      enter,
      events,
      gainVolumes: gains.map(gain => gain.gain.value),
      good,
      pendingAborted: pendingControllers.every(signal => signal.aborted),
      pendingResult,
      sourceCount: sources.length,
    };
  }, soundModuleURL);

  expect(result.enter).toBe(true);
  expect(result.good).toBe(true);
  expect(result.duplicate).toBe(false);
  expect(result.ambientActive).toBe(true);
  expect(result.ambient).toEqual({ loop: true, volume: 0.25 });
  expect(result.gainVolumes).toEqual([0.65, 0.8]);
  expect(result.sourceCount).toBe(2);
  expect(result.pendingResult).toBe(false);
  expect(result.pendingAborted).toBe(true);
  expect(result.contextCreations).toBe(result.creationsBeforeDetachedGesture);
  expect(result.events).toContain('context:resume');
  expect(result.events).toContain('source:stop');
  expect(result.events).toContain('source:disconnect');
  expect(result.events).toContain('gain:disconnect');
  expect(result.events).toContain('ambient:pause');
  expect(result.events.at(-1)).toBe('context:close');
});
