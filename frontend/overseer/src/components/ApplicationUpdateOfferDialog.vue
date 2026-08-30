<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';

const props = defineProps<{
  readonly availableVersion: string;
  readonly focusRequest: number;
  readonly installedVersion: string;
  readonly open: boolean;
  readonly pending: boolean;
  readonly releaseNotes: string;
}>();

const emit = defineEmits<{
  accept: [];
  defer: [];
}>();

const dialog = ref<HTMLDialogElement | null>(null);
const deferButton = ref<HTMLButtonElement | null>(null);

async function syncDialog(): Promise<void> {
  await nextTick();
  const element = dialog.value;
  if (element === null) return;
  if (props.open) {
    if (!element.open) element.showModal();
    deferButton.value?.focus();
  } else if (element.open) {
    element.close();
  }
}

watch(() => props.open, () => { void syncDialog(); }, { immediate: true, flush: 'post' });
watch(() => props.focusRequest, () => { if (props.open) deferButton.value?.focus(); });
onBeforeUnmount(() => { if (dialog.value?.open === true) dialog.value.close(); });
</script>

<template>
  <dialog
    id="applicationUpdateDialog"
    ref="dialog"
    class="application-update-dialog"
    aria-modal="true"
    aria-labelledby="applicationUpdateDialogTitle"
    aria-describedby="applicationUpdateDialogDescription applicationUpdateReleaseNotes"
    :aria-busy="pending"
    :hidden="!open"
    @cancel.prevent="emit('defer')"
  >
    <div class="application-update-dialog-panel">
      <h2 id="applicationUpdateDialogTitle" class="application-update-dialog-title">ДОСТУПНО ОБНОВЛЕНИЕ</h2>
      <p id="applicationUpdateDialogDescription" class="application-update-dialog-description">Обновление начнёт загружаться только после вашего подтверждения. Можно продолжить работу с текущей версией.</p>
      <dl class="application-update-versions">
        <div>
          <dt>УСТАНОВЛЕНА</dt>
          <dd id="applicationUpdateInstalledVersion">{{ installedVersion || '—' }}</dd>
        </div>
        <div>
          <dt>ДОСТУПНА</dt>
          <dd id="applicationUpdateAvailableVersion">{{ availableVersion || '—' }}</dd>
        </div>
      </dl>
      <section class="application-update-release" aria-labelledby="applicationUpdateReleaseNotesHeading">
        <h3 id="applicationUpdateReleaseNotesHeading">ИСТОРИЯ ИЗМЕНЕНИЙ</h3>
        <div id="applicationUpdateReleaseNotes" class="application-update-release-notes">{{ releaseNotes || 'Для этого выпуска описание не предоставлено.' }}</div>
      </section>
      <div class="application-update-actions" role="group" aria-label="Решение об обновлении приложения">
        <button id="btnAcceptApplicationUpdate" class="btn btn-primary" type="button" :disabled="pending" @click="emit('accept')">ОБНОВИТЬ СЕЙЧАС</button>
        <button id="btnDeferApplicationUpdate" ref="deferButton" class="btn btn-secondary" type="button" :disabled="pending" @click="emit('defer')">ПРОДОЛЖИТЬ БЕЗ ОБНОВЛЕНИЯ</button>
      </div>
    </div>
  </dialog>
</template>
