'use strict';

import { Clipboard, Events } from '@wailsio/runtime';
import * as desktopService from '../bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js';

const APP_METHODS = Object.freeze({
  getRuntimeStatus: desktopService.GetRuntimeStatus,
  getApplicationUpdateStatus: desktopService.GetApplicationUpdateStatus,
  newSession: desktopService.NewSession,
  openSession: desktopService.OpenSession,
  saveSession: desktopService.SaveSession,
  loadReferencedPlayerConfig: desktopService.LoadReferencedPlayerConfig,
  newPlayerConfig: desktopService.NewPlayerConfig,
  openPlayerConfig: desktopService.OpenPlayerConfig,
  requestTerminalActivation: desktopService.RequestTerminalActivation,
  updateLiveTerminal: desktopService.UpdateLiveTerminal,
  requestTerminalClear: desktopService.RequestTerminalClear,
  resolveTerminalSwitch: desktopService.ResolveTerminalSwitch,
  resolveCommandExecution: desktopService.ResolveCommandExecution,
  resolveTerminalNavigation: desktopService.ResolveTerminalNavigation,
  forceHackSuccess: desktopService.ForceHackSuccess,
  resetFailedHack: desktopService.ResetFailedHack,
  resetCommandState: desktopService.ResetCommandState,
  resetTerminalCommandStates: desktopService.ResetTerminalCommandStates,
  resolveApplicationUpdateOffer: desktopService.ResolveApplicationUpdateOffer,
  resolveApplicationUpdateRestart: desktopService.ResolveApplicationUpdateRestart,
  replaceTerminalGroups: desktopService.ReplaceTerminalGroups,
  inspectFacilityDependencies: desktopService.InspectFacilityDependencies,
  previewFacility: desktopService.PreviewFacility,
  recoverFacilityCondition: desktopService.RecoverFacilityCondition,
  resetFacility: desktopService.ResetFacility,
  resetFacilityDevice: desktopService.ResetFacilityDevice,
  saveFacilityAuthoring: desktopService.SaveFacilityAuthoring,
  addCharacter: desktopService.AddCharacter,
  updateCharacter: desktopService.UpdateCharacter,
  deleteCharacter: desktopService.DeleteCharacter,
  renameLogicalSession: desktopService.RenameLogicalSession,
  assignCharacter: desktopService.AssignCharacter,
  releaseCharacter: desktopService.ReleaseCharacter,
  moveCharacter: desktopService.MoveCharacter,
  setActiveController: desktopService.SetActiveController,
  startBroadcast: desktopService.StartBroadcast,
  endBroadcast: desktopService.EndBroadcast,
  openUrl: desktopService.OpenURL,
  openLogLocation: desktopService.OpenLogLocation,
  getPublicAccess: desktopService.GetPublicAccess,
  copyPublicAccessCredentials: desktopService.CopyPublicAccessCredentials,
  savePublicAccessSettings: desktopService.SavePublicAccessSettings,
  generatePlayerPassword: desktopService.GeneratePlayerPassword,
  startPublicAccess: desktopService.StartPublicAccess,
  stopPublicAccess: desktopService.StopPublicAccess,
});

const DISPOSE = Symbol.for('fallout-terminal.desktop-api.dispose');
const subscriptions = new Set();
const eventSubscriptions = new Map();
const requiredEvents = Object.freeze([
  ['server-info', 'serverInfo'],
  ['client-count', 'clientCount'],
  ['hack-state', 'hackState'],
  ['coordination-state', 'coordinationState'],
]);

function invoke(binding, ...args) {
  try {
    if (typeof binding !== 'function') throw new Error('Wails desktop binding is unavailable');
    return Promise.resolve(binding(...args));
  } catch (error) {
    return Promise.reject(error);
  }
}

function command(binding, ...args) {
  return invoke(binding, ...args).catch((error) => ({
    ok: false,
    error: error instanceof Error ? error.message : String(error),
  }));
}

function snapshotPortableSession(session) {
  if (typeof globalThis.structuredClone === 'function') return globalThis.structuredClone(session);
  return JSON.parse(JSON.stringify(session));
}

function normalizeTerminalGroups(groups) {
  if (!Array.isArray(groups)) return [];
  return groups.map((group) => {
    const value = group && typeof group === 'object' ? group : {};
    return {
      id: typeof value.id === 'string' ? value.id : '',
      name: typeof value.name === 'string' ? value.name : '',
      terminalIds: Array.isArray(value.terminalIds)
        ? value.terminalIds.filter(terminalId => typeof terminalId === 'string')
        : [],
    };
  });
}

function saveSessionCommand(session) {
  try {
    // Wails may complete the native call asynchronously. Capture the complete
    // portable document now so later UI/event mutations cannot change which
    // terminals the backend validates for this revision.
    return command(APP_METHODS.saveSession, snapshotPortableSession(session));
  } catch (error) {
    return Promise.resolve({
      ok: false,
      error: error instanceof Error ? error.message : String(error),
    });
  }
}

function normalizePortableSession(session) {
  if (!session || typeof session !== 'object') return null;
  return { ...snapshotPortableSession(session), terminalGroups: normalizeTerminalGroups(session.terminalGroups) };
}

function normalizeSessionResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  return Object.freeze({ ...value, session: normalizePortableSession(value.session) });
}

async function writeClipboardText(value) {
  if (typeof value !== 'string' || value === '') return false;
  try {
    if (typeof Clipboard?.SetText !== 'function') return false;
    await Clipboard.SetText(value);
    return true;
  } catch {
    return false;
  }
}

const TERMINAL_SWITCH_STATUSES = new Set(['activated', 'cleared', 'decision-required', 'cancelled']);

function normalizeSwitchCommandResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  const status = typeof value.status === 'string' && TERMINAL_SWITCH_STATUSES.has(value.status)
    ? value.status
    : '';
  const switchId = typeof value.switchId === 'string' ? value.switchId : '';
  const state = value.state && typeof value.state === 'object' ? value.state : null;
  let error = typeof value.error === 'string' ? value.error : '';
  if (!ok && !error) error = 'Terminal switch command failed';
  return Object.freeze({ ok, error, status, switchId, state });
}

