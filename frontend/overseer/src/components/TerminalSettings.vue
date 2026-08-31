<script setup lang="ts">
defineProps<{
  readonly completedCount: number;
  readonly hackLevel: number;
  readonly introText: string;
  readonly pending: boolean;
  readonly resetPending: boolean;
  readonly terminalAvailable: boolean;
}>();

defineEmits<{
  apply: [];
  reset: [];
  updateHackLevel: [value: number];
  updateIntroText: [value: string];
}>();
</script>

<template>
  <div class="term-settings" id="termSettings">
    <div class="settings-row">
      <label class="field-label-inline" for="hackLevelSelect">УРОВЕНЬ ВЗЛОМА</label>
      <select
        id="hackLevelSelect"
        class="field-input mini-select"
        :disabled="!terminalAvailable || pending"
        :value="String(hackLevel)"
        @change="$emit('updateHackLevel', Number(($event.target as HTMLSelectElement).value))"
      >
        <option value="0">Нет (0)</option>
        <option value="1">1 — 4 буквы</option>
        <option value="2">2 — 5 букв</option>
        <option value="3">3 — 6 букв</option>
        <option value="4">4 — 7 букв</option>
        <option value="5">5 — 8 букв</option>
      </select>
      <button id="btnApplySettings" class="btn btn-mini" type="button" :disabled="!terminalAvailable || pending" @click="$emit('apply')">ПРИМЕНИТЬ</button>
    </div>
    <div class="settings-row">
      <label class="field-label-inline" for="introTextArea">ТЕКСТ ТЕРМИНАЛА (общий для всех экранов игрока)</label>
      <textarea
        id="introTextArea"
        class="field-textarea settings-textarea"
        placeholder="Необязательный текст — покажется под шапкой на каждом экране этого терминала"
        :disabled="!terminalAvailable || pending"
        :value="introText"
        @input="$emit('updateIntroText', ($event.target as HTMLTextAreaElement).value)"
      ></textarea>
    </div>
    <div class="settings-row command-state-terminal-actions" :hidden="!terminalAvailable">
      <button
        id="btnResetTerminalCommandStates"
        class="btn btn-mini btn-danger"
        type="button"
        :disabled="completedCount === 0 || resetPending"
        @click="$emit('reset')"
      >СБРОСИТЬ ВСЕ СОСТОЯНИЯ</button>
    </div>
  </div>
</template>
