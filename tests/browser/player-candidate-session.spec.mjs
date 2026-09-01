import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const playerAppFixtureURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  './fixtures/player-app.html',
  import.meta.url,
))}`;

test('candidate App owns one session one stream and cascading cleanup', async ({ page }) => {
  await page.addInitScript(() => {
    const evidence = {
      aborts: 0,
      handles: [],
      listenerAdds: 0,
      listenerRemoves: 0,
      streamReturns: 0,
      streams: 0,
    };
    let recognitionHandle = null;
    const listeners = new Set();
    const storage = {
      contenderKey: owner => `contender.${owner}`,
      listContenders: () => [],
      readLease: () => null,
      readRecognitionHandle: () => recognitionHandle,
      removeContender: () => true,
      removeLease: () => true,
      subscribe(listener) {
        evidence.listenerAdds += 1;
        listeners.add(listener);
        return () => {
          if (listeners.delete(listener)) evidence.listenerRemoves += 1;
        };
      },
      writeContender: () => true,
      writeLease: () => true,
      writeRecognitionHandle(handle) {
        recognitionHandle = handle;
        evidence.handles.push(handle);
        for (const listener of listeners) listener();
        return true;
      },
    };
    const rpc = {
      subscribe(input, options) {
        evidence.streams += 1;
        const signal = options?.signal;
        signal?.addEventListener('abort', () => { evidence.aborts += 1; }, { once: true });
        let delivered = false;
        return {
          [Symbol.asyncIterator]() {
            return {
              async next() {
                if (!delivered) {
                  delivered = true;
                  return {
                    done: false,
                    value: {
                      payload: {
                        case: 'snapshot',
                        value: {
                          playerState: {
                            fallbackName: 'PLAYER 1',
                            logicalSessionId: 'logical-session',
                            phase: 1,
                            role: 1,
                            roster: [],
                          },
                          recognitionHandle: 'shared-handle',
                          revision: 1n,
                          terminalPresentation: { presentation: { case: 'noLiveTerminal', value: {} } },
                        },
                      },
                    },
                  };
                }
                await new Promise(resolve => signal?.addEventListener('abort', resolve, { once: true }));
                return { done: true, value: undefined };
              },
              async return() {
                evidence.streamReturns += 1;
                return { done: true, value: undefined };
              },
            };
          },
        };
      },
      activatePattern: async () => { throw new Error('unexpected'); },
      guess: async () => { throw new Error('unexpected'); },
      navigate: async () => { throw new Error('unexpected'); },
      presentationUplink: async () => ({}),
      selectCharacter: async () => { throw new Error('unexpected'); },
      setPresentation: async () => { throw new Error('unexpected'); },
      soundManifest: async () => ({ assets: [], category: 1 }),
    };
    window.__playerCandidateEvidence = evidence;
    window.__playerAppTestDependencies = {
      clientInstanceID: 'candidate-client',
      rpc,
      storage,
    };
  });

  await page.goto(playerAppFixtureURL);
  await expect.poll(() => page.evaluate(() => window.__playerCandidateEvidence.handles.length)).toBe(1);
  await expect(page.locator('#connOverlay')).toBeHidden();
  expect(await page.evaluate(() => ({
    handles: window.__playerCandidateEvidence.handles,
    listenerAdds: window.__playerCandidateEvidence.listenerAdds,
    streams: window.__playerCandidateEvidence.streams,
  }))).toEqual({ handles: ['shared-handle'], listenerAdds: 1, streams: 1 });

  await page.evaluate(() => window.__playerAppFixture.unmount());
  await expect.poll(() => page.evaluate(() => ({
    aborts: window.__playerCandidateEvidence.aborts,
    listenerRemoves: window.__playerCandidateEvidence.listenerRemoves,
    streamReturns: window.__playerCandidateEvidence.streamReturns,
  }))).toEqual({ aborts: 1, listenerRemoves: 1, streamReturns: 1 });
  await expect(page.locator('#playerApp').locator('*')).toHaveCount(0);
});
