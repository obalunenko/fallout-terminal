<script setup lang="ts">
import ApplicationUpdateOfferDialog from './components/ApplicationUpdateOfferDialog.vue';
import ApplicationUpdateRestartDialog from './components/ApplicationUpdateRestartDialog.vue';
import ApplicationUpdateStatus from './components/ApplicationUpdateStatus.vue';
import BroadcastControls from './components/BroadcastControls.vue';
import CommandExecutionDialog from './components/CommandExecutionDialog.vue';
import CommandStateResetDialog from './components/CommandStateResetDialog.vue';
import CreateTerminalDialog from './components/CreateTerminalDialog.vue';
import EndBroadcastDialog from './components/EndBroadcastDialog.vue';
import GeneratedPasswordDialog from './components/GeneratedPasswordDialog.vue';
import HackControlPanel from './components/HackControlPanel.vue';
import LogicalSessionDialog from './components/LogicalSessionDialog.vue';
import OverseerLayout from './components/OverseerLayout.vue';
import PlayerConfigControls from './components/PlayerConfigControls.vue';
import PlayerCredentialsDialog from './components/PlayerCredentialsDialog.vue';
import PlayerDeleteDialog from './components/PlayerDeleteDialog.vue';
import PlayerManagementDialog from './components/PlayerManagementDialog.vue';
import ProviderTokenDialog from './components/ProviderTokenDialog.vue';
import PublicAccessPanel from './components/PublicAccessPanel.vue';
import PublicAccessSettingsDialog from './components/PublicAccessSettingsDialog.vue';
import RuntimeHeader from './components/RuntimeHeader.vue';
import StartScreen from './components/StartScreen.vue';
import TakeOffAirDialog from './components/TakeOffAirDialog.vue';
import TerminalGroupDraftDialog from './components/TerminalGroupDraftDialog.vue';
import TerminalGroupImpactDialog from './components/TerminalGroupImpactDialog.vue';
import TerminalEditor from './components/TerminalEditor.vue';
import TerminalNavigationDialog from './components/TerminalNavigationDialog.vue';
import TerminalSidebar from './components/TerminalSidebar.vue';
import TerminalSwitchDialog from './components/TerminalSwitchDialog.vue';
import TerminalTree from './components/TerminalTree.vue';
import { useApplicationUpdate } from './composables/useApplicationUpdate.js';
import { useBroadcastControls } from './composables/useBroadcastControls.js';
import { useCommandApproval } from './composables/useCommandApproval.js';
import { useDesktopRuntime } from './composables/useDesktopRuntime.js';
import { useHackControls } from './composables/useHackControls.js';
import { useLogicalSessions } from './composables/useLogicalSessions.js';
import { usePlayerConfiguration } from './composables/usePlayerConfiguration.js';
import { usePlayerManagement } from './composables/usePlayerManagement.js';
import { usePublicAccess } from './composables/usePublicAccess.js';
import { useRuntimeStatus } from './composables/useRuntimeStatus.js';
import { useSessionDocument } from './composables/useSessionDocument.js';
import { useTerminalNavigationApproval } from './composables/useTerminalNavigationApproval.js';
import { useTerminalAuthoring } from './composables/useTerminalAuthoring.js';
import { useTerminalSwitch } from './composables/useTerminalSwitch.js';
import type { DesktopPort } from './ports/desktop-port.js';

const props = defineProps<{
  readonly port: DesktopPort;
}>();

const update = useApplicationUpdate(props.port);
const broadcastControls = useBroadcastControls(props.port);
const commandApproval = useCommandApproval(props.port);
const logicalSessions = useLogicalSessions(props.port);
const playerConfiguration = usePlayerConfiguration(props.port);
const playerManagement = usePlayerManagement(props.port);
const publicAccess = usePublicAccess(props.port);
const runtimeStatus = useRuntimeStatus(props.port);
const desktopRuntime = useDesktopRuntime(
  props.port,
  runtimeStatus.applyServerInfo,
  runtimeStatus.dispose,
);
const sessionDocument = useSessionDocument(props.port);
const terminalNavigation = useTerminalNavigationApproval(props.port);
const terminalAuthoring = useTerminalAuthoring();
const hackControls = useHackControls(props.port);
const terminalSwitch = useTerminalSwitch(props.port);
</script>

