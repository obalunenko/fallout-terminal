import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.goto('/__fixture/desktop-api');
  await expect.poll(() => page.evaluate(() => typeof window.desktopAPI)).toBe('object');
});

test('desktop facade retains one v2 service with 46 methods and seven named events', async ({ page }) => {
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
    'InspectFacilityDependencies',
    'LoadReferencedPlayerConfig',
    'MoveCharacter',
    'NewPlayerConfig',
    'NewSession',
	'OpenLogLocation',
    'OpenPlayerConfig',
    'OpenSession',
    'OpenURL',
    'PreviewFacility',
    'RecoverFacilityCondition',
    'ReleaseCharacter',
    'RenameLogicalSession',
    'ReplaceTerminalGroups',
    'RequestTerminalActivation',
    'RequestTerminalClear',
    'ResetCommandState',
    'ResetFacility',
    'ResetFacilityDevice',
    'ResetFailedHack',
    'ResetTerminalCommandStates',
    'ResolveApplicationUpdateOffer',
    'ResolveApplicationUpdateRestart',
    'ResolveCommandExecution',
    'ResolveTerminalNavigation',
    'ResolveTerminalSwitch',
    'SaveFacilityAuthoring',
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
	logs: await desktopAPI.openLogLocation(),
    terminal: await desktopAPI.requestTerminalClear(),
    playerConfig: await desktopAPI.openPlayerConfig(),
  }));
  expect(results.open.ok).toBe(true);
  expect(results.save.ok).toBe(true);
  expect(results.url.ok).toBe(true);
	expect(results.logs).toEqual(expect.objectContaining({ ok: true, directoryPath: expect.stringContaining('/logs') }));
  expect(results.terminal).toEqual(expect.objectContaining({ ok: true, status: '', switchId: '' }));
  expect(results.playerConfig).toEqual(expect.objectContaining({ ok: true, canceled: false }));

  const calls = await page.evaluate(() => __desktopFixture.calls.map(call => call.method));
  expect(calls).toEqual(expect.arrayContaining([
	'OpenSession', 'SaveSession', 'OpenURL', 'OpenLogLocation', 'RequestTerminalClear', 'OpenPlayerConfig',
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

test('facility authoring and dependency inspection preserve typed revisions and detached data', async ({ page }) => {
  const request = {
    session: {
      version: 1,
      name: 'Facility draft',
      terminals: [],
      facility: { revision: 7, devices: [], conditions: [], recoveryPrograms: [] },
    },
    expectedSessionRevision: 17,
    expectedFacilityRevision: 7,
    correlationId: 'authoring-17',
  };

  const observed = await page.evaluate(async candidate => {
    const pending = desktopAPI.saveFacilityAuthoring(candidate);
    candidate.session.name = 'mutated after invocation';
    candidate.expectedSessionRevision = -1;
    const saved = await pending;
    saved.session.name = 'mutated result';

    const inspected = await desktopAPI.inspectFacilityDependencies({
      target: { kind: 'device', entityId: 'security-door' },
      expectedSessionRevision: 18,
      expectedFacilityRevision: 8,
    });
    __desktopFixture.setNextTerminalActionResult('InspectFacilityDependencies', {
      ok: false,
      failure: 'missing-reference',
      issues: [{
        code: 'missing-reference',
        entityKind: 'device-state',
        entityId: 'sealed',
        referenceKind: 'owner',
        referenceId: 'security-door',
      }],
      sessionRevision: 18,
      facilityRevision: 8,
      report: null,
    });
    const failed = await desktopAPI.inspectFacilityDependencies({
      target: { kind: 'device-state', entityId: 'sealed', ownerId: 'security-door' },
      expectedSessionRevision: 18,
      expectedFacilityRevision: 8,
    });
    return {
      saved,
      inspected,
      failed,
      savedFrozen: Object.isFrozen(saved),
      dependencyFrozen: Object.isFrozen(inspected.report.dependencies[0]),
      calls: __desktopFixture.calls.filter(call => [
        'SaveFacilityAuthoring', 'InspectFacilityDependencies',
      ].includes(call.method)).slice(-3),
    };
  }, request);

  expect(observed.saved).toEqual(expect.objectContaining({
    ok: true,
    changed: true,
    correlationId: 'authoring-17',
    failure: '',
    sessionRevision: 18,
    previousFacilityRevision: 7,
    resultingFacilityRevision: 8,
    affectedDeviceIds: ['security-door'],
  }));
  expect(observed.savedFrozen).toBe(true);
  expect(observed.calls[0].args[0]).toEqual(request);
  expect(observed.calls[0].args[0].session.name).toBe('Facility draft');
  expect(observed.inspected).toEqual(expect.objectContaining({
    ok: true,
    failure: '',
    sessionRevision: 18,
    facilityRevision: 8,
    report: {
      target: { kind: 'device', entityId: 'security-door' },
      dependencies: [{
        kind: 'command-action',
        sourceId: 'open-door',
        targetId: 'security-door',
        property: 'stateChange.facilityAction.transitions[0].deviceId',
        terminalId: 'terminal-security',
      }],
    },
  }));
  expect(observed.dependencyFrozen).toBe(true);
  expect(observed.failed).toEqual(expect.objectContaining({
    ok: false,
    failure: 'missing-reference',
    issues: [{
      code: 'missing-reference',
      entityKind: 'device-state',
      entityId: 'sealed',
      referenceKind: 'owner',
      referenceId: 'security-door',
    }],
    report: null,
  }));
  expect(observed.calls[1].args[0].target).toEqual({ kind: 'device', entityId: 'security-door' });
  expect(Object.hasOwn(observed.calls[1].args[0].target, 'ownerId')).toBe(false);
  expect(observed.calls[2].args[0].target.ownerId).toBe('security-door');
});

test('facility authoring rejects inconsistent or out-of-order successful responses', async ({ page }) => {
  const observed = await page.evaluate(async () => {
    const request = {
      session: {
        version: 1,
        name: 'Facility draft',
        terminals: [],
        facility: { revision: 7, devices: [], conditions: [], recoveryPrograms: [] },
      },
      expectedSessionRevision: 17,
      expectedFacilityRevision: 7,
      correlationId: 'authoring-17',
    };
    const success = {
      ok: true,
      changed: true,
      correlationId: request.correlationId,
      failure: '',
      issues: [],
      sessionRevision: 18,
      previousFacilityRevision: 7,
      resultingFacilityRevision: 8,
      session: {
        ...request.session,
        facility: { ...request.session.facility, revision: 8 },
      },
    };
    const candidates = [
      { ...success, correlationId: 'another-operation' },
      { ...success, previousFacilityRevision: 6 },
      { ...success, resultingFacilityRevision: 10 },
      { ...success, sessionRevision: 16 },
      { ...success, sessionRevision: 19 },
      { ...success, session: null },
      { ...success, session: request.session },
      { ...success, session: { ...success.session, facility: { ...success.session.facility, revision: 9 } } },
      {
        ...success,
        changed: false,
        sessionRevision: 18,
        resultingFacilityRevision: 7,
        session: request.session,
      },
      {
        ...success,
        changed: false,
        sessionRevision: 17,
        resultingFacilityRevision: 8,
      },
    ];
    const results = [];
    for (const candidate of candidates) {
      __desktopFixture.setNextTerminalActionResult('SaveFacilityAuthoring', candidate);
      results.push(await desktopAPI.saveFacilityAuthoring(request));
    }
    __desktopFixture.setNextTerminalActionResult('SaveFacilityAuthoring', {
      ...success,
      changed: false,
      sessionRevision: 17,
      resultingFacilityRevision: 7,
      session: null,
    });
    return { failures: results, unchanged: await desktopAPI.saveFacilityAuthoring(request) };
  });

  expect(observed.failures).toHaveLength(10);
  for (const result of observed.failures) {
    expect(result).toEqual(expect.objectContaining({
      ok: false,
      changed: false,
      correlationId: 'authoring-17',
      failure: 'conflict',
      sessionRevision: 17,
      previousFacilityRevision: 7,
      resultingFacilityRevision: 7,
      session: null,
    }));
    expect(result.issues).toEqual([{ code: 'conflict', entityKind: 'desktop-result' }]);
  }
  expect(observed.unchanged).toEqual(expect.objectContaining({
    ok: true,
    changed: false,
    sessionRevision: 17,
    previousFacilityRevision: 7,
    resultingFacilityRevision: 7,
    session: null,
  }));
});

test('facility inspection, preview, resets, and recovery use normalized detached contracts', async ({ page }) => {
  const observed = await page.evaluate(async () => {
    const previewRequest = {
      expectedFacilityRevision: 8,
      terminalId: 'terminal-security',
      deviceState: { deviceId: 'security-door', stateId: 'open' },
    };
    const recoveryRequest = {
      conditionId: 'grid-offline',
      expectedFacilityRevision: 8,
      correlationId: 'recover-8',
      recovery: { transition: { deviceId: 'power-grid', transitionId: 'restore' } },
    };
    const previewPending = desktopAPI.previewFacility(previewRequest);
    const recoveryPending = desktopAPI.recoverFacilityCondition(recoveryRequest);
    previewRequest.deviceState.stateId = 'offline';
    recoveryRequest.recovery.transition.transitionId = 'mutated';
    const [preview, recovery, deviceReset, facilityReset] = await Promise.all([
      previewPending,
      recoveryPending,
      desktopAPI.resetFacilityDevice({
        deviceId: 'security-door', expectedFacilityRevision: 8, correlationId: 'device-reset-8',
      }),
      desktopAPI.resetFacility({ expectedFacilityRevision: 8, correlationId: 'facility-reset-8' }),
    ]);
    preview.terminal.terminalName = 'mutated result';
    recovery.session.name = 'mutated result';
    return {
      preview,
      recovery,
      deviceReset,
      facilityReset,
      calls: __desktopFixture.calls.filter(call => [
        'PreviewFacility', 'RecoverFacilityCondition', 'ResetFacilityDevice', 'ResetFacility',
      ].includes(call.method)).slice(-4),
    };
  });

  expect(observed.calls).toEqual([
    {
      method: 'PreviewFacility',
      args: [{
        expectedFacilityRevision: 8,
        terminalId: 'terminal-security',
        deviceState: { DeviceID: 'security-door', StateID: 'open' },
      }],
    },
    {
      method: 'RecoverFacilityCondition',
      args: [{
        conditionId: 'grid-offline',
        expectedFacilityRevision: 8,
        correlationId: 'recover-8',
        recovery: { transition: { deviceId: 'power-grid', transitionId: 'restore' } },
      }],
    },
    {
      method: 'ResetFacilityDevice',
      args: [{ deviceId: 'security-door', expectedFacilityRevision: 8, correlationId: 'device-reset-8' }],
    },
    {
      method: 'ResetFacility',
      args: [{ expectedFacilityRevision: 8, correlationId: 'facility-reset-8' }],
    },
  ]);
  expect(observed.preview).toEqual(expect.objectContaining({
    ok: true,
    failure: '',
    facilityRevision: 8,
    terminal: expect.objectContaining({ terminalId: 'terminal-security', terminalName: 'mutated result' }),
  }));
  expect(observed.recovery).toEqual(expect.objectContaining({
    ok: true,
    changed: true,
    correlationId: 'recover-8',
    previousFacilityRevision: 8,
    resultingFacilityRevision: 9,
    affectedConditionIds: ['grid-offline'],
  }));
  expect(observed.deviceReset).toEqual(expect.objectContaining({
    ok: true,
    changed: true,
    correlationId: 'device-reset-8',
    affectedDeviceIds: ['security-door'],
  }));
  expect(observed.facilityReset).toEqual(expect.objectContaining({
    ok: true,
    changed: true,
    correlationId: 'facility-reset-8',
    resultingFacilityRevision: 9,
  }));
});

test('facility read and mutation calls reject inconsistent successful revisions', async ({ page }) => {
  const observed = await page.evaluate(async () => {
    __desktopFixture.setNextTerminalActionResult('InspectFacilityDependencies', {
      ok: true,
      facilityRevision: 9,
      sessionRevision: 18,
      report: {
        target: { kind: 'device', entityId: 'security-door' },
        dependencies: [],
      },
    });
    const inspection = await desktopAPI.inspectFacilityDependencies({
      target: { kind: 'device', entityId: 'security-door' },
      expectedSessionRevision: 18,
      expectedFacilityRevision: 8,
    });

    __desktopFixture.setNextTerminalActionResult('PreviewFacility', {
      ok: true,
      facilityRevision: 9,
      terminal: { terminalId: 'terminal-security' },
    });
    const preview = await desktopAPI.previewFacility({
      expectedFacilityRevision: 8,
      terminalId: 'terminal-security',
      condition: { conditionId: 'grid-offline', active: false },
    });

    __desktopFixture.setNextTerminalActionResult('ResetFacilityDevice', {
      ok: true,
      changed: true,
      correlationId: 'device-reset-8',
      sessionRevision: 19,
      previousFacilityRevision: 8,
      resultingFacilityRevision: 10,
      affectedDeviceIds: ['security-door'],
      session: { version: 1, terminals: [], facility: { revision: 10 } },
    });
    const reset = await desktopAPI.resetFacilityDevice({
      deviceId: 'security-door', expectedFacilityRevision: 8, correlationId: 'device-reset-8',
    });

    __desktopFixture.setNextTerminalActionResult('RecoverFacilityCondition', {
      ok: false,
      changed: false,
      correlationId: 'recover-8',
      failure: 'stale-revision',
      issues: [{ code: 'stale-revision', entityKind: 'facility' }],
      previousFacilityRevision: 9,
      resultingFacilityRevision: 9,
    });
    const staleRecovery = await desktopAPI.recoverFacilityCondition({
      conditionId: 'grid-offline',
      expectedFacilityRevision: 8,
      correlationId: 'recover-8',
      recovery: { privateOverseerAction: true },
    });
    return { inspection, preview, reset, staleRecovery };
  });

  for (const result of [observed.inspection, observed.preview, observed.reset]) {
    expect(result).toEqual(expect.objectContaining({
      ok: false,
      failure: 'conflict',
      issues: [{ code: 'conflict', entityKind: 'desktop-result' }],
    }));
  }
  expect(observed.inspection).toEqual(expect.objectContaining({ sessionRevision: 18, facilityRevision: 8 }));
  expect(observed.preview).toEqual(expect.objectContaining({ facilityRevision: 8, terminal: null }));
  expect(observed.reset).toEqual(expect.objectContaining({
    changed: false,
    correlationId: 'device-reset-8',
    previousFacilityRevision: 8,
    resultingFacilityRevision: 8,
    session: null,
  }));
  expect(observed.staleRecovery).toEqual(expect.objectContaining({
    ok: false,
    changed: false,
    correlationId: 'recover-8',
    failure: 'stale-revision',
    previousFacilityRevision: 9,
    resultingFacilityRevision: 9,
  }));
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
