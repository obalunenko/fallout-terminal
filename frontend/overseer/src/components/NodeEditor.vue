<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';

import type { TerminalTreeNodeView, TerminalTreeOption } from './TerminalTree.vue';

type CommandMode = 'ordinary' | 'state-change' | 'terminal-transition';

const props = defineProps<{
  readonly node: TerminalTreeNodeView | null;
  readonly pending: boolean;
  readonly terminalID: string;
  readonly terminalOptions: readonly TerminalTreeOption[];
}>();

const emit = defineEmits<{
  apply: [draft: Readonly<Record<string, unknown>>];
  delete: [];
  reset: [];
}>();

const name = ref('');
const mode = ref<CommandMode>('ordinary');
const completedName = ref('');
const confirmationText = ref('');
const targetTerminalID = ref('');
const text = ref('');
const error = ref('');
const nameInput = ref<HTMLInputElement | null>(null);
const completedNameInput = ref<HTMLInputElement | null>(null);
const confirmationTextInput = ref<HTMLTextAreaElement | null>(null);
const targetInput = ref<HTMLSelectElement | null>(null);
const textInput = ref<HTMLTextAreaElement | null>(null);

function nodeMode(node: TerminalTreeNodeView): CommandMode {
  if (node.stateChange !== null) return 'state-change';
  if (node.terminalTransition !== null) return 'terminal-transition';
  return 'ordinary';
}

function load(node: TerminalTreeNodeView | null): void {
  name.value = node?.name ?? '';
  mode.value = node?.type === 'command' ? nodeMode(node) : 'ordinary';
  completedName.value = node?.stateChange?.completedName ?? '';
  confirmationText.value = node?.stateChange?.confirmationText ?? '';
  targetTerminalID.value = node?.terminalTransition?.targetTerminalId ?? '';
  text.value = node?.type === 'entry' ? node.description : node?.text ?? '';
  error.value = '';
}

watch(() => props.node, load, { immediate: true });
watch(mode, () => { error.value = ''; });

function reject(message: string, field: { focus(): void } | null): void {
  error.value = message;
  void nextTick(() => field?.focus());
}

function apply(): void {
  const node = props.node;
  if (node === null || node.id === 'root') return;
  const trimmedName = name.value.trim();
  if (trimmedName === '') {
    reject(node.type === 'command' ? 'УКАЖИТЕ ИСХОДНОЕ НАЗВАНИЕ КОМАНДЫ' : 'УКАЖИТЕ НАЗВАНИЕ', nameInput.value);
    return;
  }
  if (node.type === 'command' && mode.value === 'state-change') {
    if (completedName.value.trim() === '') {
      reject('УКАЖИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ', completedNameInput.value);
      return;
    }
    if (confirmationText.value.trim() === '') {
      reject('УКАЖИТЕ ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ', confirmationTextInput.value);
      return;
    }
    if (text.value.trim() === '') {
      reject('УКАЖИТЕ ТЕКСТ УСПЕШНОГО РЕЗУЛЬТАТА', textInput.value);
      return;
    }
  }
  if (node.type === 'command' && mode.value === 'terminal-transition'
    && (targetTerminalID.value === '' || targetTerminalID.value === props.terminalID
      || !props.terminalOptions.some(option => option.id === targetTerminalID.value))) {
    reject('ВЫБЕРИТЕ ДРУГОЙ СУЩЕСТВУЮЩИЙ ТЕРМИНАЛ', targetInput.value);
    return;
  }
  error.value = '';
  emit('apply', {
    commandMode: mode.value,
    completedName: completedName.value,
    confirmationText: confirmationText.value,
    name: trimmedName,
    targetTerminalID: targetTerminalID.value,
    text: text.value,
  });
}

function deleteNode(): void {
  const node = props.node;
  if (node === null || node.id === 'root') return;
  const childCount = node.type === 'folder' ? node.children.length : 0;
  const message = childCount > 0
    ? `Удалить "${node.name}" вместе со всем содержимым (${childCount} элемент(ов))?`
    : `Удалить "${node.name}"?`;
  if (window.confirm(message)) emit('delete');
}
</script>

