<script setup lang="ts">
const emit = defineEmits<{
  save: [value: string];
}>();

const emitSave: (value: string) => void = (value) => emit('save', value);

emitSave(42);
</script>

<template>
  <button type="button">
    Save
  </button>
</template>
