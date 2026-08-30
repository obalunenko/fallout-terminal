import { inject, onUnmounted, readonly, ref } from 'vue';

import { overseerCoexistenceBridgeKey, type OverseerCoexistenceMessage } from '../mount.js';
import type { DesktopCommandResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

export interface PlayerProfile {
  readonly hackerPerkAvailable: boolean;
  readonly id: string;
  readonly intelligence: number;
  readonly name: string;
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function playerProfile(value: unknown): PlayerProfile | null {
  if (!isRecord(value) || typeof value.id !== 'string' || typeof value.name !== 'string'
    || !Number.isInteger(value.intelligence) || Number(value.intelligence) < 1
    || Number(value.intelligence) > 10 || typeof value.hackerPerkAvailable !== 'boolean') return null;
  return Object.freeze({
    hackerPerkAvailable: value.hackerPerkAvailable,
    id: value.id,
    intelligence: Number(value.intelligence),
    name: value.name,
  });
}

function profiles(value: unknown): readonly PlayerProfile[] {
  if (!Array.isArray(value)) return Object.freeze([]);
  const parsedProfiles: PlayerProfile[] = [];
  for (const item of value) {
    const parsed = playerProfile(item);
    if (parsed !== null) parsedProfiles.push(parsed);
  }
  return Object.freeze(parsedProfiles);
}

export function usePlayerManagement(port: DesktopPort) {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const addResetRequest = ref(0);
  const deleteFocusCharacterId = ref('');
  const deleteFocusRequest = ref(0);
  const error = ref('');
  const open = ref(false);
  const pending = ref(false);
  const playerConfigActive = ref(false);
  const profileList = ref<readonly PlayerProfile[]>(Object.freeze([]));
  const readOnly = ref(false);
  const revision = ref(0);
  const status = ref('');
  let active = true;
  let lifecycle = 0;

  function applyCoordination(coordination: DesktopRecord, externalPending: boolean): void {
    const nextRevision = Number(coordination.revision ?? 0);
    if (!Number.isSafeInteger(nextRevision) || nextRevision < revision.value) return;
    revision.value = nextRevision;
    playerConfigActive.value = isRecord(coordination.playerConfig);
    readOnly.value = isRecord(coordination.broadcast) || !playerConfigActive.value;
    profileList.value = profiles(coordination.roster);
    pending.value = externalPending;
  }

  function handleLegacyMessage(message: OverseerCoexistenceMessage): void {
    if (message.kind === 'player-management-open-request') {
      if (!playerConfigActive.value) return;
      lifecycle += 1;
      error.value = '';
      status.value = '';
      open.value = true;
      return;
    }
    if (message.kind === 'player-management-delete-focus-request'
      && typeof message.characterId === 'string') {
      deleteFocusCharacterId.value = message.characterId;
      deleteFocusRequest.value += 1;
      return;
    }
    if (message.kind === 'player-management-feedback') {
      status.value = typeof message.status === 'string' ? message.status : '';
      error.value = typeof message.error === 'string' ? message.error : '';
      return;
    }
    if (message.kind !== 'coordination-state') return;
    if (!isRecord(message.coordination)) {
      revision.value = 0;
      playerConfigActive.value = false;
      profileList.value = Object.freeze([]);
      readOnly.value = true;
      pending.value = message.pending === true;
      return;
    }
    applyCoordination(message.coordination, message.pending === true);
  }

  async function run(
    successMessage: string,
    pendingMessage: string,
    command: () => Promise<DesktopCommandResult>,
  ): Promise<boolean> {
    if (!open.value || pending.value || readOnly.value) return false;
    const expectedRevision = revision.value;
    const expectedLifecycle = lifecycle;
    pending.value = true;
    error.value = '';
    status.value = pendingMessage;
    bridge?.vueToLegacy({ expectedRevision, kind: 'player-management-command-started', status: pendingMessage });
    let result: DesktopCommandResult;
    try {
      result = await command();
    } catch (cause) {
      result = { error: cause instanceof Error ? cause.message : String(cause), ok: false };
    }
    if (result.state !== undefined && isRecord(result.state)) applyCoordination(result.state, false);
    const current = active && open.value && lifecycle === expectedLifecycle
      && revision.value >= expectedRevision;
    pending.value = false;
    if (current) {
      status.value = result.ok ? successMessage : '';
      error.value = result.ok ? '' : (result.error || 'ОПЕРАЦИЯ СО СПИСКОМ ИГРОКОВ ОТКЛОНЕНА');
    }
    bridge?.vueToLegacy({ expectedRevision, kind: 'player-management-command-finished', result, successMessage });
    return current && result.ok;
  }

  function add(name: string, intelligence: number, hackerPerkAvailable: boolean): void {
    if (name === '' || !Number.isInteger(intelligence) || intelligence < 1 || intelligence > 10) return;
    void run('ИГРОК ДОБАВЛЕН', 'ДОБАВЛЕНИЕ ИГРОКА...', () => port.addCharacter({
      name,
      intelligence,
      hackerPerkAvailable,
      expectedRevision: revision.value,
    })).then(succeeded => { if (succeeded) addResetRequest.value += 1; });
  }

  function save(characterId: string, name: string, intelligence: number, hackerPerkAvailable: boolean): void {
    if (!profileList.value.some(profile => profile.id === characterId)) return;
    void run('ПРОФИЛЬ ИГРОКА СОХРАНЁН', 'СОХРАНЕНИЕ ПРОФИЛЯ ИГРОКА...', () => port.updateCharacter({
      characterId,
      name,
      intelligence,
      hackerPerkAvailable,
      expectedRevision: revision.value,
    }));
  }

  function close(): void {
    if (!open.value) return;
    lifecycle += 1;
    open.value = false;
    bridge?.vueToLegacy({ kind: 'player-management-closed' });
  }

  function requestDelete(characterId: string, name: string): void {
    if (readOnly.value || pending.value) return;
    bridge?.vueToLegacy({ characterId, kind: 'player-management-delete-request', name });
  }

  const release = bridge?.subscribeLegacyState(handleLegacyMessage) ?? (() => {});
  onUnmounted(() => {
    active = false;
    lifecycle += 1;
    release();
  });

  return {
    add,
    addResetRequest: readonly(addResetRequest),
    close,
    deleteFocusCharacterId: readonly(deleteFocusCharacterId),
    deleteFocusRequest: readonly(deleteFocusRequest),
    error: readonly(error),
    open: readonly(open),
    pending: readonly(pending),
    profiles: readonly(profileList),
    readOnly: readonly(readOnly),
    requestDelete,
    save,
    status: readonly(status),
  };
}
