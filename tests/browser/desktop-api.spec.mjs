import { expect, test } from '@playwright/test';
import { readFile } from 'node:fs/promises';

const boundaryManifest = JSON.parse(await readFile(
  new URL('./fixtures/frontend-boundary-manifest.json', import.meta.url),
  'utf8',
));

test.beforeEach(async ({ page }) => {
  await page.goto('/__fixture/desktop-api');
  await expect.poll(() => page.evaluate(() => typeof window.desktopAPI)).toBe('object');
});

test('desktop adapter rejected fixture has no-state-change assertion', async ({ page }) => {
  const fixture = boundaryManifest.fixtures.find(candidate =>
    candidate.fixtureId === 'desktop-invalid-runtime-status-rejected');
  expect(fixture).toBeDefined();
  const legacyObservation = await page.evaluate(() => ({
    callCount: __desktopFixture.calls.length,
    typedAdapterAvailable: typeof window.__typedDesktopAdapter === 'object',
  }));

  if (!legacyObservation.typedAdapterAvailable) {
    process.stderr.write('AssertionError: desktop adapter rejected fixture has no-state-change assertion\n');
    throw new Error('typed desktop adapter rejection assertion is not implemented');
  }
});

test('desktop facade retains one v2 service with 39 methods and seven named events', async ({ page }) => {
  const contract = await page.evaluate(async () => {
    const imports = JSON.parse(document.querySelector('script[type="importmap"]').textContent).imports;
    const servicePaths = Object.keys(imports)
      .filter(path => path.endsWith('/desktopservice.js'))
      .sort();
    const bindings = await import(servicePaths[0]);
    const methods = Object.entries(bindings)
      .filter(([, value]) => typeof value === 'function')
      .map(([name]) => name)
      .sort();

    const releases = [
      desktopAPI.onServerInfo(() => {}),
      desktopAPI.onClientCount(() => {}),
      desktopAPI.onHackState(() => {}),
      desktopAPI.onCoordinationState(() => {}),
      desktopAPI.onSessionState(() => {}),
      desktopAPI.onPublicAccessStatus(() => {}),
      desktopAPI.onApplicationUpdateStatus(() => {}),
    ];
    releases.forEach(release => release());

    const events = [...new Set(__desktopFixture.calls
      .map(call => call.method)
      .filter(method => method.startsWith('event:on:'))
      .map(method => method.slice('event:on:'.length)))]
      .sort();
    return { servicePaths, methods, events };
  });

  expect(contract.servicePaths).toEqual([
    '/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js',
  ]);
  expect(contract.methods).toEqual([
    'AddCharacter',
    'AssignCharacter',
    'CopyDemo',
    'CopyPublicAccessCredentials',
    'DeleteCharacter',
    'EndBroadcast',
    'ForceHackSuccess',
    'GeneratePlayerPassword',
    'GetApplicationUpdateStatus',
    'GetPublicAccess',
    'GetRuntimeStatus',
    'LoadReferencedPlayerConfig',
    'MoveCharacter',
    'NewPlayerConfig',
    'NewSession',
    'OpenPlayerConfig',
    'OpenSession',
    'OpenURL',
    'ReleaseCharacter',
    'RenameLogicalSession',
    'ReplaceTerminalGroups',
    'RequestTerminalActivation',
    'RequestTerminalClear',
    'ResetCommandState',
    'ResetFailedHack',
    'ResetTerminalCommandStates',
    'ResolveApplicationUpdateOffer',
    'ResolveApplicationUpdateRestart',
    'ResolveCommandExecution',
    'ResolveTerminalNavigation',
    'ResolveTerminalSwitch',
    'SavePublicAccessSettings',
    'SaveSession',
    'SetActiveController',
    'StartBroadcast',
    'StartPublicAccess',
    'StopPublicAccess',
    'UpdateCharacter',
    'UpdateLiveTerminal',
  ]);
  expect(contract.events).toEqual([
    'application-update-status',
    'client-count',
    'coordination-state',
    'hack-state',
    'public-access-status',
    'server-info',
    'session-state',
  ]);
});