function switchCommand(binding, ...args) {
  return command(binding, ...args).then(normalizeSwitchCommandResult);
}

function normalizeCommandExecutionResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  const state = value.state && typeof value.state === 'object' ? value.state : null;
  const facilityResult = value.facilityResult && typeof value.facilityResult === 'object'
    ? normalizeFacilityOperationResult(value.facilityResult)
    : null;
  const error = ok ? '' : 'СОСТОЯНИЕ КОМАНДЫ НЕ УДАЛОСЬ СОХРАНИТЬ';
  return Object.freeze({ ok, error, state, facilityResult });
}

function normalizeTerminalNavigationResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  const state = value.state && typeof value.state === 'object' ? value.state : null;
  const error = ok ? '' : (typeof value.error === 'string' && value.error
    ? value.error
    : 'ПЕРЕХОД БОЛЬШЕ НЕ ДЕЙСТВИТЕЛЕН');
  return Object.freeze({ ok, error, state });
}

function normalizePlayerConfigResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  const canceled = value.canceled === true;
  const config = value.playerConfig && typeof value.playerConfig === 'object' ? value.playerConfig : null;
  const session = normalizePortableSession(value.session);
  const state = value.state && typeof value.state === 'object' ? value.state : null;
  let error = typeof value.error === 'string' ? value.error : '';
  if (!ok && !canceled && !error) error = 'Player config command failed';
  return Object.freeze({ ok, canceled, error, config, session, state });
}

function playerConfigCommand(binding, ...args) {
  return command(binding, ...args).then(normalizePlayerConfigResult);
}

function normalizeSessionStateResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  const revision = Number.isSafeInteger(value.revision) ? value.revision : 0;
  const session = normalizePortableSession(value.session);
  let error = typeof value.error === 'string' ? value.error : '';
  if (!ok && !error) error = 'Command state mutation failed';
  return Object.freeze({ ok, error, revision, session });
}

function sessionStateCommand(binding, payload) {
  return command(binding, payload).then(normalizeSessionStateResult);
}

function normalizeTerminalGroupReplacementPayload(payload) {
  const source = payload && typeof payload === 'object' ? payload : {};
  return {
    terminalGroups: normalizeTerminalGroups(source.terminalGroups),
    expectedSessionRevision: Number.isSafeInteger(source.expectedSessionRevision)
      && source.expectedSessionRevision >= 0 ? source.expectedSessionRevision : 0,
    expectedCoordinationRevision: Number.isSafeInteger(source.expectedCoordinationRevision)
      && source.expectedCoordinationRevision >= 0 ? source.expectedCoordinationRevision : 0,
  };
}

function normalizeTerminalGroupReplacementResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  let error = typeof value.error === 'string' ? value.error : '';
  if (ok) error = '';
  if (!ok && !error) error = 'Terminal group replacement failed';
  return Object.freeze({
    ok,
    error,
    sessionRevision: Number.isSafeInteger(value.sessionRevision) && value.sessionRevision >= 0
      ? value.sessionRevision
      : 0,
    session: normalizePortableSession(value.session),
    coordinationState: value.coordinationState && typeof value.coordinationState === 'object'
      ? snapshotPortableSession(value.coordinationState)
      : null,
  });
}

function replaceTerminalGroupsCommand(payload) {
  try {
    const request = normalizeTerminalGroupReplacementPayload(payload);
    return command(APP_METHODS.replaceTerminalGroups, request)
      .then(normalizeTerminalGroupReplacementResult);
  } catch (error) {
    return Promise.resolve(normalizeTerminalGroupReplacementResult({
      error: error instanceof Error ? error.message : String(error),
    }));
  }
}

const FACILITY_FAILURES = new Set([
  'unspecified',
  'rejected',
  'missing-reference',
  'invalid-transition',
  'precondition-failed',
  'stale-revision',
  'conflict',
  'duplicate',
  'invalid-configuration',
  'persistence-failed',
  'runtime-context-ended',
]);
const FACILITY_ENTITY_KINDS = new Set([
  'unspecified',
  'device',
  'device-state',
  'device-transition',
  'condition',
  'recovery-program',
]);
const FACILITY_DEPENDENCY_KINDS = new Set([
  'unspecified',
  'transition-precondition',
  'transition-condition-effect',
  'recovery-reference',
  'recovery-program-transition',
  'command-action',
  'name-variant',
  'entry-content-variant',
  'visibility',
  'availability',
  'diagnostic-scope',
  'diagnostic-effect',
]);

function facilityFailure(value, ok = false) {
  if (ok) return '';
  return typeof value === 'string' && FACILITY_FAILURES.has(value) ? value : 'unspecified';
}

function normalizeFacilityIssue(issue) {
  const value = issue && typeof issue === 'object' ? issue : {};
  const result = {
    code: facilityFailure(value.code),
    entityKind: typeof value.entityKind === 'string' ? value.entityKind : '',
  };
  if (Object.hasOwn(value, 'entityId') && typeof value.entityId === 'string') {
    result.entityId = value.entityId;
  }
  if (Object.hasOwn(value, 'referenceKind') && typeof value.referenceKind === 'string') {
    result.referenceKind = value.referenceKind;
  }
  if (Object.hasOwn(value, 'referenceId') && typeof value.referenceId === 'string') {
    result.referenceId = value.referenceId;
  }
  return Object.freeze(result);
}

function normalizeFacilityEntityReference(reference) {
  if (!reference || typeof reference !== 'object') return null;
  const rawKind = typeof reference.kind === 'string' ? reference.kind : '';
  const result = {
    kind: FACILITY_ENTITY_KINDS.has(rawKind) ? rawKind : 'unspecified',
    entityId: typeof reference.entityId === 'string' ? reference.entityId : '',
  };
  if (Object.hasOwn(reference, 'ownerId') && typeof reference.ownerId === 'string') {
    result.ownerId = reference.ownerId;
  }
  return Object.freeze(result);
}