<template>
  <OverseerLayout :loaded="sessionDocument.loaded.value">
  <StartScreen
    v-if="!sessionDocument.loaded.value"
    :error="sessionDocument.error.value"
    :fatal="runtimeStatus.fatal.value"
    :pending="sessionDocument.pending.value"
    :state="runtimeStatus.state.value"
    :status="runtimeStatus.text.value"
    @create="sessionDocument.create"
    @open="sessionDocument.open"
  />
  <RuntimeHeader
    :client-count="desktopRuntime.clientCount.value"
    :file-path="sessionDocument.filePath.value"
    :server-error="desktopRuntime.serverError.value"
    :server-label="desktopRuntime.label.value"
    :server-title="desktopRuntime.title.value"
    :server-u-r-l="desktopRuntime.playerURL.value"
    @open="desktopRuntime.openPlayerURL"
  />
  <ApplicationUpdateStatus
    :failure="update.failure.value"
    :show-button="update.showButton.value"
    :silent="update.silent.value"
    :snapshot="update.snapshot.value"
    :status-text="update.statusText.value"
    @show="update.showCurrent"
  />
  <BroadcastControls
    :error="broadcastControls.error.value"
    :focus-request="broadcastControls.focusRequest.value"
    :pending="broadcastControls.pending.value"
    :snapshot="broadcastControls.current.value"
    :status="broadcastControls.status.value"
    @end="broadcastControls.requestEnd"
    @manage="broadcastControls.manageSessions"
    @start="broadcastControls.start"
    @take-off="broadcastControls.requestTakeOff"
  />
  <EndBroadcastDialog
    :open="broadcastControls.endConfirmationOpen.value"
    :pending="broadcastControls.pending.value"
    @cancel="broadcastControls.cancelEnd"
    @confirm="broadcastControls.confirmEnd"
  />
  <TakeOffAirDialog
    :error="broadcastControls.takeOffError.value"
    :open="broadcastControls.takeOffConfirmationOpen.value"
    :pending="broadcastControls.pending.value"
    @cancel="broadcastControls.cancelTakeOff"
    @confirm="broadcastControls.confirmTakeOff"
  />
  <HackControlPanel
    :error="hackControls.error.value"
    :pending="hackControls.pending.value"
    :snapshot="hackControls.current.value"
    :visible="hackControls.visible.value"
    @force="hackControls.forceSuccess"
    @reset="hackControls.reset"
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
  <CreateTerminalDialog />
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
  <GeneratedPasswordDialog :port="port" />
  <PlayerCredentialsDialog :port="port" :snapshot="publicAccess.snapshot.value" @snapshot="publicAccess.applySnapshot" />
  <PublicAccessPanel
    :controls-disabled="publicAccess.controlsDisabled.value"
    :copy-status="publicAccess.copyStatus.value"
    :failure="publicAccess.failure.value"
    :loaded="publicAccess.loaded.value"
    :pending="publicAccess.pending.value"
    :public-u-r-l="publicAccess.publicURL.value"
    :snapshot="publicAccess.snapshot.value"
    @copy="publicAccess.copyURL"
    @settings="publicAccess.openSettings"
    @start="publicAccess.start"
    @stop="publicAccess.stop"
  />
  <PublicAccessSettingsDialog :port="port" :snapshot="publicAccess.snapshot.value" />
  <ProviderTokenDialog :port="port" :snapshot="publicAccess.snapshot.value" @snapshot="publicAccess.applySnapshot" />
  <TerminalGroupDraftDialog :port="port" />
  <TerminalGroupImpactDialog :port="port" />
  <TerminalEditor />
  <TerminalTree />
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
  <TerminalSidebar
    :collapsed-group-i-ds="terminalAuthoring.collapsedGroupIDs.value"
    :focus-request="terminalAuthoring.focusRequest.value"
    :group-revision="terminalAuthoring.revision.value"
    :groups="terminalAuthoring.groups.value"
    :revision="terminalAuthoring.revision.value"
    :terminals="terminalAuthoring.terminals.value"
    @action="terminalAuthoring.action"
    @select="terminalAuthoring.select"
    @toggle="terminalAuthoring.toggle"
  />
  <TerminalSwitchDialog
    :error="terminalSwitch.error.value"
    :pending="terminalSwitch.pending.value"
    :switch-id="terminalSwitch.switchId.value"
    @cancel="terminalSwitch.cancel"
    @discard="terminalSwitch.discard"
    @preserve="terminalSwitch.preserve"
  />
  </OverseerLayout>
</template>
