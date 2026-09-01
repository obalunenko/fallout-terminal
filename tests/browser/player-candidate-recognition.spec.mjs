import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

const recognitionLeaseModulePath = fileURLToPath(new URL(
  '../../frontend/client/src/composables/useRecognitionLease.ts',
  import.meta.url,
));
const expectedAssertion = 'qualifying tabs converge to one logical session and each owns one stream';
const recognitionStorageModuleURL = `http://127.0.0.1:34120/@fs${fileURLToPath(new URL(
  '../../frontend/client/src/adapters/recognition-storage.ts',
  import.meta.url,
))}`;
const recognitionLeaseModuleURL = `http://127.0.0.1:34120/@fs${recognitionLeaseModulePath}`;

test.use({ bypassCSP: true });

test('recognition storage validates values and releases its listener', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async moduleURL => {
    const module = await import(moduleURL);
    const values = new Map();
    let denied = false;
    const storage = {
      get length() { if (denied) throw new Error('denied'); return values.size; },
      clear() { values.clear(); },
      getItem(key) { if (denied) throw new Error('denied'); return values.get(key) ?? null; },
      key(index) { if (denied) throw new Error('denied'); return [...values.keys()][index] ?? null; },
      removeItem(key) { if (denied) throw new Error('denied'); values.delete(key); },
      setItem(key, value) { if (denied) throw new Error('denied'); values.set(key, String(value)); },
    };
    const listeners = new Set();
    let addCalls = 0;
    let removeCalls = 0;
    const eventTarget = {
      addEventListener(_type, listener) { addCalls += 1; listeners.add(listener); },
      removeEventListener(_type, listener) { removeCalls += 1; listeners.delete(listener); },
    };
    const adapter = module.createRecognitionStorage({ eventTarget, storage });
    const record = { expiresAt: 5000, owner: 'owner-a', token: 'token-a', version: 1 };
    const handleWritten = adapter.writeRecognitionHandle('opaque-handle');
    const leaseWritten = adapter.writeLease(record);
    const contenderWritten = adapter.writeContender(record);
    const valid = {
      contenders: adapter.listContenders(1000),
      handle: adapter.readRecognitionHandle(),
      lease: adapter.readLease(),
    };

    values.set(module.PLAYER_SESSION_INIT_LEASE_KEY, '{malformed');
    const malformedLease = adapter.readLease();
    values.set(module.PLAYER_SESSION_INIT_LEASE_KEY, JSON.stringify({ ...record, version: 2 }));
    const wrongVersion = adapter.readLease();
    values.set(module.PLAYER_SESSION_INIT_LEASE_KEY, JSON.stringify({ ...record, expiresAt: Number.MAX_SAFE_INTEGER + 1 }));
    const unsafeExpiry = adapter.readLease();
    values.delete(`${module.PLAYER_SESSION_INIT_CONTENDER_PREFIX}owner-a`);
    values.set(`${module.PLAYER_SESSION_INIT_CONTENDER_PREFIX}owner-b`, JSON.stringify(record));
    const mismatchedContenders = adapter.listContenders(1000);

    let notifications = 0;
    const release = adapter.subscribe(() => { notifications += 1; });
    const validEvent = new StorageEvent('storage', {
      key: module.PLAYER_RECOGNITION_HANDLE_KEY,
      newValue: 'new-handle',
    });
    const invalidEvent = new StorageEvent('storage', {
      key: module.PLAYER_SESSION_INIT_LEASE_KEY,
      newValue: '{bad',
    });
    for (const listener of listeners) listener(validEvent);
    for (const listener of listeners) listener(invalidEvent);
    release();
    release();
    for (const listener of listeners) listener(validEvent);

    denied = true;
    const deniedResult = {
      contenders: adapter.listContenders(1000),
      handle: adapter.readRecognitionHandle(),
      lease: adapter.readLease(),
      writeHandle: adapter.writeRecognitionHandle('another-handle'),
      writeLease: adapter.writeLease(record),
    };
    return {
      addCalls,
      contenderWritten,
      deniedResult,
      handleWritten,
      invalid: { malformedLease, mismatchedContenders, unsafeExpiry, wrongVersion },
      leaseWritten,
      notifications,
      removeCalls,
      valid,
    };
  }, recognitionStorageModuleURL);

  expect(observation.handleWritten).toBe(true);
  expect(observation.leaseWritten).toBe(true);
  expect(observation.contenderWritten).toBe(true);
  expect(observation.valid.handle).toBe('opaque-handle');
  expect(observation.valid.lease).toEqual({ expiresAt: 5000, owner: 'owner-a', token: 'token-a', version: 1 });
  expect(observation.valid.contenders).toEqual([{ expiresAt: 5000, owner: 'owner-a', token: 'token-a', version: 1 }]);
  expect(observation.invalid).toEqual({
    malformedLease: null,
    mismatchedContenders: [],
    unsafeExpiry: null,
    wrongVersion: null,
  });
  expect(observation.notifications).toBe(1);
  expect({ add: observation.addCalls, remove: observation.removeCalls }).toEqual({ add: 1, remove: 1 });
  expect(observation.deniedResult).toEqual({
    contenders: [],
    handle: null,
    lease: null,
    writeHandle: false,
    writeLease: false,
  });
});

