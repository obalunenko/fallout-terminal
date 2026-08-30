<script setup lang="ts">
defineProps<{
  label: string;
}>();

defineEmits<{
  save: [value: string];
}>();
</script>

<template>
  <button type="button" @click="$emit('save', label)">
    {{ label }}
  </button>
</template>
