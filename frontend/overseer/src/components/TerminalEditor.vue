<script setup lang="ts">
import { inject, nextTick, onUnmounted, ref, watch } from 'vue';

import { overseerControllerKey } from '../controllers/overseer-controller.js';
import type { DesktopRecord } from '../models/overseer-view-state.js';
import TerminalSettings from './TerminalSettings.vue';

interface EditorTerminal {
  readonly completedCount: number;
  readonly hackLevel: number;
  readonly id: string;
  readonly introText: string;
  readonly live: boolean;
  readonly name: string;
}

function isRecord(value: unknown): value is DesktopRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function terminal(value: unknown): EditorTerminal | null {
  if (!isRecord(value) || typeof value.id !== 'string' || value.id === ''
    || typeof value.name !== 'string' || typeof value.introText !== 'string'
    || !Number.isSafeInteger(value.hackLevel) || Number(value.hackLevel) < 0
    || Number(value.hackLevel) > 5 || !Number.isSafeInteger(value.completedCount)
    || Number(value.completedCount) < 0) return null;
  return Object.freeze({
    completedCount: Number(value.completedCount),
    hackLevel: Number(value.hackLevel),
    id: value.id,
    introText: value.introText,
    live: value.live === true,
    name: value.name,
  });
}

const controller = inject(overseerControllerKey, null);
const current = ref<EditorTerminal | null>(null);
const revision = ref(-1);
const broadcastActive = ref(false);
const pending = ref(false);
const resetPending = ref(false);
const hackLevel = ref(0);
const introText = ref('');
const settingsOpen = ref(false);
const publishLabel = ref('ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ');
let canonicalSettings = '';
let publishAcknowledgement = -1;
let publishTimer: ReturnType<typeof setTimeout> | null = null;

const release = controller?.subscribeState(message => {
  if (message.kind === 'terminal-editor-focus-settings') {
    void nextTick(() => document.getElementById('hackLevelSelect')?.focus());
    return;
  }
  if (message.kind !== 'terminal-editor-snapshot'
    || !Number.isSafeInteger(message.revision) || Number(message.revision) <= revision.value) return;
  const next = message.terminal === null ? null : terminal(message.terminal);
  if (message.terminal !== null && next === null) return;
  const nextSettings = next === null ? '' : `${next.id}\u0000${next.hackLevel}\u0000${next.introText}`;
  if (nextSettings !== canonicalSettings) {
    canonicalSettings = nextSettings;
    hackLevel.value = next?.hackLevel ?? 0;
    introText.value = next?.introText ?? '';
  }
  current.value = next;
  revision.value = Number(message.revision);
  broadcastActive.value = message.broadcastActive === true;
  pending.value = message.pending === true;
  resetPending.value = message.resetPending === true;
  const acknowledgement = Number(message.publishAcknowledgement);
  if (Number.isSafeInteger(acknowledgement) && acknowledgement > publishAcknowledgement) {
    publishAcknowledgement = acknowledgement;
    publishLabel.value = 'ОБНОВЛЕНО ✓';
    if (publishTimer !== null) clearTimeout(publishTimer);
    publishTimer = setTimeout(() => {
      publishTimer = null;
      publishLabel.value = 'ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ';
    }, 1200);
  }
});

watch(() => current.value?.live, live => {
  if (live !== true) settingsOpen.value = false;
});

function request(action: string, extra: DesktopRecord = {}): void {
  controller?.dispatch({
    action,
    ...extra,
    kind: 'terminal-editor-action-request',
    revision: revision.value,
    terminalID: current.value?.id ?? '',
  });
}

function applySettings(): void {
  request('apply-settings', { hackLevel: hackLevel.value, introText: introText.value });
}

function reapply(): void {
  settingsOpen.value = false;
  request('reapply-settings');
}

function settingsToggled(event: Event): void {
  settingsOpen.value = (event.currentTarget as HTMLDetailsElement).open;
}

onUnmounted(() => {
  release?.();
  if (publishTimer !== null) clearTimeout(publishTimer);
  publishTimer = null;
  current.value = null;
});
</script>

<template>
  <Teleport to="#terminalEditorVueLeaf">
    <div
      class="panel-hdr tree-hdr"
      v-bind="{
        'data-editor-revision': String(revision),
        'data-stale-suppression': 'enforced',
      }"
    >
      <span id="editingTermName">{{ current?.name ?? '—' }}</span>
      <div class="tree-hdr-actions">
        <div class="selected-terminal-actions" role="group" aria-label="Выбранный терминал">
          <span id="liveFlag" class="live-flag" :hidden="current?.live !== true">● В ЭФИРЕ</span>
          <button id="btnMakeLive" class="btn btn-primary" type="button" :hidden="current?.live === true" :disabled="current === null || !broadcastActive || pending" @click="request('make-live')">СДЕЛАТЬ АКТИВНЫМ</button>
          <details id="terminalSettingsMenu" :open="settingsOpen" class="terminal-settings-menu" :hidden="current?.live !== true" @toggle="settingsToggled">
            <summary>ДОПОЛНИТЕЛЬНО</summary>
            <div class="terminal-settings-menu-panel">
              <p>Отправляет игрокам все настройки выбранного терминала, а не только содержимое. Незавершённый запрос может потребовать отдельного решения.</p>
              <button id="btnReapplySettings" class="btn btn-secondary" type="button" :disabled="current?.live !== true || pending" @click="reapply">ПЕРЕПРИМЕНИТЬ НАСТРОЙКИ</button>
            </div>
          </details>
        </div>
        <div class="editor-publish-actions" role="group" aria-label="Публикация содержимого редактора">
          <button id="btnPublish" class="btn btn-accent" type="button" :hidden="current?.live !== true" :disabled="current?.live !== true || pending" :aria-busy="current?.live === true && pending" @click="request('publish')">{{ publishLabel }}</button>
        </div>
      </div>
    </div>
  </Teleport>
  <Teleport to="#terminalSettingsVueLeaf">
    <TerminalSettings
      :completed-count="current?.completedCount ?? 0"
      :hack-level="hackLevel"
      :intro-text="introText"
      :pending="pending"
      :reset-pending="resetPending"
      :terminal-available="current !== null"
      @apply="applySettings"
      @reset="request('reset-command-states')"
      @update-hack-level="hackLevel = $event"
      @update-intro-text="introText = $event"
    />
  </Teleport>
</template>
