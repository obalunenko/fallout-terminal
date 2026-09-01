import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const uplinkModulePath = fileURLToPath(new URL(
  '../../frontend/client/src/composables/usePresentationUplink.ts',
  import.meta.url,
));
const expectedAssertion = 'latest-value mailbox cancels stale stream and falls back unary without losing operation';
const transportModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/adapters/presentation-uplink-transport.ts',
  import.meta.url,
))}`;
const queueModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/composables/usePresentationQueue.ts',
  import.meta.url,
))}`;
const lifecycleModuleURL = `http://127.0.0.1:34120/@fs${uplinkModulePath}`;

test.use({ bypassCSP: true });

test(expectedAssertion, async () => {
  if (!existsSync(uplinkModulePath)) {
    process.stderr.write(`AssertionError: ${expectedAssertion}\n`);
    throw new Error('Player candidate presentation uplink lifecycle is not implemented');
  }
});

test('uplink transport validates capability envelopes and unary fallback', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const module = await import(source);
    class StreamingRequest {
      constructor(url, init) { this.body = init.body; this.duplex = init.duplex; this.url = url; }
    }
    const secureScope = {
      fetch() {},
      isSecureContext: true,
      location: { origin: 'https://terminal.example', protocol: 'https:' },
      ReadableStream,
      Request: StreamingRequest,
    };
    const calls = [];
    const rpc = {
      async presentationUplink(input, options) {
        calls.push({ aborted: options.signal.aborted, kind: 'stream', input: typeof input[Symbol.asyncIterator] === 'function' });
        return { $typeName: 'fallout.terminal.player.v1.PresentationUplinkResponse' };
      },
      async setPresentation(input, options) {
        calls.push({ aborted: options.signal.aborted, kind: 'unary', requestId: input.requestId });
        return { accepted: true, reason: 1, requestId: input.requestId, revision: 8n };
      },
    };
    const transport = module.createPresentationUplinkTransport(rpc, secureScope);
    const open = module.createPresentationOpen('client-a', 3, 'recognition-a');
    const intent = module.createPresentationIntent({
      broadcastId: 'broadcast-a',
      contextKey: 'context-a',
      presentation: { contextKey: 'context-a', presentation: { case: 'none', value: {} } },
      recognitionHandle: 'recognition-a',
      requestId: 'request-a',
      terminalId: 'terminal-a',
    });
    async function* frames() { yield open; yield intent; }
    const signal = new AbortController().signal;
    const streamed = await transport.open(frames(), signal);
    const unary = await transport.fallback({
      broadcastId: 'broadcast-a', contextKey: 'context-a', presentation: intent.payload.value.presentation,
      recognitionHandle: 'recognition-a', requestId: 'request-a', terminalId: 'terminal-a',
    }, signal);
    const expectation = { clientInstanceID: 'client-a', generation: 3, requestIDs: new Set(['request-a']) };
    const ready = module.validatePresentationResult({
      clientInstanceId: 'client-a', payload: { case: 'ready', value: {} }, uplinkGeneration: 3n,
    }, expectation);
    const action = module.validatePresentationResult({
      clientInstanceId: 'client-a', payload: { case: 'action', value: unary }, uplinkGeneration: 3n,
    }, expectation);
    const stale = module.validatePresentationResult({
      clientInstanceId: 'client-a', payload: { case: 'action', value: unary }, uplinkGeneration: 2n,
    }, expectation);
    return {
      action: action?.kind,
      calls,
      insecure: module.supportsPresentationRequestStreaming({ ...secureScope, isSecureContext: false }),
      intentCase: intent.payload.case,
      openCase: open.payload.case,
      ready,
      stale,
      streamed,
      streaming: transport.streaming,
      unary: { accepted: unary.accepted, requestId: unary.requestId, revision: Number(unary.revision) },
    };
  }, transportModuleURL);

  expect(result).toEqual({
    action: 'action',
    calls: [
      { aborted: false, input: true, kind: 'stream' },
      { aborted: false, kind: 'unary', requestId: 'request-a' },
    ],
    insecure: false,
    intentCase: 'intent',
    openCase: 'open',
    ready: { kind: 'ready' },
    stale: null,
    streamed: true,
    streaming: true,
    unary: { accepted: true, requestId: 'request-a', revision: 8 },
  });
});

