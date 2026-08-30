export type DesktopRecord = Readonly<Record<string, unknown>>;

export interface DesktopCommandResult extends DesktopRecord {
  readonly ok: boolean;
  readonly error: string;
}

export interface DesktopDocumentResult extends DesktopCommandResult {
  readonly canceled: boolean;
  readonly session: DesktopRecord | null;
}

export interface DesktopRuntimeStatus extends DesktopRecord {
  readonly ok: boolean;
}

export interface DesktopPublicAccessSnapshot extends DesktopRecord {
  readonly generation: number;
  readonly settingsRevision: number;
}

export interface DesktopApplicationUpdateSnapshot extends DesktopRecord {
  readonly revision: number;
}