function normalizeFacilityDependency(dependency) {
  const value = dependency && typeof dependency === 'object' ? dependency : {};
  const rawKind = typeof value.kind === 'string' ? value.kind : '';
  const result = {
    kind: FACILITY_DEPENDENCY_KINDS.has(rawKind) ? rawKind : 'unspecified',
    sourceId: typeof value.sourceId === 'string' ? value.sourceId : '',
    targetId: typeof value.targetId === 'string' ? value.targetId : '',
    property: typeof value.property === 'string' ? value.property : '',
  };
  if (Object.hasOwn(value, 'terminalId') && typeof value.terminalId === 'string') {
    result.terminalId = value.terminalId;
  }
  return Object.freeze(result);
}

function normalizeFacilityDependencyReport(report) {
  if (!report || typeof report !== 'object') return null;
  return Object.freeze({
    target: normalizeFacilityEntityReference(report.target),
    dependencies: Object.freeze(Array.isArray(report.dependencies)
      ? report.dependencies.map(normalizeFacilityDependency)
      : []),
  });
}

function facilityIDs(values) {
  return Object.freeze(Array.isArray(values) ? values.filter(value => typeof value === 'string') : []);
}

function normalizeFacilityOperationResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  return Object.freeze({
    ok,
    changed: value.changed === true,
    correlationId: typeof value.correlationId === 'string' ? value.correlationId : '',
    failure: facilityFailure(value.failure, ok),
    issues: Object.freeze(Array.isArray(value.issues) ? value.issues.map(normalizeFacilityIssue) : []),
    sessionRevision: nonnegativeSafeInteger(value.sessionRevision),
    previousFacilityRevision: nonnegativeSafeInteger(value.previousFacilityRevision),
    resultingFacilityRevision: nonnegativeSafeInteger(value.resultingFacilityRevision),
    affectedDeviceIds: facilityIDs(value.affectedDeviceIds),
    affectedConditionIds: facilityIDs(value.affectedConditionIds),
    session: normalizePortableSession(value.session),
  });
}

function normalizeFacilityAuthoringPayload(payload) {
  const value = payload && typeof payload === 'object' ? payload : {};
  return {
    session: value.session && typeof value.session === 'object'
      ? snapshotPortableSession(value.session)
      : null,
    expectedSessionRevision: nonnegativeSafeInteger(value.expectedSessionRevision),
    expectedFacilityRevision: nonnegativeSafeInteger(value.expectedFacilityRevision),
    correlationId: typeof value.correlationId === 'string' ? value.correlationId : '',
  };
}

function normalizeFacilityDependencyInspectionPayload(payload) {
  const value = payload && typeof payload === 'object' ? payload : {};
  return {
    target: normalizeFacilityEntityReference(value.target),
    expectedSessionRevision: nonnegativeSafeInteger(value.expectedSessionRevision),
    expectedFacilityRevision: nonnegativeSafeInteger(value.expectedFacilityRevision),
  };
}

function normalizeFacilityDependencyInspectionResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  return Object.freeze({
    ok,
    failure: facilityFailure(value.failure, ok),
    issues: Object.freeze(Array.isArray(value.issues) ? value.issues.map(normalizeFacilityIssue) : []),
    sessionRevision: nonnegativeSafeInteger(value.sessionRevision),
    facilityRevision: nonnegativeSafeInteger(value.facilityRevision),
    report: normalizeFacilityDependencyReport(value.report),
  });
}

function sameFacilityEntityReference(left, right) {
  if (left === null || right === null) return left === right;
  return left.kind === right.kind
    && left.entityId === right.entityId
    && Object.hasOwn(left, 'ownerId') === Object.hasOwn(right, 'ownerId')
    && left.ownerId === right.ownerId;
}

function facilityInspectionConflict(request) {
  return normalizeFacilityDependencyInspectionResult({
    ok: false,
    failure: 'conflict',
    issues: [{ code: 'conflict', entityKind: 'desktop-result' }],
    sessionRevision: request.expectedSessionRevision,
    facilityRevision: request.expectedFacilityRevision,
  });
}

function validateFacilityDependencyInspectionResult(result, request) {
  if (!result.ok) return result;
  const consistent = result.sessionRevision === request.expectedSessionRevision
    && result.facilityRevision === request.expectedFacilityRevision
    && result.report !== null
    && sameFacilityEntityReference(result.report.target, request.target);
  return consistent ? result : facilityInspectionConflict(request);
}

function normalizeFacilityPreviewPayload(payload) {
  const value = payload && typeof payload === 'object' ? payload : {};
  const result = {
    expectedFacilityRevision: nonnegativeSafeInteger(value.expectedFacilityRevision),
    terminalId: typeof value.terminalId === 'string' ? value.terminalId : '',
  };
  if (value.deviceState && typeof value.deviceState === 'object') {
    result.deviceState = {
      DeviceID: typeof value.deviceState.deviceId === 'string' ? value.deviceState.deviceId : '',
      StateID: typeof value.deviceState.stateId === 'string' ? value.deviceState.stateId : '',
    };
  }
  if (value.condition && typeof value.condition === 'object') {
    result.condition = {
      ConditionID: typeof value.condition.conditionId === 'string' ? value.condition.conditionId : '',
      Active: value.condition.active === true,
    };
  }
  return result;
}

function normalizeFacilityPreviewResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  return Object.freeze({
    ok,
    failure: facilityFailure(value.failure, ok),
    issues: Object.freeze(Array.isArray(value.issues) ? value.issues.map(normalizeFacilityIssue) : []),
    facilityRevision: nonnegativeSafeInteger(value.facilityRevision),
    terminal: value.terminal && typeof value.terminal === 'object'
      ? snapshotPortableSession(value.terminal)
      : null,
  });
}

function facilityPreviewConflict(request) {
  return normalizeFacilityPreviewResult({
    ok: false,
    failure: 'conflict',
    issues: [{ code: 'conflict', entityKind: 'desktop-result' }],
    facilityRevision: request.expectedFacilityRevision,
  });
}

function validateFacilityPreviewResult(result, request) {
  if (!result.ok) return result;
  const consistent = result.facilityRevision === request.expectedFacilityRevision
    && result.terminal !== null
    && result.terminal.terminalId === request.terminalId;
  return consistent ? result : facilityPreviewConflict(request);
}

