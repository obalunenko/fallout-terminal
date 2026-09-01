import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const subscriptionModulePath = fileURLToPath(new URL(
  '../../frontend/client/src/composables/usePlayerSubscription.ts',
  import.meta.url,
));
const expectedAssertion = 'candidate rejects pre-snapshot delta and stale generation, reconnects after fixed delay, cancels prior stream';
const playerRPCModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/adapters/player-rpc.ts',
  import.meta.url,
))}`;
const playerSubscriptionModuleURL = `http://127.0.0.1:34120/@fs${subscriptionModulePath}`;
const playerActionsModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/usePlayerActions.ts',
  import.meta.url,
))}`;

test.use({ bypassCSP: true });

test('Player RPC adapter preserves all eight public RPC contracts and rejects invalid values', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async moduleURL => {
    const { createPlayerRPCAdapter, PLAYER_RPC_CONTRACTS, PlayerRPCValidationError } = await import(moduleURL);
    const calls = [];
    const action = request => ({ accepted: true, reason: 1, requestId: request.requestId, revision: 7n });
    const client = {
      subscribe(request) {
        calls.push(['subscribe', request]);
        return (async function* messages() {})();
      },
      async selectCharacter(request) { calls.push(['selectCharacter', request]); return action(request); },
      async navigate(request) { calls.push(['navigate', request]); return action(request); },
      async guess(request) { calls.push(['guess', request]); return action(request); },
      async activatePattern(request) { calls.push(['activatePattern', request]); return action(request); },
      async setPresentation(request) { calls.push(['setPresentation', request]); return action(request); },
      async presentationUplink(requests) {
        for await (const request of requests) calls.push(['presentationUplink', request]);
        return {};
      },
      async soundManifest(request) {
        calls.push(['soundManifest', request]);
        return { assets: ['sounds/ambient/obj_computerzax_hum_lp.wav'], category: request.category };
      },
    };
    const adapter = createPlayerRPCAdapter({ client });
    const common = { broadcastId: 'broadcast-1', recognitionHandle: 'handle-1', requestId: 'request-1' };
    adapter.subscribe({ clientInstanceId: 'client-1', recognitionHandle: 'handle-1' });
    await adapter.selectCharacter({ ...common, characterId: 'character-1' });
    await adapter.navigate({ ...common, terminalId: 'terminal-1', action: { case: 'back', value: {} } });
    await adapter.guess({ ...common, terminalId: 'terminal-1', target: { case: 'wordId', value: 'word-1' } });
    await adapter.activatePattern({ ...common, terminalId: 'terminal-1', patternId: 'pattern-1' });
    await adapter.setPresentation({
      ...common,
      terminalId: 'terminal-1',
      contextKey: 'context-1',
      presentation: { contextKey: 'context-1', presentation: { case: 'none', value: {} } },
    });
    await adapter.presentationUplink((async function* frames() {
      yield {
        payload: {
          case: 'open',
          value: { clientInstanceId: 'client-1', recognitionHandle: 'handle-1', uplinkGeneration: 1n },
        },
      };
    })());
    await adapter.soundManifest({ category: 1 });

    const callsBeforeInvalid = calls.length;
    const invalidMessages = [];
    for (const operation of [
      () => adapter.selectCharacter({ ...common, requestId: '', characterId: 'character-1' }),
      () => adapter.navigate({
        ...common,
        terminalId: 'terminal-1',
        action: { case: 'entry', value: { nodeId: 'не-ascii' } },
      }),
      () => adapter.guess({
        ...common,
        terminalId: 'terminal-1',
        target: { case: 'filler', value: { column: 2, character: 0 } },
      }),
      () => adapter.soundManifest({ category: 0 }),
    ]) {
      try {
        await operation();
      } catch (error) {
        invalidMessages.push({ message: error.message, validation: error instanceof PlayerRPCValidationError });
      }
    }
    return {
      callsAfterInvalid: calls.length,
      callsBeforeInvalid,
      contracts: PLAYER_RPC_CONTRACTS,
      invalidMessages,
      methods: calls.map(([method]) => method),
    };
  }, playerRPCModuleURL);

  expect(observation.contracts).toEqual([
    { cardinality: 'server_streaming', localName: 'subscribe', procedure: '/fallout.terminal.player.v1.PlayerService/Subscribe' },
    { cardinality: 'unary', localName: 'selectCharacter', procedure: '/fallout.terminal.player.v1.PlayerService/SelectCharacter' },
    { cardinality: 'unary', localName: 'navigate', procedure: '/fallout.terminal.player.v1.PlayerService/Navigate' },
    { cardinality: 'unary', localName: 'guess', procedure: '/fallout.terminal.player.v1.PlayerService/Guess' },
    { cardinality: 'unary', localName: 'activatePattern', procedure: '/fallout.terminal.player.v1.PlayerService/ActivatePattern' },
    { cardinality: 'unary', localName: 'setPresentation', procedure: '/fallout.terminal.player.v1.PlayerService/SetPresentation' },
    { cardinality: 'client_streaming', localName: 'presentationUplink', procedure: '/fallout.terminal.player.v1.PlayerService/PresentationUplink' },
    { cardinality: 'unary', localName: 'soundManifest', procedure: '/fallout.terminal.player.v1.PlayerService/SoundManifest' },
  ]);
  expect(observation.methods).toEqual([
    'subscribe', 'selectCharacter', 'navigate', 'guess', 'activatePattern', 'setPresentation',
    'presentationUplink', 'soundManifest',
  ]);
  expect(observation.invalidMessages).toHaveLength(4);
  expect(observation.invalidMessages.every(result => result.validation)).toBe(true);
  expect(observation.callsAfterInvalid).toBe(observation.callsBeforeInvalid);
});

test('subscription applies snapshot before deltas and releases superseded streams', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async moduleURL => {
    const { createPlayerSubscriptionController, PLAYER_RECONNECT_DELAY_MS } = await import(moduleURL);
    const streams = [];
    const scheduled = [];
    const states = [];
    const messages = [];

    function controlledStream() {
      const values = [];
      const waiters = [];
      let returnCalls = 0;
      const iterator = {
        next() {
          if (values.length > 0) return Promise.resolve(values.shift());
          return new Promise(resolve => waiters.push(resolve));
        },
        return() {
          returnCalls += 1;
          while (waiters.length > 0) waiters.shift()({ done: true });
          return Promise.resolve({ done: true });
        },
      };
      return {
        [Symbol.asyncIterator]() { return iterator; },
        get returnCalls() { return returnCalls; },
        push(value) {
          const result = { done: false, value };
          if (waiters.length > 0) waiters.shift()(result);
          else values.push(result);
        },
        end() {
          const result = { done: true };
          if (waiters.length > 0) waiters.shift()(result);
          else values.push(result);
        },
      };
    }

    const rpc = {
      subscribe(_request, options) {
        const stream = controlledStream();
        stream.signal = options.signal;
        streams.push(stream);
        return stream;
      },
    };
    const scheduler = {
      clearTimeout(handle) { handle.cleared = true; },
      setTimeout(callback, delay) {
        const handle = { callback, cleared: false, delay };
        scheduled.push(handle);
        return handle;
      },
    };
    const controller = createPlayerSubscriptionController({
      onMessage: message => messages.push(message.payload.case),
      onState: state => states.push({ ...state }),
      rpc,
      scheduler,
    });

    const firstStart = controller.start({ clientInstanceId: 'client-1' });
    streams[0].push({ payload: { case: 'update', value: { revision: 1n } } });
    let preSnapshotError = '';
    try { await firstStart; } catch (error) { preSnapshotError = error.message; }
    const reconnect = scheduled.at(-1);
    reconnect.callback();
    await Promise.resolve();
    streams[1].push({ payload: { case: 'snapshot', value: { revision: 5n } } });
    await Promise.resolve();
    await Promise.resolve();
    streams[1].push({ payload: { case: 'update', value: { revision: 4n } } });
    streams[1].push({ payload: { case: 'update', value: { revision: 6n } } });
    await Promise.resolve();
    await Promise.resolve();

    const replacementStart = controller.start({ clientInstanceId: 'client-2' });
    streams[2].push({ payload: { case: 'snapshot', value: { revision: 8n } } });
    await replacementStart;
    streams[1].push({ payload: { case: 'update', value: { revision: 99n } } });
    await Promise.resolve();
    controller.dispose();
    controller.dispose();

    return {
      aborts: streams.map(stream => stream.signal.aborted),
      finalState: controller.state,
      messages,
      preSnapshotError,
      reconnectDelay: reconnect.delay,
      returnCalls: streams.map(stream => stream.returnCalls),
      revisions: states.map(state => state.revision),
    };
  }, playerSubscriptionModuleURL);

  expect(observation.preSnapshotError).toBe('first subscription value is not a complete snapshot');
  expect(observation.reconnectDelay).toBe(3000);
  expect(observation.reconnectDelay).toBe(await page.evaluate(url => import(url).then(module => module.PLAYER_RECONNECT_DELAY_MS), playerSubscriptionModuleURL));
  expect(observation.messages).toEqual(['snapshot', 'update', 'snapshot']);
  expect(observation.revisions).toContain(5);
  expect(observation.revisions).toContain(6);
  expect(observation.revisions).toContain(8);
  expect(observation.revisions).not.toContain(4);
  expect(observation.revisions).not.toContain(99);
  expect(observation.aborts).toEqual([true, true, true]);
  expect(observation.returnCalls).toEqual([1, 1, 1]);
  expect(observation.finalState.phase).toBe('stopped');
});

test(expectedAssertion, async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const evidence = await page.evaluate(async urls => {
    const [subscriptionModule, actionsModule] = await Promise.all([
      import(urls.subscription),
      import(urls.actions),
    ]);
    const streams = [];
    const timers = [];
    const messages = [];
    const controlledStream = () => {
      const queued = [];
      const waiters = [];
      let returnCalls = 0;
      const iterator = {
        next() {
          if (queued.length > 0) return Promise.resolve(queued.shift());
          return new Promise(resolve => waiters.push(resolve));
        },
        return() {
          returnCalls += 1;
          while (waiters.length > 0) waiters.shift()({ done: true });
          return Promise.resolve({ done: true });
        },
      };
      const deliver = result => {
        if (waiters.length > 0) waiters.shift()(result);
        else queued.push(result);
      };
      return {
        [Symbol.asyncIterator]: () => iterator,
        end: () => deliver({ done: true }),
        get returnCalls() { return returnCalls; },
        push: value => deliver({ done: false, value }),
      };
    };
    const rpc = {
      subscribe(_input, options) {
        const stream = controlledStream();
        stream.signal = options.signal;
        streams.push(stream);
        return stream;
      },
    };
    const scheduler = {
      clearTimeout(handle) { handle.cleared = true; },
      setTimeout(callback, delay) {
        const handle = { callback, cleared: false, delay };
        timers.push(handle);
        return handle;
      },
    };
    const subscription = subscriptionModule.createPlayerSubscriptionController({
      onMessage: message => messages.push(message.payload.case),
      rpc,
      scheduler,
    });

    const invalidStart = subscription.start({ clientInstanceId: 'client' });
    streams[0].push({ payload: { case: 'update', value: { revision: 1n } } });
    let preSnapshotRejected = false;
    try { await invalidStart; } catch { preSnapshotRejected = true; }
    timers[0].callback();
    await Promise.resolve();
    streams[1].push({ payload: { case: 'snapshot', value: { revision: 5n } } });
    await Promise.resolve();
    await Promise.resolve();
    streams[1].push({ payload: { case: 'update', value: { revision: 4n } } });
    streams[1].end();
    await Promise.resolve();
    await Promise.resolve();
    const recoveryTimer = timers.at(-1);
    recoveryTimer.callback();
    await Promise.resolve();
    streams[2].push({ payload: { case: 'snapshot', value: { revision: 8n } } });
    await Promise.resolve();
    await Promise.resolve();
    streams[1].push({ payload: { case: 'update', value: { revision: 99n } } });

    const actionRPC = {
      navigate: async input => ({ accepted: true, reason: 1, requestId: input.requestId, revision: 9n }),
      selectCharacter: async input => ({ accepted: true, reason: 1, requestId: input.requestId, revision: 9n }),
    };
    const actions = actionsModule.createPlayerActionsController({
      authorize: () => true,
      requestIDFactory: () => 'correlated-request',
      rpc: actionRPC,
    });
    actions.applyRevision(8);
    const actionAccepted = await actions.begin({
      input: { broadcastId: 'broadcast', characterId: 'character', recognitionHandle: 'handle' },
      kind: 'selectCharacter',
    });
    const pendingBeforeRevision = actions.state.pending?.acceptedRevision;
    actions.applyRevision(9);
    const pendingAfterRevision = actions.state.pending;

    subscription.dispose();
    actions.dispose();
    return {
      aborts: streams.map(stream => stream.signal.aborted),
      actionAccepted,
      messages,
      pendingAfterRevision,
      pendingBeforeRevision,
      preSnapshotRejected,
      reconnectDelays: timers.map(timer => timer.delay),
      returnCalls: streams.map(stream => stream.returnCalls),
      revision: subscription.state.revision,
    };
  }, { actions: playerActionsModuleURL, subscription: playerSubscriptionModuleURL });

  expect(evidence.preSnapshotRejected).toBe(true);
  expect(evidence.reconnectDelays).toEqual([3000, 3000]);
  expect(evidence.messages).toEqual(['snapshot', 'snapshot']);
  expect(evidence.actionAccepted).toBe(true);
  expect(evidence.pendingBeforeRevision).toBe(9);
  expect(evidence.pendingAfterRevision).toBeNull();
  expect(evidence.aborts).toEqual([true, true, true]);
  expect(evidence.returnCalls).toEqual([1, 1, 1]);
  expect(evidence.revision).toBe(0);
});
