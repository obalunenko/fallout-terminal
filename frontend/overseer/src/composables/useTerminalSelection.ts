import { inject, nextTick, onUnmounted, readonly, ref, shallowRef } from 'vue';

import { overseerCoexistenceBridgeKey } from '../mount.js';
import type { DesktopRecord } from '../models/overseer-view-state.js';

export interface TerminalSelectionRow {
  readonly groupID: string;
  readonly groupName: string;
  readonly id: string;
  readonly live: boolean;
  readonly memberCount: number;
  readonly memberIndex: number;
  readonly name: string;
  readonly revision: number;
  readonly selected: boolean;
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function rows(value: unknown): readonly TerminalSelectionRow[] | null {
  if (!Array.isArray(value)) return null;
  const projected: TerminalSelectionRow[] = [];
  for (const candidate of value) {
    if (!isRecord(candidate) || typeof candidate.id !== 'string' || candidate.id === ''
      || typeof candidate.name !== 'string' || typeof candidate.groupID !== 'string'
      || typeof candidate.groupName !== 'string'
      || !Number.isSafeInteger(candidate.memberCount) || Number(candidate.memberCount) < 1
      || !Number.isSafeInteger(candidate.memberIndex) || Number(candidate.memberIndex) < 0
      || Number(candidate.memberIndex) >= Number(candidate.memberCount)
      || !Number.isSafeInteger(candidate.revision) || Number(candidate.revision) < 0) return null;
    projected.push(Object.freeze({
      groupID: candidate.groupID,
      groupName: candidate.groupName,
      id: candidate.id,
      live: candidate.live === true,
      memberCount: Number(candidate.memberCount),
      memberIndex: Number(candidate.memberIndex),
      name: candidate.name,
      revision: Number(candidate.revision),
      selected: candidate.selected === true,
    }));
  }
  return Object.freeze(projected);
}

export function useTerminalSelection() {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const revision = ref(-1);
  const terminals = shallowRef<readonly TerminalSelectionRow[]>(Object.freeze([]));
  const focusRequest = shallowRef<Readonly<{ ownerID: string; scope: string }> | null>(null);

  const release = bridge?.subscribeLegacyState(message => {
    if (message.kind === 'terminal-selection-focus-request') {
      if (typeof message.ownerID !== 'string' || typeof message.scope !== 'string') return;
      focusRequest.value = Object.freeze({ ownerID: message.ownerID, scope: message.scope });
      void nextTick(() => {
        focusRequest.value = null;
      });
      return;
    }
    if (message.kind !== 'terminal-selection-snapshot'
      || !Number.isSafeInteger(message.revision) || Number(message.revision) <= revision.value) return;
    const next = rows(message.terminals);
    if (next === null) return;
    revision.value = Number(message.revision);
    terminals.value = next;
  });

  function select(terminalID: string): void {
    bridge?.vueToLegacy({ kind: 'terminal-selection-request', revision: revision.value, terminalID });
  }

  onUnmounted(() => {
    release?.();
    terminals.value = Object.freeze([]);
    focusRequest.value = null;
  });

  return {
    focusRequest: readonly(focusRequest),
    revision: readonly(revision),
    terminals: readonly(terminals),
    select,
  };
}
