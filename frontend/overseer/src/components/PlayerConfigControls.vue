<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  readonly active: boolean;
  readonly blocked: boolean;
  readonly error: string;
  readonly status: string;
}>();

defineEmits<{
  create: [];
  manage: [];
  open: [];
}>();

const statusAttributes = computed(() => ({
  'data-active': String(props.active),
}));
</script>

<template>
  <Teleport to="#playerConfigVueLeaf">
    <h2 id="playerConfigHeading" class="coord-group-title">КОНФИГУРАЦИЯ ИГРОКОВ</h2>
    <div id="playerConfigStatus" v-bind="statusAttributes" class="player-config-status" role="status" aria-live="polite" aria-atomic="true">{{ status }}</div>
    <div class="player-config-actions" role="group" aria-label="Выбор конфигурации игроков">
      <button id="btnOpenPlayerConfig" class="btn btn-mini" type="button" :disabled="blocked" @click="$emit('open')">ВЫБРАТЬ ФАЙЛ</button>
      <button id="btnNewPlayerConfig" class="btn btn-mini btn-secondary" type="button" :disabled="blocked" @click="$emit('create')">СОЗДАТЬ ФАЙЛ</button>
      <button id="btnManagePlayers" class="btn btn-mini btn-secondary" type="button" :disabled="!active" @click="$emit('manage')">УПРАВЛЯТЬ ИГРОКАМИ</button>
    </div>
    <div id="playerConfigError" class="player-config-error" role="alert" aria-live="assertive" aria-atomic="true" :hidden="error === ''">{{ error }}</div>
  </Teleport>
</template>