function normalizeFacilityDeviceResetPayload(payload) {
  const value = payload && typeof payload === 'object' ? payload : {};
  return {
    deviceId: typeof value.deviceId === 'string' ? value.deviceId : '',
    expectedFacilityRevision: nonnegativeSafeInteger(value.expectedFacilityRevision),
    correlationId: typeof value.correlationId === 'string' ? value.correlationId : '',
  };
}

function normalizeFacilityResetPayload(payload) {
  const value = payload && typeof payload === 'object' ? payload : {};
  return {
    expectedFacilityRevision: nonnegativeSafeInteger(value.expectedFacilityRevision),
    correlationId: typeof value.correlationId === 'string' ? value.correlationId : '',
  };
}

function normalizeFacilityRecoveryPayload(payload) {
  const value = payload && typeof payload === 'object' ? payload : {};
  return {
    conditionId: typeof value.conditionId === 'string' ? value.conditionId : '',
    expectedFacilityRevision: nonnegativeSafeInteger(value.expectedFacilityRevision),
    correlationId: typeof value.correlationId === 'string' ? value.correlationId : '',
    recovery: value.recovery && typeof value.recovery === 'object'
      ? snapshotPortableSession(value.recovery)
      : null,
  };
}

function facilityOperationConflict(request) {
  return normalizeFacilityOperationResult({
    ok: false,
    changed: false,
    correlationId: request.correlationId,
    failure: 'conflict',
    issues: [{ code: 'conflict', entityKind: 'desktop-result' }],
    previousFacilityRevision: request.expectedFacilityRevision,
    resultingFacilityRevision: request.expectedFacilityRevision,
  });
}

function facilityIDsAreUnique(values) {
  return values.every(value => value !== '') && new Set(values).size === values.length;
}

function validateFacilityMutationResult(result, request, operation) {
  if (!result.ok) return result;
  const canonicalFacilityRevision = Number(result.session?.facility?.revision);
  const expectedResultingRevision = request.expectedFacilityRevision + (result.changed ? 1 : 0);
  let affectedIDsAreValid = facilityIDsAreUnique(result.affectedDeviceIds)
    && facilityIDsAreUnique(result.affectedConditionIds);
  if (!result.changed) {
    affectedIDsAreValid = affectedIDsAreValid
      && result.affectedDeviceIds.length === 0
      && result.affectedConditionIds.length === 0;
  } else if (operation === 'recovery') {
    affectedIDsAreValid = affectedIDsAreValid
      && result.affectedDeviceIds.length === 0
      && result.affectedConditionIds.length === 1
      && result.affectedConditionIds[0] === request.conditionId;
  } else if (operation === 'device-reset') {
    affectedIDsAreValid = affectedIDsAreValid
      && result.affectedDeviceIds.length === 1
      && result.affectedDeviceIds[0] === request.deviceId;
  } else {
    affectedIDsAreValid = affectedIDsAreValid
      && result.affectedDeviceIds.length + result.affectedConditionIds.length > 0;
  }
  const consistent = result.correlationId === request.correlationId
    && result.previousFacilityRevision === request.expectedFacilityRevision
    && result.resultingFacilityRevision === expectedResultingRevision
    && affectedIDsAreValid
    && (result.session === null || (Number.isSafeInteger(canonicalFacilityRevision)
      && canonicalFacilityRevision === result.resultingFacilityRevision))
    && (!result.changed || (result.session !== null
      && result.sessionRevision > 0
      && Number.isSafeInteger(canonicalFacilityRevision)));
  return consistent ? result : facilityOperationConflict(request);
}

function facilityBindingFailure(normalize) {
  return normalize({
    ok: false,
    failure: 'runtime-context-ended',
    issues: [{ code: 'runtime-context-ended', entityKind: 'desktop-binding' }],
  });
}

function validateFacilityAuthoringResult(result, request) {
  if (!result.ok) return result;
  const canonicalFacilityRevision = Number(result.session?.facility?.revision);
  const changedRevisionIsValid = result.changed
    ? result.resultingFacilityRevision === request.expectedFacilityRevision + 1
    : result.resultingFacilityRevision === request.expectedFacilityRevision;
  const sessionRevisionIsValid = result.changed
    ? result.sessionRevision === request.expectedSessionRevision + 1
    : result.sessionRevision === request.expectedSessionRevision;
  const consistent = result.correlationId === request.correlationId
    && result.previousFacilityRevision === request.expectedFacilityRevision
    && changedRevisionIsValid
    && sessionRevisionIsValid
    && (result.session === null || (Number.isSafeInteger(canonicalFacilityRevision)
      && canonicalFacilityRevision === result.resultingFacilityRevision))
    && (!result.changed || (result.session !== null
      && Number.isSafeInteger(canonicalFacilityRevision)));
  if (consistent) return result;
  return normalizeFacilityOperationResult({
    ok: false,
    changed: false,
    correlationId: request.correlationId,
    failure: 'conflict',
    issues: [{ code: 'conflict', entityKind: 'desktop-result' }],
    sessionRevision: request.expectedSessionRevision,
    previousFacilityRevision: request.expectedFacilityRevision,
    resultingFacilityRevision: request.expectedFacilityRevision,
  });
}

function saveFacilityAuthoringCommand(payload) {
  try {
    const request = normalizeFacilityAuthoringPayload(payload);
    return invoke(APP_METHODS.saveFacilityAuthoring, request)
      .then(normalizeFacilityOperationResult)
      .then(result => validateFacilityAuthoringResult(result, request))
      .catch(() => facilityBindingFailure(normalizeFacilityOperationResult));
  } catch {
    return Promise.resolve(facilityBindingFailure(normalizeFacilityOperationResult));
  }
}

function inspectFacilityDependenciesCommand(payload) {
  try {
    const request = normalizeFacilityDependencyInspectionPayload(payload);
    return invoke(APP_METHODS.inspectFacilityDependencies, request)
      .then(normalizeFacilityDependencyInspectionResult)
      .then(result => validateFacilityDependencyInspectionResult(result, request))
      .catch(() => facilityBindingFailure(normalizeFacilityDependencyInspectionResult));
  } catch {
    return Promise.resolve(facilityBindingFailure(normalizeFacilityDependencyInspectionResult));
  }
}

