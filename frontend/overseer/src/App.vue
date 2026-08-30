<script setup lang="ts">
import ApplicationUpdateOfferDialog from './components/ApplicationUpdateOfferDialog.vue';
import ApplicationUpdateRestartDialog from './components/ApplicationUpdateRestartDialog.vue';
import ApplicationUpdateStatus from './components/ApplicationUpdateStatus.vue';
import CommandExecutionDialog from './components/CommandExecutionDialog.vue';
import CommandStateResetDialog from './components/CommandStateResetDialog.vue';
import TerminalNavigationDialog from './components/TerminalNavigationDialog.vue';
import TerminalSwitchDialog from './components/TerminalSwitchDialog.vue';
import { useApplicationUpdate } from './composables/useApplicationUpdate.js';
import { useCommandApproval } from './composables/useCommandApproval.js';
import { useTerminalNavigationApproval } from './composables/useTerminalNavigationApproval.js';
import { useTerminalSwitch } from './composables/useTerminalSwitch.js';
import type { DesktopPort } from './ports/desktop-port.js';

const props = defineProps<{
  readonly port: DesktopPort;
}>();

const update = useApplicationUpdate(props.port);
const commandApproval = useCommandApproval(props.port);
const terminalNavigation = useTerminalNavigationApproval(props.port);
const terminalSwitch = useTerminalSwitch(props.port);
</script>

<template>
  <ApplicationUpdateStatus
    :failure="update.failure.value"
    :show-button="update.showButton.value"
    :silent="update.silent.value"
    :snapshot="update.snapshot.value"
    :status-text="update.statusText.value"
    @show="update.showCurrent"
  />
  <ApplicationUpdateOfferDialog
    :available-version="update.snapshot.value.availableVersion"
    :focus-request="update.offerFocusRequest.value"
    :installed-version="update.snapshot.value.installedVersion"
    :open="update.offerOpen.value"
    :pending="update.offerPending.value"
    :release-notes="update.snapshot.value.releaseNotes"
    @accept="update.accept"
    @defer="update.defer"
  />
  <ApplicationUpdateRestartDialog
    :focus-request="update.restartFocusRequest.value"
    :open="update.restartOpen.value"
    :pending="update.restartPending.value"
    @postpone="update.postpone"
    @restart="update.restart"
  />
  <CommandExecutionDialog
    :outcome-error="commandApproval.outcomeError.value"
    :pending="commandApproval.pending.value"
    :request="commandApproval.current.value"
    :status="commandApproval.status.value"
    @approve="commandApproval.approve"
    @reject="commandApproval.reject"
  />
  <CommandStateResetDialog />
  <TerminalNavigationDialog
    :outcome-error="terminalNavigation.outcomeError.value"
    :pending="terminalNavigation.pending.value"
    :request="terminalNavigation.current.value"
    :status="terminalNavigation.status.value"
    @approve="terminalNavigation.approve"
    @reject="terminalNavigation.reject"
  />
  <TerminalSwitchDialog
    :error="terminalSwitch.error.value"
    :pending="terminalSwitch.pending.value"
    :switch-id="terminalSwitch.switchId.value"
    @cancel="terminalSwitch.cancel"
    @discard="terminalSwitch.discard"
    @preserve="terminalSwitch.preserve"
  />
</template>
