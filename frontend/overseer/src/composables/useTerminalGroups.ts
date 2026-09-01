import { inject, onUnmounted, readonly, ref, shallowRef } from 'vue';

import { overseerControllerKey } from '../controllers/overseer-controller.js';
import type { DesktopRecord } from '../models/overseer-view-state.js';

export type TerminalGroupAction =
  | 'dissolve-terminal-group'
  | 'move-terminal-group-down'
  | 'move-terminal-group-up'
  | 'rename-terminal-group';

export interface TerminalGroupRow {
  readonly id: string;
  readonly name: string;
  readonly terminalIDs: readonly string[];
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseGroups(value: unknown): readonly TerminalGroupRow[] | null {
  if (!Array.isArray(value)) return null;
  const groupIDs = new Set<string>();
  const terminalIDs = new Set<string>();
  const parsed: TerminalGroupRow[] = [];
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
    parsed.push(Object.freeze({
      id: candidate.id,
      name: candidate.name,
      terminalIDs: Object.freeze(members),
    }));
  }
  return Object.freeze(parsed);
}

export function useTerminalGroups() {
  const controller = inject(overseerControllerKey, null);
  const collapsedGroupIDs = shallowRef<ReadonlySet<string>>(new Set());
  const groups = shallowRef<readonly TerminalGroupRow[]>(Object.freeze([]));
  const revision = ref(-1);

  const release = controller?.subscribeState(message => {
    if (message.kind !== 'terminal-groups-snapshot'
      || !Number.isSafeInteger(message.revision) || Number(message.revision) <= revision.value) return;
    const next = parseGroups(message.groups);
    if (next === null) return;
    const retained = new Set(next.filter(group => collapsedGroupIDs.value.has(group.id)).map(group => group.id));
    collapsedGroupIDs.value = retained;
    groups.value = next;
    revision.value = Number(message.revision);
  });

  function action(groupID: string, requestedAction: TerminalGroupAction): void {
    controller?.dispatch({
      action: requestedAction,
      groupID,
      kind: 'terminal-group-action-request',
      revision: revision.value,
    });
  }

  function toggle(groupID: string): void {
    if (!groups.value.some(group => group.id === groupID)) return;
    const next = new Set(collapsedGroupIDs.value);
    if (next.has(groupID)) next.delete(groupID);
    else next.add(groupID);
    collapsedGroupIDs.value = next;
  }

  onUnmounted(() => {
    release?.();
    collapsedGroupIDs.value = new Set();
    groups.value = Object.freeze([]);
  });

  return {
    action,
    collapsedGroupIDs: readonly(collapsedGroupIDs),
    groups: readonly(groups),
    revision: readonly(revision),
    toggle,
  };
}
