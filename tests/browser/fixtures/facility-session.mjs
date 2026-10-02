const FACILITY_IDS = deepFreeze({
  terminals: {
    security: 'terminal-facility-security',
    reactor: 'terminal-facility-reactor',
    maintenance: 'terminal-facility-maintenance',
    network: 'terminal-facility-network',
    archive: 'terminal-facility-archive',
  },
  groups: {
    operations: 'group-facility-operations',
    engineering: 'group-facility-engineering',
    records: 'group-facility-records',
  },
  devices: {
    power: 'device-primary-power',
    cooling: 'device-reactor-cooling',
    reactor: 'device-main-reactor',
    door: 'device-security-door',
    alarm: 'device-security-alarm',
    network: 'device-operations-network',
  },
  conditions: {
    offline: 'condition-security-offline',
    unpowered: 'condition-reactor-unpowered',
    networkIsolated: 'condition-network-isolated',
    storageDamaged: 'condition-archive-damaged',
    authorizationCorrupted: 'condition-security-authorization',
    displayUnstable: 'condition-reactor-display',
    custom: 'condition-cooling-contamination',
  },
  programs: {
    networkRecovery: 'program-network-recovery',
  },
  nodes: {
    securityStatus: 'entry-security-status',
    securityPowerBlock: 'block-security-power',
    securityDoorBlock: 'block-security-door',
    openDoor: 'command-open-security-door',
    restrictedArchive: 'folder-restricted-archive',
    reactorStatus: 'entry-reactor-status',
    reactorPowerBlock: 'block-reactor-power',
    reactorCoreBlock: 'block-reactor-core',
    startReactor: 'command-start-reactor',
    maintenanceDiagnostics: 'folder-maintenance-diagnostics',
    runNetworkRecovery: 'command-network-recovery',
    networkDiagnostics: 'folder-network-diagnostics',
    networkReport: 'entry-network-report',
    archiveRecord: 'entry-archive-record',
    archiveRecordBlock: 'block-archive-record',
    legacyPowerCommand: 'command-legacy-power',
  },
});

