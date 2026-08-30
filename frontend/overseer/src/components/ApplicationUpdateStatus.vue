<script setup lang="ts">
import { computed } from 'vue';

import type { ApplicationUpdateViewSnapshot } from '../composables/useApplicationUpdate.js';

const props = defineProps<{
  readonly failure: string;
  readonly showButton: boolean;
  readonly silent: boolean;
  readonly snapshot: ApplicationUpdateViewSnapshot;
  readonly statusText: string;
}>();

defineEmits<{
  show: [];
}>();

const progressVisible = computed(() => ['downloading', 'verifying', 'staging'].includes(props.snapshot.state));
const determinate = computed(() => props.snapshot.state === 'downloading'
  && props.snapshot.downloadSize !== null
  && props.snapshot.downloadSize > 0);
const progressMax = computed(() => determinate.value ? props.snapshot.downloadSize ?? undefined : undefined);
const progressValue = computed(() => {
  if (!determinate.value || props.snapshot.downloadSize === null) return undefined;
  const downloaded = props.snapshot.bytesDownloaded ?? 0;
  return Math.max(0, Math.min(downloaded, props.snapshot.downloadSize));
});
const progressLabel = computed(() => {
  if (determinate.value && progressMax.value !== undefined) {
    return `Загрузка обновления: ${progressValue.value ?? 0} из ${progressMax.value} байт`;
  }
  if (props.snapshot.state === 'verifying') return 'Проверка загруженного обновления';
  if (props.snapshot.state === 'staging') return 'Подготовка обновления к перезапуску';
  return 'Загрузка обновления';
});
const dialogId = computed(() => props.snapshot.state === 'ready-to-restart'
  ? 'applicationUpdateRestartDialog'
  : 'applicationUpdateDialog');
const stateAttributes = computed(() => ({
  'data-revision': String(props.snapshot.revision),
  'data-state': props.snapshot.state,
}));
</script>

<template>
  <aside
    id="applicationUpdateStatusPanel"
    v-bind="stateAttributes"
    class="application-update-status-panel"
    aria-label="Обновление приложения"
    :hidden="silent"
  >
    <div class="application-update-status-copy">
      <div
        id="applicationUpdateStatus"
        class="application-update-status"
        role="status"
        aria-live="polite"
        aria-atomic="true"
      >{{ statusText }}</div>
      <div
        id="applicationUpdateError"
        class="application-update-error"
        role="alert"
        aria-live="assertive"
        aria-atomic="true"
        :hidden="failure.length === 0"
      >{{ failure }}</div>
      <progress
        id="applicationUpdateProgress"
        class="application-update-progress"
        :aria-label="progressLabel"
        :hidden="!progressVisible"
        :max="progressMax"
        :value="progressValue"
      />
    </div>
    <button
      id="btnShowApplicationUpdate"
      class="btn btn-mini btn-accent"
      type="button"
      aria-haspopup="dialog"
      :aria-controls="dialogId"
      :hidden="!showButton"
      @click="$emit('show')"
    >ПОКАЗАТЬ ОБНОВЛЕНИЕ</button>
  </aside>
</template>