function previewFacilityCommand(payload) {
  try {
    const request = normalizeFacilityPreviewPayload(payload);
    return invoke(APP_METHODS.previewFacility, request)
      .then(normalizeFacilityPreviewResult)
      .then(result => validateFacilityPreviewResult(result, request))
      .catch(() => facilityBindingFailure(normalizeFacilityPreviewResult));
  } catch {
    return Promise.resolve(facilityBindingFailure(normalizeFacilityPreviewResult));
  }
}

function facilityMutationCommand(binding, payload, normalizePayload, operation) {
  try {
    const request = normalizePayload(payload);
    return invoke(binding, request)
      .then(normalizeFacilityOperationResult)
      .then(result => validateFacilityMutationResult(result, request, operation))
      .catch(() => facilityBindingFailure(normalizeFacilityOperationResult));
  } catch {
    return Promise.resolve(facilityBindingFailure(normalizeFacilityOperationResult));
  }
}

function normalizeAddCharacterPayload(payload) {
  const source = payload && typeof payload === 'object' ? payload : {};
  return {
    name: typeof source.name === 'string' ? source.name : '',
    intelligence: Number.isInteger(source.intelligence) ? source.intelligence : 0,
    hackerPerkAvailable: typeof source.hackerPerkAvailable === 'boolean'
      ? source.hackerPerkAvailable
      : undefined,
    expectedRevision: Number.isSafeInteger(source.expectedRevision) && source.expectedRevision >= 0
      ? source.expectedRevision
      : 0,
  };
}

function normalizeUpdateCharacterPayload(payload) {
  const source = payload && typeof payload === 'object' ? payload : {};
  return {
    characterId: typeof source.characterId === 'string' ? source.characterId : '',
    name: typeof source.name === 'string' ? source.name : '',
    intelligence: Number.isInteger(source.intelligence) ? source.intelligence : 0,
    hackerPerkAvailable: typeof source.hackerPerkAvailable === 'boolean'
      ? source.hackerPerkAvailable
      : undefined,
    expectedRevision: Number.isSafeInteger(source.expectedRevision) && source.expectedRevision >= 0
      ? source.expectedRevision
      : 0,
  };
}

function normalizeDeleteCharacterPayload(payload) {
  const source = payload && typeof payload === 'object' ? payload : {};
  return {
    characterId: typeof source.characterId === 'string' ? source.characterId : '',
    expectedRevision: Number.isSafeInteger(source.expectedRevision) && source.expectedRevision >= 0
      ? source.expectedRevision
      : 0,
  };
}

function normalizeSessionStateEvent(event) {
  const value = event && typeof event === 'object' ? event : {};
  return Object.freeze({
    revision: Number.isSafeInteger(value.revision) ? value.revision : 0,
    session: normalizePortableSession(value.session),
  });
}

const APPLICATION_UPDATE_STATES = new Set([
  'disabled',
  'idle',
  'checking',
  'current',
  'available',
  'deferred',
  'downloading',
  'verifying',
  'staging',
  'ready-to-restart',
  'applying',
  'failed',
]);
const APPLICATION_UPDATE_FAILURE_STAGES = new Set([
  'check',
  'download',
  'verify',
  'stage',
  'apply',
  'relaunch',
  'recovery',
]);
const APPLICATION_UPDATE_OFFER_DECISIONS = new Set(['accept', 'defer']);
const APPLICATION_UPDATE_RESTART_DECISIONS = new Set(['restart', 'postpone']);

function optionalString(value) {
  return typeof value === 'string' ? value : '';
}

function nonnegativeSafeInteger(value) {
  return Number.isSafeInteger(value) && value >= 0 ? value : 0;
}

function optionalNonnegativeSafeInteger(value) {
  return Number.isSafeInteger(value) && value >= 0 ? value : null;
}

function normalizeApplicationUpdateSnapshot(snapshot) {
  const value = snapshot && typeof snapshot === 'object' ? snapshot : {};
  const rawState = optionalString(value.state);
  const state = APPLICATION_UPDATE_STATES.has(rawState) ? rawState : '';
  const rawFailedStage = optionalString(value.failedStage);
  return Object.freeze({
    revision: nonnegativeSafeInteger(value.revision),
    attemptId: optionalString(value.attemptId),
    state,
    installedVersion: optionalString(value.installedVersion),
    availableVersion: optionalString(value.availableVersion),
    releaseNotes: optionalString(value.releaseNotes),
    bytesDownloaded: nonnegativeSafeInteger(value.bytesDownloaded),
    downloadSize: optionalNonnegativeSafeInteger(value.downloadSize),
    failedStage: APPLICATION_UPDATE_FAILURE_STAGES.has(rawFailedStage) ? rawFailedStage : '',
    errorMessage: optionalString(value.errorMessage),
    recoveryAction: optionalString(value.recoveryAction),
  });
}

let latestApplicationUpdateSnapshot = null;

function retainLatestApplicationUpdateSnapshot(snapshot) {
  const candidate = normalizeApplicationUpdateSnapshot(snapshot);
  if (!latestApplicationUpdateSnapshot
    || candidate.revision > latestApplicationUpdateSnapshot.revision) {
    latestApplicationUpdateSnapshot = candidate;
  }
  return latestApplicationUpdateSnapshot;
}

function normalizeApplicationUpdateCommandResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  let error = typeof value.error === 'string' ? value.error : '';
  if (ok) error = '';
  if (!ok && !error) error = 'Application update command failed';
  return Object.freeze({
    ok,
    error,
    snapshot: retainLatestApplicationUpdateSnapshot(value.snapshot),
  });
}

function applicationUpdateCommand(binding, payload) {
  return command(binding, payload).then(normalizeApplicationUpdateCommandResult);
}

function normalizeApplicationUpdateDecisionPayload(payload, decisions) {
  const value = payload && typeof payload === 'object' ? payload : {};
  const decision = optionalString(value.decision);
  return {
    attemptId: optionalString(value.attemptId),
    decision: decisions.has(decision) ? decision : '',
  };
}

