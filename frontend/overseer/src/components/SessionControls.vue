<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  readonly error: string;
  readonly fatal: boolean;
  readonly loaded: boolean;
  readonly pending: boolean;
  readonly startupState: string;
  readonly status: string;
}>();

const emit = defineEmits<{
  create: [];
  open: [];
}>();

const statusAttributes = computed(() => ({
  'data-state': props.error ? 'error' : props.startupState,
}));
</script>

<template>
  <div v-if="!loaded" id="startScreen" class="start-screen">
    <div class="start-box">
      <div class="start-title">FALLOUT TERMINAL</div>
      <div class="start-sub">OVERSEER CONTROL — VAULT-TEC R&amp;D</div>
      <div class="start-actions">
        <button id="btnOpenSession" class="btn btn-primary" type="button" :disabled="fatal || pending" @click="emit('open')">ОТКРЫТЬ СЕССИЮ</button>
        <button id="btnNewSession" class="btn btn-secondary" type="button" :disabled="fatal || pending" @click="emit('create')">НОВАЯ СЕССИЯ</button>
      </div>
      <div id="startStatus" v-bind="statusAttributes" class="start-status" role="status" aria-live="polite" aria-atomic="true">{{ error ? `Ошибка: ${error}` : status }}</div>
    </div>
  </div>
</template>
