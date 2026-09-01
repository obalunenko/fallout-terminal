<script setup lang="ts">
import { computed, inject, nextTick, onUnmounted, ref, shallowRef } from 'vue';

import { overseerControllerKey } from '../controllers/overseer-controller.js';
import type { DesktopRecord } from '../models/overseer-view-state.js';
import NodeEditor from './NodeEditor.vue';
import TerminalTreeNode from './TerminalTreeNode.vue';

export interface TerminalTreeExecution {
  readonly completedName: string;
  readonly resultText: string;
}

export interface TerminalTreeNodeView {
  readonly children: readonly TerminalTreeNodeView[];
  readonly completed: boolean;
  readonly description: string;
  readonly displayName: string;
  readonly execution: TerminalTreeExecution | null;
  readonly id: string;
  readonly name: string;
  readonly stateChange: Readonly<{ completedName: string; confirmationText: string }> | null;
  readonly terminalTransition: Readonly<{ targetTerminalId: string }> | null;
  readonly text: string;
  readonly type: 'folder' | 'command' | 'entry';
}

export interface TerminalTreeOption {
  readonly id: string;
  readonly name: string;
}

interface TerminalTreeSnapshot {
  readonly addTargetName: string;
  readonly expandedIDs: readonly string[];
  readonly pending: boolean;
  readonly root: TerminalTreeNodeView;
  readonly selectedNodeID: string | null;
  readonly terminalID: string;
  readonly terminalOptions: readonly TerminalTreeOption[];
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function string(value: unknown): string | null {
  return typeof value === 'string' ? value : null;
}

function node(value: unknown, ids: Set<string>): TerminalTreeNodeView | null {
  if (!isRecord(value) || typeof value.id !== 'string' || value.id === '' || ids.has(value.id)
    || typeof value.name !== 'string'
    || (value.type !== 'folder' && value.type !== 'command' && value.type !== 'entry')) return null;
  ids.add(value.id);
  const rawChildren = value.children === undefined ? [] : value.children;
  if (!Array.isArray(rawChildren) || (value.type !== 'folder' && rawChildren.length !== 0)) return null;
  const children: TerminalTreeNodeView[] = [];
  for (const rawChild of rawChildren) {
    const child = node(rawChild, ids);
    if (child === null) return null;
    children.push(child);
  }
  const executionRecord = value.execution;
  const execution = executionRecord === null ? null
    : isRecord(executionRecord) && typeof executionRecord.completedName === 'string'
      && typeof executionRecord.resultText === 'string'
      ? Object.freeze({ completedName: executionRecord.completedName, resultText: executionRecord.resultText })
      : null;
  if (executionRecord !== null && execution === null) return null;
  const stateChangeRecord = value.stateChange;
  const stateChange = stateChangeRecord === null ? null
    : isRecord(stateChangeRecord) && typeof stateChangeRecord.completedName === 'string'
      && typeof stateChangeRecord.confirmationText === 'string'
      ? Object.freeze({ completedName: stateChangeRecord.completedName, confirmationText: stateChangeRecord.confirmationText })
      : null;
  if (stateChangeRecord !== null && stateChange === null) return null;
  const transitionRecord = value.terminalTransition;
  const terminalTransition = transitionRecord === null ? null
    : isRecord(transitionRecord) && typeof transitionRecord.targetTerminalId === 'string'
      ? Object.freeze({ targetTerminalId: transitionRecord.targetTerminalId })
      : null;
  if (transitionRecord !== null && terminalTransition === null) return null;
  return Object.freeze({
    children: Object.freeze(children),
    completed: execution !== null,
    description: string(value.description) ?? '',
    displayName: string(value.displayName) ?? value.name,
    execution,
    id: value.id,
    name: value.name,
    stateChange,
    terminalTransition,
    text: string(value.text) ?? '',
    type: value.type,
  });
}

function snapshot(message: DesktopRecord): TerminalTreeSnapshot | null {
  if (typeof message.terminalID !== 'string' || message.terminalID === ''
    || typeof message.addTargetName !== 'string' || !Array.isArray(message.expandedIDs)
    || !message.expandedIDs.every(id => typeof id === 'string')
    || !Array.isArray(message.terminalOptions)) return null;
  const root = node(message.root, new Set());
  if (root === null || root.id !== 'root' || root.type !== 'folder') return null;
  const selectedNodeID = message.selectedNodeID === null ? null : string(message.selectedNodeID);
  if (message.selectedNodeID !== null && selectedNodeID === null) return null;
  const terminalOptions: TerminalTreeOption[] = [];
  for (const option of message.terminalOptions) {
    if (!isRecord(option) || typeof option.id !== 'string' || typeof option.name !== 'string') return null;
    terminalOptions.push(Object.freeze({ id: option.id, name: option.name }));
  }
  return Object.freeze({
    addTargetName: message.addTargetName,
    expandedIDs: Object.freeze([...new Set(message.expandedIDs)]),
    pending: message.pending === true,
    root,
    selectedNodeID,
    terminalID: message.terminalID,
    terminalOptions: Object.freeze(terminalOptions),
  });
}

function findNode(root: TerminalTreeNodeView | null, nodeID: string | null): TerminalTreeNodeView | null {
  if (root === null || nodeID === null) return null;
  if (root.id === nodeID) return root;
  for (const child of root.children) {
    const found = findNode(child, nodeID);
    if (found !== null) return found;
  }
  return null;
}

const controller = inject(overseerControllerKey, null);
const current = shallowRef<TerminalTreeSnapshot | null>(null);
const revision = ref(-1);
const treeViewElement = ref<HTMLElement | null>(null);
const expandedIDs = computed<ReadonlySet<string>>(() => new Set(current.value?.expandedIDs ?? []));
const selectedNode = computed(() => findNode(current.value?.root ?? null, current.value?.selectedNodeID ?? null));

const release = controller?.subscribeState(message => {
  if (message.kind === 'terminal-tree-focus-request') {
    if (typeof message.nodeID !== 'string') return;
    const nodeID = message.nodeID;
    void nextTick(() => treeViewElement.value
      ?.querySelector<HTMLElement>(`[data-node-key="${CSS.escape(nodeID)}"]`)?.focus());
    return;
  }
  if (message.kind !== 'terminal-tree-snapshot'
    || !Number.isSafeInteger(message.revision) || Number(message.revision) <= revision.value) return;
  if (message.available === false) {
    revision.value = Number(message.revision);
    current.value = null;
    return;
  }
  const next = snapshot(message);
  if (next === null) return;
  revision.value = Number(message.revision);
  current.value = next;
});

function request(action: string, extra: DesktopRecord = {}): void {
  controller?.dispatch({
    action,
    ...extra,
    kind: 'terminal-tree-action-request',
    revision: revision.value,
    terminalID: current.value?.terminalID ?? '',
  });
}

function apply(draft: Readonly<Record<string, unknown>>): void {
  request('apply-node', { draft, nodeID: current.value?.selectedNodeID ?? '' });
}

onUnmounted(() => {
  release?.();
  current.value = null;
  treeViewElement.value = null;
});
</script>

<template>
  <Teleport to="#terminalTreeVueLeaf">
    <div class="tree-toolbar" id="treeToolbar">
      <button class="btn btn-mini" id="btnAddFolder" type="button" :disabled="current === null" @click="request('add-node', { nodeType: 'folder' })">+ ПАПКА</button>
      <button class="btn btn-mini" id="btnAddCommand" type="button" :disabled="current === null" @click="request('add-node', { nodeType: 'command' })">+ КОМАНДА</button>
      <button class="btn btn-mini" id="btnAddEntry" type="button" :disabled="current === null" @click="request('add-node', { nodeType: 'entry' })">+ ЗАПИСЬ</button>
      <span class="toolbar-hint" id="toolbarHint">{{ current === null ? '' : `Добавление в: ${current.addTargetName}` }}</span>
    </div>
    <div id="treeView" ref="treeViewElement" class="tree-view" v-bind="{ 'data-tree-revision': String(revision) }">
      <div v-if="current === null" class="tree-empty-hint">Нет терминала — создайте его слева</div>
      <TerminalTreeNode
        v-else
        :expanded-i-ds="expandedIDs"
        is-root
        :node="current.root"
        :selected-node-i-d="current.selectedNodeID"
        @select="request('select-node', { nodeID: $event })"
        @toggle="request('toggle-node', { nodeID: $event })"
      />
    </div>
  </Teleport>
  <Teleport to="#nodeEditorVueLeaf">
    <div id="nodeForm" class="node-form" v-bind="{ 'data-editor-revision': String(revision) }">
      <NodeEditor
        :key="selectedNode?.id ?? 'empty'"
        :node="selectedNode"
        :pending="current?.pending ?? false"
        :terminal-i-d="current?.terminalID ?? ''"
        :terminal-options="current?.terminalOptions ?? []"
        @apply="apply"
        @delete="request('delete-node', { nodeID: current?.selectedNodeID ?? '' })"
        @reset="request('reset-command-state', { nodeID: current?.selectedNodeID ?? '' })"
      />
    </div>
  </Teleport>
</template>