test('generated desktop service calls remain explicit and normalized behind the facade', async ({ page }) => {
  const results = await page.evaluate(async () => ({
    open: await desktopAPI.openSession(),
    save: await desktopAPI.saveSession({ version: 1 }),
    url: await desktopAPI.openUrl('https://fallout.example'),
    terminal: await desktopAPI.requestTerminalClear(),
    playerConfig: await desktopAPI.openPlayerConfig(),
  }));
  expect(results.open.ok).toBe(true);
  expect(results.save.ok).toBe(true);
  expect(results.url.ok).toBe(true);
  expect(results.terminal).toEqual(expect.objectContaining({ ok: true, status: '', switchId: '' }));
  expect(results.playerConfig).toEqual(expect.objectContaining({ ok: true, canceled: false }));

  const calls = await page.evaluate(() => __desktopFixture.calls.map(call => call.method));
  expect(calls).toEqual(expect.arrayContaining([
    'OpenSession', 'SaveSession', 'OpenURL', 'RequestTerminalClear', 'OpenPlayerConfig',
  ]));
  expect(calls).not.toContain('Dispatch');
  expect(calls).not.toContain('Call');
});

test('terminal-group replacement forwards both revisions and returns detached canonical authorities', async ({ page }) => {
  const request = {
    terminalGroups: [
      { id: 'group-alpha', name: 'Alpha', terminalIds: ['terminal-a', 'terminal-b'] },
      { id: 'group-charlie', name: 'Charlie', terminalIds: ['terminal-c'] },
    ],
    expectedSessionRevision: 17,
    expectedCoordinationRevision: 29,
  };

  const observed = await page.evaluate(async candidate => {
    const result = await desktopAPI.replaceTerminalGroups(candidate);
    candidate.terminalGroups[0].terminalIds.reverse();
    result.session.terminalGroups[0].terminalIds.reverse();
    return {
      result,
      call: __desktopFixture.calls
        .filter(entry => entry.method === 'ReplaceTerminalGroups').at(-1),
    };
  }, request);

  expect(observed.call).toEqual({ method: 'ReplaceTerminalGroups', args: [request] });
  expect(observed.result).toEqual(expect.objectContaining({
    ok: true,
    error: '',
    sessionRevision: 18,
    coordinationState: expect.objectContaining({ revision: 30 }),
  }));
  expect(observed.result.session.terminalGroups).toEqual([
    { id: 'group-alpha', name: 'Alpha', terminalIds: ['terminal-b', 'terminal-a'] },
    { id: 'group-charlie', name: 'Charlie', terminalIds: ['terminal-c'] },
  ]);
  expect(observed.call.args[0].terminalGroups[0].terminalIds).toEqual(['terminal-a', 'terminal-b']);
});

test('multi-link legacy partial and complete candidates remain exact at the desktop facade', async ({ page }) => {
  const partial = {
    terminalGroups: [
      {
        id: 'singleton-service', name: 'Service',
        terminalIds: ['t-krel-service', 't-krel-admin'],
      },
      {
        id: 'singleton-emergency', name: 'Emergency',
        terminalIds: ['t-krel-emergency'],
      },
    ],
    expectedSessionRevision: 17,
    expectedCoordinationRevision: 29,
  };
  const complete = {
    terminalGroups: [{
      id: 'singleton-service', name: 'Service',
      terminalIds: ['t-krel-service', 't-krel-admin', 't-krel-emergency'],
    }],
    expectedSessionRevision: 17,
    expectedCoordinationRevision: 29,
  };

  const calls = await page.evaluate(async candidates => {
    await desktopAPI.replaceTerminalGroups(candidates.partial);
    await desktopAPI.replaceTerminalGroups(candidates.complete);
    return __desktopFixture.calls.filter(entry => entry.method === 'ReplaceTerminalGroups');
  }, { partial, complete });

  expect(calls.slice(-2)).toEqual([
    { method: 'ReplaceTerminalGroups', args: [partial] },
    { method: 'ReplaceTerminalGroups', args: [complete] },
  ]);
});