test('recognition lease recovers expiry and releases timers and locks', async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async ({ leaseURL, storageURL }) => {
    const leaseModule = await import(leaseURL);
    const storageModule = await import(storageURL);
    const createFixture = () => {
      const values = new Map();
      const listeners = new Set();
      const storage = {
        get length() { return values.size; },
        clear() { values.clear(); },
        getItem(key) { return values.get(key) ?? null; },
        key(index) { return [...values.keys()][index] ?? null; },
        removeItem(key) { values.delete(key); },
        setItem(key, value) { values.set(key, String(value)); },
      };
      const eventTarget = {
        addEventListener(_type, listener) { listeners.add(listener); },
        removeEventListener(_type, listener) { listeners.delete(listener); },
      };
      return {
        adapter: storageModule.createRecognitionStorage({ eventTarget, storage }),
        listenerCount: () => listeners.size,
        values,
      };
    };
    const activeTimers = new Set();
    const scheduler = {
      clearTimeout(handle) { clearTimeout(handle); activeTimers.delete(handle); },
      now: () => Date.now(),
      setTimeout(callback, delay) {
        const handle = setTimeout(() => { activeTimers.delete(handle); callback(); }, delay);
        activeTimers.add(handle);
        return handle;
      },
    };

    const lockFixture = createFixture();
    let activeLocks = 0;
    let lockHeldDuringSubscription = false;
    let lockSignal;
    const lockController = leaseModule.createRecognitionLeaseController({
      lockManager: {
        async request(name, signal, callback) {
          if (name !== storageModule.PLAYER_SESSION_INIT_LOCK_NAME) throw new Error('wrong lock');
          lockSignal = signal;
          activeLocks += 1;
          try { await callback(); } finally { activeLocks -= 1; }
        },
      },
      owner: 'lock-owner',
      scheduler,
      startSubscription: async handle => {
        lockHeldDuringSubscription = activeLocks === 1 && handle === '';
        lockFixture.adapter.writeRecognitionHandle('shared-handle');
      },
      storage: lockFixture.adapter,
      tokenFactory: () => 'lock-token',
    });
    await lockController.start();
    lockController.dispose();
    lockController.dispose();

    const fallbackFixture = createFixture();
    fallbackFixture.adapter.writeLease({
      expiresAt: Date.now() - 1,
      owner: 'expired-owner',
      token: 'expired-token',
      version: 1,
    });
    const fallbackStarts = [];
    const fallbackController = leaseModule.createRecognitionLeaseController({
      electionMilliseconds: 1,
      leaseMilliseconds: 50,
      lockManager: { request: async () => { throw new Error('locks unavailable'); } },
      owner: 'fallback-owner',
      retryMilliseconds: 1,
      scheduler,
      startSubscription: async handle => {
        fallbackStarts.push(handle);
        fallbackFixture.adapter.writeRecognitionHandle('fallback-handle');
      },
      storage: fallbackFixture.adapter,
      tokenFactory: () => 'fallback-token',
    });
    await fallbackController.start();
    fallbackController.dispose();

    const waitingFixture = createFixture();
    waitingFixture.adapter.writeLease({
      expiresAt: Date.now() + 60_000,
      owner: 'other-owner',
      token: 'other-token',
      version: 1,
    });
    const waitingController = leaseModule.createRecognitionLeaseController({
      lockManager: null,
      owner: 'waiting-owner',
      retryMilliseconds: 60_000,
      scheduler,
      startSubscription: async () => { throw new Error('disposed waiter subscribed'); },
      storage: waitingFixture.adapter,
      tokenFactory: () => 'waiting-token',
    });
    const waitingStart = waitingController.start();
    await new Promise(resolve => setTimeout(resolve, 0));
    waitingController.dispose();
    await waitingStart;

    return {
      activeLocks,
      activeTimers: activeTimers.size,
      fallbackHandle: fallbackFixture.adapter.readRecognitionHandle(),
      fallbackLease: fallbackFixture.adapter.readLease(),
      fallbackListeners: fallbackFixture.listenerCount(),
      fallbackStarts,
      lockAborted: lockSignal?.aborted ?? false,
      lockHeldDuringSubscription,
      lockListeners: lockFixture.listenerCount(),
      waitingListeners: waitingFixture.listenerCount(),
    };
  }, { leaseURL: recognitionLeaseModuleURL, storageURL: recognitionStorageModuleURL });

  expect(observation).toEqual({
    activeLocks: 0,
    activeTimers: 0,
    fallbackHandle: 'fallback-handle',
    fallbackLease: null,
    fallbackListeners: 0,
    fallbackStarts: [''],
    lockAborted: true,
    lockHeldDuringSubscription: true,
    lockListeners: 0,
    waitingListeners: 0,
  });
});