function deepFreeze(value) {
  if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
  Object.freeze(value);
  for (const nested of Object.values(value)) deepFreeze(nested);
  return value;
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function state(id, name) {
  return { id, name };
}

function equality(deviceId, stateId) {
  return { deviceId, stateId };
}

function transitionRequest(deviceId, transitionId) {
  return { deviceId, transitionId };
}

function device({ id, name, kind, initialStateId, states, transitions }) {
  return {
    id,
    name,
    kind,
    initialStateId,
    currentStateId: initialStateId,
    states,
    transitions,
  };
}

function terminal(id, name, children) {
  return {
    id,
    name,
    hackLevel: 0,
    introText: `${name.toUpperCase()} // FACILITY NETWORK`,
    root: { id: 'root', type: 'folder', name: 'ROOT', children },
  };
}

function baseFacility() {
  const { devices, conditions, programs } = FACILITY_IDS;
  return {
    revision: 0,
    devices: [
      device({
        id: devices.power,
        name: 'Primary power grid',
        kind: 'power-grid',
        initialStateId: 'offline',
        states: [state('offline', 'Offline'), state('online', 'Online')],
        transitions: [{
          id: 'restore',
          name: 'Restore primary power',
          sourceStateId: 'offline',
          destinationStateId: 'online',
          preconditions: [equality(devices.cooling, 'online')],
          conditionEffects: [{ conditionId: conditions.unpowered, active: false }],
          recovery: true,
        }, {
          id: 'disconnect',
          name: 'Disconnect primary power',
          sourceStateId: 'online',
          destinationStateId: 'offline',
          conditionEffects: [{ conditionId: conditions.unpowered, active: true }],
        }],
      }),
      device({
        id: devices.cooling,
        name: 'Reactor cooling loop',
        kind: 'ventilation',
        initialStateId: 'offline',
        states: [state('offline', 'Offline'), state('online', 'Online')],
        transitions: [{
          id: 'prime',
          name: 'Prime cooling loop',
          sourceStateId: 'offline',
          destinationStateId: 'online',
          conditionEffects: [{ conditionId: conditions.custom, active: false }],
          recovery: true,
        }],
      }),
      device({
        id: devices.reactor,
        name: 'Main reactor',
        kind: 'reactor',
        initialStateId: 'offline',
        states: [
          state('offline', 'Offline'),
          state('starting', 'Starting'),
          state('online', 'Online'),
          state('scrammed', 'Scrammed'),
        ],
        transitions: [{
          id: 'start',
          name: 'Start reactor',
          sourceStateId: 'offline',
          destinationStateId: 'online',
          preconditions: [
            equality(devices.power, 'online'),
            equality(devices.cooling, 'online'),
          ],
          conditionEffects: [
            { conditionId: conditions.unpowered, active: false },
            { conditionId: conditions.displayUnstable, active: false },
          ],
        }, {
          id: 'scram',
          name: 'Scram reactor',
          sourceStateId: 'online',
          destinationStateId: 'scrammed',
          conditionEffects: [{ conditionId: conditions.displayUnstable, active: true }],
        }],
      }),
      device({
        id: devices.door,
        name: 'Security sector door',
        kind: 'door',
        initialStateId: 'locked',
        states: [state('locked', 'Locked'), state('open', 'Open')],
        transitions: [{
          id: 'open',
          name: 'Open security door',
          sourceStateId: 'locked',
          destinationStateId: 'open',
          preconditions: [equality(devices.power, 'online')],
          conditionEffects: [{ conditionId: conditions.authorizationCorrupted, active: false }],
        }, {
          id: 'secure',
          name: 'Secure security door',
          sourceStateId: 'open',
          destinationStateId: 'locked',
        }],
      }),
      device({
        id: devices.alarm,
        name: 'Security alarm',
        kind: 'alarm',
        initialStateId: 'armed',
        states: [state('armed', 'Armed'), state('silent', 'Silent')],
        transitions: [{
          id: 'silence',
          name: 'Silence security alarm',
          sourceStateId: 'armed',
          destinationStateId: 'silent',
        }, {
          id: 'arm',
          name: 'Arm security alarm',
          sourceStateId: 'silent',
          destinationStateId: 'armed',
        }],
      }),
      device({
        id: devices.network,
        name: 'Operations network',
        kind: 'network-segment',
        initialStateId: 'isolated',
        states: [state('isolated', 'Isolated'), state('connected', 'Connected')],
        transitions: [{
          id: 'reconnect',
          name: 'Reconnect operations network',
          sourceStateId: 'isolated',
          destinationStateId: 'connected',
          preconditions: [equality(devices.power, 'online')],
          conditionEffects: [{ conditionId: conditions.networkIsolated, active: false }],
          recovery: true,
        }],
      }),
    ],
    conditions: baseConditions(),
    recoveryPrograms: [{
      id: programs.networkRecovery,
      name: 'VAULT-TEC NETWORK RECOVERY',
      transitions: [transitionRequest(devices.network, 'reconnect')],
    }],
  };
}

function baseConditions() {
  const { terminals, devices, conditions, programs, nodes } = FACILITY_IDS;
  return [
    {
      id: conditions.offline,
      name: 'Security terminal offline',
      category: 'offline',
      terminal: { terminalId: terminals.security },
      initialActive: false,
      currentActive: false,
      effects: [{ capabilityBlock: { capability: 'view-entry' } }],
      recovery: [{ privateOverseerAction: true }],
    },
    {
      id: conditions.unpowered,
      name: 'Reactor controls unpowered',
      category: 'unpowered',
      device: { deviceId: devices.reactor },
      initialActive: true,
      currentActive: true,
      effects: [{ capabilityBlock: { capability: 'execute-command' } }],
      recovery: [{ transition: transitionRequest(devices.power, 'restore') }],
    },
    {
      id: conditions.networkIsolated,
      name: 'Operations network isolated',
      category: 'network-isolated',
      terminal: { terminalId: terminals.network },
      initialActive: true,
      currentActive: true,
      effects: [
        { capabilityBlock: { capability: 'terminal-transition' } },
        { diagnosticPath: { terminalId: terminals.network, nodeId: nodes.networkDiagnostics } },
      ],
      recovery: [{ recoveryProgramId: programs.networkRecovery }, { privateOverseerAction: true }],
    },
    {
      id: conditions.storageDamaged,
      name: 'Archive storage damaged',
      category: 'storage-damaged',
      terminal: { terminalId: terminals.archive },
      initialActive: false,
      currentActive: false,
      effects: [{
        recordSubstitution: {
          terminalId: terminals.archive,
          blockId: nodes.archiveRecordBlock,
          replacementText: 'R_C_RD 04-B // S_CT_R 7 C_RR_PT_D',
        },
      }],
      recovery: [{ privateOverseerAction: true }],
    },
    {
      id: conditions.authorizationCorrupted,
      name: 'Security authorization corrupted',
      category: 'authorization-corrupted',
      device: { deviceId: devices.door },
      initialActive: true,
      currentActive: true,
      effects: [{ capabilityBlock: { capability: 'execute-command' } }],
      recovery: [{ transition: transitionRequest(devices.door, 'open') }, { privateOverseerAction: true }],
    },
    {
      id: conditions.displayUnstable,
      name: 'Reactor display unstable',
      category: 'display-unstable',
      terminal: { terminalId: terminals.reactor },
      initialActive: false,
      currentActive: false,
      effects: [{ displayInstability: {} }],
      recovery: [{ privateOverseerAction: true }],
    },
    {
      id: conditions.custom,
      name: 'Cooling loop contamination',
      category: 'custom',
      customCategory: 'coolant-contamination',
      device: { deviceId: devices.cooling },
      initialActive: true,
      currentActive: true,
      effects: [{ capabilityBlock: { capability: 'hack' } }],
      recovery: [{ transition: transitionRequest(devices.cooling, 'prime') }],
    },
  ];
}

function baseTerminals() {
  const { terminals, devices, nodes, programs } = FACILITY_IDS;
  return [
    {
      ...terminal(terminals.security, 'Security control', [{
        id: nodes.securityStatus,
        type: 'entry',
        name: 'FACILITY STATUS',
        facilityNameVariants: [{ when: equality(devices.door, 'open'), text: 'FACILITY STATUS // ACCESS OPEN' }],
        blocks: [{
          id: nodes.securityPowerBlock,
          initialText: 'PRIMARY POWER: OFFLINE',
          facilityTextVariants: [{ when: equality(devices.power, 'online'), text: 'PRIMARY POWER: ONLINE' }],
        }, {
          id: nodes.securityDoorBlock,
          initialText: 'SECURITY DOOR: LOCKED',
          facilityTextVariants: [{ when: equality(devices.door, 'open'), text: 'SECURITY DOOR: OPEN' }],
        }],
      }, {
        id: nodes.openDoor,
        type: 'command',
        name: 'OPEN SECURITY DOOR',
        text: 'Security door and alarm updated.',
        availableWhen: equality(devices.power, 'online'),
        stateChange: {
          completedName: 'SECURITY DOOR OPEN',
          confirmationText: 'Authorize the security-sector world action?',
          entryContentChange: {
            blockId: nodes.securityDoorBlock,
            completedText: 'SECURITY DOOR: LEGACY COMMAND COMPLETE',
          },
          facilityAction: {
            transitions: {
              transitions: [
                transitionRequest(devices.door, 'open'),
                transitionRequest(devices.alarm, 'silence'),
              ],
            },
          },
        },
      }, {
        id: nodes.restrictedArchive,
        type: 'folder',
        name: 'RESTRICTED ARCHIVE',
        visibleWhen: equality(devices.door, 'open'),
        children: [{
          id: 'entry-security-clearance',
          type: 'entry',
          name: 'CLEARANCE ACCEPTED',
          description: 'Protected records are now available.',
        }],
      }]),
    },
    terminal(terminals.reactor, 'Reactor control', [{
      id: nodes.reactorStatus,
      type: 'entry',
      name: 'REACTOR STATUS',
      blocks: [{
        id: nodes.reactorPowerBlock,
        initialText: 'CONTROL POWER: OFFLINE',
        facilityTextVariants: [{ when: equality(devices.power, 'online'), text: 'CONTROL POWER: ONLINE' }],
      }, {
        id: nodes.reactorCoreBlock,
        initialText: 'REACTOR CORE: OFFLINE',
        facilityTextVariants: [
          { when: equality(devices.reactor, 'online'), text: 'REACTOR CORE: ONLINE' },
          { when: equality(devices.reactor, 'scrammed'), text: 'REACTOR CORE: SCRAMMED' },
        ],
      }],
    }, {
      id: nodes.startReactor,
      type: 'command',
      name: 'START MAIN REACTOR',
      text: 'Main reactor startup complete.',
      availableWhen: equality(devices.power, 'online'),
      stateChange: {
        completedName: 'MAIN REACTOR ONLINE',
        confirmationText: 'Authorize reactor startup?',
        facilityAction: {
          transitions: { transitions: [transitionRequest(devices.reactor, 'start')] },
        },
      },
    }]),
    terminal(terminals.maintenance, 'Maintenance station', [{
      id: nodes.maintenanceDiagnostics,
      type: 'folder',
      name: 'DIAGNOSTIC TOOLS',
      children: [{
        id: nodes.runNetworkRecovery,
        type: 'command',
        name: 'RUN NETWORK RECOVERY HOLOTAPE',
        text: 'Network recovery program completed.',
        stateChange: {
          completedName: 'NETWORK RECOVERY COMPLETE',
          confirmationText: 'Run the compatible recovery program?',
          facilityAction: { recoveryProgramId: programs.networkRecovery },
        },
      }],
    }]),
    terminal(terminals.network, 'Network operations', [{
      id: nodes.networkReport,
      type: 'entry',
      name: 'NETWORK STATUS',
      blocks: [{
        id: 'block-network-status',
        initialText: 'NETWORK: ISOLATED',
        facilityTextVariants: [{ when: equality(devices.network, 'connected'), text: 'NETWORK: CONNECTED' }],
      }],
    }, {
      id: nodes.networkDiagnostics,
      type: 'folder',
      name: 'ISOLATION DIAGNOSTICS',
      visibleWhen: equality(devices.network, 'connected'),
      children: [{
        id: 'entry-network-diagnostics',
        type: 'entry',
        name: 'DIAGNOSTIC REPORT',
        description: 'Run the network recovery holotape from a maintenance station.',
      }],
    }]),
    terminal(terminals.archive, 'Records archive', [{
      id: nodes.archiveRecord,
      type: 'entry',
      name: 'RECORD 04-B',
      blocks: [{
        id: nodes.archiveRecordBlock,
        initialText: 'RECORD 04-B // SECTOR 7 CORRIDOR PRESSURE NOMINAL',
      }],
    }, {
      id: nodes.legacyPowerCommand,
      type: 'command',
      name: 'LEGACY DIAGNOSTIC',
      text: 'Legacy command remains available.',
    }]),
  ];
}

function baseSession() {
  const { terminals, groups } = FACILITY_IDS;
  return {
    version: 1,
    name: 'Shared facility browser fixture',
    terminals: baseTerminals(),
    terminalGroups: [{
      id: groups.operations,
      name: 'Operations',
      terminalIds: [terminals.security, terminals.network],
    }, {
      id: groups.engineering,
      name: 'Engineering',
      terminalIds: [terminals.reactor, terminals.maintenance],
    }, {
      id: groups.records,
      name: 'Records',
      terminalIds: [terminals.archive],
    }],
    facility: baseFacility(),
  };
}

function findById(values, id, kind) {
  const value = values.find(candidate => candidate.id === id);
  if (!value) throw new RangeError(`unknown facility fixture ${kind}: ${id}`);
  return value;
}

function applyFacilityState(session, { revision, deviceStates = {}, conditionStates = {} } = {}) {
  if (revision !== undefined) {
    if (!Number.isSafeInteger(revision) || revision < 0) {
      throw new TypeError('facility fixture revision must be a non-negative safe integer');
    }
    session.facility.revision = revision;
  }
  for (const [deviceId, stateId] of Object.entries(deviceStates)) {
    const candidate = findById(session.facility.devices, deviceId, 'device');
    if (!candidate.states.some(value => value.id === stateId)) {
      throw new RangeError(`unknown state ${stateId} for facility fixture device ${deviceId}`);
    }
    candidate.currentStateId = stateId;
  }
  for (const [conditionId, active] of Object.entries(conditionStates)) {
    if (typeof active !== 'boolean') {
      throw new TypeError(`facility fixture condition ${conditionId} must be boolean`);
    }
    findById(session.facility.conditions, conditionId, 'condition').currentActive = active;
  }
  return session;
}

function buildFacilityAuthoringSession({ name = 'Facility authoring browser fixture' } = {}) {
  const session = baseSession();
  session.name = name;
  return session;
}

function buildMultiTerminalFacilitySession({ revision = 7 } = {}) {
  const { devices, conditions } = FACILITY_IDS;
  const session = baseSession();
  session.name = 'Multi-terminal facility projection fixture';
  applyFacilityState(session, {
    revision,
    deviceStates: {
      [devices.power]: 'online',
      [devices.cooling]: 'online',
      [devices.reactor]: 'online',
      [devices.door]: 'open',
      [devices.alarm]: 'silent',
      [devices.network]: 'connected',
    },
    conditionStates: {
      [conditions.unpowered]: false,
      [conditions.networkIsolated]: false,
      [conditions.authorizationCorrupted]: false,
      [conditions.custom]: false,
    },
  });
  findById(session.terminals, FACILITY_IDS.terminals.security, 'terminal').commandStates = {
    [FACILITY_IDS.nodes.openDoor]: {
      completedName: 'SECURITY DOOR OPEN // LEGACY SNAPSHOT',
      resultText: 'Previously completed command result.',
      entryContentChange: {
        blockId: FACILITY_IDS.nodes.securityDoorBlock,
        completedText: 'SECURITY DOOR: LEGACY COMMAND COMPLETE',
      },
    },
  };
  return session;
}

function buildDiagnosticFacilitySession({ revision = 11, activeConditions } = {}) {
  const { devices, conditions } = FACILITY_IDS;
  const selected = activeConditions ?? Object.values(conditions);
  if (!Array.isArray(selected)) {
    throw new TypeError('diagnostic fixture activeConditions must be an array');
  }
  const selectedSet = new Set(selected);
  const known = new Set(Object.values(conditions));
  for (const conditionId of selectedSet) {
    if (!known.has(conditionId)) throw new RangeError(`unknown diagnostic fixture condition: ${conditionId}`);
  }
  const conditionStates = Object.fromEntries([...known].map(id => [id, selectedSet.has(id)]));
  const session = baseSession();
  session.name = 'Deterministic facility diagnostics fixture';
  return applyFacilityState(session, {
    revision,
    deviceStates: {
      [devices.power]: 'offline',
      [devices.cooling]: 'offline',
      [devices.reactor]: 'scrammed',
      [devices.door]: 'locked',
      [devices.alarm]: 'armed',
      [devices.network]: 'isolated',
    },
    conditionStates,
  });
}

function facilityStateSnapshot(session) {
  if (!session?.facility) return null;
  return {
    revision: session.facility.revision,
    deviceStates: Object.fromEntries(session.facility.devices.map(candidate => [
      candidate.id,
      candidate.currentStateId,
    ])),
    conditionStates: Object.fromEntries(session.facility.conditions.map(candidate => [
      candidate.id,
      candidate.currentActive,
    ])),
  };
}

function buildFacilityLifecycleFixture({ sessionRevision = 23, facilityRevision = 19 } = {}) {
  if (!Number.isSafeInteger(sessionRevision) || sessionRevision < 0) {
    throw new TypeError('session fixture revision must be a non-negative safe integer');
  }
  const { devices, conditions } = FACILITY_IDS;
  const session = baseSession();
  session.name = 'Durable facility lifecycle fixture';
  applyFacilityState(session, {
    revision: facilityRevision,
    deviceStates: {
      [devices.power]: 'online',
      [devices.cooling]: 'online',
      [devices.reactor]: 'scrammed',
      [devices.door]: 'open',
      [devices.alarm]: 'silent',
      [devices.network]: 'connected',
    },
    conditionStates: {
      [conditions.offline]: false,
      [conditions.unpowered]: false,
      [conditions.networkIsolated]: false,
      [conditions.storageDamaged]: true,
      [conditions.authorizationCorrupted]: false,
      [conditions.displayUnstable]: true,
      [conditions.custom]: false,
    },
  });
  findById(session.terminals, FACILITY_IDS.terminals.security, 'terminal').commandStates = {
    [FACILITY_IDS.nodes.openDoor]: {
      completedName: 'SECURITY DOOR OPEN',
      resultText: 'Security door and alarm updated.',
      entryContentChange: {
        blockId: FACILITY_IDS.nodes.securityDoorBlock,
        completedText: 'SECURITY DOOR: LEGACY COMMAND COMPLETE',
      },
    },
  };
  return {
    revision: sessionRevision,
    session,
    expectedFacilityState: facilityStateSnapshot(session),
  };
}

export {
  FACILITY_IDS,
  applyFacilityState,
  buildDiagnosticFacilitySession,
  buildFacilityAuthoringSession,
  buildFacilityLifecycleFixture,
  buildMultiTerminalFacilitySession,
  clone as cloneFacilityFixture,
  facilityStateSnapshot,
};
