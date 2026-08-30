<script setup lang="ts">
import ApplicationUpdateOfferDialog from './components/ApplicationUpdateOfferDialog.vue';
import ApplicationUpdateRestartDialog from './components/ApplicationUpdateRestartDialog.vue';
import ApplicationUpdateStatus from './components/ApplicationUpdateStatus.vue';
import CommandExecutionDialog from './components/CommandExecutionDialog.vue';
import CommandStateResetDialog from './components/CommandStateResetDialog.vue';
import LogicalSessionDialog from './components/LogicalSessionDialog.vue';
import PlayerConfigControls from './components/PlayerConfigControls.vue';
import PlayerDeleteDialog from './components/PlayerDeleteDialog.vue';
import PlayerManagementDialog from './components/PlayerManagementDialog.vue';
import SessionControls from './components/SessionControls.vue';
import TerminalNavigationDialog from './components/TerminalNavigationDialog.vue';
import TerminalSwitchDialog from './components/TerminalSwitchDialog.vue';
import { useApplicationUpdate } from './composables/useApplicationUpdate.js';
import { useCommandApproval } from './composables/useCommandApproval.js';
import { useLogicalSessions } from './composables/useLogicalSessions.js';
import { usePlayerConfiguration } from './composables/usePlayerConfiguration.js';
import { usePlayerManagement } from './composables/usePlayerManagement.js';
import { useSessionDocument } from './composables/useSessionDocument.js';
import { useTerminalNavigationApproval } from './composables/useTerminalNavigationApproval.js';
import { useTerminalSwitch } from './composables/useTerminalSwitch.js';
import type { DesktopPort } from './ports/desktop-port.js';

const props = defineProps<{
  readonly port: DesktopPort;
}>();

const update = useApplicationUpdate(props.port);
const commandApproval = useCommandApproval(props.port);
const logicalSessions = useLogicalSessions(props.port);
const playerConfiguration = usePlayerConfiguration(props.port);
const playerManagement = usePlayerManagement(props.port);
const sessionDocument = useSessionDocument(props.port);
const terminalNavigation = useTerminalNavigationApproval(props.port);
const terminalSwitch = useTerminalSwitch(props.port);
</script>

<template>
  <SessionControls
    :error="sessionDocument.error.value"
    :fatal="sessionDocument.fatal.value"
    :loaded="sessionDocument.loaded.value"
    :pending="sessionDocument.pending.value"
    :startup-state="sessionDocument.startupState.value"
    :status="sessionDocument.status.value"
    @create="sessionDocument.create"
    @open="sessionDocument.open"
  />
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
  <PlayerConfigControls
    :active="playerConfiguration.active.value"
    :blocked="playerConfiguration.blocked.value"
    :error="playerConfiguration.error.value"
    :status="playerConfiguration.status.value"
    @create="playerConfiguration.create"
    @manage="playerConfiguration.manage"
    @open="playerConfiguration.open"
  />
  <PlayerManagementDialog
    :add-reset-request="playerManagement.addResetRequest.value"
    :delete-focus-character-id="playerManagement.deleteFocusCharacterId.value"
    :delete-focus-request="playerManagement.deleteFocusRequest.value"
    :error="playerManagement.error.value"
    :open="playerManagement.open.value"
    :pending="playerManagement.pending.value"
    :profiles="playerManagement.profiles.value"
    :read-only="playerManagement.readOnly.value"
    :status="playerManagement.status.value"
    @add="playerManagement.add"
    @close="playerManagement.close"
    @delete="playerManagement.requestDelete"
    @save="playerManagement.save"
  />
  <PlayerDeleteDialog :port="port" />
  <LogicalSessionDialog
    :broadcast-active="logicalSessions.broadcastActive.value"
    :error="logicalSessions.error.value"
    :open="logicalSessions.open.value"
    :pending="logicalSessions.pending.value"
    :player-config-active="logicalSessions.playerConfigActive.value"
    :roster="logicalSessions.roster.value"
    :sessions="logicalSessions.sessions.value"
    :status="logicalSessions.status.value"
    @assign="logicalSessions.assign"
    @close="logicalSessions.close"
    @controller="logicalSessions.controller"
    @move="logicalSessions.move"
    @release="logicalSessions.release"
    @rename="logicalSessions.rename"
  />
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
