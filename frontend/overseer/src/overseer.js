'use strict';

const desktopAPI = window.desktopAPI;

// ── State ─────────────────────────────────────────────────
const state = {
  session:        null,   // { version, name, terminals: [{id,name,hackLevel,introText,root}] }
  filePath:       null,
  liveTerminalId: null,
  editTerminalId: null,
  selectedNodeId: null,
  expanded:       new Set(['root']),
  collapsedTerminalGroupIds: new Set(),
  liveHack:       null,   // last known hack-state of the live terminal, or null
  coordination:   null,   // authoritative roster/session/broadcast projection
};

let idCounter = 0;
function uid(prefix) {
  idCounter++;
  return `${prefix}_${Date.now().toString(36)}_${idCounter}`;
}

// ── DOM refs ──────────────────────────────────────────────
const startScreen      = document.getElementById('startScreen');
const startStatus      = document.getElementById('startStatus');
const btnOpenSession   = document.getElementById('btnOpenSession');
const btnNewSession    = document.getElementById('btnNewSession');
const logLocationButtons = Array.from(document.querySelectorAll('[data-action="open-log-location"]'));
const logAccessStatuses = Array.from(document.querySelectorAll('[data-log-access-status]'));
const mainLayout        = document.getElementById('mainLayout');
const sessionFileLabel  = document.getElementById('sessionFileLabel');
const serverUrlEl       = document.getElementById('serverUrl');
const clientCountEl     = document.getElementById('clientCount');
const termList          = document.getElementById('termList');
const saveStatus        = document.getElementById('saveStatus');
const btnUndo           = document.getElementById('btnUndo');
const btnRedo           = document.getElementById('btnRedo');
const btnReapplyEdit    = document.getElementById('btnReapplyEdit');
const btnResetScene     = document.getElementById('btnResetScene');
const editingTermName   = document.getElementById('editingTermName');
const liveFlag          = document.getElementById('liveFlag');
const btnMakeLive       = document.getElementById('btnMakeLive');
const terminalSettingsMenu = document.getElementById('terminalSettingsMenu');
const btnReapplySettings = document.getElementById('btnReapplySettings');
const treeView          = document.getElementById('treeView');
const nodeForm          = document.getElementById('nodeForm');
const toolbarHint       = document.getElementById('toolbarHint');
const btnAddFolder      = document.getElementById('btnAddFolder');
const btnAddCommand     = document.getElementById('btnAddCommand');
const btnAddEntry       = document.getElementById('btnAddEntry');
const btnAddTerminal    = document.getElementById('btnAddTerminal');
const btnCreateTerminalGroup = document.getElementById('btnCreateTerminalGroup');
const terminalGroupError = document.getElementById('terminalGroupError');
const terminalGroupDraftDialog = document.getElementById('terminalGroupDraftDialog');
const terminalGroupDraftForm = document.getElementById('terminalGroupDraftForm');
const terminalGroupNameInput = document.getElementById('terminalGroupNameInput');
const terminalGroupTerminalChoices = document.querySelector('#terminalGroupTerminalChoices .terminal-group-terminal-choice-list');
const terminalGroupTerminalChoiceTemplate = document.getElementById('terminalGroupTerminalChoiceTemplate');
const terminalGroupDestinationSelect = document.getElementById('terminalGroupDestinationSelect');
const terminalGroupImpactDialog = document.getElementById('terminalGroupImpactDialog');
const terminalGroupImpactSummary = document.getElementById('terminalGroupImpactSummary');
const terminalGroupImpactError = document.getElementById('terminalGroupImpactError');
const amendTerminalGroupChangeButton = terminalGroupImpactDialog.querySelector('[data-action="amend-terminal-group-change"]');
const btnStopBroadcast  = document.getElementById('btnStopBroadcast');
const createTerminalDialog = document.getElementById('createTerminalDialog');
const createTerminalForm = document.getElementById('createTerminalForm');
const createTerminalName = document.getElementById('createTerminalName');
const createTerminalError = document.getElementById('createTerminalError');
const btnCancelCreateTerminal = document.getElementById('btnCancelCreateTerminal');
const btnConfirmCreateTerminal = document.getElementById('btnConfirmCreateTerminal');
const takeOffAirDialog = document.getElementById('takeOffAirDialog');
const takeOffAirError = document.getElementById('takeOffAirError');
const btnCancelTakeOffAir = document.getElementById('btnCancelTakeOffAir');
const btnConfirmTakeOffAir = document.getElementById('btnConfirmTakeOffAir');
const hackStatus        = document.getElementById('hackStatus');
const hackStatusLine    = document.getElementById('hackStatusLine');
const btnHackSuccess    = document.getElementById('btnHackSuccess');
const btnResetFailedHack = document.getElementById('btnResetFailedHack');
const hackLevelSelect   = document.getElementById('hackLevelSelect');
const introTextArea     = document.getElementById('introTextArea');
const btnApplySettings  = document.getElementById('btnApplySettings');
const btnCancelSettings = document.getElementById('btnCancelSettings');
const termSettings      = document.getElementById('termSettings');
const broadcastSummary = document.getElementById('broadcastSummary');
const coordinationPanel = document.getElementById('coordinationPanel');
const playerConfigStatus = document.getElementById('playerConfigStatus');
const playerConfigError = document.getElementById('playerConfigError');
const btnOpenPlayerConfig = document.getElementById('btnOpenPlayerConfig');
const btnNewPlayerConfig = document.getElementById('btnNewPlayerConfig');
const btnManagePlayers = document.getElementById('btnManagePlayers');
const btnStartBroadcast = document.getElementById('btnStartBroadcast');
const btnEndBroadcast = document.getElementById('btnEndBroadcast');
const endBroadcastDialog = document.getElementById('endBroadcastDialog');
const btnCancelEndBroadcast = document.getElementById('btnCancelEndBroadcast');
const btnConfirmEndBroadcast = document.getElementById('btnConfirmEndBroadcast');
const coordinationStatus = document.getElementById('coordinationStatus');
const coordinationError = document.getElementById('coordinationError');
const activeLogicalSessionCount = document.getElementById('activeLogicalSessionCount');
const btnManageLogicalSessions = document.getElementById('btnManageLogicalSessions');
const logicalSessionDialog = document.getElementById('logicalSessionDialog');
const btnCloseLogicalSessions = document.getElementById('btnCloseLogicalSessions');
const logicalSessionDialogStatus = document.getElementById('logicalSessionDialogStatus');
const logicalSessionDialogError = document.getElementById('logicalSessionDialogError');
const logicalSessionList = document.getElementById('logicalSessionList');
const logicalSessionRowTemplate = document.getElementById('logicalSessionRowTemplate');
const playerManagementDialog = document.getElementById('playerManagementDialog');
const playerManagementMode = document.getElementById('playerManagementMode');
const playerManagementRoster = document.getElementById('playerManagementRoster');
const playerManagementEmpty = document.getElementById('playerManagementEmpty');
const playerManagementAddForm = document.getElementById('playerManagementAddForm');
const playerNameInput = document.getElementById('playerNameInput');
const playerIntelligenceInput = document.getElementById('playerIntelligenceInput');
const playerHackerPerkAvailability = document.getElementById('playerHackerPerkAvailability');
const btnAddPlayer = document.getElementById('btnAddPlayer');
const playerManagementStatus = document.getElementById('playerManagementStatus');
const playerManagementError = document.getElementById('playerManagementError');
const btnClosePlayerManagement = document.getElementById('btnClosePlayerManagement');
const playerManagementRowTemplate = document.getElementById('playerManagementRowTemplate');
const playerDeleteDialog = document.getElementById('playerDeleteDialog');
const playerDeleteDialogDescription = document.getElementById('playerDeleteDialogDescription');
const btnConfirmPlayerDelete = document.getElementById('btnConfirmPlayerDelete');
const btnCancelPlayerDelete = document.getElementById('btnCancelPlayerDelete');
const terminalSwitchDialog = document.getElementById('terminalSwitchDialog');
const terminalSwitchStatus = document.getElementById('terminalSwitchStatus');
const terminalSwitchError = document.getElementById('terminalSwitchError');
const terminalSwitchButtons = Array.from(document.querySelectorAll('[data-switch-decision]'));
const publicAccessSection = document.getElementById('publicAccessSection');
const publicAccessStateRow = document.querySelector('.public-access-state-row');
const publicAccessSettingsDialog = document.getElementById('publicAccessSettingsDialog');
const publicAccessSetupRequired = document.getElementById('publicAccessSetupRequired');
const publicAccessSettingsError = document.getElementById('publicAccessSettingsError');
const publicAccessGuide = document.getElementById('publicAccessGuide');
const publicAccessForm = document.getElementById('publicAccessForm');
const publicAccessDomain = document.getElementById('publicAccessDomain');
const publicAccessUsernameSummary = document.getElementById('publicAccessUsernameSummary');
const publicAccessPasswordMask = document.getElementById('publicAccessPasswordMask');
const publicAccessProviderSetup = document.getElementById('publicAccessProviderSetup');
const publicAccessProviderConfigured = document.getElementById('publicAccessProviderConfigured');
const publicAccessProviderToken = document.getElementById('publicAccessProviderToken');
const publicAccessProviderPresence = document.getElementById('publicAccessProviderPresence');
const publicAccessPasswordPresence = document.getElementById('publicAccessPasswordPresence');
const publicAccessNgrokDocLinks = document.querySelectorAll('[data-ngrok-doc-url]');
const publicAccessProviderTokenDialog = document.getElementById('publicAccessProviderTokenDialog');
const publicAccessProviderTokenForm = document.getElementById('publicAccessProviderTokenForm');
const publicAccessReplacementProviderToken = document.getElementById('publicAccessReplacementProviderToken');
const publicAccessProviderTokenError = document.getElementById('publicAccessProviderTokenError');
const publicAccessPlayerCredentialsDialog = document.getElementById('publicAccessPlayerCredentialsDialog');
const publicAccessPlayerCredentialsForm = document.getElementById('publicAccessPlayerCredentialsForm');
const publicAccessReplacementUsername = document.getElementById('publicAccessReplacementUsername');
const publicAccessReplacementPlayerPassword = document.getElementById('publicAccessReplacementPlayerPassword');
const publicAccessReplacementPlayerPasswordHint = document.getElementById('publicAccessReplacementPlayerPasswordHint');
const publicAccessPlayerCredentialsError = document.getElementById('publicAccessPlayerCredentialsError');
const publicAccessStatus = document.getElementById('publicAccessStatus');
const publicAccessError = document.getElementById('publicAccessError');
const publicAccessURL = document.getElementById('publicAccessURL');
const publicAccessCopyStatus = document.getElementById('publicAccessCopyStatus');
const publicAccessSettingsCopyStatus = document.getElementById('publicAccessSettingsCopyStatus');
const btnSavePublicAccess = document.getElementById('btnSavePublicAccess');
const btnStartPublicAccess = document.getElementById('btnStartPublicAccess');
const btnStopPublicAccess = document.getElementById('btnStopPublicAccess');
const btnCopyPublicURL = document.getElementById('btnCopyPublicURL');
const btnOpenPublicAccessSettings = document.getElementById('btnOpenPublicAccessSettings');
const btnClosePublicAccessSettings = document.getElementById('btnClosePublicAccessSettings');
const btnCancelPublicAccessSettings = document.getElementById('btnCancelPublicAccessSettings');
const btnOpenPublicAccessProviderToken = document.getElementById('btnOpenPublicAccessProviderToken');
const btnCancelPublicAccessProviderToken = document.getElementById('btnCancelPublicAccessProviderToken');
const btnSavePublicAccessProviderToken = document.getElementById('btnSavePublicAccessProviderToken');
const btnDeletePublicAccessProviderToken = document.getElementById('btnDeletePublicAccessProviderToken');
const btnOpenPublicAccessPlayerCredentials = document.getElementById('btnOpenPublicAccessPlayerCredentials');
const btnSharePublicAccessCredentials = document.getElementById('btnSharePublicAccessCredentials');
const btnCancelPublicAccessPlayerCredentials = document.getElementById('btnCancelPublicAccessPlayerCredentials');
const btnSavePublicAccessPlayerCredentials = document.getElementById('btnSavePublicAccessPlayerCredentials');
const btnDeletePublicAccessPlayerCredentials = document.getElementById('btnDeletePublicAccessPlayerCredentials');
const btnGeneratePlayerPassword = document.getElementById('btnGeneratePlayerPassword');
const generatedPasswordDialog = document.getElementById('generatedPasswordDialog');
const generatedPasswordValue = document.getElementById('generatedPasswordValue');
const btnCopyGeneratedPassword = document.getElementById('btnCopyGeneratedPassword');
const btnDismissGeneratedPassword = document.getElementById('btnDismissGeneratedPassword');
const applicationUpdateStatusPanel = document.getElementById('applicationUpdateStatusPanel');
const applicationUpdateStatus = document.getElementById('applicationUpdateStatus');
const applicationUpdateError = document.getElementById('applicationUpdateError');
const applicationUpdateProgress = document.getElementById('applicationUpdateProgress');
const btnShowApplicationUpdate = document.getElementById('btnShowApplicationUpdate');
const applicationUpdateDialog = document.getElementById('applicationUpdateDialog');
const applicationUpdateInstalledVersion = document.getElementById('applicationUpdateInstalledVersion');
const applicationUpdateAvailableVersion = document.getElementById('applicationUpdateAvailableVersion');
const applicationUpdateReleaseNotes = document.getElementById('applicationUpdateReleaseNotes');
const btnAcceptApplicationUpdate = document.getElementById('btnAcceptApplicationUpdate');
const btnDeferApplicationUpdate = document.getElementById('btnDeferApplicationUpdate');
const applicationUpdateRestartDialog = document.getElementById('applicationUpdateRestartDialog');
const btnRestartApplicationUpdate = document.getElementById('btnRestartApplicationUpdate');
const btnPostponeApplicationUpdate = document.getElementById('btnPostponeApplicationUpdate');
const entryBlockCommandDialog = document.getElementById('entryBlockCommandDialog');
const entryBlockCommandDialogDescription = document.getElementById('entryBlockCommandDialogDescription');
const entryBlockCommandForm = document.getElementById('entryBlockCommandForm');
const entryBlockCommandModeInputs = Array.from(document.querySelectorAll('input[name="entryBlockCommandMode"]'));
const entryBlockExistingCommand = document.getElementById('entryBlockExistingCommand');
const entryBlockCompletedText = document.getElementById('entryBlockCompletedText');
const entryBlockNewCommandName = document.getElementById('entryBlockNewCommandName');
const entryBlockNewCompletedName = document.getElementById('entryBlockNewCompletedName');
const entryBlockNewConfirmationText = document.getElementById('entryBlockNewConfirmationText');
const entryBlockNewResultText = document.getElementById('entryBlockNewResultText');
const entryBlockNewDestinationFolder = document.getElementById('entryBlockNewDestinationFolder');
const entryBlockCommandError = document.getElementById('entryBlockCommandError');
const btnApplyEntryBlockCommand = entryBlockCommandDialog.querySelector('[data-entry-block-command-action="apply"]');
const btnRemoveEntryBlockCommand = entryBlockCommandDialog.querySelector('[data-entry-block-command-action="remove"]');
const btnCancelEntryBlockCommand = entryBlockCommandDialog.querySelector('[data-entry-block-command-action="cancel"]');

const terminalWorkspaceTab = document.getElementById('terminalWorkspaceTab');
const facilityWorkspaceTab = document.getElementById('facilityWorkspaceTab');
const terminalWorkspace = document.getElementById('terminalWorkspace');
const facilityWorkspace = document.getElementById('facilityWorkspace');
const facilityStatus = document.getElementById('facilityStatus');
const facilityValidation = document.getElementById('facilityValidation');
const facilityDeviceList = document.getElementById('facilityDeviceList');
const facilityBindingList = document.getElementById('facilityBindingList');
const facilityConditionList = document.getElementById('facilityConditionList');
const facilityRecoveryProgramList = document.getElementById('facilityRecoveryProgramList');
const facilitySelectionEditor = document.getElementById('facilitySelectionEditor');
const facilitySelectedDeviceName = document.getElementById('facilitySelectedDeviceName');
const facilitySelectionStatus = document.getElementById('facilitySelectionStatus');
const facilitySelectionError = document.getElementById('facilitySelectionError');
const facilityConditionSelectionEditor = document.getElementById('facilityConditionSelectionEditor');
const facilitySelectedConditionSummary = document.getElementById('facilitySelectedConditionSummary');
const btnPreviewFacilityDevice = document.getElementById('btnPreviewFacilityDevice');
const btnResetFacilityDevice = document.getElementById('btnResetFacilityDevice');
const btnPreviewFacilityCondition = document.getElementById('btnPreviewFacilityCondition');
const btnRecoverFacilityCondition = document.getElementById('btnRecoverFacilityCondition');
const btnAddFacilityDevice = document.getElementById('btnAddFacilityDevice');
const btnAddFacilityCondition = document.getElementById('btnAddFacilityCondition');
const btnAddFacilityRecoveryProgram = document.getElementById('btnAddFacilityRecoveryProgram');
const btnAddFacilityBinding = document.getElementById('btnAddFacilityBinding');
const btnInspectFacilityDependencies = document.getElementById('btnInspectFacilityDependencies');
const btnSaveFacilityDeviceChanges = document.getElementById('btnSaveFacilityDeviceChanges');
const btnCancelFacilityDeviceChanges = document.getElementById('btnCancelFacilityDeviceChanges');
const btnDeleteFacilityDevice = document.getElementById('btnDeleteFacilityDevice');
const facilityDeviceDialog = document.getElementById('facilityDeviceDialog');
const facilityDeviceForm = document.getElementById('facilityDeviceForm');
const facilityDeviceID = document.getElementById('facilityDeviceID');
const facilityDeviceName = document.getElementById('facilityDeviceName');
const facilityDeviceKind = document.getElementById('facilityDeviceKind');
const facilityDeviceStates = document.getElementById('facilityDeviceStates');
const facilityDeviceInitialState = document.getElementById('facilityDeviceInitialState');
const facilityDeviceTransitions = document.getElementById('facilityDeviceTransitions');
const facilityDeviceError = document.getElementById('facilityDeviceError');
const btnAddFacilityState = document.getElementById('btnAddFacilityState');
const btnAddFacilityTransition = document.getElementById('btnAddFacilityTransition');
const btnCancelFacilityDevice = document.getElementById('btnCancelFacilityDevice');
const facilityStateRowTemplate = document.getElementById('facilityStateRowTemplate');
const facilityTransitionRowTemplate = document.getElementById('facilityTransitionRowTemplate');
const facilityConditionDialog = document.getElementById('facilityConditionDialog');
const facilityConditionForm = document.getElementById('facilityConditionForm');
const facilityConditionID = document.getElementById('facilityConditionID');
const facilityConditionName = document.getElementById('facilityConditionName');
const facilityConditionCategory = document.getElementById('facilityConditionCategory');
const facilityConditionDevice = document.getElementById('facilityConditionDevice');
const facilityConditionCapability = document.getElementById('facilityConditionCapability');
const facilityConditionPrivateRecovery = document.getElementById('facilityConditionPrivateRecovery');
const facilityConditionInitialWorking = document.getElementById('facilityConditionInitialWorking');
const facilityConditionInitialActive = document.getElementById('facilityConditionInitialActive');
const facilityConditionError = document.getElementById('facilityConditionError');
const btnCancelFacilityCondition = document.getElementById('btnCancelFacilityCondition');
const facilityRecoveryProgramDialog = document.getElementById('facilityRecoveryProgramDialog');
const facilityRecoveryProgramForm = document.getElementById('facilityRecoveryProgramForm');
const facilityRecoveryProgramID = document.getElementById('facilityRecoveryProgramID');
const facilityRecoveryProgramName = document.getElementById('facilityRecoveryProgramName');
const facilityRecoveryProgramDevice = document.getElementById('facilityRecoveryProgramDevice');
const facilityRecoveryProgramTransition = document.getElementById('facilityRecoveryProgramTransition');
const facilityRecoveryProgramError = document.getElementById('facilityRecoveryProgramError');
const btnCancelFacilityRecoveryProgram = document.getElementById('btnCancelFacilityRecoveryProgram');
const facilityBindingDialog = document.getElementById('facilityBindingDialog');
const facilityBindingForm = document.getElementById('facilityBindingForm');
const facilityBindingTerminal = document.getElementById('facilityBindingTerminal');
const facilityBindingNode = document.getElementById('facilityBindingNode');
const facilityBindingDevice = document.getElementById('facilityBindingDevice');
const facilityBindingState = document.getElementById('facilityBindingState');
const facilityBindingText = document.getElementById('facilityBindingText');
const facilityBindingError = document.getElementById('facilityBindingError');
const btnCancelFacilityBinding = document.getElementById('btnCancelFacilityBinding');
const facilityDependencyDialog = document.getElementById('facilityDependencyDialog');
const facilityDependencyList = document.getElementById('facilityDependencyList');
const facilityDependencyStatus = document.getElementById('facilityDependencyStatus');
const facilityDependencyError = document.getElementById('facilityDependencyError');
const btnCloseFacilityDependencies = document.getElementById('btnCloseFacilityDependencies');
const facilityRepairDialog = document.getElementById('facilityRepairDialog');
const facilityRepairError = document.getElementById('facilityRepairError');
const facilityRepairTarget = document.getElementById('facilityRepairTarget');
const facilityRepairImpact = document.getElementById('facilityRepairImpact');
const btnValidateFacilityRepair = document.getElementById('btnValidateFacilityRepair');
const btnCancelFacilityRepair = document.getElementById('btnCancelFacilityRepair');
const btnDeleteFacilityWithoutRepair = document.getElementById('btnDeleteFacilityWithoutRepair');
const btnApplyFacilityRepair = document.getElementById('btnApplyFacilityRepair');
const facilityPreviewDialog = document.getElementById('facilityPreviewDialog');
const facilityPreviewTerminal = document.getElementById('facilityPreviewTerminal');
const facilityPreviewStateLabel = document.getElementById('facilityPreviewStateLabel');
const facilityPreviewState = document.getElementById('facilityPreviewState');
const facilityPreviewActiveLabel = document.getElementById('facilityPreviewActiveLabel');
const facilityPreviewActive = document.getElementById('facilityPreviewActive');
const facilityPreviewSummary = document.getElementById('facilityPreviewSummary');
const facilityPreviewCurrent = document.getElementById('facilityPreviewCurrent');
const facilityPreviewHypothetical = document.getElementById('facilityPreviewHypothetical');
const facilityPreviewStatus = document.getElementById('facilityPreviewStatus');
const facilityPreviewError = document.getElementById('facilityPreviewError');
const btnRefreshFacilityPreview = document.getElementById('btnRefreshFacilityPreview');
const btnCloseFacilityPreview = document.getElementById('btnCloseFacilityPreview');
const facilityDeviceResetDialog = document.getElementById('facilityDeviceResetDialog');
const btnCancelFacilityDeviceReset = document.getElementById('btnCancelFacilityDeviceReset');
const btnConfirmFacilityDeviceReset = document.getElementById('btnConfirmFacilityDeviceReset');
const sceneResetDialog = document.getElementById('sceneResetDialog');
const sceneResetCountsList = document.getElementById('sceneResetCounts');
const sceneResetPending = document.getElementById('sceneResetPending');
const sceneResetStatus = document.getElementById('sceneResetStatus');
const btnCancelSceneReset = document.getElementById('btnCancelSceneReset');
const btnConfirmSceneReset = document.getElementById('btnConfirmSceneReset');
const facilityRecoveryConfirmationDialog = document.getElementById('facilityRecoveryConfirmationDialog');
const btnCancelFacilityRecovery = document.getElementById('btnCancelFacilityRecovery');
const btnConfirmFacilityRecovery = document.getElementById('btnConfirmFacilityRecovery');

let serverUrl = null;
let serverUrlTitle = '';
let newestDurableRevision = 0;
let coordinationCommandPending = false;
let createTerminalSubmitting = false;
let takeOffAirPending = false;
let pendingTerminalSwitch = null;
let startupStatus = null;
let publicAccessSnapshot = null;
let publicAccessCommandPending = false;
let publicAccessSettingsDialogOpener = null;
let publicAccessProviderTokenDialogOpener = null;
let publicAccessPlayerCredentialsDialogOpener = null;
let sessionStateCommandPending = false;
let commandExecutionDialogRequestID = null;
let commandExecutionDecisionRequestID = null;
let commandExecutionDialogMode = null;
let commandExecutionDialogEpoch = 0;
const resolvedCommandExecutionRequestIDs = new Set();
let terminalNavigationDialogRequestID = null;
let terminalNavigationDecisionRequestID = null;
let terminalNavigationDialogEpoch = 0;
const resolvedTerminalNavigationRequestIDs = new Set();
let logicalSessionDialogOpener = null;
let playerManagementOpener = null;
let pendingPlayerDelete = null;
let terminalGroupDraft = null;
let pendingTerminalGroupImpact = null;
let terminalGroupSubmitting = false;
let terminalGroupDialogOpener = null;
let applicationUpdateSnapshot = null;
let applicationUpdateDialogAttemptID = '';
let applicationUpdateDialogOpener = null;
let applicationUpdateCommandPending = false;
let applicationUpdateRestartDialogAttemptID = '';
let applicationUpdateRestartDialogOpener = null;
let applicationUpdateRestartCommandPending = false;
let latestRenderedApplicationUpdateRevision = -1;
let entryBlockCommandDraft = null;
let selectedFacilityDeviceID = '';
let selectedFacilityConditionID = '';
let facilityPreviewSelection = null;
let facilityPreviewGeneration = 0;
let facilityOperationOpener = null;
let pendingFacilityRepair = null;
let editingFacilityDeviceID = '';
let editingFacilityConditionID = '';
let editingFacilityBinding = null;
let editingFacilityProgramID = '';
let facilityStoryStep = 0;
let facilityBindingKind = null;
let facilityBindingActionRequests = null;
let facilityBindingProgram = null;
let btnAddFacilityBindingAction = null;
let facilityBindingCompletedName = null;
let facilityBindingConfirmationText = null;
let facilityBindingBlock = null;
let facilityConditionScope = null;
let facilityConditionTerminal = null;
let btnEditFacilityDeviceGraph = null;
const promptedApplicationUpdateRevisions = new Set();
const promptedApplicationUpdateRestartRevisions = new Set();
const suppressedApplicationUpdateAttempts = new Set();

const commandExecutionDialog = document.createElement('dialog');
commandExecutionDialog.className = 'terminal-switch-dialog command-execution-dialog';
commandExecutionDialog.id = 'commandExecutionDialog';
commandExecutionDialog.hidden = true;
commandExecutionDialog.setAttribute('aria-modal', 'true');
commandExecutionDialog.setAttribute('aria-labelledby', 'commandExecutionDialogTitle');
commandExecutionDialog.setAttribute('aria-describedby', 'commandExecutionDialogDescription commandExecutionFacilityImpact commandExecutionDialogStatus commandExecutionDialogError');
commandExecutionDialog.innerHTML = `
  <div class="terminal-switch-dialog-panel">
    <h2 class="terminal-switch-dialog-title" id="commandExecutionDialogTitle">ПОДТВЕРЖДЕНИЕ КОМАНДЫ</h2>
    <p class="terminal-switch-dialog-description" id="commandExecutionDialogDescription"></p>
    <div class="terminal-navigation-summary" id="commandExecutionFacilityImpact" aria-label="Влияние на состояние объекта" hidden></div>
    <div class="terminal-switch-actions" role="group" aria-label="Решение мастера по выполнению команды" style="grid-template-columns:repeat(2,minmax(0,1fr))">
      <button class="btn btn-primary" id="btnApproveCommandExecution" type="button">ОДОБРИТЬ</button>
      <button class="btn btn-danger" id="btnRejectCommandExecution" type="button">ОТКЛОНИТЬ</button>
    </div>
    <div class="terminal-switch-status" id="commandExecutionDialogStatus" role="status" aria-live="polite" aria-atomic="true"></div>
    <div class="terminal-switch-error" id="commandExecutionDialogError" role="alert" aria-live="assertive" aria-atomic="true" hidden></div>
  </div>`;
document.body.appendChild(commandExecutionDialog);
const commandExecutionDialogDescription = document.getElementById('commandExecutionDialogDescription');
const commandExecutionFacilityImpact = document.getElementById('commandExecutionFacilityImpact');
const commandExecutionDialogStatus = document.getElementById('commandExecutionDialogStatus');
const commandExecutionDialogError = document.getElementById('commandExecutionDialogError');
const btnApproveCommandExecution = document.getElementById('btnApproveCommandExecution');
const btnRejectCommandExecution = document.getElementById('btnRejectCommandExecution');

const terminalNavigationDialog = document.createElement('dialog');
terminalNavigationDialog.className = 'terminal-switch-dialog terminal-navigation-dialog';
terminalNavigationDialog.id = 'terminalNavigationDialog';
terminalNavigationDialog.hidden = true;
terminalNavigationDialog.setAttribute('aria-modal', 'true');
terminalNavigationDialog.setAttribute('aria-labelledby', 'terminalNavigationDialogTitle');
terminalNavigationDialog.innerHTML = `
  <div class="terminal-switch-dialog-panel">
    <h2 class="terminal-switch-dialog-title" id="terminalNavigationDialogTitle">ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ</h2>
    <div class="terminal-navigation-summary" id="terminalNavigationSummary"></div>
    <div class="terminal-switch-actions" role="group" aria-label="Решение мастера по переходу" style="grid-template-columns:repeat(2,minmax(0,1fr))">
      <button class="btn btn-primary" id="btnApproveTerminalNavigation" type="button">ОДОБРИТЬ</button>
      <button class="btn btn-danger" id="btnRejectTerminalNavigation" type="button">ОТКЛОНИТЬ</button>
    </div>
    <div class="terminal-switch-status" id="terminalNavigationStatus" role="status" aria-live="polite"></div>
    <div class="terminal-switch-error" id="terminalNavigationError" role="alert" hidden></div>
  </div>`;
document.body.appendChild(terminalNavigationDialog);
const terminalNavigationSummary = document.getElementById('terminalNavigationSummary');
const terminalNavigationStatus = document.getElementById('terminalNavigationStatus');
const terminalNavigationError = document.getElementById('terminalNavigationError');
const btnApproveTerminalNavigation = document.getElementById('btnApproveTerminalNavigation');
const btnRejectTerminalNavigation = document.getElementById('btnRejectTerminalNavigation');

let resetConfirmationResolve = null;
const resetConfirmationDialog = document.createElement('dialog');
resetConfirmationDialog.className = 'terminal-switch-dialog command-state-reset-dialog';
resetConfirmationDialog.id = 'resetConfirmationDialog';
resetConfirmationDialog.hidden = true;
resetConfirmationDialog.setAttribute('aria-modal', 'true');
resetConfirmationDialog.setAttribute('aria-labelledby', 'resetConfirmationDialogTitle');
resetConfirmationDialog.setAttribute('aria-describedby', 'resetConfirmationDialogDescription');
resetConfirmationDialog.innerHTML = `
  <div class="terminal-switch-dialog-panel">
    <h2 class="terminal-switch-dialog-title" id="resetConfirmationDialogTitle">ПОДТВЕРЖДЕНИЕ СБРОСА</h2>
    <p class="terminal-switch-dialog-description" id="resetConfirmationDialogDescription"></p>
    <div class="terminal-switch-actions" role="group" aria-label="Подтверждение сброса состояния команды" style="grid-template-columns:repeat(2,minmax(0,1fr))">
      <button class="btn btn-danger" id="btnConfirmCommandStateReset" type="button">ПОДТВЕРДИТЬ</button>
      <button class="btn" id="btnCancelCommandStateReset" type="button">ОТМЕНИТЬ</button>
    </div>
  </div>`;
document.body.appendChild(resetConfirmationDialog);
const resetConfirmationDialogDescription = document.getElementById('resetConfirmationDialogDescription');
const btnConfirmCommandStateReset = document.getElementById('btnConfirmCommandStateReset');
const btnCancelCommandStateReset = document.getElementById('btnCancelCommandStateReset');

function finishResetConfirmation(confirmed) {
  const resolve = resetConfirmationResolve;
  resetConfirmationResolve = null;
  if (resetConfirmationDialog.open) resetConfirmationDialog.close();
  resetConfirmationDialog.hidden = true;
  if (resolve) resolve(confirmed);
}

function confirmCommandStateReset(message) {
  if (resetConfirmationResolve) return Promise.resolve(false);
  resetConfirmationDialogDescription.textContent = message;
  resetConfirmationDialog.hidden = false;
  resetConfirmationDialog.showModal();
  btnCancelCommandStateReset.focus();
  return new Promise(resolve => { resetConfirmationResolve = resolve; });
}

btnConfirmCommandStateReset.addEventListener('click', () => finishResetConfirmation(true));
btnCancelCommandStateReset.addEventListener('click', () => finishResetConfirmation(false));
resetConfirmationDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  finishResetConfirmation(false);
});

function renderStartupPresentation(status) {
  startupStatus = status && typeof status === 'object' ? status : {};
  const info = startupStatus.serverInfo && typeof startupStatus.serverInfo === 'object'
    ? startupStatus.serverInfo
    : null;
  const startupError = typeof startupStatus.startupError === 'string' ? startupStatus.startupError : '';
  const tunnelError = typeof info?.tunnelError === 'string' ? info.tunnelError : '';
  const fatal = !info && Boolean(startupError);

  btnOpenSession.disabled = fatal;
  btnNewSession.disabled = fatal;
  if (fatal) {
    startStatus.dataset.state = 'failed';
    startStatus.textContent = `ЗАПУСК НЕ ЗАВЕРШЁН: ${startupError}`;
  } else if (info?.tunnel && info.url) {
    startStatus.dataset.state = 'ready-public';
    startStatus.textContent = `ГОТОВО · ПУБЛИЧНЫЙ И ЛОКАЛЬНЫЙ ДОСТУП${info.localUrl ? ` · ${info.localUrl}` : ''}`;
  } else if (info) {
    const warning = tunnelError || startupError;
    startStatus.dataset.state = warning ? 'warning' : 'ready-local';
    startStatus.textContent = warning
      ? `ЛОКАЛЬНЫЙ РЕЖИМ ГОТОВ · ПУБЛИЧНЫЙ ДОСТУП НЕДОСТУПЕН: ${warning}`
      : `ЛОКАЛЬНЫЙ РЕЖИМ ГОТОВ · ${info.localUrl || info.url}`;
  } else {
    startStatus.dataset.state = 'starting';
    startStatus.textContent = 'ЗАПУСК ЛОКАЛЬНОГО СЕРВЕРА…';
  }
}

// ── Server info / connection count ─────────────────────────
desktopAPI.onServerInfo((info) => {
  renderStartupPresentation({ ...(startupStatus || {}), serverInfo: info });
  const publicUrl = info.tunnel && info.url ? info.url : '';
  const localUrl = info.localUrl || (!info.tunnel ? info.url : '');
  const tunnelUnavailable = info.tunnel && !publicUrl;

  serverUrl = publicUrl || localUrl || null;
  serverUrlEl.classList.toggle('server-url-error', Boolean(info.tunnelError) || tunnelUnavailable);

  if (info.tunnelError) {
    serverUrlEl.textContent = localUrl
      ? `NGROK: ОШИБКА · ЛОКАЛЬНО: ${localUrl}`
      : 'NGROK: ОШИБКА';
    serverUrlTitle = localUrl
      ? `${info.tunnelError}\nЛокальная ссылка остаётся доступна (нажмите, чтобы открыть)`
      : info.tunnelError;
  } else if (publicUrl) {
    serverUrlEl.textContent = publicUrl;
    serverUrlTitle = localUrl
      ? `Публичная ссылка (нажмите, чтобы открыть)\nЛокально: ${localUrl}`
      : 'Публичная ссылка (нажмите, чтобы открыть)';
  } else if (localUrl) {
    serverUrlEl.textContent = localUrl;
    serverUrlTitle = tunnelUnavailable
      ? 'Публичная ссылка недоступна; локальная ссылка остаётся доступна'
      : 'Локальная ссылка (нажмите, чтобы открыть)';
  } else {
    serverUrlEl.textContent = '—';
    serverUrlTitle = 'Адрес игроков пока недоступен';
  }
  serverUrlEl.title = serverUrlTitle;
});
desktopAPI.onClientCount((count) => {
  clientCountEl.textContent = count;
});
desktopAPI.onHackState((hack) => {
  state.liveHack = hack;
  renderHackStatus();
});
desktopAPI.onCoordinationState((coordination) => {
  applyCoordinationState(coordination);
  renderCoordination();
  if (sceneResetDialog.open) renderSceneResetPending();
  if (state.session) {
    renderTermList();
    renderTreeHeader();
    renderHackStatus();
  }
});
if (typeof desktopAPI.onSessionState === 'function') {
  desktopAPI.onSessionState((event) => {
    const revision = Number(event?.revision);
    if (!event?.session || !Number.isSafeInteger(revision)) return;
    saveStatus.dataset.sessionStateRevision = String(revision);
    updateSessionStateEvidenceDescription();
    if (!state.session || revision <= newestDurableRevision) return;
    applyLiveSession(event.session, revision);
    renderAll();
  });
}
void desktopAPI.getRuntimeStatus().then(renderStartupPresentation);

// ── Application update: nonblocking status and explicit offer ──
const applicationUpdateStatusLabels = Object.freeze({
  checking: 'ПРОВЕРКА ОБНОВЛЕНИЙ…',
  available: 'ДОСТУПНО ОБНОВЛЕНИЕ ПРИЛОЖЕНИЯ',
  deferred: 'ОБНОВЛЕНИЕ ОТЛОЖЕНО ДО СЛЕДУЮЩЕГО ЗАПУСКА',
  downloading: 'ЗАГРУЗКА ОБНОВЛЕНИЯ…',
  verifying: 'ПРОВЕРКА ЗАГРУЖЕННОГО ОБНОВЛЕНИЯ…',
  staging: 'ПОДГОТОВКА ОБНОВЛЕНИЯ…',
  'ready-to-restart': 'ОБНОВЛЕНИЕ ГОТОВО К ПЕРЕЗАПУСКУ',
  applying: 'ПРИМЕНЕНИЕ ОБНОВЛЕНИЯ…',
  failed: 'НЕ УДАЛОСЬ ПОДГОТОВИТЬ ОБНОВЛЕНИЕ',
});
const applicationUpdateFailureStageLabels = Object.freeze({
  check: 'ПРОВЕРКА ОБНОВЛЕНИЙ',
  download: 'ЗАГРУЗКА ОБНОВЛЕНИЯ',
  verify: 'ПРОВЕРКА ЗАГРУЖЕННОГО ОБНОВЛЕНИЯ',
  stage: 'ПОДГОТОВКА ОБНОВЛЕНИЯ',
  apply: 'ПРИМЕНЕНИЕ ОБНОВЛЕНИЯ',
  relaunch: 'ПЕРЕЗАПУСК ОБНОВЛЁННОГО ПРИЛОЖЕНИЯ',
  recovery: 'ВОССТАНОВЛЕНИЕ РАБОЧЕЙ ВЕРСИИ',
});
const MAX_APPLICATION_UPDATE_TEXT = 16_384;

function boundedApplicationUpdateText(value, fallback = '') {
  if (typeof value !== 'string') return fallback;
  if (value.length <= MAX_APPLICATION_UPDATE_TEXT) return value;
  return `${value.slice(0, MAX_APPLICATION_UPDATE_TEXT)}\n\n[Описание выпуска сокращено]`;
}

function applicationUpdatePromptKey(snapshot) {
  return `${snapshot.attemptId}:${snapshot.revision}`;
}

function applicationUpdateFailureText(snapshot) {
  const stage = applicationUpdateFailureStageLabels[snapshot.failedStage]
    || 'ОБНОВЛЕНИЕ ПРИЛОЖЕНИЯ';
  const message = boundedApplicationUpdateText(
    snapshot.errorMessage,
    'Операция обновления не завершена.',
  );
  const recoveryAction = boundedApplicationUpdateText(
    snapshot.recoveryAction,
    'Продолжайте работу и повторите попытку при следующем запуске.',
  );
  return `ЭТАП: ${stage}.\n${message}\n${recoveryAction}`;
}

function restoreApplicationUpdateFocus() {
  const opener = applicationUpdateDialogOpener;
  applicationUpdateDialogOpener = null;
  if (opener?.isConnected && typeof opener.focus === 'function') opener.focus();
}

function closeApplicationUpdateDialog() {
  applicationUpdateDialogAttemptID = '';
  if (applicationUpdateDialog.open) applicationUpdateDialog.close();
  applicationUpdateDialog.hidden = true;
  queueMicrotask(restoreApplicationUpdateFocus);
}

function restoreApplicationUpdateRestartFocus() {
  const opener = applicationUpdateRestartDialogOpener;
  applicationUpdateRestartDialogOpener = null;
  if (opener?.isConnected && typeof opener.focus === 'function') opener.focus();
}

function closeApplicationUpdateRestartDialog() {
  applicationUpdateRestartDialogAttemptID = '';
  if (applicationUpdateRestartDialog.open) applicationUpdateRestartDialog.close();
  applicationUpdateRestartDialog.hidden = true;
  queueMicrotask(restoreApplicationUpdateRestartFocus);
}

function renderApplicationUpdateOffer(snapshot) {
  applicationUpdateInstalledVersion.textContent = boundedApplicationUpdateText(snapshot.installedVersion, '—');
  applicationUpdateAvailableVersion.textContent = boundedApplicationUpdateText(snapshot.availableVersion, '—');
  applicationUpdateReleaseNotes.textContent = boundedApplicationUpdateText(
    snapshot.releaseNotes,
    'Для этого выпуска описание не предоставлено.',
  );
}

function showApplicationUpdateOffer(snapshot, { automatic = false } = {}) {
  if (!snapshot?.attemptId || snapshot.state !== 'available') return;
  if (suppressedApplicationUpdateAttempts.has(snapshot.attemptId)) return;

  renderApplicationUpdateOffer(snapshot);
  if (applicationUpdateDialog.open) {
    applicationUpdateDialogAttemptID = snapshot.attemptId;
    return;
  }
  if (automatic && document.querySelector('dialog[open]')) {
    btnShowApplicationUpdate.hidden = false;
    return;
  }

  const key = applicationUpdatePromptKey(snapshot);
  if (automatic && promptedApplicationUpdateRevisions.has(key)) return;
  promptedApplicationUpdateRevisions.add(key);
  applicationUpdateDialogAttemptID = snapshot.attemptId;
  applicationUpdateDialogOpener = document.activeElement;
  applicationUpdateDialog.hidden = false;
  applicationUpdateDialog.showModal();
  btnShowApplicationUpdate.hidden = true;
  btnDeferApplicationUpdate.focus();
}

function showApplicationUpdateRestart(snapshot, { automatic = false } = {}) {
  if (!snapshot?.attemptId || snapshot.state !== 'ready-to-restart') return;
  if (applicationUpdateRestartDialog.open) {
    applicationUpdateRestartDialogAttemptID = snapshot.attemptId;
    return;
  }
  if (automatic && document.querySelector('dialog[open]')) {
    btnShowApplicationUpdate.hidden = false;
    return;
  }

  const key = applicationUpdatePromptKey(snapshot);
  if (automatic && promptedApplicationUpdateRestartRevisions.has(key)) return;
  promptedApplicationUpdateRestartRevisions.add(key);
  applicationUpdateRestartDialogAttemptID = snapshot.attemptId;
  applicationUpdateRestartDialogOpener = document.activeElement;
  applicationUpdateRestartDialog.hidden = false;
  applicationUpdateRestartDialog.showModal();
  btnShowApplicationUpdate.hidden = true;
  btnPostponeApplicationUpdate.focus();
}

function renderApplicationUpdateProgress(snapshot, stateName) {
  const progressVisible = stateName === 'downloading'
    || stateName === 'verifying'
    || stateName === 'staging';
  applicationUpdateProgress.hidden = !progressVisible;
  applicationUpdateProgress.removeAttribute('max');
  applicationUpdateProgress.removeAttribute('value');
  if (!progressVisible) return;

  if (stateName === 'downloading'
    && Number.isSafeInteger(snapshot.downloadSize)
    && snapshot.downloadSize > 0) {
    const downloaded = Number.isSafeInteger(snapshot.bytesDownloaded)
      ? Math.max(0, Math.min(snapshot.bytesDownloaded, snapshot.downloadSize))
      : 0;
    applicationUpdateProgress.max = snapshot.downloadSize;
    applicationUpdateProgress.value = downloaded;
    applicationUpdateProgress.setAttribute(
      'aria-label',
      `Загрузка обновления: ${downloaded} из ${snapshot.downloadSize} байт`,
    );
    return;
  }

  applicationUpdateProgress.setAttribute(
    'aria-label',
    stateName === 'verifying'
      ? 'Проверка загруженного обновления'
      : stateName === 'staging'
        ? 'Подготовка обновления к перезапуску'
        : 'Загрузка обновления',
  );
}

function renderApplicationUpdateSnapshot(snapshot, { prompt = true } = {}) {
  if (!snapshot || typeof snapshot !== 'object') return;
  const revision = Number(snapshot.revision);
  if (!Number.isSafeInteger(revision) || revision < 0) return;
  if (revision < latestRenderedApplicationUpdateRevision) return;
  if (revision === latestRenderedApplicationUpdateRevision
    && applicationUpdateSnapshot
    && (snapshot.attemptId !== applicationUpdateSnapshot.attemptId
      || snapshot.state !== applicationUpdateSnapshot.state)) return;
  latestRenderedApplicationUpdateRevision = revision;
  applicationUpdateSnapshot = snapshot;
  const stateName = typeof snapshot.state === 'string' ? snapshot.state : '';
  const silent = stateName === '' || stateName === 'disabled' || stateName === 'idle' || stateName === 'current';
  const attemptSuppressed = suppressedApplicationUpdateAttempts.has(snapshot.attemptId);

  applicationUpdateStatusPanel.hidden = silent;
  applicationUpdateStatusPanel.dataset.state = stateName;
  applicationUpdateStatus.textContent = applicationUpdateStatusLabels[stateName] || '';
  renderApplicationUpdateProgress(snapshot, stateName);
  applicationUpdateError.textContent = stateName === 'failed'
    ? applicationUpdateFailureText(snapshot)
    : '';
  applicationUpdateError.hidden = !applicationUpdateError.textContent;
  const offerCanOpen = stateName === 'available' && !attemptSuppressed;
  const restartCanOpen = stateName === 'ready-to-restart';
  btnShowApplicationUpdate.hidden = (!offerCanOpen && !restartCanOpen)
    || applicationUpdateDialog.open
    || applicationUpdateRestartDialog.open;
  btnShowApplicationUpdate.setAttribute(
    'aria-controls',
    restartCanOpen ? 'applicationUpdateRestartDialog' : 'applicationUpdateDialog',
  );

  if (!offerCanOpen && applicationUpdateDialog.open) closeApplicationUpdateDialog();
  if (!restartCanOpen && applicationUpdateRestartDialog.open) closeApplicationUpdateRestartDialog();

  if (offerCanOpen) {
    renderApplicationUpdateOffer(snapshot);
    if (prompt) showApplicationUpdateOffer(snapshot, { automatic: true });
  } else if (restartCanOpen && prompt) {
    showApplicationUpdateRestart(snapshot, { automatic: true });
  }
}

function setApplicationUpdateDecisionPending(pending) {
  applicationUpdateCommandPending = pending;
  btnAcceptApplicationUpdate.disabled = pending;
  btnDeferApplicationUpdate.disabled = pending;
  applicationUpdateDialog.setAttribute('aria-busy', String(pending));
}

async function resolveApplicationUpdateOffer(decision) {
  if (applicationUpdateCommandPending) return;
  const snapshot = applicationUpdateSnapshot;
  if (!snapshot?.attemptId || snapshot.attemptId !== applicationUpdateDialogAttemptID) return;

  setApplicationUpdateDecisionPending(true);
  const result = await desktopAPI.resolveApplicationUpdateOffer({
    attemptId: snapshot.attemptId,
    decision,
  });
  setApplicationUpdateDecisionPending(false);

  if (result?.ok !== true) {
    renderApplicationUpdateSnapshot(result?.snapshot || snapshot, { prompt: false });
    applicationUpdateError.textContent = boundedApplicationUpdateText(
      result?.error,
      'Не удалось сохранить решение об обновлении.',
    );
    applicationUpdateError.hidden = false;
    btnDeferApplicationUpdate.focus();
    return;
  }

  suppressedApplicationUpdateAttempts.add(snapshot.attemptId);
  closeApplicationUpdateDialog();
  const resultSnapshot = result?.snapshot;
  if (resultSnapshot
    && (resultSnapshot.revision > snapshot.revision
      || resultSnapshot.state !== snapshot.state)) {
    renderApplicationUpdateSnapshot(resultSnapshot, { prompt: false });
    return;
  }
  if (decision === 'defer') {
    renderApplicationUpdateSnapshot({ ...snapshot, state: 'deferred' }, { prompt: false });
    return;
  }
  applicationUpdateStatus.textContent = 'ПОДГОТОВКА ОБНОВЛЕНИЯ ЗАПРОШЕНА…';
}

function setApplicationUpdateRestartPending(pending) {
  applicationUpdateRestartCommandPending = pending;
  btnRestartApplicationUpdate.disabled = pending;
  btnPostponeApplicationUpdate.disabled = pending;
  applicationUpdateRestartDialog.setAttribute('aria-busy', String(pending));
}

async function resolveApplicationUpdateRestart(decision) {
  if (applicationUpdateRestartCommandPending) return;
  const snapshot = applicationUpdateSnapshot;
  if (!snapshot?.attemptId || snapshot.attemptId !== applicationUpdateRestartDialogAttemptID) return;

  setApplicationUpdateRestartPending(true);
  const result = await desktopAPI.resolveApplicationUpdateRestart({
    attemptId: snapshot.attemptId,
    decision,
  });
  setApplicationUpdateRestartPending(false);

  if (result?.ok !== true) {
    renderApplicationUpdateSnapshot(result?.snapshot || snapshot, { prompt: false });
    applicationUpdateError.textContent = boundedApplicationUpdateText(
      result?.error,
      'Не удалось сохранить решение о перезапуске.',
    );
    applicationUpdateError.hidden = false;
    btnPostponeApplicationUpdate.focus();
    return;
  }

  closeApplicationUpdateRestartDialog();
  renderApplicationUpdateSnapshot(result?.snapshot || snapshot, { prompt: false });
}

btnShowApplicationUpdate.addEventListener('click', () => {
  if (applicationUpdateSnapshot?.state === 'ready-to-restart') {
    showApplicationUpdateRestart(applicationUpdateSnapshot);
    return;
  }
  showApplicationUpdateOffer(applicationUpdateSnapshot);
});
btnAcceptApplicationUpdate.addEventListener('click', () => {
  void resolveApplicationUpdateOffer('accept');
});
btnDeferApplicationUpdate.addEventListener('click', () => {
  void resolveApplicationUpdateOffer('defer');
});
applicationUpdateDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  void resolveApplicationUpdateOffer('defer');
});
applicationUpdateDialog.addEventListener('close', () => {
  applicationUpdateDialog.hidden = true;
});
btnRestartApplicationUpdate.addEventListener('click', () => {
  void resolveApplicationUpdateRestart('restart');
});
btnPostponeApplicationUpdate.addEventListener('click', () => {
  void resolveApplicationUpdateRestart('postpone');
});
applicationUpdateRestartDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  void resolveApplicationUpdateRestart('postpone');
});
applicationUpdateRestartDialog.addEventListener('close', () => {
  applicationUpdateRestartDialog.hidden = true;
});
if (typeof desktopAPI.onApplicationUpdateStatus === 'function') {
  desktopAPI.onApplicationUpdateStatus(renderApplicationUpdateSnapshot);
}

serverUrlEl.addEventListener('click', async () => {
  const requestedUrl = serverUrl;
  if (!requestedUrl) return;

  // The frontend never opens a URL directly. The Go command parses the final
  // value and rejects malformed or non-HTTP(S) protocols at the privilege edge.
  const result = await desktopAPI.openUrl(requestedUrl);
  if (requestedUrl === serverUrl && result && result.ok === false) {
    const detail = result.error ? `: ${result.error}` : '';
    serverUrlEl.title = `${serverUrlTitle}\nНе удалось открыть ссылку${detail}`;
  }
});

// ── Public access: trusted settings and explicit lifecycle ──
const publicAccessStateLabels = Object.freeze({
  stopped: 'ОСТАНОВЛЕН',
  starting: 'ЗАПУСК…',
  ready: 'ГОТОВ',
  stopping: 'ОСТАНОВКА…',
  error: 'ОШИБКА',
});

const secureCredentialStoreFailures = Object.freeze({
  secret_store_locked: 'Unlock the secure credential store and try again.',
  secret_store_denied: 'Allow secure credential store access and try again.',
  secret_store_unavailable: 'The secure credential store is unavailable; local access remains available.',
});

function publicAccessFailureMessage(status) {
  return secureCredentialStoreFailures[status.errorCategory]
    || status.errorMessage
    || 'ПУБЛИЧНЫЙ ДОСТУП НЕДОСТУПЕН';
}

function renderSecretPresence(element, presence, presentLabel = 'СОХРАНЕН') {
  element.dataset.presence = presence;
  element.textContent = presence === 'present'
    ? presentLabel
    : presence === 'absent' ? 'НЕ СОХРАНЕН' : 'НЕДОСТУПЕН';
}

function publicAccessDisplayURL(preferences, status) {
  if (status.state === 'ready' && status.publicUrl) return status.publicUrl;
  if (status.state === 'error') return '';
  const reservedDomain = String(preferences.reservedDomain || '').trim();
  if (!reservedDomain) return '';
  return /^https?:\/\//i.test(reservedDomain) ? reservedDomain : `https://${reservedDomain}`;
}

function syncPublicAccessPlayerCredentialControls() {
  const configured = publicAccessSnapshot?.playerPasswordPresence === 'present';
  const transitioning = publicAccessSnapshot?.status?.state === 'starting' ||
    publicAccessSnapshot?.status?.state === 'stopping';
  const disabled = publicAccessCommandPending || transitioning;
  const username = publicAccessReplacementUsername.value.trim();
  const password = publicAccessReplacementPlayerPassword.value;
  const passwordValid = password === '' || Array.from(password).length >= 8;
  publicAccessReplacementUsername.disabled = disabled;
  publicAccessReplacementPlayerPassword.disabled = disabled;
  btnGeneratePlayerPassword.disabled = disabled;
  btnCancelPublicAccessPlayerCredentials.disabled = publicAccessCommandPending;
  btnDeletePublicAccessPlayerCredentials.hidden = !configured;
  btnDeletePublicAccessPlayerCredentials.disabled = disabled || !configured;
  btnSavePublicAccessPlayerCredentials.disabled = disabled || username === '' ||
    !passwordValid || (!configured && password === '');
  publicAccessReplacementPlayerPasswordHint.textContent = configured
    ? 'Оставьте пустым, чтобы сохранить текущий пароль.'
    : 'Введите пароль не короче 8 символов или сгенерируйте новый.';
}

function renderPublicAccess(snapshot) {
  if (!snapshot || typeof snapshot !== 'object') return;
  if (publicAccessSnapshot) {
    const candidateGeneration = Number(snapshot.status?.generation || 0);
    const currentGeneration = Number(publicAccessSnapshot.status?.generation || 0);
    const candidateRevision = Number(snapshot.status?.settingsRevision || snapshot.preferences?.revision || 0);
    const currentRevision = Number(publicAccessSnapshot.status?.settingsRevision || publicAccessSnapshot.preferences?.revision || 0);
    if (candidateGeneration < currentGeneration ||
      (candidateGeneration === currentGeneration && candidateRevision < currentRevision)) return;
  }
  publicAccessSection.hidden = false;
  publicAccessSnapshot = snapshot;
  const preferences = snapshot.preferences || {};
  const status = snapshot.status || {};
  publicAccessDomain.value = preferences.reservedDomain || '';
  publicAccessUsernameSummary.textContent = preferences.username || 'players';
  const providerTokenConfigured = snapshot.providerTokenPresence === 'present';
  const playerPasswordConfigured = snapshot.playerPasswordPresence === 'present';
  publicAccessProviderSetup.hidden = providerTokenConfigured;
  publicAccessProviderConfigured.hidden = !providerTokenConfigured;
  if (providerTokenConfigured) publicAccessProviderToken.value = '';
  renderSecretPresence(publicAccessProviderPresence, snapshot.providerTokenPresence, 'НАСТРОЕН');
  renderSecretPresence(publicAccessPasswordPresence, snapshot.playerPasswordPresence);
  publicAccessPasswordPresence.hidden = playerPasswordConfigured;
  publicAccessPasswordMask.hidden = !playerPasswordConfigured;
  publicAccessPasswordMask.textContent = playerPasswordConfigured ? '*****' : '';
  publicAccessStatus.textContent = publicAccessStateLabels[status.state] || 'ЗАГРУЗКА…';
  publicAccessStatus.dataset.state = status.state || 'loading';
  publicAccessStateRow.dataset.state = status.state || 'loading';
  publicAccessStatus.dataset.generation = String(Number(status.generation || 0));
  publicAccessStatus.dataset.settingsRevision = String(Number(status.settingsRevision || preferences.revision || 0));
  const publicFailure = publicAccessFailureMessage(status);
  publicAccessError.textContent = status.state === 'error'
    ? `${publicFailure} · ЛОКАЛЬНЫЙ РЕЖИМ ПРОДОЛЖАЕТ РАБОТАТЬ`
    : '';
  publicAccessError.hidden = publicAccessError.textContent === '';
  const displayURL = publicAccessDisplayURL(preferences, status);
  publicAccessURL.textContent = displayURL || 'ПОЯВИТСЯ ПОСЛЕ ЗАПУСКА';
  publicAccessURL.dataset.available = String(Boolean(displayURL));
  btnCopyPublicURL.hidden = !displayURL;
  const transitioning = status.state === 'starting' || status.state === 'stopping';
  const disabled = publicAccessCommandPending || transitioning;
  for (const control of [
    publicAccessDomain, publicAccessProviderToken, publicAccessReplacementProviderToken,
    btnSavePublicAccess,
    btnOpenPublicAccessProviderToken, btnSavePublicAccessProviderToken,
    btnDeletePublicAccessProviderToken, btnOpenPublicAccessPlayerCredentials,
  ]) control.disabled = disabled;
  const stopping = status.state === 'ready' || status.state === 'stopping';
  btnStartPublicAccess.hidden = stopping;
  btnStopPublicAccess.hidden = !stopping;
  btnStartPublicAccess.textContent = status.state === 'starting' ? 'ЗАПУСК…' : 'ВКЛЮЧИТЬ ДОСТУП';
  btnStopPublicAccess.textContent = status.state === 'stopping' ? 'ОСТАНОВКА…' : 'ОСТАНОВИТЬ ДОСТУП';
  btnStartPublicAccess.disabled = disabled || status.state === 'ready';
  btnStopPublicAccess.disabled = disabled || status.state === 'stopped';
  btnOpenPublicAccessSettings.disabled = disabled;
  btnClosePublicAccessSettings.disabled = publicAccessCommandPending;
  btnCancelPublicAccessSettings.disabled = publicAccessCommandPending;
  btnCancelPublicAccessProviderToken.disabled = publicAccessCommandPending;
  btnSavePublicAccessProviderToken.disabled = disabled || publicAccessReplacementProviderToken.value.trim() === '';
  btnSharePublicAccessCredentials.disabled = disabled || !playerPasswordConfigured ||
    publicAccessUsernameSummary.textContent.trim() === '';
  syncPublicAccessPlayerCredentialControls();
}

function publicAccessRevision() {
  return Number(publicAccessSnapshot?.preferences?.revision || 0);
}

async function copyTransientText(value, successMessage, statusElement = publicAccessCopyStatus) {
  if (!value) return false;
  let copied = false;
  try {
    if (typeof navigator.clipboard?.writeText === 'function') {
      await navigator.clipboard.writeText(value);
      copied = true;
    }
  } catch {
    // Packaged WebViews may deny the browser Clipboard API. The native Wails
    // runtime is the bounded fallback and returns no copy of the value.
  }
  if (!copied) {
    copied = await desktopAPI.writeClipboardText(value);
  }
  if (copied) {
    statusElement.textContent = successMessage;
    return true;
  }
  statusElement.textContent = 'НЕ УДАЛОСЬ СКОПИРОВАТЬ';
  return false;
}

function showPublicAccessSettings({ setupRequired = false } = {}) {
  if (publicAccessCommandPending) return;
  publicAccessSettingsDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnOpenPublicAccessSettings;
  publicAccessSetupRequired.hidden = !setupRequired;
  if (setupRequired) publicAccessGuide.open = true;
  publicAccessSettingsError.textContent = '';
  publicAccessSettingsError.hidden = true;
  publicAccessSettingsCopyStatus.textContent = '';
  publicAccessSettingsDialog.hidden = false;
  if (typeof publicAccessSettingsDialog.showModal === 'function' && !publicAccessSettingsDialog.open) {
    publicAccessSettingsDialog.showModal();
  } else {
    publicAccessSettingsDialog.setAttribute('open', '');
  }
  queueMicrotask(() => btnClosePublicAccessSettings.focus());
}

function hidePublicAccessSettings() {
  if (publicAccessCommandPending) return;
  if (typeof publicAccessSettingsDialog.close === 'function' && publicAccessSettingsDialog.open) {
    publicAccessSettingsDialog.close();
  } else {
    publicAccessSettingsDialog.removeAttribute('open');
  }
  publicAccessSettingsDialog.hidden = true;
  publicAccessSetupRequired.hidden = true;
  publicAccessSettingsError.textContent = '';
  publicAccessSettingsError.hidden = true;
  publicAccessSettingsCopyStatus.textContent = '';
  publicAccessProviderToken.value = '';
  renderPublicAccess(publicAccessSnapshot);
  const opener = publicAccessSettingsDialogOpener;
  publicAccessSettingsDialogOpener = null;
  if (opener?.isConnected) opener.focus();
}

function publicAccessNonSecretDraft() {
  return {
    reservedDomain: publicAccessDomain.value,
  };
}

function restorePublicAccessNonSecretDraft(draft) {
  publicAccessDomain.value = draft.reservedDomain;
}

function confirmActivePublicAccessChange() {
  return publicAccessSnapshot?.status?.state !== 'ready' || window.confirm(
    'ПУБЛИЧНЫЙ ДОСТУП АКТИВЕН. ИЗМЕНЕНИЕ ОСТАНОВИТ И ПЕРЕЗАПУСТИТ ССЫЛКУ. ПРОДОЛЖИТЬ?',
  );
}

function showPublicAccessProviderTokenDialog() {
  if (publicAccessCommandPending || publicAccessSnapshot?.providerTokenPresence !== 'present') return;
  publicAccessProviderTokenDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnOpenPublicAccessProviderToken;
  publicAccessReplacementProviderToken.value = '';
  publicAccessProviderTokenError.textContent = '';
  publicAccessProviderTokenError.hidden = true;
  publicAccessProviderTokenDialog.hidden = false;
  if (typeof publicAccessProviderTokenDialog.showModal === 'function' && !publicAccessProviderTokenDialog.open) {
    publicAccessProviderTokenDialog.showModal();
  } else {
    publicAccessProviderTokenDialog.setAttribute('open', '');
  }
  queueMicrotask(() => publicAccessReplacementProviderToken.focus());
}

function hidePublicAccessProviderTokenDialog() {
  if (publicAccessCommandPending) return;
  if (typeof publicAccessProviderTokenDialog.close === 'function' && publicAccessProviderTokenDialog.open) {
    publicAccessProviderTokenDialog.close();
  } else {
    publicAccessProviderTokenDialog.removeAttribute('open');
  }
  publicAccessProviderTokenDialog.hidden = true;
  publicAccessReplacementProviderToken.value = '';
  publicAccessProviderTokenError.textContent = '';
  publicAccessProviderTokenError.hidden = true;
  const opener = publicAccessProviderTokenDialogOpener;
  publicAccessProviderTokenDialogOpener = null;
  if (opener?.isConnected && !opener.closest('[hidden]')) {
    opener.focus();
  } else {
    publicAccessProviderToken.focus();
  }
}

async function runPublicAccessProviderTokenMutation({ replacementProviderToken = '', deleteProviderToken = false }) {
  if (publicAccessCommandPending || !confirmActivePublicAccessChange()) return;
  const preferences = publicAccessSnapshot?.preferences || {};
  const draft = publicAccessNonSecretDraft();
  const request = {
    expectedRevision: publicAccessRevision(),
    enabledPreference: false,
    reservedDomain: preferences.reservedDomain || '',
    username: preferences.username || 'players',
    replacementProviderToken,
    deleteProviderToken,
    replacementPlayerPassword: '',
    deletePlayerPassword: false,
  };
  publicAccessProviderTokenError.textContent = '';
  publicAccessProviderTokenError.hidden = true;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const pending = desktopAPI.savePublicAccessSettings(request);
  publicAccessReplacementProviderToken.value = '';
  request.replacementProviderToken = '';
  const result = await pending;
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  restorePublicAccessNonSecretDraft(draft);
  if (!result.ok) {
    publicAccessProviderTokenError.textContent = result.error || 'НЕ УДАЛОСЬ ИЗМЕНИТЬ ТОКЕН NGROK';
    publicAccessProviderTokenError.hidden = false;
    return;
  }
  hidePublicAccessProviderTokenDialog();
}

function showPublicAccessPlayerCredentialsDialog() {
  if (publicAccessCommandPending) return;
  publicAccessPlayerCredentialsDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnOpenPublicAccessPlayerCredentials;
  publicAccessReplacementUsername.value = publicAccessSnapshot?.preferences?.username || 'players';
  publicAccessReplacementPlayerPassword.value = '';
  publicAccessPlayerCredentialsError.textContent = '';
  publicAccessPlayerCredentialsError.hidden = true;
  syncPublicAccessPlayerCredentialControls();
  publicAccessPlayerCredentialsDialog.hidden = false;
  if (typeof publicAccessPlayerCredentialsDialog.showModal === 'function' && !publicAccessPlayerCredentialsDialog.open) {
    publicAccessPlayerCredentialsDialog.showModal();
  } else {
    publicAccessPlayerCredentialsDialog.setAttribute('open', '');
  }
  queueMicrotask(() => publicAccessReplacementUsername.focus());
}

function hidePublicAccessPlayerCredentialsDialog() {
  if (publicAccessCommandPending) return;
  if (typeof publicAccessPlayerCredentialsDialog.close === 'function' && publicAccessPlayerCredentialsDialog.open) {
    publicAccessPlayerCredentialsDialog.close();
  } else {
    publicAccessPlayerCredentialsDialog.removeAttribute('open');
  }
  publicAccessPlayerCredentialsDialog.hidden = true;
  publicAccessReplacementPlayerPassword.value = '';
  publicAccessPlayerCredentialsError.textContent = '';
  publicAccessPlayerCredentialsError.hidden = true;
  const opener = publicAccessPlayerCredentialsDialogOpener;
  publicAccessPlayerCredentialsDialogOpener = null;
  if (opener?.isConnected && !opener.closest('[hidden]')) {
    opener.focus();
  } else {
    btnOpenPublicAccessPlayerCredentials.focus();
  }
}

async function runPublicAccessPlayerCredentialMutation({
  username,
  replacementPlayerPassword = '',
  deletePlayerPassword = false,
}) {
  if (publicAccessCommandPending || !confirmActivePublicAccessChange()) return;
  const preferences = publicAccessSnapshot?.preferences || {};
  const mainDraft = publicAccessNonSecretDraft();
  const request = {
    expectedRevision: publicAccessRevision(),
    enabledPreference: false,
    reservedDomain: preferences.reservedDomain || '',
    username,
    replacementProviderToken: '',
    deleteProviderToken: false,
    replacementPlayerPassword,
    deletePlayerPassword,
  };
  publicAccessPlayerCredentialsError.textContent = '';
  publicAccessPlayerCredentialsError.hidden = true;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const pending = desktopAPI.savePublicAccessSettings(request);
  publicAccessReplacementPlayerPassword.value = '';
  request.replacementPlayerPassword = '';
  const result = await pending;
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  restorePublicAccessNonSecretDraft(mainDraft);
  if (!result.ok) {
    publicAccessReplacementUsername.value = username;
    publicAccessPlayerCredentialsError.textContent = result.error || 'НЕ УДАЛОСЬ СОХРАНИТЬ ДАННЫЕ ИГРОКОВ';
    publicAccessPlayerCredentialsError.hidden = false;
    syncPublicAccessPlayerCredentialControls();
    return;
  }
  hidePublicAccessPlayerCredentialsDialog();
}

function showGeneratedPassword(oneTimeValue) {
  let transientValue = oneTimeValue;
  generatedPasswordValue.textContent = transientValue;
  const clearAndClose = () => {
    transientValue = '';
    generatedPasswordValue.textContent = '';
    btnCopyGeneratedPassword.onclick = null;
    btnDismissGeneratedPassword.onclick = null;
    generatedPasswordDialog.oncancel = null;
    if (generatedPasswordDialog.open) generatedPasswordDialog.close();
    btnGeneratePlayerPassword.focus();
  };
  btnCopyGeneratedPassword.onclick = async () => {
    await copyTransientText(transientValue, 'ПАРОЛЬ СКОПИРОВАН', publicAccessSettingsCopyStatus);
    clearAndClose();
  };
  btnDismissGeneratedPassword.onclick = clearAndClose;
  generatedPasswordDialog.oncancel = (event) => {
    event.preventDefault();
    clearAndClose();
  };
  generatedPasswordDialog.showModal();
  btnCopyGeneratedPassword.focus();
}

// The facade owns exact `public-access-status` event ordering and stale-snapshot suppression.
desktopAPI.onPublicAccessStatus(renderPublicAccess);

publicAccessForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  if (publicAccessCommandPending) return;
  if (!confirmActivePublicAccessChange()) return;
  const request = {
    expectedRevision: publicAccessRevision(),
    enabledPreference: false,
    reservedDomain: publicAccessDomain.value,
    username: publicAccessSnapshot?.preferences?.username || 'players',
    replacementProviderToken: publicAccessProviderToken.value,
    deleteProviderToken: false,
    replacementPlayerPassword: '',
    deletePlayerPassword: false,
  };
  publicAccessSettingsError.textContent = '';
  publicAccessSettingsError.hidden = true;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const pending = desktopAPI.savePublicAccessSettings(request);
  publicAccessProviderToken.value = '';
  request.replacementProviderToken = '';
  const result = await pending;
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  if (!result.ok) {
    publicAccessDomain.value = request.reservedDomain;
    publicAccessSettingsError.textContent = result.error || 'НЕ УДАЛОСЬ СОХРАНИТЬ НАСТРОЙКИ';
    publicAccessSettingsError.hidden = false;
    return;
  }
  hidePublicAccessSettings();
});

async function runPublicAccessLifecycle(command) {
  if (publicAccessCommandPending) return;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const result = await command({ expectedRevision: publicAccessRevision() });
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  if (!result.ok) {
    publicAccessError.textContent = result.error || 'ОПЕРАЦИЯ ПУБЛИЧНОГО ДОСТУПА НЕ ВЫПОЛНЕНА';
    publicAccessError.hidden = false;
  }
}

btnStartPublicAccess.addEventListener('click', () => {
  const configured = publicAccessSnapshot?.providerTokenPresence === 'present'
    && publicAccessSnapshot?.playerPasswordPresence === 'present';
  if (!configured) {
    showPublicAccessSettings({ setupRequired: true });
    return;
  }
  void runPublicAccessLifecycle(desktopAPI.startPublicAccess);
});
btnStopPublicAccess.addEventListener('click', () => runPublicAccessLifecycle(desktopAPI.stopPublicAccess));
btnOpenPublicAccessSettings.addEventListener('click', () => showPublicAccessSettings());
btnClosePublicAccessSettings.addEventListener('click', hidePublicAccessSettings);
btnCancelPublicAccessSettings.addEventListener('click', hidePublicAccessSettings);
publicAccessSettingsDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePublicAccessSettings();
});
btnOpenPublicAccessProviderToken.addEventListener('click', showPublicAccessProviderTokenDialog);
btnCancelPublicAccessProviderToken.addEventListener('click', hidePublicAccessProviderTokenDialog);
publicAccessProviderTokenDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePublicAccessProviderTokenDialog();
});
publicAccessProviderTokenForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const replacementProviderToken = publicAccessReplacementProviderToken.value;
  if (!replacementProviderToken.trim()) return;
  void runPublicAccessProviderTokenMutation({ replacementProviderToken });
});
btnDeletePublicAccessProviderToken.addEventListener('click', () => {
  void runPublicAccessProviderTokenMutation({ deleteProviderToken: true });
});
publicAccessReplacementProviderToken.addEventListener('input', () => {
  btnSavePublicAccessProviderToken.disabled = publicAccessCommandPending ||
    publicAccessReplacementProviderToken.value.trim() === '';
});
btnOpenPublicAccessPlayerCredentials.addEventListener('click', showPublicAccessPlayerCredentialsDialog);
btnCancelPublicAccessPlayerCredentials.addEventListener('click', hidePublicAccessPlayerCredentialsDialog);
publicAccessPlayerCredentialsDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePublicAccessPlayerCredentialsDialog();
});
publicAccessPlayerCredentialsForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const username = publicAccessReplacementUsername.value.trim();
  const replacementPlayerPassword = publicAccessReplacementPlayerPassword.value;
  const configured = publicAccessSnapshot?.playerPasswordPresence === 'present';
  if (!username) return;
  if ((!configured && replacementPlayerPassword === '') ||
    (replacementPlayerPassword !== '' && Array.from(replacementPlayerPassword).length < 8)) {
    publicAccessPlayerCredentialsError.textContent = 'ПАРОЛЬ ДОЛЖЕН СОДЕРЖАТЬ НЕ МЕНЕЕ 8 СИМВОЛОВ';
    publicAccessPlayerCredentialsError.hidden = false;
    return;
  }
  void runPublicAccessPlayerCredentialMutation({ username, replacementPlayerPassword });
});
btnDeletePublicAccessPlayerCredentials.addEventListener('click', () => {
  void runPublicAccessPlayerCredentialMutation({ username: 'players', deletePlayerPassword: true });
});
publicAccessReplacementUsername.addEventListener('input', syncPublicAccessPlayerCredentialControls);
publicAccessReplacementPlayerPassword.addEventListener('input', syncPublicAccessPlayerCredentialControls);
btnGeneratePlayerPassword.addEventListener('click', async () => {
  if (publicAccessCommandPending) return;
  if (!confirmActivePublicAccessChange()) return;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const result = await desktopAPI.generatePlayerPassword({ expectedRevision: publicAccessRevision() });
  publicAccessCommandPending = false;
  if (!result.ok || !result.generatedPassword) {
    renderPublicAccess(publicAccessSnapshot);
    publicAccessPlayerCredentialsError.textContent = result.error || 'НЕ УДАЛОСЬ СОЗДАТЬ ПАРОЛЬ';
    publicAccessPlayerCredentialsError.hidden = false;
    return;
  }
  showGeneratedPassword(result.generatedPassword);
  const refreshed = await desktopAPI.getPublicAccess();
  renderPublicAccess(refreshed);
});
btnCopyPublicURL.addEventListener('click', () => {
  if (publicAccessURL.dataset.available !== 'true') return;
  void copyTransientText(publicAccessURL.textContent, 'URL СКОПИРОВАН');
});
btnSharePublicAccessCredentials.addEventListener('click', async () => {
  if (publicAccessCommandPending || publicAccessSnapshot?.playerPasswordPresence !== 'present') return;
  publicAccessSettingsCopyStatus.textContent = '';
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const result = await desktopAPI.copyPublicAccessCredentials();
  publicAccessCommandPending = false;
  renderPublicAccess(publicAccessSnapshot);
  publicAccessSettingsCopyStatus.textContent = result.ok
    ? 'ЛОГИН И ПАРОЛЬ СКОПИРОВАНЫ'
    : (result.error || 'НЕ УДАЛОСЬ СКОПИРОВАТЬ ДАННЫЕ ИГРОКОВ');
});
for (const link of publicAccessNgrokDocLinks) {
  link.addEventListener('click', () => {
    void desktopAPI.openUrl(link.dataset.ngrokDocUrl);
  });
}

let logAccessPending = false;
const safeLogAccessErrors = new Set([
  'Could not open the application log directory.',
  'The application log directory is unavailable.',
]);

function logAccessMessage(result) {
  const value = result && typeof result === 'object' ? result : {};
  const directoryPath = typeof value.directoryPath === 'string' ? value.directoryPath : '';
  const activeLogPath = typeof value.activeLogPath === 'string' ? value.activeLogPath : '';
  // An absent active file means retained output is degraded and records reach only standard error.
  const retention = activeLogPath ? '' : ' · ЗАПИСЬ ЛОГОВ В ФАЙЛ НЕДОСТУПНА';
  if (value.ok !== true) {
    const error = safeLogAccessErrors.has(value.error)
      ? value.error
      : 'НЕ УДАЛОСЬ ОТКРЫТЬ ЛОГИ';
    // A failed open names the directory so the Overseer can navigate there manually.
    return `${error}${directoryPath ? ` · ${directoryPath}` : ''}${retention}`;
  }
  const path = activeLogPath || directoryPath;
  return `ЛОГИ ОТКРЫТЫ${path ? ` · ${path}` : ''}${retention}`;
}

for (const button of logLocationButtons) {
  button.addEventListener('click', async () => {
    if (logAccessPending) return;
    logAccessPending = true;
    logLocationButtons.forEach(control => { control.disabled = true; });
    logAccessStatuses.forEach(status => { status.textContent = 'ОТКРЫТИЕ ПАПКИ ЛОГОВ…'; });
    try {
      const result = await desktopAPI.openLogLocation();
      const message = logAccessMessage(result);
      logAccessStatuses.forEach(status => { status.textContent = message; });
    } finally {
      logLocationButtons.forEach(control => { control.disabled = false; });
      logAccessPending = false;
    }
  });
}
// ── Start screen: open / new session ───────────────────────
btnOpenSession.addEventListener('click', async () => {
  const res = await desktopAPI.openSession();
  if (!res.ok) {
    if (res.error) startStatus.textContent = 'Ошибка: ' + res.error;
    return;
  }
  await loadSession(res.session, res.filePath, res.authoringRevision);
});

btnNewSession.addEventListener('click', async () => {
  const res = await desktopAPI.newSession();
  if (!res.ok) {
    if (res.error) startStatus.textContent = 'Ошибка: ' + res.error;
    return;
  }
  await loadSession(res.session, res.filePath, res.authoringRevision);
});

// ── What players see (R11) ──────────────────────────────────
// playerView caches the server's explanation of what players see, keyed by
// terminal and node. The newest request wins.
const playerView = { requested: 0, applied: 0, inFlight: 0, nodes: new Map() };

function playerViewNode(terminalID, nodeID) {
  return playerView.nodes.get(`${terminalID}/${nodeID}`) || null;
}

async function refreshPlayerView() {
  if (!state.session || !state.filePath) return;
  const request = ++playerView.requested;
  const generation = authoring.generation;
  playerView.inFlight++;
  renderCommandRules();
  const result = await desktopAPI.describePresentation({ expectedAuthoringRevision: 0 });
  playerView.inFlight--;
  if (result.ok && generation === authoring.generation && request > playerView.applied) {
    playerView.applied = request;
    playerView.nodes = new Map();
    for (const terminal of result.terminals) {
      for (const node of terminal.nodes || []) playerView.nodes.set(`${terminal.terminalId}/${node.nodeId}`, node);
    }
    renderTree();
  }
  renderCommandRules();
}

// ── Command form lists (FR-008–FR-010) ──────────────────────
const PRESENTATION_RULE_LABELS = {
  'authored-name': 'ИСХОДНОЕ НАЗВАНИЕ',
  'name-after-execution': 'НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ',
  'state-name': 'НАЗВАНИЕ ПО СОСТОЯНИЮ',
  'state-visibility': 'ВИДИМОСТЬ ПО СОСТОЯНИЮ',
  'diagnostic-forced-visibility': 'ВИДИМОСТЬ ПРИ НЕИСПРАВНОСТИ',
  'state-availability': 'ДОСТУПНОСТЬ ПО СОСТОЯНИЮ',
  'diagnostic-capability-block': 'БЛОКИРОВКА НЕИСПРАВНОСТЬЮ',
  'initial-text': 'ИСХОДНЫЙ ТЕКСТ',
  'text-after-execution': 'ТЕКСТ ПОСЛЕ ВЫПОЛНЕНИЯ',
  'state-text': 'ТЕКСТ ПО СОСТОЯНИЮ',
  'diagnostic-substitution': 'ЗАМЕНА ПРИ НЕИСПРАВНОСТИ',
};

const PRESENTATION_RULE_BINDINGS = {
  'state-name': 'menu-name', 'state-visibility': 'visibility',
  'state-availability': 'availability', 'state-text': 'entry-content',
};

function facilityTransitionEffect(request) {
  const device = facilityDeviceByID(request.deviceId);
  const transition = device?.transitions?.find(value => value.id === request.transitionId);
  if (!transition) return `${device?.name || request.deviceId}: переход не найден`;
  return `${device.name}: ${facilityStateName(device, transition.sourceStateId)} → ${facilityStateName(device, transition.destinationStateId)}`;
}

// commandEffectItems lists what approving the authored command changes.
function commandEffectItems(term, node) {
  if (node.terminalTransition) {
    const targetID = node.terminalTransition.targetTerminalId;
    const target = state.session.terminals.find(candidate => candidate.id === targetID);
    return [{ kind: 'terminal-transition', text: `ПЕРЕХОД: ${target?.name || targetID}` }];
  }
  const change = node.stateChange;
  if (!change) return [];
  const items = [];
  const action = change.facilityAction;
  for (const request of action?.transitions?.transitions || []) {
    items.push({ kind: 'transition', text: facilityTransitionEffect(request) });
  }
  if (action?.recoveryProgramId) {
    const program = authoredFacility().recoveryPrograms.find(value => value.id === action.recoveryProgramId);
    items.push({ kind: 'recovery', text: `ПРОГРАММА ВОССТАНОВЛЕНИЯ: ${program?.name || action.recoveryProgramId}` });
    for (const request of program?.transitions || []) items.push({ kind: 'transition', text: facilityTransitionEffect(request) });
  }
  if (change.completedName) items.push({ kind: 'completed-name', text: `НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ: ${change.completedName}` });
  const blockChange = change.entryContentChange;
  if (blockChange?.blockId) {
    const location = entryBlockLocation(term, blockChange.blockId);
    items.push({
      kind: 'entry-block',
      text: `${location?.entry.name || 'Запись'} · блок ${(location?.index ?? 0) + 1}: ${blockChange.completedText}`,
    });
  }
  return items;
}

function presentationConditionText(term, condition) {
  if (condition?.deviceState) {
    const device = facilityDeviceByID(condition.deviceState.deviceId);
    return `КОГДА ${device?.name || condition.deviceState.deviceId} = ${facilityStateName(device, condition.deviceState.stateId)}`;
  }
  if (condition?.commandExecuted) {
    const command = locateNode(term.root, condition.commandExecuted.commandId)?.node;
    return command?.id === state.selectedNodeId ? 'ПОСЛЕ ВЫПОЛНЕНИЯ' : `ПОСЛЕ ВЫПОЛНЕНИЯ «${command?.name || condition.commandExecuted.commandId}»`;
  }
  if (condition?.diagnosticActive) {
    const id = condition.diagnosticActive.conditionId;
    const fault = authoredFacility().conditions.find(value => value.id === id);
    return `КОГДА АКТИВНА «${fault?.name || id}»`;
  }
  return 'ВСЕГДА';
}

function presentationRuleValue(rule) {
  const effects = { visible: 'ПОКАЗАТЬ', available: 'РАЗРЕШИТЬ', unavailable: 'ЗАПРЕТИТЬ' };
  return effects[rule.effect] || rule.text || '';
}

// openPresentationRuleEditor opens where rule is authored: a node form
// field, the binding dialog that ОБЪЕКТЫ uses, or the condition dialog.
function openPresentationRuleEditor(term, ownerID, blockID, rule) {
  const condition = rule.condition || {};
  if (condition.diagnosticActive) {
    openFacilityConditionDialog(condition.diagnosticActive.conditionId);
    return;
  }
  const property = PRESENTATION_RULE_BINDINGS[rule.kind];
  if (property) {
    const binding = facilityBindingCatalog().find(value => value.terminal.id === term.id
      && value.node.id === ownerID && value.property === property && (value.block?.id || '') === blockID
      && value.variant.when?.deviceId === condition.deviceState?.deviceId
      && value.variant.when?.stateId === condition.deviceState?.stateId
      && (!['menu-name', 'entry-content'].includes(property) || value.variant.text === rule.text));
    if (binding) openFacilityBindingDialog(binding);
    return;
  }
  const fieldNodeID = rule.kind === 'text-after-execution' ? condition.commandExecuted?.commandId : ownerID;
  if (fieldNodeID && fieldNodeID !== state.selectedNodeId) {
    state.selectedNodeId = fieldNodeID;
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  }
  const fields = {
    'authored-name': 'fldName', 'name-after-execution': 'fldCompletedName',
    'text-after-execution': 'fldCompletedEntryBlockContent',
  };
  document.getElementById(fields[rule.kind] || '')?.focus();
}

// renderCommandRules fills #commandRules from the cached explanation for
// the selected command, grouped by target, lowest precedence first.
function renderCommandRules() {
  const section = document.getElementById('commandRules');
  if (!section) return;
  section.setAttribute('aria-busy', String(playerView.inFlight > 0));
  const term = getEditTerminal();
  const node = term && state.selectedNodeId ? locateNode(term.root, state.selectedNodeId)?.node : null;
  const seen = node ? playerViewNode(term.id, node.id) : null;
  const targets = seen ? [
    ['name', node.id, '', 'НАЗВАНИЕ', seen.nameRules],
    ['visibility', node.id, '', 'ВИДИМОСТЬ', seen.visibilityRules],
    ['availability', node.id, '', 'ДОСТУПНОСТЬ', seen.availabilityRules],
  ] : [];
  const blockID = node?.stateChange?.entryContentChange?.blockId;
  const location = blockID ? entryBlockLocation(term, blockID) : null;
  if (location) {
    const block = playerViewNode(term.id, location.entry.id)?.blocks?.find(value => value.blockId === blockID);
    targets.push([`block:${blockID}`, location.entry.id, blockID, `${location.entry.name} · БЛОК ${location.index + 1}`, block?.textRules]);
  }
  const container = section.querySelector('.command-rule-targets');
  const groups = [];
  for (const [target, ownerID, ownerBlockID, heading, rules] of targets) {
    if (!rules?.length) continue;
    const ordered = [...rules].sort((left, right) => left.precedence - right.precedence);
    const group = document.createElement('section');
    group.className = 'command-rule-target';
    group.dataset.target = target;
    const title = document.createElement('div');
    title.className = 'command-rule-target-heading';
    title.textContent = heading;
    const list = document.createElement('ul');
    for (const rule of ordered) {
      const item = document.createElement('li');
      item.className = 'command-rule';
      item.dataset.kind = rule.kind;
      const text = document.createElement('span');
      text.textContent = `${PRESENTATION_RULE_LABELS[rule.kind] || rule.kind} · ${presentationConditionText(term, rule.condition)} · ${presentationRuleValue(rule)}`;
      item.appendChild(text);
      if (rule.playersSee) {
        const mark = document.createElement('span');
        mark.className = 'rule-players-see';
        mark.textContent = 'ВИДЯТ ИГРОКИ';
        item.appendChild(mark);
      }
      const lower = [...new Set(ordered.filter(value => value.precedence < rule.precedence)
        .map(value => PRESENTATION_RULE_LABELS[value.kind] || value.kind))];
      if (rule.conditionHolds && lower.length) {
        const overrides = document.createElement('span');
        overrides.className = 'rule-overrides';
        overrides.textContent = `ПЕРЕКРЫВАЕТ: ${lower.join(', ')}`;
        item.appendChild(overrides);
      }
      const edit = document.createElement('button');
      edit.className = 'btn btn-mini btn-secondary';
      edit.type = 'button';
      edit.textContent = 'ИЗМЕНИТЬ';
      edit.addEventListener('click', () => openPresentationRuleEditor(term, ownerID, ownerBlockID, rule));
      item.appendChild(edit);
      list.appendChild(item);
    }
    group.append(title, list);
    groups.push(group);
  }
  if (!groups.length && playerView.inFlight > 0) return;
  if (!groups.length) {
    const empty = document.createElement('div');
    empty.className = 'command-mode-hint';
    empty.textContent = seen ? 'ИГРОКИ ВИДЯТ ИСХОДНОЕ НАЗВАНИЕ' : 'ПРАВИЛА ПОЯВЯТСЯ ПОСЛЕ СОХРАНЕНИЯ';
    groups.push(empty);
  }
  container.replaceChildren(...groups);
}

async function loadSession(session, filePath, authoringRevision) {
  newestDurableRevision = 0;
  delete saveStatus.dataset.savedRevision;
  state.session        = session;
  resetAuthoring(session, authoringRevision);
  renderSaveStatus();
  resetFacilitySelection();
  state.filePath        = filePath;
  state.liveTerminalId  = state.coordination?.broadcast?.activeTerminalId || null;
  state.editTerminalId  = (session.terminals[0] && session.terminals[0].id) || null;
  state.selectedNodeId  = null;
  state.expanded         = new Set(['root']);
  state.collapsedTerminalGroupIds = new Set();
  state.liveHack         = null;

  sessionFileLabel.textContent = filePath;
  startScreen.style.display = 'none';
  mainLayout.style.display  = 'flex';
  playerView.nodes = new Map();
  renderAll();
  void refreshPlayerView();

  if (session.playerConfig) {
    await runPlayerConfigCommand(
      () => desktopAPI.loadReferencedPlayerConfig(),
      'КОНФИГУРАЦИЯ ИГРОКОВ ЗАГРУЖЕНА'
    );
  } else {
    setPlayerConfigError('ВЫБЕРИТЕ ИЛИ СОЗДАЙТЕ КОНФИГУРАЦИЮ ИГРОКОВ');
  }
}

// ── Authoring: one save model ───────────────────────────────
// Every applied edit is one SaveAuthoring request. Requests go one at a time
// and in order; a rejected edit reverts only itself. The server keeps every
// live value (executed commands, device states, fault activity), so authoring
// never overwrites live play.
const AUTHORING_HISTORY_LIMIT = 20;
const authoring = {
  generation: 0,
  revision: 0,
  lastSaved: null,
  lastSavedAt: null,
  queue: [],
  inFlight: null,
  held: [],
  error: null,
  historyError: null,
  historyTrimmed: false,
  undo: [],
  redo: [],
  nextID: 0,
  idleWaiters: [],
};

function resetAuthoring(session, authoringRevision) {
  authoring.generation++;
  authoring.revision = Number(authoringRevision) || 1;
  authoring.lastSaved = session ? structuredClone(session) : null;
  authoring.lastSavedAt = null;
  authoring.queue = [];
  authoring.inFlight = null;
  authoring.held = [];
  authoring.error = null;
  authoring.historyError = null;
  authoring.historyTrimmed = false;
  authoring.undo = [];
  authoring.redo = [];
  unappliedInputs.clear();
  for (const resolve of authoring.idleWaiters.splice(0)) resolve();
}

function authoringBusy() {
  return Boolean(authoring.inFlight || authoring.queue.length);
}

// authoringIdle resolves once no save is queued or in flight.
function authoringIdle() {
  if (!authoringBusy()) return Promise.resolve();
  return new Promise(resolve => authoring.idleWaiters.push(resolve));
}

// mergeLiveValues copies the server-owned live values of source into target.
function mergeLiveValues(target, source) {
  if (!target || !source) return target;
  const sourceTerminals = new Map((source.terminals || []).map(terminal => [terminal.id, terminal]));
  for (const terminal of target.terminals || []) {
    const states = sourceTerminals.get(terminal.id)?.commandStates;
    if (states && Object.keys(states).length) terminal.commandStates = structuredClone(states);
    else delete terminal.commandStates;
  }
  if (target.facility && source.facility) {
    target.facility.revision = source.facility.revision;
    const devices = new Map((source.facility.devices || []).map(device => [device.id, device]));
    for (const device of target.facility.devices || []) {
      if (devices.has(device.id)) device.currentStateId = devices.get(device.id).currentStateId;
    }
    const conditions = new Map((source.facility.conditions || []).map(condition => [condition.id, condition]));
    for (const condition of target.facility.conditions || []) {
      if (conditions.has(condition.id)) condition.currentActive = conditions.get(condition.id).currentActive;
    }
  }
  return target;
}

// authoredKey is a stable comparison key for the authored configuration: the
// session without live values or terminal groups, with sorted keys and
// without empty values, so local and server documents compare equal.
function authoredKey(session) {
  if (!session) return '';
  const copy = structuredClone(session);
  delete copy.terminalGroups;
  for (const terminal of copy.terminals || []) delete terminal.commandStates;
  if (copy.facility) {
    delete copy.facility.revision;
    for (const device of copy.facility.devices || []) delete device.currentStateId;
    for (const condition of copy.facility.conditions || []) delete condition.currentActive;
  }
  const normalize = value => {
    if (Array.isArray(value)) return value.map(normalize);
    if (!value || typeof value !== 'object') return value;
    const result = {};
    for (const key of Object.keys(value).sort()) {
      const item = normalize(value[key]);
      const empty = item === undefined || item === null || item === ''
        || (Array.isArray(item) && !item.length) || (typeof item === 'object' && !Object.keys(item).length);
      if (!empty) result[key] = item;
    }
    return result;
  };
  return JSON.stringify(normalize(copy));
}

// replaceAuthoredContent swaps the authored configuration of session for that
// of source while keeping session's live values and terminal groups.
function replaceAuthoredContent(session, source) {
  const replacement = mergeLiveValues(structuredClone(source), session);
  replacement.terminalGroups = session.terminalGroups;
  for (const key of Object.keys(session)) delete session[key];
  Object.assign(session, replacement);
}

// applyLiveSession installs a server result that changed live values only.
function applyLiveSession(session, revision = 0) {
  if (!session || !state.session) return;
  mergeLiveValues(state.session, session);
  if (authoring.lastSaved) mergeLiveValues(authoring.lastSaved, session);
  // A live change may make a rejected undo or redo possible again.
  authoring.historyError = null;
  newestDurableRevision = Math.max(newestDurableRevision, Number(revision) || 0);
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  void refreshPlayerView();
}

// applyAuthoredSession installs a server result that changed the authored
// configuration outside the save pipeline (groups, player configuration).
function applyAuthoredSession(session, authoringRevision, revision = 0) {
  if (!session) return;
  if (Number(authoringRevision) > 0) authoring.revision = Number(authoringRevision);
  authoring.lastSaved = structuredClone(session);
  if (authoringBusy()) mergeLiveValues(state.session, session);
  else state.session = structuredClone(session);
  newestDurableRevision = Math.max(newestDurableRevision, Number(revision) || 0);
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  void refreshPlayerView();
}

// commitAuthoringEdit applies one edit to the working copy and queues its
// save. mutate(session) must find what it changes by ID so it can run again
// on a newer session; reopen(reason) brings the editor back after a failure.
function commitAuthoringEdit({ label, scope = 'terminal', mutate, reopen = null, kind = 'edit', entry = null }) {
  if (!state.session || !state.filePath) return false;
  const before = authoredKey(state.session);
  mutate(state.session);
  if (authoredKey(state.session) === before) {
    renderSaveStatus();
    return true;
  }
  if (kind === 'edit') authoring.redo = [];
  authoring.error = null;
  authoring.historyError = null;
  authoring.queue.push({
    id: ++authoring.nextID, label, scope, mutate, reopen, kind, entry,
    candidate: structuredClone(state.session),
  });
  renderSaveStatus();
  void pumpAuthoringQueue();
  return true;
}

async function pumpAuthoringQueue() {
  if (authoring.inFlight) return;
  if (!authoring.queue.length) {
    for (const resolve of authoring.idleWaiters.splice(0)) resolve();
    return;
  }
  const generation = authoring.generation;
  const edit = authoring.queue.shift();
  authoring.inFlight = edit;
  renderSaveStatus();
  const result = await desktopAPI.saveAuthoring({
    session: edit.candidate,
    expectedAuthoringRevision: authoring.revision,
    correlationId: `authoring-${generation}-${edit.id}`,
  });
  if (generation !== authoring.generation) return;
  authoring.inFlight = null;
  if (result.ok) acceptAuthoringSave(edit, result);
  else if (result.failure === 'stale-revision') holdStaleAuthoringEdits(edit, result);
  else rejectAuthoringEdit(edit, result);
  renderAll();
  void pumpAuthoringQueue();
}

function acceptAuthoringSave(edit, result) {
  const previous = authoring.lastSaved;
  authoring.revision = result.authoringRevision;
  authoring.lastSaved = structuredClone(result.session);
  authoring.lastSavedAt = new Date();
  newestDurableRevision = Math.max(newestDurableRevision, result.sessionRevision);
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  if (result.changed) {
    if (edit.kind === 'edit') {
      authoring.undo.push({ label: edit.label, before: previous, after: authoring.lastSaved });
      if (authoring.undo.length > AUTHORING_HISTORY_LIMIT) {
        authoring.undo.shift();
        authoring.historyTrimmed = true;
      }
    } else if (edit.kind === 'undo') {
      authoring.undo = authoring.undo.filter(entry => entry !== edit.entry);
      authoring.redo.push(edit.entry);
    } else if (edit.kind === 'redo') {
      authoring.redo = authoring.redo.filter(entry => entry !== edit.entry);
      authoring.undo.push(edit.entry);
    }
  }
  if (authoring.queue.length) mergeLiveValues(state.session, result.session);
  else state.session = structuredClone(result.session);
  if (result.changed) void refreshPlayerView();
}

// rebuildWorkingCopy replays the queued edits on the last saved session.
function rebuildWorkingCopy(liveSource) {
  state.session = structuredClone(authoring.lastSaved);
  if (liveSource) mergeLiveValues(state.session, liveSource);
  authoring.queue = authoring.queue.filter(edit => {
    try {
      edit.mutate(state.session);
      edit.candidate = structuredClone(state.session);
      return true;
    } catch {
      return false;
    }
  });
}

function rejectAuthoringEdit(edit, result) {
  const message = facilityIssueMessage(result);
  authoring.error = { message, label: edit.label };
  if (edit.kind !== 'edit') authoring.historyError = { kind: edit.kind, message };
  rebuildWorkingCopy(result.session);
  edit.reopen?.(message);
}

function holdStaleAuthoringEdits(edit, result) {
  if (Number(result.authoringRevision) > 0) authoring.revision = result.authoringRevision;
  if (result.session) authoring.lastSaved = structuredClone(result.session);
  authoring.held = [edit, ...authoring.queue].filter(held => held.kind === 'edit');
  authoring.queue = [];
  state.session = structuredClone(authoring.lastSaved);
  authoring.error = { message: 'СЕССИЯ ИЗМЕНИЛАСЬ ДРУГИМ ДЕЙСТВИЕМ — ПРАВКА НЕ ПРИМЕНЕНА', label: edit.label, stale: true };
}

// reapplyHeldEdits runs the edits rejected as stale again, in order, on the
// newer session («ПРИМЕНИТЬ СНОВА»).
function reapplyHeldEdits() {
  const held = authoring.held;
  authoring.held = [];
  authoring.error = null;
  for (const edit of held) {
    commitAuthoringEdit({ label: edit.label, scope: edit.scope, mutate: edit.mutate, reopen: edit.reopen });
  }
  renderAll();
}

function renderSaveStatus() {
  let saveState = 'idle';
  let text = '';
  if (authoring.error) {
    saveState = 'error';
    text = `ОШИБКА СОХРАНЕНИЯ: ${authoring.error.message}`;
  } else if (authoringBusy()) {
    saveState = 'saving';
    text = 'СОХРАНЕНИЕ…';
  } else if (authoring.lastSavedAt) {
    saveState = 'saved';
    text = `СОХРАНЕНО · ${authoring.lastSavedAt.toLocaleTimeString('ru-RU')}`;
  }
  const unapplied = unappliedInputLabels();
  if (unapplied.length) text = [text, `НЕ ПРИМЕНЕНО: ${unapplied.join(', ')}`].filter(Boolean).join(' · ');
  saveStatus.textContent = text;
  saveStatus.dataset.state = saveState;
  saveStatus.dataset.unappliedCount = String(unapplied.length);
  saveStatus.classList.toggle('err', saveState === 'error');
  renderAuthoringControls();
}

// ── Unapplied input (R7) ────────────────────────────────────
// unappliedInputs keeps form input that is not applied yet, keyed by surface
// and entity, so a tab switch or a live re-render shows it again.
const unappliedInputs = new Map();
// unappliedSurfaces maps each tracked form to the entity it shows now.
const unappliedSurfaces = new Map();
let unappliedRestoring = false;

function fieldValue(field) {
  return field.type === 'checkbox' ? field.checked : field.value;
}

function fieldValues(container) {
  const values = {};
  for (const field of container.querySelectorAll('input[id], select[id], textarea[id]')) {
    values[field.id] = fieldValue(field);
  }
  return values;
}

function surfaceFields(container, surface) {
  return [...container.querySelectorAll('input[id], select[id], textarea[id]')]
    .filter(field => Object.hasOwn(surface.applied, field.id));
}

// showUnappliedInput fills container's fields for surface: staged input where
// there is some, the applied values elsewhere. surface is { key, label,
// exists, applied, blocks? }, or null when the form shows no entity.
function showUnappliedInput(container, surface) {
  if (!surface) unappliedSurfaces.delete(container);
  else unappliedSurfaces.set(container, surface);
  const staged = surface && unappliedInputs.get(surface.key);
  const restored = [];
  for (const field of surface ? surfaceFields(container, surface) : []) {
    const value = staged && Object.hasOwn(staged.values, field.id) ? staged.values[field.id] : surface.applied[field.id];
    if (fieldValue(field) === value) continue;
    if (field.type === 'checkbox') field.checked = value;
    else field.value = value;
    if (staged) restored.push(field);
  }
  // Restored fields announce their value so dependent controls follow it.
  unappliedRestoring = true;
  try {
    restored.forEach(field => field.dispatchEvent(new Event('change', { bubbles: true })));
  } finally {
    unappliedRestoring = false;
  }
  markUnappliedFields(container);
  return staged || null;
}

function stageUnappliedInput(container) {
  const surface = unappliedSurfaces.get(container);
  if (!surface || unappliedRestoring) return;
  const values = {};
  let changed = false;
  for (const field of surfaceFields(container, surface)) {
    values[field.id] = fieldValue(field);
    if (values[field.id] !== surface.applied[field.id]) changed = true;
  }
  const blocks = surface.blocks?.() || null;
  if (changed || blocks) unappliedInputs.set(surface.key, { label: surface.label, exists: surface.exists, values, blocks });
  else unappliedInputs.delete(surface.key);
  markUnappliedFields(container);
  renderSaveStatus();
}

function markUnappliedFields(container) {
  const surface = unappliedSurfaces.get(container);
  const staged = Boolean(surface && unappliedInputs.has(surface.key));
  for (const field of container.querySelectorAll('input[id], select[id], textarea[id]')) {
    if (staged && Object.hasOwn(surface.applied, field.id) && fieldValue(field) !== surface.applied[field.id]) {
      field.dataset.unapplied = 'true';
    } else {
      delete field.dataset.unapplied;
    }
  }
  container.querySelectorAll('[data-unapplied-cancel]').forEach(button => { button.hidden = !staged; });
}

// takeUnappliedInput removes and returns the staged input of container, so
// an apply can hand it back if the server rejects the edit.
function takeUnappliedInput(container) {
  const surface = unappliedSurfaces.get(container);
  if (!surface) return null;
  const staged = unappliedInputs.get(surface.key) || null;
  unappliedInputs.delete(surface.key);
  markUnappliedFields(container);
  const key = surface.key;
  return staged && (() => { unappliedInputs.set(key, staged); renderAll(); });
}

function unappliedInputLabels() {
  return [...unappliedInputs.values()].map(entry => (entry.exists()
    ? entry.label : `${entry.label} — ОБЪЕКТ ПРАВКИ БОЛЬШЕ НЕ СУЩЕСТВУЕТ`));
}

// explainControl blocks control with reason, or leaves it runnable with an
// optional description. A blocked control stays focusable; its
// aria-describedby element and title carry the text (FR-005).
function explainControl(control, reason, description = '') {
  if (reason) control.setAttribute('aria-disabled', 'true');
  else control.removeAttribute('aria-disabled');
  const text = reason || description;
  const reasonElement = document.getElementById(control.getAttribute('aria-describedby') || '');
  if (reasonElement) reasonElement.textContent = text;
  control.title = text;
}

function controlBlocked(control) {
  return control.getAttribute('aria-disabled') === 'true';
}

// historyBlockReason says why the newest entry of stack cannot be undone
// (kind 'undo') or redone (kind 'redo'), or returns ''.
function historyBlockReason(kind) {
  if (!state.session || !state.filePath) return 'НЕТ ОТКРЫТОЙ СЕССИИ';
  if (authoringBusy()) return 'ДОЖДИТЕСЬ СОХРАНЕНИЯ';
  const stack = kind === 'undo' ? authoring.undo : authoring.redo;
  const entry = stack.at(-1);
  if (!entry) {
    if (kind === 'undo' && authoring.historyTrimmed) return 'ИСТОРИЯ ХРАНИТ 20 ПОСЛЕДНИХ ПРАВОК';
    return kind === 'undo' ? 'НЕЧЕГО ОТМЕНЯТЬ' : 'НЕЧЕГО ПОВТОРЯТЬ';
  }
  if (authoring.historyError?.kind === kind) return authoring.historyError.message;
  const expected = kind === 'undo' ? entry.after : entry.before;
  if (authoredKey(state.session) !== authoredKey(expected)) return 'СЕССИЯ ИЗМЕНИЛАСЬ ПОСЛЕ ЭТОЙ ПРАВКИ';
  return '';
}

// runHistoryEntry saves the snapshot on the other side of the newest undo or
// redo entry through the save pipeline; the server keeps current live values.
function runHistoryEntry(kind) {
  const control = kind === 'undo' ? btnUndo : btnRedo;
  if (controlBlocked(control)) return;
  const entry = (kind === 'undo' ? authoring.undo : authoring.redo).at(-1);
  const target = kind === 'undo' ? entry.before : entry.after;
  commitAuthoringEdit({
    label: entry.label, kind, entry,
    mutate: session => replaceAuthoredContent(session, target),
  });
  renderAll();
}

function renderAuthoringControls() {
  explainControl(btnUndo, historyBlockReason('undo'),
    authoring.undo.length ? `ОТМЕНИТЬ: ${authoring.undo.at(-1).label}` : '');
  explainControl(btnRedo, historyBlockReason('redo'),
    authoring.redo.length ? `ПОВТОРИТЬ: ${authoring.redo.at(-1).label}` : '');
  btnReapplyEdit.hidden = !authoring.held.length;
  explainControl(btnResetScene, controlBlockReason(btnResetScene));
}

// ── Helpers ─────────────────────────────────────────────────
function escHtml(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
function escAttr(s) {
  return escHtml(s).replace(/"/g, '&quot;');
}

function getEditTerminal() {
  if (!state.session) return null;
  return state.session.terminals.find(t => t.id === state.editTerminalId) || null;
}

function locateNode(root, id) {
  function walk(node, parent) {
    if (node.id === id) return { node, parent };
    if (node.children) {
      for (const child of node.children) {
        const res = walk(child, node);
        if (res) return res;
      }
    }
    return null;
  }
  return walk(root, null);
}

function commandExecutionState(term, commandID) {
  const commandStates = term?.commandStates;
  if (!commandStates || typeof commandStates !== 'object') return null;
  const snapshot = commandStates[commandID];
  return snapshot && typeof snapshot === 'object' ? snapshot : null;
}

function nodeKindLabel(node) {
  return node.type === 'folder' ? 'Папка' : node.type === 'command' ? 'Команда' : 'Запись';
}

function effectiveNodeName(term, node) {
  if (node?.type !== 'command') return node?.name || '';
  const completedName = commandExecutionState(term, node.id)?.completedName;
  return typeof completedName === 'string' && completedName ? completedName : node.name;
}

function visitContentNodes(node, visit) {
  if (!node) return;
  visit(node);
  for (const child of node.children ?? []) visitContentNodes(child, visit);
}

function entryBlockCatalog(term) {
  const catalog = [];
  visitContentNodes(term?.root, (node) => {
    if (node.type !== 'entry') return;
    for (const [index, block] of (node.blocks ?? []).entries()) {
      catalog.push({ entry: node, block, index });
    }
  });
  return catalog;
}

function entryBlockOwners(term) {
  const owners = new Map();
  visitContentNodes(term?.root, (node) => {
    const blockID = node.type === 'command'
      ? node.stateChange?.entryContentChange?.blockId
      : '';
    if (blockID && !owners.has(blockID)) owners.set(blockID, node);
  });
  return owners;
}

function entryBlockLocation(term, blockID) {
  return entryBlockCatalog(term).find(({ block }) => block.id === blockID) || null;
}

function entryBlockCommandCandidates(term, blockID, currentOwnerID = '') {
  const candidates = [];
  visitContentNodes(term?.root, (node) => {
    if (node.type !== 'command' || !node.stateChange) return;
    const targetBlockID = node.stateChange.entryContentChange?.blockId || '';
    if (targetBlockID && targetBlockID !== blockID) return;
    if (commandExecutionState(term, node.id) && node.id !== currentOwnerID) return;
    candidates.push(node);
  });
  return candidates;
}

function entryBlockFolderCatalog(term) {
  const folders = [];
  function walk(node, path) {
    if (node.type !== 'folder') return;
    const name = node.id === 'root' ? 'ROOT' : node.name;
    const nextPath = [...path, name];
    folders.push({ folder: node, label: nextPath.join(' › ') });
    for (const child of node.children ?? []) {
      if (child.type === 'folder') walk(child, nextPath);
    }
  }
  if (term?.root) walk(term.root, []);
  return folders;
}

function setEntryBlockCommandError(message, field = null) {
  entryBlockCommandError.textContent = message;
  entryBlockCommandError.hidden = !message;
  field?.focus();
}

function selectedEntryBlockCommandMode() {
  return entryBlockCommandModeInputs.find(input => input.checked)?.value || 'existing';
}

function updateEntryBlockCommandMode() {
  const mode = selectedEntryBlockCommandMode();
  entryBlockCommandDialog.querySelectorAll('[data-entry-block-command-fields]').forEach((fields) => {
    fields.hidden = fields.dataset.entryBlockCommandFields !== mode;
  });
  setEntryBlockCommandError('');
}

function closeEntryBlockCommandDialog({ restoreFocus = true } = {}) {
  const draft = entryBlockCommandDraft;
  if (entryBlockCommandDialog.open && typeof entryBlockCommandDialog.close === 'function') {
    entryBlockCommandDialog.close();
  } else {
    entryBlockCommandDialog.removeAttribute('open');
  }
  entryBlockCommandDialog.hidden = true;
  entryBlockCommandDraft = null;
  if (!restoreFocus || !draft) return;
  const trigger = nodeForm.querySelector(
    `.entry-content-block[data-block-id="${CSS.escape(draft.blockID)}"] [data-entry-block-action="configure-command"]`,
  );
  trigger?.focus();
}

function openEntryBlockCommandDialog(term, blockID) {
  const location = entryBlockLocation(term, blockID);
  if (!location) return;
  const owner = entryBlockOwners(term).get(blockID) || null;
  const completedOwner = owner ? commandExecutionState(term, owner.id) : null;
  const locked = Boolean(completedOwner);
  const candidates = entryBlockCommandCandidates(term, blockID, owner?.id);

  entryBlockCommandDraft = { terminalID: term.id, blockID };
  entryBlockCommandForm.reset();
  entryBlockCommandDialogDescription.textContent = entryBlockTargetLabel(
    location.entry,
    location.block,
    location.index,
  );
  if (completedOwner) {
    entryBlockCommandDialogDescription.textContent += ` · ${effectiveNodeName(term, owner)} · СНАЧАЛА СБРОСЬТЕ СОСТОЯНИЕ КОМАНДЫ`;
  }

  entryBlockExistingCommand.innerHTML = '<option value="">ВЫБЕРИТЕ КОМАНДУ</option>'
    + candidates.map(command =>
      `<option value="${escAttr(command.id)}">${escHtml(effectiveNodeName(term, command))}</option>`
    ).join('');
  entryBlockExistingCommand.value = owner?.id || '';
  entryBlockCompletedText.value = completedOwner?.entryContentChange?.completedText
    ?? owner?.stateChange?.entryContentChange?.completedText
    ?? '';
  entryBlockNewDestinationFolder.innerHTML = '<option value="">ВЫБЕРИТЕ ПАПКУ</option>'
    + entryBlockFolderCatalog(term).map(({ folder, label }) =>
      `<option value="${escAttr(folder.id)}">${escHtml(label)}</option>`
    ).join('');

  entryBlockCommandModeInputs.forEach(input => { input.disabled = locked; });
  entryBlockExistingCommand.disabled = locked;
  entryBlockCompletedText.disabled = locked;
  entryBlockCommandDialog.querySelectorAll('[data-entry-block-command-fields="new"] input, [data-entry-block-command-fields="new"] textarea, [data-entry-block-command-fields="new"] select')
    .forEach(control => { control.disabled = locked; });
  btnApplyEntryBlockCommand.disabled = locked;
  btnRemoveEntryBlockCommand.disabled = locked || !owner;
  setEntryBlockCommandError('');
  updateEntryBlockCommandMode();

  showEntryBlockCommandDialog();
  (completedOwner ? btnCancelEntryBlockCommand : entryBlockExistingCommand).focus();
}

function showEntryBlockCommandDialog() {
  entryBlockCommandDialog.hidden = false;
  if (typeof entryBlockCommandDialog.showModal === 'function' && !entryBlockCommandDialog.open) {
    entryBlockCommandDialog.showModal();
  } else {
    entryBlockCommandDialog.setAttribute('open', '');
  }
}

// commitTerminalEdit queues an edit of one terminal, found again by ID
// whenever the edit is replayed on a newer session.
function commitTerminalEdit(label, terminalID, mutate, reopen = null) {
  return commitAuthoringEdit({
    label,
    reopen,
    mutate: session => {
      const term = session.terminals.find(terminal => terminal.id === terminalID);
      if (!term) throw new Error('terminal removed');
      mutate(term);
    },
  });
}

// commitEntryBlockCommandEdit saves the dialog's edit and closes it; a
// rejected save reopens it with the author's input and the reason.
function commitEntryBlockCommandEdit(label, draft, mutate) {
  commitTerminalEdit(label, draft.terminalID, mutate, reason => {
    entryBlockCommandDraft = draft;
    showEntryBlockCommandDialog();
    setEntryBlockCommandError(reason);
  });
  renderTree();
  renderNodeForm();
  renderToolbarHint();
  closeEntryBlockCommandDialog();
}

function applyEntryBlockCommandDialog() {
  const draft = entryBlockCommandDraft;
  if (!draft || !state.session) return;
  const term = state.session.terminals.find(candidate => candidate.id === draft.terminalID);
  const location = entryBlockLocation(term, draft.blockID);
  if (!location) {
    setEntryBlockCommandError('БЛОК БОЛЬШЕ НЕ СУЩЕСТВУЕТ');
    return;
  }
  const owner = entryBlockOwners(term).get(draft.blockID) || null;
  if (owner && commandExecutionState(term, owner.id)) {
    setEntryBlockCommandError('СНАЧАЛА СБРОСЬТЕ СОСТОЯНИЕ КОМАНДЫ');
    return;
  }

  const mode = selectedEntryBlockCommandMode();
  let selectedCommandID = '';
  let destinationFolderID = '';
  if (mode === 'existing') {
    selectedCommandID = entryBlockExistingCommand.value;
    const selected = selectedCommandID ? locateNode(term.root, selectedCommandID)?.node : null;
    const targetBlockID = selected?.stateChange?.entryContentChange?.blockId || '';
    if (!selected || selected.type !== 'command' || !selected.stateChange
      || commandExecutionState(term, selected.id)
      || (targetBlockID && targetBlockID !== draft.blockID)) {
      setEntryBlockCommandError('ВЫБЕРИТЕ ДОСТУПНУЮ КОМАНДУ', entryBlockExistingCommand);
      return;
    }
  } else {
    const required = [
      [entryBlockNewCommandName, 'УКАЖИТЕ НАЗВАНИЕ ДО ВЫПОЛНЕНИЯ'],
      [entryBlockNewCompletedName, 'УКАЖИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ'],
      [entryBlockNewConfirmationText, 'УКАЖИТЕ ТЕКСТ ПОДТВЕРЖДЕНИЯ'],
      [entryBlockNewResultText, 'УКАЖИТЕ РЕЗУЛЬТАТ КОМАНДЫ'],
    ];
    for (const [field, message] of required) {
      if (!field.value.trim()) {
        setEntryBlockCommandError(message, field);
        return;
      }
    }
    destinationFolderID = entryBlockNewDestinationFolder.value;
    const destination = destinationFolderID ? locateNode(term.root, destinationFolderID)?.node : null;
    if (!destination || destination.type !== 'folder') {
      setEntryBlockCommandError('ВЫБЕРИТЕ ПАПКУ ЭТОГО ТЕРМИНАЛА', entryBlockNewDestinationFolder);
      return;
    }
  }

  const entryContentChange = { blockId: draft.blockID, completedText: entryBlockCompletedText.value };
  let newCommand = null;
  if (mode !== 'existing') {
    let commandID;
    do commandID = uid('n'); while (locateNode(term.root, commandID));
    selectedCommandID = commandID;
    newCommand = {
      id: commandID,
      type: 'command',
      name: entryBlockNewCommandName.value.trim(),
      text: entryBlockNewResultText.value,
      stateChange: {
        completedName: entryBlockNewCompletedName.value,
        confirmationText: entryBlockNewConfirmationText.value,
        entryContentChange,
      },
    };
  }

  commitEntryBlockCommandEdit('СВЯЗЬ БЛОКА С КОМАНДОЙ', draft, candidateTerm => {
    const candidateOwner = entryBlockOwners(candidateTerm).get(draft.blockID) || null;
    if (candidateOwner && candidateOwner.id !== selectedCommandID) {
      delete candidateOwner.stateChange.entryContentChange;
    }
    if (newCommand) {
      const destination = locateNode(candidateTerm.root, destinationFolderID)?.node;
      if (!destination) throw new Error('folder removed');
      destination.children ??= [];
      if (!destination.children.some(child => child.id === newCommand.id)) {
        destination.children.push(structuredClone(newCommand));
      }
      return;
    }
    const selected = locateNode(candidateTerm.root, selectedCommandID)?.node;
    if (!selected?.stateChange) throw new Error('command removed');
    selected.stateChange.entryContentChange = { ...entryContentChange };
  });
}

function removeEntryBlockCommand() {
  const draft = entryBlockCommandDraft;
  if (!draft || !state.session) return;
  const term = state.session.terminals.find(candidate => candidate.id === draft.terminalID);
  if (!entryBlockLocation(term, draft.blockID)) {
    setEntryBlockCommandError('БЛОК БОЛЬШЕ НЕ СУЩЕСТВУЕТ');
    return;
  }
  const owner = entryBlockOwners(term).get(draft.blockID) || null;
  if (!owner) {
    setEntryBlockCommandError('СВЯЗЬ БОЛЬШЕ НЕ СУЩЕСТВУЕТ');
    return;
  }
  if (commandExecutionState(term, owner.id)) {
    setEntryBlockCommandError('СНАЧАЛА СБРОСЬТЕ СОСТОЯНИЕ КОМАНДЫ');
    return;
  }
  commitEntryBlockCommandEdit('СВЯЗЬ БЛОКА С КОМАНДОЙ', draft, candidateTerm => {
    const candidateOwner = entryBlockOwners(candidateTerm).get(draft.blockID);
    if (candidateOwner) delete candidateOwner.stateChange.entryContentChange;
  });
}

entryBlockCommandForm.addEventListener('submit', (event) => event.preventDefault());
entryBlockCommandModeInputs.forEach(input => input.addEventListener('change', updateEntryBlockCommandMode));
btnApplyEntryBlockCommand.addEventListener('click', applyEntryBlockCommandDialog);
btnRemoveEntryBlockCommand.addEventListener('click', removeEntryBlockCommand);
btnCancelEntryBlockCommand.addEventListener('click', closeEntryBlockCommandDialog);
entryBlockCommandDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  closeEntryBlockCommandDialog();
});

function normalizedEntryBlockPreview(value) {
  const normalized = String(value ?? '').trim().replace(/\s+/gu, ' ');
  if (!normalized) return 'ПУСТО';
  const codePoints = Array.from(normalized);
  return codePoints.length > 48 ? `${codePoints.slice(0, 48).join('')}…` : normalized;
}

function entryBlockTargetLabel(entry, block, index) {
  return `${entry.name} · БЛОК ${index + 1} · ${normalizedEntryBlockPreview(block.initialText)}`;
}

function entryBlockTargetOptions(term, selectedBlockID) {
  return entryBlockCatalog(term).map(({ entry, block, index }) =>
    `<option value="${escAttr(block.id)}"${block.id === selectedBlockID ? ' selected' : ''}>${escHtml(entryBlockTargetLabel(entry, block, index))}</option>`
  ).join('');
}

function commandOwningEntryBlock(term, blockID, excludedCommandID = '') {
  const owner = entryBlockOwners(term).get(blockID);
  return owner && owner.id !== excludedCommandID ? owner : null;
}

function targetingCommandsWithinNode(term, node) {
  const blockIDs = new Set();
  visitContentNodes(node, (candidate) => {
    if (candidate.type !== 'entry') return;
    for (const block of candidate.blocks ?? []) blockIDs.add(block.id);
  });
  const commands = [];
  const seen = new Set();
  const owners = entryBlockOwners(term);
  for (const blockID of blockIDs) {
    const command = owners.get(blockID);
    if (command && !seen.has(command.id)) {
      seen.add(command.id);
      commands.push(command);
    }
  }
  return commands;
}

function entryBlockDeletionError(term, commands) {
  const names = commands.map(command => effectiveNodeName(term, command)).join(', ');
  return `НЕЛЬЗЯ УДАЛИТЬ: БЛОК ИЗМЕНЯЕТСЯ КОМАНДОЙ: ${names}. СНАЧАЛА СБРОСЬТЕ, ПЕРЕНАЗНАЧЬТЕ ИЛИ УДАЛИТЕ КОМАНДУ.`;
}

function uniqueEntryBlockID(term) {
  const existing = new Set(entryBlockCatalog(term).map(({ block }) => block.id));
  let id;
  do id = uid('b'); while (existing.has(id));
  return id;
}

function entryContentBlockMarkup(term, block, index, { legacyDraft = false, owners = entryBlockOwners(term) } = {}) {
  const owner = block.id ? owners.get(block.id) : null;
  const ownerMarkup = owner
    ? `<div class="entry-content-block-owner" data-entry-block-owner data-command-id="${escAttr(owner.id)}">ИЗМЕНЯЕТСЯ КОМАНДОЙ: ${escHtml(effectiveNodeName(term, owner))}</div>`
    : '';
  return `<div class="entry-content-block" data-block-id="${legacyDraft ? '' : escAttr(block.id)}"${legacyDraft ? ' data-legacy-draft="true"' : ''}>
    <div class="entry-content-block-heading">
      <span class="entry-content-block-index">БЛОК ${index + 1}</span>
      <span class="entry-content-block-actions" role="group" aria-label="Действия блока ${index + 1}">
        <button class="btn btn-mini" type="button" data-entry-block-action="configure-command" aria-label="Настроить команду для блока ${index + 1}" aria-haspopup="dialog" aria-controls="entryBlockCommandDialog"${legacyDraft ? ' disabled title="Сначала примените блок"' : ''}>КОМАНДА…</button>
        <button class="btn btn-mini" type="button" data-entry-block-action="move-up" aria-label="Переместить блок ${index + 1} вверх">↑</button>
        <button class="btn btn-mini" type="button" data-entry-block-action="move-down" aria-label="Переместить блок ${index + 1} вниз">↓</button>
        <button class="btn btn-mini btn-danger" type="button" data-entry-block-action="delete" aria-label="Удалить блок ${index + 1}">×</button>
      </span>
    </div>
    <textarea class="field-textarea entry-content-block-text" data-entry-block-content aria-label="ИСХОДНЫЙ ТЕКСТ БЛОКА ${index + 1}">${escHtml(block.initialText)}</textarea>
    ${ownerMarkup}
  </div>`;
}

function renderSessionStateResult(result, successMessage, acceptsCanonicalResult = null) {
  if (!result?.ok || !result.session) {
    setCoordinationStatus('ОШИБКА ИЗМЕНЕНИЯ СОСТОЯНИЯ: ' + (result?.error || 'сессия не обновлена'), true);
    return false;
  }
  if (typeof acceptsCanonicalResult === 'function' && !acceptsCanonicalResult(result)) {
    setCoordinationStatus('ОШИБКА ИЗМЕНЕНИЯ СОСТОЯНИЯ: сервер не подтвердил сброс', true);
    return false;
  }

  applyLiveSession(result.session, Number(result.revision || result.savedRevision || 0));
  setCoordinationStatus(successMessage);
  renderAll();
  return true;
}

function updateSessionStateEvidenceDescription() {
  const evidence = [];
  if (saveStatus.dataset.wailsCommand) {
    evidence.push(`Wails command ${saveStatus.dataset.wailsCommand} ${saveStatus.dataset.wailsResult || 'unknown'}`);
  }
  if (saveStatus.dataset.wailsTerminalId) evidence.push(`terminal ${saveStatus.dataset.wailsTerminalId}`);
  if (saveStatus.dataset.wailsRevision) evidence.push(`document revision ${saveStatus.dataset.wailsRevision}`);
  if (saveStatus.dataset.sessionStateRevision) {
    evidence.push(`session-state revision ${saveStatus.dataset.sessionStateRevision}`);
  }
  if (evidence.length) {
    const accessibleEvidence = evidence.join('; ');
    saveStatus.setAttribute('aria-label', accessibleEvidence);
    saveStatus.setAttribute('aria-description', accessibleEvidence);
  } else {
    saveStatus.removeAttribute('aria-label');
    saveStatus.removeAttribute('aria-description');
  }
}

async function runSessionStateCommand(command, successMessage, acceptsCanonicalResult = null) {
  if (sessionStateCommandPending) return;
  sessionStateCommandPending = true;
  renderSettingsPanel();
  renderNodeForm();
  try {
    const result = await command();
    renderSessionStateResult(result, successMessage, acceptsCanonicalResult);
  } catch (error) {
    setCoordinationStatus('ОШИБКА ИЗМЕНЕНИЯ СОСТОЯНИЯ: '
      + (error instanceof Error ? error.message : String(error)), true);
  } finally {
    sessionStateCommandPending = false;
    renderSettingsPanel();
    renderNodeForm();
    renderSaveStatus();
  }
}

function currentAddTarget() {
  const term = getEditTerminal();
  if (!term) return null;
  if (!state.selectedNodeId) return term.root;
  const loc = locateNode(term.root, state.selectedNodeId);
  if (!loc) return term.root;
  return loc.node.type === 'folder' ? loc.node : (loc.parent || term.root);
}

// ── Facility authoring ──────────────────────────────────────
function emptyFacility() {
  return { revision: 0, devices: [], conditions: [], recoveryPrograms: [] };
}

function resetFacilitySelection() {
  selectedFacilityDeviceID = '';
  selectedFacilityConditionID = '';
  pendingFacilityRepair = null;
}

const EMPTY_FACILITY = Object.freeze({
  revision: 0, devices: Object.freeze([]), conditions: Object.freeze([]), recoveryPrograms: Object.freeze([]),
});

// authoredFacility is the working copy's facility. A session without one reads
// as empty; only an edit creates it (ensureFacility), so saving a legacy
// session never adds a facility to it.
function authoredFacility() {
  if (!state.session) return null;
  return state.session.facility || EMPTY_FACILITY;
}

function ensureFacility(session) {
  if (!session.facility || typeof session.facility !== 'object') session.facility = emptyFacility();
  for (const key of ['devices', 'conditions', 'recoveryPrograms']) session.facility[key] ||= [];
  return session.facility;
}

// commitFacilityEdit saves one facility edit and redraws the workspace.
function commitFacilityEdit(label, mutate, reopen = null) {
  commitAuthoringEdit({ label, scope: 'facility', mutate, reopen });
  renderFacilityWorkspace();
}

function setFacilityFeedback(element, message = '', field = null) {
  element.textContent = message;
  element.hidden = !message;
  const dialog = element.closest('dialog');
  dialog?.querySelectorAll('[aria-invalid="true"]').forEach(control => control.removeAttribute('aria-invalid'));
  if (!message || !field) return;
  for (let parent = field.parentElement; parent; parent = parent.parentElement) {
    if (parent instanceof HTMLDetailsElement) parent.open = true;
  }
  field.setAttribute('aria-invalid', 'true');
  field.focus();
}

function showFacilityDialog(dialog, focusTarget) {
  dialog.facilityReturnToStory = Boolean(document.activeElement?.closest('#facilityStoryActions'));
  dialog.hidden = false;
  if (typeof dialog.showModal === 'function' && !dialog.open) dialog.showModal();
  focusTarget?.focus();
}

function hideFacilityDialog(dialog, focusTarget = null) {
  if (dialog.open && typeof dialog.close === 'function') dialog.close();
  else dialog.removeAttribute('open');
  dialog.hidden = true;
  if (dialog.facilityReturnToStory && document.getElementById('facilityStoryGuide').open) {
    document.getElementById('facilityStoryTitle').focus();
  } else {
    focusTarget?.focus();
  }
  delete dialog.facilityReturnToStory;
}

function populateSelect(select, values, selected = '', placeholder = '') {
  select.replaceChildren();
  if (placeholder) {
    const option = document.createElement('option');
    option.value = '';
    option.textContent = placeholder;
    select.appendChild(option);
  }
  for (const value of values) {
    const option = document.createElement('option');
    option.value = value.id;
    option.textContent = value.name || value.id;
    option.title = value.id;
    select.appendChild(option);
  }
  if ([...select.options].some(option => option.value === selected)) select.value = selected;
}

const CYRILLIC_TO_LATIN = {
  а: 'a', б: 'b', в: 'v', г: 'g', д: 'd', е: 'e', ё: 'yo', ж: 'zh', з: 'z', и: 'i', й: 'y', к: 'k',
  л: 'l', м: 'm', н: 'n', о: 'o', п: 'p', р: 'r', с: 's', т: 't', у: 'u', ф: 'f', х: 'kh', ц: 'ts',
  ч: 'ch', ш: 'sh', щ: 'shch', ъ: '', ы: 'y', ь: '', э: 'e', ю: 'yu', я: 'ya',
  і: 'i', ї: 'yi', є: 'ye', ґ: 'g', ў: 'u',
};
const FACILITY_ID_LIMIT = 256;

// Suggests an identifier for a new facility entity. Transliteration runs before NFKD, which would
// otherwise split й and ё; uniqueness is checked against the final values in existingIDs.
function suggestFacilityID(prefix, name, existingIDs, keepPrefix = true) {
  const stem = [...String(name || '').toLowerCase()]
    .map(character => CYRILLIC_TO_LATIN[character] ?? character)
    .join('')
    .normalize('NFKD')
    .replace(/\p{M}/gu, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  const base = !stem ? prefix : keepPrefix && !stem.startsWith(`${prefix}-`) ? `${prefix}-${stem}` : stem;
  for (let attempt = 1; ; attempt++) {
    const suffix = stem && attempt === 1 ? '' : `-${attempt}`;
    const candidate = base.slice(0, FACILITY_ID_LIMIT - suffix.length).replace(/-+$/, '') + suffix;
    if (!existingIDs.has(candidate)) return candidate;
  }
}

function facilityDeviceByID(deviceID, session = state.session) {
  return session?.facility?.devices?.find(device => device.id === deviceID) || null;
}

function facilityNodeCatalog(session = state.session) {
  const values = [];
  for (const terminal of session?.terminals || []) {
    visitContentNodes(terminal.root, node => values.push({ terminal, node }));
  }
  return values;
}

function facilityBindingCatalog(session = state.session) {
  const bindings = [];
  for (const { terminal, node } of facilityNodeCatalog(session)) {
    for (const [index, variant] of (node.facilityNameVariants || []).entries()) {
      bindings.push({ terminal, node, variant, property: 'menu-name', index });
    }
    for (const block of node.blocks || []) {
      for (const [index, variant] of (block.facilityTextVariants || []).entries()) {
        bindings.push({ terminal, node, block, variant, property: 'entry-content', index });
      }
    }
    if (node.visibleWhen) {
      bindings.push({ terminal, node, variant: { when: node.visibleWhen, text: 'ВИДИМОСТЬ ЭЛЕМЕНТА' }, property: 'visibility', index: 0 });
    }
    if (node.availableWhen) {
      bindings.push({ terminal, node, variant: { when: node.availableWhen, text: 'ДОСТУПНОСТЬ КОМАНДЫ' }, property: 'availability', index: 0 });
    }
    const requests = node.stateChange?.facilityAction?.transitions?.transitions || [];
    if (requests.length) {
      bindings.push({
        terminal, node, requests,
        variant: { text: 'ДЕЙСТВИЕ КОМАНДЫ' },
        property: 'command-action', index: 0,
      });
    }
    const programID = node.stateChange?.facilityAction?.recoveryProgramId;
    if (programID) {
      const program = session.facility?.recoveryPrograms?.find(value => value.id === programID);
      bindings.push({ terminal, node, programID, requests: program?.transitions || [],
        variant: { text: program?.name || programID }, property: 'recovery-action', index: 0 });
    }
  }
  return bindings;
}

function facilityListRow(kind, id, heading, detail = '') {
  const row = document.createElement(kind === 'device' || kind === 'condition' ? 'button' : 'div');
  if (row instanceof HTMLButtonElement) row.type = 'button';
  row.dataset.facilityKind = kind;
  row.dataset.facilityId = id;
  if (!(row instanceof HTMLButtonElement)) row.setAttribute('role', 'listitem');
  row.title = id;
  const title = document.createElement('strong');
  title.textContent = heading || id;
  row.appendChild(title);
  if (detail) {
    const description = document.createElement('div');
    description.textContent = detail;
    row.appendChild(description);
  }
  return row;
}

function facilityConditionActivity(condition) {
  return `СЕЙЧАС: ${condition.currentActive ? 'АКТИВНА' : 'НЕАКТИВНА'} · В НАЧАЛЕ: ${condition.initialActive ? 'АКТИВНА' : 'ИСПРАВНО'}`;
}

function facilityStateName(device, stateID) {
  return device?.states?.find(value => value.id === stateID)?.name || stateID || 'Не задано';
}

function facilityEqualitySummary(equality) {
  const device = facilityDeviceByID(equality?.deviceId);
  return `${device?.name || equality?.deviceId || 'Устройство не найдено'}: ${facilityStateName(device, equality?.stateId)}`;
}

function facilityTransitionSummary(request) {
  const device = facilityDeviceByID(request.deviceId);
  const transition = device?.transitions?.find(value => value.id === request.transitionId);
  if (!transition) return `${device?.name || request.deviceId}: переход не найден (${request.transitionId})`;
  const requirement = (transition.preconditions || []).map(facilityEqualitySummary).join('; ');
  return `${device.name} — ${transition.name}: ${facilityStateName(device, transition.sourceStateId)} → ${facilityStateName(device, transition.destinationStateId)}${requirement ? ` · Требуется: ${requirement}` : ''}`;
}

function facilityCommandActionSummary(action) {
  const requests = action?.transitions?.transitions || [];
  if (requests.length) return requests.map(facilityTransitionSummary).join('\n');
  const programID = action?.recoveryProgramId;
  if (!programID) return 'Действие устройств не настроено.';
  const program = authoredFacility()?.recoveryPrograms?.find(value => value.id === programID);
  const heading = `Программа восстановления: ${program?.name || programID}`;
  const transitions = (program?.transitions || []).map(facilityTransitionSummary).join('\n');
  return transitions ? `${heading}\n${transitions}` : heading;
}

function facilityBindingSummary(binding) {
  if (Array.isArray(binding.requests)) {
    const heading = binding.property === 'recovery-action'
      ? `Программа: ${binding.variant.text}\n` : 'ДЕЙСТВИЕ КОМАНДЫ\n';
    return `${heading}${binding.requests.map(facilityTransitionSummary).join('\n')}`;
  }
  const when = facilityEqualitySummary(binding.variant.when);
  const purposes = {
    'menu-name': 'Название в меню', 'entry-content': 'Текст блока записи',
    visibility: 'Показывать элемент', availability: 'Разрешить команду',
  };
  return `${purposes[binding.property]} · Когда ${when}${['menu-name', 'entry-content'].includes(binding.property) ? `\n${binding.variant.text}` : ''}`;
}

function facilityBindingUsesDevice(binding, deviceID) {
  return Array.isArray(binding.requests)
    ? binding.requests.some(value => value.deviceId === deviceID)
    : binding.variant.when.deviceId === deviceID;
}

function facilityTechnicalDetails(parent, text) {
  const details = document.createElement('details');
  details.className = 'facility-technical-details';
  const summary = document.createElement('summary');
  summary.textContent = 'Технические идентификаторы';
  const content = document.createElement('p');
  content.textContent = text;
  details.append(summary, content);
  parent.appendChild(details);
}

function facilityEmptyList(list, message) {
  if (list.children.length) return;
  const empty = document.createElement('p');
  empty.className = 'facility-empty';
  empty.textContent = message;
  list.appendChild(empty);
}

function facilityRowActions(row, name, edit, remove) {
  const actions = document.createElement('div');
  actions.className = 'facility-related-actions';
  for (const [label, callback] of [['Изменить', edit], ['Удалить', remove]]) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `btn btn-mini ${label === 'Удалить' ? 'btn-danger' : 'btn-secondary'}`;
    button.textContent = label;
    button.setAttribute('aria-label', `${label}: ${name}`);
    button.addEventListener('click', callback);
    actions.appendChild(button);
  }
  row.appendChild(actions);
}

function facilityBindingKey(binding) {
  return {
    terminalID: binding.terminal.id, nodeID: binding.node.id, blockID: binding.block?.id || '',
    property: binding.property, index: binding.index,
  };
}

// removeFacilityBinding removes the binding named by key from session.
function removeFacilityBinding(session, key) {
  const terminal = session.terminals.find(candidate => candidate.id === key.terminalID);
  const node = terminal ? locateNode(terminal.root, key.nodeID)?.node : null;
  if (!node) return;
  const block = node.blocks?.find(candidate => candidate.id === key.blockID);
  if (key.property === 'menu-name') node.facilityNameVariants?.splice(key.index, 1);
  else if (key.property === 'entry-content') block?.facilityTextVariants?.splice(key.index, 1);
  else if (key.property === 'visibility') delete node.visibleWhen;
  else if (key.property === 'availability') delete node.availableWhen;
  else if (node.stateChange) delete node.stateChange.facilityAction;
}

function removeFacilityProgram(program) {
  const references = [
    ...authoredFacility().conditions.filter(value => value.recovery?.some(ref => ref.recoveryProgramId === program.id)).map(value => value.name),
    ...facilityNodeCatalog().filter(({ node }) => node.stateChange?.facilityAction?.recoveryProgramId === program.id).map(({ node }) => node.name),
  ];
  if (references.length) {
    setFacilityFeedback(facilityValidation, `Программа «${program.name}» используется: ${references.join('; ')}. Сначала измените связанные команды и пути восстановления.`);
    return;
  }
  commitFacilityEdit(`Программа «${program.name}»`, session => {
    const facility = ensureFacility(session);
    facility.recoveryPrograms = facility.recoveryPrograms.filter(value => value.id !== program.id);
  });
}

const facilityStorySteps = [
  {
    title: '1. Что может измениться в сцене?',
    explanation: 'Создайте устройство — предмет или систему, важную для истории. Назовите его состояния и выберите начальное. Если устройство уже есть, выберите его слева и продолжайте настройку.',
    example: 'Пример: «Дверь в убежище». Состояния: «Заперта» и «Открыта». Начальное: «Заперта». В форме также добавьте переход «Открыть дверь»: «Заперта» → «Открыта».',
  },
  {
    title: '2. Как игроки изменят ситуацию?',
    explanation: 'Переход описывает изменение устройства. Чтобы игроки могли его запросить, свяжите переход с командой терминала. Если команды ещё нет, создайте её на вкладке «ТЕРМИНАЛЫ», затем вернитесь сюда.',
    example: 'Пример: команда «Открыть дверь» запускает переход двери «Заперта» → «Открыта». Игрок запрашивает действие, Смотритель подтверждает. Если нужно питание, добавьте предусловие «Электросеть: В работе» в переход.',
  },
  {
    title: '3. Как игроки узнают о результате?',
    explanation: 'Добавьте правило отображения: текст записи, название в меню, видимость элемента или доступность команды. Оно проверяет состояние устройства, но само его не меняет. Исходный текст остаётся для остальных состояний.',
    example: 'Пример: в записи «Состояние шлюза» исходный текст — «Дверь заперта». Для состояния «Открыта» задайте текст блока «Путь свободен». Отдельным правилом можно показать новый маршрут или разрешить команду.',
  },
  {
    title: '4. Что может пойти не так? · Необязательно',
    explanation: 'Простая сцена уже обходится без неисправностей и программ. Добавляйте их, если истории нужны препятствие и способ его устранить. Неисправность задаёт ограничение, эффект перехода включает или снимает её.',
    example: 'Пример: «Привод двери повреждён» мешает выполнить команду. Переход ремонта снимает эту неисправность. Восстановление оператором — ручной выход; программа объединяет несколько переходов. После создания программы вернитесь к шагу «Действие» и выберите её в поле «Что запустить» у команды.',
  },
  {
    title: '5. Проверьте сцену',
    explanation: 'Каждая правка сохраняется сразу и сразу доходит до игроков. «ОТМЕНИТЬ» на общей панели возвращает последнюю правку. Сравните состояния в предпросмотре — он не меняет мир для игроков.',
    example: 'Проверка двери: выберите в предпросмотре «Открыта» и нужный терминал. Видно ли «Путь свободен»? После этого проверьте реальный запрос команды игроком и подтверждение Смотрителем. Предпросмотр проверяет отображение, а не выполнение команды.',
  },
];

function openFacilityStoryTerminals() {
  selectWorkspace('terminal');
  terminalWorkspaceTab.focus();
}

function setFacilityStoryStep(step, focusHeading = false) {
  facilityStoryStep = Math.max(0, Math.min(step, facilityStorySteps.length - 1));
  renderFacilityStoryGuide();
  if (focusHeading) document.getElementById('facilityStoryTitle').focus();
}

function openFacilityStoryBinding(kind, target, binding = null) {
  if (kind === 'command-action') {
    if (binding?.property === 'recovery-action') {
      openFacilityBindingDialog(binding);
      return;
    }
    openFacilityCommandActionEditor(target.terminal.id, target.node.id);
    if (!binding) {
      facilityBindingActionRequests.replaceChildren();
      appendFacilityActionRequest({ deviceId: selectedFacilityDeviceID });
    }
    return;
  }
  openFacilityBindingDialog(binding);
  if (binding) return;
  facilityBindingKind.value = target.node.blocks?.length ? 'entry-content' : 'menu-name';
  facilityBindingTerminal.value = target.terminal.id;
  updateFacilityBindingNodes();
  facilityBindingNode.value = target.node.id;
  updateFacilityBindingMode();
}

function renderFacilityStoryGuide() {
  const facility = authoredFacility();
  if (!facility) return;
  const device = facilityDeviceByID(selectedFacilityDeviceID);
  const bindings = facilityBindingCatalog().filter(binding => device && facilityBindingUsesDevice(binding, device.id));
  const actions = bindings.filter(binding => Array.isArray(binding.requests));
  const information = bindings.filter(binding => !Array.isArray(binding.requests));
  const step = facilityStorySteps[facilityStoryStep];
  document.getElementById('facilityStoryTitle').textContent = step.title;
  document.getElementById('facilityStoryExplanation').textContent = step.explanation;
  document.getElementById('facilityStoryExample').textContent = step.example;
  for (const button of document.querySelectorAll('[data-story-step]')) {
    if (Number(button.dataset.storyStep) === facilityStoryStep) button.setAttribute('aria-current', 'step');
    else button.removeAttribute('aria-current');
  }
  document.getElementById('btnFacilityStoryBack').disabled = facilityStoryStep === 0;
  document.getElementById('btnFacilityStoryNext').disabled = facilityStoryStep === facilityStorySteps.length - 1;
  const context = document.getElementById('facilityStoryContext');
  context.textContent = device
    ? `Настраиваем «${device.name}». Состояний: ${device.states.length}; переходов: ${device.transitions?.length || 0}; команд: ${actions.length}; правил отображения: ${information.length}.`
    : `Устройств в сессии: ${facility.devices.length}. Выберите устройство слева или создайте первое для своей сцены.`;
  const controls = document.getElementById('facilityStoryActions');
  controls.replaceChildren();
  const addAction = (label, action, primary = false) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `btn ${primary ? 'btn-primary' : 'btn-secondary'}`;
    button.textContent = label;
    button.addEventListener('click', action);
    controls.appendChild(button);
  };
  const addTerminalsAction = (primary = false) => {
    addAction('ОТКРЫТЬ ТЕРМИНАЛЫ', openFacilityStoryTerminals, primary);
  };
  const editDevice = () => openFacilityDeviceDialog(device?.id || '');
  if (!device) {
    addAction('СОЗДАТЬ УСТРОЙСТВО', () => openFacilityDeviceDialog(), true);
    if (facility.devices.length) addAction('ВЫБРАТЬ УСТРОЙСТВО', () => {
      document.getElementById('facilityDeviceSearch').focus();
    });
    if (facilityStoryStep !== 4) return;
  } else if (facilityStoryStep === 0) {
    addAction('НАСТРОИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', editDevice, true);
    addAction('СОЗДАТЬ ЕЩЁ УСТРОЙСТВО', () => openFacilityDeviceDialog());
  } else if (facilityStoryStep === 1) {
    if (!device.transitions?.length) {
      context.textContent += ' Сначала добавьте переход: из какого состояния и в какое ведёт действие.';
      addAction('ДОБАВИТЬ ПЕРЕХОД В УСТРОЙСТВО', editDevice, true);
      return;
    }
    const existing = actions[0];
    const freeCommand = facilityNodeCatalog().find(({ node }) => node.type === 'command'
      && !node.terminalTransition && !node.stateChange?.facilityAction);
    const target = existing || freeCommand;
    if (target) {
      addAction(existing ? 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ' : 'СВЯЗАТЬ С КОМАНДОЙ',
        () => openFacilityStoryBinding('command-action', target, existing), true);
      if (existing && freeCommand) addAction('СВЯЗАТЬ С ДРУГОЙ КОМАНДОЙ',
        () => openFacilityStoryBinding('command-action', freeCommand));
    } else {
      context.textContent += ' Нет свободной команды. Создайте команду в нужном терминале и вернитесь на этот шаг.';
      addTerminalsAction(true);
    }
    addAction('ИЗМЕНИТЬ ПЕРЕХОДЫ И ПРЕДУСЛОВИЯ', editDevice);
  } else if (facilityStoryStep === 2) {
    const existing = information[0];
    const nodes = facilityNodeCatalog().filter(({ node }) => node.id !== 'root');
    const target = existing || nodes.find(({ node }) => node.type === 'entry' && node.blocks?.length) || nodes[0];
    if (target) {
      addAction(existing ? 'ИЗМЕНИТЬ ПРАВИЛО ОТОБРАЖЕНИЯ' : 'НАСТРОИТЬ РЕЗУЛЬТАТ НА ТЕРМИНАЛЕ',
        () => openFacilityStoryBinding('information', target, existing), true);
      addAction('ДОБАВИТЬ ДРУГОЕ ПРАВИЛО', () => openFacilityStoryBinding('information', target));
    } else {
      context.textContent += ' Сначала создайте запись или другой элемент в терминале: здесь будет показан результат.';
      addTerminalsAction(true);
    }
  } else if (facilityStoryStep === 3) {
    addAction('ДОБАВИТЬ НЕИСПРАВНОСТЬ', () => openFacilityConditionDialog(), true);
    if (device.transitions?.length) {
      addAction('СОБРАТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', () => openFacilityRecoveryProgramDialog());
    } else {
      context.textContent += ' Если нужна программа восстановления, сначала добавьте переход устройства: программа запускает готовые переходы. Для простой сцены этот шаг можно пропустить.';
      addAction('ДОБАВИТЬ ПЕРЕХОД ДЛЯ ПРОГРАММЫ', editDevice);
    }
    addAction('НАСТРОИТЬ АКТИВАЦИЮ И СНЯТИЕ', editDevice);
  }
  if (facilityStoryStep === 4) {
    if (device) {
      context.textContent += ' Все правки уже сохранены. Сравните результат в нужном терминале.';
      addAction('ПРОВЕРИТЬ В ПРЕДПРОСМОТРЕ', () => openFacilityPreview('device', controls.firstElementChild), true);
    }
    addTerminalsAction();
  }
}

function renderFacilityWorkspace() {
  const facility = authoredFacility();
  if (!facility) return;
  const search = document.getElementById('facilityDeviceSearch')?.value.trim().toLocaleLowerCase() || '';
  const bindings = facilityBindingCatalog();
  renderFacilityStoryGuide();
  facilityDeviceList.replaceChildren();
  for (const device of facility.devices || []) {
    if (search && !`${device.name} ${device.id} ${facilityStateName(device, device.currentStateId)}`.toLocaleLowerCase().includes(search)) continue;
    const kind = [...facilityDeviceKind.options].find(option => option.value === device.kind)?.textContent || device.kind;
    const activeFaults = (facility.conditions || []).filter(value => value.device?.deviceId === device.id && value.currentActive).length;
    const row = facilityListRow('device', device.id, device.name,
      `${kind} · ${facilityStateName(device, device.currentStateId)}${activeFaults ? ` · Активных неисправностей: ${activeFaults}` : ''}`);
    row.dataset.facilitySelected = String(device.id === selectedFacilityDeviceID);
    row.setAttribute('aria-pressed', String(device.id === selectedFacilityDeviceID));
    row.addEventListener('click', () => {
      selectedFacilityDeviceID = device.id;
      selectedFacilityConditionID = '';
      facilitySelectionStatus.textContent = '';
      setFacilityFeedback(facilitySelectionError);
      renderFacilityWorkspace();
      rowByFacilityIdentity('device', device.id)?.focus({ preventScroll: true });
    });
    facilityDeviceList.appendChild(row);
  }
  facilityEmptyList(facilityDeviceList, search ? 'Устройства не найдены. Измените поиск.' : 'Добавьте первое устройство, затем задайте его состояния и переходы.');

  facilityConditionList.replaceChildren();
  for (const condition of facility.conditions || []) {
    if (selectedFacilityDeviceID && condition.device?.deviceId !== selectedFacilityDeviceID) continue;
    const scope = facilityDeviceByID(condition.device?.deviceId)?.name
      || state.session.terminals.find(value => value.id === condition.terminal?.terminalId)?.name || 'Вся сессия';
    const row = facilityListRow(
      'condition', condition.id, condition.name, `${facilityConditionActivity(condition)} · ${scope}`,
    );
    row.dataset.facilitySelected = String(condition.id === selectedFacilityConditionID);
    row.addEventListener('click', () => {
      selectedFacilityConditionID = condition.id;
      renderFacilityWorkspace();
      rowByFacilityIdentity('condition', condition.id)?.focus({ preventScroll: true });
    });
    facilityConditionList.appendChild(row);
  }
  facilityEmptyList(facilityConditionList, 'Неисправности не настроены для выбранного устройства.');

  facilityRecoveryProgramList.replaceChildren();
  for (const program of facility.recoveryPrograms || []) {
    if (selectedFacilityDeviceID && !program.transitions?.some(value => value.deviceId === selectedFacilityDeviceID)) continue;
    const row = facilityListRow(
      'recovery-program', program.id, program.name, (program.transitions || []).map(facilityTransitionSummary).join('\n'),
    );
    facilityRowActions(row, program.name, () => openFacilityRecoveryProgramDialog(program.id), () => removeFacilityProgram(program));
    facilityRecoveryProgramList.appendChild(row);
  }
  facilityEmptyList(facilityRecoveryProgramList, 'Программы восстановления не настроены.');

  facilityBindingList.replaceChildren();
  const bindingGroups = new Map();
  for (const [kind, title, description] of [
    ['action', 'Что делают игроки', 'Команды запускают переходы устройств или программы восстановления после подтверждения Смотрителем.'],
    ['information', 'Что видят игроки', 'Текст, меню и доступность зависят от состояния устройства. Эти правила ничего не переключают.'],
  ]) {
    const group = document.createElement('section');
    group.className = 'facility-binding-group';
    const heading = document.createElement('h3');
    heading.textContent = title;
    const help = document.createElement('p');
    help.className = 'facility-section-help';
    help.textContent = description;
    const list = document.createElement('div');
    list.setAttribute('role', 'list');
    list.setAttribute('aria-label', title);
    group.append(heading, help, list);
    facilityBindingList.appendChild(group);
    bindingGroups.set(kind, list);
  }
  for (const binding of bindings) {
    if (selectedFacilityDeviceID && !facilityBindingUsesDevice(binding, selectedFacilityDeviceID)) continue;
    const id = `${binding.terminal.id}/${binding.block?.id || binding.node.id}/${binding.property}/${binding.index}`;
    const row = facilityListRow(
      'binding', id, `${binding.terminal.name} › ${binding.node.name}${binding.block ? ` › Блок ${binding.node.blocks.indexOf(binding.block) + 1}` : ''}`,
      facilityBindingSummary(binding),
    );
    facilityRowActions(row, `${binding.node.name} · ${binding.property}`,
      () => binding.property === 'recovery-action' ? openFacilityRecoveryProgramDialog(binding.programID) : openFacilityBindingDialog(binding),
      () => {
        const key = facilityBindingKey(binding);
        commitFacilityEdit(`Правило «${binding.node.name}»`, session => removeFacilityBinding(session, key));
      });
    bindingGroups.get(Array.isArray(binding.requests) ? 'action' : 'information').appendChild(row);
  }
  facilityEmptyList(bindingGroups.get('action'), 'Команды ещё не связаны. Откройте шаг 2 «Действие» в инструкции.');
  facilityEmptyList(bindingGroups.get('information'), 'Правила отображения ещё не настроены. Откройте шаг 3 «Результат» в инструкции.');

  const selected = facilityDeviceByID(selectedFacilityDeviceID);
  facilitySelectionEditor.hidden = !selected;
  const empty = document.getElementById('facilitySelectionEmpty');
  if (empty) empty.hidden = Boolean(selected || selectedFacilityConditionID);
  const clear = document.getElementById('btnClearFacilitySelection');
  if (clear) clear.hidden = !selectedFacilityDeviceID && !selectedFacilityConditionID;
  const summary = document.getElementById('facilityDeviceSummary');
  if (summary) {
    summary.hidden = !selected;
    summary.replaceChildren();
    if (selected) {
      const status = document.createElement('p');
      status.className = 'facility-device-status';
      status.textContent = `Сейчас: ${facilityStateName(selected, selected.currentStateId)} · При сбросе: ${facilityStateName(selected, selected.initialStateId)}`;
      const affected = document.createElement('p');
      const terminals = [...new Set(bindings.filter(value => facilityBindingUsesDevice(value, selected.id)).map(value => value.terminal.name))];
      affected.textContent = terminals.length ? `Связанные терминалы: ${terminals.join('; ')}` : 'Терминалы ещё не связаны с этим устройством.';
      summary.append(status, affected);
      facilityTechnicalDetails(summary, `${selected.id} · ${selected.kind} · ${selected.currentStateId}`);
    }
  }
  const selectedDeviceID = selected?.id;
  showUnappliedInput(facilitySelectionEditor, selected && {
    key: `device:${selectedDeviceID}`,
    label: `Устройство «${selected.name}»`,
    exists: () => Boolean(facilityDeviceByID(selectedDeviceID, state.session)),
    applied: { facilitySelectedDeviceName: selected.name },
  });
  const selectedCondition = facility.conditions?.find(condition => condition.id === selectedFacilityConditionID) || null;
  facilityConditionSelectionEditor.hidden = !selectedCondition;
  facilitySelectedConditionSummary.textContent = selectedCondition
    ? `${selectedCondition.name} · ${facilityConditionActivity(selectedCondition)}` : '';
  for (const control of [btnResetFacilityDevice, btnPreviewFacilityDevice, btnPreviewFacilityCondition, btnRecoverFacilityCondition]) {
    explainControl(control, controlBlockReason(control));
  }
}

// controlBlockReason says why a reset, preview, or recovery control cannot
// run now, or returns '' (FR-005).
function controlBlockReason(control) {
  if (!state.session || !state.filePath) return 'НЕТ ОТКРЫТОЙ СЕССИИ';
  if (coordinationCommandPending || sessionStateCommandPending) return 'ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА';
  // A scene reset counts against the saved session; a preview waits instead.
  if (control === btnResetScene && authoringBusy()) return 'ДОЖДИТЕСЬ СОХРАНЕНИЯ';
  if (control === btnResetScene && sceneResetCounts(state.session).total === 0) return 'СЦЕНА УЖЕ В НАЧАЛЬНОМ СОСТОЯНИИ';
  if (control === btnRecoverFacilityCondition) {
    const condition = selectedFacilityCondition(state.session);
    if (!condition?.currentActive) return 'НЕИСПРАВНОСТЬ НЕ АКТИВНА';
    if (!(condition.recovery || []).some(reference => reference.privateOverseerAction === true)) {
      return 'ВОССТАНОВЛЕНИЕ ОПЕРАТОРОМ НЕ НАСТРОЕНО';
    }
  }
  return '';
}

function rowByFacilityIdentity(kind, id) {
  return document.querySelector(`[data-facility-kind="${CSS.escape(kind)}"][data-facility-id="${CSS.escape(id)}"]`);
}

function selectWorkspace(kind) {
  const facilitySelected = kind === 'facility';
  terminalWorkspace.hidden = facilitySelected;
  terminalWorkspace.style.display = facilitySelected ? 'none' : 'flex';
  facilityWorkspace.hidden = !facilitySelected;
  terminalWorkspaceTab.setAttribute('aria-selected', String(!facilitySelected));
  facilityWorkspaceTab.setAttribute('aria-selected', String(facilitySelected));
  terminalWorkspaceTab.tabIndex = facilitySelected ? -1 : 0;
  facilityWorkspaceTab.tabIndex = facilitySelected ? 0 : -1;
  if (facilitySelected) renderFacilityWorkspace();
}

function facilityIssueMessage(result) {
  const issue = result?.issues?.[0];
  const parts = [facilityFailureMessages[result?.failure] || 'КОНФИГУРАЦИЯ ОБЪЕКТОВ НЕ СОХРАНЕНА'];
  if (issue?.entityKind) parts.push(issue.entityKind);
  if (issue?.entityId) parts.push(issue.entityId);
  if (issue?.referenceKind || issue?.referenceId) {
    parts.push([issue.referenceKind, issue.referenceId].filter(Boolean).join(': '));
  }
  return parts.join(' · ');
}


function selectedFacilityCondition(session = state.session) {
  return session?.facility?.conditions?.find(condition => condition.id === selectedFacilityConditionID) || null;
}

function facilityRevision() {
  return Number(state.session?.facility?.revision || 0);
}

function facilityPreviewNodeMap(node, result = new Map()) {
  if (!node || typeof node !== 'object') return result;
  result.set(node.id, node);
  for (const child of node.children || []) facilityPreviewNodeMap(child, result);
  return result;
}

const PREVIEW_DIFFERENCE_LABELS = {
  'name-changed': 'ПЕРЕИМЕНУЕТСЯ',
  'text-changed': 'ТЕКСТ ИЗМЕНИТСЯ',
  appears: 'ПОЯВИТСЯ',
  disappears: 'ИСЧЕЗНЕТ',
  'becomes-available': 'СТАНЕТ ДОСТУПНО',
  'becomes-unavailable': 'СТАНЕТ НЕДОСТУПНО',
};

// renderFacilityPreviewNode draws one projected node; kinds maps node IDs to
// the difference kinds that mark them in both panes, in summary order.
function renderFacilityPreviewNode(node, parent, kinds) {
  if (!node || typeof node !== 'object') return;
  const item = document.createElement('section');
  item.className = 'facility-preview-node';
  item.dataset.previewNodeId = node.id || '';
  const nodeKinds = kinds.get(node.id) || [];
  const heading = document.createElement('strong');
  heading.textContent = node.name || node.id || '—';
  item.appendChild(heading);
  if (nodeKinds.length) item.dataset.kind = nodeKinds.join(' ');
  for (const kind of nodeKinds) {
    const status = document.createElement('span');
    status.className = 'facility-preview-node-status';
    status.textContent = PREVIEW_DIFFERENCE_LABELS[kind];
    item.appendChild(status);
  }
  const texts = node.description ? [node.description] : (node.blocks || []).map(block => block.initialText || '');
  for (const value of texts) {
    const text = document.createElement('p');
    text.textContent = value;
    item.appendChild(text);
  }
  parent.appendChild(item);
  for (const child of node.children || []) renderFacilityPreviewNode(child, item, kinds);
}

function clearFacilityPreview(summary = '') {
  facilityPreviewSummary.textContent = summary;
  facilityPreviewCurrent.replaceChildren();
  facilityPreviewHypothetical.replaceChildren();
}

function closeFacilityPreview() {
  const opener = facilityOperationOpener;
  facilityPreviewGeneration++;
  facilityPreviewSelection = null;
  facilityPreviewHypothetical.removeAttribute('aria-busy');
  facilityOperationOpener = null;
  hideFacilityDialog(facilityPreviewDialog, opener);
}

function openFacilityPreview(kind, opener) {
  const device = facilityDeviceByID(selectedFacilityDeviceID, state.session);
  const condition = selectedFacilityCondition(state.session);
  if ((kind === 'device' && !device) || (kind === 'condition' && !condition)) return;
  facilityPreviewSelection = { kind, id: kind === 'device' ? device.id : condition.id };
  facilityOperationOpener = opener;
  populateSelect(facilityPreviewTerminal,
    (state.session?.terminals || []).map(terminal => ({ id: terminal.id, name: terminal.name })),
    state.session?.terminals?.[0]?.id || '', 'ВЫБЕРИТЕ ТЕРМИНАЛ');
  const deviceMode = kind === 'device';
  facilityPreviewStateLabel.hidden = !deviceMode;
  facilityPreviewState.hidden = !deviceMode;
  facilityPreviewActiveLabel.hidden = deviceMode;
  if (deviceMode) {
    const different = (device.states || []).find(value => value.id !== device.currentStateId);
    populateSelect(facilityPreviewState, device.states || [], different?.id || device.currentStateId);
  } else {
    facilityPreviewActive.checked = !condition.currentActive;
  }
  clearFacilityPreview();
  facilityPreviewStatus.textContent = '';
  setFacilityFeedback(facilityPreviewError);
  showFacilityDialog(facilityPreviewDialog, deviceMode ? facilityPreviewState : facilityPreviewActive);
  void refreshFacilityPreview();
}

// refreshFacilityPreview asks, once the save queue is idle, for the current
// and the previewed projection of the applied configuration in one request;
// a stale authoring revision is retried once (FR-016–FR-020).
async function refreshFacilityPreview(retried = false) {
  if (!facilityPreviewSelection || !facilityPreviewTerminal.value) return;
  const generation = ++facilityPreviewGeneration;
  const selection = { ...facilityPreviewSelection };
  setFacilityFeedback(facilityPreviewError);
  const payload = { terminalId: facilityPreviewTerminal.value };
  let currentLabel;
  let previewLabel;
  if (selection.kind === 'device') {
    const device = facilityDeviceByID(selection.id, state.session);
    if (!device) return;
    if ((device.states || []).length < 2) {
      facilityPreviewStatus.textContent = '';
      clearFacilityPreview('НЕЧЕГО СРАВНИВАТЬ: У УСТРОЙСТВА ОДНО СОСТОЯНИЕ');
      return;
    }
    currentLabel = `${device.name}: ${facilityStateName(device, device.currentStateId)}`;
    previewLabel = `${device.name}: ${facilityStateName(device, facilityPreviewState.value)}`;
    payload.deviceState = { deviceId: selection.id, stateId: facilityPreviewState.value };
  } else {
    const condition = state.session?.facility?.conditions?.find(value => value.id === selection.id);
    if (!condition) return;
    const activity = active => (active ? 'Активно' : 'Неактивно');
    currentLabel = `${condition.name}: ${activity(condition.currentActive)}`;
    previewLabel = `${condition.name}: ${activity(facilityPreviewActive.checked)}`;
    payload.condition = { conditionId: selection.id, active: facilityPreviewActive.checked };
  }
  facilityPreviewStatus.textContent = 'ПОДГОТОВКА ПРЕДПРОСМОТРА…';
  facilityPreviewHypothetical.setAttribute('aria-busy', 'true');
  await authoringIdle();
  if (generation !== facilityPreviewGeneration) return;
  payload.expectedFacilityRevision = facilityRevision();
  payload.expectedAuthoringRevision = authoring.revision;
  const result = await desktopAPI.previewFacility(payload);
  if (generation !== facilityPreviewGeneration || facilityPreviewSelection?.kind !== selection.kind
      || facilityPreviewSelection.id !== selection.id) return;
  facilityPreviewHypothetical.removeAttribute('aria-busy');
  if (!result.ok && result.failure === 'stale-revision' && !retried) {
    void refreshFacilityPreview(true);
    return;
  }
  if (!result.ok || !result.terminal || !result.current) {
    facilityPreviewStatus.textContent = '';
    setFacilityFeedback(facilityPreviewError, facilityIssueMessage(result));
    return;
  }
  const kinds = new Map();
  for (const { nodeId, kind } of result.differences) {
    const nodeKinds = kinds.get(nodeId) || [];
    if (!nodeKinds.includes(kind)) kinds.set(nodeId, [...nodeKinds, kind]);
  }
  document.getElementById('facilityPreviewCurrentHeading').textContent = `СЕЙЧАС: ${currentLabel}`;
  document.getElementById('facilityPreviewHypotheticalHeading').textContent = `ЕСЛИ: ${previewLabel}`;
  clearFacilityPreview();
  renderFacilityPreviewNode(result.current.tree, facilityPreviewCurrent, kinds);
  renderFacilityPreviewNode(result.terminal.tree, facilityPreviewHypothetical, kinds);
  const nodes = new Map([...facilityPreviewNodeMap(result.current.tree), ...facilityPreviewNodeMap(result.terminal.tree)]);
  if (!result.differences.length) {
    facilityPreviewSummary.textContent = 'ИГРОКИ НЕ УВИДЯТ ИЗМЕНЕНИЙ';
  } else {
    const heading = document.createElement('div');
    heading.textContent = `ИЗМЕНЕНИЙ: ${result.differences.length}`;
    const list = document.createElement('ul');
    // The player projection joins an entry's blocks into one text, so block
    // numbers come from the authored terminal.
    const authored = state.session?.terminals?.find(terminal => terminal.id === payload.terminalId);
    for (const difference of result.differences) {
      const node = nodes.get(difference.nodeId);
      const block = difference.blockId ? entryBlockLocation(authored, difference.blockId) : null;
      const item = document.createElement('li');
      item.dataset.kind = difference.kind;
      item.textContent = `${PREVIEW_DIFFERENCE_LABELS[difference.kind] || difference.kind}: ${node?.name || difference.nodeId}`
        + (block ? ` · блок ${block.index + 1}` : '');
      list.appendChild(item);
    }
    facilityPreviewSummary.replaceChildren(heading, list);
  }
  facilityPreviewStatus.textContent = 'ПРЕДПРОСМОТР ГОТОВ · МИР ДЛЯ ИГРОКОВ НЕ ИЗМЕНЁН';
}

function openFacilityConfirmation(dialog, opener, focusTarget) {
  const description = dialog.querySelector('p');
  const device = facilityDeviceByID(selectedFacilityDeviceID, state.session);
  const condition = selectedFacilityCondition(state.session);
  if (dialog === facilityDeviceResetDialog && device) {
    description.textContent = `${device.name}: ${facilityStateName(device, device.currentStateId)} → ${facilityStateName(device, device.initialStateId)}. Неисправности этого устройства также вернутся к начальным значениям. Изменение сразу сохранится и станет видно игрокам.`;
  } else if (dialog === facilityRecoveryConfirmationDialog && condition) {
    description.textContent = `Снять неисправность «${condition.name}» через настроенное восстановление оператором? Изменение сразу сохранится и станет видно игрокам.`;
  }
  facilityOperationOpener = opener;
  showFacilityDialog(dialog, focusTarget);
}

function closeFacilityConfirmation(dialog) {
  const opener = facilityOperationOpener;
  facilityOperationOpener = null;
  hideFacilityDialog(dialog, opener);
}

function reconcileFacilityOperation(result, successMessage, focusKind = '', focusID = '') {
  if (!result?.ok || !result.session) {
    facilityStatus.dataset.error = 'true';
    const message = facilityIssueMessage(result);
    facilityStatus.textContent = message;
    setFacilityFeedback(facilityValidation, message);
    return false;
  }
  applyLiveSession(result.session, result.sessionRevision);
  if (focusKind === 'device') selectedFacilityDeviceID = focusID;
  if (focusKind === 'condition') selectedFacilityConditionID = focusID;
  delete facilityStatus.dataset.error;
  facilityStatus.textContent = successMessage;
  setFacilityFeedback(facilityValidation);
  renderAll();
  rowByFacilityIdentity(focusKind, focusID)?.focus();
  return true;
}

async function resetSelectedFacilityDevice() {
  const deviceID = selectedFacilityDeviceID;
  closeFacilityConfirmation(facilityDeviceResetDialog);
  const result = await desktopAPI.resetFacilityDevice({
    deviceId: deviceID,
    expectedFacilityRevision: facilityRevision(),
    correlationId: uid('facility-device-reset'),
  });
  reconcileFacilityOperation(result, 'УСТРОЙСТВО СБРОШЕНО', 'device', deviceID);
}

// ── Scene reset (FR-014) ────────────────────────────────────
let sceneResetCounted = null;
let sceneResetSubmitting = false;

// sceneResetCounts counts what a scene reset changes in session.
function sceneResetCounts(session) {
  const facility = session?.facility;
  const devices = (facility?.devices || []).filter(device => device.currentStateId !== device.initialStateId).length;
  const faults = (facility?.conditions || [])
    .filter(condition => Boolean(condition.currentActive) !== Boolean(condition.initialActive)).length;
  const terminals = (session?.terminals || [])
    .map(terminal => ({ name: terminal.name, count: Object.keys(terminal.commandStates || {}).length }))
    .filter(terminal => terminal.count > 0);
  const commands = terminals.reduce((sum, terminal) => sum + terminal.count, 0);
  return { devices, faults, commands, terminals, total: devices + faults + commands };
}

// renderSceneResetDialog lists the counts and records the revisions they
// were counted against.
function renderSceneResetDialog(sessionRevision = currentSessionRevision(), facilityRevisionValue = facilityRevision()) {
  const counts = sceneResetCounts(state.session);
  const perTerminal = counts.terminals.map(terminal => `${terminal.name}: ${terminal.count}`).join(', ');
  sceneResetCountsList.replaceChildren(...[
    `УСТРОЙСТВА: ${counts.devices}`,
    `НЕИСПРАВНОСТИ: ${counts.faults}`,
    `ВЫПОЛНЕННЫЕ КОМАНДЫ: ${counts.commands}${perTerminal ? ` (${perTerminal})` : ''}`,
  ].map(text => {
    const item = document.createElement('li');
    item.textContent = text;
    return item;
  }));
  renderSceneResetPending();
  sceneResetCounted = { sessionRevision, facilityRevision: facilityRevisionValue };
}

// renderSceneResetPending says what the reset does to a pending player
// request; it follows coordination changes while the dialog is open.
function renderSceneResetPending() {
  const pending = state.coordination?.pendingCommandExecution;
  if (pending?.requestId) {
    const terminal = state.session?.terminals.find(candidate => candidate.id === pending.terminalId);
    const rejected = Boolean(pending.facilityAction) || Object.hasOwn(terminal?.commandStates || {}, pending.commandId);
    sceneResetPending.textContent = `ОЖИДАЮЩИЙ ЗАПРОС ИГРОКА: ${pending.commandName || pending.commandId} — ${rejected ? 'БУДЕТ ОТКЛОНЁН' : 'ОСТАНЕТСЯ В ОЖИДАНИИ'}`;
  }
  sceneResetPending.hidden = !pending?.requestId;
}

function setSceneResetStatus(message = '') {
  sceneResetStatus.textContent = message;
  sceneResetStatus.hidden = !message;
}

function openSceneResetDialog() {
  if (controlBlocked(btnResetScene)) return;
  setSceneResetStatus();
  renderSceneResetDialog();
  showFacilityDialog(sceneResetDialog, btnCancelSceneReset);
}

function closeSceneResetDialog() {
  if (!sceneResetSubmitting) hideFacilityDialog(sceneResetDialog, btnResetScene);
}

async function confirmSceneReset() {
  if (sceneResetSubmitting || !sceneResetCounted) return;
  sceneResetSubmitting = true;
  btnConfirmSceneReset.disabled = true;
  btnCancelSceneReset.disabled = true;
  const result = await desktopAPI.resetScene({
    expectedSessionRevision: sceneResetCounted.sessionRevision,
    expectedFacilityRevision: sceneResetCounted.facilityRevision,
    correlationId: uid('scene-reset'),
  });
  sceneResetSubmitting = false;
  btnConfirmSceneReset.disabled = false;
  btnCancelSceneReset.disabled = false;
  saveStatus.dataset.wailsCommand = 'ResetScene';
  saveStatus.dataset.wailsResult = result.ok ? 'ok' : 'error';
  saveStatus.dataset.wailsRevision = String(Number(result.sessionRevision || 0));
  delete saveStatus.dataset.wailsTerminalId;
  updateSessionStateEvidenceDescription();
  if (result.ok && result.session) {
    applyLiveSession(result.session, result.sessionRevision);
    closeSceneResetDialog();
    setCoordinationStatus('СЦЕНА СБРОШЕНА');
    renderAll();
    return;
  }
  if (result.failure === 'stale-revision' && result.session) {
    applyLiveSession(result.session, result.sessionRevision);
    renderAll();
    renderSceneResetDialog(result.sessionRevision, result.previousFacilityRevision);
    setSceneResetStatus('СЦЕНА ИЗМЕНИЛАСЬ — ПРОВЕРЬТЕ СПИСОК');
    return;
  }
  setSceneResetStatus(`СЦЕНА НЕ СБРОШЕНА: ${facilityIssueMessage(result)}`);
}

async function recoverSelectedFacilityCondition() {
  const conditionID = selectedFacilityConditionID;
  const condition = selectedFacilityCondition(state.session);
  const recovery = condition?.recovery?.find(reference => reference.privateOverseerAction === true)
    || { privateOverseerAction: true };
  closeFacilityConfirmation(facilityRecoveryConfirmationDialog);
  const result = await desktopAPI.recoverFacilityCondition({
    conditionId: conditionID,
    expectedFacilityRevision: facilityRevision(),
    correlationId: uid('facility-recovery'),
    recovery: structuredClone(recovery),
  });
  reconcileFacilityOperation(result, 'УСЛОВИЕ ВОССТАНОВЛЕНО', 'condition', conditionID);
}

function refreshFacilityStateChoices() {
  const states = [...facilityDeviceStates.querySelectorAll('.facility-state-row')].map(row => ({
    id: row.querySelector('.facility-state-id').value.trim(),
    name: row.querySelector('.facility-state-name').value.trim(),
  })).filter(value => value.id);
  const selectedInitial = facilityDeviceInitialState.value;
  populateSelect(facilityDeviceInitialState, states, selectedInitial, 'ВЫБЕРИТЕ СОСТОЯНИЕ');
  for (const row of facilityDeviceTransitions.querySelectorAll('.facility-transition-row')) {
    for (const select of row.querySelectorAll('.facility-transition-source, .facility-transition-destination')) {
      const selected = select.value;
      populateSelect(select, states, selected, 'ВЫБЕРИТЕ СОСТОЯНИЕ');
    }
    for (const reference of row.querySelectorAll('.facility-precondition-row')) {
      if (reference.querySelector('.facility-precondition-device').value !== editingFacilityDeviceID) continue;
      const select = reference.querySelector('.facility-precondition-state');
      populateSelect(select, states, select.value, 'ВЫБЕРИТЕ СОСТОЯНИЕ');
    }
    row.facilityUpdatePresentation?.();
  }
}

function appendFacilityStateRow(value = {}) {
  const row = facilityStateRowTemplate.content.firstElementChild.cloneNode(true);
  row.facilityOriginal = structuredClone(value);
  const idInput = row.querySelector('.facility-state-id');
  const nameInput = row.querySelector('.facility-state-name');
  idInput.value = value.id || '';
  if (value.id) idInput.dataset.manual = 'true';
  nameInput.value = value.name || '';
  row.querySelectorAll('input').forEach(input => input.addEventListener('input', refreshFacilityStateChoices));
  idInput.addEventListener('input', () => { idInput.dataset.manual = 'true'; });
  nameInput.addEventListener('input', () => {
    if (idInput.dataset.manual === 'true') return;
    const existing = new Set([...facilityDeviceStates.querySelectorAll('.facility-state-id')]
      .filter(input => input !== idInput).map(input => input.value.trim()).filter(Boolean));
    idInput.value = suggestFacilityID('state', nameInput.value, existing, false);
    refreshFacilityStateChoices();
  });
  row.querySelector('[data-action="remove-facility-state"]').addEventListener('click', () => {
    row.remove();
    refreshFacilityStateChoices();
  });
  facilityDeviceStates.appendChild(row);
  refreshFacilityStateChoices();
  return row;
}

function appendFacilityTransitionRow(value = {}) {
  const row = facilityTransitionRowTemplate.content.firstElementChild.cloneNode(true);
  row.facilityOriginal = structuredClone(value);
  const idInput = row.querySelector('.facility-transition-id');
  const nameInput = row.querySelector('.facility-transition-name');
  idInput.value = value.id || '';
  if (value.id) idInput.dataset.manual = 'true';
  nameInput.value = value.name || '';
  facilityDeviceTransitions.appendChild(row);
  refreshFacilityStateChoices();
  row.querySelector('.facility-transition-source').value = value.sourceStateId || '';
  row.querySelector('.facility-transition-destination').value = value.destinationStateId || '';
  const summary = document.createElement('p');
  summary.className = 'facility-transition-summary';
  row.prepend(summary);
  const updatePresentation = () => {
    const stateNames = new Map([...facilityDeviceStates.querySelectorAll('.facility-state-row')].map(stateRow => [
      stateRow.querySelector('.facility-state-id').value,
      stateRow.querySelector('.facility-state-name').value,
    ]));
    const sourceID = row.querySelector('.facility-transition-source').value;
    const destinationID = row.querySelector('.facility-transition-destination').value;
    const preconditionCount = row.querySelectorAll('.facility-precondition-row').length;
    const effectCount = row.querySelectorAll('.facility-condition-effect-row').length;
    summary.textContent = `${nameInput.value.trim() || 'Новый переход'}: `
      + `${stateNames.get(sourceID) || sourceID || 'состояние не выбрано'} → `
      + `${stateNames.get(destinationID) || destinationID || 'состояние не выбрано'}`;
    referencesSummary.textContent = `ПРЕДУСЛОВИЯ: ${preconditionCount} · ЭФФЕКТЫ: ${effectCount}`;
  };
  row.facilityUpdatePresentation = updatePresentation;
  nameInput.addEventListener('input', () => {
    if (idInput.dataset.manual !== 'true') {
      const existing = new Set([...facilityDeviceTransitions.querySelectorAll('.facility-transition-id')]
        .filter(input => input !== idInput).map(input => input.value.trim()).filter(Boolean));
      idInput.value = suggestFacilityID('transition', nameInput.value, existing, false);
    }
    updatePresentation();
  });
  idInput.addEventListener('input', () => { idInput.dataset.manual = 'true'; });
  const references = document.createElement('details');
  references.className = 'facility-transition-references';
  references.open = Boolean(value.preconditions?.length || value.conditionEffects?.length);
  const referencesSummary = document.createElement('summary');
  const referenceFields = document.createElement('div');
  const referenceHelp = document.createElement('p');
  referenceHelp.className = 'facility-section-help';
  referenceHelp.textContent = 'Необязательные правила. Предусловие — что должно быть верно до действия (например, питание включено). Эффект — какую неисправность действие активирует или снимает. Для простой двери эти поля можно оставить пустыми.';
  const preconditions = document.createElement('div');
  preconditions.className = 'facility-transition-preconditions';
  const addPrecondition = document.createElement('button');
  addPrecondition.className = 'btn btn-secondary';
  addPrecondition.type = 'button';
  addPrecondition.textContent = 'ДОБАВИТЬ ПРЕДУСЛОВИЕ';
  addPrecondition.addEventListener('click', () => {
    references.open = true;
    appendFacilityTransitionPrecondition(preconditions, {}, updatePresentation);
    updatePresentation();
  });
  const effects = document.createElement('div');
  effects.className = 'facility-transition-condition-effects';
  const addEffect = document.createElement('button');
  addEffect.className = 'btn btn-secondary';
  addEffect.type = 'button';
  addEffect.textContent = 'ДОБАВИТЬ ЭФФЕКТ УСЛОВИЯ';
  addEffect.addEventListener('click', () => {
    references.open = true;
    appendFacilityTransitionConditionEffect(effects, {}, updatePresentation);
    updatePresentation();
  });
  referenceFields.append(referenceHelp, preconditions, addPrecondition, effects, addEffect);
  references.append(referencesSummary, referenceFields);
  row.appendChild(references);
  for (const precondition of value.preconditions || []) {
    appendFacilityTransitionPrecondition(preconditions, precondition, updatePresentation);
  }
  for (const effect of value.conditionEffects || []) {
    appendFacilityTransitionConditionEffect(effects, effect, updatePresentation);
  }
  row.querySelector('.facility-transition-source').addEventListener('change', updatePresentation);
  row.querySelector('.facility-transition-destination').addEventListener('change', updatePresentation);
  row.querySelector('[data-action="remove-facility-transition"]').addEventListener('click', () => row.remove());
  updatePresentation();
  return row;
}

function openFacilityDeviceDialog(deviceID = '') {
  facilityDeviceForm.reset();
  facilityDeviceStates.replaceChildren();
  facilityDeviceTransitions.replaceChildren();
  editingFacilityDeviceID = deviceID;
  delete facilityDeviceID.dataset.manual;
  const device = facilityDeviceByID(deviceID);
  facilityDeviceID.readOnly = Boolean(device);
  if (device) {
    facilityDeviceID.dataset.manual = 'true';
    facilityDeviceID.value = device.id;
    facilityDeviceName.value = device.name;
    facilityDeviceKind.value = device.kind;
    for (const stateValue of device.states || []) appendFacilityStateRow(stateValue);
    facilityDeviceInitialState.value = device.initialStateId;
    for (const transition of device.transitions || []) appendFacilityTransitionRow(transition);
  }
  setFacilityFeedback(facilityDeviceError);
  showFacilityDialog(facilityDeviceDialog, facilityDeviceName);
}

function readFacilityDeviceDraft() {
  const states = [...facilityDeviceStates.querySelectorAll('.facility-state-row')].map(row => ({
    ...row.facilityOriginal,
    id: row.querySelector('.facility-state-id').value.trim(),
    name: row.querySelector('.facility-state-name').value.trim(),
  }));
  const transitions = [...facilityDeviceTransitions.querySelectorAll('.facility-transition-row')].map(row => ({
    ...row.facilityOriginal,
    id: row.querySelector('.facility-transition-id').value.trim(),
    name: row.querySelector('.facility-transition-name').value.trim(),
    sourceStateId: row.querySelector('.facility-transition-source').value,
    destinationStateId: row.querySelector('.facility-transition-destination').value,
    preconditions: [...row.querySelectorAll('.facility-precondition-row')].map(reference => ({
      ...reference.facilityOriginal,
      deviceId: reference.querySelector('.facility-precondition-device').value,
      stateId: reference.querySelector('.facility-precondition-state').value,
    })),
    conditionEffects: [...row.querySelectorAll('.facility-condition-effect-row')].map(reference => ({
      ...reference.facilityOriginal,
      conditionId: reference.querySelector('.facility-effect-condition').value,
      active: reference.querySelector('.facility-effect-active').value === 'activate',
    })),
  }));
  const initialStateId = facilityDeviceInitialState.value;
  const device = {
    ...structuredClone(facilityDeviceByID(editingFacilityDeviceID) || {}),
    id: facilityDeviceID.value.trim(), name: facilityDeviceName.value.trim(), kind: facilityDeviceKind.value,
    initialStateId, currentStateId: facilityDeviceByID(editingFacilityDeviceID)?.currentStateId || initialStateId, states, transitions,
  };
  if (device.kind === 'custom') device.customKind ||= device.id;
  else delete device.customKind;
  return device;
}

// Returns at most one problem per state or transition row, in reading order.
function facilityDeviceRowProblems() {
  const problems = [];
  const check = (row, rules) => {
    const failed = rules.find(([invalid]) => invalid);
    if (failed) problems.push({ row, control: failed[1], message: failed[2] });
  };
  const stateIDs = new Set();
  const stateNames = new Set();
  for (const row of facilityDeviceStates.querySelectorAll('.facility-state-row')) {
    const id = row.querySelector('.facility-state-id');
    const name = row.querySelector('.facility-state-name');
    const value = id.value.trim();
    // The server compares state names case-insensitively after trimming.
    const normalizedName = name.value.trim().toLowerCase();
    check(row, [
      [!normalizedName, name, 'УКАЖИТЕ НАЗВАНИЕ СОСТОЯНИЯ'],
      [stateNames.has(normalizedName), name, `НАЗВАНИЕ УЖЕ ИСПОЛЬЗУЕТСЯ: ${name.value.trim()}`],
      [!value, id, 'УКАЖИТЕ ИДЕНТИФИКАТОР СОСТОЯНИЯ'],
      [stateIDs.has(value), id, `ИДЕНТИФИКАТОР УЖЕ ИСПОЛЬЗУЕТСЯ: ${value}`],
    ]);
    if (value) stateIDs.add(value);
    if (normalizedName) stateNames.add(normalizedName);
  }
  const transitionIDs = new Set();
  for (const row of facilityDeviceTransitions.querySelectorAll('.facility-transition-row')) {
    const id = row.querySelector('.facility-transition-id');
    const name = row.querySelector('.facility-transition-name');
    const source = row.querySelector('.facility-transition-source');
    const destination = row.querySelector('.facility-transition-destination');
    const value = id.value.trim();
    check(row, [
      [!name.value.trim(), name, 'УКАЖИТЕ НАЗВАНИЕ ПЕРЕХОДА'],
      [!value, id, 'УКАЖИТЕ ИДЕНТИФИКАТОР ПЕРЕХОДА'],
      [transitionIDs.has(value), id, `ИДЕНТИФИКАТОР УЖЕ ИСПОЛЬЗУЕТСЯ: ${value}`],
      [!stateIDs.has(source.value), source, 'ВЫБЕРИТЕ СОСТОЯНИЕ «ИЗ»'],
      [!stateIDs.has(destination.value), destination, 'ВЫБЕРИТЕ СОСТОЯНИЕ «В»'],
    ]);
    if (value) transitionIDs.add(value);
  }
  return problems;
}

function clearFacilityRowProblem(row) {
  row.querySelectorAll('.facility-row-error').forEach(note => note.remove());
  for (const control of row.querySelectorAll('[aria-invalid="true"]')) {
    control.removeAttribute('aria-invalid');
    control.removeAttribute('aria-describedby');
  }
}

function showFacilityRowProblems(problems) {
  problems.forEach(({ row, control, message }, index) => {
    const note = document.createElement('p');
    note.className = 'facility-row-error';
    note.id = `facilityRowProblem${index}`;
    note.textContent = message;
    row.appendChild(note);
    control.setAttribute('aria-invalid', 'true');
    control.setAttribute('aria-describedby', note.id);
    row.addEventListener('input', () => clearFacilityRowProblem(row), { once: true });
  });
}

function validateFacilityDeviceDraft(device) {
  if (!device.id) return ['УКАЖИТЕ СТАБИЛЬНЫЙ ИДЕНТИФИКАТОР УСТРОЙСТВА', facilityDeviceID];
  if (authoredFacility().devices.some(candidate => candidate.id === device.id && candidate.id !== editingFacilityDeviceID)) {
    return ['Такой идентификатор устройства уже используется', facilityDeviceID];
  }
  if (!device.name) return ['УКАЖИТЕ НАЗВАНИЕ УСТРОЙСТВА', facilityDeviceName];
  if (!device.kind) return ['ВЫБЕРИТЕ ТИП УСТРОЙСТВА', facilityDeviceKind];
  if (!device.states.length) return ['ДОБАВЬТЕ ХОТЯ БЫ ОДНО СОСТОЯНИЕ', btnAddFacilityState];
  const problems = facilityDeviceRowProblems();
  if (problems.length) return [`ИСПРАВЬТЕ ОТМЕЧЕННЫЕ СТРОКИ: ${problems.length}`, problems[0].control, problems];
  const stateIDs = new Set(device.states.map(stateValue => stateValue.id));
  if (!stateIDs.has(device.initialStateId)) return ['ВЫБЕРИТЕ НАЧАЛЬНОЕ СОСТОЯНИЕ', facilityDeviceInitialState];
  if (!stateIDs.has(device.currentStateId)) return ['НЕЛЬЗЯ УДАЛИТЬ ТЕКУЩЕЕ СОСТОЯНИЕ УСТРОЙСТВА', btnAddFacilityState];
  for (const row of facilityDeviceTransitions.querySelectorAll('.facility-precondition-row')) {
    const deviceSelect = row.querySelector('.facility-precondition-device');
    const stateSelect = row.querySelector('.facility-precondition-state');
    const target = deviceSelect.value === device.id ? device : facilityDeviceByID(deviceSelect.value);
    if (!target) return ['ВЫБЕРИТЕ УСТРОЙСТВО ПРЕДУСЛОВИЯ', deviceSelect];
    if (!target.states.some(value => value.id === stateSelect.value)) {
      return ['ВЫБЕРИТЕ СОСТОЯНИЕ ПРЕДУСЛОВИЯ', stateSelect];
    }
  }
  for (const select of facilityDeviceTransitions.querySelectorAll('.facility-effect-condition')) {
    if (!authoredFacility().conditions.some(value => value.id === select.value)) {
      return ['ВЫБЕРИТЕ ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ', select];
    }
  }
  return null;
}

function populateFacilityDeviceSelect(select, selected = '') {
  populateSelect(select, (authoredFacility()?.devices || []).map(device => ({ id: device.id, name: device.name })),
    selected, 'ВЫБЕРИТЕ УСТРОЙСТВО');
}

function updateFacilityConditionScopePresentation() {
  const terminalScope = facilityConditionScope.value === 'terminal';
  const deviceLabel = facilityConditionForm.querySelector('label[for="facilityConditionDevice"]');
  const terminalLabel = facilityConditionForm.querySelector('label[for="facilityConditionTerminal"]');
  deviceLabel.hidden = terminalScope;
  facilityConditionDevice.hidden = terminalScope;
  terminalLabel.hidden = !terminalScope;
  facilityConditionTerminal.hidden = !terminalScope;
}

function openFacilityConditionDialog(conditionID = '') {
  editingFacilityConditionID = typeof conditionID === 'string' ? conditionID : '';
  const condition = authoredFacility().conditions.find(value => value.id === editingFacilityConditionID);
  facilityConditionForm.reset();
  delete facilityConditionID.dataset.manual;
  facilityConditionID.readOnly = Boolean(condition);
  facilityConditionID.value = condition?.id || '';
  facilityConditionName.value = condition?.name || '';
  facilityConditionCategory.value = condition?.category || '';
  if (condition) facilityConditionID.dataset.manual = 'true';
  facilityConditionScope.value = condition?.terminal ? 'terminal' : 'device';
  populateFacilityDeviceSelect(facilityConditionDevice, condition?.device?.deviceId || selectedFacilityDeviceID);
  populateSelect(facilityConditionTerminal,
    (state.session?.terminals || []).map(terminal => ({ id: terminal.id, name: terminal.name })),
    condition?.terminal?.terminalId || '', 'ВЫБЕРИТЕ ТЕРМИНАЛ');
  facilityConditionCapability.value = condition?.effects
    ?.find(effect => effect.capabilityBlock)?.capabilityBlock?.capability || '';
  facilityConditionPrivateRecovery.checked = Boolean(
    condition?.recovery?.some(reference => reference.privateOverseerAction === true),
  );
  (condition?.initialActive ? facilityConditionInitialActive : facilityConditionInitialWorking).checked = true;
  updateFacilityConditionScopePresentation();
  setFacilityFeedback(facilityConditionError);
  showFacilityDialog(facilityConditionDialog, condition ? facilityConditionName : facilityConditionID);
}

function updateRecoveryTransitionChoices() {
  const device = facilityDeviceByID(facilityRecoveryProgramDevice.value);
  populateSelect(facilityRecoveryProgramTransition,
    (device?.transitions || []).map(transition => ({ id: transition.id, name: transition.name })),
    facilityRecoveryProgramTransition.value, 'ВЫБЕРИТЕ ПЕРЕХОД');
}

function openFacilityRecoveryProgramDialog(programID = '') {
  editingFacilityProgramID = typeof programID === 'string' ? programID : '';
  const program = authoredFacility().recoveryPrograms.find(value => value.id === editingFacilityProgramID);
  facilityRecoveryProgramForm.reset();
  delete facilityRecoveryProgramID.dataset.manual;
  facilityRecoveryProgramID.readOnly = Boolean(program);
  facilityRecoveryProgramID.value = program?.id || '';
  facilityRecoveryProgramName.value = program?.name || '';
  if (program) facilityRecoveryProgramID.dataset.manual = 'true';
  populateFacilityDeviceSelect(facilityRecoveryProgramDevice, program?.transitions?.[0]?.deviceId || selectedFacilityDeviceID);
  updateRecoveryTransitionChoices();
  facilityRecoveryProgramTransition.value = program?.transitions?.[0]?.transitionId || '';
  let extra = document.getElementById('facilityProgramExtraTransitions');
  if (!extra) {
    extra = document.createElement('div');
    extra.id = 'facilityProgramExtraTransitions';
    const add = document.createElement('button');
    add.type = 'button';
    add.className = 'btn btn-secondary';
    add.textContent = 'ДОБАВИТЬ ПЕРЕХОД ПРОГРАММЫ';
    add.addEventListener('click', () => appendFacilityActionRequest({}, extra));
    facilityRecoveryProgramError.before(extra, add);
  }
  extra.replaceChildren();
  for (const request of program?.transitions?.slice(1) || []) appendFacilityActionRequest(request, extra);
  setFacilityFeedback(facilityRecoveryProgramError);
  showFacilityDialog(facilityRecoveryProgramDialog, facilityRecoveryProgramName);
}

function updateFacilityBindingNodes() {
  const terminal = state.session?.terminals?.find(candidate => candidate.id === facilityBindingTerminal.value);
  const current = facilityBindingNode.value;
  const nodes = [];
  if (terminal) visitContentNodes(terminal.root, node => {
    if (node.id !== 'root') nodes.push({ id: node.id, name: node.name });
  });
  populateSelect(facilityBindingNode, nodes, current, 'ВЫБЕРИТЕ ЭЛЕМЕНТ');
  updateFacilityBindingBlocks();
}

function updateFacilityBindingStates() {
  const device = facilityDeviceByID(facilityBindingDevice.value);
  populateSelect(facilityBindingState, device?.states || [], facilityBindingState.value, 'ВЫБЕРИТЕ СОСТОЯНИЕ');
}

function openFacilityBindingDialog(binding = null) {
  editingFacilityBinding = binding?.property ? binding : null;
  facilityBindingForm.reset();
  facilityBindingKind.value = editingFacilityBinding?.property === 'recovery-action'
    ? 'command-action' : (editingFacilityBinding?.property || 'menu-name');
  facilityBindingActionRequests.replaceChildren();
  populateSelect(facilityBindingProgram, authoredFacility().recoveryPrograms,
    editingFacilityBinding?.programID || '', 'ОТДЕЛЬНЫЕ ПЕРЕХОДЫ');
  populateSelect(facilityBindingTerminal,
    (state.session?.terminals || []).map(terminal => ({ id: terminal.id, name: terminal.name })),
    editingFacilityBinding?.terminal.id || '', 'ВЫБЕРИТЕ ТЕРМИНАЛ');
  updateFacilityBindingNodes();
  if (editingFacilityBinding) facilityBindingNode.value = binding.node.id;
  populateFacilityDeviceSelect(facilityBindingDevice, editingFacilityBinding?.variant.when?.deviceId || selectedFacilityDeviceID);
  updateFacilityBindingStates();
  if (editingFacilityBinding?.variant.when) facilityBindingState.value = binding.variant.when.stateId;
  facilityBindingText.value = editingFacilityBinding?.variant.text || '';
  updateFacilityBindingBlocks();
  if (editingFacilityBinding?.block) facilityBindingBlock.value = binding.block.id;
  for (const request of editingFacilityBinding?.requests || []) appendFacilityActionRequest(request);
  updateFacilityBindingMode();
  setFacilityFeedback(facilityBindingError);
  showFacilityDialog(facilityBindingDialog, facilityBindingTerminal);
}

function openFacilityCommandActionEditor(terminalID, nodeID) {
  const commandBindings = facilityBindingCatalog().filter(value => value.terminal.id === terminalID
    && value.node.id === nodeID);
  const binding = commandBindings.find(value => value.property === 'command-action');
  const recovery = commandBindings.find(value => value.property === 'recovery-action');
  if (recovery) {
    openFacilityRecoveryProgramDialog(recovery.programID);
    return;
  }
  openFacilityBindingDialog(binding || null);
  if (binding) return;
  facilityBindingKind.value = 'command-action';
  facilityBindingTerminal.value = terminalID;
  updateFacilityBindingNodes();
  facilityBindingNode.value = nodeID;
  updateFacilityBindingMode();
}

function renderFacilityDependencies(report) {
  facilityDependencyList.replaceChildren();
  for (const dependency of report?.dependencies || []) {
    const row = document.createElement('div');
    row.className = 'facility-dependency-row';
    row.setAttribute('role', 'listitem');
    const kind = document.createElement('span');
    kind.className = 'facility-dependency-kind';
    const descriptions = {
      'transition-precondition': 'Предусловие перехода',
      'transition-condition-effect': 'Эффект перехода',
      'recovery-reference': 'Путь восстановления',
      'recovery-program-transition': 'Программа восстановления',
      'command-action': 'Действие команды',
      'name-variant': 'Название в меню',
      'entry-content-variant': 'Текст блока записи',
      visibility: 'Видимость элемента',
      availability: 'Доступность команды',
      'diagnostic-scope': 'Область неисправности',
      'diagnostic-effect': 'Эффект неисправности',
    };
    kind.textContent = descriptions[dependency.kind] || 'Связанная ссылка';
    const reference = document.createElement('span');
    reference.textContent = facilityDependencyDescription(dependency);
    const property = document.createElement('span');
    property.className = 'facility-dependency-property';
    property.textContent = `Технический путь: ${dependency.property}`;
    const navigate = document.createElement('button');
    navigate.type = 'button';
    navigate.className = 'btn btn-mini btn-secondary facility-dependency-navigation';
    navigate.textContent = 'ОТКРЫТЬ';
    navigate.setAttribute('aria-label', `Открыть ссылку ${dependency.sourceId || dependency.targetId || ''}`);
    navigate.addEventListener('click', () => navigateToFacilityDependency(dependency));
    row.append(kind, reference, property, navigate);
    facilityDependencyList.appendChild(row);
  }
  facilityDependencyStatus.textContent = report?.dependencies?.length
    ? `НАЙДЕНО ССЫЛОК: ${report.dependencies.length}` : 'ССЫЛКИ НЕ НАЙДЕНЫ';
}

function navigateToFacilityDependency(dependency) {
  const sourceID = dependency?.sourceId || '';
  const terminalID = dependency?.terminalId || '';
  const facility = authoredFacility();
  if (['transition-precondition', 'transition-condition-effect'].includes(dependency.kind)) {
    const slash = sourceID.indexOf('/');
    const deviceID = slash < 0 ? sourceID : sourceID.slice(0, slash);
    const transitionID = slash < 0 ? '' : sourceID.slice(slash + 1);
    if (facilityDeviceByID(deviceID)) {
      hideFacilityDialog(facilityDependencyDialog);
      selectedFacilityDeviceID = deviceID;
      renderFacilityWorkspace();
      openFacilityDeviceDialog(deviceID);
      const row = [...facilityDeviceTransitions.querySelectorAll('.facility-transition-row')]
        .find(value => value.querySelector('.facility-transition-id').value === transitionID);
      row?.scrollIntoView({ block: 'center' });
      row?.querySelector('input')?.focus();
      return;
    }
  }
  if (dependency.kind === 'recovery-program-transition') {
    const program = facility?.recoveryPrograms?.find(value => value.id === sourceID);
    if (program) {
      hideFacilityDialog(facilityDependencyDialog);
      openFacilityRecoveryProgramDialog(program.id);
      return;
    }
  }
  if (['recovery-reference', 'diagnostic-scope', 'diagnostic-effect'].includes(dependency.kind)) {
    const condition = facility?.conditions?.find(value => value.id === sourceID);
    if (condition) {
      hideFacilityDialog(facilityDependencyDialog);
      selectedFacilityConditionID = condition.id;
      renderFacilityWorkspace();
      openFacilityConditionDialog(condition.id);
      return;
    }
  }
  const program = facility?.recoveryPrograms?.find(value => value.id === sourceID);
  if (program) {
    hideFacilityDialog(facilityDependencyDialog);
    openFacilityRecoveryProgramDialog(program.id);
    return;
  }
  const terminal = state.session?.terminals?.find(value => value.id === terminalID);
  const blockLocation = terminal ? entryBlockLocation(terminal, sourceID) : null;
  const nodeID = blockLocation?.entry.id || sourceID;
  if (!terminal || !locateNode(terminal.root, nodeID)) {
    setFacilityFeedback(facilityDependencyError, 'СВЯЗАННЫЙ ЭЛЕМЕНТ БОЛЬШЕ НЕ СУЩЕСТВУЕТ ИЛИ НЕ МОЖЕТ БЫТЬ ОТКРЫТ.');
    return;
  }
  hideFacilityDialog(facilityDependencyDialog);
  state.editTerminalId = terminal.id;
  state.selectedNodeId = nodeID;
  selectWorkspace('terminal');
  renderAll();
  const focus = blockLocation
    ? nodeForm.querySelector(`[data-block-id="${CSS.escape(sourceID)}"] textarea`)
    : nodeForm.querySelector('input, select, textarea, button');
  focus?.focus();
}

function facilityDependencyDescription(dependency) {
  const terminal = state.session?.terminals?.find(value => value.id === dependency.terminalId);
  const node = terminal ? locateNode(terminal.root, dependency.sourceId)?.node : null;
  const block = terminal ? entryBlockLocation(terminal, dependency.sourceId) : null;
  const condition = authoredFacility()?.conditions?.find(value => value.id === dependency.sourceId);
  const program = authoredFacility()?.recoveryPrograms?.find(value => value.id === dependency.sourceId);
  let source = node?.name || block?.entry.name || condition?.name || program?.name || dependency.sourceId;
  if (['transition-precondition', 'transition-condition-effect'].includes(dependency.kind)) {
    const [deviceID, transitionID] = dependency.sourceId.split('/');
    const device = facilityDeviceByID(deviceID);
    source = device?.transitions?.find(value => value.id === transitionID)?.name || dependency.sourceId;
    if (device) source = `${device.name} › ${source}`;
  }
  const targetDevice = facilityDeviceByID(dependency.targetId);
  const targetCondition = authoredFacility()?.conditions?.find(value => value.id === dependency.targetId);
  const targetProgram = authoredFacility()?.recoveryPrograms?.find(value => value.id === dependency.targetId);
  const target = targetDevice?.name || targetCondition?.name || targetProgram?.name || dependency.targetId;
  return `${terminal ? `${terminal.name} › ` : ''}${source} → ${target}`;
}

async function inspectSelectedFacilityDevice() {
  const device = facilityDeviceByID(selectedFacilityDeviceID, state.session);
  if (!device) return null;
  facilityDependencyStatus.textContent = 'ПОИСК ЗАВИСИМОСТЕЙ…';
  setFacilityFeedback(facilityDependencyError);
  showFacilityDialog(facilityDependencyDialog, btnCloseFacilityDependencies);
  const result = await desktopAPI.inspectFacilityDependencies({
    target: { kind: 'device', entityId: device.id },
    expectedSessionRevision: currentSessionRevision(),
    expectedFacilityRevision: Number(state.session.facility?.revision || 0),
  });
  if (!result?.ok || !result.report) {
    facilityDependencyStatus.textContent = '';
    setFacilityFeedback(facilityDependencyError, facilityIssueMessage(result));
    return null;
  }
  renderFacilityDependencies(result.report);
  return result.report;
}

function replacementTransitionID(device, transitionID) {
  return device.transitions?.some(transition => transition.id === transitionID)
    ? transitionID : (device.transitions?.[0]?.id || transitionID);
}

function replacementStateID(device, stateID) {
  return device.states?.some(value => value.id === stateID) ? stateID : device.initialStateId;
}

function rewriteFacilityDeviceReferences(session, oldID, replacementID) {
  const replacement = facilityDeviceByID(replacementID, session);
  let changes = 0;
  const equality = value => {
    if (value?.deviceId !== oldID) return;
    value.deviceId = replacementID;
    value.stateId = replacementStateID(replacement, value.stateId);
    changes++;
  };
  const request = value => {
    if (value?.deviceId !== oldID) return;
    value.deviceId = replacementID;
    value.transitionId = replacementTransitionID(replacement, value.transitionId);
    changes++;
  };
  for (const device of session.facility.devices || []) {
    for (const transition of device.transitions || []) {
      (transition.preconditions || []).forEach(equality);
    }
  }
  for (const condition of session.facility.conditions || []) {
    if (condition.device?.deviceId === oldID) {
      condition.device.deviceId = replacementID;
      changes++;
    }
    for (const recovery of condition.recovery || []) request(recovery.transition);
  }
  for (const program of session.facility.recoveryPrograms || []) (program.transitions || []).forEach(request);
  for (const { node } of facilityNodeCatalog(session)) {
    (node.facilityNameVariants || []).forEach(variant => equality(variant.when));
    equality(node.visibleWhen);
    equality(node.availableWhen);
    for (const block of node.blocks || []) {
      (block.facilityTextVariants || []).forEach(variant => equality(variant.when));
    }
    const action = node.stateChange?.facilityAction;
    (action?.transitions?.transitions || []).forEach(request);
  }
  return changes;
}

function prepareFacilityRepair(report) {
  const source = facilityDeviceByID(selectedFacilityDeviceID);
  if (!source) return;
  const dependencies = report?.dependencies || [];
  populateSelect(facilityRepairTarget,
    authoredFacility().devices.filter(device => device.id !== source.id), '', 'ВЫБЕРИТЕ УСТРОЙСТВО');
  pendingFacilityRepair = { sourceID: source.id, dependencies, candidate: null };
  setFacilityFeedback(facilityRepairError, dependencies.length
    ? `Устройство используется: ${dependencies.length} ссылок. Выполните полное переназначение.`
    : 'ССЫЛКИ НЕ НАЙДЕНЫ. УСТРОЙСТВО МОЖНО УДАЛИТЬ.');
  facilityRepairImpact.textContent = '';
  btnDeleteFacilityWithoutRepair.disabled = dependencies.length > 0;
  btnApplyFacilityRepair.disabled = true;
  btnApplyFacilityRepair.hidden = true;
  hideFacilityDialog(facilityDependencyDialog);
  showFacilityDialog(facilityRepairDialog, dependencies.length ? facilityRepairTarget : btnDeleteFacilityWithoutRepair);
}

async function openFacilityRepair() {
  const report = await inspectSelectedFacilityDevice();
  if (report) prepareFacilityRepair(report);
}

function validateFacilityRepair() {
  if (!pendingFacilityRepair) return;
  const replacementID = facilityRepairTarget.value;
  if (!replacementID) {
    facilityRepairImpact.textContent = 'ВЫБЕРИТЕ УСТРОЙСТВО ДЛЯ ПЕРЕНАЗНАЧЕНИЯ';
    btnApplyFacilityRepair.disabled = true;
    btnApplyFacilityRepair.hidden = true;
    return;
  }
  const candidate = structuredClone(state.session);
  const changed = rewriteFacilityDeviceReferences(candidate, pendingFacilityRepair.sourceID, replacementID);
  pendingFacilityRepair.replacementID = replacementID;
  facilityRepairImpact.textContent = `ПЕРЕНАЗНАЧЕНО ССЫЛОК: ${changed} · УДАЛЯЕТСЯ ${pendingFacilityRepair.sourceID} · НАЗНАЧАЕТСЯ ${replacementID}`;
  btnApplyFacilityRepair.disabled = false;
  btnApplyFacilityRepair.hidden = false;
}

// applyFacilityDeletion deletes the repaired device, first moving its
// references to replacementID when one is given.
function applyFacilityDeletion(replacementID) {
  const sourceID = pendingFacilityRepair?.sourceID;
  if (!sourceID) return;
  const name = facilityDeviceByID(sourceID)?.name || sourceID;
  selectedFacilityDeviceID = '';
  pendingFacilityRepair = null;
  hideFacilityDialog(facilityRepairDialog, btnAddFacilityDevice);
  commitFacilityEdit(`Удаление «${name}»`, session => {
    if (replacementID) rewriteFacilityDeviceReferences(session, sourceID, replacementID);
    const facility = ensureFacility(session);
    facility.devices = facility.devices.filter(device => device.id !== sourceID);
  });
}

function installFacilityBindingControls() {
  if (facilityBindingKind) return;
  const kindLabel = document.createElement('label');
  kindLabel.className = 'field-label-inline';
  kindLabel.htmlFor = 'facilityBindingKind';
  kindLabel.textContent = 'ТИП ПРИВЯЗКИ';
  facilityBindingKind = document.createElement('select');
  facilityBindingKind.className = 'field-input';
  facilityBindingKind.id = 'facilityBindingKind';
  facilityBindingKind.innerHTML = `
    <option value="menu-name">НАЗВАНИЕ В МЕНЮ</option>
    <option value="entry-content">ТЕКСТ БЛОКА ЗАПИСИ</option>
    <option value="visibility">ВИДИМОСТЬ ЭЛЕМЕНТА</option>
    <option value="availability">ДОСТУПНОСТЬ КОМАНДЫ</option>
    <option value="command-action">ДЕЙСТВИЕ КОМАНДЫ</option>`;
  const terminalLabel = facilityBindingForm.querySelector('label[for="facilityBindingTerminal"]');
  facilityBindingForm.insertBefore(kindLabel, terminalLabel);
  facilityBindingForm.insertBefore(facilityBindingKind, terminalLabel);

  const blockLabel = document.createElement('label');
  blockLabel.className = 'field-label-inline';
  blockLabel.htmlFor = 'facilityBindingBlock';
  blockLabel.textContent = 'БЛОК ЗАПИСИ';
  facilityBindingBlock = document.createElement('select');
  facilityBindingBlock.className = 'field-input';
  facilityBindingBlock.id = 'facilityBindingBlock';
  blockLabel.hidden = true;
  facilityBindingBlock.hidden = true;
  const deviceLabel = facilityBindingForm.querySelector('label[for="facilityBindingDevice"]');
  facilityBindingForm.insertBefore(blockLabel, deviceLabel);
  facilityBindingForm.insertBefore(facilityBindingBlock, deviceLabel);

  const action = document.createElement('fieldset');
  action.id = 'facilityBindingAction';
  action.hidden = true;
  const legend = document.createElement('legend');
  legend.textContent = 'АТОМАРНЫЕ ПЕРЕХОДЫ КОМАНДЫ';
  const programLabel = document.createElement('label');
  programLabel.textContent = 'ЧТО ЗАПУСТИТЬ';
  programLabel.htmlFor = 'facilityBindingProgram';
  facilityBindingProgram = document.createElement('select');
  facilityBindingProgram.id = 'facilityBindingProgram';
  facilityBindingProgram.className = 'field-input';
  facilityBindingProgram.addEventListener('change', updateFacilityBindingMode);
  facilityBindingActionRequests = document.createElement('div');
  facilityBindingActionRequests.id = 'facilityBindingActionRequests';
  btnAddFacilityBindingAction = document.createElement('button');
  btnAddFacilityBindingAction.className = 'btn btn-secondary';
  btnAddFacilityBindingAction.type = 'button';
  btnAddFacilityBindingAction.textContent = 'ДОБАВИТЬ ПЕРЕХОД К ДЕЙСТВИЮ';
  btnAddFacilityBindingAction.addEventListener('click', () => appendFacilityActionRequest());
  // An ordinary command becomes state-changing, so the author writes the texts players will see.
  const completedNameLabel = document.createElement('label');
  completedNameLabel.htmlFor = 'facilityBindingCompletedName';
  completedNameLabel.textContent = 'НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ';
  facilityBindingCompletedName = document.createElement('input');
  facilityBindingCompletedName.id = 'facilityBindingCompletedName';
  facilityBindingCompletedName.className = 'field-input';
  facilityBindingCompletedName.maxLength = 256;
  facilityBindingCompletedName.autocomplete = 'off';
  facilityBindingCompletedName.placeholder = 'Например: Выход открыт';
  const confirmationLabel = document.createElement('label');
  confirmationLabel.htmlFor = 'facilityBindingConfirmationText';
  confirmationLabel.textContent = 'ЗАПРОС ПОДТВЕРЖДЕНИЯ';
  facilityBindingConfirmationText = document.createElement('textarea');
  facilityBindingConfirmationText.id = 'facilityBindingConfirmationText';
  facilityBindingConfirmationText.className = 'field-input';
  facilityBindingConfirmationText.rows = 2;
  facilityBindingConfirmationText.placeholder = 'Например: Открыть гермодверь и включить сигнал?';
  action.append(legend, programLabel, facilityBindingProgram, facilityBindingActionRequests, btnAddFacilityBindingAction,
    completedNameLabel, facilityBindingCompletedName, confirmationLabel, facilityBindingConfirmationText);
  facilityBindingForm.insertBefore(action, facilityBindingForm.querySelector('#facilityBindingStatus'));
  facilityBindingKind.addEventListener('change', updateFacilityBindingMode);
  facilityBindingNode.addEventListener('change', updateFacilityBindingBlocks);
  facilityBindingNode.addEventListener('change', updateFacilityBindingCommandTexts);
}

function updateFacilityBindingCommandTexts() {
  const terminal = state.session?.terminals?.find(candidate => candidate.id === facilityBindingTerminal.value);
  const node = terminal ? locateNode(terminal.root, facilityBindingNode.value)?.node : null;
  const needed = facilityBindingKind.value === 'command-action' && node?.type === 'command'
    && !node.stateChange && !node.terminalTransition;
  for (const control of [facilityBindingCompletedName, facilityBindingConfirmationText]) {
    control.hidden = !needed;
    control.required = needed;
    facilityBindingForm.querySelector(`label[for="${control.id}"]`).hidden = !needed;
  }
}

function updateFacilityBindingBlocks() {
  const terminal = state.session?.terminals?.find(candidate => candidate.id === facilityBindingTerminal.value);
  const node = terminal ? locateNode(terminal.root, facilityBindingNode.value)?.node : null;
  populateSelect(facilityBindingBlock, (node?.blocks || []).map(block => ({
    id: block.id,
    name: block.initialText?.slice(0, 48) || block.id,
  })), facilityBindingBlock.value, 'ВЫБЕРИТЕ БЛОК');
}

function appendFacilityActionRequest(value = {}, container = facilityBindingActionRequests) {
  const row = document.createElement('div');
  row.facilityOriginal = structuredClone(value);
  row.className = 'facility-transition-row facility-action-request-row';
  const deviceLabel = document.createElement('label');
  deviceLabel.textContent = 'УСТРОЙСТВО ПЕРЕХОДА';
  const device = document.createElement('select');
  device.className = 'field-input facility-action-device';
  device.setAttribute('aria-label', 'УСТРОЙСТВО ПЕРЕХОДА');
  populateFacilityDeviceSelect(device, value.deviceId || '');
  deviceLabel.appendChild(device);
  const transitionLabel = document.createElement('label');
  transitionLabel.textContent = 'ПЕРЕХОД КОМАНДЫ';
  const transition = document.createElement('select');
  transition.className = 'field-input facility-action-transition';
  transition.setAttribute('aria-label', 'ПЕРЕХОД КОМАНДЫ');
  const update = () => {
    const selected = facilityDeviceByID(device.value);
    populateSelect(transition, selected?.transitions || [], transition.value || value.transitionId || '', 'ВЫБЕРИТЕ ПЕРЕХОД');
  };
  device.addEventListener('change', update);
  transitionLabel.appendChild(transition);
  const remove = document.createElement('button');
  remove.className = 'btn btn-danger';
  remove.type = 'button';
  remove.textContent = 'УДАЛИТЬ';
  remove.setAttribute('aria-label', 'Удалить переход действия');
  remove.addEventListener('click', () => row.remove());
  row.append(deviceLabel, transitionLabel, remove);
  container.appendChild(row);
  update();
}

function validateFacilityActionRows(container) {
  for (const row of container.querySelectorAll('.facility-action-request-row')) {
    const device = row.querySelector('.facility-action-device');
    const transition = row.querySelector('.facility-action-transition');
    const target = facilityDeviceByID(device.value);
    if (!target) return ['ВЫБЕРИТЕ УСТРОЙСТВО ПЕРЕХОДА', device];
    if (!(target.transitions || []).some(value => value.id === transition.value)) {
      return ['ВЫБЕРИТЕ ПЕРЕХОД', transition];
    }
  }
  return null;
}

function readFacilityActionRows(container) {
  return [...container.querySelectorAll('.facility-action-request-row')].map(row => ({
    ...row.facilityOriginal,
    deviceId: row.querySelector('.facility-action-device').value,
    transitionId: row.querySelector('.facility-action-transition').value,
  }));
}

function updateFacilityBindingMode() {
  const kind = facilityBindingKind.value;
  const instructions = {
    'menu-name': 'Название в меню: выберите терминал, элемент и состояние устройства. Например, при открытой двери элемент «Шлюз закрыт» станет «Путь свободен». Это меняет только надпись, а не состояние двери.',
    'entry-content': 'Текст записи: выберите терминал, запись и конкретный блок. Для состояния двери «Открыта» укажите «Путь свободен». В других состояниях останется исходный текст блока.',
    visibility: 'Видимость: элемент появится только в выбранном состоянии. Например, маршрут за дверью виден, когда дверь «Открыта».',
    availability: 'Доступность: команда остаётся видимой, но её можно запросить только в выбранном состоянии. Само действие команды настраивается отдельно.',
    'command-action': 'Действие: выберите команду терминала, затем отдельные переходы или готовую программу в поле «Что запустить». Например, «Открыть дверь» запускает «Заперта» → «Открыта». Несколько переходов выполнятся вместе после подтверждения Смотрителем; все предусловия проверяются до выполнения.',
  };
  document.getElementById('facilityBindingDialogDescription').textContent = instructions[kind];
  const usesText = ['menu-name', 'entry-content'].includes(kind);
  const usesEquality = kind !== 'command-action';
  facilityBindingForm.querySelector('label[for="facilityBindingDevice"]').hidden = !usesEquality;
  facilityBindingDevice.hidden = !usesEquality;
  facilityBindingForm.querySelector('label[for="facilityBindingState"]').hidden = !usesEquality;
  facilityBindingState.hidden = !usesEquality;
  facilityBindingForm.querySelector('label[for="facilityBindingText"]').hidden = !usesText;
  facilityBindingText.hidden = !usesText;
  facilityBindingForm.querySelector('label[for="facilityBindingBlock"]').hidden = kind !== 'entry-content';
  facilityBindingBlock.hidden = kind !== 'entry-content';
  if (kind === 'entry-content') updateFacilityBindingBlocks();
  document.getElementById('facilityBindingAction').hidden = kind !== 'command-action';
  const usesProgram = Boolean(facilityBindingProgram.value);
  facilityBindingActionRequests.hidden = usesProgram;
  btnAddFacilityBindingAction.hidden = usesProgram;
  if (kind === 'command-action' && !usesProgram && !facilityBindingActionRequests.children.length) appendFacilityActionRequest();
  updateFacilityBindingCommandTexts();
}

function installFacilityConditionScopeControls() {
  if (facilityConditionScope) return;
  const scopeLabel = document.createElement('label');
  scopeLabel.className = 'field-label-inline';
  scopeLabel.htmlFor = 'facilityConditionScope';
  scopeLabel.textContent = 'ОБЛАСТЬ УСЛОВИЯ';
  facilityConditionScope = document.createElement('select');
  facilityConditionScope.className = 'field-input';
  facilityConditionScope.id = 'facilityConditionScope';
  facilityConditionScope.innerHTML = '<option value="device">УСТРОЙСТВО</option><option value="terminal">ТЕРМИНАЛ</option>';
  const deviceLabel = facilityConditionForm.querySelector('label[for="facilityConditionDevice"]');
  facilityConditionForm.insertBefore(scopeLabel, deviceLabel);
  facilityConditionForm.insertBefore(facilityConditionScope, deviceLabel);
  const terminalLabel = document.createElement('label');
  terminalLabel.className = 'field-label-inline';
  terminalLabel.htmlFor = 'facilityConditionTerminal';
  terminalLabel.textContent = 'ТЕРМИНАЛ';
  facilityConditionTerminal = document.createElement('select');
  facilityConditionTerminal.className = 'field-input';
  facilityConditionTerminal.id = 'facilityConditionTerminal';
  terminalLabel.hidden = true;
  facilityConditionTerminal.hidden = true;
  facilityConditionForm.insertBefore(terminalLabel, facilityConditionForm.querySelector('label[for="facilityConditionCapability"]'));
  facilityConditionForm.insertBefore(facilityConditionTerminal, facilityConditionForm.querySelector('label[for="facilityConditionCapability"]'));
  facilityConditionScope.addEventListener('change', updateFacilityConditionScopePresentation);
}

function installFacilitySelectionControls() {
  if (btnEditFacilityDeviceGraph) return;
  btnEditFacilityDeviceGraph = document.createElement('button');
  btnEditFacilityDeviceGraph.className = 'btn btn-secondary';
  btnEditFacilityDeviceGraph.type = 'button';
  btnEditFacilityDeviceGraph.textContent = 'РЕДАКТИРОВАТЬ ГРАФ';
  btnEditFacilityDeviceGraph.addEventListener('click', () => openFacilityDeviceDialog(selectedFacilityDeviceID));
  btnInspectFacilityDependencies.before(btnEditFacilityDeviceGraph);
}

function appendFacilityTransitionPrecondition(container, value = {}, onChange = () => {}) {
  const row = document.createElement('div');
  row.facilityOriginal = structuredClone(value);
  row.className = 'facility-transition-reference-row facility-precondition-row';
  const device = document.createElement('select');
  device.className = 'field-input facility-precondition-device';
  device.setAttribute('aria-label', 'УСТРОЙСТВО ПРЕДУСЛОВИЯ');
  populateFacilityDeviceSelect(device, value.deviceId || '');
  const stateSelect = document.createElement('select');
  stateSelect.className = 'field-input facility-precondition-state';
  stateSelect.setAttribute('aria-label', 'СОСТОЯНИЕ ПРЕДУСЛОВИЯ');
  const update = () => {
    const target = facilityDeviceByID(device.value);
    populateSelect(stateSelect, target?.states || [], stateSelect.value || value.stateId || '', 'ВЫБЕРИТЕ СОСТОЯНИЕ');
  };
  device.addEventListener('change', () => {
    update();
    onChange();
  });
  stateSelect.addEventListener('change', onChange);
  const remove = document.createElement('button');
  remove.className = 'btn btn-danger';
  remove.type = 'button';
  remove.textContent = 'УДАЛИТЬ';
  remove.setAttribute('aria-label', 'Удалить предусловие');
  remove.addEventListener('click', () => {
    row.remove();
    onChange();
  });
  row.append(device, stateSelect, remove);
  container.appendChild(row);
  update();
}

function appendFacilityTransitionConditionEffect(container, value = {}, onChange = () => {}) {
  const row = document.createElement('div');
  row.facilityOriginal = structuredClone(value);
  row.className = 'facility-transition-reference-row facility-condition-effect-row';
  const condition = document.createElement('select');
  condition.className = 'field-input facility-effect-condition';
  condition.setAttribute('aria-label', 'ДИАГНОСТИЧЕСКОЕ УСЛОВИЕ ПЕРЕХОДА');
  populateSelect(condition, (authoredFacility()?.conditions || []).map(item => ({ id: item.id, name: item.name })),
    value.conditionId || '', 'ВЫБЕРИТЕ УСЛОВИЕ');
  const active = document.createElement('select');
  active.className = 'field-input facility-effect-active';
  active.setAttribute('aria-label', 'ДЕЙСТВИЕ С НЕИСПРАВНОСТЬЮ');
  active.innerHTML = '<option value="activate">АКТИВИРОВАТЬ НЕИСПРАВНОСТЬ</option>'
    + '<option value="remove">СНЯТЬ НЕИСПРАВНОСТЬ</option>';
  active.value = value.active === true ? 'activate' : 'remove';
  condition.addEventListener('change', onChange);
  active.addEventListener('change', onChange);
  const remove = document.createElement('button');
  remove.className = 'btn btn-danger';
  remove.type = 'button';
  remove.textContent = 'УДАЛИТЬ';
  remove.setAttribute('aria-label', 'Удалить эффект условия');
  remove.addEventListener('click', () => {
    row.remove();
    onChange();
  });
  row.append(condition, active, remove);
  container.appendChild(row);
}

installFacilityBindingControls();
installFacilityConditionScopeControls();
installFacilitySelectionControls();

for (const button of document.querySelectorAll('[data-story-step]')) {
  button.addEventListener('click', () => setFacilityStoryStep(Number(button.dataset.storyStep)));
}
document.getElementById('btnFacilityStoryBack').addEventListener('click', () => setFacilityStoryStep(facilityStoryStep - 1, true));
document.getElementById('btnFacilityStoryNext').addEventListener('click', () => setFacilityStoryStep(facilityStoryStep + 1, true));

terminalWorkspaceTab.addEventListener('click', () => selectWorkspace('terminal'));
facilityWorkspaceTab.addEventListener('click', () => selectWorkspace('facility'));
document.getElementById('facilityDeviceSearch')?.addEventListener('input', renderFacilityWorkspace);
document.getElementById('btnClearFacilitySelection')?.addEventListener('click', () => {
  selectedFacilityDeviceID = '';
  selectedFacilityConditionID = '';
  renderFacilityWorkspace();
});
for (const tab of [terminalWorkspaceTab, facilityWorkspaceTab]) {
  tab.addEventListener('keydown', event => {
    if (!['ArrowLeft', 'ArrowRight'].includes(event.key)) return;
    event.preventDefault();
    const next = tab === terminalWorkspaceTab ? facilityWorkspaceTab : terminalWorkspaceTab;
    selectWorkspace(next === facilityWorkspaceTab ? 'facility' : 'terminal');
    next.focus();
  });
}

btnAddFacilityDevice.addEventListener('click', () => openFacilityDeviceDialog());
btnAddFacilityState.addEventListener('click', () => appendFacilityStateRow());
btnAddFacilityTransition.addEventListener('click', () => appendFacilityTransitionRow());
btnCancelFacilityDevice.addEventListener('click', () => {
  editingFacilityDeviceID = '';
  hideFacilityDialog(facilityDeviceDialog, btnAddFacilityDevice);
});
facilityDeviceDialog.addEventListener('cancel', event => {
  event.preventDefault();
  editingFacilityDeviceID = '';
  hideFacilityDialog(facilityDeviceDialog, btnAddFacilityDevice);
});
facilityDeviceForm.addEventListener('submit', event => {
  event.preventDefault();
  const device = readFacilityDeviceDraft();
  facilityDeviceForm.querySelectorAll('.facility-state-row, .facility-transition-row').forEach(clearFacilityRowProblem);
  const invalid = validateFacilityDeviceDraft(device);
  if (invalid) {
    const [message, field, problems = []] = invalid;
    setFacilityFeedback(facilityDeviceError, message, field);
    showFacilityRowProblems(problems);
    return;
  }
  const editingID = editingFacilityDeviceID;
  selectedFacilityDeviceID = device.id;
  selectedFacilityConditionID = '';
  editingFacilityDeviceID = '';
  hideFacilityDialog(facilityDeviceDialog, btnAddFacilityDevice);
  commitFacilityEdit(`Устройство «${device.name}»`, session => {
    const facility = ensureFacility(session);
    const index = editingID ? facility.devices.findIndex(candidate => candidate.id === editingID) : -1;
    if (index >= 0) facility.devices.splice(index, 1, structuredClone(device));
    else facility.devices.push(structuredClone(device));
  }, reason => {
    editingFacilityDeviceID = editingID;
    showFacilityDialog(facilityDeviceDialog);
    setFacilityFeedback(facilityDeviceError, reason);
  });
});
facilityDeviceName.addEventListener('input', () => {
  if (facilityDeviceID.dataset.manual === 'true') return;
  facilityDeviceID.value = suggestFacilityID('device', facilityDeviceName.value,
    new Set((authoredFacility()?.devices || []).map(device => device.id)));
});
facilityDeviceID.addEventListener('input', () => { facilityDeviceID.dataset.manual = 'true'; });

btnAddFacilityCondition.addEventListener('click', openFacilityConditionDialog);
btnCancelFacilityCondition.addEventListener('click', () => {
  editingFacilityConditionID = '';
  hideFacilityDialog(facilityConditionDialog, btnAddFacilityCondition);
});
facilityConditionDialog.addEventListener('cancel', event => {
  event.preventDefault();
  editingFacilityConditionID = '';
  hideFacilityDialog(facilityConditionDialog, btnAddFacilityCondition);
});
facilityConditionForm.addEventListener('submit', event => {
  event.preventDefault();
  const id = facilityConditionID.value.trim();
  const name = facilityConditionName.value.trim();
  const original = authoredFacility().conditions.find(condition => condition.id === editingFacilityConditionID);
  if (!id || authoredFacility().conditions.some(condition => condition.id === id && condition.id !== editingFacilityConditionID)) {
    return setFacilityFeedback(facilityConditionError, 'УКАЖИТЕ УНИКАЛЬНЫЙ ИДЕНТИФИКАТОР УСЛОВИЯ', facilityConditionID);
  }
  if (!name) return setFacilityFeedback(facilityConditionError, 'УКАЖИТЕ НАЗВАНИЕ УСЛОВИЯ', facilityConditionName);
  if (!facilityConditionCategory.value) {
    return setFacilityFeedback(facilityConditionError, 'ВЫБЕРИТЕ КАТЕГОРИЮ УСЛОВИЯ', facilityConditionCategory);
  }
  if (!facilityConditionCapability.value && !original) {
    return setFacilityFeedback(facilityConditionError, 'ВЫБЕРИТЕ БЛОКИРУЕМУЮ ВОЗМОЖНОСТЬ', facilityConditionCapability);
  }
  const terminalScope = facilityConditionScope.value === 'terminal';
  if ((!terminalScope && !facilityConditionDevice.value) || (terminalScope && !facilityConditionTerminal.value)) {
    return setFacilityFeedback(facilityConditionError, terminalScope ? 'ВЫБЕРИТЕ ТЕРМИНАЛ' : 'ВЫБЕРИТЕ УСТРОЙСТВО',
      terminalScope ? facilityConditionTerminal : facilityConditionDevice);
  }
  const effects = structuredClone(original?.effects || []);
  const capabilityIndex = effects.findIndex(effect => effect.capabilityBlock);
  if (facilityConditionCapability.value) {
    const previous = capabilityIndex >= 0 ? effects[capabilityIndex] : {};
    const capability = {
      ...previous,
      capabilityBlock: {
        ...previous.capabilityBlock,
        capability: facilityConditionCapability.value,
      },
    };
    if (capabilityIndex >= 0) effects.splice(capabilityIndex, 1, capability);
    else effects.push(capability);
  } else if (capabilityIndex >= 0) {
    effects.splice(capabilityIndex, 1);
  }
  const recovery = structuredClone(original?.recovery || []);
  const privateRecoveryIndex = recovery.findIndex(reference => reference.privateOverseerAction === true);
  if (facilityConditionPrivateRecovery.checked && privateRecoveryIndex < 0) {
    recovery.push({ privateOverseerAction: true });
  } else if (!facilityConditionPrivateRecovery.checked && privateRecoveryIndex >= 0) {
    recovery.splice(privateRecoveryIndex, 1);
  }
  if (!recovery.length) {
    return setFacilityFeedback(facilityConditionError, 'УКАЖИТЕ ДОСТУПНЫЙ ПУТЬ ВОССТАНОВЛЕНИЯ', facilityConditionPrivateRecovery);
  }
  const condition = {
    ...structuredClone(original || {}),
    id, name, category: facilityConditionCategory.value, effects, recovery,
    initialActive: facilityConditionInitialActive.checked,
  };
  // Only a new fault starts in its starting activity; an existing one keeps its live value.
  if (!original) condition.currentActive = condition.initialActive;
  if (condition.category === 'custom') condition.customCategory ||= condition.id;
  else delete condition.customCategory;
  if (terminalScope) condition.terminal = { ...original?.terminal, terminalId: facilityConditionTerminal.value };
  else condition.device = { ...original?.device, deviceId: facilityConditionDevice.value };
  if (terminalScope) delete condition.device;
  else delete condition.terminal;
  const originalID = original?.id || '';
  const editingID = editingFacilityConditionID;
  editingFacilityConditionID = '';
  hideFacilityDialog(facilityConditionDialog, btnAddFacilityCondition);
  commitFacilityEdit(`Неисправность «${condition.name}»`, session => {
    const facility = ensureFacility(session);
    const index = originalID ? facility.conditions.findIndex(candidate => candidate.id === originalID) : -1;
    if (index >= 0) facility.conditions.splice(index, 1, structuredClone(condition));
    else facility.conditions.push(structuredClone(condition));
  }, reason => {
    editingFacilityConditionID = editingID;
    showFacilityDialog(facilityConditionDialog);
    setFacilityFeedback(facilityConditionError, reason);
  });
});
facilityConditionName.addEventListener('input', () => {
  if (facilityConditionID.dataset.manual === 'true') return;
  facilityConditionID.value = suggestFacilityID('condition', facilityConditionName.value,
    new Set((authoredFacility()?.conditions || []).map(condition => condition.id)));
});
facilityConditionID.addEventListener('input', () => { facilityConditionID.dataset.manual = 'true'; });

btnAddFacilityRecoveryProgram.addEventListener('click', openFacilityRecoveryProgramDialog);
facilityRecoveryProgramDevice.addEventListener('change', updateRecoveryTransitionChoices);
btnCancelFacilityRecoveryProgram.addEventListener('click', () =>
  hideFacilityDialog(facilityRecoveryProgramDialog, btnAddFacilityRecoveryProgram));
facilityRecoveryProgramDialog.addEventListener('cancel', event => {
  event.preventDefault();
  hideFacilityDialog(facilityRecoveryProgramDialog, btnAddFacilityRecoveryProgram);
});
facilityRecoveryProgramForm.addEventListener('submit', event => {
  event.preventDefault();
  const id = facilityRecoveryProgramID.value.trim();
  const name = facilityRecoveryProgramName.value.trim();
  if (!id || authoredFacility().recoveryPrograms.some(program => program.id === id && program.id !== editingFacilityProgramID)) {
    return setFacilityFeedback(facilityRecoveryProgramError, 'УКАЖИТЕ УНИКАЛЬНЫЙ ИДЕНТИФИКАТОР ПРОГРАММЫ', facilityRecoveryProgramID);
  }
  if (!name) return setFacilityFeedback(facilityRecoveryProgramError, 'УКАЖИТЕ НАЗВАНИЕ ПРОГРАММЫ', facilityRecoveryProgramName);
  if (!facilityRecoveryProgramDevice.value || !facilityRecoveryProgramTransition.value) {
    return setFacilityFeedback(facilityRecoveryProgramError, 'ВЫБЕРИТЕ УСТРОЙСТВО И ПЕРЕХОД', facilityRecoveryProgramDevice);
  }
  const extra = document.getElementById('facilityProgramExtraTransitions');
  const invalid = validateFacilityActionRows(extra);
  if (invalid) return setFacilityFeedback(facilityRecoveryProgramError, ...invalid);
  const original = authoredFacility().recoveryPrograms.find(value => value.id === editingFacilityProgramID);
  const program = {
    ...structuredClone(original || {}),
    id, name, transitions: [{
      ...original?.transitions?.[0],
      deviceId: facilityRecoveryProgramDevice.value,
      transitionId: facilityRecoveryProgramTransition.value,
    }, ...readFacilityActionRows(extra)],
  };
  const originalID = original?.id || '';
  const editingID = editingFacilityProgramID;
  hideFacilityDialog(facilityRecoveryProgramDialog, btnAddFacilityRecoveryProgram);
  commitFacilityEdit(`Программа «${program.name}»`, session => {
    const facility = ensureFacility(session);
    const index = originalID ? facility.recoveryPrograms.findIndex(candidate => candidate.id === originalID) : -1;
    if (index >= 0) facility.recoveryPrograms.splice(index, 1, structuredClone(program));
    else facility.recoveryPrograms.push(structuredClone(program));
  }, reason => {
    editingFacilityProgramID = editingID;
    showFacilityDialog(facilityRecoveryProgramDialog);
    setFacilityFeedback(facilityRecoveryProgramError, reason);
  });
});
facilityRecoveryProgramName.addEventListener('input', () => {
  if (facilityRecoveryProgramID.dataset.manual === 'true') return;
  facilityRecoveryProgramID.value = suggestFacilityID('program', facilityRecoveryProgramName.value,
    new Set((authoredFacility()?.recoveryPrograms || []).map(program => program.id)));
});
facilityRecoveryProgramID.addEventListener('input', () => { facilityRecoveryProgramID.dataset.manual = 'true'; });

btnAddFacilityBinding.addEventListener('click', openFacilityBindingDialog);
facilityBindingTerminal.addEventListener('change', updateFacilityBindingNodes);
facilityBindingTerminal.addEventListener('change', updateFacilityBindingCommandTexts);
facilityBindingDevice.addEventListener('change', updateFacilityBindingStates);
btnCancelFacilityBinding.addEventListener('click', () => hideFacilityDialog(facilityBindingDialog, btnAddFacilityBinding));
facilityBindingDialog.addEventListener('cancel', event => {
  event.preventDefault();
  hideFacilityDialog(facilityBindingDialog, btnAddFacilityBinding);
});
facilityBindingForm.addEventListener('submit', event => {
  event.preventDefault();
  const terminal = state.session.terminals.find(candidate => candidate.id === facilityBindingTerminal.value);
  const located = terminal ? locateNode(terminal.root, facilityBindingNode.value) : null;
  const kind = facilityBindingKind.value;
  const text = facilityBindingText.value.trim();
  if (!located) {
    return setFacilityFeedback(facilityBindingError, 'ВЫБЕРИТЕ ТЕРМИНАЛ И ЭЛЕМЕНТ', !terminal ? facilityBindingTerminal : facilityBindingNode);
  }
  if (kind === 'command-action' || kind === 'availability') {
    if (located.node.type !== 'command' || (kind === 'command-action' && located.node.terminalTransition)) {
      return setFacilityFeedback(facilityBindingError, 'ВЫБЕРИТЕ КОМАНДУ БЕЗ ПЕРЕХОДА В ДРУГОЙ ТЕРМИНАЛ', facilityBindingNode);
    }
  }
  const variant = {
    ...structuredClone(editingFacilityBinding?.variant || {}),
    when: { ...editingFacilityBinding?.variant.when, deviceId: facilityBindingDevice.value, stateId: facilityBindingState.value }, text,
  };
  const firstBlock = located.node.blocks?.find(block => block.id === facilityBindingBlock.value);
  let apply;
  if (kind === 'command-action') {
    const programID = facilityBindingProgram.value;
    if (programID && !authoredFacility().recoveryPrograms.some(program => program.id === programID)) {
      return setFacilityFeedback(facilityBindingError, 'ВЫБЕРИТЕ СУЩЕСТВУЮЩУЮ ПРОГРАММУ', facilityBindingProgram);
    }
    const invalid = programID ? null : validateFacilityActionRows(facilityBindingActionRequests);
    if (invalid) return setFacilityFeedback(facilityBindingError, ...invalid);
    const requests = readFacilityActionRows(facilityBindingActionRequests);
    if (!programID && !requests.length) {
      return setFacilityFeedback(facilityBindingError,
        'ДЕЙСТВИЕ ТРЕБУЕТ КОМАНДУ И ХОТЯ БЫ ОДИН ПЕРЕХОД', facilityBindingNode);
    }
    const completedName = facilityBindingCompletedName.value.trim();
    const confirmationText = facilityBindingConfirmationText.value.trim();
    if (!located.node.stateChange && (!completedName || !confirmationText)) {
      return setFacilityFeedback(facilityBindingError, 'ЗАПОЛНИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ И ЗАПРОС ПОДТВЕРЖДЕНИЯ',
        completedName ? facilityBindingConfirmationText : facilityBindingCompletedName);
    }
    apply = node => {
      node.stateChange ||= { completedName, confirmationText };
      node.stateChange.facilityAction = programID
        ? { recoveryProgramId: programID } : { transitions: { transitions: structuredClone(requests) } };
    };
  } else if (!facilityBindingDevice.value || !facilityBindingState.value
      || (['menu-name', 'entry-content'].includes(kind) && !text)) {
    return setFacilityFeedback(facilityBindingError, 'ЗАПОЛНИТЕ УСТРОЙСТВО, СОСТОЯНИЕ И ТЕКСТ ДЛЯ ВЫБРАННОГО ТИПА',
      !facilityBindingDevice.value ? facilityBindingDevice : (!facilityBindingState.value ? facilityBindingState : facilityBindingText));
  } else {
    if (kind === 'entry-content' && (located.node.type !== 'entry' || !firstBlock)) {
      return setFacilityFeedback(facilityBindingError, 'ВЫБРАННАЯ ЗАПИСЬ НЕ СОДЕРЖИТ БЛОКОВ', facilityBindingNode);
    }
    const blockID = firstBlock?.id || '';
    apply = node => {
      const equality = structuredClone(variant.when);
      if (kind === 'entry-content') {
        const block = node.blocks?.find(candidate => candidate.id === blockID);
        if (block) (block.facilityTextVariants ||= []).push(structuredClone(variant));
      } else if (kind === 'visibility') {
        node.visibleWhen = equality;
      } else if (kind === 'availability') {
        node.availableWhen = equality;
      } else {
        (node.facilityNameVariants ||= []).push(structuredClone(variant));
      }
    };
  }
  const terminalID = terminal.id;
  const nodeID = located.node.id;
  const editingKey = editingFacilityBinding ? facilityBindingKey(editingFacilityBinding) : null;
  hideFacilityDialog(facilityBindingDialog, btnAddFacilityBinding);
  commitFacilityEdit(`Правило «${located.node.name}»`, session => {
    if (editingKey) removeFacilityBinding(session, editingKey);
    const owner = session.terminals.find(candidate => candidate.id === terminalID);
    const node = owner ? locateNode(owner.root, nodeID)?.node : null;
    if (node) apply(node);
  }, reason => {
    showFacilityDialog(facilityBindingDialog);
    setFacilityFeedback(facilityBindingError, reason);
  });
  renderNodeForm();
});

btnSaveFacilityDeviceChanges.addEventListener('click', () => {
  const selected = facilityDeviceByID(selectedFacilityDeviceID);
  const name = facilitySelectedDeviceName.value.trim();
  if (!selected || !name) {
    setFacilityFeedback(facilitySelectionError, 'НАЗВАНИЕ УСТРОЙСТВА НЕ ДОЛЖНО БЫТЬ ПУСТЫМ', facilitySelectedDeviceName);
    return;
  }
  const deviceID = selected.id;
  setFacilityFeedback(facilitySelectionError);
  commitFacilityEdit(`Устройство «${name}»`, session => {
    const device = ensureFacility(session).devices.find(candidate => candidate.id === deviceID);
    if (device) device.name = name;
  }, takeUnappliedInput(facilitySelectionEditor));
});
btnCancelFacilityDeviceChanges.addEventListener('click', () => {
  takeUnappliedInput(facilitySelectionEditor);
  renderFacilityWorkspace();
  renderSaveStatus();
});
btnInspectFacilityDependencies.addEventListener('click', () => { void inspectSelectedFacilityDevice(); });
btnCloseFacilityDependencies.addEventListener('click', () => hideFacilityDialog(facilityDependencyDialog, btnInspectFacilityDependencies));
facilityDependencyDialog.addEventListener('cancel', event => {
  event.preventDefault();
  hideFacilityDialog(facilityDependencyDialog, btnInspectFacilityDependencies);
});
btnDeleteFacilityDevice.addEventListener('click', () => { void openFacilityRepair(); });
btnPreviewFacilityDevice.addEventListener('click', () => {
  if (!controlBlocked(btnPreviewFacilityDevice)) openFacilityPreview('device', btnPreviewFacilityDevice);
});
btnPreviewFacilityCondition.addEventListener('click', () => {
  if (!controlBlocked(btnPreviewFacilityCondition)) openFacilityPreview('condition', btnPreviewFacilityCondition);
});
btnRefreshFacilityPreview.addEventListener('click', () => { void refreshFacilityPreview(); });
facilityPreviewTerminal.addEventListener('change', () => { void refreshFacilityPreview(); });
facilityPreviewState.addEventListener('change', () => { void refreshFacilityPreview(); });
facilityPreviewActive.addEventListener('change', () => { void refreshFacilityPreview(); });
btnCloseFacilityPreview.addEventListener('click', closeFacilityPreview);
facilityPreviewDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityPreview();
});
btnResetFacilityDevice.addEventListener('click', () => {
  if (controlBlocked(btnResetFacilityDevice)) return;
  openFacilityConfirmation(facilityDeviceResetDialog, btnResetFacilityDevice, btnCancelFacilityDeviceReset);
});
btnCancelFacilityDeviceReset.addEventListener('click', () => closeFacilityConfirmation(facilityDeviceResetDialog));
btnConfirmFacilityDeviceReset.addEventListener('click', () => { void resetSelectedFacilityDevice(); });
facilityDeviceResetDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityConfirmation(facilityDeviceResetDialog);
});
btnResetScene.addEventListener('click', openSceneResetDialog);
btnCancelSceneReset.addEventListener('click', closeSceneResetDialog);
btnConfirmSceneReset.addEventListener('click', () => { void confirmSceneReset(); });
sceneResetDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeSceneResetDialog();
});
btnRecoverFacilityCondition.addEventListener('click', () => {
  if (controlBlocked(btnRecoverFacilityCondition)) return;
  openFacilityConfirmation(facilityRecoveryConfirmationDialog, btnRecoverFacilityCondition, btnCancelFacilityRecovery);
});
btnCancelFacilityRecovery.addEventListener('click', () => closeFacilityConfirmation(facilityRecoveryConfirmationDialog));
btnConfirmFacilityRecovery.addEventListener('click', () => { void recoverSelectedFacilityCondition(); });
facilityRecoveryConfirmationDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityConfirmation(facilityRecoveryConfirmationDialog);
});
btnCancelFacilityRepair.addEventListener('click', () => {
  pendingFacilityRepair = null;
  setFacilityFeedback(facilityRepairError);
  hideFacilityDialog(facilityRepairDialog, btnDeleteFacilityDevice);
});
facilityRepairDialog.addEventListener('cancel', event => {
  event.preventDefault();
  pendingFacilityRepair = null;
  setFacilityFeedback(facilityRepairError);
  hideFacilityDialog(facilityRepairDialog, btnDeleteFacilityDevice);
});
facilityRepairTarget.addEventListener('change', () => {
  pendingFacilityRepair && (pendingFacilityRepair.replacementID = '');
  btnApplyFacilityRepair.disabled = true;
  btnApplyFacilityRepair.hidden = true;
  facilityRepairImpact.textContent = '';
});
btnValidateFacilityRepair.addEventListener('click', validateFacilityRepair);
btnApplyFacilityRepair.addEventListener('click', () => {
  if (pendingFacilityRepair?.replacementID) applyFacilityDeletion(pendingFacilityRepair.replacementID);
});
btnDeleteFacilityWithoutRepair.addEventListener('click', () => {
  if (!pendingFacilityRepair || pendingFacilityRepair.dependencies.length) return;
  applyFacilityDeletion('');
});

// ── Render: everything ──────────────────────────────────────
function renderAll() {
  renderTermList();
  renderTreeHeader();
  renderSettingsPanel();
  renderToolbarState();
  renderTree();
  renderNodeForm();
  renderToolbarHint();
  renderHackStatus();
  renderCoordination();
  if (!facilityWorkspace.hidden) renderFacilityWorkspace();
  renderSaveStatus();
}

// ── Render: authoritative roster and broadcast state ────────
function renderCoordination() {
  const coordination = state.coordination;
  const roster = Array.isArray(coordination?.roster) ? coordination.roster : [];
  const sessions = Array.isArray(coordination?.sessions) ? coordination.sessions : [];
  const broadcast = coordination?.broadcast || null;
  const playerConfig = coordination?.playerConfig || null;
  const availableCharacters = roster.filter(character => !character.claimedBySessionId);
  const unassignedSessions = sessions.filter(session => !session.character);

  broadcastSummary.textContent = broadcast
    ? (broadcast.activeTerminalId
      ? `ТРАНСЛЯЦИЯ АКТИВНА · ТЕРМИНАЛ ${broadcast.activeTerminalId}`
      : `ТРАНСЛЯЦИЯ АКТИВНА · ОЖИДАНИЕ ТЕРМИНАЛА · ${broadcast.id}`)
    : 'ТРАНСЛЯЦИЯ НЕ ЗАПУЩЕНА';
  broadcastSummary.classList.toggle('is-live', Boolean(broadcast));
  coordinationPanel.dataset.playerConfigActive = String(Boolean(playerConfig));
  playerConfigStatus.dataset.active = String(Boolean(playerConfig));
  playerConfigStatus.textContent = playerConfig
    ? `${playerConfig.name} · ${playerConfig.filePath}`
    : 'НЕ ВЫБРАНА · СОЗДАЙТЕ ИЛИ ВЫБЕРИТЕ ФАЙЛ';
  btnOpenPlayerConfig.disabled = coordinationCommandPending || Boolean(broadcast);
  btnNewPlayerConfig.disabled = coordinationCommandPending || Boolean(broadcast);
  btnManagePlayers.disabled = !playerConfig;
  btnStartBroadcast.disabled = coordinationCommandPending || Boolean(broadcast) || !playerConfig;
  btnEndBroadcast.hidden = !broadcast;
  btnEndBroadcast.disabled = coordinationCommandPending || !broadcast;
  activeLogicalSessionCount.textContent = String(sessions.filter(session => Boolean(session.connected)).length);

  renderPlayerManagementRoster(roster, Boolean(broadcast), Boolean(playerConfig));

  logicalSessionList.replaceChildren();
  if (!sessions.length) {
    const empty = document.createElement('div');
    empty.className = 'session-empty';
    empty.setAttribute('role', 'listitem');
    empty.textContent = 'СЕССИИ НЕ ПОДКЛЮЧЕНЫ';
    logicalSessionList.appendChild(empty);
    return;
  }

  for (const session of sessions) {
    const fragment = logicalSessionRowTemplate.content.cloneNode(true);
    const row = fragment.querySelector('.session-row');
    const assigned = Boolean(session.character);
    const role = session.role || 'unassigned';
    row.dataset.sessionId = session.id;
    row.dataset.connected = String(Boolean(session.connected));
    row.dataset.role = role;
    row.querySelector('.session-primary-name').textContent = assigned
      ? session.character.name
      : session.fallbackName;
    const presence = row.querySelector('.session-presence');
    presence.dataset.presence = session.connected ? 'connected' : 'disconnected';
    presence.textContent = session.connected ? 'ПОДКЛЮЧЕН' : 'ОТКЛЮЧЕН';
    const roleLabel = row.querySelector('.session-role');
    roleLabel.dataset.sessionRole = role;
    roleLabel.textContent = role === 'active'
      ? (session.connected ? 'УПРАВЛЯЮЩИЙ' : 'УПРАВЛЯЮЩИЙ · НЕТ СВЯЗИ')
      : role === 'observer' ? 'НАБЛЮДАТЕЛЬ' : 'БЕЗ РОЛИ';
    row.querySelector('.session-character-name').textContent = assigned
      ? `ПЕРСОНАЖ: ${session.character.name}`
      : 'ПЕРСОНАЖ НЕ НАЗНАЧЕН';
    row.querySelector('.session-fallback-label').textContent = `СЕССИЯ: ${session.fallbackName}`;

    const nameInput = row.querySelector('.session-name-input');
    nameInput.value = session.fallbackName || '';
    const renameButton = row.querySelector('.session-rename');
    const assignmentControls = row.querySelector('.session-assignment-controls');
    const characterSelect = row.querySelector('.session-character-select');
    const assignButton = row.querySelector('.session-assign');
    const claimedControls = row.querySelector('.session-claimed-controls');
    const releaseButton = row.querySelector('.session-release');
    const controllerButton = row.querySelector('.session-controller');
    const moveSelect = row.querySelector('.session-move-session-select');
    const moveButton = row.querySelector('.session-move');
    for (const control of row.querySelectorAll('input, select, button')) {
      control.disabled = coordinationCommandPending;
    }
    renameButton.addEventListener('click', () => {
      const currentSession = findLogicalSession(session.id);
      if (coordinationCommandPending || !currentSession) return;
      const fallbackName = nameInput.value.trim();
      if (!fallbackName) return setCoordinationStatus('УКАЖИТЕ МЕТКУ СЕССИИ', true);
      runCoordinationCommand(
        () => desktopAPI.renameLogicalSession({ sessionId: session.id, fallbackName }),
        'МЕТКА СЕССИИ ОБНОВЛЕНА',
        'ПЕРЕИМЕНОВАНИЕ СЕССИИ...'
      );
    });

    assignmentControls.hidden = assigned || !broadcast;
    fillSelect(characterSelect, availableCharacters, character => character.id, character => character.name, 'НЕТ ДОСТУПНЫХ ПЕРСОНАЖЕЙ');
    assignButton.disabled = coordinationCommandPending || assigned || !broadcast || !characterSelect.value;
    assignButton.addEventListener('click', () => {
      const currentSession = findLogicalSession(session.id);
      const currentCharacter = findRosterCharacter(characterSelect.value);
      if (coordinationCommandPending || !state.coordination?.broadcast || currentSession?.character ||
          !currentCharacter || currentCharacter.claimedBySessionId) return;
      runCoordinationCommand(
        () => desktopAPI.assignCharacter({ sessionId: session.id, characterId: characterSelect.value }),
        'ПЕРСОНАЖ НАЗНАЧЕН',
        'НАЗНАЧЕНИЕ ПЕРСОНАЖА...'
      );
    });

    claimedControls.hidden = !assigned;
    releaseButton.disabled = coordinationCommandPending || !assigned;
    releaseButton.addEventListener('click', () => {
      if (coordinationCommandPending || !findLogicalSession(session.id)?.character) return;
      runCoordinationCommand(
        () => desktopAPI.releaseCharacter(session.id),
        'ПЕРСОНАЖ ОСВОБОЖДЁН',
        'ОСВОБОЖДЕНИЕ ПЕРСОНАЖА...'
      );
    });
    controllerButton.disabled = true;
    controllerButton.hidden = role === 'active';
    if (assigned && session.connected && role !== 'active') {
      controllerButton.disabled = coordinationCommandPending;
      controllerButton.addEventListener('click', () => {
        const currentSession = findLogicalSession(session.id);
        if (coordinationCommandPending || !currentSession?.character || !currentSession.connected ||
            currentSession.role === 'active') return;
        runCoordinationCommand(
          () => desktopAPI.setActiveController(session.id),
          'УПРАВЛЕНИЕ ПЕРЕДАНО',
          'ПЕРЕДАЧА УПРАВЛЕНИЯ...'
        );
      });
    }

    fillSelect(moveSelect, unassignedSessions, candidate => candidate.id, candidate => sessionLabel(candidate), 'НЕТ СВОБОДНЫХ СЕССИЙ');
    moveSelect.disabled = coordinationCommandPending || !assigned || !playerConfig || !moveSelect.value;
    moveButton.disabled = coordinationCommandPending || !assigned || !playerConfig || !moveSelect.value;
    moveButton.addEventListener('click', () => {
      const currentSession = findLogicalSession(session.id);
      const destination = findLogicalSession(moveSelect.value);
      if (coordinationCommandPending || !state.coordination?.playerConfig ||
          !currentSession?.character || destination?.character || !destination) return;
      runCoordinationCommand(
        () => desktopAPI.moveCharacter({
          characterId: currentSession.character.id,
          toSessionId: destination.id,
        }),
        'НАЗНАЧЕНИЕ ПЕРЕМЕЩЕНО',
        'ПЕРЕМЕЩЕНИЕ НАЗНАЧЕНИЯ...'
      );
    });
    logicalSessionList.appendChild(fragment);
  }
}

function findLogicalSession(sessionId) {
  const sessions = Array.isArray(state.coordination?.sessions) ? state.coordination.sessions : [];
  return sessions.find(session => session.id === sessionId) || null;
}

function findRosterCharacter(characterId) {
  const roster = Array.isArray(state.coordination?.roster) ? state.coordination.roster : [];
  return roster.find(character => character.id === characterId) || null;
}

function renderPlayerManagementRoster(roster, broadcastActive, playerConfigActive) {
  const readOnly = broadcastActive || !playerConfigActive;
  playerManagementDialog.setAttribute('aria-readonly', String(readOnly));
  playerManagementMode.textContent = broadcastActive
    ? 'ТРАНСЛЯЦИЯ АКТИВНА · ПРОСМОТР БЕЗ РЕДАКТИРОВАНИЯ'
    : playerConfigActive ? 'РЕДАКТИРОВАНИЕ ДОСТУПНО' : 'КОНФИГУРАЦИЯ ИГРОКОВ НЕ ВЫБРАНА';

  const addDisabled = coordinationCommandPending || readOnly;
  for (const control of playerManagementAddForm.elements) control.disabled = addDisabled;
  syncPlayerDeleteDialogControls();
  playerManagementRoster.replaceChildren();
  playerManagementEmpty.hidden = roster.length > 0;

  for (const character of roster) {
    const fragment = playerManagementRowTemplate.content.cloneNode(true);
    const row = fragment.querySelector('.player-management-row');
    const nameInput = row.querySelector('.player-name-input');
    const intelligenceInput = row.querySelector('.player-intelligence-input');
    const hackerInput = row.querySelector('.player-hacker-perk-availability');
    row.dataset.characterId = character.id;
    nameInput.value = character.name || '';
    intelligenceInput.value = String(Number.isInteger(character.intelligence) ? character.intelligence : 1);
    hackerInput.value = character.hackerPerkAvailable === true ? 'true' : 'false';
    const saveButton = row.querySelector('.player-save');
    const deleteButton = row.querySelector('.player-delete');
    for (const control of row.querySelectorAll('input, select, button')) {
      control.disabled = coordinationCommandPending || readOnly;
    }
    saveButton.addEventListener('click', async () => {
      if (!playerMutationAllowed()) return;
      const name = nameInput.value.trim();
      const intelligence = intelligenceInput.valueAsNumber;
      const hackerChoice = hackerInput.value;
      if (!name) {
        setPlayerManagementFeedback('УКАЖИТЕ ИМЯ ИГРОКА', true);
        nameInput.focus();
        return;
      }
      if (!Number.isInteger(intelligence) || intelligence < 1 || intelligence > 10) {
        setPlayerManagementFeedback('ИНТЕЛЛЕКТ ДОЛЖЕН БЫТЬ ЦЕЛЫМ ЧИСЛОМ ОТ 1 ДО 10', true);
        intelligenceInput.focus();
        return;
      }
      if (hackerChoice !== 'true' && hackerChoice !== 'false') {
        setPlayerManagementFeedback('ВЫБЕРИТЕ ДОСТУПНОСТЬ ПЕРКА «ХАКЕР»', true);
        hackerInput.focus();
        return;
      }
      await runPlayerManagementMutation(
        () => desktopAPI.updateCharacter({
          characterId: character.id,
          name,
          intelligence,
          hackerPerkAvailable: hackerChoice === 'true',
          expectedRevision: coordinationRevision(state.coordination),
        }),
        'ПРОФИЛЬ ИГРОКА СОХРАНЁН',
        'СОХРАНЕНИЕ ПРОФИЛЯ ИГРОКА...'
      );
    });
    deleteButton.addEventListener('click', () => {
      if (!playerMutationAllowed()) return;
      showPlayerDeleteConfirmation(character.id, character.name || '', deleteButton);
    });
    playerManagementRoster.appendChild(fragment);
  }
}

function playerMutationAllowed() {
  return !coordinationCommandPending &&
    !state.coordination?.broadcast &&
    Boolean(state.coordination?.playerConfig);
}

function syncPlayerDeleteDialogControls() {
  if (!pendingPlayerDelete) return;
  const active = Boolean(state.coordination?.broadcast);
  const configMissing = !state.coordination?.playerConfig;
  btnConfirmPlayerDelete.disabled = coordinationCommandPending || active || configMissing;
  btnCancelPlayerDelete.disabled = coordinationCommandPending;
}

function showPlayerDeleteConfirmation(characterId, name, opener) {
  if (!playerMutationAllowed()) return;
  pendingPlayerDelete = { characterId, opener };
  playerDeleteDialogDescription.textContent = `Удалить игрока «${name}»? Это действие нельзя отменить.`;
  playerDeleteDialog.hidden = false;
  syncPlayerDeleteDialogControls();
  if (!playerDeleteDialog.open) playerDeleteDialog.showModal();
  btnCancelPlayerDelete.focus();
}

function hidePlayerDeleteConfirmation(restoreFocus = true) {
  const pending = pendingPlayerDelete;
  pendingPlayerDelete = null;
  if (playerDeleteDialog.open) playerDeleteDialog.close();
  playerDeleteDialog.hidden = true;
  btnConfirmPlayerDelete.disabled = false;
  btnCancelPlayerDelete.disabled = false;
  if (!restoreFocus) return;
  const currentDelete = pending?.characterId
    ? playerManagementRoster.querySelector(`[data-character-id="${CSS.escape(pending.characterId)}"] .player-delete`)
    : null;
  if (currentDelete instanceof HTMLElement) currentDelete.focus();
  else if (pending?.opener?.isConnected) pending.opener.focus();
}

async function runPlayerManagementMutation(command, successMessage, pendingMessage) {
  if (!playerMutationAllowed()) return null;
  coordinationCommandPending = true;
  setPlayerManagementFeedback(pendingMessage);
  setCoordinationStatus(pendingMessage);
  renderCoordination();

  let result;
  try {
    result = await command();
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  if (result?.state) applyCoordinationState(result.state);
  renderCoordination();

  if (!result?.ok) {
    const message = result?.error || 'ОПЕРАЦИЯ СО СПИСКОМ ИГРОКОВ ОТКЛОНЕНА';
    setPlayerManagementFeedback(message, true);
    setCoordinationStatus(message, true);
    return result;
  }

  setPlayerManagementFeedback(successMessage);
  setCoordinationStatus(successMessage);
  return result;
}

function setPlayerManagementFeedback(message = '', isError = false) {
  playerManagementStatus.textContent = isError ? '' : message;
  playerManagementError.textContent = isError ? message : '';
  playerManagementError.hidden = !isError || !message;
}

function showLogicalSessionManagement() {
  logicalSessionDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnManageLogicalSessions;
  setLogicalSessionDialogFeedback('');
  logicalSessionDialog.hidden = false;
  if (typeof logicalSessionDialog.showModal === 'function' && !logicalSessionDialog.open) {
    logicalSessionDialog.showModal();
  } else {
    logicalSessionDialog.setAttribute('open', '');
  }
  queueMicrotask(() => btnCloseLogicalSessions.focus());
}

function hideLogicalSessionManagement() {
  if (typeof logicalSessionDialog.close === 'function' && logicalSessionDialog.open) {
    logicalSessionDialog.close();
  } else {
    logicalSessionDialog.removeAttribute('open');
  }
  logicalSessionDialog.hidden = true;
  const opener = logicalSessionDialogOpener;
  logicalSessionDialogOpener = null;
  if (opener?.isConnected) opener.focus();
}

function showPlayerManagement() {
  if (!state.coordination?.playerConfig || btnManagePlayers.disabled) return;
  playerManagementOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnManagePlayers;
  setPlayerManagementFeedback('');
  playerManagementDialog.hidden = false;
  if (!playerManagementDialog.open) playerManagementDialog.showModal();
  queueMicrotask(() => btnClosePlayerManagement.focus());
}

function hidePlayerManagement() {
  if (pendingPlayerDelete) hidePlayerDeleteConfirmation(false);
  if (playerManagementDialog.open) playerManagementDialog.close();
  playerManagementDialog.hidden = true;
  const opener = playerManagementOpener;
  playerManagementOpener = null;
  if (opener?.isConnected) opener.focus();
}

async function addPlayerProfile({ name, intelligence, hackerPerkAvailable }) {
  if (coordinationCommandPending || state.coordination?.broadcast || !state.coordination?.playerConfig) return null;
  coordinationCommandPending = true;
  setPlayerManagementFeedback('ДОБАВЛЕНИЕ ИГРОКА...');
  setCoordinationStatus('ДОБАВЛЕНИЕ ПЕРСОНАЖА...');
  renderCoordination();

  let result;
  try {
    result = await desktopAPI.addCharacter({
      name,
      intelligence,
      hackerPerkAvailable,
      expectedRevision: coordinationRevision(state.coordination),
    });
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  applyCoordinationState(result?.state || state.coordination);

  if (!result?.ok) {
    const message = result?.error || 'НЕ УДАЛОСЬ ДОБАВИТЬ ИГРОКА';
    setPlayerManagementFeedback(message, true);
    setCoordinationStatus(message, true);
    renderCoordination();
    return result;
  }

  playerManagementAddForm.reset();
  setPlayerManagementFeedback('ИГРОК ДОБАВЛЕН');
  setCoordinationStatus('ПЕРСОНАЖ ДОБАВЛЕН');
  renderCoordination();
  return result;
}

function setPlayerConfigError(message = '') {
  playerConfigError.textContent = message;
  playerConfigError.hidden = !message;
}

async function runPlayerConfigCommand(command, successMessage) {
  if (coordinationCommandPending || state.coordination?.broadcast) return null;
  coordinationCommandPending = true;
  setPlayerConfigError('');
  setCoordinationStatus('ЗАГРУЗКА КОНФИГУРАЦИИ ИГРОКОВ...');
  renderCoordination();
  let result;
  try {
    result = await command();
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  if (result?.canceled) {
    setCoordinationStatus('ВЫБОР КОНФИГУРАЦИИ ОТМЕНЁН');
    setPlayerConfigError('');
    renderCoordination();
    return result;
  }
  if (!result?.ok) {
    const message = result?.error || 'НЕ УДАЛОСЬ ЗАГРУЗИТЬ КОНФИГУРАЦИЮ ИГРОКОВ';
    setCoordinationStatus(message, true);
    setPlayerConfigError(message);
    renderCoordination();
    return result;
  }
  if (result.session) applyAuthoredSession(result.session, result.authoringRevision);
  applyCoordinationState(result.state || state.coordination);
  setCoordinationStatus(successMessage);
  setPlayerConfigError('');
  renderAll();
  return result;
}

btnOpenPlayerConfig.addEventListener('click', () => runPlayerConfigCommand(
  () => desktopAPI.openPlayerConfig(),
  'КОНФИГУРАЦИЯ ИГРОКОВ ВЫБРАНА'
));

btnNewPlayerConfig.addEventListener('click', () => runPlayerConfigCommand(
  () => desktopAPI.newPlayerConfig(),
  'КОНФИГУРАЦИЯ ИГРОКОВ СОЗДАНА'
));

function coordinationRevision(coordination) {
  const revision = Number(coordination?.revision || 0);
  return Number.isSafeInteger(revision) && revision >= 0 ? revision : 0;
}

function facilityImpactIDs(values) {
  if (!Array.isArray(values)) return [];
  return values.filter(value => typeof value === 'string' && value.trim()).map(value => value.trim());
}

function renderCommandExecutionFacilityImpact(summary) {
  commandExecutionFacilityImpact.replaceChildren();
  commandExecutionFacilityImpact.hidden = !summary || typeof summary !== 'object';
  if (commandExecutionFacilityImpact.hidden) return;

  const revision = Number(summary.expectedFacilityRevision);
  const rows = [
    ['ОЖИДАЕМАЯ РЕВИЗИЯ ОБЪЕКТА', Number.isSafeInteger(revision) && revision >= 0 ? String(revision) : '—'],
    ['УСТРОЙСТВА', facilityImpactIDs(summary.deviceIds).join(' · ') || '—'],
    ['ДИАГНОСТИЧЕСКИЕ УСЛОВИЯ', facilityImpactIDs(summary.conditionIds).join(' · ') || '—'],
  ];
  if (typeof summary.recoveryProgramId === 'string' && summary.recoveryProgramId.trim()) {
    rows.push(['ПРОГРАММА ВОССТАНОВЛЕНИЯ', summary.recoveryProgramId.trim()]);
  }
  for (const [label, value] of rows) {
    const row = document.createElement('div');
    row.textContent = `${label}: ${value}`;
    commandExecutionFacilityImpact.appendChild(row);
  }
}

const facilityFailureMessages = Object.freeze({
  'missing-reference': 'ССЫЛКА НА ОБЪЕКТ БОЛЬШЕ НЕ ДЕЙСТВИТЕЛЬНА',
  'invalid-transition': 'ПЕРЕХОД ОБЪЕКТА БОЛЬШЕ НЕ ДОПУСТИМ',
  'precondition-failed': 'УСЛОВИЯ ПЕРЕХОДА ОБЪЕКТА НЕ ВЫПОЛНЕНЫ',
  'stale-revision': 'СОСТОЯНИЕ ОБЪЕКТА ИЗМЕНИЛОСЬ. ПОВТОРИТЕ ЗАПРОС',
  conflict: 'КОМАНДА ИЛИ ЕЁ ДЕЙСТВИЕ ИЗМЕНИЛИСЬ',
  duplicate: 'ЗАПРОС УЖЕ ОБРАБОТАН',
  'invalid-configuration': 'ДЕЙСТВИЕ ОБЪЕКТА НАСТРОЕНО НЕКОРРЕКТНО',
  'persistence-failed': 'НЕ УДАЛОСЬ СОХРАНИТЬ СОСТОЯНИЕ ОБЪЕКТА',
  'runtime-context-ended': 'СЕАНС ЗАВЕРШИЛСЯ ДО СОХРАНЕНИЯ СОСТОЯНИЯ ОБЪЕКТА',
});

function facilityFailureMessage(result) {
  const failure = result?.facilityResult?.failure;
  return typeof failure === 'string' ? (facilityFailureMessages[failure] || '') : '';
}

function rememberResolvedCommandExecution(requestID) {
  resolvedCommandExecutionRequestIDs.add(requestID);
  if (resolvedCommandExecutionRequestIDs.size <= 128) return;
  const oldest = resolvedCommandExecutionRequestIDs.values().next().value;
  resolvedCommandExecutionRequestIDs.delete(oldest);
}

function hideCommandExecutionDialog() {
  // Any authoritative close invalidates the promise currently resolving this
  // dialog. Its eventual callback must not overwrite a newer lifecycle state
  // or dismiss a different request shown in the meantime.
  commandExecutionDialogEpoch += 1;
  commandExecutionDecisionRequestID = null;
  commandExecutionDialogRequestID = null;
  commandExecutionDialogMode = null;
  commandExecutionDialog.hidden = true;
  if (typeof commandExecutionDialog.close === 'function' && commandExecutionDialog.open) {
    commandExecutionDialog.close();
  } else {
    commandExecutionDialog.removeAttribute('open');
  }
  btnApproveCommandExecution.disabled = false;
  btnRejectCommandExecution.disabled = false;
  renderCommandExecutionFacilityImpact(null);
  commandExecutionDialogStatus.textContent = '';
  commandExecutionDialogError.textContent = '';
  commandExecutionDialogError.hidden = true;
}

function showCommandExecutionDialog(pending) {
  commandExecutionDialogEpoch += 1;
  commandExecutionDecisionRequestID = null;
  commandExecutionDialogRequestID = pending.requestId;
  commandExecutionDialogMode = pending.mode || null;
  commandExecutionDialogDescription.textContent = pending.confirmationText;
  renderCommandExecutionFacilityImpact(pending.facilityAction);
  const modeLabels = {
    ordinary: 'ОБЫЧНАЯ',
    'state-change': 'ИЗМЕНЕНИЕ СОСТОЯНИЯ',
    'completed-state-change': 'ЗАВЕРШЁННОЕ ИЗМЕНЕНИЕ СОСТОЯНИЯ',
  };
  const mode = modeLabels[pending.mode] || pending.mode || 'НЕИЗВЕСТЕН';
  const commandName = pending.commandName || pending.commandId || '—';
  commandExecutionDialogStatus.textContent = `ЗАПРОС: ${pending.requestId} · РЕЖИМ: ${mode} · КОМАНДА: ${commandName}`;
  commandExecutionDialogError.textContent = '';
  commandExecutionDialogError.hidden = true;
  btnApproveCommandExecution.disabled = false;
  btnRejectCommandExecution.disabled = false;
  commandExecutionDialog.hidden = false;
  if (typeof commandExecutionDialog.showModal === 'function' && !commandExecutionDialog.open) {
    commandExecutionDialog.showModal();
  } else {
    commandExecutionDialog.setAttribute('open', '');
  }
  btnApproveCommandExecution.focus();
}

function syncCommandExecutionDialog(coordination) {
  const pending = coordination?.pendingCommandExecution;
  const requestID = typeof pending?.requestId === 'string' ? pending.requestId : '';
  if (!requestID) {
    if (commandExecutionDialogRequestID) hideCommandExecutionDialog();
    return;
  }
  if (requestID === commandExecutionDecisionRequestID || resolvedCommandExecutionRequestIDs.has(requestID)) {
    return;
  }
  if (requestID === commandExecutionDialogRequestID) return;
  if (commandExecutionDialogRequestID) hideCommandExecutionDialog();
  showCommandExecutionDialog(pending);
}

function rememberResolvedTerminalNavigation(requestID) {
  resolvedTerminalNavigationRequestIDs.add(requestID);
  if (resolvedTerminalNavigationRequestIDs.size > 128) {
    resolvedTerminalNavigationRequestIDs.delete(resolvedTerminalNavigationRequestIDs.values().next().value);
  }
}

function hideTerminalNavigationDialog() {
  terminalNavigationDialogEpoch += 1;
  terminalNavigationDecisionRequestID = null;
  terminalNavigationDialogRequestID = null;
  terminalNavigationDialog.hidden = true;
  if (terminalNavigationDialog.open) terminalNavigationDialog.close();
  else terminalNavigationDialog.removeAttribute('open');
  btnApproveTerminalNavigation.disabled = false;
  btnRejectTerminalNavigation.disabled = false;
  terminalNavigationStatus.textContent = '';
  terminalNavigationError.hidden = true;
}

function showTerminalNavigationDialog(pending) {
  terminalNavigationDialogEpoch += 1;
  terminalNavigationDecisionRequestID = null;
  terminalNavigationDialogRequestID = pending.requestId;
  const direction = pending.direction === 'return' ? 'ВОЗВРАТ' : 'ПЕРЕХОД';
  terminalNavigationSummary.innerHTML = `
    <div>ЗАПРОС: ${escHtml(pending.requestId)}</div>
    <div class="terminal-navigation-direction">${direction}</div>
    <div>ИЗ: ${escHtml(pending.sourceTerminalName || pending.sourceTerminalId || '—')}</div>
    <div>КОМАНДА: ${escHtml(pending.commandName || pending.commandId || '—')}</div>
    <div>В: ${escHtml(pending.targetTerminalName || pending.targetTerminalId || '—')}</div>`;
  terminalNavigationError.hidden = true;
  terminalNavigationStatus.textContent = 'ИСХОДНЫЙ ТЕРМИНАЛ ОСТАЁТСЯ АКТИВНЫМ ДО РЕШЕНИЯ';
  terminalNavigationDialog.hidden = false;
  if (!terminalNavigationDialog.open) terminalNavigationDialog.showModal();
  btnApproveTerminalNavigation.focus();
}

function syncTerminalNavigationDialog(coordination) {
  const pending = coordination?.pendingTerminalNavigation;
  const requestID = typeof pending?.requestId === 'string' ? pending.requestId : '';
  if (!requestID) {
    if (terminalNavigationDialogRequestID) hideTerminalNavigationDialog();
    return;
  }
  if (requestID === terminalNavigationDecisionRequestID || resolvedTerminalNavigationRequestIDs.has(requestID)) return;
  if (requestID === terminalNavigationDialogRequestID) return;
  if (terminalNavigationDialogRequestID) hideTerminalNavigationDialog();
  showTerminalNavigationDialog(pending);
}

function applyCoordinationState(coordination) {
  if (coordination && state.coordination &&
      coordinationRevision(coordination) <= coordinationRevision(state.coordination)) {
    return false;
  }
  state.coordination = coordination || null;
  state.liveTerminalId = coordination?.broadcast?.activeTerminalId || null;
  syncCommandExecutionDialog(coordination);
  syncTerminalNavigationDialog(coordination);
  syncTerminalNavigationNotice(coordination);
  return true;
}

function syncTerminalNavigationNotice(coordination) {
  const notice = coordination?.terminalNavigationNotice;
  if (!notice) {
    if (coordinationError.dataset.kind === 'terminal-navigation') {
      coordinationError.dataset.kind = '';
      setCoordinationStatus('');
    }
    return;
  }
  const labels = {
    'target-missing': 'ЦЕЛЕВОЙ ТЕРМИНАЛ ПЕРЕХОДА БОЛЬШЕ НЕ СУЩЕСТВУЕТ',
    'self-target': 'КОМАНДА ПЕРЕХОДА НЕ МОЖЕТ ССЫЛАТЬСЯ НА ТЕКУЩИЙ ТЕРМИНАЛ',
    'command-stale': 'КОМАНДА ПЕРЕХОДА БЫЛА ИЗМЕНЕНА ИЛИ УДАЛЕНА',
    'target-changed': 'ЦЕЛЬ КОМАНДЫ ПЕРЕХОДА ИЗМЕНИЛАСЬ',
  };
  const detail = [notice.sourceTerminalId, notice.commandId, notice.targetTerminalId].filter(Boolean).join(' · ');
  coordinationError.dataset.kind = 'terminal-navigation';
  setCoordinationStatus(`${labels[notice.reason] || 'ПЕРЕХОД БОЛЬШЕ НЕ ДЕЙСТВИТЕЛЕН'}${detail ? ` · ${detail}` : ''}`, true);
}

async function resolveTerminalNavigation(decision) {
  const requestID = terminalNavigationDialogRequestID;
  if (!requestID || terminalNavigationDecisionRequestID) return null;
  terminalNavigationDecisionRequestID = requestID;
  const epoch = terminalNavigationDialogEpoch;
  btnApproveTerminalNavigation.disabled = true;
  btnRejectTerminalNavigation.disabled = true;
  terminalNavigationStatus.textContent = decision === 'approve' ? 'ВЫПОЛНЕНИЕ ПЕРЕХОДА...' : 'ОТКЛОНЕНИЕ ПЕРЕХОДА...';
  let result;
  try {
    result = await desktopAPI.resolveTerminalNavigation({ requestId: requestID, decision });
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  if (epoch !== terminalNavigationDialogEpoch || terminalNavigationDecisionRequestID !== requestID) return result;
  terminalNavigationDecisionRequestID = null;
  rememberResolvedTerminalNavigation(requestID);
  if (result?.state) applyCoordinationState(result.state);
  if (terminalNavigationDialogRequestID === requestID) hideTerminalNavigationDialog();
  if (result?.ok) {
    setCoordinationStatus(decision === 'approve' ? 'ПЕРЕХОД ВЫПОЛНЕН' : 'ПЕРЕХОД ОТКЛОНЁН');
  } else if (result?.state?.terminalNavigationNotice) {
    syncTerminalNavigationNotice(result.state);
  } else {
    setCoordinationStatus(result?.error || 'ПЕРЕХОД НЕ ВЫПОЛНЕН', true);
  }
  renderCoordination();
  return result;
}

btnApproveTerminalNavigation.addEventListener('click', () => { void resolveTerminalNavigation('approve'); });
btnRejectTerminalNavigation.addEventListener('click', () => { void resolveTerminalNavigation('reject'); });
terminalNavigationDialog.addEventListener('keydown', (event) => {
  if (event.key === 'ArrowLeft') {
    event.preventDefault();
    btnApproveTerminalNavigation.focus();
  } else if (event.key === 'ArrowRight') {
    event.preventDefault();
    btnRejectTerminalNavigation.focus();
  }
});
terminalNavigationDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  void resolveTerminalNavigation('reject');
});

async function resolveCommandExecution(decision) {
  const requestID = commandExecutionDialogRequestID;
  if (!requestID || commandExecutionDecisionRequestID) return null;

  const commandMode = commandExecutionDialogMode;
  commandExecutionDecisionRequestID = requestID;
  const epoch = commandExecutionDialogEpoch;
  const startingRevision = coordinationRevision(state.coordination);
  btnApproveCommandExecution.disabled = true;
  btnRejectCommandExecution.disabled = true;
  commandExecutionDialogStatus.textContent = decision === 'approve'
    ? (commandMode === 'state-change' ? 'СОХРАНЕНИЕ И ВЫПОЛНЕНИЕ...' : 'ВЫПОЛНЕНИЕ КОМАНДЫ...')
    : 'ОТКЛОНЕНИЕ ЗАПРОСА...';
  commandExecutionDialogError.textContent = '';
  commandExecutionDialogError.hidden = true;

  let result;
  try {
    result = await desktopAPI.resolveCommandExecution({ requestId: requestID, decision });
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }

  if (epoch !== commandExecutionDialogEpoch || commandExecutionDecisionRequestID !== requestID) {
    return result;
  }
  commandExecutionDecisionRequestID = null;
  const resultRevision = coordinationRevision(result?.state);
  if (resultRevision > 0 && resultRevision < coordinationRevision(state.coordination)) {
    return result;
  }
  if (!result?.state && coordinationRevision(state.coordination) > startingRevision &&
      state.coordination?.pendingCommandExecution?.requestId !== requestID) {
    return result;
  }

  rememberResolvedCommandExecution(requestID);
  if (result?.state) applyCoordinationState(result.state);
  if (commandExecutionDialogRequestID === requestID) hideCommandExecutionDialog();

  if (!result?.ok) {
    setCoordinationStatus(facilityFailureMessage(result) || result?.error || 'СОСТОЯНИЕ КОМАНДЫ НЕ УДАЛОСЬ СОХРАНИТЬ', true);
  } else if (decision === 'approve') {
    setCoordinationStatus(commandMode === 'state-change'
      ? 'КОМАНДА ВЫПОЛНЕНА И СОХРАНЕНА'
      : 'КОМАНДА ВЫПОЛНЕНА');
  } else {
    setCoordinationStatus('ЗАПРОС ОТКЛОНЁН');
  }
  renderCoordination();
  return result;
}

btnApproveCommandExecution.addEventListener('click', () => {
  void resolveCommandExecution('approve');
});
btnRejectCommandExecution.addEventListener('click', () => {
  void resolveCommandExecution('reject');
});
commandExecutionDialog.addEventListener('keydown', (event) => {
  if (event.key === 'ArrowLeft') {
    event.preventDefault();
    btnApproveCommandExecution.focus();
  } else if (event.key === 'ArrowRight') {
    event.preventDefault();
    btnRejectCommandExecution.focus();
  }
});
commandExecutionDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  void resolveCommandExecution('reject');
});

function setCoordinationStatus(message, isError = false) {
  coordinationStatus.textContent = isError ? '' : (message || '');
  coordinationStatus.classList.remove('err');
  coordinationError.textContent = isError ? (message || '') : '';
  coordinationError.hidden = !isError || !message;
  setLogicalSessionDialogFeedback(message, isError);
}

function setLogicalSessionDialogFeedback(message = '', isError = false) {
  logicalSessionDialogStatus.textContent = isError ? '' : (message || '');
  logicalSessionDialogError.textContent = isError ? (message || '') : '';
  logicalSessionDialogError.hidden = !isError || !message;
}

function fillSelect(select, values, valueOf, labelOf, emptyLabel) {
  select.replaceChildren();
  for (const value of values) {
    const option = document.createElement('option');
    option.value = valueOf(value);
    option.textContent = labelOf(value);
    select.appendChild(option);
  }
  if (!values.length) {
    const option = document.createElement('option');
    option.value = '';
    option.textContent = emptyLabel;
    select.appendChild(option);
  }
}

function sessionLabel(session) {
  const character = session.character ? ` · ${session.character.name}` : '';
  return `${session.fallbackName}${character}`;
}

async function runCoordinationCommand(command, successMessage, pendingMessage) {
  if (coordinationCommandPending) return null;
  coordinationCommandPending = true;
  setCoordinationStatus(pendingMessage || 'ВЫПОЛНЕНИЕ ОПЕРАЦИИ...');
  renderCoordination();
  if (state.session) renderTreeHeader();
  renderHackStatus();
  let result;
  try {
    result = await command();
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  renderHackStatus();
  if (state.session) renderTreeHeader();
  if (!result?.ok) {
    if (result?.state) applyCoordinationState(result.state);
    setCoordinationStatus(result?.error || 'ОПЕРАЦИЯ ОТКЛОНЕНА', true);
    renderCoordination();
    return result;
  }
  if (result.state) applyCoordinationState(result.state);
  setCoordinationStatus(successMessage || 'ОПЕРАЦИЯ ВЫПОЛНЕНА');
  renderCoordination();
  return result;
}

function showTerminalSwitchDecision(result) {
  pendingTerminalSwitch = result?.switchId || null;
  if (!pendingTerminalSwitch) return;
  terminalSwitchStatus.textContent = 'ИСХОДНЫЙ ТЕРМИНАЛ ОСТАЁТСЯ АКТИВНЫМ ДО ВЫБОРА';
  terminalSwitchError.textContent = '';
  terminalSwitchError.hidden = true;
  terminalSwitchDialog.hidden = false;
  if (typeof terminalSwitchDialog.showModal === 'function' && !terminalSwitchDialog.open) {
    terminalSwitchDialog.showModal();
  } else {
    terminalSwitchDialog.setAttribute('open', '');
  }
  terminalSwitchButtons[0]?.focus();
}

function hideTerminalSwitchDecision() {
  pendingTerminalSwitch = null;
  terminalSwitchDialog.hidden = true;
  if (typeof terminalSwitchDialog.close === 'function' && terminalSwitchDialog.open) {
    terminalSwitchDialog.close();
  } else {
    terminalSwitchDialog.removeAttribute('open');
  }
}

function showEndBroadcastConfirmation() {
  endBroadcastDialog.hidden = false;
  if (typeof endBroadcastDialog.showModal === 'function' && !endBroadcastDialog.open) {
    endBroadcastDialog.showModal();
  } else {
    endBroadcastDialog.setAttribute('open', '');
  }
  btnCancelEndBroadcast.focus();
}

function hideEndBroadcastConfirmation({ restoreFocus = true } = {}) {
  endBroadcastDialog.hidden = true;
  if (typeof endBroadcastDialog.close === 'function' && endBroadcastDialog.open) {
    endBroadcastDialog.close();
  } else {
    endBroadcastDialog.removeAttribute('open');
  }
  btnCancelEndBroadcast.disabled = false;
  btnConfirmEndBroadcast.disabled = false;
  if (restoreFocus && !btnEndBroadcast.hidden) btnEndBroadcast.focus();
}

function showTakeOffAirConfirmation() {
  if (takeOffAirPending || !state.coordination?.broadcast || !state.liveTerminalId) return;
  takeOffAirError.textContent = '';
  takeOffAirError.hidden = true;
  btnCancelTakeOffAir.disabled = false;
  btnConfirmTakeOffAir.disabled = false;
  takeOffAirDialog.hidden = false;
  if (typeof takeOffAirDialog.showModal === 'function' && !takeOffAirDialog.open) {
    takeOffAirDialog.showModal();
  } else {
    takeOffAirDialog.setAttribute('open', '');
  }
  btnCancelTakeOffAir.focus();
}

function hideTakeOffAirConfirmation({ restoreFocus = true } = {}) {
  if (typeof takeOffAirDialog.close === 'function' && takeOffAirDialog.open) {
    takeOffAirDialog.close();
  } else {
    takeOffAirDialog.removeAttribute('open');
  }
  takeOffAirDialog.hidden = true;
  takeOffAirPending = false;
  btnCancelTakeOffAir.disabled = false;
  btnConfirmTakeOffAir.disabled = false;
  if (restoreFocus && !btnStopBroadcast.hidden) btnStopBroadcast.focus();
}

async function runTerminalSwitchRequest(command, completedMessage, pendingMessage) {
  const result = await runCoordinationCommand(command, completedMessage, pendingMessage);
  if (result?.ok && result.status === 'decision-required' && result.switchId) {
    showTerminalSwitchDecision(result);
  }
  return result;
}

// ── Render: terminal list ────────────────────────────────────
function inboundTerminalTransitions(targetTerminalID) {
  const inbound = [];
  const visit = (source, node) => {
    if (node?.terminalTransition?.targetTerminalId === targetTerminalID) {
      inbound.push(`${source.name}: ${node.name}`);
    }
    for (const child of node?.children ?? []) visit(source, child);
  };
  for (const source of state.session?.terminals ?? []) visit(source, source.root);
  return inbound;
}

function renderTermList() {
  termList.innerHTML = '';
  if (!state.session.terminals.length) {
    termList.innerHTML = '<div class="tree-empty-hint">Нет терминалов</div>';
    return;
  }

  ensureSessionTerminalGroups();
  const currentGroupIDs = new Set(state.session.terminalGroups.map(group => group.id));
  for (const groupID of state.collapsedTerminalGroupIds) {
    if (!currentGroupIDs.has(groupID)) state.collapsedTerminalGroupIds.delete(groupID);
  }
  const terminalsByID = new Map(state.session.terminals.map(term => [term.id, term]));
  state.session.terminalGroups.forEach((group, groupIndex) => {
    const collapsed = state.collapsedTerminalGroupIds.has(group.id);
    const groupEl = document.createElement('section');
    groupEl.className = 'terminal-group' + (group.terminalIds.length === 1 ? ' is-singleton' : '');
    groupEl.dataset.groupId = group.id;
    groupEl.dataset.singleton = String(group.terminalIds.length === 1);
    groupEl.dataset.collapsed = String(collapsed);
    groupEl.setAttribute('role', 'listitem');

    const header = document.createElement('header');
    header.className = 'terminal-group-header';

    const members = document.createElement('div');
    members.className = 'terminal-group-members';
    members.id = `terminal-group-members-${groupIndex}`;
    members.hidden = collapsed;
    members.setAttribute('role', 'list');
    members.setAttribute('aria-label', `Терминалы группы ${group.name}`);

    const toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.className = 'terminal-group-toggle';
    toggle.dataset.action = 'toggle-terminal-group';
    toggle.setAttribute('aria-controls', members.id);
    toggle.setAttribute('aria-expanded', String(!collapsed));
    toggle.setAttribute('aria-label', `${collapsed ? 'Развернуть' : 'Свернуть'} группу ${group.name}`);

    const caret = document.createElement('span');
    caret.className = 'terminal-group-caret';
    caret.setAttribute('aria-hidden', 'true');
    caret.textContent = collapsed ? '▸' : '▾';
    const heading = document.createElement('span');
    heading.className = 'terminal-group-name';
    heading.textContent = group.name;
    toggle.append(caret, heading);
    toggle.addEventListener('click', () => {
      const nextCollapsed = !state.collapsedTerminalGroupIds.has(group.id);
      if (nextCollapsed) state.collapsedTerminalGroupIds.add(group.id);
      else state.collapsedTerminalGroupIds.delete(group.id);
      members.hidden = nextCollapsed;
      groupEl.dataset.collapsed = String(nextCollapsed);
      caret.textContent = nextCollapsed ? '▸' : '▾';
      toggle.setAttribute('aria-expanded', String(!nextCollapsed));
      toggle.setAttribute('aria-label', `${nextCollapsed ? 'Развернуть' : 'Свернуть'} группу ${group.name}`);
    });

    const memberCount = document.createElement('span');
    memberCount.className = 'terminal-group-member-count';
    memberCount.textContent = terminalCountLabel(group.terminalIds.length);

    const groupActions = terminalActionMenu({
      scope: 'terminal-group',
      ownerID: group.id,
      ownerName: group.name,
      items: [
        { label: 'ПЕРЕИМЕНОВАТЬ ГРУППУ', action: 'rename-terminal-group', handler: () => showTerminalGroupRename(group.id) },
        { label: 'ПЕРЕМЕСТИТЬ ГРУППУ ВВЕРХ', action: 'move-terminal-group-up', handler: () => prepareTerminalGroupOrder(group.id, -1), disabled: groupIndex === 0 },
        { label: 'ПЕРЕМЕСТИТЬ ГРУППУ ВНИЗ', action: 'move-terminal-group-down', handler: () => prepareTerminalGroupOrder(group.id, 1), disabled: groupIndex === state.session.terminalGroups.length - 1 },
        { label: 'РАСФОРМИРОВАТЬ ГРУППУ', action: 'dissolve-terminal-group', handler: () => prepareTerminalGroupDissolution(group.id), destructive: true },
      ],
    });
    header.append(toggle, memberCount, groupActions);
    groupEl.append(header, members);

    group.terminalIds.forEach((terminalID, memberIndex) => {
      const term = terminalsByID.get(terminalID);
      if (term) members.appendChild(buildTerminalRow(term, group, memberIndex));
    });
    termList.appendChild(groupEl);
  });
}

function terminalCountLabel(count) {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return `${count} ТЕРМИНАЛ`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${count} ТЕРМИНАЛА`;
  return `${count} ТЕРМИНАЛОВ`;
}

function terminalActionMenu({ scope, ownerID, ownerName, items }) {
  const menu = document.createElement('details');
  menu.className = 'terminal-action-menu';
  menu.addEventListener('click', event => event.stopPropagation());
  menu.addEventListener('toggle', () => {
    if (!menu.open) return;
    for (const other of termList.querySelectorAll('.terminal-action-menu[open]')) {
      if (other !== menu) other.open = false;
    }
  });

  const trigger = document.createElement('summary');
  trigger.className = 'terminal-action-menu-trigger';
  trigger.dataset.actionMenuTrigger = scope;
  trigger.dataset.actionMenuOwnerId = ownerID;
  trigger.setAttribute('aria-label', `Действия: ${ownerName}`);
  trigger.textContent = '•••';

  const panel = document.createElement('div');
  panel.className = 'terminal-action-menu-panel';
  panel.setAttribute('role', 'menu');
  panel.setAttribute('aria-label', `Действия: ${ownerName}`);
  for (const item of items) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'btn btn-mini terminal-action-menu-item'
      + (item.destructive ? ' btn-danger terminal-action-menu-destructive' : '');
    button.textContent = item.label;
    button.dataset.action = item.action;
    button.disabled = Boolean(item.disabled);
    button.setAttribute('role', 'menuitem');
    button.addEventListener('click', event => {
      event.stopPropagation();
      terminalGroupDialogOpener = { scope, ownerID };
      menu.open = false;
      item.handler();
      if (!terminalGroupDraftDialog.open && !terminalGroupImpactDialog.open) {
        terminalGroupDialogOpener = null;
        if (document.activeElement === button) trigger.focus();
      }
    });
    panel.appendChild(button);
  }
  menu.append(trigger, panel);
  return menu;
}

function closeTerminalActionMenus({ restoreFocus = false } = {}) {
  const openMenu = termList.querySelector('.terminal-action-menu[open]');
  if (!openMenu) return false;
  const trigger = openMenu.querySelector('.terminal-action-menu-trigger');
  openMenu.open = false;
  if (restoreFocus) trigger?.focus();
  return true;
}

document.addEventListener('click', event => {
  if (!event.target.closest('.terminal-action-menu')) closeTerminalActionMenus();
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && closeTerminalActionMenus({ restoreFocus: true })) event.preventDefault();
});

function ensureSessionTerminalGroups(session = state.session) {
  if (!session) return;
  const terminalIDs = new Set(session.terminals.map(term => term.id));
  const represented = new Set();
  const groups = [];
  for (const group of session.terminalGroups ?? []) {
    const members = (group.terminalIds ?? []).filter(id => terminalIDs.has(id) && !represented.has(id));
    members.forEach(id => represented.add(id));
    if (members.length) groups.push({ id: group.id, name: group.name, terminalIds: members });
  }
  const usedNames = new Set(groups.map(group => group.name.trim().toLocaleLowerCase()));
  for (const term of session.terminals) {
    if (represented.has(term.id)) continue;
    let name = term.name.trim() || 'Terminal';
    let suffix = 2;
    while (usedNames.has(name.toLocaleLowerCase())) name = `${term.name.trim() || 'Terminal'} (${suffix++})`;
    usedNames.add(name.toLocaleLowerCase());
    groups.push({ id: uid('g'), name, terminalIds: [term.id] });
  }
  session.terminalGroups = groups;
}

function setTerminalGroupError(message = '') {
  terminalGroupError.textContent = message;
  terminalGroupError.hidden = !message;
}

function setTerminalGroupImpactError(message = '', canAmend = false) {
  terminalGroupImpactError.textContent = message;
  terminalGroupImpactError.hidden = !message;
  amendTerminalGroupChangeButton.hidden = !canAmend;
}

function showTerminalGroupDialog(dialog, focusTarget) {
  if (!terminalGroupDialogOpener) {
    terminalGroupDialogOpener = { element: document.activeElement };
  }
  dialog.hidden = false;
  if (typeof dialog.showModal === 'function' && !dialog.open) dialog.showModal();
  else dialog.setAttribute('open', '');
  focusTarget?.focus();
}

function hideTerminalGroupDialog(dialog) {
  if (dialog.open && typeof dialog.close === 'function') dialog.close();
  else dialog.removeAttribute('open');
  dialog.hidden = true;
}

function restoreTerminalGroupDialogFocus() {
  const opener = terminalGroupDialogOpener;
  terminalGroupDialogOpener = null;
  if (!opener) return;
  if (opener.element?.isConnected) {
    opener.element.focus();
    return;
  }
  const trigger = [...termList.querySelectorAll(`[data-action-menu-trigger="${opener.scope}"]`)]
    .find(candidate => candidate.dataset.actionMenuOwnerId === opener.ownerID);
  trigger?.focus();
}

function closeTerminalGroupDraft({ restoreFocus = true } = {}) {
  terminalGroupDraft = null;
  terminalGroupDraftForm.reset();
  hideTerminalGroupDialog(terminalGroupDraftDialog);
  if (restoreFocus) restoreTerminalGroupDialogFocus();
}

function closeTerminalGroupImpact({ restoreFocus = true } = {}) {
  pendingTerminalGroupImpact = null;
  hideTerminalGroupDialog(terminalGroupImpactDialog);
  if (restoreFocus) restoreTerminalGroupDialogFocus();
}

function populateTerminalChoices(selectedIDs = []) {
  terminalGroupTerminalChoices.innerHTML = '';
  const selected = new Set(selectedIDs);
  for (const terminal of state.session.terminals) {
    const fragment = terminalGroupTerminalChoiceTemplate.content.cloneNode(true);
    const input = fragment.querySelector('input[name="terminalIds"]');
    input.value = terminal.id;
    input.checked = selected.has(terminal.id);
    fragment.querySelector('.terminal-group-terminal-choice-name').textContent = terminal.name;
    terminalGroupTerminalChoices.appendChild(fragment);
  }
}

function configureTerminalGroupDraft(mode) {
  const terminalChoicesFieldset = document.getElementById('terminalGroupTerminalChoices');
  const destinationLabel = document.querySelector('label[for="terminalGroupDestinationSelect"]');
  const reviewButton = terminalGroupDraftDialog.querySelector('[data-action="review-terminal-group-change"]');
  const renameButton = terminalGroupDraftDialog.querySelector('[data-action="save-terminal-group-rename"]');
  terminalChoicesFieldset.hidden = mode !== 'create';
  terminalGroupNameInput.hidden = mode === 'move';
  document.querySelector('label[for="terminalGroupNameInput"]').hidden = mode === 'move';
  terminalGroupDestinationSelect.hidden = mode !== 'move';
  destinationLabel.hidden = mode !== 'move';
  reviewButton.hidden = mode === 'rename';
  renameButton.hidden = mode !== 'rename';
}

function showTerminalGroupCreate() {
  if (!state.session) return;
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'create' };
  configureTerminalGroupDraft('create');
  populateTerminalChoices();
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupNameInput);
}

function showTerminalGroupRename(groupID) {
  const group = state.session.terminalGroups.find(candidate => candidate.id === groupID);
  if (!group) return;
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'rename', groupID };
  configureTerminalGroupDraft('rename');
  terminalGroupNameInput.value = group.name;
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupNameInput);
}

function showTerminalMoveDraft(terminalID) {
  const source = state.session.terminalGroups.find(group => group.terminalIds.includes(terminalID));
  if (!source) return;
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'move', terminalID, sourceGroupID: source.id };
  configureTerminalGroupDraft('move');
  terminalGroupDestinationSelect.innerHTML = '<option value="">ВЫБЕРИТЕ ГРУППУ</option>';
  if (source.terminalIds.length > 1) {
    const singletonOption = document.createElement('option');
    singletonOption.value = 'new-singleton';
    singletonOption.dataset.newSingleton = 'true';
    singletonOption.textContent = 'НОВАЯ ОДИНОЧНАЯ ГРУППА';
    terminalGroupDestinationSelect.appendChild(singletonOption);
  }
  for (const group of state.session.terminalGroups) {
    if (group.id === source.id) continue;
    const option = document.createElement('option');
    option.value = group.id;
    option.textContent = group.name;
    terminalGroupDestinationSelect.appendChild(option);
  }
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupDestinationSelect);
}

function terminalName(terminalID) {
  return state.session.terminals.find(terminal => terminal.id === terminalID)?.name || terminalID;
}

function groupName(groups, groupID) {
  return groups.find(group => group.id === groupID)?.name || groupID || '—';
}

function uniqueLocalGroupName(base, groups) {
  const used = new Set(groups.map(group => group.name.trim().toLocaleLowerCase()));
  const encoder = new TextEncoder();
  const truncate = (value, byteLimit) => {
    let result = '';
    let byteCount = 0;
    for (const symbol of value) {
      const symbolBytes = encoder.encode(symbol).length;
      if (byteCount + symbolBytes > byteLimit) break;
      result += symbol;
      byteCount += symbolBytes;
    }
    return result;
  };
  const rawBase = String(base || 'Terminal').trim() || 'Terminal';
  let candidate = truncate(rawBase, 256);
  let suffix = 2;
  while (used.has(candidate.toLocaleLowerCase())) {
    const ending = ` (${suffix++})`;
    candidate = truncate(rawBase, 256 - encoder.encode(ending).length) + ending;
  }
  return candidate;
}

function newLocalSingletonGroup(terminalID, groups) {
  return {
    id: uid('group'),
    name: uniqueLocalGroupName(terminalName(terminalID), groups),
    terminalIds: [terminalID],
  };
}

function reviewTerminalGroupDraft() {
  if (!terminalGroupDraft || !state.session) return;
  const before = structuredClone(state.session.terminalGroups);
  if (terminalGroupDraft.kind === 'create') {
    const name = terminalGroupNameInput.value.trim();
    const terminalIDs = [...terminalGroupDraftDialog.querySelectorAll('input[name="terminalIds"]:checked')]
      .map(input => input.value);
    if (!name || terminalIDs.length < 2) {
      setTerminalGroupError('УКАЖИТЕ УНИКАЛЬНОЕ НАЗВАНИЕ И ВЫБЕРИТЕ НЕ МЕНЕЕ ДВУХ ТЕРМИНАЛОВ');
      return;
    }
    const selected = new Set(terminalIDs);
    const sourceNames = before.filter(group => group.terminalIds.some(id => selected.has(id))).map(group => group.name);
    const candidate = before
      .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => !selected.has(id)) }))
      .filter(group => group.terminalIds.length);
    if (candidate.some(group => group.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
      setTerminalGroupError('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ');
      return;
    }
    const reusableGroup = before.find(group =>
      group.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase()
      && group.terminalIds.every(id => selected.has(id)));
    candidate.push({ id: reusableGroup?.id || uid('group'), name, terminalIds: terminalIDs });
    showTerminalGroupImpact({
      kind: 'СОЗДАНИЕ ГРУППЫ', candidate, affectedGroupNames: [...new Set([...sourceNames, name])],
      affectedTerminalIDs: terminalIDs, membership: `${name}: ${terminalIDs.map(terminalName).join(' → ')}`,
      orderBefore: before.flatMap(group => group.terminalIds).map(terminalName),
      orderAfter: candidate.flatMap(group => group.terminalIds).map(terminalName),
    });
    return;
  }
  if (terminalGroupDraft.kind === 'move') {
    const destinationGroupID = terminalGroupDestinationSelect.value;
    const sourceGroupID = terminalGroupDraft.sourceGroupID;
    const terminalID = terminalGroupDraft.terminalID;
    if (!destinationGroupID) {
      setTerminalGroupError('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ');
      return;
    }
    const candidate = before
      .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => id !== terminalID) }))
      .filter(group => group.terminalIds.length);
    const splitToSingleton = terminalGroupDestinationSelect.selectedOptions[0]?.dataset.newSingleton === 'true';
    if (splitToSingleton) {
      const singleton = newLocalSingletonGroup(terminalID, candidate);
      candidate.push(singleton);
      const source = candidate.find(group => group.id === sourceGroupID);
      showTerminalGroupImpact({
        kind: 'ОТДЕЛЕНИЕ ТЕРМИНАЛА', candidate,
        affectedGroupNames: [groupName(before, sourceGroupID), singleton.name],
        affectedTerminalIDs: [terminalID], sourceGroupID, destinationGroupID: singleton.id,
        destinationGroupName: singleton.name,
        membership: `${source.name}: ${source.terminalIds.map(terminalName).join(' → ')} · ${singleton.name}: ${terminalName(terminalID)}`,
        orderBefore: before.find(group => group.id === sourceGroupID)?.terminalIds.map(terminalName) || [],
        orderAfter: [...source.terminalIds, terminalID].map(terminalName),
      });
      return;
    }
    const destination = candidate.find(group => group.id === destinationGroupID);
    if (!destination) {
      setTerminalGroupError('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ');
      return;
    }
    destination.terminalIds.push(terminalID);
    showTerminalGroupImpact({
      kind: 'ПЕРЕМЕЩЕНИЕ ТЕРМИНАЛА', candidate,
      affectedGroupNames: [groupName(before, sourceGroupID), groupName(before, destinationGroupID)],
      affectedTerminalIDs: [terminalID], sourceGroupID, destinationGroupID,
      membership: `${groupName(before, destinationGroupID)}: ${destination.terminalIds.map(terminalName).join(' → ')}`,
      orderBefore: before.find(group => group.id === sourceGroupID)?.terminalIds.map(terminalName) || [],
      orderAfter: destination.terminalIds.map(terminalName),
    });
  }
}

function prepareTerminalMemberOrder(groupID, terminalID, delta) {
  const before = structuredClone(state.session.terminalGroups);
  const candidate = structuredClone(before);
  const group = candidate.find(item => item.id === groupID);
  const index = group?.terminalIds.indexOf(terminalID) ?? -1;
  const next = index + delta;
  if (!group || index < 0 || next < 0 || next >= group.terminalIds.length) return;
  [group.terminalIds[index], group.terminalIds[next]] = [group.terminalIds[next], group.terminalIds[index]];
  showTerminalGroupImpact({
    kind: 'ИЗМЕНЕНИЕ ПОРЯДКА', candidate, affectedGroupNames: [group.name],
    affectedTerminalIDs: [terminalID, group.terminalIds[index]],
    membership: `${group.name}: ${group.terminalIds.map(terminalName).join(' → ')}`,
    orderBefore: before.find(item => item.id === groupID).terminalIds.map(terminalName),
    orderAfter: group.terminalIds.map(terminalName),
  });
}

function prepareTerminalGroupOrder(groupID, delta) {
  const before = structuredClone(state.session.terminalGroups);
  const candidate = structuredClone(before);
  const index = candidate.findIndex(group => group.id === groupID);
  const next = index + delta;
  if (index < 0 || next < 0 || next >= candidate.length) return;
  [candidate[index], candidate[next]] = [candidate[next], candidate[index]];
  showTerminalGroupImpact({
    kind: 'ИЗМЕНЕНИЕ ПОРЯДКА', candidate,
    affectedGroupNames: [candidate[index].name, candidate[next].name], affectedTerminalIDs: [],
    membership: candidate.map(group => group.name).join(' → '),
    orderBefore: before.map(group => group.name), orderAfter: candidate.map(group => group.name),
  });
}

function prepareTerminalGroupDissolution(groupID) {
  const before = structuredClone(state.session.terminalGroups);
  const group = before.find(candidate => candidate.id === groupID);
  if (!group) return;
  if (group.terminalIds.length === 1) {
    setTerminalGroupError('ОДИНОЧНУЮ ГРУППУ НЕЛЬЗЯ РАСФОРМИРОВАТЬ: ТЕРМИНАЛ ДОЛЖЕН ОСТАТЬСЯ В ГРУППЕ');
    return;
  }
  const candidate = before.filter(candidate => candidate.id !== groupID);
  const resultantSingletons = [];
  for (const terminalID of group.terminalIds) {
    const singleton = newLocalSingletonGroup(terminalID, candidate);
    candidate.push(singleton);
    resultantSingletons.push(singleton);
  }
  showTerminalGroupImpact({
    kind: 'РАСФОРМИРОВАНИЕ ГРУППЫ', candidate,
    affectedGroupNames: [group.name, ...resultantSingletons.map(singleton => singleton.name)],
    affectedTerminalIDs: group.terminalIds, sourceGroupID: group.id,
    membership: resultantSingletons
      .map(singleton => `${terminalName(singleton.terminalIds[0])} → ${singleton.name}`)
      .join(' · '),
    orderBefore: group.terminalIds.map(terminalName), orderAfter: group.terminalIds.map(terminalName),
  });
}

function currentSessionRevision() {
  return Number(saveStatus.dataset.savedRevision || startupStatus?.savedRevision || newestDurableRevision || 0);
}

function terminalGroupCandidateMembership(groups) {
  return groups
    .map(group => `${group.name}: ${group.terminalIds.map(terminalName).join(' → ')}`)
    .join(' · ');
}

function authoredTransitionRejections(message) {
  const pattern = /terminal transition command "([^"]+)" in terminal "([^"]+)" targets terminal "([^"]+)" and crosses groups "([^"]+)" and "([^"]+)"/g;
  return [...String(message || '').matchAll(pattern)].map(match => ({
    commandID: match[1],
    sourceTerminalID: match[2],
    targetTerminalID: match[3],
    sourceGroupID: match[4],
    targetGroupID: match[5],
  }));
}

function actionableTransitionRejection(rejections, candidate) {
  const edges = rejections.map(rejection =>
    `КОМАНДА «${rejection.commandID}»: «${terminalName(rejection.sourceTerminalID)}» → «${terminalName(rejection.targetTerminalID)}» ОСТАЁТСЯ МЕЖДУ ГРУППАМИ «${groupName(candidate, rejection.sourceGroupID)}» И «${groupName(candidate, rejection.targetGroupID)}»`);
  return `${edges.join(' · ')}. ДОБАВЬТЕ СВЯЗАННЫЕ ТЕРМИНАЛЫ В ОДНО ПРЕДЛОЖЕНИЕ И ПРОВЕРЬТЕ ЕГО ПЕРЕД ПРИМЕНЕНИЕМ.`;
}

function amendRejectedTerminalGroupImpact() {
  const impact = pendingTerminalGroupImpact;
  if (!impact?.rejections?.length) return;
  const involvedGroupIDs = new Set(impact.rejections.flatMap(rejection =>
    [rejection.sourceGroupID, rejection.targetGroupID]));
  const selectedTerminalIDs = impact.candidate
    .filter(group => involvedGroupIDs.has(group.id))
    .flatMap(group => group.terminalIds);
  const preferredGroup = impact.candidate.find(group => group.id === impact.destinationGroupID)
    || impact.candidate.find(group => group.id === impact.rejections[0].sourceGroupID);

  pendingTerminalGroupImpact = null;
  hideTerminalGroupDialog(terminalGroupImpactDialog);
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'create' };
  configureTerminalGroupDraft('create');
  populateTerminalChoices(selectedTerminalIDs);
  terminalGroupNameInput.value = preferredGroup?.name || '';
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupNameInput);
}

function showTerminalGroupImpact(impact) {
  setTerminalGroupError();
  setTerminalGroupImpactError();
  closeTerminalGroupDraft({ restoreFocus: false });
  pendingTerminalGroupImpact = {
    ...impact,
    expectedSessionRevision: currentSessionRevision(),
    expectedCoordinationRevision: coordinationRevision(state.coordination),
  };
  const values = {
    kind: impact.kind,
    groups: impact.affectedGroupNames.join(' · ') || '—',
    terminals: impact.affectedTerminalIDs.map(terminalName).join(' · ') || '—',
    'source-group': groupName(state.session.terminalGroups, impact.sourceGroupID),
    'destination-group': impact.destinationGroupName || groupName(state.session.terminalGroups, impact.destinationGroupID),
    membership: terminalGroupCandidateMembership(impact.candidate) || '—',
    'order-before': (impact.orderBefore || []).join(' → ') || '—',
    'order-after': (impact.orderAfter || []).join(' → ') || '—',
  };
  for (const [key, value] of Object.entries(values)) {
    terminalGroupImpactSummary.querySelector(`[data-impact="${key}"]`).textContent = value;
  }
  showTerminalGroupDialog(
    terminalGroupImpactDialog,
    terminalGroupImpactDialog.querySelector('[data-action="cancel-terminal-group-change"]'),
  );
}

async function submitTerminalGroupCandidate(candidate, expectedSessionRevision, expectedCoordinationRevision) {
  if (terminalGroupSubmitting) return false;
  terminalGroupSubmitting = true;
  const controls = terminalGroupImpactDialog.querySelectorAll('button');
  controls.forEach(control => { control.disabled = true; });
  const result = await desktopAPI.replaceTerminalGroups({
    terminalGroups: structuredClone(candidate), expectedSessionRevision, expectedCoordinationRevision,
  });
  terminalGroupSubmitting = false;
  controls.forEach(control => { control.disabled = false; });
  const revision = Number(result?.sessionRevision || 0);
  if (result?.session) applyAuthoredSession(result.session, result.authoringRevision, revision);
  if (result?.coordinationState) applyCoordinationState(result.coordinationState);
  if (!result?.ok) {
    const error = result?.error || 'НЕ УДАЛОСЬ ИЗМЕНИТЬ ГРУППЫ ТЕРМИНАЛОВ';
    setTerminalGroupError(error);
    const rejections = authoredTransitionRejections(error);
    if (pendingTerminalGroupImpact && rejections.length) {
      pendingTerminalGroupImpact.rejections = rejections;
      setTerminalGroupImpactError(
        actionableTransitionRejection(rejections, pendingTerminalGroupImpact.candidate),
        true,
      );
    }
    renderAll();
    return false;
  }
  setTerminalGroupError();
  setCoordinationStatus('ГРУППЫ СОХРАНЕНЫ');
  renderAll();
  return true;
}

async function saveTerminalGroupRename() {
  const group = state.session.terminalGroups.find(candidate => candidate.id === terminalGroupDraft?.groupID);
  const name = terminalGroupNameInput.value.trim();
  if (!group || !name) {
    setTerminalGroupError('НАЗВАНИЕ ГРУППЫ НЕ ДОЛЖНО БЫТЬ ПУСТЫМ');
    return;
  }
  if (state.session.terminalGroups.some(candidate => candidate.id !== group.id && candidate.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
    setTerminalGroupError('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ');
    return;
  }
  const candidate = structuredClone(state.session.terminalGroups);
  candidate.find(item => item.id === group.id).name = name;
  const ok = await submitTerminalGroupCandidate(candidate, currentSessionRevision(), coordinationRevision(state.coordination));
  if (ok) closeTerminalGroupDraft();
}

btnCreateTerminalGroup.addEventListener('click', showTerminalGroupCreate);
terminalGroupDraftDialog.querySelector('[data-action="review-terminal-group-change"]').addEventListener('click', reviewTerminalGroupDraft);
terminalGroupDraftDialog.querySelector('[data-action="save-terminal-group-rename"]').addEventListener('click', () => { void saveTerminalGroupRename(); });
for (const action of ['close-terminal-group-draft', 'cancel-terminal-group-draft']) {
  terminalGroupDraftDialog.querySelector(`[data-action="${action}"]`).addEventListener('click', () => closeTerminalGroupDraft());
}
for (const action of ['close-terminal-group-change', 'cancel-terminal-group-change']) {
  terminalGroupImpactDialog.querySelector(`[data-action="${action}"]`).addEventListener('click', () => closeTerminalGroupImpact());
}
terminalGroupImpactDialog.querySelector('[data-action="confirm-terminal-group-change"]').addEventListener('click', async () => {
  const impact = pendingTerminalGroupImpact;
  if (!impact || terminalGroupSubmitting) return;
  const ok = await submitTerminalGroupCandidate(impact.candidate, impact.expectedSessionRevision, impact.expectedCoordinationRevision);
  if (ok || !impact.rejections?.length) closeTerminalGroupImpact();
  else amendTerminalGroupChangeButton.focus();
});
amendTerminalGroupChangeButton.addEventListener('click', amendRejectedTerminalGroupImpact);
terminalGroupDraftDialog.addEventListener('cancel', event => { event.preventDefault(); closeTerminalGroupDraft(); });
terminalGroupImpactDialog.addEventListener('cancel', event => { event.preventDefault(); closeTerminalGroupImpact(); });

function buildTerminalRow(term, group, memberIndex) {
  const row = document.createElement('div');
  const selected = term.id === state.editTerminalId;
  row.className = 'term-row'
    + (selected ? ' editing' : '')
    + (term.id === state.liveTerminalId ? ' is-live' : '');
  row.dataset.terminalId = term.id;
  row.tabIndex = 0;
  row.setAttribute('role', 'listitem');
  row.setAttribute('aria-current', String(selected));

  const nameRow = document.createElement('div');
  nameRow.className = 'term-row-name';
  nameRow.textContent = term.name;
  row.appendChild(nameRow);

  const metaRow = document.createElement('div');
  metaRow.className = 'term-row-meta';
  metaRow.textContent = '● В ЭФИРЕ';
  row.appendChild(metaRow);

  const actions = terminalActionMenu({
    scope: 'terminal',
    ownerID: term.id,
    ownerName: term.name,
    items: [
      { label: 'ПЕРЕИМЕНОВАТЬ ТЕРМИНАЛ', action: 'rename-terminal', handler: () => startRenameTerminal(term, nameRow) },
      { label: 'ПЕРЕМЕСТИТЬ В ДРУГУЮ ГРУППУ', action: 'move-terminal', handler: () => showTerminalMoveDraft(term.id) },
      { label: 'ПЕРЕМЕСТИТЬ ТЕРМИНАЛ ВВЕРХ', action: 'move-terminal-up', handler: () => prepareTerminalMemberOrder(group.id, term.id, -1), disabled: memberIndex === 0 },
      { label: 'ПЕРЕМЕСТИТЬ ТЕРМИНАЛ ВНИЗ', action: 'move-terminal-down', handler: () => prepareTerminalMemberOrder(group.id, term.id, 1), disabled: memberIndex === group.terminalIds.length - 1 },
      { label: 'УДАЛИТЬ ТЕРМИНАЛ', action: 'delete-terminal', handler: () => deleteTerminal(term), destructive: true },
    ],
  });
  row.appendChild(actions);

  const selectTerminal = () => {
    if (state.editTerminalId === term.id) return;
    state.editTerminalId = term.id;
    state.selectedNodeId = null;
    state.expanded = new Set(['root']);
    renderAll();
  };
  row.addEventListener('click', selectTerminal);
  row.addEventListener('keydown', event => {
    if (event.target !== row || (event.key !== 'Enter' && event.key !== ' ')) return;
    event.preventDefault();
    selectTerminal();
  });
  return row;
}

function deleteTerminal(term) {
  if (state.liveTerminalId === term.id || state.coordination?.pendingSwitch?.sourceTerminalId === term.id) {
    setCoordinationStatus('АКТИВНЫЙ ИЛИ СОХРАНЁННЫЙ ТЕРМИНАЛ НЕЛЬЗЯ УДАЛИТЬ', true);
    return;
  }
  const inbound = inboundTerminalTransitions(term.id);
  if (inbound.length) {
    setCoordinationStatus(`ТЕРМИНАЛ ИСПОЛЬЗУЕТСЯ КОМАНДАМИ ПЕРЕХОДА: ${inbound.join(', ')}`, true);
    return;
  }
  if (!window.confirm(`Удалить терминал "${term.name}" целиком?`)) return;
  const terminalID = term.id;
  commitAuthoringEdit({
    label: `УДАЛЕНИЕ ТЕРМИНАЛА «${term.name}»`,
    mutate: session => {
      session.terminals = session.terminals.filter(candidate => candidate.id !== terminalID);
      session.terminalGroups = (session.terminalGroups ?? [])
        .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => id !== terminalID) }))
        .filter(group => group.terminalIds.length > 0);
    },
  });
  if (state.editTerminalId === terminalID) {
    state.editTerminalId = state.session.terminals[0]?.id || null;
    state.selectedNodeId = null;
    state.expanded = new Set(['root']);
  }
  renderAll();
}

function startRenameTerminal(term, nameRow) {
  const input = document.createElement('input');
  input.className = 'field-input';
  input.value = term.name;
  nameRow.replaceWith(input);
  input.focus();
  input.select();

  const commit = () => {
    const val = input.value.trim();
    if (val) {
      commitTerminalEdit('НАЗВАНИЕ ТЕРМИНАЛА', term.id, target => { target.name = val; });
    }
    renderAll();
  };
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') input.blur();
    if (e.key === 'Escape') renderAll();
  });
  input.addEventListener('blur', commit);
}

// ── Render: tree header + toolbar state ──────────────────────
function renderTreeHeader() {
  const term = getEditTerminal();
  editingTermName.textContent = term ? term.name : '—';
  const isLive = !!term && term.id === state.liveTerminalId;
  const broadcastActive = Boolean(state.coordination?.broadcast);
  liveFlag.hidden = !isLive;
  btnMakeLive.hidden = isLive;
  btnMakeLive.textContent = 'СДЕЛАТЬ АКТИВНЫМ';
  btnMakeLive.disabled = !term || !broadcastActive || coordinationCommandPending;
  terminalSettingsMenu.hidden = !isLive;
  if (!isLive) terminalSettingsMenu.open = false;
  btnReapplySettings.disabled = !isLive || coordinationCommandPending;
  btnStopBroadcast.hidden = !broadcastActive || !state.liveTerminalId;
  btnStopBroadcast.disabled = !broadcastActive || !state.liveTerminalId || coordinationCommandPending;
}

// ── Render: per-terminal settings (hack level / intro text) ──
function renderSettingsPanel() {
  const term = getEditTerminal();
  hackLevelSelect.disabled = !term;
  introTextArea.disabled   = !term;
  btnApplySettings.disabled = !term;
  const applied = { hackLevelSelect: term ? String(term.hackLevel || 0) : '0', introTextArea: term ? (term.introText || '') : '' };
  if (!term) {
    hackLevelSelect.value = applied.hackLevelSelect;
    introTextArea.value = applied.introTextArea;
  }
  const terminalID = term?.id;
  showUnappliedInput(termSettings, term && {
    key: `settings:${terminalID}`,
    label: `Настройки «${term.name}»`,
    exists: () => Boolean(state.session?.terminals.some(candidate => candidate.id === terminalID)),
    applied,
  });
}


// ── Render: live hack status (term panel footer) ──────────────
function renderHackStatus() {
  const liveTerm = state.session && state.liveTerminalId
    ? state.session.terminals.find(t => t.id === state.liveTerminalId)
    : null;

  if (!liveTerm || !liveTerm.hackLevel || !state.liveHack) {
    hackStatus.style.display = 'none';
    btnResetFailedHack.hidden = true;
    return;
  }

  hackStatus.style.display = '';
  const h = state.liveHack;
  if (h.solved) {
    hackStatusLine.textContent = 'ВЗЛОМ: ПРОЙДЕН';
  } else if (h.failed) {
    hackStatusLine.textContent = 'ВЗЛОМ: ЗАБЛОКИРОВАН';
  } else {
    hackStatusLine.textContent = `ВЗЛОМ: осталось попыток ${h.attemptsLeft}/${h.attemptsMax}`;
  }
  btnHackSuccess.disabled = h.solved || h.failed;
  btnHackSuccess.hidden = h.failed;
  btnResetFailedHack.hidden = !h.failed;
  btnResetFailedHack.disabled = !h.failed || coordinationCommandPending;
}

function renderToolbarState() {
  const term = getEditTerminal();
  const disabled = !term;
  btnAddFolder.disabled  = disabled;
  btnAddCommand.disabled = disabled;
  btnAddEntry.disabled   = disabled;
}

function renderToolbarHint() {
  const target = currentAddTarget();
  toolbarHint.textContent = target ? `Добавление в: ${target.id === 'root' ? 'ROOT' : target.name}` : '';
}

// ── Render: tree view ─────────────────────────────────────────
function renderTree() {
  const term = getEditTerminal();
  treeView.innerHTML = '';
  if (!term) {
    treeView.innerHTML = '<div class="tree-empty-hint">Нет терминала — создайте его слева</div>';
    return;
  }
  treeView.appendChild(renderNode(term.root, true));
}

function renderNode(node, isRoot) {
  const term = getEditTerminal();
  const wrap = document.createElement('div');
  wrap.className = 'tree-node';

  const row = document.createElement('div');
  const completed = commandExecutionState(term, node.id);
  row.className = 'tree-row'
    + (state.selectedNodeId === node.id ? ' selected' : '')
    + (completed ? ' command-completed' : '');

  const hasChildren = node.type === 'folder' && node.children && node.children.length > 0;
  const isExpanded   = state.expanded.has(node.id);

  const caret = document.createElement('span');
  caret.className = 'tree-caret';
  if (node.type === 'folder') {
    caret.textContent = hasChildren ? (isExpanded ? '▾' : '▸') : '·';
    caret.addEventListener('click', (e) => {
      e.stopPropagation();
      if (!hasChildren) return;
      if (isExpanded) state.expanded.delete(node.id); else state.expanded.add(node.id);
      renderTree();
    });
  }
  row.appendChild(caret);

  const icon = document.createElement('span');
  icon.className = 'tree-icon ' + node.type;
  icon.textContent = node.type === 'folder' ? '[ПАПКА]' : node.type === 'command' ? '[КОМАНДА]' : '[ЗАПИСЬ]';
  row.appendChild(icon);

  const label = document.createElement('span');
  label.className = 'tree-label';
  const seen = isRoot ? null : playerViewNode(term.id, node.id);
  label.textContent = isRoot ? 'ROOT' : (seen?.playerName || effectiveNodeName(term, node));
  if (!isRoot) {
    label.dataset.authoredName = node.name;
    if (seen && seen.playerName !== node.name) label.title = `В РЕДАКТОРЕ: ${node.name}`;
  }
  row.appendChild(label);
  const addBadge = (className, text) => {
    const badge = document.createElement('span');
    badge.className = `tree-badge ${className}`;
    badge.textContent = text;
    row.appendChild(badge);
  };
  if (seen && !seen.visible) addBadge('tree-badge-hidden', 'СКРЫТО ОТ ИГРОКОВ');
  if (seen?.available === false) addBadge('tree-badge-unavailable', 'НЕДОСТУПНО');

  row.addEventListener('click', () => {
    state.selectedNodeId = node.id;
    if (node.type === 'folder') state.expanded.add(node.id);
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  });

  wrap.appendChild(row);

  if (node.type === 'folder' && isExpanded) {
    const childrenWrap = document.createElement('div');
    childrenWrap.className = 'tree-children';
    if (!node.children || node.children.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'tree-empty-hint';
      empty.textContent = '(пусто)';
      childrenWrap.appendChild(empty);
    } else {
      node.children.forEach(child => childrenWrap.appendChild(renderNode(child, false)));
    }
    wrap.appendChild(childrenWrap);
  }

  return wrap;
}

// ── Render: node property form ────────────────────────────────
function renderNodeForm() {
  const term = getEditTerminal();
  const loc = term && state.selectedNodeId ? locateNode(term.root, state.selectedNodeId) : null;
  if (!loc) {
    showUnappliedInput(nodeForm, null);
    nodeForm.innerHTML = '<div class="node-empty">Выберите узел дерева слева</div>';
    return;
  }
  const node = loc.node;

  if (node.id === 'root') {
    showUnappliedInput(nodeForm, null);
    nodeForm.innerHTML = `
      <div class="node-type-label">КОРЕНЬ ТЕРМИНАЛА</div>
      <div class="node-empty">Это главный экран терминала. Добавляйте папки, команды и записи через панель инструментов сверху.</div>`;
    return;
  }
  const unappliedKey = `node:${term.id}:${node.id}`;
  const stagedBlocks = unappliedInputs.get(unappliedKey)?.blocks || null;

  const typeLabel = node.type === 'folder' ? 'ПАПКА' : node.type === 'command' ? 'КОМАНДА' : 'ЗАПИСЬ';
  const snapshot = commandExecutionState(term, node.id);
  const hasExplicitEntryBlocks = node.type === 'entry' && Array.isArray(node.blocks) && node.blocks.length > 0;
  const hasLegacyEntryDescription = node.type === 'entry' && !hasExplicitEntryBlocks && typeof node.description === 'string' && node.description.length > 0;
  const entryBlocks = stagedBlocks || (hasExplicitEntryBlocks
    ? node.blocks
    : hasLegacyEntryDescription
      ? [{ id: '', initialText: node.description }]
      : []);
  const blockOwners = node.type === 'entry' ? entryBlockOwners(term) : null;
  let entryBlocksChanged = Boolean(stagedBlocks);
  let html = `<div class="node-type-label">${typeLabel}</div>
    <label class="field-label" for="fldName">${node.type === 'command' ? 'ИСХОДНОЕ НАЗВАНИЕ' : 'НАЗВАНИЕ'}</label>
    <input class="field-input" id="fldName" value="${escAttr(node.name)}">`;

  if (node.type === 'command') {
    const commandMode = node.stateChange
      ? 'state-change'
      : node.terminalTransition
        ? 'terminal-transition'
        : 'ordinary';
    const authoredGroup = state.session.terminalGroups?.find(group => group.terminalIds.includes(term.id));
    const eligibleTransitionIDs = new Set(authoredGroup?.terminalIds ?? []);
    const transitionOptions = state.session.terminals
      .filter(candidate => candidate.id !== term.id && eligibleTransitionIDs.has(candidate.id))
      .map(candidate => `<option value="${escAttr(candidate.id)}"${node.terminalTransition?.targetTerminalId === candidate.id ? ' selected' : ''}>${escHtml(candidate.name)}</option>`)
      .join('');
    const configuredEntryChange = snapshot?.entryContentChange ?? node.stateChange?.entryContentChange ?? null;
    const selectedBlockID = configuredEntryChange?.blockId || '';
    const completedBlockText = configuredEntryChange?.completedText ?? '';
    const entryTargetLocked = Boolean(snapshot);
    const facilityAction = node.stateChange?.facilityAction;
    const facilityActionSummary = facilityCommandActionSummary(facilityAction);
    const commandEffects = commandEffectItems(term, node);
    html += `
      <label class="field-label" for="fldCommandMode">РЕЖИМ КОМАНДЫ</label>
      <select class="field-input command-mode-select" id="fldCommandMode"${snapshot ? ' disabled' : ''}>
        <option value="ordinary"${commandMode === 'ordinary' ? ' selected' : ''}>ОБЫЧНАЯ КОМАНДА</option>
        <option value="state-change"${commandMode === 'state-change' ? ' selected' : ''}>ИЗМЕНЯЕТ СОСТОЯНИЕ</option>
        <option value="terminal-transition"${commandMode === 'terminal-transition' ? ' selected' : ''}>ПЕРЕХОД В ДРУГОЙ ТЕРМИНАЛ</option>
      </select>
      ${snapshot ? '<div class="command-mode-hint">Сначала сбросьте выполненное состояние команды, чтобы изменить режим.</div>' : ''}
      <div id="commandModeDecision" role="status" data-has-action="${facilityAction ? 'true' : 'false'}" hidden>
        <div data-decision-message></div>
        <div class="command-mode-decision-summary">${escHtml(facilityActionSummary)}</div>
        <button class="btn btn-secondary" id="btnKeepStateChangeMode" type="button">ОСТАВИТЬ «ИЗМЕНЯЕТ СОСТОЯНИЕ»</button>
        <button class="btn btn-danger" id="btnRemoveDeviceAction" type="button">УДАЛИТЬ ДЕЙСТВИЕ И ПЕРЕКЛЮЧИТЬ</button>
      </div>
      <div class="state-change-fields" id="stateChangeFields"${commandMode === 'state-change' ? '' : ' hidden'}>
        <label class="field-label" for="fldCompletedName">НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ</label>
        <input class="field-input" id="fldCompletedName" value="${escAttr(node.stateChange?.completedName || '')}">
        <label class="field-label" for="fldConfirmationText">ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ</label>
        <textarea class="field-textarea state-change-textarea" id="fldConfirmationText">${escHtml(node.stateChange?.confirmationText || '')}</textarea>
        <label class="field-label" for="fldEntryBlockTarget">БЛОК ЗАПИСИ</label>
        <select class="field-input entry-block-target" id="fldEntryBlockTarget"${entryTargetLocked ? ' disabled' : ''}>
          <option value="">НЕ ИЗМЕНЯТЬ СОДЕРЖИМОЕ ЗАПИСИ</option>
          ${entryBlockTargetOptions(term, selectedBlockID)}
        </select>
        <label class="field-label" for="fldCompletedEntryBlockContent">ТЕКСТ БЛОКА ПОСЛЕ ВЫПОЛНЕНИЯ</label>
        <textarea class="field-textarea state-change-textarea" id="fldCompletedEntryBlockContent"${entryTargetLocked || !selectedBlockID ? ' disabled' : ''}>${escHtml(completedBlockText)}</textarea>
        ${entryTargetLocked ? '<div class="command-mode-hint">Сначала сбросьте выполненное состояние команды, чтобы изменить блок и его итоговый текст.</div>' : ''}
      </div>
      <div class="state-change-fields terminal-transition-fields" id="terminalTransitionFields"${commandMode === 'terminal-transition' ? '' : ' hidden'}>
        <label class="field-label" for="fldTerminalTransitionTarget">ЦЕЛЕВОЙ ТЕРМИНАЛ</label>
        <select class="field-input" id="fldTerminalTransitionTarget">
          <option value="">ВЫБЕРИТЕ ТЕРМИНАЛ</option>${transitionOptions}
        </select>
      </div>
      <label class="field-label" for="fldText">ТЕКСТ УСПЕШНОГО ВЫПОЛНЕНИЯ</label>
      <textarea class="field-textarea" id="fldText">${escHtml(node.text || '')}</textarea>
      <section class="command-facility-action" aria-labelledby="commandFacilityActionHeading"${commandMode === 'state-change' ? '' : ' hidden'}>
        <div class="field-label" id="commandFacilityActionHeading">ДЕЙСТВИЕ УСТРОЙСТВ</div>
        <div class="command-facility-action-summary">${escHtml(facilityActionSummary)}</div>
        <div class="command-mode-hint"${node.stateChange ? ' hidden' : ''}>ПРИМЕНИТЕ РЕЖИМ, ЧТОБЫ ДОБАВИТЬ ДЕЙСТВИЕ УСТРОЙСТВ</div>
        <button class="btn btn-secondary" id="btnEditCommandFacilityAction" type="button"${node.stateChange ? '' : ' disabled'}>
          ${facilityAction ? 'ИЗМЕНИТЬ ДЕЙСТВИЕ УСТРОЙСТВ' : 'ДОБАВИТЬ ДЕЙСТВИЕ УСТРОЙСТВ'}
        </button>
      </section>
      <section class="command-effects" id="commandEffects" aria-labelledby="commandEffectsHeading"${commandEffects.length ? '' : ' hidden'}>
        <div class="field-label" id="commandEffectsHeading">ПРИ ОДОБРЕНИИ</div>
        <ul>${commandEffects.map(effect => `<li class="command-effect" data-kind="${effect.kind}">${escHtml(effect.text)}</li>`).join('')}</ul>
      </section>
      <section class="command-rules" id="commandRules" aria-labelledby="commandRulesHeading">
        <div class="field-label" id="commandRulesHeading">ЧТО ВИДЯТ ИГРОКИ</div>
        <div class="command-rule-targets"></div>
      </section>`;
    if (snapshot) {
      html += `
        <div class="command-execution-snapshot" role="status" aria-label="СОХРАНЁННОЕ СОСТОЯНИЕ КОМАНДЫ">
          <div class="command-execution-heading">ВЫПОЛНЕНО</div>
          <div class="command-execution-label">ЗАФИКСИРОВАННОЕ НАЗВАНИЕ</div>
          <div class="command-execution-value">${escHtml(snapshot.completedName || '')}</div>
          <div class="command-execution-label">ЗАФИКСИРОВАННЫЙ РЕЗУЛЬТАТ</div>
          <div class="command-execution-value command-execution-result">${escHtml(snapshot.resultText || '')}</div>
          ${snapshot.entryContentChange ? `<div class="command-execution-label">ЗАФИКСИРОВАННЫЙ ТЕКСТ БЛОКА</div>
          <div class="command-execution-value command-execution-result">${escHtml(snapshot.entryContentChange.completedText)}</div>` : ''}
        </div>`;
    }
  } else if (node.type === 'entry') {
    html += `<div class="field-label">СОДЕРЖИМОЕ ЗАПИСИ</div>
      <div class="entry-content-blocks" id="entryContentBlocks" aria-label="БЛОКИ СОДЕРЖИМОГО ЗАПИСИ">
        ${entryBlocks.map((block, index) => entryContentBlockMarkup(term, block, index, {
          legacyDraft: hasLegacyEntryDescription,
          owners: blockOwners,
        })).join('')}
      </div>
      <button class="btn btn-mini entry-content-block-add" id="btnAddEntryContentBlock" type="button">+ ДОБАВИТЬ БЛОК</button>`;
  } else if (node.type === 'folder') {
    const count = node.children ? node.children.length : 0;
    html += `<div class="field-label">СОДЕРЖИМОЕ</div><div class="node-empty">${count} элемент(ов)</div>`;
  }

  html += '<div class="node-validation-error" id="nodeValidationError" role="alert" hidden></div>';
  html += `<div class="node-actions">
      <button class="btn btn-primary" id="btnApplyNode">ПРИМЕНИТЬ</button>
      <button class="btn btn-secondary" id="btnCancelNodeInput" type="button" data-unapplied-cancel hidden>ОТМЕНА</button>
      ${snapshot ? '<button class="btn btn-secondary" id="btnResetCommandState" type="button" aria-describedby="btnResetCommandStateReason">СБРОСИТЬ СОСТОЯНИЕ</button>' : ''}
      <button class="btn btn-danger" id="btnDeleteNode">УДАЛИТЬ</button>
    </div>
    ${snapshot ? '<span class="control-reason" id="btnResetCommandStateReason"></span>' : ''}`;

  nodeForm.innerHTML = html;
  renderCommandRules();
  const appliedNodeFields = fieldValues(nodeForm);
  // currentEntryBlocks returns the edited block rows, or null when the blocks
  // are unchanged.
  const currentEntryBlocks = () => (node.type === 'entry' && entryBlocksChanged
    ? Array.from(nodeForm.querySelectorAll('#entryContentBlocks .entry-content-block')).map((row) => ({
      id: row.dataset.blockId || uniqueEntryBlockID(term),
      initialText: row.querySelector('[data-entry-block-content]').value,
    }))
    : null);

  const validationError = document.getElementById('nodeValidationError');
  const showValidationError = (message, field) => {
    validationError.textContent = message;
    validationError.hidden = false;
    field?.focus();
  };

  if (node.type === 'entry') {
    const blockList = document.getElementById('entryContentBlocks');
    const refreshBlockPositions = () => {
      const rows = Array.from(blockList.querySelectorAll('.entry-content-block'));
      rows.forEach((row, index) => {
        const position = index + 1;
        row.querySelector('.entry-content-block-index').textContent = `БЛОК ${position}`;
        const content = row.querySelector('[data-entry-block-content]');
        content.setAttribute('aria-label', `ИСХОДНЫЙ ТЕКСТ БЛОКА ${position}`);
        const actions = row.querySelector('.entry-content-block-actions');
        actions.setAttribute('aria-label', `Действия блока ${position}`);
        const moveUp = row.querySelector('[data-entry-block-action="move-up"]');
        const moveDown = row.querySelector('[data-entry-block-action="move-down"]');
        const remove = row.querySelector('[data-entry-block-action="delete"]');
        const configure = row.querySelector('[data-entry-block-action="configure-command"]');
        moveUp.disabled = index === 0;
        moveDown.disabled = index === rows.length - 1;
        moveUp.setAttribute('aria-label', `Переместить блок ${position} вверх`);
        moveDown.setAttribute('aria-label', `Переместить блок ${position} вниз`);
        remove.setAttribute('aria-label', `Удалить блок ${position}`);
        configure.setAttribute('aria-label', `Настроить команду для блока ${position}`);
      });
    };
    const bindBlockRow = (row) => {
      row.querySelector('[data-entry-block-content]').addEventListener('input', () => {
        entryBlocksChanged = true;
        validationError.hidden = true;
      });
      row.querySelectorAll('[data-entry-block-action]').forEach((button) => {
        button.addEventListener('click', () => {
          const action = button.dataset.entryBlockAction;
          if (action === 'configure-command') {
            const blockID = row.dataset.blockId;
            if (blockID) openEntryBlockCommandDialog(term, blockID);
            return;
          }
          if (action === 'delete') {
            const owner = row.querySelector('[data-entry-block-owner]');
            if (owner) {
              const command = locateNode(term.root, owner.dataset.commandId)?.node;
              showValidationError(entryBlockDeletionError(term, command ? [command] : []), button);
              return;
            }
            if (!window.confirm('Удалить этот блок содержимого?')) return;
            row.remove();
          } else if (action === 'move-up' && row.previousElementSibling) {
            row.previousElementSibling.before(row);
          } else if (action === 'move-down' && row.nextElementSibling) {
            row.nextElementSibling.after(row);
          } else {
            return;
          }
          entryBlocksChanged = true;
          validationError.hidden = true;
          refreshBlockPositions();
          stageUnappliedInput(nodeForm);
        });
      });
    };
    blockList.querySelectorAll('.entry-content-block').forEach(bindBlockRow);
    document.getElementById('btnAddEntryContentBlock').addEventListener('click', () => {
      const block = { id: uniqueEntryBlockID(term), initialText: '' };
      blockList.insertAdjacentHTML('beforeend', entryContentBlockMarkup(
        term,
        block,
        blockList.querySelectorAll('.entry-content-block').length,
        { owners: blockOwners },
      ));
      const row = blockList.lastElementChild;
      bindBlockRow(row);
      entryBlocksChanged = true;
      validationError.hidden = true;
      refreshBlockPositions();
      stageUnappliedInput(nodeForm);
      row.querySelector('[data-entry-block-content]').focus();
    });
    refreshBlockPositions();
  }

  if (node.type === 'command') {
    const mode = document.getElementById('fldCommandMode');
    const fields = document.getElementById('stateChangeFields');
    const transitionFields = document.getElementById('terminalTransitionFields');
    const actionSection = nodeForm.querySelector('.command-facility-action');
    const decision = document.getElementById('commandModeDecision');
    const applyButton = document.getElementById('btnApplyNode');
    let removeDeviceAction = false;
    const updateModePresentation = () => {
      fields.hidden = mode.value !== 'state-change';
      transitionFields.hidden = mode.value !== 'terminal-transition';
      actionSection.hidden = mode.value !== 'state-change';
      const deciding = decision.dataset.hasAction === 'true' && mode.value !== 'state-change';
      if (!deciding) removeDeviceAction = false;
      decision.hidden = !deciding;
      decision.querySelector('[data-decision-message]').textContent = removeDeviceAction
        ? 'ДЕЙСТВИЕ УСТРОЙСТВ БУДЕТ УДАЛЕНО, КОГДА ВЫ ПРИМЕНИТЕ ФОРМУ.'
        : 'ВЫБЕРИТЕ, ЧТО СДЕЛАТЬ С ДЕЙСТВИЕМ УСТРОЙСТВ. ОНО РАБОТАЕТ ТОЛЬКО В РЕЖИМЕ «ИЗМЕНЯЕТ СОСТОЯНИЕ».';
      document.getElementById('btnRemoveDeviceAction').hidden = removeDeviceAction;
      applyButton.disabled = deciding && !removeDeviceAction;
    };
    mode.addEventListener('change', () => {
      updateModePresentation();
      validationError.hidden = true;
      validationError.textContent = '';
    });
    document.getElementById('btnKeepStateChangeMode').addEventListener('click', () => {
      mode.value = 'state-change';
      updateModePresentation();
      mode.focus();
    });
    document.getElementById('btnRemoveDeviceAction').addEventListener('click', () => {
      removeDeviceAction = true;
      updateModePresentation();
      mode.focus();
    });
    updateModePresentation();
    const entryTarget = document.getElementById('fldEntryBlockTarget');
    const completedBlockContent = document.getElementById('fldCompletedEntryBlockContent');
    entryTarget.addEventListener('change', () => {
      completedBlockContent.disabled = !entryTarget.value;
      const owner = commandOwningEntryBlock(term, entryTarget.value, node.id);
      if (owner) {
        showValidationError(
          `ЭТОТ БЛОК УЖЕ ИЗМЕНЯЕТСЯ КОМАНДОЙ: ${effectiveNodeName(term, owner)}`,
          entryTarget,
        );
      } else {
        validationError.hidden = true;
        validationError.textContent = '';
      }
    });
    document.getElementById('btnEditCommandFacilityAction').addEventListener('click', () => {
      openFacilityCommandActionEditor(term.id, node.id);
    });
  }

  document.getElementById('btnApplyNode').addEventListener('click', () => {
    const nameEl = document.getElementById('fldName');
    const name = nameEl.value.trim();
    if (!name) {
      showValidationError(
        node.type === 'command' ? 'УКАЖИТЕ ИСХОДНОЕ НАЗВАНИЕ КОМАНДЫ' : 'УКАЖИТЕ НАЗВАНИЕ',
        nameEl
      );
      return;
    }

    let commandEdit = null;
    if (node.type === 'command') {
      const commandMode = document.getElementById('fldCommandMode').value;
      const textEl = document.getElementById('fldText');
      let nextStateChange = null;
      let nextTerminalTransition = null;
      if (commandMode === 'state-change') {
        const completedNameEl = document.getElementById('fldCompletedName');
        const confirmationTextEl = document.getElementById('fldConfirmationText');
        if (!completedNameEl.value.trim()) {
          showValidationError('УКАЖИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ', completedNameEl);
          return;
        }
        if (!confirmationTextEl.value.trim()) {
          showValidationError('УКАЖИТЕ ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ', confirmationTextEl);
          return;
        }
        if (!textEl.value.trim()) {
          showValidationError('УКАЖИТЕ ТЕКСТ УСПЕШНОГО РЕЗУЛЬТАТА', textEl);
          return;
        }
        nextStateChange = {
          completedName: completedNameEl.value,
          confirmationText: confirmationTextEl.value,
        };
        const entryTargetEl = document.getElementById('fldEntryBlockTarget');
        const targetBlockID = entryTargetEl.value;
        if (targetBlockID) {
          const targetExists = entryBlockCatalog(term).some(({ block }) => block.id === targetBlockID);
          if (!targetExists) {
            showValidationError('ВЫБЕРИТЕ СУЩЕСТВУЮЩИЙ БЛОК ЗАПИСИ ЭТОГО ТЕРМИНАЛА', entryTargetEl);
            return;
          }
          const owner = commandOwningEntryBlock(term, targetBlockID, node.id);
          if (owner) {
            showValidationError(
              `ЭТОТ БЛОК УЖЕ ИЗМЕНЯЕТСЯ КОМАНДОЙ: ${effectiveNodeName(term, owner)}`,
              entryTargetEl,
            );
            return;
          }
          nextStateChange.entryContentChange = {
            blockId: targetBlockID,
            completedText: document.getElementById('fldCompletedEntryBlockContent').value,
          };
        }
      }
      if (commandMode === 'terminal-transition') {
        const targetEl = document.getElementById('fldTerminalTransitionTarget');
        const targetID = targetEl.value;
        if (!targetID || targetID === term.id || !state.session.terminals.some(candidate => candidate.id === targetID)) {
          showValidationError('ВЫБЕРИТЕ ДРУГОЙ СУЩЕСТВУЮЩИЙ ТЕРМИНАЛ', targetEl);
          return;
        }
        nextTerminalTransition = { targetTerminalId: targetID };
      }
      commandEdit = { stateChange: nextStateChange, terminalTransition: nextTerminalTransition, text: textEl.value };
    }

    const blocks = currentEntryBlocks();
    const nodeID = node.id;
    commitTerminalEdit(`${nodeKindLabel(node)} «${name}»`, term.id, candidateTerm => {
      const target = locateNode(candidateTerm.root, nodeID)?.node;
      if (!target) throw new Error('node removed');
      if (commandEdit) {
        const stateChange = commandEdit.stateChange && structuredClone(commandEdit.stateChange);
        if (stateChange && target.stateChange?.facilityAction) {
          stateChange.facilityAction = structuredClone(target.stateChange.facilityAction);
        }
        if (stateChange) target.stateChange = stateChange;
        else delete target.stateChange;
        if (commandEdit.terminalTransition) target.terminalTransition = { ...commandEdit.terminalTransition };
        else delete target.terminalTransition;
        target.text = commandEdit.text;
      }
      target.name = name;
      if (blocks) {
        if (blocks.length > 0) target.blocks = structuredClone(blocks);
        else delete target.blocks;
        delete target.description;
      }
    }, takeUnappliedInput(nodeForm));
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  });

  document.getElementById('btnCancelNodeInput').addEventListener('click', () => {
    takeUnappliedInput(nodeForm);
    renderNodeForm();
    renderSaveStatus();
  });

  const btnResetCommandState = document.getElementById('btnResetCommandState');
  if (btnResetCommandState) {
    // The form re-renders when a session-state command starts and ends.
    explainControl(btnResetCommandState, sessionStateCommandPending ? 'ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА' : '');
    btnResetCommandState.addEventListener('click', async () => {
      if (controlBlocked(btnResetCommandState)) return;
      const displayedName = snapshot?.completedName || node.name;
      if (!await confirmCommandStateReset(`Сбросить выполненное состояние команды "${displayedName}"?`)) return;
      const revisionBeforeReset = newestDurableRevision;
      runSessionStateCommand(
        () => desktopAPI.resetCommandState({ terminalId: term.id, commandId: node.id }),
        'СОСТОЯНИЕ КОМАНДЫ СБРОШЕНО',
        (result) => {
          const revision = Number(result?.revision || 0);
          const canonicalTerminal = result?.session?.terminals?.find(candidate => candidate.id === term.id);
          return revision > revisionBeforeReset && canonicalTerminal &&
            !Object.hasOwn(canonicalTerminal.commandStates ?? {}, node.id);
        }
      );
    });
  }

  document.getElementById('btnDeleteNode').addEventListener('click', () => {
    const targetingCommands = targetingCommandsWithinNode(term, node);
    if (targetingCommands.length > 0) {
      showValidationError(entryBlockDeletionError(term, targetingCommands));
      return;
    }
    const childCount = (node.type === 'folder' && node.children) ? node.children.length : 0;
    const msg = childCount > 0
      ? `Удалить "${node.name}" вместе со всем содержимым (${childCount} элемент(ов))?`
      : `Удалить "${node.name}"?`;
    if (!window.confirm(msg)) return;
    const nodeID = node.id;
    commitTerminalEdit(`УДАЛЕНИЕ «${node.name}»`, term.id, candidateTerm => {
      const parent = locateNode(candidateTerm.root, nodeID)?.parent;
      if (parent) parent.children = parent.children.filter(child => child.id !== nodeID);
    });
    state.selectedNodeId = null;
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  });

  const terminalID = term.id;
  const nodeID = node.id;
  showUnappliedInput(nodeForm, {
    key: unappliedKey,
    label: `${nodeKindLabel(node)} «${node.name}»`,
    exists: () => {
      const terminal = state.session?.terminals.find(candidate => candidate.id === terminalID);
      return Boolean(terminal && locateNode(terminal.root, nodeID));
    },
    applied: appliedNodeFields,
    blocks: currentEntryBlocks,
  });
}

// ── Toolbar: add nodes ────────────────────────────────────────
function addNode(type) {
  const term = getEditTerminal();
  const target = currentAddTarget();
  if (!term || !target) return;

  const node = {
    id:   uid('n'),
    type,
    name: type === 'folder' ? 'Новая папка' : type === 'command' ? 'Новая команда' : 'Новая запись',
  };
  if (type === 'folder')  node.children = [];
  if (type === 'command') node.text = '';
  if (type === 'entry')   node.description = '';

  const targetID = target.id;
  commitTerminalEdit(`НОВЫЙ ЭЛЕМЕНТ «${node.name}»`, term.id, candidateTerm => {
    const parent = locateNode(candidateTerm.root, targetID)?.node;
    if (!parent) throw new Error('folder removed');
    parent.children ??= [];
    parent.children.push(structuredClone(node));
  });
  state.expanded.add(targetID);
  state.selectedNodeId = node.id;

  renderTree();
  renderNodeForm();
  renderToolbarHint();
}

btnAddFolder.addEventListener('click', () => addNode('folder'));
btnAddCommand.addEventListener('click', () => addNode('command'));
btnAddEntry.addEventListener('click', () => addNode('entry'));

// ── Player roster and broadcast management ──────────────────
btnManageLogicalSessions.addEventListener('click', showLogicalSessionManagement);
btnCloseLogicalSessions.addEventListener('click', hideLogicalSessionManagement);
logicalSessionDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hideLogicalSessionManagement();
});
btnManagePlayers.addEventListener('click', showPlayerManagement);
btnClosePlayerManagement.addEventListener('click', hidePlayerManagement);
playerManagementDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePlayerManagement();
});
btnCancelPlayerDelete.addEventListener('click', () => hidePlayerDeleteConfirmation(true));
playerDeleteDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!coordinationCommandPending) hidePlayerDeleteConfirmation(true);
});
btnConfirmPlayerDelete.addEventListener('click', async () => {
  const characterId = pendingPlayerDelete?.characterId || '';
  if (!characterId || !playerMutationAllowed()) return;
  const result = await runPlayerManagementMutation(
    () => desktopAPI.deleteCharacter({
      characterId,
      expectedRevision: coordinationRevision(state.coordination),
    }),
    'ИГРОК УДАЛЁН',
    'УДАЛЕНИЕ ИГРОКА...'
  );
  hidePlayerDeleteConfirmation(!result?.ok);
});

playerManagementAddForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  if (!playerManagementAddForm.checkValidity()) {
    playerManagementAddForm.reportValidity();
    return;
  }

  const name = playerNameInput.value.trim();
  const intelligence = playerIntelligenceInput.valueAsNumber;
  const hackerChoice = playerHackerPerkAvailability.value;
  if (!name) {
    setPlayerManagementFeedback('УКАЖИТЕ ИМЯ ИГРОКА', true);
    playerNameInput.focus();
    return;
  }
  if (!Number.isInteger(intelligence) || intelligence < 1 || intelligence > 10) {
    setPlayerManagementFeedback('ИНТЕЛЛЕКТ ДОЛЖЕН БЫТЬ ЦЕЛЫМ ЧИСЛОМ ОТ 1 ДО 10', true);
    playerIntelligenceInput.focus();
    return;
  }
  if (hackerChoice !== 'true' && hackerChoice !== 'false') {
    setPlayerManagementFeedback('ВЫБЕРИТЕ ДОСТУПНОСТЬ ПЕРКА «ХАКЕР»', true);
    playerHackerPerkAvailability.focus();
    return;
  }

  await addPlayerProfile({
    name,
    intelligence,
    hackerPerkAvailable: hackerChoice === 'true',
  });
});

btnStartBroadcast.addEventListener('click', async () => {
  if (coordinationCommandPending || state.coordination?.broadcast || !state.coordination?.playerConfig) return;
  coordinationCommandPending = true;
  setCoordinationStatus('ЗАПУСК ТРАНСЛЯЦИИ...');
  renderCoordination();
  const result = await desktopAPI.startBroadcast();
  coordinationCommandPending = false;
  if (!result?.ok) {
    setCoordinationStatus(result?.error || 'НЕ УДАЛОСЬ ЗАПУСТИТЬ ТРАНСЛЯЦИЮ', true);
    renderCoordination();
    return;
  }

  state.coordination = result.state || state.coordination;
  setCoordinationStatus('ТРАНСЛЯЦИЯ ЗАПУЩЕНА');
  renderCoordination();
  renderTreeHeader();
});

btnEndBroadcast.addEventListener('click', () => {
  if (coordinationCommandPending || !state.coordination?.broadcast) return;
  showEndBroadcastConfirmation();
});

btnCancelEndBroadcast.addEventListener('click', () => hideEndBroadcastConfirmation());
endBroadcastDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!coordinationCommandPending) hideEndBroadcastConfirmation();
});

btnConfirmEndBroadcast.addEventListener('click', async () => {
  if (coordinationCommandPending || !state.coordination?.broadcast) return;
  btnCancelEndBroadcast.disabled = true;
  btnConfirmEndBroadcast.disabled = true;
  const result = await runCoordinationCommand(
    () => desktopAPI.endBroadcast(),
    'ТРАНСЛЯЦИЯ ЗАВЕРШЕНА · СЕССИИ И ПЕРСОНАЖИ СОХРАНЕНЫ',
    'ЗАВЕРШЕНИЕ ТРАНСЛЯЦИИ...'
  );
  if (!result?.ok) {
    hideEndBroadcastConfirmation();
    return;
  }
  if (!result.state || result.state.broadcast) {
    setCoordinationStatus('ЗАВЕРШЕНИЕ НЕ ПОДТВЕРЖДЕНО АВТОРИТЕТНЫМ СОСТОЯНИЕМ', true);
    renderCoordination();
    hideEndBroadcastConfirmation();
    return;
  }
  hideEndBroadcastConfirmation({ restoreFocus: false });
  hideTerminalSwitchDecision();
  state.liveHack = null;
  renderAll();
  btnStartBroadcast.focus();
});

// ── Terminal management ───────────────────────────────────────
function showCreateTerminalDialog() {
  if (!state.session || createTerminalSubmitting) return;
  createTerminalName.value = '';
  createTerminalError.textContent = '';
  createTerminalError.hidden = true;
  btnCancelCreateTerminal.disabled = false;
  btnConfirmCreateTerminal.disabled = false;
  createTerminalDialog.hidden = false;
  if (typeof createTerminalDialog.showModal === 'function' && !createTerminalDialog.open) {
    createTerminalDialog.showModal();
  } else {
    createTerminalDialog.setAttribute('open', '');
  }
  createTerminalName.focus();
}

function hideCreateTerminalDialog({ restoreFocus = true } = {}) {
  if (typeof createTerminalDialog.close === 'function' && createTerminalDialog.open) {
    createTerminalDialog.close();
  } else {
    createTerminalDialog.removeAttribute('open');
  }
  createTerminalDialog.hidden = true;
  createTerminalSubmitting = false;
  btnCancelCreateTerminal.disabled = false;
  btnConfirmCreateTerminal.disabled = false;
  if (restoreFocus && !btnAddTerminal.hidden) btnAddTerminal.focus();
}

btnAddTerminal.addEventListener('click', showCreateTerminalDialog);
btnReapplyEdit.addEventListener('click', reapplyHeldEdits);
for (const container of [nodeForm, termSettings, facilitySelectionEditor]) {
  container.addEventListener('input', () => stageUnappliedInput(container));
  container.addEventListener('change', () => stageUnappliedInput(container));
}
btnUndo.addEventListener('click', () => runHistoryEntry('undo'));
btnRedo.addEventListener('click', () => runHistoryEntry('redo'));

btnCancelCreateTerminal.addEventListener('click', () => {
  if (!createTerminalSubmitting) hideCreateTerminalDialog();
});

createTerminalDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!createTerminalSubmitting) hideCreateTerminalDialog();
});

createTerminalForm.addEventListener('submit', (event) => {
  event.preventDefault();
  if (!state.session || createTerminalSubmitting) return;
  const name = createTerminalName.value.trim();
  if (!name) {
    createTerminalError.textContent = 'УКАЖИТЕ НАЗВАНИЕ ТЕРМИНАЛА';
    createTerminalError.hidden = false;
    createTerminalName.focus();
    return;
  }

  createTerminalSubmitting = true;
  btnCancelCreateTerminal.disabled = true;
  btnConfirmCreateTerminal.disabled = true;
  const term = {
    id:        uid('t'),
    name,
    hackLevel: 0,
    introText: '',
    root:      { id: 'root', type: 'folder', name: 'ROOT', children: [] },
  };
  commitAuthoringEdit({
    label: `НОВЫЙ ТЕРМИНАЛ «${name}»`,
    mutate: session => {
      session.terminals.push(structuredClone(term));
      ensureSessionTerminalGroups(session);
    },
    reopen: reason => {
      if (state.editTerminalId === term.id) state.editTerminalId = state.session.terminals[0]?.id || null;
      showCreateTerminalDialog();
      createTerminalName.value = name;
      createTerminalError.textContent = reason;
      createTerminalError.hidden = false;
    },
  });
  state.editTerminalId = term.id;
  state.selectedNodeId = null;
  state.expanded = new Set(['root']);
  renderAll();
  hideCreateTerminalDialog({ restoreFocus: false });
  hackLevelSelect.focus();
});

btnApplySettings.addEventListener('click', () => {
  const term = getEditTerminal();
  if (!term) return;
  const hackLevel = Number(hackLevelSelect.value) || 0;
  const introText = introTextArea.value;
  // The save reaches players at once; a new hack level applies on the next
  // activation so it never disrupts an in-progress hack.
  commitTerminalEdit('НАСТРОЙКИ ТЕРМИНАЛА', term.id, target => {
    target.hackLevel = hackLevel;
    target.introText = introText;
  }, takeUnappliedInput(termSettings));
});

btnCancelSettings.addEventListener('click', () => {
  takeUnappliedInput(termSettings);
  renderSettingsPanel();
  renderSaveStatus();
});

function terminalActivationRequest(term) {
  return desktopAPI.requestTerminalActivation({
    terminalId: term.id,
    terminalName: term.name,
    tree: term.root,
    hackLevel: term.hackLevel || 0,
    introText: term.introText || '',
  });
}

btnMakeLive.addEventListener('click', async () => {
  const term = getEditTerminal();
  if (!term || term.id === state.liveTerminalId || !state.coordination?.broadcast || coordinationCommandPending) return;
  const result = await runTerminalSwitchRequest(
    () => terminalActivationRequest(term),
    'АКТИВНЫЙ ТЕРМИНАЛ ВЫБРАН',
    'ПЕРЕКЛЮЧЕНИЕ АКТИВНОГО ТЕРМИНАЛА...'
  );
  if (result?.ok && result.status === 'activated') state.liveHack = null;
  renderTermList();
  renderTreeHeader();
  renderHackStatus();
});

btnReapplySettings.addEventListener('click', async () => {
  const term = getEditTerminal();
  if (!term || term.id !== state.liveTerminalId || !state.coordination?.broadcast || coordinationCommandPending) return;
  terminalSettingsMenu.open = false;
  const result = await runTerminalSwitchRequest(
    () => terminalActivationRequest(term),
    'НАСТРОЙКИ АКТИВНОГО ТЕРМИНАЛА ПЕРЕПРИМЕНЕНЫ',
    'ПЕРЕПРИМЕНЕНИЕ НАСТРОЕК...'
  );
  if (result?.ok && result.status === 'activated') state.liveHack = null;
  renderTermList();
  renderTreeHeader();
  renderHackStatus();
});


btnStopBroadcast.addEventListener('click', showTakeOffAirConfirmation);

btnCancelTakeOffAir.addEventListener('click', () => {
  if (!takeOffAirPending) hideTakeOffAirConfirmation();
});

takeOffAirDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!takeOffAirPending) hideTakeOffAirConfirmation();
});

btnConfirmTakeOffAir.addEventListener('click', async () => {
  if (takeOffAirPending || coordinationCommandPending || !state.coordination?.broadcast || !state.liveTerminalId) return;
  takeOffAirPending = true;
  takeOffAirError.textContent = '';
  takeOffAirError.hidden = true;
  btnCancelTakeOffAir.disabled = true;
  btnConfirmTakeOffAir.disabled = true;
  const result = await runCoordinationCommand(
    () => desktopAPI.requestTerminalClear(),
    'АКТИВНЫЙ ТЕРМИНАЛ УБРАН · ТРАНСЛЯЦИЯ ПРОДОЛЖАЕТСЯ',
    'ОЧИСТКА АКТИВНОГО ТЕРМИНАЛА...'
  );
  takeOffAirPending = false;
  if (!result?.ok) {
    takeOffAirError.textContent = result?.error || 'НЕ УДАЛОСЬ СНЯТЬ ТЕРМИНАЛ С ЭФИРА';
    takeOffAirError.hidden = false;
    btnCancelTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.focus();
    return;
  }
  if (result.status === 'decision-required' && result.switchId) {
    hideTakeOffAirConfirmation({ restoreFocus: false });
    showTerminalSwitchDecision(result);
    return;
  }
  if (result.status !== 'cleared') {
    takeOffAirError.textContent = 'СНЯТИЕ С ЭФИРА НЕ ПОДТВЕРЖДЕНО';
    takeOffAirError.hidden = false;
    btnCancelTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.focus();
    return;
  }
  state.liveHack = null;
  hideTakeOffAirConfirmation({ restoreFocus: false });
  renderTermList();
  renderTreeHeader();
  renderHackStatus();
  if (!btnEndBroadcast.hidden) btnEndBroadcast.focus();
});

for (const button of terminalSwitchButtons) {
  button.addEventListener('click', async () => {
    if (!pendingTerminalSwitch || coordinationCommandPending) return;
    const decision = button.dataset.switchDecision;
    terminalSwitchButtons.forEach(control => { control.disabled = true; });
    terminalSwitchStatus.textContent = 'ПРИМЕНЕНИЕ РЕШЕНИЯ...';
    terminalSwitchError.hidden = true;
    const result = await runCoordinationCommand(
      () => desktopAPI.resolveTerminalSwitch({ switchId: pendingTerminalSwitch, decision }),
      decision === 'cancel' ? 'ПЕРЕКЛЮЧЕНИЕ ОТМЕНЕНО' : 'РЕШЕНИЕ ПРИМЕНЕНО',
      'ПРИМЕНЕНИЕ РЕШЕНИЯ...'
    );
    terminalSwitchButtons.forEach(control => { control.disabled = false; });
    if (!result?.ok) {
      terminalSwitchError.textContent = result?.error || 'РЕШЕНИЕ ОТКЛОНЕНО';
      terminalSwitchError.hidden = false;
      terminalSwitchStatus.textContent = 'ИСХОДНЫЙ ТЕРМИНАЛ ОСТАЁТСЯ АКТИВНЫМ';
      return;
    }
    if (result.status === 'activated' || result.status === 'cleared') state.liveHack = null;
    hideTerminalSwitchDecision();
    renderAll();
  });
}

btnHackSuccess.addEventListener('click', () => {
  if (!state.liveHack || state.liveHack.solved || state.liveHack.failed) return;
  desktopAPI.forceHackSuccess();
});

btnResetFailedHack.addEventListener('click', async () => {
  const term = state.session && state.liveTerminalId
    ? state.session.terminals.find(candidate => candidate.id === state.liveTerminalId)
    : null;
  if (!term || !state.liveHack?.failed || coordinationCommandPending) return;
  const result = await runCoordinationCommand(
    () => desktopAPI.resetFailedHack({
      terminalId: term.id,
      terminalName: term.name,
      tree: term.root,
      hackLevel: term.hackLevel || 0,
      introText: term.introText || '',
    }),
    'СОЗДАНА НОВАЯ ГОЛОВОЛОМКА',
    'ПОДГОТОВКА НОВОЙ ГОЛОВОЛОМКИ...'
  );
  if (!result?.ok) renderHackStatus();
});
