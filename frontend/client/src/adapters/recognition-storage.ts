export const PLAYER_RECOGNITION_HANDLE_KEY = 'fallout-terminal.player-token';
export const PLAYER_SESSION_INIT_LOCK_NAME = 'fallout-terminal.player-session-init';
export const PLAYER_SESSION_INIT_LEASE_KEY = 'fallout-terminal.player-session-init-lease';
export const PLAYER_SESSION_INIT_CONTENDER_PREFIX = 'fallout-terminal.player-session-init-contender.';
export const RECOGNITION_RECORD_VERSION = 1;

const MAX_OPAQUE_ID_BYTES = 128;

export interface RecognitionCoordinationRecord {
  readonly expiresAt: number;
  readonly owner: string;
  readonly token: string;
  readonly version: typeof RECOGNITION_RECORD_VERSION;
}

export interface RecognitionStorageEventTarget {
  addEventListener(type: 'storage', listener: (event: StorageEvent) => void): void;
  removeEventListener(type: 'storage', listener: (event: StorageEvent) => void): void;
}

export interface RecognitionStorageAdapterOptions {
  readonly eventTarget?: RecognitionStorageEventTarget;
  readonly storage?: Storage;
}

export interface RecognitionStorageAdapter {
  contenderKey(owner: string): string | null;
  listContenders(now: number): readonly RecognitionCoordinationRecord[];
  readLease(): RecognitionCoordinationRecord | null;
  readRecognitionHandle(): string | null;
  removeContender(owner: string, token: string): boolean;
  removeLease(owner: string, token: string): boolean;
  subscribe(listener: () => void): () => void;
  writeContender(record: RecognitionCoordinationRecord): boolean;
  writeLease(record: RecognitionCoordinationRecord): boolean;
  writeRecognitionHandle(handle: string): boolean;
}

function validOpaqueID(value: unknown): value is string {
  if (typeof value !== 'string' || value === '' || value.trim() !== value ||
      new TextEncoder().encode(value).byteLength > MAX_OPAQUE_ID_BYTES) return false;
  for (const character of value) {
    const point = character.codePointAt(0) ?? 0;
    if (point < 0x21 || point > 0x7e) return false;
  }
  return true;
}

function validNow(value: number): boolean {
  return Number.isSafeInteger(value) && value >= 0;
}

function parseRecord(raw: string | null, expectedOwner?: string): RecognitionCoordinationRecord | null {
  if (raw === null || raw.length > 1_024) return null;
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  if (record.version !== RECOGNITION_RECORD_VERSION || !validOpaqueID(record.owner) ||
      !validOpaqueID(record.token) || !Number.isSafeInteger(record.expiresAt) ||
      Number(record.expiresAt) <= 0 || (expectedOwner !== undefined && record.owner !== expectedOwner)) {
    return null;
  }
  return Object.freeze({
    expiresAt: Number(record.expiresAt),
    owner: record.owner,
    token: record.token,
    version: RECOGNITION_RECORD_VERSION,
  });
}

function validRecord(record: RecognitionCoordinationRecord): boolean {
  return record.version === RECOGNITION_RECORD_VERSION && validOpaqueID(record.owner) &&
    validOpaqueID(record.token) && Number.isSafeInteger(record.expiresAt) && record.expiresAt > 0;
}

function encodedRecord(record: RecognitionCoordinationRecord): string {
  return JSON.stringify({
    expiresAt: record.expiresAt,
    owner: record.owner,
    token: record.token,
    version: RECOGNITION_RECORD_VERSION,
  });
}