function monotonicRevision(value) {
  const revision = Number(value?.revision);
  return Number.isSafeInteger(revision) && revision >= 0 ? revision : 0;
}

const PUBLIC_ACCESS_STATES = Object.freeze({
  disabled: 'stopped',
  starting: 'starting',
  ready: 'ready',
  stopping: 'stopping',
  failed: 'error',
  stopped: 'stopped',
  error: 'error',
});
const SECRET_PRESENCES = new Set(['absent', 'present', 'unknown']);

function normalizePublicAccessSnapshot(snapshot) {
  const value = snapshot && typeof snapshot === 'object' ? snapshot : {};
  const rawPreferences = value.preferences && typeof value.preferences === 'object' ? value.preferences : {};
  const rawStatus = value.status && typeof value.status === 'object' ? value.status : {};
  const preferences = Object.freeze({
    version: Number.isInteger(rawPreferences.version) ? rawPreferences.version : 1,
    enabledPreference: rawPreferences.enabledPreference === true,
    reservedDomain: typeof rawPreferences.reservedDomain === 'string' ? rawPreferences.reservedDomain : '',
    username: typeof rawPreferences.username === 'string' && rawPreferences.username ? rawPreferences.username : 'players',
    providerTokenPresentHint: rawPreferences.providerTokenPresentHint === true,
    playerPasswordPresentHint: rawPreferences.playerPasswordPresentHint === true,
    revision: Number.isSafeInteger(rawPreferences.revision) ? rawPreferences.revision : 0,
  });
  const state = PUBLIC_ACCESS_STATES[rawStatus.state] ?? '';
  const status = Object.freeze({
    state,
    generation: Number.isSafeInteger(rawStatus.generation) ? rawStatus.generation : 0,
    settingsRevision: Number.isSafeInteger(rawStatus.settingsRevision)
      ? rawStatus.settingsRevision
      : preferences.revision,
    publicUrl: state === 'ready' && typeof rawStatus.publicUrl === 'string' ? rawStatus.publicUrl : '',
    errorCategory: state === 'error' && typeof rawStatus.errorCategory === 'string' ? rawStatus.errorCategory : '',
    errorMessage: state === 'error' && typeof rawStatus.errorMessage === 'string' ? rawStatus.errorMessage : '',
  });
  const presence = (candidate) => SECRET_PRESENCES.has(candidate) ? candidate : 'unknown';
  return Object.freeze({
    preferences,
    providerTokenPresence: presence(value.providerTokenPresence),
    playerPasswordPresence: presence(value.playerPasswordPresence),
    status,
  });
}

function normalizePublicAccessCommandResult(result) {
  const value = result && typeof result === 'object' ? result : {};
  const ok = value.ok === true;
  let error = typeof value.error === 'string' ? value.error : '';
  if (!ok && !error) error = 'Public access command failed';
  return Object.freeze({ ok, error, snapshot: normalizePublicAccessSnapshot(value.snapshot) });
}

let latestPublicAccessSnapshot = null;

function retainLatestPublicAccessSnapshot(snapshot) {
  if (!latestPublicAccessSnapshot) {
    latestPublicAccessSnapshot = snapshot;
    return snapshot;
  }
  const candidate = publicAccessVersion(snapshot);
  const latest = publicAccessVersion(latestPublicAccessSnapshot);
  if (versionIsNewer(candidate, latest) || (candidate[0] === latest[0] && candidate[1] === latest[1])) {
    latestPublicAccessSnapshot = snapshot;
  }
  return latestPublicAccessSnapshot;
}

function publicAccessCommand(binding, payload) {
  return command(binding, payload).then((value) => {
    const result = normalizePublicAccessCommandResult(value);
    return Object.freeze({
      ok: result.ok,
      error: result.error,
      snapshot: retainLatestPublicAccessSnapshot(result.snapshot),
    });
  });
}

function publicAccessVersion(snapshot) {
  return [snapshot.status.generation, snapshot.status.settingsRevision];
}

function versionIsNewer(candidate, baseline) {
  return candidate[0] > baseline[0] || (candidate[0] === baseline[0] && candidate[1] > baseline[1]);
}

function clearSecretMutationFields(value) {
  if (!value || typeof value !== 'object') return;
  if (Object.hasOwn(value, 'replacementProviderToken')) value.replacementProviderToken = '';
  if (Object.hasOwn(value, 'replacementPlayerPassword')) value.replacementPlayerPassword = '';
}

let latestServerInfo = null;

function normalizeServerInfo(payload) {
  if (!payload || typeof payload !== 'object') return null;
  const url = typeof payload.url === 'string' ? payload.url : '';
  const tunnel = Boolean(payload.tunnel);
  const previousLocalUrl = latestServerInfo?.localUrl
    || (latestServerInfo && !latestServerInfo.tunnel ? latestServerInfo.url : '');
  const suppliedLocalUrl = typeof payload.localUrl === 'string' ? payload.localUrl : '';
  latestServerInfo = Object.freeze({
    ip: typeof payload.ip === 'string' ? payload.ip : '',
    port: Number.isInteger(payload.port) ? payload.port : 0,
    url,
    localUrl: suppliedLocalUrl || (!tunnel ? url : previousLocalUrl),
    tunnel,
    tunnelError: typeof payload.tunnelError === 'string' ? payload.tunnelError : '',
  });
  return latestServerInfo;
}

function unwrapEvent(event) {
  return event && typeof event === 'object' && Object.hasOwn(event, 'data') ? event.data : event;
}

let runtimeStatusPromise = null;

function beginStatusSnapshotWhenReady() {
  if (runtimeStatusPromise || !requiredEvents.every(([name]) => eventSubscriptions.has(name))) return;
  runtimeStatusPromise = command(APP_METHODS.getRuntimeStatus);
  void runtimeStatusPromise.then((status) => {
    if (!status || status.ok === false) return;
    for (const [eventName, field] of requiredEvents) {
      for (const subscription of eventSubscriptions.get(eventName) ?? []) {
        if (!subscription.active || (subscription.eventReceived && !subscription.revisionOf)) continue;
        subscription.deliver(status[field]);
      }
    }
  });
}

