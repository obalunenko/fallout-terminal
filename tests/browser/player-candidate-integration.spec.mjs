import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const playerAppFixtureURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  './fixtures/player-app.html',
  import.meta.url,
))}`;

test('candidate App integrates hacking timing sound uplink with one cleanup owner', async ({ page }) => {
  await page.addInitScript(() => {
    const evidence = { aborts: 0, guesses: 0, listenerAdds: 0, listenerRemoves: 0, presentations: 0 };
    const trackedTypes = new Set(['keydown', 'keyup', 'pointerdown']);
    const originalAdd = Document.prototype.addEventListener;
    const originalRemove = Document.prototype.removeEventListener;
    Document.prototype.addEventListener = function(type, listener, options) {
      if (trackedTypes.has(type)) evidence.listenerAdds += 1;
      return originalAdd.call(this, type, listener, options);
    };
    Document.prototype.removeEventListener = function(type, listener, options) {
      if (trackedTypes.has(type)) evidence.listenerRemoves += 1;
      return originalRemove.call(this, type, listener, options);
    };
    const storage = {
      contenderKey: owner => `contender.${owner}`, listContenders: () => [], readLease: () => null,
      readRecognitionHandle: () => null, removeContender: () => true, removeLease: () => true,
      subscribe: () => () => undefined, writeContender: () => true, writeLease: () => true,
      writeRecognitionHandle: () => true,
    };
    const rpc = {
      subscribe(input, options) {
        const signal = options.signal;
        signal.addEventListener('abort', () => { evidence.aborts += 1; }, { once: true });
        let delivered = false;
        return { [Symbol.asyncIterator]() { return {
          async next() {
            if (!delivered) {
              delivered = true;
              const snapshot = {
                playerState: {
                  activeTerminalId: 'terminal-a', assignedCharacter: { characterId: 'character-a', displayName: 'PLAYER' },
                  broadcastId: 'broadcast-a', fallbackName: 'PLAYER', logicalSessionId: 'session-a', phase: 4, role: 2, roster: [],
                },
                recognitionHandle: 'recognition-a', revision: 1n,
                terminalPresentation: { presentation: { case: 'liveTerminal', value: {
                  controllerPresentation: { contextKey: 'context-a', presentation: { case: 'none', value: {} } },
                  hackLevel: 1,
                  hacking: {
                    attemptsLeft: 4, attemptsMax: 4,
                    columns: [{ addresses: ['0xF000'], text: '..WORD......', words: [{ id: 'word-a', length: 4, start: 2 }] }],
                    failed: false, level: 1, log: ['READY'], patterns: [], solved: false, wordLength: 4,
                  },
                  introText: '', navigation: { mode: 1, path: ['root'] },
                  terminalId: 'terminal-a', terminalName: 'TERMINAL',
                  tree: { content: { case: 'folder', value: { children: [] } }, id: 'root', name: 'ROOT' },
                } } },
              };
              return { done: false, value: { payload: { case: 'snapshot', value: snapshot } } };
            }
            await new Promise(resolve => signal.addEventListener('abort', resolve, { once: true }));
            return { done: true };
          },
          async return() { return { done: true }; },
        }; } };
      },
      async activatePattern(input) { return { accepted: true, reason: 1, requestId: input.requestId, revision: 2n }; },
      async guess(input) { evidence.guesses += 1; return { accepted: true, reason: 1, requestId: input.requestId, revision: 2n }; },
      async navigate() { throw new Error('unexpected navigation'); },
      async presentationUplink() { return { $typeName: 'fallout.terminal.player.v1.PresentationUplinkResponse' }; },
      async selectCharacter() { throw new Error('unexpected selection'); },
      async setPresentation(input) { evidence.presentations += 1; return { accepted: true, reason: 1, requestId: input.requestId, revision: 2n }; },
      async soundManifest(input) { return { assets: [], category: input.category }; },
    };
    window.__playerCandidateIntegrationEvidence = evidence;
    window.__playerAppTestDependencies = { clientInstanceID: 'client-a', rpc, storage };
  });

  await page.goto(playerAppFixtureURL);
  await expect(page.locator('#playerApp[data-lifecycle-owner="vue"]')).toHaveCount(1);
  await expect(page.locator('#hackBoard .hcell.word[data-target="word-a"]')).toHaveText('WORD');
  await page.locator('#hackBoard .hcell.word[data-target="word-a"]').dispatchEvent('mouseover');
  await page.locator('#hackBoard .hcell.word[data-target="word-a"]').dispatchEvent('click');
  await expect.poll(() => page.evaluate(() => window.__playerCandidateIntegrationEvidence.guesses)).toBe(1);
  await page.evaluate(() => window.__playerAppFixture.unmount());
  await expect(page.locator('#playerApp').locator('*')).toHaveCount(0);
  const evidence = await page.evaluate(() => window.__playerCandidateIntegrationEvidence);
  expect(evidence.aborts).toBe(1);
  expect(evidence.listenerRemoves).toBe(evidence.listenerAdds);
  expect(evidence.presentations).toBe(1);
});