test('addCharacter forwards one complete profile with explicit false and expected revision', async ({ page }) => {
  const request = {
    name: 'Mara',
    intelligence: 8,
    hackerPerkAvailable: false,
    expectedRevision: 42,
  };

  await page.evaluate(candidate => desktopAPI.addCharacter(candidate), request);

  const call = await page.evaluate(() => __desktopFixture.calls
    .filter(candidate => candidate.method === 'AddCharacter').at(-1));
  expect(call).toEqual({ method: 'AddCharacter', args: [request] });
});

test('updateCharacter and deleteCharacter forward complete revisioned payloads without a rename facade', async ({ page }) => {
  const update = {
    characterId: 'character-mara',
    name: 'Mara',
    intelligence: 8,
    hackerPerkAvailable: false,
    expectedRevision: 42,
  };
  const deletion = {
    characterId: 'character-boone',
    expectedRevision: 43,
  };

  const facade = await page.evaluate(async ({ updateRequest, deleteRequest }) => {
    const bindings = await import('/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js');
    await desktopAPI.updateCharacter(updateRequest);
    await desktopAPI.deleteCharacter(deleteRequest);
    return {
      updateType: typeof desktopAPI.updateCharacter,
      renameType: typeof desktopAPI.renameCharacter,
      deleteType: typeof desktopAPI.deleteCharacter,
      bindingUpdateType: typeof bindings.UpdateCharacter,
      bindingRenameType: typeof bindings.RenameCharacter,
      bindingDeleteType: typeof bindings.DeleteCharacter,
      calls: __desktopFixture.calls.filter(call => [
        'UpdateCharacter', 'RenameCharacter', 'DeleteCharacter',
      ].includes(call.method)),
    };
  }, { updateRequest: update, deleteRequest: deletion });

  expect(facade.updateType).toBe('function');
  expect(facade.renameType).toBe('undefined');
  expect(facade.deleteType).toBe('function');
  expect(facade.bindingUpdateType).toBe('function');
  expect(facade.bindingRenameType).toBe('undefined');
  expect(facade.bindingDeleteType).toBe('function');
  expect(facade.calls).toEqual([
    { method: 'UpdateCharacter', args: [update] },
    { method: 'DeleteCharacter', args: [deletion] },
  ]);
});

test('saveSession snapshots the complete cross-terminal document at the Wails boundary', async ({ page }) => {
  const retained = await page.evaluate(async () => {
    const candidate = {
      version: 1,
      name: 'demo boundary candidate',
      terminals: [
        {
          id: 't_demo1', name: 'Source', hackLevel: 0, introText: '',
          root: {
            id: 'root', type: 'folder', name: 'ROOT',
            children: [{
              id: 'go', type: 'command', name: 'GO',
              terminalTransition: { targetTerminalId: 't_demo2' },
            }],
          },
        },
        {
          id: 't_demo2', name: 'Target', hackLevel: 0, introText: '',
          root: { id: 'root', type: 'folder', name: 'ROOT', children: [] },
        },
      ],
    };
    const pending = desktopAPI.saveSession(candidate);
    candidate.terminals.splice(1, 1);
    await pending;
    return __desktopFixture.calls.filter(call => call.method === 'SaveSession').at(-1).args[0];
  });

  expect(retained.terminals.map(terminal => terminal.id)).toEqual(['t_demo1', 't_demo2']);
  expect(retained.terminals[0].root.children[0].terminalTransition.targetTerminalId).toBe('t_demo2');
});