test(expectedAssertion, async ({ page }) => {
  await page.goto('http://127.0.0.1:34120/');
  const observation = await page.evaluate(async ({ leaseURL, storageURL }) => {
    const [leaseModule, storageModule] = await Promise.all([import(leaseURL), import(storageURL)]);
    const values = new Map();
    const listeners = new Set();
    const dispatch = (key, newValue) => {
      for (const listener of [...listeners]) listener({ key, newValue });
    };
    const storage = {
      get length() { return values.size; },
      clear() { values.clear(); },
      getItem(key) { return values.get(key) ?? null; },
      key(index) { return [...values.keys()][index] ?? null; },
      removeItem(key) { values.delete(key); dispatch(key, null); },
      setItem(key, value) { const next = String(value); values.set(key, next); dispatch(key, next); },
    };
    const eventTarget = {
      addEventListener(_type, listener) { listeners.add(listener); },
      removeEventListener(_type, listener) { listeners.delete(listener); },
    };
    const adapterA = storageModule.createRecognitionStorage({ eventTarget, storage });
    const adapterB = storageModule.createRecognitionStorage({ eventTarget, storage });
    const starts = [];
    const logicalSessions = [];
    const controller = (owner, token, adapter) => leaseModule.createRecognitionLeaseController({
      electionMilliseconds: 2,
      leaseMilliseconds: 50,
      lockManager: null,
      owner,
      retryMilliseconds: 2,
      startSubscription: async handle => {
        starts.push({ handle, owner });
        if (handle === '') adapter.writeRecognitionHandle('shared-handle');
        logicalSessions.push({ handle: adapter.readRecognitionHandle(), owner });
      },
      storage: adapter,
      tokenFactory: () => token,
    });
    const first = controller('owner-a', 'token-a', adapterA);
    const second = controller('owner-b', 'token-b', adapterB);
    await Promise.all([first.start(), second.start()]);
    first.dispose();
    second.dispose();

    storage.removeItem(storageModule.PLAYER_RECOGNITION_HANDLE_KEY);
    adapterA.writeLease({ expiresAt: Date.now() - 1, owner: 'expired', token: 'expired-token', version: 1 });
    const expiryStarts = [];
    const expiry = leaseModule.createRecognitionLeaseController({
      electionMilliseconds: 1,
      leaseMilliseconds: 20,
      lockManager: null,
      owner: 'owner-c',
      retryMilliseconds: 1,
      startSubscription: async handle => {
        expiryStarts.push(handle);
        adapterA.writeRecognitionHandle('recovered-handle');
      },
      storage: adapterA,
      tokenFactory: () => 'token-c',
    });
    await expiry.start();
    expiry.dispose();

    storage.removeItem(storageModule.PLAYER_RECOGNITION_HANDLE_KEY);
    storage.setItem(storageModule.PLAYER_SESSION_INIT_LEASE_KEY, '{malformed');
    const malformedStarts = [];
    const malformed = leaseModule.createRecognitionLeaseController({
      electionMilliseconds: 1,
      lockManager: null,
      owner: 'owner-d',
      retryMilliseconds: 1,
      startSubscription: async handle => { malformedStarts.push(handle); },
      storage: adapterA,
      tokenFactory: () => 'token-d',
    });
    await malformed.start();
    malformed.dispose();

    const deniedStorage = {
      get length() { throw new Error('denied'); },
      clear() { throw new Error('denied'); },
      getItem() { throw new Error('denied'); },
      key() { throw new Error('denied'); },
      removeItem() { throw new Error('denied'); },
      setItem() { throw new Error('denied'); },
    };
    const deniedAdapter = storageModule.createRecognitionStorage({ eventTarget, storage: deniedStorage });
    const deniedStarts = [];
    const denied = leaseModule.createRecognitionLeaseController({
      lockManager: null,
      owner: 'owner-e',
      startSubscription: async handle => { deniedStarts.push(handle); },
      storage: deniedAdapter,
      tokenFactory: () => 'token-e',
    });
    await denied.start();
    denied.dispose();

    return {
      deniedStarts,
      expiryStarts,
      lease: adapterA.readLease(),
      listenerCount: listeners.size,
      logicalSessions,
      malformedStarts,
      starts,
    };
  }, { leaseURL: recognitionLeaseModuleURL, storageURL: recognitionStorageModuleURL });

  expect(observation.starts).toHaveLength(2);
  expect(observation.starts.filter(start => start.handle === '')).toHaveLength(1);
  expect(observation.starts.filter(start => start.handle === 'shared-handle')).toHaveLength(1);
  expect(new Set(observation.logicalSessions.map(session => session.handle))).toEqual(new Set(['shared-handle']));
  expect(new Set(observation.logicalSessions.map(session => session.owner)).size).toBe(2);
  expect(observation.expiryStarts).toEqual(['']);
  expect(observation.malformedStarts).toEqual(['']);
  expect(observation.deniedStarts).toEqual(['']);
  expect(observation.lease).toBeNull();
  expect(observation.listenerCount).toBe(0);
});
