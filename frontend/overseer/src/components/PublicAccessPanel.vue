<script setup lang="ts">
import { computed } from 'vue';

import type { PublicAccessViewSnapshot } from '../composables/usePublicAccess.js';

const props = defineProps<{
  readonly controlsDisabled: boolean;
  readonly copyStatus: string;
  readonly failure: string;
  readonly loaded: boolean;
  readonly pending: boolean;
  readonly publicURL: string;
  readonly snapshot: PublicAccessViewSnapshot;
}>();

defineEmits<{
  copy: [];
  settings: [];
  start: [];
  stop: [];
}>();

const labels: Readonly<Record<string, string>> = Object.freeze({
  stopped: 'ОСТАНОВЛЕН',
  starting: 'ЗАПУСК…',
  ready: 'ГОТОВ',
  stopping: 'ОСТАНОВКА…',
  error: 'ОШИБКА',
});

const statusText = computed(() => labels[props.snapshot.status.state] ?? 'ЗАГРУЗКА…');
const stopping = computed(() => ['ready', 'stopping'].includes(props.snapshot.status.state));
const panelAttributes = computed(() => ({
  'data-fail-closed-ordering': 'enforced',
  'data-generation': String(props.snapshot.generation),
  'data-settings-revision': String(props.snapshot.settingsRevision),
  'data-subscription-cleanup': 'released',
}));
const stateAttributes = computed(() => ({
  'data-generation': String(props.snapshot.generation),
  'data-settings-revision': String(props.snapshot.settingsRevision),
  'data-state': props.snapshot.status.state || 'loading',
}));
</script>

<template>
  <Teleport to="#publicAccessVueLeaf">
    <section
      id="publicAccessSection"
      v-bind="panelAttributes"
      class="public-access"
      aria-labelledby="publicAccessHeading"
      :hidden="!loaded"
    >
      <h2 id="publicAccessHeading" class="panel-hdr public-access-heading">ПУБЛИЧНЫЙ ДОСТУП</h2>
      <div class="public-access-compact">
        <div class="public-access-state-row" v-bind="{ 'data-state': snapshot.status.state }">
          <span class="public-access-state-marker" aria-hidden="true"></span>
          <div
            id="publicAccessStatus"
            v-bind="stateAttributes"
            role="status"
            aria-live="polite"
            aria-atomic="true"
          >{{ statusText }}</div>
        </div>
        <div
          id="publicAccessError"
          role="alert"
          aria-live="assertive"
          aria-atomic="true"
          :hidden="failure.length === 0"
        >{{ failure }}</div>
        <div class="public-access-address-label">ПУБЛИЧНЫЙ АДРЕС</div>
        <div class="public-access-url-row">
          <span id="publicAccessURL" v-bind="{ 'data-available': String(publicURL.length > 0) }">
            {{ publicURL || 'ПОЯВИТСЯ ПОСЛЕ ЗАПУСКА' }}
          </span>
          <button
            id="btnCopyPublicURL"
            class="btn btn-mini btn-secondary"
            type="button"
            :hidden="publicURL.length === 0"
            :disabled="controlsDisabled"
            @click="$emit('copy')"
          >КОПИРОВАТЬ</button>
        </div>
        <div class="public-access-runtime-actions" role="group" aria-label="Управление публичным доступом">
          <button
            id="btnStartPublicAccess"
            class="btn btn-accent"
            type="button"
            :hidden="stopping"
            :disabled="controlsDisabled || snapshot.status.state === 'ready'"
            @click="$emit('start')"
          >{{ snapshot.status.state === 'starting' || pending ? 'ЗАПУСК…' : 'ВКЛЮЧИТЬ ДОСТУП' }}</button>
          <button
            id="btnStopPublicAccess"
            class="btn btn-danger"
            type="button"
            :hidden="!stopping"
            :disabled="controlsDisabled || snapshot.status.state === 'stopped'"
            @click="$emit('stop')"
          >{{ snapshot.status.state === 'stopping' || pending ? 'ОСТАНОВКА…' : 'ОСТАНОВИТЬ ДОСТУП' }}</button>
        </div>
        <button
          id="btnOpenPublicAccessSettings"
          class="btn btn-secondary public-access-settings-trigger"
          type="button"
          :disabled="controlsDisabled"
          @click="$emit('settings')"
        >НАСТРОЙКИ…</button>
        <div
          id="publicAccessCopyStatus"
          class="public-access-copy-status"
          role="status"
          aria-live="polite"
          aria-atomic="true"
        >{{ copyStatus }}</div>
      </div>
    </section>
  </Teleport>
</template>
