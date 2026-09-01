<script setup lang="ts">
defineProps<{
  readonly visible: boolean;
}>();
</script>

<template>
  <div
    id="hackBlocked"
    class="hack-blocked"
    :hidden="!visible"
    role="status"
    aria-live="assertive"
    aria-atomic="true"
  >
    <div>Вход заблокирован.</div>
    <div>Обратитесь к администратору.</div>
  </div>
</template>
