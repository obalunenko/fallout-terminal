<script setup lang="ts">
const props = defineProps<{
  readonly attemptsLeft: number;
  readonly attemptsMax: number;
}>();

function pluralAttempts(value: number): string {
  const mod10 = value % 10;
  const mod100 = value % 100;
  if (mod10 === 1 && mod100 !== 11) return 'ПОПЫТКА';
  if ([2, 3, 4].includes(mod10) && ![12, 13, 14].includes(mod100)) return 'ПОПЫТКИ';
  return 'ПОПЫТОК';
}
</script>

<template>
  <div id="attemptsLine">
    {{ props.attemptsLeft }} {{ pluralAttempts(props.attemptsLeft) }} ОСТАЛОСЬ:
    <span
      v-for="index in props.attemptsMax"
      :key="index"
      class="atsq"
      :class="{ empty: index > props.attemptsLeft, full: index <= props.attemptsLeft }"
      aria-hidden="true"
    >■</span>
  </div>
</template>
