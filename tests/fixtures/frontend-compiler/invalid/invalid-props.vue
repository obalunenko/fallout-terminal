<script setup lang="ts">
interface InvalidProps {
  label: string;
}

defineProps<InvalidProps>();

const invalidProps: InvalidProps = { label: 42 };

void invalidProps;
</script>

<template>
  <span>{{ label }}</span>
</template>
