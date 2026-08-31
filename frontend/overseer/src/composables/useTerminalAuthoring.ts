import { inject, nextTick, onUnmounted, provide, readonly, ref, shallowRef } from 'vue';

import {
  overseerCoexistenceBridgeKey,
  type OverseerCoexistenceBridge,
  type OverseerCoexistenceMessage,
} from '../mount.js';
import type { DesktopRecord } from '../models/overseer-view-state.js';
import type { TerminalGroupAction, TerminalGroupRow } from './useTerminalGroups.js';
import type { TerminalSelectionRow } from './useTerminalSelection.js';

interface AuthoringSnapshot {
  readonly create: DesktopRecord;
  readonly editor: DesktopRecord;
  readonly groups: readonly TerminalGroupRow[];
  readonly revision: number;
  readonly terminals: readonly TerminalSelectionRow[];
  readonly tree: DesktopRecord;
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseGroups(value: unknown): readonly TerminalGroupRow[] | null {
  if (!Array.isArray(value)) return null;
  const groupIDs = new Set<string>();
  const terminalIDs = new Set<string>();
  const groups: TerminalGroupRow[] = [];
  for (const candidate of value) {
    if (!isRecord(candidate) || typeof candidate.id !== 'string' || candidate.id === ''
      || typeof candidate.name !== 'string' || !Array.isArray(candidate.terminalIDs)
      || candidate.terminalIDs.length === 0 || groupIDs.has(candidate.id)) return null;
    const members: string[] = [];
    for (const terminalID of candidate.terminalIDs) {
      if (typeof terminalID !== 'string' || terminalID === '' || terminalIDs.has(terminalID)) return null;
      terminalIDs.add(terminalID);
      members.push(terminalID);
    }
    groupIDs.add(candidate.id);
    groups.push(Object.freeze({ id: candidate.id, name: candidate.name, terminalIDs: Object.freeze(members) }));
  }
  return Object.freeze(groups);
}

function parseTerminals(value: unknown, revision: number): readonly TerminalSelectionRow[] | null {
  if (!Array.isArray(value)) return null;
  const terminalIDs = new Set<string>();
  const terminals: TerminalSelectionRow[] = [];
  for (const candidate of value) {
    if (!isRecord(candidate) || typeof candidate.id !== 'string' || candidate.id === ''
      || terminalIDs.has(candidate.id) || typeof candidate.name !== 'string'
      || typeof candidate.groupID !== 'string' || typeof candidate.groupName !== 'string'
      || !Number.isSafeInteger(candidate.memberCount) || Number(candidate.memberCount) < 1
      || !Number.isSafeInteger(candidate.memberIndex) || Number(candidate.memberIndex) < 0
      || Number(candidate.memberIndex) >= Number(candidate.memberCount)) return null;
    terminalIDs.add(candidate.id);
    terminals.push(Object.freeze({
      groupID: candidate.groupID,
      groupName: candidate.groupName,
      id: candidate.id,
      live: candidate.live === true,
      memberCount: Number(candidate.memberCount),
      memberIndex: Number(candidate.memberIndex),
      name: candidate.name,
      revision,
      selected: candidate.selected === true,
    }));
  }
  return Object.freeze(terminals);
}

function parseSnapshot(message: OverseerCoexistenceMessage): AuthoringSnapshot | null {
  if (message.kind !== 'terminal-authoring-snapshot' || !Number.isSafeInteger(message.revision)
    || !isRecord(message.editor) || !isRecord(message.tree) || !isRecord(message.create)) return null;
  const revision = Number(message.revision);
  const groups = parseGroups(message.groups);
  const terminals = parseTerminals(message.terminals, revision);
  if (groups === null || terminals === null) return null;
  const groupedTerminalIDs = groups.flatMap(group => group.terminalIDs);
  if (groupedTerminalIDs.length !== terminals.length
    || groupedTerminalIDs.some((terminalID, index) => terminalID !== terminals[index]?.id)) return null;
  return Object.freeze({
    create: message.create,
    editor: message.editor,
    groups,
    revision,
    terminals,
    tree: message.tree,
  });
}

function childMessage(kind: string, revision: number, value: DesktopRecord): OverseerCoexistenceMessage {
  return Object.freeze({ ...value, kind, revision });
}

function isAuthoringRequest(message: unknown): message is OverseerCoexistenceMessage {
  if (!isRecord(message) || typeof message.kind !== 'string') return false;
  return message.kind === 'terminal-action-request'
    || message.kind === 'terminal-editor-action-request'
    || message.kind === 'terminal-group-action-request'
    || message.kind === 'terminal-selection-request'
    || message.kind === 'terminal-tree-action-request'
    || message.kind === 'create-terminal-action-request';
}

export function useTerminalAuthoring() {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const collapsedGroupIDs = shallowRef<ReadonlySet<string>>(new Set());
  const groups = shallowRef<readonly TerminalGroupRow[]>(Object.freeze([]));
  const terminals = shallowRef<readonly TerminalSelectionRow[]>(Object.freeze([]));
  const focusRequest = shallowRef<Readonly<{ ownerID: string; scope: string }> | null>(null);
  const revision = ref(-1);
  const childListeners = new Set<(message: OverseerCoexistenceMessage) => void>();

  function emitToChildren(message: OverseerCoexistenceMessage): void {
    for (const listener of childListeners) listener(message);
  }

  const release = bridge?.subscribeLegacyState(message => {
    if (message.kind === 'terminal-selection-focus-request') {
      if (typeof message.ownerID === 'string' && typeof message.scope === 'string') {
        focusRequest.value = Object.freeze({ ownerID: message.ownerID, scope: message.scope });
        void nextTick(() => { focusRequest.value = null; });
      }
      return;
    }
    const next = parseSnapshot(message);
    if (next !== null) {
      if (next.revision <= revision.value) return;
      const retained = new Set(next.groups
        .filter(group => collapsedGroupIDs.value.has(group.id))
        .map(group => group.id));
      collapsedGroupIDs.value = retained;
      groups.value = next.groups;
      terminals.value = next.terminals;
      revision.value = next.revision;
      emitToChildren(childMessage('terminal-editor-snapshot', next.revision, next.editor));
      emitToChildren(childMessage('terminal-tree-snapshot', next.revision, next.tree));
      emitToChildren(childMessage('create-terminal-snapshot', next.revision, next.create));
      return;
    }
    emitToChildren(message);
  }) ?? (() => {});

  const authoringBridge: OverseerCoexistenceBridge | null = bridge === null ? null : Object.freeze({
    legacyToVue: bridge.legacyToVue,
    subscribeLegacyState(listener: (message: OverseerCoexistenceMessage) => void) {
      childListeners.add(listener);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        childListeners.delete(listener);
      };
    },
    subscribeVueRequests: bridge.subscribeVueRequests,
    vueToLegacy(value: unknown) {
      if (isAuthoringRequest(value)
        && (!Number.isSafeInteger(value.revision) || Number(value.revision) !== revision.value)) return false;
      return bridge.vueToLegacy(value);
    },
  });
  if (authoringBridge !== null) provide(overseerCoexistenceBridgeKey, authoringBridge);

  function request(kind: string, extra: DesktopRecord): void {
    bridge?.vueToLegacy({ ...extra, kind, revision: revision.value });
  }

  function toggle(groupID: string): void {
    if (!groups.value.some(group => group.id === groupID)) return;
    const next = new Set(collapsedGroupIDs.value);
    if (next.has(groupID)) next.delete(groupID);
    else next.add(groupID);
    collapsedGroupIDs.value = next;
  }

  onUnmounted(() => {
    release();
    childListeners.clear();
    collapsedGroupIDs.value = new Set();
    groups.value = Object.freeze([]);
    terminals.value = Object.freeze([]);
    focusRequest.value = null;
  });

  return {
    action: (groupID: string, action: TerminalGroupAction) => request('terminal-group-action-request', { action, groupID }),
    collapsedGroupIDs: readonly(collapsedGroupIDs),
    focusRequest: readonly(focusRequest),
    groups: readonly(groups),
    revision: readonly(revision),
    select: (terminalID: string) => request('terminal-selection-request', { terminalID }),
    terminals: readonly(terminals),
    toggle,
  };
}