function subscribe(eventName, statusField, callback, project = (payload) => payload, revisionOf = null) {
  if (typeof callback !== 'function') throw new TypeError(`${eventName} listener must be a function`);

  const bucket = eventSubscriptions.get(eventName) ?? new Set();
  eventSubscriptions.set(eventName, bucket);
  const subscription = {
    active: true,
    eventReceived: false,
    released: false,
    revisionOf,
    latestRevision: null,
    deliver(payload) {
      if (!this.active) return;
      const projected = project(payload);
      if (statusField === 'serverInfo' && projected == null) return;
      if (this.revisionOf) {
        const revision = this.revisionOf(projected);
        if (this.latestRevision != null && revision <= this.latestRevision) return;
        this.latestRevision = revision;
      }
      callback(projected);
    },
    releaseRuntime: () => {},
  };
  subscription.releaseRuntime = Events.On(eventName, (event) => {
    if (!subscription.active) return;
    subscription.eventReceived = true;
    subscription.deliver(unwrapEvent(event));
  });
  bucket.add(subscription);

  const unsubscribe = () => {
    if (!subscription.active) return;
    subscription.active = false;
    bucket.delete(subscription);
    subscriptions.delete(unsubscribe);
    if (!subscription.released) {
      subscription.released = true;
      subscription.releaseRuntime();
    }
  };
  subscriptions.add(unsubscribe);
  beginStatusSnapshotWhenReady();
  return unsubscribe;
}

const previousFacade = window.desktopAPI;
if (typeof previousFacade?.[DISPOSE] === 'function') previousFacade[DISPOSE]();

