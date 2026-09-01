import { onScopeDispose, readonly, shallowRef, type DeepReadonly, type Ref } from 'vue';

import {
  PlayerPhase,
  PlayerRole,
  RosterAvailability,
  type PlayerState,
} from '../../gen/fallout/terminal/player/v1/player_pb.js';

export type PlayerIdentityRole = 'active' | 'observer' | 'unassigned';
export type PlayerIdentityPhase = 'controlling' | 'no-broadcast' | 'observing' | 'selecting' | 'waiting';
export type PlayerRosterStatus = 'available' | 'claimed';

export interface PlayerRosterIdentity {
  readonly id: string;
  readonly name: string;
  readonly status: PlayerRosterStatus;
}

export interface PlayerAssignedIdentity {
  readonly id: string;
  readonly name: string;
}

export interface PlayerIdentityState {
  readonly assigned: PlayerAssignedIdentity | null;
  readonly fallbackName: string;
  readonly logicalSessionID: string;
  readonly phase: PlayerIdentityPhase;
  readonly recognitionHandle: string;
  readonly revision: number;
  readonly role: PlayerIdentityRole;
  readonly roster: readonly PlayerRosterIdentity[];
}

export interface PlayerIdentityController {
  readonly state: Readonly<PlayerIdentityState> | null;
  apply(recognitionHandle: string, revision: number, playerState: PlayerState): boolean;
  dispose(): void;
}

const encoder = new TextEncoder();

function opaque(value: string, maximumBytes: number): boolean {
  if (value === '' || value.trim() !== value || encoder.encode(value).byteLength > maximumBytes) return false;
  for (const character of value) {
    const point = character.codePointAt(0) ?? 0;
    if (point < 0x21 || point > 0x7e) return false;
  }
  return true;
}

function displayText(value: string): boolean {
  if (value === '' || value.trim() !== value || encoder.encode(value).byteLength > 512) return false;
  for (const character of value) {
    const point = character.codePointAt(0) ?? 0;
    if (point < 0x20 || point === 0x7f) return false;
  }
  return true;
}

function roleName(role: PlayerRole): PlayerIdentityRole | null {
  switch (role) {
    case PlayerRole.ACTIVE: return 'active';
    case PlayerRole.OBSERVER: return 'observer';
    case PlayerRole.UNASSIGNED: return 'unassigned';
    default: return null;
  }
}

function phaseName(phase: PlayerPhase): PlayerIdentityPhase | null {
  switch (phase) {
    case PlayerPhase.CONTROLLING: return 'controlling';
    case PlayerPhase.NO_BROADCAST: return 'no-broadcast';
    case PlayerPhase.OBSERVING: return 'observing';
    case PlayerPhase.SELECTING: return 'selecting';
    case PlayerPhase.WAITING: return 'waiting';
    default: return null;
  }
}

function rosterStatus(status: RosterAvailability): PlayerRosterStatus | null {
  switch (status) {
    case RosterAvailability.AVAILABLE: return 'available';
    case RosterAvailability.CLAIMED: return 'claimed';
    default: return null;
  }
}

function project(
  recognitionHandle: string,
  revision: number,
  playerState: PlayerState,
): Readonly<PlayerIdentityState> | null {
  const role = roleName(playerState.role);
  const phase = phaseName(playerState.phase);
  if (!opaque(recognitionHandle, 128) || !Number.isSafeInteger(revision) || revision < 0 ||
      !opaque(playerState.logicalSessionId, 128) || !displayText(playerState.fallbackName) ||
      role === null || phase === null || playerState.roster.length > 256) return null;

  const seen = new Set<string>();
  const roster: PlayerRosterIdentity[] = [];
  for (const entry of playerState.roster) {
    const status = rosterStatus(entry.availability);
    if (!opaque(entry.characterId, 256) || !displayText(entry.displayName) ||
        status === null || seen.has(entry.characterId)) return null;
    seen.add(entry.characterId);
    roster.push(Object.freeze({ id: entry.characterId, name: entry.displayName, status }));
  }

  let assigned: PlayerAssignedIdentity | null = null;
  if (playerState.assignedCharacter !== undefined) {
    const value = playerState.assignedCharacter;
    if (!opaque(value.characterId, 256) || !displayText(value.displayName)) return null;
    const rosterEntry = roster.find(entry => entry.id === value.characterId);
    if (rosterEntry !== undefined && rosterEntry.name !== value.displayName) return null;
    assigned = Object.freeze({ id: value.characterId, name: value.displayName });
  }

  return Object.freeze({
    assigned,
    fallbackName: playerState.fallbackName,
    logicalSessionID: playerState.logicalSessionId,
    phase,
    recognitionHandle,
    revision,
    role,
    roster: Object.freeze(roster),
  });
}

export function createPlayerIdentityController(
  onState?: (state: Readonly<PlayerIdentityState>) => void,
): PlayerIdentityController {
  let disposed = false;
  let state: Readonly<PlayerIdentityState> | null = null;
  return Object.freeze({
    get state() { return state; },
    apply(recognitionHandle: string, revision: number, playerState: PlayerState): boolean {
      if (disposed || (state !== null && revision <= state.revision)) return false;
      const next = project(recognitionHandle, revision, playerState);
      if (next === null) return false;
      state = next;
      onState?.(state);
      return true;
    },
    dispose(): void { disposed = true; },
  });
}

export interface PlayerIdentityComposable {
  readonly state: DeepReadonly<Ref<Readonly<PlayerIdentityState> | null>>;
  apply(recognitionHandle: string, revision: number, playerState: PlayerState): boolean;
  dispose(): void;
}

export function usePlayerIdentity(): PlayerIdentityComposable {
  const state = shallowRef<Readonly<PlayerIdentityState> | null>(null);
  const controller = createPlayerIdentityController(next => { state.value = next; });
  onScopeDispose(controller.dispose, true);
  return Object.freeze({ apply: controller.apply, dispose: controller.dispose, state: readonly(state) });
}