<template>
  <div v-if="node === null" class="node-empty">Выберите узел дерева слева</div>
  <template v-else-if="node.id === 'root'">
    <div class="node-type-label">КОРЕНЬ ТЕРМИНАЛА</div>
    <div class="node-empty">Это главный экран терминала. Добавляйте папки, команды и записи через панель инструментов сверху.</div>
  </template>
  <template v-else>
    <div class="node-type-label">{{ node.type === 'folder' ? 'ПАПКА' : node.type === 'command' ? 'КОМАНДА' : 'ЗАПИСЬ' }}</div>
    <label class="field-label" for="fldName">{{ node.type === 'command' ? 'ИСХОДНОЕ НАЗВАНИЕ' : 'НАЗВАНИЕ' }}</label>
    <input id="fldName" ref="nameInput" v-model="name" class="field-input">

    <template v-if="node.type === 'command'">
      <label class="field-label" for="fldCommandMode">РЕЖИМ КОМАНДЫ</label>
      <select id="fldCommandMode" v-model="mode" class="field-input command-mode-select" :disabled="node.execution !== null">
        <option value="ordinary">ОБЫЧНАЯ КОМАНДА</option>
        <option value="state-change">ИЗМЕНЯЕТ СОСТОЯНИЕ</option>
        <option value="terminal-transition">ПЕРЕХОД В ДРУГОЙ ТЕРМИНАЛ</option>
      </select>
      <div v-if="node.execution !== null" class="command-mode-hint">Сначала сбросьте выполненное состояние команды, чтобы изменить режим.</div>
      <div id="stateChangeFields" class="state-change-fields" :hidden="mode !== 'state-change'">
        <label class="field-label" for="fldCompletedName">НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ</label>
        <input id="fldCompletedName" ref="completedNameInput" v-model="completedName" class="field-input">
        <label class="field-label" for="fldConfirmationText">ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ</label>
        <textarea id="fldConfirmationText" ref="confirmationTextInput" v-model="confirmationText" class="field-textarea state-change-textarea"></textarea>
      </div>
      <div id="terminalTransitionFields" class="state-change-fields terminal-transition-fields" :hidden="mode !== 'terminal-transition'">
        <label class="field-label" for="fldTerminalTransitionTarget">ЦЕЛЕВОЙ ТЕРМИНАЛ</label>
        <select id="fldTerminalTransitionTarget" ref="targetInput" v-model="targetTerminalID" class="field-input">
          <option value="">ВЫБЕРИТЕ ТЕРМИНАЛ</option>
          <option v-for="option in terminalOptions" :key="option.id" :value="option.id">{{ option.name }}</option>
        </select>
      </div>
      <label class="field-label" for="fldText">ТЕКСТ УСПЕШНОГО ВЫПОЛНЕНИЯ</label>
      <textarea id="fldText" ref="textInput" v-model="text" class="field-textarea"></textarea>
      <div v-if="node.execution !== null" class="command-execution-snapshot" role="status" aria-label="СОХРАНЁННОЕ СОСТОЯНИЕ КОМАНДЫ">
        <div class="command-execution-heading">ВЫПОЛНЕНО</div>
        <div class="command-execution-label">ЗАФИКСИРОВАННОЕ НАЗВАНИЕ</div>
        <div class="command-execution-value">{{ node.execution.completedName }}</div>
        <div class="command-execution-label">ЗАФИКСИРОВАННЫЙ РЕЗУЛЬТАТ</div>
        <div class="command-execution-value command-execution-result">{{ node.execution.resultText }}</div>
      </div>
    </template>
    <template v-else-if="node.type === 'entry'">
      <label class="field-label" for="fldText">ОПИСАНИЕ ЗАПИСИ</label>
      <textarea id="fldText" ref="textInput" v-model="text" class="field-textarea"></textarea>
    </template>
    <template v-else>
      <div class="field-label">СОДЕРЖИМОЕ</div>
      <div class="node-empty">{{ node.children.length }} элемент(ов)</div>
    </template>

    <div id="nodeValidationError" class="node-validation-error" role="alert" :hidden="error === ''">{{ error }}</div>
    <div class="node-actions">
      <button id="btnApplyNode" class="btn btn-primary" type="button" :disabled="pending" @click="apply">ПРИМЕНИТЬ</button>
      <button v-if="node.execution !== null" id="btnResetCommandState" class="btn btn-secondary" type="button" :disabled="pending" @click="emit('reset')">СБРОСИТЬ СОСТОЯНИЕ</button>
      <button id="btnDeleteNode" class="btn btn-danger" type="button" :disabled="pending" @click="deleteNode">УДАЛИТЬ</button>
    </div>
  </template>
</template>