const desktopAPI = {
  onServerInfo: (callback) => subscribe('server-info', 'serverInfo', callback, normalizeServerInfo),
  onClientCount: (callback) => subscribe('client-count', 'clientCount', callback),
  onHackState: (callback) => subscribe('hack-state', 'hackState', callback),
  onCoordinationState: (callback) => subscribe(
    'coordination-state', 'coordinationState', callback, (payload) => payload, monotonicRevision,
  ),
  onSessionState: (callback) => subscribe('session-state', null, callback, normalizeSessionStateEvent),
  onPublicAccessStatus: (callback) => {
    if (typeof callback !== 'function') throw new TypeError('public-access-status listener must be a function');
    let active = true;
    let released = false;
    let latestEventVersion = [-1, -1];
    const releaseRuntime = Events.On('public-access-status', (event) => {
      if (!active) return;
      const snapshot = normalizePublicAccessSnapshot(unwrapEvent(event));
      const candidate = publicAccessVersion(snapshot);
      if (!versionIsNewer(candidate, latestEventVersion)) return;
      latestEventVersion = candidate;
      callback(retainLatestPublicAccessSnapshot(snapshot));
    });
    void command(APP_METHODS.getPublicAccess).then((value) => {
      if (!active || value?.ok === false) return;
      const snapshot = normalizePublicAccessSnapshot(value);
      const candidate = publicAccessVersion(snapshot);
      if (latestEventVersion[0] >= 0 && !versionIsNewer(candidate, latestEventVersion)) return;
      callback(retainLatestPublicAccessSnapshot(snapshot));
    });
    const unsubscribe = () => {
      if (!active) return;
      active = false;
      subscriptions.delete(unsubscribe);
      if (!released) {
        released = true;
        releaseRuntime();
      }
    };
    subscriptions.add(unsubscribe);
    return unsubscribe;
  },
  onApplicationUpdateStatus: (callback) => {
    if (typeof callback !== 'function') {
      throw new TypeError('application-update-status listener must be a function');
    }
    let active = true;
    let released = false;
    let latestDeliveredRevision = -1;
    const deliver = (value) => {
      if (!active) return;
      const snapshot = retainLatestApplicationUpdateSnapshot(value);
      if (snapshot.revision <= latestDeliveredRevision) return;
      latestDeliveredRevision = snapshot.revision;
      callback(snapshot);
    };
    const releaseRuntime = Events.On('application-update-status', (event) => {
      deliver(unwrapEvent(event));
    });
    void command(APP_METHODS.getApplicationUpdateStatus).then(deliver);
    const unsubscribe = () => {
      if (!active) return;
      active = false;
      subscriptions.delete(unsubscribe);
      if (!released) {
        released = true;
        releaseRuntime();
      }
    };
    subscriptions.add(unsubscribe);
    return unsubscribe;
  },
  getRuntimeStatus: () => {
    beginStatusSnapshotWhenReady();
    return runtimeStatusPromise ?? command(APP_METHODS.getRuntimeStatus);
  },
  openUrl: (url) => command(APP_METHODS.openUrl, url),
  openLogLocation: () => command(APP_METHODS.openLogLocation),
  writeClipboardText,
  openSession: () => command(APP_METHODS.openSession).then(normalizeSessionResult),
  newSession: () => command(APP_METHODS.newSession).then(normalizeSessionResult),
  saveSession: saveSessionCommand,
  loadReferencedPlayerConfig: () => playerConfigCommand(APP_METHODS.loadReferencedPlayerConfig),
  newPlayerConfig: () => playerConfigCommand(APP_METHODS.newPlayerConfig),
  openPlayerConfig: () => playerConfigCommand(APP_METHODS.openPlayerConfig),
  requestTerminalActivation: (payload) => switchCommand(APP_METHODS.requestTerminalActivation, payload),
  updateLiveTerminal: (payload) => command(APP_METHODS.updateLiveTerminal, payload),
  requestTerminalClear: () => switchCommand(APP_METHODS.requestTerminalClear),
  resolveTerminalSwitch: (payload) => switchCommand(APP_METHODS.resolveTerminalSwitch, payload),
  resolveCommandExecution: (payload) => command(APP_METHODS.resolveCommandExecution, {
    requestId: typeof payload?.requestId === 'string' ? payload.requestId : '',
    decision: payload?.decision === 'approve' || payload?.decision === 'reject'
      ? payload.decision
      : '',
  }).then(normalizeCommandExecutionResult),
  resolveTerminalNavigation: (payload) => command(APP_METHODS.resolveTerminalNavigation, {
    requestId: typeof payload?.requestId === 'string' ? payload.requestId : '',
    decision: payload?.decision === 'approve' || payload?.decision === 'reject' ? payload.decision : '',
  }).then(normalizeTerminalNavigationResult),
  forceHackSuccess: () => command(APP_METHODS.forceHackSuccess),
  resetFailedHack: (payload) => command(APP_METHODS.resetFailedHack, payload),
  resetCommandState: (payload) => sessionStateCommand(APP_METHODS.resetCommandState, {
    terminalId: typeof payload?.terminalId === 'string' ? payload.terminalId : '',
    commandId: typeof payload?.commandId === 'string' ? payload.commandId : '',
  }),
  resetTerminalCommandStates: (payload) => sessionStateCommand(APP_METHODS.resetTerminalCommandStates, {
    terminalId: typeof payload?.terminalId === 'string' ? payload.terminalId : '',
  }),
  replaceTerminalGroups: replaceTerminalGroupsCommand,
  inspectFacilityDependencies: inspectFacilityDependenciesCommand,
  previewFacility: previewFacilityCommand,
  recoverFacilityCondition: payload => facilityMutationCommand(
    APP_METHODS.recoverFacilityCondition,
    payload,
    normalizeFacilityRecoveryPayload,
    'recovery',
  ),
  resetFacility: payload => facilityMutationCommand(
    APP_METHODS.resetFacility,
    payload,
    normalizeFacilityResetPayload,
    'facility-reset',
  ),
  resetFacilityDevice: payload => facilityMutationCommand(
    APP_METHODS.resetFacilityDevice,
    payload,
    normalizeFacilityDeviceResetPayload,
    'device-reset',
  ),
  saveFacilityAuthoring: saveFacilityAuthoringCommand,
  addCharacter: (payload) => command(APP_METHODS.addCharacter, normalizeAddCharacterPayload(payload)),
  updateCharacter: (payload) => command(APP_METHODS.updateCharacter, normalizeUpdateCharacterPayload(payload)),
  deleteCharacter: (payload) => command(APP_METHODS.deleteCharacter, normalizeDeleteCharacterPayload(payload)),
  renameLogicalSession: (payload) => command(APP_METHODS.renameLogicalSession, payload),
  assignCharacter: (payload) => command(APP_METHODS.assignCharacter, payload),
  releaseCharacter: (sessionId) => command(APP_METHODS.releaseCharacter, sessionId),
  moveCharacter: (payload) => command(APP_METHODS.moveCharacter, payload),
  setActiveController: (sessionId) => command(APP_METHODS.setActiveController, sessionId),
  startBroadcast: () => command(APP_METHODS.startBroadcast),
  endBroadcast: () => command(APP_METHODS.endBroadcast),
  getPublicAccess: () => command(APP_METHODS.getPublicAccess)
    .then(normalizePublicAccessSnapshot)
    .then(retainLatestPublicAccessSnapshot),
  copyPublicAccessCredentials: () => command(APP_METHODS.copyPublicAccessCredentials),
  savePublicAccessSettings: (request) => {
    const source = request && typeof request === 'object' ? request : {};
    const nativeRequest = {
      expectedRevision: Number.isSafeInteger(source.expectedRevision) ? source.expectedRevision : 0,
      enabledPreference: source.enabledPreference === true,
      reservedDomain: typeof source.reservedDomain === 'string' ? source.reservedDomain : '',
      username: typeof source.username === 'string' ? source.username : '',
      replacementProviderToken: typeof source.replacementProviderToken === 'string' ? source.replacementProviderToken : '',
      deleteProviderToken: source.deleteProviderToken === true,
      replacementPlayerPassword: typeof source.replacementPlayerPassword === 'string' ? source.replacementPlayerPassword : '',
      deletePlayerPassword: source.deletePlayerPassword === true,
    };
    const pending = publicAccessCommand(APP_METHODS.savePublicAccessSettings, nativeRequest);
    clearSecretMutationFields(nativeRequest);
    clearSecretMutationFields(source);
    return pending;
  },
  generatePlayerPassword: (request) => command(APP_METHODS.generatePlayerPassword, {
    expectedRevision: Number.isSafeInteger(request?.expectedRevision) ? request.expectedRevision : 0,
  }).then((result) => {
    const value = result && typeof result === 'object' ? result : {};
    return {
      ok: value.ok === true,
      error: typeof value.error === 'string' ? value.error : '',
      generatedPassword: typeof value.generatedPassword === 'string' ? value.generatedPassword : '',
      settingsRevision: Number.isSafeInteger(value.settingsRevision) ? value.settingsRevision : 0,
    };
  }),
  startPublicAccess: (request) => publicAccessCommand(APP_METHODS.startPublicAccess, {
    expectedRevision: Number.isSafeInteger(request?.expectedRevision) ? request.expectedRevision : 0,
  }),
  stopPublicAccess: (request) => publicAccessCommand(APP_METHODS.stopPublicAccess, {
    expectedRevision: Number.isSafeInteger(request?.expectedRevision) ? request.expectedRevision : 0,
  }),
  resolveApplicationUpdateOffer: (payload) => applicationUpdateCommand(
    APP_METHODS.resolveApplicationUpdateOffer,
    normalizeApplicationUpdateDecisionPayload(payload, APPLICATION_UPDATE_OFFER_DECISIONS),
  ),
  resolveApplicationUpdateRestart: (payload) => applicationUpdateCommand(
    APP_METHODS.resolveApplicationUpdateRestart,
    normalizeApplicationUpdateDecisionPayload(payload, APPLICATION_UPDATE_RESTART_DECISIONS),
  ),
};

Object.defineProperty(desktopAPI, DISPOSE, {
  value: () => {
    for (const unsubscribe of [...subscriptions]) unsubscribe();
  },
});

Object.defineProperty(window, 'desktopAPI', {
  value: Object.freeze(desktopAPI),
  configurable: true,
});