test('all four listeners precede the snapshot and newer wrapped events win per field', async ({ page }) => {
  const result = await page.evaluate(async () => {
    __desktopFixture.deferStatus();
    const observed = { server: [], clients: [], hack: [], coordination: [] };
    const releases = [
      desktopAPI.onServerInfo(value => observed.server.push(value)),
      desktopAPI.onClientCount(value => observed.clients.push(value)),
      desktopAPI.onHackState(value => observed.hack.push(value)),
      desktopAPI.onCoordinationState(value => observed.coordination.push(value)),
    ];
    __desktopFixture.emit('client-count', 7);
    __desktopFixture.emit('server-info', { url: 'https://public.example', tunnel: true });
    __desktopFixture.resolveStatus({
      serverInfo: { url: 'http://127.0.0.1:3690', tunnel: false },
      clientCount: 1,
      hackState: { attemptsLeft: 2 },
      coordinationState: { revision: 4 },
    });
    await Promise.resolve();
    await Promise.resolve();
    return { observed, timeline: __desktopFixture.timeline.map(entry => entry.method) };
  });

  expect(result.timeline.slice(0, 5)).toEqual([
    'event:on:server-info', 'event:on:client-count', 'event:on:hack-state',
    'event:on:coordination-state', 'GetRuntimeStatus',
  ]);
  expect(result.observed.clients).toEqual([7]);
  expect(result.observed.server).toEqual([expect.objectContaining({
    url: 'https://public.example', localUrl: '', tunnel: true,
  })]);
  expect(result.observed.hack).toEqual([{ attemptsLeft: 2 }]);
  expect(result.observed.coordination).toEqual([{ revision: 4 }]);
});

test('release is exact-once, suppresses pending snapshot callbacks, and hot disposal releases old listeners', async ({ page }) => {
  const result = await page.evaluate(async () => {
    __desktopFixture.deferStatus();
    let callbacks = 0;
    const releases = [
      desktopAPI.onServerInfo(() => { callbacks += 1; }),
      desktopAPI.onClientCount(() => { callbacks += 1; }),
      desktopAPI.onHackState(() => { callbacks += 1; }),
      desktopAPI.onCoordinationState(() => { callbacks += 1; }),
    ];
    releases[0]();
    releases[0]();
    __desktopFixture.emit('server-info', { url: 'http://late.example', tunnel: false });
    __desktopFixture.resolveStatus();
    await Promise.resolve();
    await import('/__fixture/desktop-api.js?hot=2');
    await Promise.resolve();
    return {
      callbacks,
      releases: ['server-info', 'client-count', 'hack-state', 'coordination-state']
        .map(name => __desktopFixture.releaseCount(name)),
    };
  });

  expect(result.callbacks).toBe(3);
  expect(result.releases).toEqual([1, 1, 1, 1]);
});

test('public-access facade exposes exactly six methods with native secret-free sharing results', async ({ page }) => {
  const result = await page.evaluate(async () => {
    const request = {
      expectedRevision: 0,
      enabledPreference: true,
      reservedDomain: '',
      username: 'players',
      replacementProviderToken: 'synthetic-provider-input',
      replacementPlayerPassword: 'synthetic-player-input',
    };
    const saved = await desktopAPI.savePublicAccessSettings(request);
    const generated = await desktopAPI.generatePlayerPassword({ expectedRevision: saved.snapshot.preferences.revision });
    const afterGenerated = await desktopAPI.getPublicAccess();
    const shared = await desktopAPI.copyPublicAccessCredentials();
    const clipboardText = __desktopFixture.takeClipboardText();
    const started = await desktopAPI.startPublicAccess({ expectedRevision: afterGenerated.preferences.revision });
    const stopped = await desktopAPI.stopPublicAccess({ expectedRevision: started.snapshot.preferences.revision });
    return {
      request,
      saved,
      generated,
      afterGenerated,
      shared,
      clipboardText,
      started,
      stopped,
      methods: __desktopFixture.calls.map(call => call.method).filter(method => method.includes('PublicAccess') || method.includes('PlayerPassword')),
      retainedCalls: __desktopFixture.calls,
    };
  });

  expect(result.methods).toEqual([
    'SavePublicAccessSettings', 'GeneratePlayerPassword', 'GetPublicAccess',
    'CopyPublicAccessCredentials', 'StartPublicAccess', 'StopPublicAccess',
  ]);
  expect(result.request.replacementProviderToken).toBe('');
  expect(result.request.replacementPlayerPassword).toBe('');
  expect(result.saved.snapshot.providerTokenPresence).toBe('present');
  expect(result.saved.snapshot.playerPasswordPresence).toBe('present');
  expect(result.generated.generatedPassword).toBe('synthetic-one-time-generated-value');
  expect(result.afterGenerated.generatedPassword).toBeUndefined();
  expect(result.shared).toEqual(expect.objectContaining({ ok: true }));
  expect(result.clipboardText).toBe('Логин: players\nПароль: synthetic-one-time-generated-value');
  expect(JSON.stringify(result.shared)).not.toContain('synthetic-one-time-generated-value');
  expect(JSON.stringify([result.saved, result.afterGenerated, result.started, result.stopped, result.retainedCalls]))
    .not.toContain('synthetic-one-time-generated-value');
  expect(JSON.stringify(result.retainedCalls)).not.toContain('synthetic-provider-input');
  expect(JSON.stringify(result.retainedCalls)).not.toContain('synthetic-player-input');
});

