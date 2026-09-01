import type { InjectionKey } from 'vue';

import type { DesktopCommandResult, DesktopRecord } from '../models/overseer-view-state.js';
import type { DesktopPort, DesktopUnsubscribe } from '../ports/desktop-port.js';

export interface OverseerControllerMessage extends DesktopRecord {
  readonly kind: string;
}

export interface OverseerController {
  readonly dispatch: (value: unknown) => boolean;
  readonly dispose: DesktopUnsubscribe;
  readonly publish: (value: unknown) => boolean;
  readonly subscribeState: (
    listener: (message: OverseerControllerMessage) => void,
  ) => DesktopUnsubscribe;
}

export const overseerControllerKey: InjectionKey<OverseerController> =
  Symbol('overseer-controller');

type ControllerListener = (message: OverseerControllerMessage) => void;

interface TerminalNode extends Record<string, unknown> {
  children?: TerminalNode[];
  description?: string;
  id: string;
  name: string;
  text?: string;
  type: 'command' | 'entry' | 'folder';
}

interface Terminal extends Record<string, unknown> {
  commandStates?: Record<string, DesktopRecord>;
  hackLevel: number;
  id: string;
  introText: string;
  name: string;
  root: TerminalNode;
}

interface TerminalGroup extends Record<string, unknown> {
  id: string;
  name: string;
  terminalIds: string[];
}

interface Session extends Record<string, unknown> {
  playerConfig?: string;
  terminalGroups: TerminalGroup[];
  terminals: Terminal[];
}

interface ControllerState {
  coordination: DesktopRecord | null;
  editTerminalID: string | null;
  expanded: Set<string>;
  filePath: string;
  liveTerminalID: string | null;
  newestDurableRevision: number;
  selectedNodeID: string | null;
  session: Session | null;
  terminalAuthoringRevision: number;
}

interface TerminalGroupDraft {
  groupID?: string;
  kind: 'create' | 'move' | 'rename';
  sourceGroupID?: string;
  terminalID?: string;
}

interface TerminalGroupImpact {
  affectedGroupNames: string[];
  affectedTerminalIDs: string[];
  candidate: TerminalGroup[];
  destinationGroupID?: string;
  destinationGroupName?: string;
  kind: string;
  membership: string;
  orderAfter: string[];
  orderBefore: string[];
  rejections?: DesktopRecord[];
  sourceGroupID?: string;
}

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function mutableRecord(value: unknown): Record<string, unknown> | null {
  if (!isRecord(value)) return null;
  return { ...structuredClone(value) };
}

function controllerMessage(value: unknown): OverseerControllerMessage | null {
  const copy = mutableRecord(value);
  if (copy === null || typeof copy.kind !== 'string' || copy.kind === '') return null;
  return Object.freeze({ ...copy, kind: copy.kind });
}

