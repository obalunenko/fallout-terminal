import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.goto('/__fixture/desktop-api');
  await expect.poll(() => page.evaluate(() => typeof window.desktopAPI)).toBe('object');
});

test('desktop facade retains one v2 service with 47 methods and seven named events', async ({ page }) => {
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
    'ApplyFacilityDeviceTransition',
    'AssignCharacter',
    'CopyDemo',
    'CopyPublicAccessCredentials',
    'DeleteCharacter',
    'DescribePresentation',
    'EndBroadcast',
    'ForceFacilityDeviceState',
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
    'ResetFacilityDevice',
    'ResetFailedHack',
    'ResetScene',
    'ResolveApplicationUpdateOffer',
    'ResolveApplicationUpdateRestart',
    'ResolveCommandExecution',
    'ResolveTerminalNavigation',
    'ResolveTerminalSwitch',
    'SaveAuthoring',
    'SavePublicAccessSettings',
    'SetActiveController',
    'SetFacilityConditionActive',
    'StartBroadcast',
    'StartPublicAccess',
    'StopPublicAccess',
    'UpdateCharacter',
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
    save: await desktopAPI.saveAuthoring({
      session: { version: 1, name: 'Facade save', terminals: [] }, expectedAuthoringRevision: 1, correlationId: 'facade-save',
    }),
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
	'OpenSession', 'SaveAuthoring', 'OpenURL', 'OpenLogLocation', 'RequestTerminalClear', 'OpenPlayerConfig',
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

test('authoring saves and dependency inspection preserve typed revisions and detached data', async ({ page }) => {
  const request = {
    session: {
      version: 1,
      name: 'Facility draft',
      terminals: [],
      facility: { revision: 7, devices: [], conditions: [], recoveryPrograms: [] },
    },
    expectedAuthoringRevision: 4,
    correlationId: 'authoring-4',
  };

  const observed = await page.evaluate(async candidate => {
    __desktopFixture.setNextTerminalActionResult('SaveAuthoring', {
      ok: true, changed: true, correlationId: 'authoring-4', failure: '', issues: [],
      sessionRevision: 18, previousFacilityRevision: 7, resultingFacilityRevision: 8, authoringRevision: 5,
      affectedDeviceIds: ['security-door'],
      session: { ...candidate.session, facility: { ...candidate.session.facility, revision: 8 } },
    });
    const pending = desktopAPI.saveAuthoring(candidate);
    candidate.session.name = 'mutated after invocation';
    candidate.expectedAuthoringRevision = -1;
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
        'SaveAuthoring', 'InspectFacilityDependencies',
      ].includes(call.method)).slice(-3),
    };
  }, request);

  expect(observed.saved).toEqual(expect.objectContaining({
    ok: true,
    changed: true,
    correlationId: 'authoring-4',
    failure: '',
    sessionRevision: 18,
    previousFacilityRevision: 7,
    resultingFacilityRevision: 8,
    authoringRevision: 5,
    affectedDeviceIds: ['security-door'],
    resetCommands: [],
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

test('authoring saves reject inconsistent successful responses', async ({ page }) => {
  const observed = await page.evaluate(async () => {
    const request = {
      session: { version: 1, name: 'Facility draft', terminals: [] },
      expectedAuthoringRevision: 4,
      correlationId: 'authoring-4',
    };
    const success = {
      ok: true, changed: true, correlationId: request.correlationId, failure: '', issues: [],
      sessionRevision: 18, authoringRevision: 5, session: request.session,
    };
    const candidates = [
      { ...success, correlationId: 'another-operation' },
      { ...success, session: null },
      { ...success, authoringRevision: 4 },
      { ...success, authoringRevision: 6 },
      { ...success, sessionRevision: 0 },
      { ...success, changed: false, authoringRevision: 5 },
      { ...success, ok: false, correlationId: 'another-operation', failure: 'stale-revision' },
    ];
    const results = [];
    for (const candidate of candidates) {
      __desktopFixture.setNextTerminalActionResult('SaveAuthoring', candidate);
      results.push(await desktopAPI.saveAuthoring(request));
    }
    __desktopFixture.setNextTerminalActionResult('SaveAuthoring', { ...success, changed: false, authoringRevision: 4 });
    const unchanged = await desktopAPI.saveAuthoring(request);
    __desktopFixture.setNextTerminalActionResult('SaveAuthoring', {
      ok: false, changed: false, correlationId: request.correlationId, failure: 'stale-revision',
      issues: [], authoringRevision: 6, session: request.session,
    });
    const stale = await desktopAPI.saveAuthoring(request);
    return { failures: results, unchanged, stale };
  });

  expect(observed.failures).toHaveLength(7);
  for (const result of observed.failures) {
    expect(result).toEqual(expect.objectContaining({
      ok: false,
      changed: false,
      correlationId: 'authoring-4',
      failure: 'conflict',
      authoringRevision: 4,
      session: null,
    }));
    expect(result.issues).toEqual([{ code: 'conflict', entityKind: 'desktop-result' }]);
  }
  expect(observed.unchanged).toEqual(expect.objectContaining({ ok: true, changed: false, authoringRevision: 4 }));
  expect(observed.stale).toEqual(expect.objectContaining({
    ok: false, failure: 'stale-revision', authoringRevision: 6,
  }), 'a stale save keeps the canonical session and current authoring revision');
  expect(observed.stale.session).not.toBeNull();
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
    __desktopFixture.setNextTerminalActionResult('ResetScene', {
      ok: true, changed: true, correlationId: 'scene-reset-8', sessionRevision: 19,
      previousFacilityRevision: 8, resultingFacilityRevision: 9, authoringRevision: 3,
      affectedDeviceIds: ['security-door'],
      resetCommands: [{ terminalId: 'terminal-security', commandId: 'open-door' }],
      session: { version: 1, terminals: [], facility: { revision: 9 } },
    });
    const [preview, recovery, deviceReset, facilityReset] = await Promise.all([
      previewPending,
      recoveryPending,
      desktopAPI.resetFacilityDevice({
        deviceId: 'security-door', expectedFacilityRevision: 8, correlationId: 'device-reset-8',
      }),
      desktopAPI.resetScene({ expectedSessionRevision: 18, expectedFacilityRevision: 8, correlationId: 'scene-reset-8' }),
    ]);
    preview.terminal.terminalName = 'mutated result';
    recovery.session.name = 'mutated result';
    return {
      preview,
      recovery,
      deviceReset,
      facilityReset,
      calls: __desktopFixture.calls.filter(call => [
        'PreviewFacility', 'RecoverFacilityCondition', 'ResetFacilityDevice', 'ResetScene',
      ].includes(call.method)).slice(-4),
    };
  });

  expect(observed.calls).toEqual([
    {
      method: 'PreviewFacility',
      args: [{
        expectedFacilityRevision: 8,
        expectedAuthoringRevision: 0,
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
      method: 'ResetScene',
      args: [{ expectedSessionRevision: 18, expectedFacilityRevision: 8, correlationId: 'scene-reset-8' }],
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
    correlationId: 'scene-reset-8',
    resultingFacilityRevision: 9,
    authoringRevision: 3,
    resetCommands: [{ terminalId: 'terminal-security', commandId: 'open-door' }],
  }));
  expect(observed.preview).toEqual(expect.objectContaining({ current: null, differences: [] }));
});

test('presentation explanations are requested by authoring revision and returned detached', async ({ page }) => {
  const observed = await page.evaluate(async () => {
    __desktopFixture.setNextTerminalActionResult('DescribePresentation', {
      ok: true, sessionRevision: 18, facilityRevision: 8, authoringRevision: 5,
      terminals: [{ terminalId: 'terminal-security', nodes: [{
        nodeId: 'open-door', playerName: 'Door already open', visible: true,
        nameRules: [{ kind: 'state-name', precedence: 3, condition: { deviceState: { deviceId: 'door', stateId: 'open' } },
          text: 'Door already open', conditionHolds: true, playersSee: true }],
        visibilityRules: [], availabilityRules: [],
      }] }],
    });
    const explained = await desktopAPI.describePresentation({ expectedAuthoringRevision: 5 });
    const frozen = Object.isFrozen(explained) && Object.isFrozen(explained.terminals[0]);
    const call = __desktopFixture.calls.filter(entry => entry.method === 'DescribePresentation').at(-1);
    __desktopFixture.setNextTerminalActionResult('DescribePresentation', {
      ok: false, failure: 'stale-revision', authoringRevision: 6, terminals: [],
    });
    const stale = await desktopAPI.describePresentation({ expectedAuthoringRevision: -2 });
    const staleCall = __desktopFixture.calls.filter(entry => entry.method === 'DescribePresentation').at(-1);
    return { explained, frozen, call, stale, staleCall };
  });

  expect(observed.call.args).toEqual([{ expectedAuthoringRevision: 5 }]);
  expect(observed.frozen).toBe(true);
  expect(observed.explained).toEqual(expect.objectContaining({
    ok: true, failure: '', sessionRevision: 18, facilityRevision: 8, authoringRevision: 5,
  }));
  expect(observed.explained.terminals[0].nodes[0].nameRules[0]).toEqual(expect.objectContaining({
    kind: 'state-name', playersSee: true,
  }));
  expect(observed.staleCall.args).toEqual([{ expectedAuthoringRevision: 0 }], 'an invalid revision is normalized to zero');
  expect(observed.stale).toEqual(expect.objectContaining({ ok: false, failure: 'stale-revision', authoringRevision: 6, terminals: [] }));
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

// The four Overseer play changes, keyed by their validateFacilityMutationResult
// operation. Each request is sent at facility revision 8.
const FACILITY_PLAY_OPERATIONS = {
  'device-transition': {
    method: 'ApplyFacilityDeviceTransition',
    call: 'applyFacilityDeviceTransition',
    request: {
      deviceId: 'security-door', transitionId: 'open', expectedFacilityRevision: 8, correlationId: 'transition-8',
    },
  },
  'device-force': {
    method: 'ForceFacilityDeviceState',
    call: 'forceFacilityDeviceState',
    request: {
      deviceId: 'reactor-main', stateId: 'critical-overheat', expectedFacilityRevision: 8, correlationId: 'force-8',
    },
  },
  'condition-active': {
    method: 'SetFacilityConditionActive',
    call: 'setFacilityConditionActive',
    request: { conditionId: 'door-controller', active: true, expectedFacilityRevision: 8, correlationId: 'fault-8' },
  },
  'device-reset': {
    method: 'ResetFacilityDevice',
    call: 'resetFacilityDevice',
    request: { deviceId: 'security-door', expectedFacilityRevision: 8, correlationId: 'device-reset-8' },
  },
};

// playFacility has two faults scoped to the door, one to the grid, and one
// to a terminal, so the reset branch can tell the door's faults apart.
function playFacility(revision) {
  return {
    revision,
    devices: [{ id: 'security-door' }, { id: 'power-grid' }, { id: 'reactor-main' }],
    conditions: [
      { id: 'door-controller', device: { deviceId: 'security-door' } },
      { id: 'door-sensor', device: { deviceId: 'security-door' } },
      { id: 'grid-overload', device: { deviceId: 'power-grid' } },
      { id: 'terminal-lock', terminal: { terminalId: 't_demo' } },
    ],
    recoveryPrograms: [],
  };
}

function playSession(revision, facility = playFacility(revision)) {
  return { version: 1, name: `Canonical ${revision}`, terminals: [], facility };
}

function changedPlayResult(operation, affectedDeviceIds, affectedConditionIds, overrides = {}) {
  return {
    ok: true, changed: true, correlationId: FACILITY_PLAY_OPERATIONS[operation].request.correlationId,
    failure: '', issues: [], sessionRevision: 19, previousFacilityRevision: 8, resultingFacilityRevision: 9,
    affectedDeviceIds, affectedConditionIds, session: playSession(9), ...overrides,
  };
}

function unchangedPlayResult(operation) {
  return {
    ok: true, changed: false, correlationId: FACILITY_PLAY_OPERATIONS[operation].request.correlationId,
    failure: '', issues: [], sessionRevision: 18, previousFacilityRevision: 8, resultingFacilityRevision: 8,
    affectedDeviceIds: [], affectedConditionIds: [], session: playSession(8),
  };
}

function failedPlayResult(operation, failure, issue, revision, session) {
  const result = {
    ok: false, changed: false, correlationId: FACILITY_PLAY_OPERATIONS[operation].request.correlationId,
    failure, issues: [issue], sessionRevision: session ? 30 : 0,
    previousFacilityRevision: revision, resultingFacilityRevision: revision,
    affectedDeviceIds: [], affectedConditionIds: [],
  };
  return session === undefined ? result : { ...result, session };
}

// runFacilityPlayCases answers each case's call with its result through the
// fixture and returns what the facade resolved to, in case order.
function runFacilityPlayCases(page, cases) {
  return page.evaluate(async ({ operations, cases: entries }) => {
    const results = [];
    for (const { operation, request, result } of entries) {
      const { method, call } = operations[operation];
      __desktopFixture.setNextTerminalActionResult(method, result);
      results.push(await desktopAPI[call](request ?? operations[operation].request));
    }
    return results;
  }, { operations: FACILITY_PLAY_OPERATIONS, cases });
}

function playResultSummary(result) {
  return {
    ok: result.ok,
    changed: result.changed,
    correlationId: result.correlationId,
    failure: result.failure,
    sessionRevision: result.sessionRevision,
    previousFacilityRevision: result.previousFacilityRevision,
    resultingFacilityRevision: result.resultingFacilityRevision,
    affectedDeviceIds: result.affectedDeviceIds,
    affectedConditionIds: result.affectedConditionIds,
    sessionName: result.session?.name ?? null,
    sessionFacilityRevision: result.session?.facility?.revision ?? null,
  };
}

function expectPlayVerdicts(cases, results) {
  expect(results).toHaveLength(cases.length);
  cases.forEach(({ label, operation, verdict, result: sent }, index) => {
    const result = results[index];
    if (verdict === 'accept') {
      expect(playResultSummary(result), label).toEqual(playResultSummary(sent));
      return;
    }
    const { correlationId } = FACILITY_PLAY_OPERATIONS[operation].request;
    expect(result, label).toEqual(expect.objectContaining({
      ok: false,
      changed: false,
      correlationId,
      failure: 'conflict',
      issues: [{ code: 'conflict', entityKind: 'desktop-result' }],
      previousFacilityRevision: 8,
      resultingFacilityRevision: 8,
      affectedDeviceIds: [],
      affectedConditionIds: [],
      session: null,
    }));
  });
}

test('play change requests reach their bindings as exact normalized payloads', async ({ page }) => {
  const observed = await page.evaluate(async () => {
    const results = {
      transition: await desktopAPI.applyFacilityDeviceTransition({
        deviceId: 'security-door', transitionId: 'open', expectedFacilityRevision: 8, correlationId: 'transition-8',
        stateId: 'not-a-transition-field', sessionRevision: 4,
      }),
      force: await desktopAPI.forceFacilityDeviceState({
        deviceId: 'reactor-main', stateId: 'critical-overheat', expectedFacilityRevision: 8, correlationId: 'force-8',
        transitionId: 'not-a-force-field',
      }),
      fault: await desktopAPI.setFacilityConditionActive({
        conditionId: 'door-controller', active: true, expectedFacilityRevision: 8, correlationId: 'fault-8',
        deviceId: 'not-a-fault-field',
      }),
    };
    const frozen = Object.values(results).map(result => Object.isFrozen(result));

    await desktopAPI.applyFacilityDeviceTransition({ deviceId: 7, transitionId: null, expectedFacilityRevision: -1 });
    await desktopAPI.applyFacilityDeviceTransition('security-door');
    await desktopAPI.forceFacilityDeviceState({
      deviceId: 'reactor-main', stateId: ['standby'], expectedFacilityRevision: 8.5, correlationId: 9,
    });
    for (const active of ['true', 1, {}, null]) {
      await desktopAPI.setFacilityConditionActive({
        conditionId: 'door-controller', active, expectedFacilityRevision: 8, correlationId: 'fault-8',
      });
    }
    await desktopAPI.setFacilityConditionActive();
    return {
      results,
      frozen,
      calls: __desktopFixture.calls.filter(call => [
        'ApplyFacilityDeviceTransition', 'ForceFacilityDeviceState', 'SetFacilityConditionActive',
      ].includes(call.method)),
    };
  });

  const blankTransition = { deviceId: '', transitionId: '', expectedFacilityRevision: 0, correlationId: '' };
  const inactiveFault = { conditionId: 'door-controller', active: false, expectedFacilityRevision: 8, correlationId: 'fault-8' };
  expect(observed.calls).toEqual([
    {
      method: 'ApplyFacilityDeviceTransition',
      args: [{ deviceId: 'security-door', transitionId: 'open', expectedFacilityRevision: 8, correlationId: 'transition-8' }],
    },
    {
      method: 'ForceFacilityDeviceState',
      args: [{ deviceId: 'reactor-main', stateId: 'critical-overheat', expectedFacilityRevision: 8, correlationId: 'force-8' }],
    },
    {
      method: 'SetFacilityConditionActive',
      args: [{ conditionId: 'door-controller', active: true, expectedFacilityRevision: 8, correlationId: 'fault-8' }],
    },
    { method: 'ApplyFacilityDeviceTransition', args: [blankTransition] },
    { method: 'ApplyFacilityDeviceTransition', args: [blankTransition] },
    {
      method: 'ForceFacilityDeviceState',
      args: [{ deviceId: 'reactor-main', stateId: '', expectedFacilityRevision: 0, correlationId: '' }],
    },
    { method: 'SetFacilityConditionActive', args: [inactiveFault] },
    { method: 'SetFacilityConditionActive', args: [inactiveFault] },
    { method: 'SetFacilityConditionActive', args: [inactiveFault] },
    { method: 'SetFacilityConditionActive', args: [inactiveFault] },
    {
      method: 'SetFacilityConditionActive',
      args: [{ conditionId: '', active: false, expectedFacilityRevision: 0, correlationId: '' }],
    },
  ], 'only `active: true` switches a fault on');
  expect(observed.frozen).toEqual([true, true, true]);
  expect(observed.results.transition).toEqual(expect.objectContaining({
    ok: true, changed: true, correlationId: 'transition-8', previousFacilityRevision: 8, resultingFacilityRevision: 9,
    affectedDeviceIds: ['security-door'], affectedConditionIds: [],
  }));
  expect(observed.results.force).toEqual(expect.objectContaining({
    ok: true, changed: true, correlationId: 'force-8', previousFacilityRevision: 8, resultingFacilityRevision: 9,
    affectedDeviceIds: ['reactor-main'], affectedConditionIds: [],
  }));
  expect(observed.results.fault).toEqual(expect.objectContaining({
    ok: true, changed: true, correlationId: 'fault-8', previousFacilityRevision: 8, resultingFacilityRevision: 9,
    affectedDeviceIds: [], affectedConditionIds: ['door-controller'],
  }));
});

test('play change validators accept only the exact result of each operation', async ({ page }) => {
  const cases = [
    {
      label: 'a transition names its device and the faults its effects set',
      operation: 'device-transition', verdict: 'accept',
      result: changedPlayResult('device-transition', ['security-door'], ['door-controller', 'door-sensor']),
    },
    {
      label: 'a transition without fault effects names only its device',
      operation: 'device-transition', verdict: 'accept',
      result: changedPlayResult('device-transition', ['security-door'], []),
    },
    {
      label: 'a transition always changes its device',
      operation: 'device-transition', verdict: 'conflict',
      result: unchangedPlayResult('device-transition'),
    },
    {
      label: 'a transition that names no device',
      operation: 'device-transition', verdict: 'conflict',
      result: changedPlayResult('device-transition', [], ['door-controller']),
    },
    {
      label: 'a transition that names another device',
      operation: 'device-transition', verdict: 'conflict',
      result: changedPlayResult('device-transition', ['power-grid'], []),
    },
    {
      label: 'a transition that moves two devices',
      operation: 'device-transition', verdict: 'conflict',
      result: changedPlayResult('device-transition', ['security-door', 'power-grid'], []),
    },
    {
      label: 'a transition that repeats a fault',
      operation: 'device-transition', verdict: 'conflict',
      result: changedPlayResult('device-transition', ['security-door'], ['door-controller', 'door-controller']),
    },
    {
      label: 'a transition answered for another request',
      operation: 'device-transition', verdict: 'conflict',
      result: changedPlayResult('device-transition', ['security-door'], [], { correlationId: 'another-operation' }),
    },
    {
      label: 'a transition that skipped a facility revision',
      operation: 'device-transition', verdict: 'conflict',
      result: changedPlayResult('device-transition', ['security-door'], [], {
        resultingFacilityRevision: 10, session: playSession(10),
      }),
    },
    {
      label: 'a transition without the canonical session',
      operation: 'device-transition', verdict: 'conflict',
      result: changedPlayResult('device-transition', ['security-door'], [], { session: null }),
    },
    {
      label: 'a forced change names only its device',
      operation: 'device-force', verdict: 'accept',
      result: changedPlayResult('device-force', ['reactor-main'], []),
    },
    {
      label: 'forcing the current state changes nothing',
      operation: 'device-force', verdict: 'accept',
      result: unchangedPlayResult('device-force'),
    },
    {
      label: 'a forced change runs no fault effects',
      operation: 'device-force', verdict: 'conflict',
      result: changedPlayResult('device-force', ['reactor-main'], ['door-controller']),
    },
    {
      label: 'a forced change that names no device',
      operation: 'device-force', verdict: 'conflict',
      result: changedPlayResult('device-force', [], []),
    },
    {
      label: 'a forced change that names another device',
      operation: 'device-force', verdict: 'conflict',
      result: changedPlayResult('device-force', ['security-door'], []),
    },
    {
      label: 'a fault switch names only its fault',
      operation: 'condition-active', verdict: 'accept',
      result: changedPlayResult('condition-active', [], ['door-controller']),
    },
    {
      label: 'a fault switch accepts a terminal-scoped fault',
      operation: 'condition-active', verdict: 'accept',
      request: { ...FACILITY_PLAY_OPERATIONS['condition-active'].request, conditionId: 'terminal-lock' },
      result: changedPlayResult('condition-active', [], ['terminal-lock']),
    },
    {
      label: 'a fault already in the requested position changes nothing',
      operation: 'condition-active', verdict: 'accept',
      result: unchangedPlayResult('condition-active'),
    },
    {
      label: 'a fault switch moves no device',
      operation: 'condition-active', verdict: 'conflict',
      result: changedPlayResult('condition-active', ['security-door'], ['door-controller']),
    },
    {
      label: 'a fault switch that names another fault',
      operation: 'condition-active', verdict: 'conflict',
      result: changedPlayResult('condition-active', [], ['door-sensor']),
    },
    {
      label: 'a fault switch that names two faults',
      operation: 'condition-active', verdict: 'conflict',
      result: changedPlayResult('condition-active', [], ['door-controller', 'door-sensor']),
    },
    {
      label: 'a fault switch that names no fault',
      operation: 'condition-active', verdict: 'conflict',
      result: changedPlayResult('condition-active', [], []),
    },
    {
      label: 'a reset that restores only the device state',
      operation: 'device-reset', verdict: 'accept',
      result: changedPlayResult('device-reset', ['security-door'], []),
    },
    {
      label: 'a reset that restores the device state and its faults',
      operation: 'device-reset', verdict: 'accept',
      result: changedPlayResult('device-reset', ['security-door'], ['door-controller']),
    },
    {
      label: 'a fault-only reset names no device (R9)',
      operation: 'device-reset', verdict: 'accept',
      result: changedPlayResult('device-reset', [], ['door-controller', 'door-sensor']),
    },
    {
      label: 'a reset of a device already at its starting values changes nothing',
      operation: 'device-reset', verdict: 'accept',
      result: unchangedPlayResult('device-reset'),
    },
    {
      label: 'a changed reset that names nothing',
      operation: 'device-reset', verdict: 'conflict',
      result: changedPlayResult('device-reset', [], []),
    },
    {
      label: 'a reset that restores a fault of another device',
      operation: 'device-reset', verdict: 'conflict',
      result: changedPlayResult('device-reset', [], ['grid-overload']),
    },
    {
      label: 'a reset that restores a terminal-scoped fault',
      operation: 'device-reset', verdict: 'conflict',
      result: changedPlayResult('device-reset', ['security-door'], ['terminal-lock']),
    },
    {
      label: 'a reset that restores a fault missing from the session',
      operation: 'device-reset', verdict: 'conflict',
      result: changedPlayResult('device-reset', [], ['missing-fault']),
    },
    {
      label: 'a reset that names another device',
      operation: 'device-reset', verdict: 'conflict',
      result: changedPlayResult('device-reset', ['power-grid'], []),
    },
    {
      label: 'a reset that repeats a fault',
      operation: 'device-reset', verdict: 'conflict',
      result: changedPlayResult('device-reset', [], ['door-controller', 'door-controller']),
    },
  ];

  expectPlayVerdicts(cases, await runFacilityPlayCases(page, cases));
});

test('play change failures carry a session only when stale at the matching revision', async ({ page }) => {
  const operationFailures = {
    'device-transition': ['precondition-failed', { code: 'precondition-failed', entityKind: 'device', entityId: 'power-grid' }],
    'device-force': ['missing-reference', { code: 'missing-reference', entityKind: 'state', entityId: 'critical-overheat' }],
    'condition-active': ['missing-reference', { code: 'missing-reference', entityKind: 'condition', entityId: 'door-controller' }],
    'device-reset': ['missing-reference', { code: 'missing-reference', entityKind: 'device', entityId: 'security-door' }],
  };
  const stale = { code: 'stale-revision', entityKind: 'facility' };
  const cases = Object.entries(operationFailures).flatMap(([operation, [failure, issue]]) => [
    {
      label: `${operation}: a stale rejection keeps the canonical session at its facility revision`,
      operation, verdict: 'accept',
      result: failedPlayResult(operation, 'stale-revision', stale, 11, playSession(11)),
    },
    {
      label: `${operation}: a stale rejection without a session stays a stale rejection`,
      operation, verdict: 'accept',
      result: failedPlayResult(operation, 'stale-revision', stale, 11),
    },
    {
      label: `${operation}: a stale rejection whose session is at another facility revision`,
      operation, verdict: 'conflict',
      result: failedPlayResult(operation, 'stale-revision', stale, 11, playSession(10)),
    },
    {
      label: `${operation}: a stale rejection whose session has no facility`,
      operation, verdict: 'conflict',
      result: failedPlayResult(operation, 'stale-revision', stale, 11, { version: 1, name: 'No facility', terminals: [] }),
    },
    {
      label: `${operation}: a ${failure} rejection that carries a session`,
      operation, verdict: 'conflict',
      result: failedPlayResult(operation, failure, issue, 8, playSession(8)),
    },
    {
      label: `${operation}: a ${failure} rejection without a session`,
      operation, verdict: 'accept',
      result: failedPlayResult(operation, failure, issue, 8),
    },
  ]);

  const results = await runFacilityPlayCases(page, cases);
  expectPlayVerdicts(cases, results);
  cases.forEach(({ label, verdict, result: sent }, index) => {
    if (verdict === 'accept') expect(results[index].issues, label).toEqual(sent.issues);
  });
});

test('coordination-state feed entries are normalized, frozen, and malformed ones dropped', async ({ page }) => {
  const at = 1793390820000;
  const reasons = [
    'declined', 'precondition-failed', 'transition-unavailable', 'fault-blocked',
    'stale', 'withdrawn', 'failed', 'unspecified',
  ];
  const refusal = (sequence, reason) => ({
    sequence, deviceId: 'security-door', occurredAtUnixMs: at,
    playerRefusal: { terminalId: 't_demo', commandId: 'open-exit', reason },
  });
  const forced = { fromStateId: 'standby', toStateId: 'critical-overheat' };
  const malformed = [
    null,
    'forced',
    { sequence: 60, deviceId: 'security-door', occurredAtUnixMs: at },
    {
      sequence: 59, deviceId: 'security-door', occurredAtUnixMs: at,
      transition: { transitionId: 'open', fromStateId: 'sealed', toStateId: 'open' }, forced,
    },
    { sequence: 58, deviceId: 'reactor-main', occurredAtUnixMs: at, reset: { fromStateId: 'standby', toStateId: 'standby' }, forced },
    { sequence: 57, deviceId: '', occurredAtUnixMs: at, forced },
    { sequence: 56, occurredAtUnixMs: at, forced },
    { sequence: 55, deviceId: 42, occurredAtUnixMs: at, forced },
    { sequence: 2 ** 53, deviceId: 'reactor-main', occurredAtUnixMs: at, forced },
    { sequence: '54', deviceId: 'reactor-main', occurredAtUnixMs: at, forced },
    { sequence: 53.5, deviceId: 'reactor-main', occurredAtUnixMs: at, forced },
    { deviceId: 'reactor-main', occurredAtUnixMs: at, forced },
    { sequence: 52, deviceId: 'reactor-main', occurredAtUnixMs: at + 0.5, forced },
    { sequence: 51, deviceId: 'reactor-main', occurredAtUnixMs: String(at), forced },
    { sequence: 50, deviceId: 'reactor-main', forced },
  ];
  const valid = [
    {
      sequence: 40, deviceId: 'security-door', occurredAtUnixMs: at,
      transition: { transitionId: 'open', fromStateId: 'sealed', toStateId: 'open' },
    },
    { sequence: 39, deviceId: 'reactor-main', occurredAtUnixMs: at, forced },
    { sequence: 38, deviceId: 'security-door', occurredAtUnixMs: at, fault: { conditionId: 'door-controller', active: true } },
    {
      sequence: 37, deviceId: 'security-door', occurredAtUnixMs: at,
      reset: { fromStateId: 'open', toStateId: 'open', conditionIds: ['door-controller', 'door-sensor'] },
    },
    { sequence: 36, deviceId: 'power-grid', occurredAtUnixMs: at, sceneReset: { fromStateId: 'offline', toStateId: 'online' } },
    {
      sequence: 35, deviceId: 'security-door', occurredAtUnixMs: at,
      playerCommand: {
        terminalId: 't_demo', commandId: 'open-exit', transitionId: 'open', fromStateId: 'sealed', toStateId: 'open',
      },
    },
    ...reasons.map((reason, index) => refusal(34 - index, reason)),
    refusal(26, 'exploded'),
  ];
  const snapshotFeed = [
    { sequence: 5, deviceId: 'security-door', occurredAtUnixMs: at - 1, fault: { conditionId: 'door-sensor', active: false } },
    { sequence: 4, deviceId: '', occurredAtUnixMs: at - 1, fault: { conditionId: 'door-sensor', active: true } },
  ];

  const observed = await page.evaluate(async ({ eventFeed, snapshotFeed: statusFeed }) => {
    __desktopFixture.deferStatus();
    const states = [];
    const releases = [
      desktopAPI.onServerInfo(() => {}),
      desktopAPI.onClientCount(() => {}),
      desktopAPI.onHackState(() => {}),
      desktopAPI.onCoordinationState(value => states.push(value)),
    ];
    __desktopFixture.resolveStatus({
      serverInfo: { url: 'http://127.0.0.1:3690', tunnel: false },
      clientCount: 1,
      hackState: { attemptsLeft: 2 },
      coordinationState: { revision: 50, facilityRevision: 8, facilityChanges: statusFeed },
    });
    await new Promise(resolve => setTimeout(resolve, 0));
    __desktopFixture.emit('coordination-state', { revision: 51, facilityRevision: 9, facilityChanges: eventFeed });
    releases.forEach(release => release());
    return states.map(value => ({
      revision: value.revision,
      facilityRevision: value.facilityRevision,
      facilityChanges: value.facilityChanges,
      frozen: Array.isArray(value.facilityChanges) ? value.facilityChanges.map(entry => Object.isFrozen(entry)) : null,
    }));
  }, { eventFeed: [...malformed, ...valid], snapshotFeed });

  const entry = fields => ({
    transitionId: '',
    fromStateId: '',
    toStateId: '',
    conditionId: '',
    active: false,
    conditionIds: [],
    terminalId: '',
    commandId: '',
    reason: '',
    ...fields,
  });
  const expected = [
    entry({
      sequence: 40, deviceId: 'security-door', occurredAtUnixMs: at, kind: 'transition', author: 'overseer',
      transitionId: 'open', fromStateId: 'sealed', toStateId: 'open',
    }),
    entry({
      sequence: 39, deviceId: 'reactor-main', occurredAtUnixMs: at, kind: 'forced', author: 'overseer',
      fromStateId: 'standby', toStateId: 'critical-overheat',
    }),
    entry({
      sequence: 38, deviceId: 'security-door', occurredAtUnixMs: at, kind: 'fault', author: 'overseer',
      conditionId: 'door-controller', active: true,
    }),
    entry({
      sequence: 37, deviceId: 'security-door', occurredAtUnixMs: at, kind: 'reset', author: 'overseer',
      fromStateId: 'open', toStateId: 'open', conditionIds: ['door-controller', 'door-sensor'],
    }),
    entry({
      sequence: 36, deviceId: 'power-grid', occurredAtUnixMs: at, kind: 'scene-reset', author: 'overseer',
      fromStateId: 'offline', toStateId: 'online',
    }),
    entry({
      sequence: 35, deviceId: 'security-door', occurredAtUnixMs: at, kind: 'player-command', author: 'terminal',
      terminalId: 't_demo', commandId: 'open-exit', transitionId: 'open', fromStateId: 'sealed', toStateId: 'open',
    }),
    ...[...reasons, 'unspecified'].map((reason, index) => entry({
      sequence: index < reasons.length ? 34 - index : 26,
      deviceId: 'security-door',
      occurredAtUnixMs: at,
      kind: 'player-refusal',
      author: 'terminal',
      terminalId: 't_demo',
      commandId: 'open-exit',
      reason,
    })),
  ];

  expect(observed).toEqual([
    {
      revision: 50,
      facilityRevision: 8,
      facilityChanges: [entry({
        sequence: 5, deviceId: 'security-door', occurredAtUnixMs: at - 1, kind: 'fault', author: 'overseer',
        conditionId: 'door-sensor', active: false,
      })],
      frozen: [true],
    },
    {
      revision: 51,
      facilityRevision: 9,
      facilityChanges: expected,
      frozen: expected.map(() => true),
    },
  ]);
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

test('saveAuthoring snapshots the complete cross-terminal document at the Wails boundary', async ({ page }) => {
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
    const pending = desktopAPI.saveAuthoring({ session: candidate, expectedAuthoringRevision: 1, correlationId: 'boundary' });
    candidate.terminals.splice(1, 1);
    await pending;
    return __desktopFixture.calls.filter(call => call.method === 'SaveAuthoring').at(-1).args[0].session;
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
