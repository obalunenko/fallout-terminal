<script setup lang="ts">
defineProps<{
  readonly introText?: string;
  readonly serverNumber?: number;
  readonly visible?: boolean;
}>();
</script>

<template>
  <header id="normalHeader" class="term-header" :hidden="!visible">
    <div class="hdr-center">
      <div>ROBCO INDUSTRIES UNIFIED OPERATING SYSTEM</div>
      <div>COPYRIGHT 2075-2077 ROBCO INDUSTRIES</div>
      <div id="serverLine">-Server {{ serverNumber ?? 1 }}-</div>
    </div>
    <div id="introTextEl" class="hdr-intro">{{ introText ?? '' }}</div>
    <div class="term-divider" />
  </header>
</template>
