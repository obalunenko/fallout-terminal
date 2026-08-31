<script setup lang="ts">
defineProps<{
  readonly clientCount: number;
  readonly filePath: string;
  readonly serverError: boolean;
  readonly serverLabel: string;
  readonly serverTitle: string;
  readonly serverURL: string;
}>();

defineEmits<{
  open: [];
}>();
</script>

<template>
  <Teleport to="#runtimeHeaderVueLeaf">
    <header id="runtimeHeader" class="topbar">
      <div class="title-block">
        <span class="title-main">FALLOUT TERMINAL</span>
        <span id="sessionFileLabel" class="title-sub">{{ filePath || '—' }}</span>
      </div>
      <div class="server-block">
        <div class="server-row">
          <span class="info-lbl">АДРЕС ДЛЯ ИГРОКОВ</span>
          <span
            id="serverUrl"
            class="server-url"
            :class="{ 'server-url-error': serverError }"
            :title="serverTitle"
            :role="serverURL ? 'link' : undefined"
            :tabindex="serverURL ? 0 : undefined"
            @click="$emit('open')"
            @keydown.enter.prevent="$emit('open')"
            @keydown.space.prevent="$emit('open')"
          >{{ serverLabel }}</span>
        </div>
        <div class="server-row">
          <span class="info-lbl">ПОДКЛЮЧЕНО</span>
          <span id="clientCount" class="conn-count">{{ clientCount }}</span>
        </div>
      </div>
    </header>
  </Teleport>
</template>
