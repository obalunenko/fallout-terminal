<script setup lang="ts">
import { computed } from 'vue';

import type { HackSnapshot } from '../composables/useHackControls.js';

const props = defineProps<{
  readonly error: string;
  readonly pending: boolean;
  readonly snapshot: HackSnapshot | null;
  readonly visible: boolean;
}>();

const emit = defineEmits<{
  force: [];
  reset: [];
}>();

const status = computed(() => {
  if (props.snapshot?.solved) return 'ВЗЛОМ: ПРОЙДЕН';
  if (props.snapshot?.failed) return 'ВЗЛОМ: ЗАБЛОКИРОВАН';
  if (props.snapshot !== null) {
    return `ВЗЛОМ: осталось попыток ${props.snapshot.attemptsLeft}/${props.snapshot.attemptsMax}`;
  }
  return 'ВЗЛОМ: —';
});
</script>

<template>
  <Teleport to="#hackControlsVueLeaf">
    <div
      v-if="visible && snapshot !== null"
      id="hackStatus"
      class="hack-status"
      v-bind="{
        'data-hack-revision': String(snapshot.revision),
        'data-stale-suppression': 'enforced',
      }"
    >
      <div id="hackStatusLine" class="hack-status-line" role="status" aria-live="polite" aria-atomic="true">{{ status }}</div>
      <button
        id="btnHackSuccess"
        class="btn btn-accent full-w"
        type="button"
        :disabled="pending || snapshot.solved || snapshot.failed"
        :hidden="snapshot.failed"
        @click="emit('force')"
      >ВЗЛОМ УСПЕШЕН</button>
      <button
        id="btnResetFailedHack"
        class="btn btn-accent full-w hack-retry"
        type="button"
        :disabled="pending || !snapshot.failed"
        :hidden="!snapshot.failed"
        @click="emit('reset')"
      >ПОВТОРИТЬ ВЗЛОМ</button>
      <div id="hackControlError" class="coord-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error === ''">{{ error }}</div>
    </div>
  </Teleport>
</template>