export function createRecognitionStorage(
  options: RecognitionStorageAdapterOptions = {},
): RecognitionStorageAdapter {
  const storage: Storage = options.storage ?? globalThis.localStorage;
  const eventTarget: RecognitionStorageEventTarget = options.eventTarget ?? globalThis.window;

  const safeGet = (key: string): string | null => {
    try { return storage.getItem(key); } catch { return null; }
  };
  const safeRemove = (key: string): boolean => {
    try { storage.removeItem(key); return true; } catch { return false; }
  };
  const safeSet = (key: string, value: string): boolean => {
    try { storage.setItem(key, value); return true; } catch { return false; }
  };
  const contenderKey = (owner: string): string | null => validOpaqueID(owner)
    ? `${PLAYER_SESSION_INIT_CONTENDER_PREFIX}${owner}`
    : null;

  return Object.freeze({
    contenderKey,
    listContenders(now: number): readonly RecognitionCoordinationRecord[] {
      if (!validNow(now)) return Object.freeze([]);
      const records: RecognitionCoordinationRecord[] = [];
      try {
        for (let index = storage.length - 1; index >= 0; index -= 1) {
          const key = storage.key(index);
          if (key === null || !key.startsWith(PLAYER_SESSION_INIT_CONTENDER_PREFIX)) continue;
          const owner = key.slice(PLAYER_SESSION_INIT_CONTENDER_PREFIX.length);
          const record = validOpaqueID(owner) ? parseRecord(storage.getItem(key), owner) : null;
          if (record === null || record.expiresAt <= now) {
            storage.removeItem(key);
            continue;
          }
          records.push(record);
        }
      } catch {
        return Object.freeze([]);
      }
      records.sort((left, right) => left.owner.localeCompare(right.owner));
      return Object.freeze(records);
    },
    readLease(): RecognitionCoordinationRecord | null {
      return parseRecord(safeGet(PLAYER_SESSION_INIT_LEASE_KEY));
    },
    readRecognitionHandle(): string | null {
      const handle = safeGet(PLAYER_RECOGNITION_HANDLE_KEY);
      return validOpaqueID(handle) ? handle : null;
    },
    removeContender(owner: string, token: string): boolean {
      const key = contenderKey(owner);
      if (key === null || !validOpaqueID(token)) return false;
      const current = parseRecord(safeGet(key), owner);
      return current !== null && current.token === token && safeRemove(key);
    },
    removeLease(owner: string, token: string): boolean {
      if (!validOpaqueID(owner) || !validOpaqueID(token)) return false;
      const current = parseRecord(safeGet(PLAYER_SESSION_INIT_LEASE_KEY));
      return current !== null && current.owner === owner && current.token === token &&
        safeRemove(PLAYER_SESSION_INIT_LEASE_KEY);
    },
    subscribe(listener: () => void): () => void {
      let released = false;
      const onStorage = (event: StorageEvent): void => {
        const key = event.key;
        if (key === PLAYER_RECOGNITION_HANDLE_KEY) {
          if (event.newValue === null || validOpaqueID(event.newValue)) listener();
          return;
        }
        if (key === PLAYER_SESSION_INIT_LEASE_KEY) {
          if (event.newValue === null || parseRecord(event.newValue) !== null) listener();
          return;
        }
        if (key?.startsWith(PLAYER_SESSION_INIT_CONTENDER_PREFIX) !== true) return;
        const owner = key.slice(PLAYER_SESSION_INIT_CONTENDER_PREFIX.length);
        if (event.newValue === null ? validOpaqueID(owner) : parseRecord(event.newValue, owner) !== null) listener();
      };
      eventTarget.addEventListener('storage', onStorage);
      return () => {
        if (released) return;
        released = true;
        eventTarget.removeEventListener('storage', onStorage);
      };
    },
    writeContender(record: RecognitionCoordinationRecord): boolean {
      const key = contenderKey(record.owner);
      return key !== null && validRecord(record) && safeSet(key, encodedRecord(record));
    },
    writeLease(record: RecognitionCoordinationRecord): boolean {
      return validRecord(record) && safeSet(PLAYER_SESSION_INIT_LEASE_KEY, encodedRecord(record));
    },
    writeRecognitionHandle(handle: string): boolean {
      return validOpaqueID(handle) && safeSet(PLAYER_RECOGNITION_HANDLE_KEY, handle);
    },
  });
}