test('public-access event beats an equal or older snapshot and disposal releases exactly once', async ({ page }) => {
  const result = await page.evaluate(async () => {
    __desktopFixture.deferPublicAccess();
    const observed = [];
    const release = desktopAPI.onPublicAccessStatus(value => observed.push(value));
    __desktopFixture.emit('public-access-status', {
      preferences: { version: 1, username: 'players', revision: 3 },
      providerTokenPresence: 'unknown',
      playerPasswordPresence: 'present',
      status: { state: 'stopped', generation: 4, settingsRevision: 3 },
    });
    __desktopFixture.resolvePublicAccess({
      preferences: { version: 1, username: 'players', revision: 2 },
      providerTokenPresence: 'absent',
      playerPasswordPresence: 'absent',
      status: { state: 'stopped', generation: 3, settingsRevision: 2 },
    });
    await Promise.resolve();
    await Promise.resolve();
    release();
    release();
    return {
      observed,
      releaseCount: __desktopFixture.releaseCount('public-access-status'),
      timeline: __desktopFixture.timeline.map(entry => entry.method),
    };
  });

  expect(result.timeline).toEqual(expect.arrayContaining([
    'event:on:public-access-status', 'GetPublicAccess',
  ]));
  expect(result.observed).toHaveLength(1);
  expect(result.observed[0]).toEqual(expect.objectContaining({
    providerTokenPresence: 'unknown',
    status: expect.objectContaining({ generation: 4, settingsRevision: 3 }),
  }));
  expect(result.releaseCount).toBe(1);
});

test('application update listener precedes its getter and the greatest revision wins', async ({ page }) => {
  const result = await page.evaluate(async () => {
    __desktopFixture.deferApplicationUpdate();
    const observed = [];
    const release = desktopAPI.onApplicationUpdateStatus(snapshot => observed.push(snapshot));

    __desktopFixture.emit('application-update-status', {
      revision: 5,
      attemptId: 'attempt-newer-event',
      state: 'available',
      installedVersion: '2.0.0',
      availableVersion: '2.1.0',
      releaseNotes: 'Newer event notes',
      bytesDownloaded: 0,
      downloadSize: 4096,
      failedStage: '',
      errorMessage: '',
      recoveryAction: '',
    });
    __desktopFixture.resolveApplicationUpdate({
      revision: 4,
      attemptId: 'attempt-older-getter',
      state: 'checking',
      installedVersion: '2.0.0',
      bytesDownloaded: 0,
      failedStage: '',
    });

    await Promise.resolve();
    await Promise.resolve();
    release();
    release();

    return {
      observed,
      frozen: observed.map(snapshot => Object.isFrozen(snapshot)),
      releaseCount: __desktopFixture.releaseCount('application-update-status'),
      timeline: __desktopFixture.timeline
        .map(entry => entry.method)
        .filter(method => method === 'event:on:application-update-status'
          || method === 'GetApplicationUpdateStatus'),
    };
  });

  expect(result.timeline).toEqual([
    'event:on:application-update-status',
    'GetApplicationUpdateStatus',
  ]);
  expect(result.observed).toEqual([expect.objectContaining({
    revision: 5,
    attemptId: 'attempt-newer-event',
    state: 'available',
    installedVersion: '2.0.0',
    availableVersion: '2.1.0',
    releaseNotes: 'Newer event notes',
    bytesDownloaded: 0,
    downloadSize: 4096,
  })]);
  expect(result.frozen).toEqual([true]);
  expect(result.releaseCount).toBe(1);
});
