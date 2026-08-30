import { inject, onUnmounted, readonly, ref } from 'vue';

import { overseerCoexistenceBridgeKey, type OverseerCoexistenceMessage } from '../mount.js';
import type { DesktopCommandResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort } from '../ports/desktop-port.js';

export interface LogicalSessionCharacter {
  readonly id: string;
  readonly name: string;
}

export interface LogicalRosterCharacter extends LogicalSessionCharacter {
  readonly claimedBySessionId: string;
}

export interface LogicalSession {
  readonly character: LogicalSessionCharacter | null;
  readonly connected: boolean;
  readonly fallbackName: string;
  readonly id: string;
  readonly role: string;
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function character(value: unknown): LogicalSessionCharacter | null {
  if (!isRecord(value) || typeof value.id !== 'string' || typeof value.name !== 'string') return null;
  return Object.freeze({ id: value.id, name: value.name });
}

function rosterCharacter(value: unknown): LogicalRosterCharacter | null {
  const parsed = character(value);
  if (parsed === null || !isRecord(value)) return null;
  return Object.freeze({
    ...parsed,
    claimedBySessionId: typeof value.claimedBySessionId === 'string' ? value.claimedBySessionId : '',
  });
}

function logicalSession(value: unknown): LogicalSession | null {
  if (!isRecord(value) || typeof value.id !== 'string' || typeof value.fallbackName !== 'string') return null;
  const assigned = value.character == null ? null : character(value.character);
  if (value.character != null && assigned === null) return null;
  return Object.freeze({
    character: assigned,
    connected: value.connected === true,
    fallbackName: value.fallbackName,
    id: value.id,
    role: typeof value.role === 'string' ? value.role : 'unassigned',
  });
}

function parsedValues<T>(value: unknown, parse: (item: unknown) => T | null): readonly T[] {
  if (!Array.isArray(value)) return Object.freeze([]);
  const parsed: T[] = [];
  for (const item of value) {
    const result = parse(item);
    if (result !== null) parsed.push(result);
  }
  return Object.freeze(parsed);
}

export function useLogicalSessions(port: DesktopPort) {
  const bridge = inject(overseerCoexistenceBridgeKey, null);
  const broadcastActive = ref(false);
  const error = ref('');
  const open = ref(false);
  const pending = ref(false);
  const playerConfigActive = ref(false);
  const revision = ref(0);
  const roster = ref<readonly LogicalRosterCharacter[]>(Object.freeze([]));
  const sessions = ref<readonly LogicalSession[]>(Object.freeze([]));
  const status = ref('');
  let active = true;
  let lifecycle = 0;

  function handleLegacyMessage(message: OverseerCoexistenceMessage): void {
    if (message.kind === 'logical-session-open-request') {
      lifecycle += 1;
      error.value = '';
      status.value = '';
      open.value = true;
      return;
    }
    if (message.kind !== 'coordination-state') return;
    if (!isRecord(message.coordination)) {
      revision.value = 0;
      broadcastActive.value = false;
      playerConfigActive.value = false;
      roster.value = Object.freeze([]);
      sessions.value = Object.freeze([]);
      pending.value = message.pending === true;
      return;
    }
    const nextRevision = Number(message.coordination.revision ?? 0);
    if (!Number.isSafeInteger(nextRevision) || nextRevision < revision.value) return;
    revision.value = nextRevision;
    broadcastActive.value = isRecord(message.coordination.broadcast);
    playerConfigActive.value = isRecord(message.coordination.playerConfig);
    roster.value = parsedValues(message.coordination.roster, rosterCharacter);
    sessions.value = parsedValues(message.coordination.sessions, logicalSession);
    pending.value = message.pending === true;
  }

  function findSession(sessionId: string): LogicalSession | null {
    return sessions.value.find(session => session.id === sessionId) ?? null;
  }

  function findCharacter(characterId: string): LogicalRosterCharacter | null {
    return roster.value.find(candidate => candidate.id === characterId) ?? null;
  }

  async function run(
    sessionId: string,
    pendingMessage: string,
    successMessage: string,
    command: () => Promise<DesktopCommandResult>,
  ): Promise<void> {
    if (!open.value || pending.value || findSession(sessionId) === null) return;
    const expectedRevision = revision.value;
    const expectedLifecycle = lifecycle;
    pending.value = true;
    error.value = '';
    status.value = pendingMessage;
    bridge?.vueToLegacy({
      expectedRevision,
      kind: 'logical-session-command-started',
      status: pendingMessage,
    });

    let result: DesktopCommandResult;
    try {
      result = await command();
    } catch (cause) {
      result = {
        error: cause instanceof Error ? cause.message : String(cause),
        ok: false,
      };
    }

    const current = active
      && open.value
      && lifecycle === expectedLifecycle
      && revision.value === expectedRevision
      && findSession(sessionId) !== null;
    pending.value = false;
    if (current) {
      status.value = result.ok ? successMessage : '';
      error.value = result.ok ? '' : (result.error || 'ОПЕРАЦИЯ ОТКЛОНЕНА');
    } else if (active && open.value) {
      status.value = '';
      error.value = '';
    }
    bridge?.vueToLegacy({
      expectedRevision,
      kind: 'logical-session-command-finished',
      result,
      successMessage,
    });
  }

  function close(): void {
    if (!open.value) return;
    lifecycle += 1;
    open.value = false;
  }

  function rename(sessionId: string, fallbackName: string): void {
    const name = fallbackName.trim();
    if (name === '') {
      status.value = '';
      error.value = 'УКАЖИТЕ МЕТКУ СЕССИИ';
      return;
    }
    void run(
      sessionId,
      'ПЕРЕИМЕНОВАНИЕ СЕССИИ...',
      'МЕТКА СЕССИИ ОБНОВЛЕНА',
      () => port.renameLogicalSession({ sessionId, fallbackName: name }),
    );
  }

  function assign(sessionId: string, characterId: string): void {
    const session = findSession(sessionId);
    const selected = findCharacter(characterId);
    if (!broadcastActive.value || session === null || session.character !== null
      || selected === null || selected.claimedBySessionId !== '') return;
    void run(
      sessionId,
      'НАЗНАЧЕНИЕ ПЕРСОНАЖА...',
      'ПЕРСОНАЖ НАЗНАЧЕН',
      () => port.assignCharacter({ sessionId, characterId }),
    );
  }

  function release(sessionId: string): void {
    if (findSession(sessionId)?.character === null) return;
    void run(
      sessionId,
      'ОСВОБОЖДЕНИЕ ПЕРСОНАЖА...',
      'ПЕРСОНАЖ ОСВОБОЖДЁН',
      () => port.releaseCharacter(sessionId),
    );
  }

  function controller(sessionId: string): void {
    const session = findSession(sessionId);
    if (session?.character === null || !session?.connected || session.role === 'active') return;
    void run(
      sessionId,
      'ПЕРЕДАЧА УПРАВЛЕНИЯ...',
      'УПРАВЛЕНИЕ ПЕРЕДАНО',
      () => port.setActiveController(sessionId),
    );
  }

  function move(sessionId: string, destinationId: string): void {
    const source = findSession(sessionId);
    const destination = findSession(destinationId);
    if (!playerConfigActive.value || source === null || source.character === null
      || destination === null || destination.character !== null) return;
    const characterId = source.character.id;
    void run(
      sessionId,
      'ПЕРЕМЕЩЕНИЕ НАЗНАЧЕНИЯ...',
      'НАЗНАЧЕНИЕ ПЕРЕМЕЩЕНО',
      () => port.moveCharacter({ characterId, toSessionId: destinationId }),
    );
  }

  const releaseSubscription = bridge?.subscribeLegacyState(handleLegacyMessage) ?? (() => {});
  onUnmounted(() => {
    active = false;
    lifecycle += 1;
    releaseSubscription();
  });

  return {
    assign,
    broadcastActive: readonly(broadcastActive),
    close,
    controller,
    error: readonly(error),
    move,
    open: readonly(open),
    pending: readonly(pending),
    playerConfigActive: readonly(playerConfigActive),
    release,
    rename,
    roster: readonly(roster),
    sessions: readonly(sessions),
    status: readonly(status),
  };
}
