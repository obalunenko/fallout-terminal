<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';

import type { BroadcastSnapshot } from '../composables/useBroadcastControls.js';

const props = defineProps<{
  readonly error: string;
  readonly focusRequest: 'end' | 'start' | 'stop' | null;
  readonly pending: boolean;
  readonly snapshot: BroadcastSnapshot;
  readonly status: string;
}>();

const emit = defineEmits<{
  end: [];
  manage: [];
  start: [];
  takeOff: [];
}>();

const startButton = ref<HTMLButtonElement | null>(null);
const endButton = ref<HTMLButtonElement | null>(null);
const stopButton = ref<HTMLButtonElement | null>(null);
const summary = computed(() => {
  if (props.snapshot.broadcastID === '') return 'ТРАНСЛЯЦИЯ НЕ ЗАПУЩЕНА';
  if (props.snapshot.activeTerminalID !== '') {
    return `ТРАНСЛЯЦИЯ АКТИВНА · ТЕРМИНАЛ ${props.snapshot.activeTerminalID}`;
  }
  return `ТРАНСЛЯЦИЯ АКТИВНА · ОЖИДАНИЕ ТЕРМИНАЛА · ${props.snapshot.broadcastID}`;
});

watch(() => props.focusRequest, async request => {
  if (request === null) return;
  await nextTick();
  if (request === 'start') startButton.value?.focus();
  else if (request === 'end') endButton.value?.focus();
  else stopButton.value?.focus();
});
</script>

<template>
  <Teleport to="#broadcastControlsVueLeaf">
    <section
      id="coordinationPanel"
      class="coord-panel"
      aria-labelledby="coordinationHeading"
      v-bind="{
        'data-coordination-revision': String(snapshot.revision),
        'data-player-config-active': String(snapshot.playerConfigActive),
        'data-stale-suppression': 'enforced',
      }"
    >
      <div id="coordinationHeading" class="panel-hdr coord-hdr">ИГРОКИ И ТРАНСЛЯЦИЯ</div>
      <div id="broadcastSummary" class="broadcast-summary" :class="{ 'is-live': snapshot.broadcastID !== '' }" role="status" aria-live="polite" aria-atomic="true">
        {{ summary }}
      </div>
      <div class="broadcast-terminal-actions" role="group" aria-label="Активный терминал трансляции">
        <button id="btnStopBroadcast" ref="stopButton" class="btn btn-danger full-w" type="button" :hidden="snapshot.activeTerminalID === ''" :disabled="pending || snapshot.activeTerminalID === ''" @click="emit('takeOff')">СНЯТЬ С ЭФИРА</button>
      </div>

      <section id="playerConfigVueLeaf" class="coord-group player-config-management" aria-labelledby="playerConfigHeading"></section>

      <section class="coord-group session-management" aria-labelledby="logicalSessionsHeading">
        <h2 id="logicalSessionsHeading" class="coord-group-title">ЛОГИЧЕСКИЕ СЕССИИ</h2>
        <div class="logical-session-summary">
          <div class="logical-session-count">
            <span class="logical-session-count-label">АКТИВНО</span>
            <output id="activeLogicalSessionCount" aria-label="Количество активных логических сессий" aria-live="polite" aria-atomic="true">{{ snapshot.activeLogicalSessionCount }}</output>
          </div>
          <button id="btnManageLogicalSessions" class="btn btn-mini btn-secondary" type="button" aria-haspopup="dialog" aria-controls="logicalSessionDialog" @click="emit('manage')">УПРАВЛЯТЬ</button>
        </div>
      </section>

      <button id="btnStartBroadcast" ref="startButton" class="btn btn-primary full-w" type="button" :disabled="pending || snapshot.broadcastID !== '' || !snapshot.playerConfigActive" @click="emit('start')">НАЧАТЬ ТРАНСЛЯЦИЮ</button>
      <button id="btnEndBroadcast" ref="endButton" class="btn btn-danger full-w" type="button" :hidden="snapshot.broadcastID === ''" :disabled="pending || snapshot.broadcastID === ''" @click="emit('end')">ЗАВЕРШИТЬ ТРАНСЛЯЦИЮ</button>
      <div class="coord-feedback" aria-label="Состояние операций координации">
        <div id="coordinationStatus" class="coord-status" role="status" aria-live="polite" aria-atomic="true">{{ status }}</div>
        <div id="coordinationError" class="coord-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error === ''">{{ error }}</div>
      </div>
    </section>
  </Teleport>
</template>
