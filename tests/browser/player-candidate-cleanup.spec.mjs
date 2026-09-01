import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const moduleURL = relativePath => `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(relativePath, import.meta.url))}`;

test.use({ bypassCSP: true });

test('wave-f resources release exactly once and suppress late work', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const result = await page.evaluate(async urls => {
    const [subscriptionModule, actionsModule, leaseModule, paginationModule, navigationModule] = await Promise.all([
      import(urls.subscription),
      import(urls.actions),
      import(urls.lease),
      import(urls.pagination),
      import(urls.navigation),
    ]);

    let subscriptionAborts = 0;
    let iteratorReturns = 0;
    const streamWaiters = [];
    const stream = {
      [Symbol.asyncIterator]() {
        return {
          next() { return new Promise(resolve => streamWaiters.push(resolve)); },
          return() {
            iteratorReturns += 1;
            while (streamWaiters.length > 0) streamWaiters.shift()({ done: true });
            return Promise.resolve({ done: true });
          },
        };
      },
    };
    let reconnectTimers = 0;
    let reconnectClears = 0;
    const subscription = subscriptionModule.createPlayerSubscriptionController({
      rpc: {
        subscribe(_input, options) {
          options.signal.addEventListener('abort', () => { subscriptionAborts += 1; }, { once: true });
          return stream;
        },
      },
      scheduler: {
        clearTimeout(handle) { if (!handle.cleared) { handle.cleared = true; reconnectClears += 1; } },
        setTimeout(callback, delay) {
          reconnectTimers += 1;
          return { callback, cleared: false, delay };
        },
      },
    });
    const subscriptionStart = subscription.start({ clientInstanceId: 'cleanup-client' });
    streamWaiters.shift()({
      done: false,
      value: { payload: { case: 'snapshot', value: { revision: 1n } } },
    });
    await subscriptionStart;
    streamWaiters.shift()({ done: true });
    await Promise.resolve();
    await Promise.resolve();
    subscription.dispose();
    subscription.dispose();

    let actionAbort = 0;
    let resolveAction;
    const pendingAction = new Promise(resolve => { resolveAction = resolve; });
    const actions = actionsModule.createPlayerActionsController({
      authorize: () => true,
      requestIDFactory: () => 'cleanup-request',
      rpc: {
        navigate: async () => { throw new Error('unexpected'); },
        selectCharacter: async (_input, options) => {
          options.signal.addEventListener('abort', () => { actionAbort += 1; }, { once: true });
          return pendingAction;
        },
      },
    });
    const actionStart = actions.begin({
      input: { broadcastId: 'broadcast', characterId: 'character', recognitionHandle: 'handle' },
      kind: 'selectCharacter',
    });
    await Promise.resolve();
    actions.dispose();
    actions.dispose();
    resolveAction({ accepted: true, reason: 1, requestId: 'cleanup-request', revision: 2n });
    const lateActionAccepted = await actionStart;

    let leaseListeners = 0;
    let leaseListenerReleases = 0;
    let leaseTimers = 0;
    let leaseTimerClears = 0;
    const leaseStorage = {
      contenderKey: () => null,
      listContenders: () => [],
      readLease: () => ({ expiresAt: 60_000, owner: 'other', token: 'other-token', version: 1 }),
      readRecognitionHandle: () => null,
      removeContender: () => false,
      removeLease: () => false,
      subscribe() {
        leaseListeners += 1;
        let released = false;
        return () => { if (!released) { released = true; leaseListenerReleases += 1; } };
      },
      writeContender: () => false,
      writeLease: () => false,
      writeRecognitionHandle: () => false,
    };
    const lease = leaseModule.createRecognitionLeaseController({
      lockManager: null,
      owner: 'cleanup-owner',
      retryMilliseconds: 60_000,
      scheduler: {
        clearTimeout(handle) { if (!handle.cleared) { handle.cleared = true; leaseTimerClears += 1; } },
        now: () => 0,
        setTimeout(callback, delay) {
          leaseTimers += 1;
          return { callback, cleared: false, delay };
        },
      },
      startSubscription: async () => { throw new Error('disposed lease subscribed'); },
      storage: leaseStorage,
      tokenFactory: () => 'cleanup-token',
    });
    const leaseStart = lease.start();
    await Promise.resolve();
    lease.dispose();
    lease.dispose();
    await leaseStart;

    let frameRequests = 0;
    let frameCancels = 0;
    let observerDisconnects = 0;
    let observerObserves = 0;
    let resolveFonts;
    const fontReady = new Promise(resolve => { resolveFonts = resolve; });
    const pagination = paginationModule.createPaginationMeasurementController({
      fontReady,
      frameScheduler: {
        cancelAnimationFrame() { frameCancels += 1; },
        requestAnimationFrame() { frameRequests += 1; return frameRequests; },
      },
      observerFactory: () => ({
        disconnect() { observerDisconnects += 1; },
        observe() { observerObserves += 1; },
      }),
    });
    const container = document.createElement('div');
    document.body.append(container);
    pagination.setContent(container, 'cleanup');
    pagination.dispose();
    pagination.dispose();
    resolveFonts();
    await Promise.resolve();

    const opener = document.createElement('button');
    const other = document.createElement('button');
    document.body.append(opener, other);
    const navigation = navigationModule.createTerminalNavigationController();
    navigation.apply({ mode: 1, path: ['root'] }, 1, false);
    navigation.capturePendingFocus(opener);
    navigation.apply({ mode: 1, path: ['root'] }, 1, true);
    other.focus();
    navigation.apply({ mode: 1, path: ['root'] }, 2, false);
    navigation.dispose();
    await Promise.resolve();

    return {
      actionAbort,
      focusSuppressed: document.activeElement === other,
      frameCancels,
      frameRequests,
      iteratorReturns,
      lateActionAccepted,
      leaseListenerReleases,
      leaseListeners,
      leaseTimerClears,
      leaseTimers,
      observerDisconnects,
      observerObserves,
      reconnectClears,
      reconnectTimers,
      subscriptionAborts,
    };
  }, {
    actions: moduleURL('../../frontend/client/src/composables/usePlayerActions.ts'),
    lease: moduleURL('../../frontend/client/src/composables/useRecognitionLease.ts'),
    navigation: moduleURL('../../frontend/client/src/composables/useTerminalNavigation.ts'),
    pagination: moduleURL('../../frontend/client/src/composables/usePaginationMeasurement.ts'),
    subscription: moduleURL('../../frontend/client/src/composables/usePlayerSubscription.ts'),
  });

  expect(result).toEqual({
    actionAbort: 1,
    focusSuppressed: true,
    frameCancels: 1,
    frameRequests: 1,
    iteratorReturns: 1,
    lateActionAccepted: false,
    leaseListenerReleases: 1,
    leaseListeners: 1,
    leaseTimerClears: 1,
    leaseTimers: 1,
    observerDisconnects: 1,
    observerObserves: 1,
    reconnectClears: 1,
    reconnectTimers: 1,
    subscriptionAborts: 1,
  });
});