test('presentation queue remains one-slot and cancels replacement work', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const { createPresentationQueue } = await import(source);
    const discarded = [];
    let controller = true;
    const queue = createPresentationQueue({
      authorize: context => controller && context.startsWith('context-'),
      onDiscard: value => discarded.push(value.id),
    });
    const first = queue.offer({ id: 'one' }, 'context-a');
    const second = queue.offer({ id: 'two' }, 'context-a');
    const third = queue.offer({ id: 'three' }, 'context-b');
    const iterator = queue[Symbol.asyncIterator]();
    const consumed = await iterator.next();
    const waiting = iterator.next();
    const delivered = queue.offer({ id: 'four' }, 'context-b');
    const direct = await waiting;
    queue.offer({ id: 'five' }, 'context-b');
    queue.invalidate('context-b');
    controller = false;
    const observerRejected = queue.offer({ id: 'observer' }, 'context-c');
    queue.close();
    const ended = await iterator.next();
    return {
      closed: queue.closed,
      consumed,
      delivered,
      direct,
      discarded,
      ended,
      first,
      observerRejected,
      pending: queue.pending,
      second,
      third,
    };
  }, queueModuleURL);

  expect(result).toEqual({
    closed: true,
    consumed: { done: false, value: { id: 'three' } },
    delivered: true,
    direct: { done: false, value: { id: 'four' } },
    discarded: ['one', 'two', 'five'],
    ended: { done: true },
    first: true,
    observerRejected: false,
    pending: false,
    second: true,
    third: true,
  });
});

test('presentation lifecycle releases streams iterators controllers and retry timers', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async source => {
    const { createPresentationUplinkController } = await import(source);
    const aborted = [];
    const fallbackCalls = [];
    const results = [];
    const streamFrames = [];
    const timers = new Map();
    let nextTimer = 1;
    const transport = {
      streaming: true,
      async fallback(input, signal) {
        fallbackCalls.push(input.requestId);
        return { accepted: true, reason: 1, requestId: input.requestId, revision: 9n };
      },
      async open(input, signal) {
        aborted.push(signal);
        for await (const frame of input) streamFrames.push(frame.payload.case);
        return true;
      },
    };
    const controller = createPresentationUplinkController({
      authorize: key => key === 'context-a' || key === 'context-b',
      clientInstanceID: 'client-a',
      clock: {
        clearTimeout(handle) { timers.delete(handle); },
        setTimeout(callback) {
          const handle = nextTimer++;
          timers.set(handle, () => { timers.delete(handle); callback(); });
          return handle;
        },
      },
      onResult: action => results.push({ requestId: action.requestId, revision: Number(action.revision) }),
      recognitionHandle: () => 'recognition-a',
      retryMilliseconds: 25,
      transport,
    });
    const makeIntent = (requestId, contextKey) => ({
      broadcastId: 'broadcast-a', contextKey,
      presentation: { contextKey, presentation: { case: 'none', value: {} } },
      recognitionHandle: 'recognition-a', requestId, terminalId: 'terminal-a',
    });
    controller.start('context-a');
    await Promise.resolve();
    const firstGeneration = controller.generation;
    controller.apply({ clientInstanceId: 'client-a', payload: { case: 'ready', value: {} }, uplinkGeneration: BigInt(firstGeneration) });
    controller.offer(makeIntent('request-a', 'context-a'));
    await Promise.resolve();
    const accepted = controller.apply({
      clientInstanceId: 'client-a',
      payload: { case: 'action', value: { accepted: true, reason: 1, requestId: 'request-a', revision: 8n } },
      uplinkGeneration: BigInt(firstGeneration),
    });
    controller.offer(makeIntent('request-b', 'context-a'));
    const fallbackTimer = [...timers.values()][0];
    fallbackTimer();
    await Promise.resolve();
    await Promise.resolve();
    controller.start('context-b');
    const late = controller.apply({
      clientInstanceId: 'client-a',
      payload: { case: 'action', value: { accepted: true, reason: 1, requestId: 'request-b', revision: 10n } },
      uplinkGeneration: BigInt(firstGeneration),
    });
    const timersBeforeDispose = timers.size;
    controller.dispose();
    await Promise.resolve();
    return {
      accepted,
      active: controller.active,
      allAborted: aborted.every(signal => signal.aborted),
      fallbackCalls,
      late,
      results,
      streamFrames,
      timersAfterDispose: timers.size,
      timersBeforeDispose,
    };
  }, lifecycleModuleURL);

  expect(result).toEqual({
    accepted: true,
    active: false,
    allAborted: true,
    fallbackCalls: ['request-b'],
    late: false,
    results: [{ requestId: 'request-a', revision: 8 }, { requestId: 'request-b', revision: 9 }],
    streamFrames: ['open', 'intent', 'intent', 'open'],
    timersAfterDispose: 0,
    timersBeforeDispose: 1,
  });
});