function nonNegativeInteger(value: unknown): number {
  return Number.isSafeInteger(value) && Number(value) >= 0 ? Number(value) : 0;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function parseNode(value: unknown, ids: Set<string>): TerminalNode | null {
  const source = mutableRecord(value);
  if (source === null || typeof source.id !== 'string' || source.id === '' || ids.has(source.id)
    || typeof source.name !== 'string'
    || (source.type !== 'folder' && source.type !== 'command' && source.type !== 'entry')) return null;
  ids.add(source.id);
  const rawChildren = source.children ?? [];
  if (!Array.isArray(rawChildren) || (source.type !== 'folder' && rawChildren.length !== 0)) return null;
  const children: TerminalNode[] = [];
  for (const child of rawChildren) {
    const parsed = parseNode(child, ids);
    if (parsed === null) return null;
    children.push(parsed);
  }
  return {
    ...source,
    children,
    description: text(source.description),
    id: source.id,
    name: source.name,
    text: text(source.text),
    type: source.type,
  };
}

function parseTerminal(value: unknown): Terminal | null {
  const source = mutableRecord(value);
  if (source === null || typeof source.id !== 'string' || source.id === ''
    || typeof source.name !== 'string') return null;
  const root = parseNode(source.root, new Set<string>());
  if (root === null || root.id !== 'root' || root.type !== 'folder') return null;
  const rawCommandStates = mutableRecord(source.commandStates);
  const commandStates: Record<string, DesktopRecord> = {};
  for (const [key, candidate] of Object.entries(rawCommandStates ?? {})) {
    if (isRecord(candidate)) commandStates[key] = Object.freeze(structuredClone(candidate));
  }
  return {
    ...source,
    commandStates,
    hackLevel: Math.min(5, nonNegativeInteger(source.hackLevel)),
    id: source.id,
    introText: text(source.introText),
    name: source.name,
    root,
  };
}

function parseSession(value: unknown): Session | null {
  const source = mutableRecord(value);
  if (source === null || !Array.isArray(source.terminals)) return null;
  const terminals: Terminal[] = [];
  const terminalIDs = new Set<string>();
  for (const candidate of source.terminals) {
    const terminal = parseTerminal(candidate);
    if (terminal === null || terminalIDs.has(terminal.id)) return null;
    terminalIDs.add(terminal.id);
    terminals.push(terminal);
  }
  const terminalGroups: TerminalGroup[] = [];
  const represented = new Set<string>();
  if (source.terminalGroups !== undefined && !Array.isArray(source.terminalGroups)) return null;
  for (const candidate of source.terminalGroups ?? []) {
    const group = mutableRecord(candidate);
    if (group === null || typeof group.id !== 'string' || group.id === ''
      || typeof group.name !== 'string' || !Array.isArray(group.terminalIds)) return null;
    const members: string[] = [];
    for (const terminalID of group.terminalIds) {
      if (typeof terminalID !== 'string' || !terminalIDs.has(terminalID)
        || represented.has(terminalID)) return null;
      represented.add(terminalID);
      members.push(terminalID);
    }
    if (members.length > 0) terminalGroups.push({ ...group, id: group.id, name: group.name, terminalIds: members });
  }
  for (const terminal of terminals) {
    if (represented.has(terminal.id)) continue;
    terminalGroups.push({
      id: `group-${terminal.id}`,
      name: terminal.name,
      terminalIds: [terminal.id],
    });
  }
  const session: Session = {
    ...source,
    terminalGroups,
    terminals,
  };
  if (typeof source.playerConfig === 'string') session.playerConfig = source.playerConfig;
  return session;
}

function coordinationRevision(value: DesktopRecord | null): number {
  return nonNegativeInteger(value?.revision);
}

function broadcastTerminalID(value: DesktopRecord | null): string | null {
  const broadcast = value === null ? null : mutableRecord(value.broadcast);
  return broadcast !== null && typeof broadcast.activeTerminalId === 'string'
    ? broadcast.activeTerminalId
    : null;
}

function subscribe(
  listeners: Set<ControllerListener>,
  listener: ControllerListener,
): DesktopUnsubscribe {
  listeners.add(listener);
  let active = true;
  return () => {
    if (!active) return;
    active = false;
    listeners.delete(listener);
  };
}

function locateNode(
  root: TerminalNode,
  id: string,
  parent: TerminalNode | null = null,
): { node: TerminalNode; parent: TerminalNode | null } | null {
  if (root.id === id) return { node: root, parent };
  for (const child of root.children ?? []) {
    const located = locateNode(child, id, root);
    if (located !== null) return located;
  }
  return null;
}

function terminalTreeNode(terminal: Terminal, node: TerminalNode): DesktopRecord {
  const execution = terminal.commandStates?.[node.id];
  const stateChange = mutableRecord(node.stateChange);
  const terminalTransition = mutableRecord(node.terminalTransition);
  return Object.freeze({
    children: node.type === 'folder'
      ? (node.children ?? []).map(child => terminalTreeNode(terminal, child))
      : [],
    description: node.type === 'entry' ? text(node.description) : '',
    displayName: node.type === 'command' && typeof execution?.completedName === 'string'
      ? execution.completedName
      : node.name,
    execution: execution === undefined ? null : Object.freeze({
      completedName: text(execution.completedName),
      resultText: text(execution.resultText),
    }),
    id: node.id,
    name: node.name,
    stateChange: stateChange === null ? null : Object.freeze({
      completedName: text(stateChange.completedName),
      confirmationText: text(stateChange.confirmationText),
    }),
    terminalTransition: terminalTransition === null ? null : Object.freeze({
      targetTerminalId: text(terminalTransition.targetTerminalId),
    }),
    text: node.type === 'command' ? text(node.text) : '',
    type: node.type,
  });
}

export function createOverseerController(port: DesktopPort): OverseerController {
  const listeners = new Set<ControllerListener>();
  const releases: DesktopUnsubscribe[] = [];
  const state: ControllerState = {
    coordination: null,
    editTerminalID: null,
    expanded: new Set(['root']),
    filePath: '',
    liveTerminalID: null,
    newestDurableRevision: 0,
    selectedNodeID: null,
    session: null,
    terminalAuthoringRevision: 0,
  };
  let disposed = false;
  let coordinationPending = false;
  let coordinationStatus = '';
  let coordinationError = '';
  let createTerminalOpen = false;
  let createTerminalPending = false;
  let groupDraft: TerminalGroupDraft | null = null;
  let groupImpact: TerminalGroupImpact | null = null;
  let groupFocusOwner: Readonly<{ ownerID: string; scope: string }> | null = null;
  let idSequence = 0;
  let publicAccessSnapshot: DesktopRecord | null = null;
  let resetConfirmation: Readonly<{
    complete: (confirmed: boolean) => void;
    requestID: string;
  }> | null = null;
  let saveGeneration = 0;
  let sessionStatePending = false;
  let terminalPublishAcknowledgement = 0;

  const uniqueID = (prefix: string): string => {
    idSequence += 1;
    return `${prefix}_${Date.now().toString(36)}_${idSequence}`;
  };

  const publish = (value: unknown): boolean => {
    if (disposed) return false;
    const message = controllerMessage(value);
    if (message === null) return false;
    for (const listener of listeners) listener(message);
    return true;
  };

  const currentTerminal = (): Terminal | null => state.session?.terminals
    .find(terminal => terminal.id === state.editTerminalID) ?? null;

  const publishCoordination = (): void => {
    publish({
      coordination: state.coordination,
      error: coordinationError,
      kind: 'coordination-state',
      pending: coordinationPending,
      status: coordinationStatus,
    });
  };

  const publishAuthoring = (): void => {
    if (state.session === null) return;
    state.terminalAuthoringRevision += 1;
    const revision = state.terminalAuthoringRevision;
    const terminalsByID = new Map(state.session.terminals.map(terminal => [terminal.id, terminal]));
    const groups = state.session.terminalGroups.map(group => Object.freeze({
      id: group.id,
      name: group.name,
      terminalIDs: [...group.terminalIds],
    }));
    const terminals = state.session.terminalGroups.flatMap(group => group.terminalIds.flatMap(
      (terminalID, memberIndex) => {
        const terminal = terminalsByID.get(terminalID);
        if (terminal === undefined) return [];
        return [Object.freeze({
          groupID: group.id,
          groupName: group.name,
          id: terminal.id,
          live: terminal.id === state.liveTerminalID,
          memberCount: group.terminalIds.length,
          memberIndex,
          name: terminal.name,
          revision,
          selected: terminal.id === state.editTerminalID,
        })];
      },
    ));
    const terminal = currentTerminal();
    const selected = terminal !== null && state.selectedNodeID !== null
      ? locateNode(terminal.root, state.selectedNodeID)
      : null;
    if (state.selectedNodeID !== null && selected === null) state.selectedNodeID = null;
    const target = terminal === null
      ? null
      : state.selectedNodeID === null
        ? terminal.root
        : locateNode(terminal.root, state.selectedNodeID)?.node ?? terminal.root;
    const group = terminal === null
      ? undefined
      : state.session.terminalGroups.find(candidate => candidate.terminalIds.includes(terminal.id));
    publish({
      create: Object.freeze({ open: createTerminalOpen, pending: createTerminalPending }),
      editor: Object.freeze({
        broadcastActive: isRecord(state.coordination?.broadcast),
        pending: coordinationPending,
        publishAcknowledgement: terminalPublishAcknowledgement,
        resetPending: sessionStatePending,
        terminal: terminal === null ? null : Object.freeze({
          completedCount: Object.keys(terminal.commandStates ?? {}).length,
          hackLevel: terminal.hackLevel,
          id: terminal.id,
          introText: terminal.introText,
          live: terminal.id === state.liveTerminalID,
          name: terminal.name,
        }),
      }),
      groups,
      hackContext: terminal === null || terminal.id !== state.liveTerminalID || terminal.hackLevel === 0
        ? null
        : Object.freeze({
          coordinationRevision: coordinationRevision(state.coordination),
          hackLevel: terminal.hackLevel,
          introText: terminal.introText,
          terminalID: terminal.id,
          terminalName: terminal.name,
          tree: terminal.root,
        }),
      kind: 'terminal-authoring-snapshot',
      revision,
      terminals,
      tree: terminal === null ? Object.freeze({ available: false }) : Object.freeze({
        addTargetName: target?.id === 'root' ? 'ROOT' : target?.name ?? '',
        available: true,
        expandedIDs: [...state.expanded],
        pending: sessionStatePending,
        root: terminalTreeNode(terminal, terminal.root),
        selectedNodeID: state.selectedNodeID,
        terminalID: terminal.id,
        terminalOptions: state.session.terminals
          .filter(candidate => candidate.id !== terminal.id && group?.terminalIds.includes(candidate.id))
          .map(candidate => Object.freeze({ id: candidate.id, name: candidate.name })),
      }),
    });
  };

  const save = async (): Promise<void> => {
    if (state.session === null || state.filePath === '') return;
    const generation = ++saveGeneration;
    const result = await port.saveSession(state.session);
    if (generation !== saveGeneration) return;
    if (result.ok) {
      state.newestDurableRevision = Math.max(
        state.newestDurableRevision,
        nonNegativeInteger(result.savedRevision),
      );
    }
    publish({
      error: result.ok ? '' : result.error,
      kind: 'session-save-status',
      revision: state.newestDurableRevision,
      text: result.ok ? 'Сохранено' : `Ошибка сохранения: ${result.error}`,
    });
  };

  const applySessionResult = (
    result: DesktopCommandResult,
    successMessage: string,
    accepts?: (session: Session, revision: number) => boolean,
  ): boolean => {
    const session = parseSession(result.session);
    const revision = nonNegativeInteger(result.revision ?? result.savedRevision);
    const canonicalRejected = result.ok === true && session !== null
      && accepts !== undefined && !accepts(session, revision);
    if (!result.ok || session === null || canonicalRejected) {
      const error = canonicalRejected
        ? 'БЭКЕНД НЕ ПОДТВЕРДИЛ КАНОНИЧЕСКИЙ СБРОС'
        : result.error || 'сессия не обновлена';
      publish({
        error,
        kind: 'session-save-status',
        revision: state.newestDurableRevision,
        text: `Ошибка изменения состояния: ${error}`,
      });
      return false;
    }
    state.session = session;
    state.newestDurableRevision = Math.max(state.newestDurableRevision, revision);
    publish({
      error: '',
      kind: 'session-save-status',
      revision: state.newestDurableRevision,
      text: `${successMessage}${revision > 0 ? ` · ревизия ${revision}` : ''}`,
    });
    publishAuthoring();
    return true;
  };

  const runSessionStateCommand = async (
    command: () => Promise<DesktopCommandResult>,
    successMessage: string,
    accepts?: (session: Session, revision: number) => boolean,
  ): Promise<void> => {
    if (sessionStatePending) return;
    sessionStatePending = true;
    publishAuthoring();
    let result: DesktopCommandResult;
    try {
      result = await command();
    } catch (cause) {
      result = { error: cause instanceof Error ? cause.message : String(cause), ok: false };
    }
    if (!disposed) applySessionResult(result, successMessage, accepts);
    sessionStatePending = false;
    publishAuthoring();
  };

  const runCoordinationCommand = async (
    command: () => Promise<DesktopCommandResult>,
    successMessage: string,
    pendingMessage: string,
  ): Promise<DesktopCommandResult | null> => {
    if (coordinationPending) return null;
    coordinationPending = true;
    coordinationStatus = pendingMessage;
    coordinationError = '';
    publishCoordination();
    publishAuthoring();
    let result: DesktopCommandResult;
    try {
      result = await command();
    } catch (cause) {
      result = { error: cause instanceof Error ? cause.message : String(cause), ok: false };
    }
    if (disposed) return result;
    coordinationPending = false;
    const next = mutableRecord(result.state);
    if (next !== null) applyCoordination(next);
    coordinationStatus = result.ok ? successMessage : '';
    coordinationError = result.ok ? '' : result.error || 'ОПЕРАЦИЯ ОТКЛОНЕНА';
    publishCoordination();
    publishAuthoring();
    if (result.ok && result.status === 'decision-required' && typeof result.switchId === 'string') {
      publish({ kind: 'terminal-switch-required', switchId: result.switchId });
    }
    return result;
  };

  const confirmReset = (message: string, complete: (confirmed: boolean) => void): void => {
    if (resetConfirmation !== null) return;
    const requestID = `command-state-reset-${uniqueID('request')}`;
    resetConfirmation = Object.freeze({ complete, requestID });
    publish({ kind: 'command-state-reset-required', message, requestId: requestID });
  };

  const restoreGroupFocus = (): void => {
    if (groupFocusOwner === null) return;
    publish({
      kind: 'terminal-selection-focus-request',
      ownerID: groupFocusOwner.ownerID,
      scope: groupFocusOwner.scope,
    });
    groupFocusOwner = null;
  };

  const loadSession = (message: OverseerControllerMessage): void => {
    const session = parseSession(message.session);
    if (session === null) return;
    state.session = session;
    state.filePath = text(message.filePath);
    state.newestDurableRevision = nonNegativeInteger(message.sessionRevision);
    state.editTerminalID = session.terminals[0]?.id ?? null;
    state.selectedNodeID = null;
    state.expanded = new Set(['root']);
    state.liveTerminalID = broadcastTerminalID(state.coordination);
    publishAuthoring();
    publishCoordination();
    publish({
      error: '',
      kind: 'session-save-status',
      revision: state.newestDurableRevision,
      text: state.newestDurableRevision > 0
        ? `СОСТОЯНИЕ СЕССИИ ЗАГРУЖЕНО · ревизия ${state.newestDurableRevision}`
        : '',
    });
    publish({
      kind: session.playerConfig === undefined
        ? 'player-configuration-missing'
        : 'player-configuration-load-referenced',
    });
  };

  const applyCoordination = (candidate: unknown): void => {
    const next = mutableRecord(candidate);
    if (next === null) return;
    if (state.coordination !== null
      && coordinationRevision(next) <= coordinationRevision(state.coordination)) return;
    state.coordination = Object.freeze(next);
    state.liveTerminalID = broadcastTerminalID(state.coordination);
    publishCoordination();
    publishAuthoring();
  };

  const applyResult = (message: OverseerControllerMessage): boolean => {
    if (!message.kind.endsWith('-finished')) return false;
    coordinationPending = false;
    const result = mutableRecord(message.result);
    if (result?.state !== undefined) applyCoordination(result.state);
    if (result?.session !== undefined) {
      const session = parseSession(result.session);
      if (session !== null) state.session = session;
    }
    const ok = result?.ok === true;
    coordinationStatus = ok ? text(message.successMessage) || 'ОПЕРАЦИЯ ВЫПОЛНЕНА' : '';
    coordinationError = ok ? '' : text(result?.error) || 'ОПЕРАЦИЯ ОТКЛОНЕНА';
    publishCoordination();
    publishAuthoring();
    return true;
  };

  const currentAddTarget = (terminal: Terminal): TerminalNode => {
    if (state.selectedNodeID === null) return terminal.root;
    const located = locateNode(terminal.root, state.selectedNodeID);
    if (located === null) return terminal.root;
    return located.node.type === 'folder' ? located.node : located.parent ?? terminal.root;
  };

  const addTerminalNode = (terminal: Terminal, type: TerminalNode['type']): void => {
    const target = currentAddTarget(terminal);
    const node: TerminalNode = {
      id: uniqueID('n'),
      name: type === 'folder' ? 'Новая папка' : type === 'command' ? 'Новая команда' : 'Новая запись',
      type,
    };
    if (type === 'folder') node.children = [];
    else if (type === 'command') node.text = '';
    else node.description = '';
    target.children ??= [];
    target.children.push(node);
    state.expanded.add(target.id);
    state.selectedNodeID = node.id;
    void save();
    publishAuthoring();
    publish({ kind: 'terminal-tree-focus-request', nodeID: node.id });
  };

  const applyTerminalNodeDraft = (terminal: Terminal, node: TerminalNode, value: unknown): boolean => {
    const draft = mutableRecord(value);
    if (draft === null || typeof draft.name !== 'string' || draft.name.trim() === '') return false;
    if (node.type === 'command') {
      if (typeof draft.commandMode !== 'string' || typeof draft.text !== 'string'
        || typeof draft.completedName !== 'string' || typeof draft.confirmationText !== 'string'
        || typeof draft.targetTerminalID !== 'string') return false;
      let commandMode = draft.commandMode;
      if (terminal.commandStates?.[node.id] !== undefined) {
        commandMode = node.stateChange !== undefined
          ? 'state-change'
          : node.terminalTransition !== undefined ? 'terminal-transition' : 'ordinary';
      }
      if (commandMode !== 'ordinary' && commandMode !== 'state-change'
        && commandMode !== 'terminal-transition') return false;
      if (commandMode === 'state-change') {
        if (draft.completedName.trim() === '' || draft.confirmationText.trim() === ''
          || draft.text.trim() === '') return false;
        node.stateChange = {
          completedName: draft.completedName,
          confirmationText: draft.confirmationText,
        };
      } else {
        delete node.stateChange;
      }
      if (commandMode === 'terminal-transition') {
        const group = state.session?.terminalGroups.find(candidate => candidate.terminalIds.includes(terminal.id));
        if (draft.targetTerminalID === '' || draft.targetTerminalID === terminal.id
          || !group?.terminalIds.includes(draft.targetTerminalID)
          || !state.session?.terminals.some(candidate => candidate.id === draft.targetTerminalID)) return false;
        node.terminalTransition = { targetTerminalId: draft.targetTerminalID };
      } else {
        delete node.terminalTransition;
      }
      node.text = draft.text;
    } else if (node.type === 'entry') {
      if (typeof draft.text !== 'string') return false;
      node.description = draft.text;
    }
    node.name = draft.name.trim();
    return true;
  };

  const runTerminalTreeAction = async (
    message: OverseerControllerMessage,
    terminal: Terminal,
  ): Promise<void> => {
    if (message.action === 'select-node') {
      if (typeof message.nodeID !== 'string') return;
      const located = locateNode(terminal.root, message.nodeID);
      if (located === null) return;
      state.selectedNodeID = located.node.id;
      if (located.node.type === 'folder') state.expanded.add(located.node.id);
      publishAuthoring();
      return;
    }
    if (message.action === 'toggle-node') {
      if (typeof message.nodeID !== 'string') return;
      const located = locateNode(terminal.root, message.nodeID);
      if (located === null || located.node.type !== 'folder' || !located.node.children?.length) return;
      if (state.expanded.has(located.node.id)) state.expanded.delete(located.node.id);
      else state.expanded.add(located.node.id);
      publishAuthoring();
      return;
    }
    if (message.action === 'add-node') {
      if (message.nodeType === 'folder' || message.nodeType === 'command' || message.nodeType === 'entry') {
        addTerminalNode(terminal, message.nodeType);
      }
      return;
    }
    if (typeof message.nodeID !== 'string' || message.nodeID !== state.selectedNodeID) return;
    const located = locateNode(terminal.root, message.nodeID);
    if (located === null || located.node.id === 'root') return;
    if (message.action === 'apply-node') {
      if (!applyTerminalNodeDraft(terminal, located.node, message.draft)) return;
      void save();
      publishAuthoring();
      return;
    }
    if (message.action === 'delete-node') {
      const siblings = located.parent?.children;
      if (siblings === undefined) return;
      const index = siblings.findIndex(candidate => candidate.id === located.node.id);
      if (index < 0) return;
      siblings.splice(index, 1);
      state.selectedNodeID = null;
      void save();
      publishAuthoring();
      return;
    }
    if (message.action !== 'reset-command-state' || located.node.type !== 'command'
      || terminal.commandStates?.[located.node.id] === undefined || sessionStatePending) return;
    const commandID = located.node.id;
    const displayedName = text(terminal.commandStates[commandID]?.completedName) || located.node.name;
    const revisionBeforeReset = state.newestDurableRevision;
    confirmReset(`Сбросить выполненное состояние команды "${displayedName}"?`, confirmed => {
      if (!confirmed) return;
      void runSessionStateCommand(
        () => port.resetCommandState({ commandId: commandID, terminalId: terminal.id }),
        'СОСТОЯНИЕ КОМАНДЫ СБРОШЕНО',
        (session, revision) => revision > revisionBeforeReset
          && session.terminals.find(candidate => candidate.id === terminal.id)?.commandStates?.[commandID] === undefined,
      );
    });
  };

  const terminalActivationRequest = (terminal: Terminal): Promise<DesktopCommandResult> =>
    port.requestTerminalActivation({
      hackLevel: terminal.hackLevel,
      introText: terminal.introText,
      terminalId: terminal.id,
      terminalName: terminal.name,
      tree: terminal.root,
    });

  const runTerminalEditorAction = async (
    message: OverseerControllerMessage,
    terminal: Terminal,
  ): Promise<void> => {
    if (message.action === 'apply-settings') {
      const hackLevel = nonNegativeInteger(message.hackLevel);
      if (hackLevel > 5 || typeof message.introText !== 'string') return;
      terminal.hackLevel = hackLevel;
      terminal.introText = message.introText;
      await save();
      if (terminal.id === state.liveTerminalID) {
        await runCoordinationCommand(
          () => port.updateLiveTerminal({ introText: terminal.introText, tree: terminal.root }),
          'АКТИВНЫЙ ТЕРМИНАЛ ОБНОВЛЁН',
          'ОБНОВЛЕНИЕ АКТИВНОГО ТЕРМИНАЛА...',
        );
      }
      publishAuthoring();
      return;
    }
    if (message.action === 'make-live') {
      if (terminal.id === state.liveTerminalID || state.coordination?.broadcast === undefined) return;
      await runCoordinationCommand(
        () => terminalActivationRequest(terminal),
        'АКТИВНЫЙ ТЕРМИНАЛ ВЫБРАН',
        'ПЕРЕКЛЮЧЕНИЕ АКТИВНОГО ТЕРМИНАЛА...',
      );
      return;
    }
    if (message.action === 'reapply-settings') {
      if (terminal.id !== state.liveTerminalID || state.coordination?.broadcast === undefined) return;
      await runCoordinationCommand(
        () => terminalActivationRequest(terminal),
        'НАСТРОЙКИ АКТИВНОГО ТЕРМИНАЛА ПЕРЕПРИМЕНЕНЫ',
        'ПЕРЕПРИМЕНЕНИЕ НАСТРОЕК...',
      );
      return;
    }
    if (message.action === 'publish') {
      if (terminal.id !== state.liveTerminalID) return;
      const result = await runCoordinationCommand(
        () => port.updateLiveTerminal({ introText: terminal.introText, tree: terminal.root }),
        'ИЗМЕНЕНИЯ ОПУБЛИКОВАНЫ У ИГРОКОВ',
        'ПУБЛИКАЦИЯ ИЗМЕНЕНИЙ...',
      );
      if (result?.ok) terminalPublishAcknowledgement += 1;
      publishAuthoring();
      return;
    }
    if (message.action !== 'reset-command-states' || sessionStatePending) return;
    const revisionBeforeReset = state.newestDurableRevision;
    confirmReset(`Сбросить все выполненные состояния команд терминала "${terminal.name}"?`, confirmed => {
      if (!confirmed) return;
      void runSessionStateCommand(
        () => port.resetTerminalCommandStates({ terminalId: terminal.id }),
        'СОСТОЯНИЯ КОМАНД ТЕРМИНАЛА СБРОШЕНЫ',
        (session, revision) => revision > revisionBeforeReset
          && Object.keys(session.terminals.find(candidate => candidate.id === terminal.id)?.commandStates ?? {}).length === 0,
      );
    });
  };

  const terminalName = (terminalID: string): string => state.session?.terminals
    .find(terminal => terminal.id === terminalID)?.name ?? terminalID;

  const groupName = (groups: readonly TerminalGroup[], groupID?: string): string => groups
    .find(group => group.id === groupID)?.name ?? groupID ?? '—';

  const uniqueGroupName = (base: string, groups: readonly TerminalGroup[]): string => {
    const used = new Set(groups.map(group => group.name.trim().toLocaleLowerCase()));
    const encoder = new TextEncoder();
    const truncate = (value: string, byteLimit: number): string => {
      let result = '';
      let bytes = 0;
      for (const symbol of value) {
        const symbolBytes = encoder.encode(symbol).length;
        if (bytes + symbolBytes > byteLimit) break;
        result += symbol;
        bytes += symbolBytes;
      }
      return result;
    };
    const rawBase = base.trim() || 'Terminal';
    let candidate = truncate(rawBase, 256);
    let suffix = 2;
    while (used.has(candidate.toLocaleLowerCase())) {
      const ending = ` (${suffix})`;
      suffix += 1;
      candidate = `${truncate(rawBase, 256 - encoder.encode(ending).length)}${ending}`;
    }
    return candidate;
  };

  const singletonGroup = (terminalID: string, groups: readonly TerminalGroup[]): TerminalGroup => ({
    id: uniqueID('group'),
    name: uniqueGroupName(terminalName(terminalID), groups),
    terminalIds: [terminalID],
  });

  const publishGroupError = (error = '', target?: 'draft' | 'impact'): void => {
    publish({ error, kind: 'terminal-group-error' });
    if (target !== undefined) publish({ error, kind: 'terminal-group-command-feedback', target });
  };

  const showGroupImpact = (impact: TerminalGroupImpact): void => {
    const session = state.session;
    if (session === null) return;
    groupDraft = null;
    publish({ kind: 'terminal-group-draft-dismiss' });
    groupImpact = impact;
    publish({
      candidate: structuredClone(impact.candidate),
      changeKind: impact.kind,
      destinationGroup: impact.destinationGroupName
        ?? groupName(session.terminalGroups, impact.destinationGroupID),
      expectedCoordinationRevision: coordinationRevision(state.coordination),
      expectedSessionRevision: state.newestDurableRevision,
      groups: impact.affectedGroupNames.join(' · ') || '—',
      kind: 'terminal-group-impact-open',
      membership: impact.membership || '—',
      orderAfter: impact.orderAfter.join(' → ') || '—',
      orderBefore: impact.orderBefore.join(' → ') || '—',
      sourceGroup: groupName(session.terminalGroups, impact.sourceGroupID),
      terminals: impact.affectedTerminalIDs.map(terminalName).join(' · ') || '—',
    });
  };

  const openGroupCreate = (): void => {
    if (state.session === null) return;
    groupDraft = { kind: 'create' };
    publishGroupError();
    publish({
      kind: 'terminal-group-draft-open',
      mode: 'create',
      terminals: state.session.terminals.map(terminal => Object.freeze({
        id: terminal.id,
        name: terminal.name,
      })),
    });
  };

  const openGroupRename = (groupID: string): void => {
    const group = state.session?.terminalGroups.find(candidate => candidate.id === groupID);
    if (group === undefined) return;
    groupDraft = { groupID, kind: 'rename' };
    publishGroupError();
    publish({ kind: 'terminal-group-draft-open', mode: 'rename', name: group.name });
  };

  const openTerminalMove = (terminalID: string): void => {
    const session = state.session;
    const source = session?.terminalGroups.find(group => group.terminalIds.includes(terminalID));
    if (session === null || session === undefined || source === undefined) return;
    const destinations: DesktopRecord[] = [];
    if (source.terminalIds.length > 1) {
      destinations.push({ id: 'new-singleton', name: 'НОВАЯ ОДИНОЧНАЯ ГРУППА', newSingleton: true });
    }
    for (const group of session.terminalGroups) {
      if (group.id !== source.id) destinations.push({ id: group.id, name: group.name, newSingleton: false });
    }
    groupDraft = { kind: 'move', sourceGroupID: source.id, terminalID };
    publishGroupError();
    publish({ destinations, kind: 'terminal-group-draft-open', mode: 'move' });
  };

  const showMemberOrderImpact = (groupID: string, terminalID: string, delta: number): void => {
    const session = state.session;
    if (session === null) return;
    const before = structuredClone(session.terminalGroups);
    const candidate = structuredClone(before);
    const group = candidate.find(item => item.id === groupID);
    const index = group?.terminalIds.indexOf(terminalID) ?? -1;
    const next = index + delta;
    if (group === undefined || index < 0 || next < 0 || next >= group.terminalIds.length) return;
    const adjacentID = group.terminalIds[next];
    if (adjacentID === undefined) return;
    group.terminalIds[next] = terminalID;
    group.terminalIds[index] = adjacentID;
    showGroupImpact({
      affectedGroupNames: [group.name],
      affectedTerminalIDs: [terminalID, adjacentID],
      candidate,
      kind: 'ИЗМЕНЕНИЕ ПОРЯДКА',
      membership: `${group.name}: ${group.terminalIds.map(terminalName).join(' → ')}`,
      orderAfter: group.terminalIds.map(terminalName),
      orderBefore: before.find(item => item.id === groupID)?.terminalIds.map(terminalName) ?? [],
    });
  };

  const showGroupOrderImpact = (groupID: string, delta: number): void => {
    const session = state.session;
    if (session === null) return;
    const before = structuredClone(session.terminalGroups);
    const candidate = structuredClone(before);
    const index = candidate.findIndex(group => group.id === groupID);
    const next = index + delta;
    if (index < 0 || next < 0 || next >= candidate.length) return;
    const current = candidate[index];
    const adjacent = candidate[next];
    if (current === undefined || adjacent === undefined) return;
    candidate[index] = adjacent;
    candidate[next] = current;
    showGroupImpact({
      affectedGroupNames: [adjacent.name, current.name],
      affectedTerminalIDs: [],
      candidate,
      kind: 'ИЗМЕНЕНИЕ ПОРЯДКА',
      membership: candidate.map(group => group.name).join(' → '),
      orderAfter: candidate.map(group => group.name),
      orderBefore: before.map(group => group.name),
    });
  };

  const showGroupDissolution = (groupID: string): void => {
    const session = state.session;
    if (session === null) return;
    const before = structuredClone(session.terminalGroups);
    const group = before.find(candidate => candidate.id === groupID);
    if (group === undefined) return;
    if (group.terminalIds.length === 1) {
      publishGroupError('ОДИНОЧНУЮ ГРУППУ НЕЛЬЗЯ РАСФОРМИРОВАТЬ: ТЕРМИНАЛ ДОЛЖЕН ОСТАТЬСЯ В ГРУППЕ');
      return;
    }
    const candidate = before.filter(candidateGroup => candidateGroup.id !== groupID);
    const singletons = group.terminalIds.map(terminalID => {
      const singleton = singletonGroup(terminalID, candidate);
      candidate.push(singleton);
      return singleton;
    });
    showGroupImpact({
      affectedGroupNames: [group.name, ...singletons.map(singleton => singleton.name)],
      affectedTerminalIDs: [...group.terminalIds],
      candidate,
      kind: 'РАСФОРМИРОВАНИЕ ГРУППЫ',
      membership: singletons.map(singleton =>
        `${singleton.name}: ${terminalName(singleton.terminalIds[0] ?? '')}`).join(' · '),
      orderAfter: group.terminalIds.map(terminalName),
      orderBefore: group.terminalIds.map(terminalName),
      sourceGroupID: group.id,
    });
  };

  const reviewGroupDraft = (message: OverseerControllerMessage): void => {
    const session = state.session;
    if (session === null || groupDraft === null) return;
    const before = structuredClone(session.terminalGroups);
    if (groupDraft.kind === 'create') {
      const name = text(message.name).trim();
      const knownIDs = new Set(session.terminals.map(terminal => terminal.id));
      const selectedIDs = Array.isArray(message.selectedTerminalIds)
        ? [...new Set(message.selectedTerminalIds.filter(
          (terminalID): terminalID is string => typeof terminalID === 'string' && knownIDs.has(terminalID),
        ))]
        : [];
      if (name === '' || selectedIDs.length < 2) {
        publishGroupError('УКАЖИТЕ УНИКАЛЬНОЕ НАЗВАНИЕ И ВЫБЕРИТЕ НЕ МЕНЕЕ ДВУХ ТЕРМИНАЛОВ', 'draft');
        return;
      }
      const selected = new Set(selectedIDs);
      const sourceNames = before
        .filter(group => group.terminalIds.some(terminalID => selected.has(terminalID)))
        .map(group => group.name);
      const candidate = before
        .map(group => ({
          ...group,
          terminalIds: group.terminalIds.filter(terminalID => !selected.has(terminalID)),
        }))
        .filter(group => group.terminalIds.length > 0);
      if (candidate.some(group => group.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
        publishGroupError('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ', 'draft');
        return;
      }
      candidate.push({ id: groupDraft.groupID ?? uniqueID('group'), name, terminalIds: selectedIDs });
      showGroupImpact({
        affectedGroupNames: [...new Set([...sourceNames, name])],
        affectedTerminalIDs: selectedIDs,
        candidate,
        kind: 'СОЗДАНИЕ ГРУППЫ',
        membership: `${name}: ${selectedIDs.map(terminalName).join(' → ')}`,
        orderAfter: candidate.flatMap(group => group.terminalIds).map(terminalName),
        orderBefore: before.flatMap(group => group.terminalIds).map(terminalName),
      });
      return;
    }
    if (groupDraft.kind !== 'move' || groupDraft.terminalID === undefined
      || groupDraft.sourceGroupID === undefined) return;
    const destinationGroupID = text(message.destinationGroupId);
    if (destinationGroupID === '') {
      publishGroupError('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ', 'draft');
      return;
    }
    const terminalID = groupDraft.terminalID;
    const sourceGroupID = groupDraft.sourceGroupID;
    const candidate = before
      .map(group => ({
        ...group,
        terminalIds: group.terminalIds.filter(candidateID => candidateID !== terminalID),
      }))
      .filter(group => group.terminalIds.length > 0);
    if (destinationGroupID === 'new-singleton') {
      const singleton = singletonGroup(terminalID, candidate);
      candidate.push(singleton);
      const source = candidate.find(group => group.id === sourceGroupID);
      showGroupImpact({
        affectedGroupNames: [groupName(before, sourceGroupID), singleton.name],
        affectedTerminalIDs: [terminalID],
        candidate,
        destinationGroupID: singleton.id,
        destinationGroupName: singleton.name,
        kind: 'ОТДЕЛЕНИЕ ТЕРМИНАЛА',
        membership: `${source?.name ?? '—'}: ${source?.terminalIds.map(terminalName).join(' → ') ?? '—'} · ${singleton.name}: ${terminalName(terminalID)}`,
        orderAfter: [...(source?.terminalIds ?? []), terminalID].map(terminalName),
        orderBefore: before.find(group => group.id === sourceGroupID)?.terminalIds.map(terminalName) ?? [],
        sourceGroupID,
      });
      return;
    }
    const destination = candidate.find(group => group.id === destinationGroupID);
    if (destination === undefined) {
      publishGroupError('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ', 'draft');
      return;
    }
    destination.terminalIds.push(terminalID);
    showGroupImpact({
      affectedGroupNames: [groupName(before, sourceGroupID), groupName(before, destinationGroupID)],
      affectedTerminalIDs: [terminalID],
      candidate,
      destinationGroupID,
      kind: 'ПЕРЕМЕЩЕНИЕ ТЕРМИНАЛА',
      membership: candidate.map(group =>
        `${group.name}: ${group.terminalIds.map(terminalName).join(' → ')}`).join(' · '),
      orderAfter: destination.terminalIds.map(terminalName),
      orderBefore: before.find(group => group.id === sourceGroupID)?.terminalIds.map(terminalName) ?? [],
      sourceGroupID,
    });
  };

  const requestGroupRename = (message: OverseerControllerMessage): void => {
    const session = state.session;
    const group = session?.terminalGroups.find(candidate => candidate.id === groupDraft?.groupID);
    const name = text(message.name).trim();
    if (session === null || session === undefined || group === undefined || name === '') {
      publishGroupError('НАЗВАНИЕ ГРУППЫ НЕ ДОЛЖНО БЫТЬ ПУСТЫМ', 'draft');
      return;
    }
    if (session.terminalGroups.some(candidate => candidate.id !== group.id
      && candidate.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
      publishGroupError('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ', 'draft');
      return;
    }
    const candidate = structuredClone(session.terminalGroups);
    const renamed = candidate.find(item => item.id === group.id);
    if (renamed === undefined) return;
    renamed.name = name;
    publish({
      candidate,
      expectedCoordinationRevision: coordinationRevision(state.coordination),
      expectedSessionRevision: state.newestDurableRevision,
      kind: 'terminal-group-draft-submit',
    });
  };

  const terminalGroupRejections = (error: string): DesktopRecord[] => {
    const pattern = /terminal transition command "([^"]+)" in terminal "([^"]+)" targets terminal "([^"]+)" and crosses groups "([^"]+)" and "([^"]+)"/g;
    return [...error.matchAll(pattern)].map(match => Object.freeze({
      commandID: match[1] ?? '',
      sourceGroupID: match[4] ?? '',
      sourceTerminalID: match[2] ?? '',
      targetGroupID: match[5] ?? '',
      targetTerminalID: match[3] ?? '',
    }));
  };

  const applyGroupCommandResult = (message: OverseerControllerMessage): void => {
    const source = message.source === 'draft' ? 'draft' : 'impact';
    const result = mutableRecord(message.result) ?? { error: 'НЕ УДАЛОСЬ ИЗМЕНИТЬ ГРУППЫ ТЕРМИНАЛОВ', ok: false };
    const session = parseSession(result.session);
    if (session !== null) state.session = session;
    state.newestDurableRevision = Math.max(
      state.newestDurableRevision,
      nonNegativeInteger(result.sessionRevision),
    );
    if (result.coordinationState !== undefined) applyCoordination(result.coordinationState);
    if (result.ok !== true) {
      const rawError = text(result.error) || 'НЕ УДАЛОСЬ ИЗМЕНИТЬ ГРУППЫ ТЕРМИНАЛОВ';
      const rejections = terminalGroupRejections(rawError);
      const rejectionGroups = groupImpact?.candidate ?? state.session?.terminalGroups ?? [];
      const error = rejections.length === 0
        ? rawError
        : `ПЕРЕХОДЫ МЕЖДУ ГРУППАМИ НЕДОПУСТИМЫ: ${rejections.map(rejection => {
            const sourceTerminalID = text(rejection.sourceTerminalID);
            const targetTerminalID = text(rejection.targetTerminalID);
            const sourceGroupID = text(rejection.sourceGroupID);
            const targetGroupID = text(rejection.targetGroupID);
            return `${text(rejection.commandID)} in terminal ${sourceTerminalID} (${terminalName(sourceTerminalID)}) targets terminal ${targetTerminalID} (${terminalName(targetTerminalID)}) and crosses groups ${sourceGroupID} (${groupName(rejectionGroups, sourceGroupID)}) and ${targetGroupID} (${groupName(rejectionGroups, targetGroupID)})`;
          }).join(' · ')}`;
      const canAmend = source === 'impact' && groupImpact !== null && rejections.length > 0;
      if (canAmend && groupImpact !== null) groupImpact.rejections = rejections;
      publishGroupError(error);
      publish({
        canAmend,
        close: source === 'impact' && !canAmend,
        error,
        kind: 'terminal-group-command-feedback',
        target: source,
      });
      if (source === 'impact' && !canAmend) groupImpact = null;
      publishAuthoring();
      return;
    }
    publishGroupError();
    publish({ error: '', kind: 'terminal-group-command-feedback', close: true, target: source });
    if (source === 'draft') groupDraft = null;
    else groupImpact = null;
    publish({
      error: '',
      kind: 'session-save-status',
      revision: state.newestDurableRevision,
      text: `ГРУППЫ СОХРАНЕНЫ · ревизия ${state.newestDurableRevision}`,
    });
    publishAuthoring();
  };

  const amendGroupImpact = (): void => {
    const impact = groupImpact;
    const session = state.session;
    if (impact === null || session === null || impact.rejections === undefined
      || impact.rejections.length === 0) return;
    const involvedGroupIDs = new Set(impact.rejections.flatMap(rejection => [
      text(rejection.sourceGroupID),
      text(rejection.targetGroupID),
    ]));
    const selectedTerminalIDs = impact.candidate
      .filter(group => involvedGroupIDs.has(group.id))
      .flatMap(group => group.terminalIds);
    const preferred = impact.candidate.find(group => group.id === impact.destinationGroupID)
      ?? impact.candidate.find(group => group.id === text(impact.rejections?.[0]?.sourceGroupID));
    groupImpact = null;
    groupDraft = preferred === undefined
      ? { kind: 'create' }
      : { groupID: preferred.id, kind: 'create' };
    publish({ kind: 'terminal-group-impact-dismiss' });
    publish({
      kind: 'terminal-group-draft-open',
      mode: 'create',
      name: preferred?.name ?? '',
      selectedTerminalIds: selectedTerminalIDs,
      terminals: session.terminals.map(terminal => Object.freeze({ id: terminal.id, name: terminal.name })),
    });
  };

  const inboundTerminalTransitions = (targetTerminalID: string): string[] => {
    const inbound: string[] = [];
    const visit = (terminal: Terminal, node: TerminalNode): void => {
      const transition = mutableRecord(node.terminalTransition);
      if (transition?.targetTerminalId === targetTerminalID) inbound.push(`${terminal.name}: ${node.name}`);
      for (const child of node.children ?? []) visit(terminal, child);
    };
    for (const terminal of state.session?.terminals ?? []) visit(terminal, terminal.root);
    return inbound;
  };

  const deleteTerminal = (terminal: Terminal): void => {
    const session = state.session;
    if (session === null) return;
    const pendingSwitch = mutableRecord(state.coordination?.pendingSwitch);
    if (state.liveTerminalID === terminal.id || pendingSwitch?.sourceTerminalId === terminal.id) {
      coordinationStatus = '';
      coordinationError = 'АКТИВНЫЙ ИЛИ СОХРАНЁННЫЙ ТЕРМИНАЛ НЕЛЬЗЯ УДАЛИТЬ';
      publishCoordination();
      return;
    }
    const inbound = inboundTerminalTransitions(terminal.id);
    if (inbound.length > 0) {
      coordinationStatus = '';
      coordinationError = `ТЕРМИНАЛ ИСПОЛЬЗУЕТСЯ КОМАНДАМИ ПЕРЕХОДА: ${inbound.join(', ')}`;
      publishCoordination();
      return;
    }
    const index = session.terminals.findIndex(candidate => candidate.id === terminal.id);
    if (index >= 0) session.terminals.splice(index, 1);
    session.terminalGroups = session.terminalGroups
      .map(group => ({
        ...group,
        terminalIds: group.terminalIds.filter(terminalID => terminalID !== terminal.id),
      }))
      .filter(group => group.terminalIds.length > 0);
    if (state.editTerminalID === terminal.id) {
      state.editTerminalID = session.terminals[0]?.id ?? null;
      state.selectedNodeID = null;
      state.expanded = new Set(['root']);
    }
    void save();
    publishAuthoring();
  };

  const applyPublicAccessSnapshot = (value: unknown): void => {
    const next = mutableRecord(value);
    if (next === null) return;
    const nextStatus = mutableRecord(next.status);
    const currentStatus = mutableRecord(publicAccessSnapshot?.status);
    const nextPreferences = mutableRecord(next.preferences);
    const currentPreferences = mutableRecord(publicAccessSnapshot?.preferences);
    const nextGeneration = nonNegativeInteger(nextStatus?.generation ?? next.generation);
    const currentGeneration = nonNegativeInteger(currentStatus?.generation ?? publicAccessSnapshot?.generation);
    const nextRevision = nonNegativeInteger(
      nextStatus?.settingsRevision ?? nextPreferences?.revision ?? next.settingsRevision,
    );
    const currentRevision = nonNegativeInteger(
      currentStatus?.settingsRevision ?? currentPreferences?.revision ?? publicAccessSnapshot?.settingsRevision,
    );
    if (publicAccessSnapshot !== null && (nextGeneration < currentGeneration
      || (nextGeneration === currentGeneration && nextRevision < currentRevision))) return;
    publicAccessSnapshot = Object.freeze(next);
    publish({ kind: 'public-access-settings-snapshot', pending: false, snapshot: publicAccessSnapshot });
  };

  const dispatch = (value: unknown): boolean => {
    if (disposed) return false;
    const message = controllerMessage(value);
    if (message === null) return false;
    if (message.kind === 'session-document-loaded') {
      loadSession(message);
      return true;
    }
    if (message.kind === 'terminal-selection-request') {
      if (message.revision !== state.terminalAuthoringRevision || typeof message.terminalID !== 'string') return true;
      if (!state.session?.terminals.some(terminal => terminal.id === message.terminalID)) return true;
      state.editTerminalID = message.terminalID;
      state.selectedNodeID = null;
      state.expanded = new Set(['root']);
      publishAuthoring();
      return true;
    }
    if (message.kind === 'create-terminal-open-request') {
      if (message.revision !== state.terminalAuthoringRevision || state.session === null) return true;
      createTerminalOpen = true;
      publishAuthoring();
      return true;
    }
    if (message.kind === 'terminal-group-create-request') {
      if (message.revision !== state.terminalAuthoringRevision || state.session === null) return true;
      openGroupCreate();
      return true;
    }
    if (message.kind === 'create-terminal-action-request') {
      if (message.revision !== state.terminalAuthoringRevision || state.session === null) return true;
      if (message.action === 'cancel') {
        createTerminalOpen = false;
        publishAuthoring();
        return true;
      }
      const name = text(message.name).trim();
      if (message.action !== 'create' || name === '' || createTerminalPending) return true;
      createTerminalPending = true;
      const id = uniqueID('t');
      const terminal: Terminal = {
        hackLevel: 0,
        id,
        introText: '',
        name,
        root: { children: [], id: 'root', name: 'ROOT', type: 'folder' },
      };
      state.session.terminals.push(terminal);
      state.session.terminalGroups.push({ id: uniqueID('g'), name, terminalIds: [id] });
      state.editTerminalID = id;
      createTerminalOpen = false;
      createTerminalPending = false;
      void save();
      publishAuthoring();
      publish({ kind: 'terminal-editor-focus-settings' });
      return true;
    }
    if (message.kind === 'terminal-editor-action-request') {
      const terminal = currentTerminal();
      if (terminal === null || message.revision !== state.terminalAuthoringRevision
        || message.terminalID !== terminal.id) return true;
      void runTerminalEditorAction(message, terminal);
      return true;
    }
    if (message.kind === 'terminal-tree-action-request') {
      const terminal = currentTerminal();
      if (terminal === null || message.revision !== state.terminalAuthoringRevision
        || message.terminalID !== terminal.id) return true;
      void runTerminalTreeAction(message, terminal);
      return true;
    }
    if (message.kind === 'command-state-reset-resolved') {
      publish(message);
      if (resetConfirmation === null || message.requestId !== resetConfirmation.requestID) return true;
      const pending = resetConfirmation;
      resetConfirmation = null;
      pending.complete(message.confirmed === true);
      return true;
    }
    if (message.kind === 'terminal-switch-resolved') {
      const result = mutableRecord(message.result);
      if (result?.ok !== true) return true;
      if (result.state !== undefined) applyCoordination(result.state);
      coordinationStatus = message.decision === 'cancel' ? 'ПЕРЕКЛЮЧЕНИЕ ОТМЕНЕНО' : 'РЕШЕНИЕ ПРИМЕНЕНО';
      coordinationError = '';
      publishCoordination();
      publishAuthoring();
      return true;
    }
    if (message.kind === 'terminal-action-request') {
      if (message.revision !== state.terminalAuthoringRevision || typeof message.terminalID !== 'string') return true;
      const terminal = state.session?.terminals.find(candidate => candidate.id === message.terminalID);
      if (terminal === undefined) return true;
      if (message.action === 'rename-terminal' && typeof message.name === 'string' && message.name.trim() !== '') {
        terminal.name = message.name.trim();
        void save();
        publishAuthoring();
      } else {
        const group = state.session?.terminalGroups.find(candidate => candidate.terminalIds.includes(terminal.id));
        const memberIndex = group?.terminalIds.indexOf(terminal.id) ?? -1;
        groupFocusOwner = Object.freeze({ ownerID: terminal.id, scope: 'terminal' });
        if (message.action === 'move-terminal') openTerminalMove(terminal.id);
        else if (message.action === 'move-terminal-up' && group !== undefined && memberIndex > 0) {
          showMemberOrderImpact(group.id, terminal.id, -1);
        } else if (message.action === 'move-terminal-down' && group !== undefined
          && memberIndex < group.terminalIds.length - 1) {
          showMemberOrderImpact(group.id, terminal.id, 1);
        } else if (message.action === 'delete-terminal') deleteTerminal(terminal);
        if (groupDraft === null && groupImpact === null) groupFocusOwner = null;
      }
      return true;
    }
    if (message.kind === 'terminal-group-action-request') {
      if (message.revision !== state.terminalAuthoringRevision || typeof message.groupID !== 'string') return true;
      const groupIndex = state.session?.terminalGroups.findIndex(group => group.id === message.groupID) ?? -1;
      if (groupIndex < 0) return true;
      groupFocusOwner = Object.freeze({ ownerID: message.groupID, scope: 'terminal-group' });
      if (message.action === 'rename-terminal-group') openGroupRename(message.groupID);
      else if (message.action === 'move-terminal-group-up' && groupIndex > 0) {
        showGroupOrderImpact(message.groupID, -1);
      } else if (message.action === 'move-terminal-group-down'
        && groupIndex < (state.session?.terminalGroups.length ?? 0) - 1) {
        showGroupOrderImpact(message.groupID, 1);
      } else if (message.action === 'dissolve-terminal-group') showGroupDissolution(message.groupID);
      if (groupDraft === null && groupImpact === null) groupFocusOwner = null;
      return true;
    }
    if (message.kind === 'terminal-group-draft-reviewed') {
      reviewGroupDraft(message);
      return true;
    }
    if (message.kind === 'terminal-group-rename-requested') {
      requestGroupRename(message);
      return true;
    }
    if (message.kind === 'terminal-group-draft-closed') {
      groupDraft = null;
      restoreGroupFocus();
      return true;
    }
    if (message.kind === 'terminal-group-impact-closed') {
      groupImpact = null;
      restoreGroupFocus();
      return true;
    }
    if (message.kind === 'terminal-group-impact-amend-requested') {
      amendGroupImpact();
      return true;
    }
    if (message.kind === 'terminal-group-command-finished') {
      applyGroupCommandResult(message);
      return true;
    }
    if (message.kind === 'player-management-delete-request') {
      if (typeof message.characterId === 'string' && typeof message.name === 'string'
        && !coordinationPending && !isRecord(state.coordination?.broadcast)
        && state.coordination?.playerConfig !== undefined) {
        publish({
          characterId: message.characterId,
          expectedRevision: coordinationRevision(state.coordination),
          kind: 'player-delete-requested',
          name: message.name,
        });
      }
      return true;
    }
    if (message.kind === 'player-delete-focus-request') {
      if (typeof message.characterId === 'string') {
        publish({ characterId: message.characterId, kind: 'player-management-delete-focus-request' });
      }
      return true;
    }
    if (message.kind === 'player-delete-finished') {
      const result = mutableRecord(message.result) ?? { error: 'НЕ УДАЛОСЬ УДАЛИТЬ ИГРОКА', ok: false };
      coordinationPending = false;
      if (result.state !== undefined) applyCoordination(result.state);
      const feedback = result.ok === true ? 'ИГРОК УДАЛЁН' : text(result.error) || 'НЕ УДАЛОСЬ УДАЛИТЬ ИГРОКА';
      coordinationStatus = result.ok === true ? feedback : '';
      coordinationError = result.ok === true ? '' : feedback;
      publish({
        error: result.ok === true ? '' : feedback,
        kind: 'player-management-feedback',
        status: result.ok === true ? feedback : '',
      });
      publishCoordination();
      return true;
    }
    if (message.kind === 'public-access-snapshot') {
      applyPublicAccessSnapshot(message.snapshot);
      return true;
    }
    if (message.kind === 'public-access-settings-command-finished') {
      const result = mutableRecord(message.result);
      applyPublicAccessSnapshot(result?.snapshot ?? publicAccessSnapshot);
      return true;
    }
    if (message.kind === 'command-execution-finished') {
      const result = mutableRecord(message.result);
      if (result?.state !== undefined) applyCoordination(result.state);
      coordinationPending = false;
      coordinationStatus = result?.ok === true ? 'КОМАНДА ВЫПОЛНЕНА' : '';
      coordinationError = result?.ok === true
        ? ''
        : 'СОСТОЯНИЕ КОМАНДЫ НЕ ИЗМЕНЕНО · НЕ УДАЛОСЬ СОХРАНИТЬ РЕЗУЛЬТАТ';
      publishCoordination();
      publishAuthoring();
      return true;
    }
    if (message.kind.endsWith('-started')) {
      const expectedRevision = nonNegativeInteger(message.expectedRevision);
      if (expectedRevision !== coordinationRevision(state.coordination) || coordinationPending) return true;
      coordinationPending = true;
      coordinationStatus = text(message.status) || 'ВЫПОЛНЕНИЕ ОПЕРАЦИИ...';
      coordinationError = '';
      publishCoordination();
      publishAuthoring();
      return true;
    }
    if (applyResult(message)) return true;
    const relayedKinds = new Set([
      'command-state-reset-required',
      'logical-session-open-request',
      'player-delete-requested',
      'player-management-open-request',
      'public-access-credentials-share',
      'public-access-generated-password-open',
      'public-access-player-credentials-open',
      'public-access-provider-token-open',
      'public-access-settings-copy-status',
      'public-access-settings-focus-player',
      'public-access-settings-focus-provider',
      'public-access-settings-open',
      'terminal-group-command-feedback',
      'terminal-group-draft-dismiss',
      'terminal-group-draft-open',
      'terminal-group-impact-dismiss',
      'terminal-group-impact-open',
      'terminal-selection-focus-request',
      'terminal-switch-dismissed',
      'terminal-switch-required',
    ]);
    if (relayedKinds.has(message.kind)) return publish(message);
    return true;
  };

  releases.push(port.onCoordinationState(applyCoordination));
  releases.push(port.onSessionState(event => {
    const revision = nonNegativeInteger(event.revision);
    if (revision <= state.newestDurableRevision) return;
    const session = parseSession(event.session);
    if (session === null) return;
    state.session = session;
    state.newestDurableRevision = revision;
    publishAuthoring();
    publish({
      error: '',
      kind: 'session-save-status',
      revision,
      text: `СОСТОЯНИЕ СЕССИИ ОБНОВЛЕНО · ревизия ${revision}`,
    });
  }));

  return Object.freeze({
    dispatch,
    dispose: () => {
      if (disposed) return;
      disposed = true;
      for (const release of releases.splice(0)) release();
      listeners.clear();
    },
    publish,
    subscribeState: (listener: ControllerListener) => subscribe(listeners, listener),
  });
}
