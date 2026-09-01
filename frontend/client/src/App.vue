<script setup lang="ts">
import { create } from '@bufbuild/protobuf';
import { computed, onMounted, onScopeDispose, ref, watch } from 'vue';

import {
  PlayerNoticeKind,
  PresentationIntentSchema,
  type SubscriptionMessage,
} from '../gen/fallout/terminal/player/v1/player_pb.js';
import {
  NavigationMode,
} from '../gen/fallout/terminal/player/v1/navigation_pb.js';
import {
  CommandExecutionPhase,
  ContentNodeSchema,
  type ContentNode,
  TerminalNavigationDirection,
} from '../gen/fallout/terminal/player/v1/terminal_pb.js';
import { createPresentationUplinkTransport } from './adapters/presentation-uplink-transport.js';
import type { PlayerRPCAdapter } from './adapters/player-rpc.js';
import type { RecognitionStorageAdapter } from './adapters/recognition-storage.js';
import { createSoundManifestAdapter } from './adapters/sound-manifest.js';
import type { SoundFolder } from './adapters/sound-manifest.js';
import AssignedWaiting from './components/AssignedWaiting.vue';
import CharacterSelection from './components/CharacterSelection.vue';
import ConnectionOverlay from './components/ConnectionOverlay.vue';
import CrtShell from './components/CrtShell.vue';
import HackingAttempts from './components/HackingAttempts.vue';
import HackingInputPreview from './components/HackingInputPreview.vue';
import HackingLog from './components/HackingLog.vue';
import HackingSurface from './components/HackingSurface.vue';
import PlayerNotice from './components/PlayerNotice.vue';
import PlayerStatusLine from './components/PlayerStatusLine.vue';
import TerminalChrome from './components/TerminalChrome.vue';
import TerminalFooter from './components/TerminalFooter.vue';
import TerminalRecord from './components/TerminalRecord.vue';
import TerminalSurface from './components/TerminalSurface.vue';
import { useConnectionOverlay } from './composables/useConnectionOverlay.js';
import { useHackingSession, type HackingAction } from './composables/useHackingSession.js';
import type { HackingPointerTarget } from './composables/useHackingPointer.js';
import { usePresentationUplink } from './composables/usePresentationUplink.js';
import { usePlayerActions } from './composables/usePlayerActions.js';
import { usePlayerAuthority } from './composables/usePlayerAuthority.js';
import { usePlayerIdentity } from './composables/usePlayerIdentity.js';
import { usePlayerProjection } from './composables/usePlayerProjection.js';
import { usePlayerSubscription } from './composables/usePlayerSubscription.js';
import { useRecognitionLease } from './composables/useRecognitionLease.js';
import { useTerminalKeyboard } from './composables/useTerminalKeyboard.js';
import { useTerminalNavigation } from './composables/useTerminalNavigation.js';
import { useTerminalSound, type TerminalSoundCue } from './composables/useTerminalSound.js';
import { useTypewriterReveal } from './composables/useTypewriterReveal.js';

const props = defineProps<{
  readonly clientInstanceID: string;
  readonly rpc: PlayerRPCAdapter;
  readonly storage: RecognitionStorageAdapter;
}>();

const projection = usePlayerProjection();
const identity = usePlayerIdentity();
const authority = usePlayerAuthority();
const navigation = useTerminalNavigation();
const actions = usePlayerActions({
  authorize: () => authority.state.value.canControl || identity.state.value?.phase === 'selecting',
  rpc: props.rpc,
});
const hacking = useHackingSession({
  authorize: contextKey => authority.state.value.canControl && authority.state.value.contextKey === contextKey,
  broadcastID: () => projection.state.value.playerState?.broadcastId ?? '',
  recognitionHandle: () => identity.state.value?.recognitionHandle ?? '',
  rpc: props.rpc,
  terminalID: () => projection.state.value.liveTerminal?.terminalId ?? '',
});
const soundManifest = createSoundManifestAdapter(props.rpc);
const sound = useTerminalSound({ manifest: soundManifest });
const typewriter = useTypewriterReveal({ onCue: () => { void sound.play('charscroll'); } });
let pendingPresentationCue: TerminalSoundCue | '' = '';
let pendingPresentationTargetKey = '';
let presentationCueTimer: number | null = null;
let presentationAwaitingResult = false;
let lastPresentationCue: TerminalSoundCue | '' = '';
let lastPresentationCueAt = Number.NEGATIVE_INFINITY;
const uplink = usePresentationUplink({
  authorize: contextKey =>
    identity.state.value?.phase === 'selecting' ||
    (authority.state.value.canControl && authority.state.value.contextKey === contextKey),
  clientInstanceID: props.clientInstanceID,
  onResult: result => {
    actions.applyRevision(Number(result.revision));
  },
  recognitionHandle: () => identity.state.value?.recognitionHandle ?? '',
  transport: createPresentationUplinkTransport(props.rpc),
});
const typedHackingInput = ref('');
const localHackingHighlight = ref('');
const localHackingPreview = ref('');
const localMenuSelection = ref('');
const recordPageCount = ref(1);
const recordPageIndex = ref(0);
let ambientActive = false;
let hackingPreviewFrame: number | null = null;
let pendingHackingTarget: Readonly<HackingPointerTarget> | null = null;
let lastCommandNodeID: string | null = null;
let lastCommandExecutionNodeID: string | null = null;
let hadCommandExecution = false;
let hadPendingTerminalReturn = false;
let navigationTerminalID = '';
let retainSolvedHackingHeader = false;
let suppressedMenuPresentationID = '';
let returnTargetBroadcastID = '';
let returnTargetTerminalID = '';
let resetMenuSelectionAfterBack = false;
const retainedReturnTarget = ref<Readonly<{ terminalID: string; terminalName: string }> | null>(null);
let uplinkContext = '';

const schedulePresentationCue = (cue: TerminalSoundCue): void => {
  if (presentationCueTimer !== null) clearTimeout(presentationCueTimer);
  pendingPresentationCue = cue;
  presentationCueTimer = globalThis.setTimeout(() => {
    presentationCueTimer = null;
    const current = pendingPresentationCue;
    pendingPresentationCue = '';
    presentationAwaitingResult = false;
    localHackingHighlight.value = pendingHackingTarget?.key ?? localHackingHighlight.value;
    localHackingPreview.value = pendingHackingTarget?.text ?? localHackingPreview.value;
    const now = performance.now();
    if (current !== '' && (current !== lastPresentationCue || now - lastPresentationCueAt >= 500)) {
      lastPresentationCue = current;
      lastPresentationCueAt = now;
      void sound.play(current);
    }
  }, 105);
};

useTerminalKeyboard({
  authorize: contextKey => authority.state.value.canControl && authority.state.value.contextKey === contextKey,
  contextKey: () => authority.state.value.contextKey,
  onActivate: () => {
    if (hackingState.value !== null) return;
    if (activeRecordNode.value !== null) {
      if (liveTerminal.value?.commandExecution?.phase !== CommandExecutionPhase.PENDING &&
          liveTerminal.value?.terminalNavigation?.pending === undefined) navigateBack();
      return;
    }
    const selected = menuNodes.value.find(node => node.id === menuSelection.value) ?? menuNodes.value[0];
    if (selected !== undefined) activateNode(selected);
  },
  onBack: () => { navigateBack(); },
  onMenuIndex: index => {
    const node = menuNodes.value[index];
    if (node !== undefined) presentMenuNode(node, true);
  },
  onPageIndex: index => { presentRecordPage(index); },
  onTyped: value => { typedHackingInput.value = value; },
  state: () => identity.state.value === null || projection.state.value.liveTerminal === null ? null : ({
      blocked: actions.state.value.pending !== null ||
        liveTerminal.value?.commandExecution?.phase === CommandExecutionPhase.PENDING ||
        liveTerminal.value?.terminalNavigation?.pending !== undefined,
      contextKey: authority.state.value.contextKey,
      hackingComplete: hacking.state.value?.hack.solved === true || hacking.state.value?.hack.failed === true,
      menuCount: menuNodes.value.length,
      menuIndex: Math.max(0, menuNodes.value.findIndex(node => node.id === menuSelection.value)),
      mode: hacking.state.value !== null ? 'hacking' : activeRecordNode.value === null ? 'menu' : 'entry',
      pageCount: recordPageCount.value,
      pageIndex: recordPageIndex.value,
      typed: typedHackingInput.value,
    }),
});

const applyAuthority = (): void => {
  const currentIdentity = identity.state.value;
  if (currentIdentity === null) return;
  const playerState = projection.state.value.playerState;
  const live = projection.state.value.liveTerminal;
  authority.apply({
    activeTerminalID: playerState?.activeTerminalId ?? null,
    broadcastID: playerState?.broadcastId ?? null,
    contextKey: live?.controllerPresentation?.contextKey ?? '',
    identity: currentIdentity,
    presentedTerminalID: live?.terminalId ?? null,
  });
};

const syncTransientOwners = (): void => {
  const live = projection.state.value.liveTerminal;
  const contextKey = live?.controllerPresentation?.contextKey ?? '';
  const ownsUplink = authority.state.value.canControl || identity.state.value?.phase === 'selecting';
  const ownedUplinkContext = identity.state.value?.phase === 'selecting'
    ? contextKey || `selection:${projection.state.value.playerState?.broadcastId ?? 'pending'}`
    : contextKey;
  if (live?.hacking !== undefined && contextKey !== '') {
    const previous = hacking.state.value?.hack;
    const previousContext = hacking.state.value?.contextKey;
    const hadPendingHackingWork = hacking.state.value?.pending !== null || presentationAwaitingResult ||
      pendingPresentationTargetKey !== '';
    if (hacking.apply(live.hacking, projection.state.value.revision, contextKey)) {
      const next = hacking.state.value?.hack;
      const changedContext = previousContext !== undefined && previousContext !== contextKey;
      const changedGeneration = previous?.patterns.map(pattern => pattern.id).sort().join('|') !==
        next?.patterns.map(pattern => pattern.id).sort().join('|');
      if (changedContext || changedGeneration) {
        retainSolvedHackingHeader = false;
        localHackingHighlight.value = '';
        localHackingPreview.value = '';
      }
      if (previous?.solved !== true && next?.solved === true) {
        retainSolvedHackingHeader = hadPendingHackingWork;
      }
      if (previous?.failed !== true && next?.failed === true) void sound.play('hack-bad', projection.state.value.revision);
      if (previous?.solved !== true && next?.solved === true) void sound.play('hack-good', projection.state.value.revision);
    }
  } else if (hacking.clear(projection.state.value.revision)) {
    retainSolvedHackingHeader = false;
    localHackingHighlight.value = '';
    localHackingPreview.value = '';
  }
  if (ownsUplink && ownedUplinkContext !== '' && ownedUplinkContext !== uplinkContext &&
      uplink.start(ownedUplinkContext)) {
    uplinkContext = ownedUplinkContext;
  } else if (!ownsUplink && uplinkContext !== '') {
    uplink.stop();
    uplinkContext = '';
  }
  const nextAmbient = live !== null;
  if (nextAmbient !== ambientActive) {
    ambientActive = nextAmbient;
    void sound.setAmbientActive(nextAmbient);
  }
};

const applyMessage = (message: SubscriptionMessage): void => {
  if (message.payload.case === 'presentationUplinkResult') {
    uplink.apply(message.payload.value);
    return;
  }
  if (message.payload.case === 'snapshot') {
    const snapshot = message.payload.value;
    if (!projection.applySnapshot(snapshot)) return;
    props.storage.writeRecognitionHandle(snapshot.recognitionHandle);
    const revision = projection.state.value.revision;
    if (snapshot.playerState !== undefined) identity.apply(snapshot.recognitionHandle, revision, snapshot.playerState);
    actions.applyRevision(revision);
  } else if (message.payload.case === 'update') {
    const update = message.payload.value;
    if (!projection.applyUpdate(update)) return;
    const handle = props.storage.readRecognitionHandle();
    const revision = projection.state.value.revision;
    if (handle !== null && update.playerState !== undefined) identity.apply(handle, revision, update.playerState);
    actions.applyRevision(revision);
  } else {
    return;
  }

  applyAuthority();
  const projectedTerminal = projection.state.value.liveTerminal;
  const projectedBroadcastID = projection.state.value.playerState?.broadcastId ?? '';
  if (projectedBroadcastID !== returnTargetBroadcastID) {
    returnTargetBroadcastID = projectedBroadcastID;
    retainedReturnTarget.value = null;
  }
  const selectingCharacter = identity.state.value?.phase === 'selecting';
  if (selectingCharacter) retainedReturnTarget.value = null;
  if (projectedTerminal?.terminalId !== returnTargetTerminalID) {
    returnTargetTerminalID = projectedTerminal?.terminalId ?? '';
    retainedReturnTarget.value = null;
  }
  if (!selectingCharacter && projectedTerminal?.terminalNavigation?.returnTarget !== undefined) {
    retainedReturnTarget.value = Object.freeze({
      terminalID: projectedTerminal.terminalNavigation.returnTarget.terminalId,
      terminalName: projectedTerminal.terminalNavigation.returnTarget.terminalName,
    });
  } else if (!selectingCharacter &&
      projectedTerminal?.terminalNavigation?.pending?.direction === TerminalNavigationDirection.RETURN) {
    retainedReturnTarget.value = Object.freeze({
      terminalID: projectedTerminal.terminalNavigation.pending.targetTerminalId,
      terminalName: projectedTerminal.terminalNavigation.pending.targetTerminalName,
    });
  } else if (projectedTerminal?.terminalNavigation?.routeDepth === 0) {
    retainedReturnTarget.value = null;
  }
  const presentation = projectedTerminal?.controllerPresentation?.presentation;
  const presentationTargetKey = presentation?.case === 'menu'
    ? `menu:${presentation.value.targetId}`
    : presentation?.case === 'hacking'
      ? `hacking:${presentation.value.target.case}:${presentation.value.target.value}`
      : presentation?.case === 'page'
        ? `page:${presentation.value.pageIndex}`
      : '';
  if (presentationAwaitingResult && presentationTargetKey !== '' &&
      presentationTargetKey === pendingPresentationTargetKey) {
    pendingPresentationTargetKey = '';
    presentationAwaitingResult = false;
    localHackingHighlight.value = '';
    localHackingPreview.value = '';
  }
  if (projectedTerminal?.commandExecution !== undefined || projectedTerminal?.terminalNavigation?.pending !== undefined) {
    localMenuSelection.value = '';
  }
  if (!authority.state.value.canControl) {
    localMenuSelection.value = '';
    localHackingHighlight.value = '';
    localHackingPreview.value = '';
  } else if (presentation?.case === 'menu' && presentation.value.targetId === localMenuSelection.value) {
    localMenuSelection.value = '';
  }
  if (presentation?.case === 'page') recordPageIndex.value = presentation.value.pageIndex;
  const liveNavigation = projection.state.value.liveTerminal?.navigation;
  if (liveNavigation !== undefined) {
    const terminalID = projectedTerminal?.terminalId ?? '';
    const ordinaryTerminalSwitch = navigationTerminalID !== '' && terminalID !== navigationTerminalID &&
      (projectedTerminal?.terminalNavigation?.routeDepth ?? 0) === 0 && !hadPendingTerminalReturn;
    navigation.apply(ordinaryTerminalSwitch && projectedTerminal?.tree !== undefined
      ? {
          ...liveNavigation,
          commandNodeId: undefined,
          mode: NavigationMode.LIST,
          path: [projectedTerminal.tree.id],
          viewEntryId: undefined,
        }
      : liveNavigation, projection.state.value.revision, actions.state.value.pending !== null);
    navigationTerminalID = terminalID;
  }
  hadPendingTerminalReturn = projectedTerminal?.terminalNavigation?.pending?.direction ===
    TerminalNavigationDirection.RETURN;
  const commandNodeID = navigation.state.value?.commandNodeID ?? null;
  const hasCommandExecution = projectedTerminal?.commandExecution !== undefined;
  if ((lastCommandNodeID !== null && commandNodeID === null) || (hadCommandExecution && !hasCommandExecution)) {
    suppressedMenuPresentationID = lastCommandNodeID ?? lastCommandExecutionNodeID ??
      (presentation?.case === 'menu' ? presentation.value.targetId : '');
    localMenuSelection.value = menuNodes.value[0]?.id ?? '';
  }
  if (hasCommandExecution) suppressedMenuPresentationID = '';
  if (resetMenuSelectionAfterBack && projectedTerminal?.commandExecution === undefined &&
      projectedTerminal?.terminalNavigation?.pending === undefined && activeRecordNode.value === null) {
    localMenuSelection.value = menuNodes.value[0]?.id ?? '';
    resetMenuSelectionAfterBack = false;
  }
  lastCommandNodeID = commandNodeID;
  lastCommandExecutionNodeID = projectedTerminal?.commandExecution?.commandNodeId ?? lastCommandExecutionNodeID;
  hadCommandExecution = hasCommandExecution;
  syncTransientOwners();
  syncReveal();
};

const subscriptionInput = (): { clientInstanceId: string; recognitionHandle?: string } => {
  const recognitionHandle = props.storage.readRecognitionHandle();
  return recognitionHandle === null
    ? { clientInstanceId: props.clientInstanceID }
    : { clientInstanceId: props.clientInstanceID, recognitionHandle };
};
const subscription = usePlayerSubscription({
  onMessage: applyMessage,
  reconnectInput: subscriptionInput,
  rpc: props.rpc,
});
const overlay = useConnectionOverlay();
watch(subscription.state, next => { overlay.apply(next); }, { immediate: true });

const lease = useRecognitionLease({
  startSubscription: async recognitionHandle => {
    const input = recognitionHandle === '' ? subscriptionInput() : {
      clientInstanceId: props.clientInstanceID,
      recognitionHandle,
    };
    await subscription.start(input);
  },
  storage: props.storage,
});

const liveTerminal = computed(() => projection.state.value.liveTerminal);
const currentFolder = computed<ContentNode | null>(() => {
  const tree = liveTerminal.value?.tree;
  if (tree === undefined) return null;
  let current = tree;
  for (const segment of navigation.state.value?.path ?? []) {
    if (segment === current.id) continue;
    if (current.content.case !== 'folder') return null;
    const child = current.content.value.children.find(node => node.id === segment);
    if (child === undefined) return null;
    current = child;
  }
  return current.content.case === 'folder' ? current : null;
});
const menuNodes = computed<readonly ContentNode[]>(() => currentFolder.value?.content.case === 'folder'
  ? currentFolder.value.content.value.children
  : []);
const findContentNode = (node: ContentNode | undefined, id: string): ContentNode | null => {
  if (node === undefined) return null;
  if (node.id === id) return node;
  if (node.content.case !== 'folder') return null;
  for (const child of node.content.value.children) {
    const match = findContentNode(child, id);
    if (match !== null) return match;
  }
  return null;
};
const activeRecordNode = computed<ContentNode | null>(() => {
  const executionID = liveTerminal.value?.commandExecution?.commandNodeId;
  if (executionID !== undefined) return findContentNode(liveTerminal.value?.tree, executionID);
  if (liveTerminal.value?.terminalNavigation?.pending !== undefined) {
    return findContentNode(liveTerminal.value?.tree, menuSelection.value) ?? create(ContentNodeSchema, {
      content: { case: 'command', value: { text: '' } },
      id: '__terminal-navigation-pending',
      name: liveTerminal.value.terminalNavigation?.pending?.targetTerminalName || 'ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ',
    });
  }
  const navigationState = navigation.state.value;
  if (navigationState === null) return null;
  const id = navigationState.mode === 'entry'
    ? navigationState.viewEntryID
    : navigationState.commandNodeID;
  return id === null ? null : findContentNode(liveTerminal.value?.tree, id);
});
const recordText = computed(() => {
  const node = activeRecordNode.value;
  if (node === null) return '';
  const execution = liveTerminal.value?.commandExecution;
  if (liveTerminal.value?.terminalNavigation?.pending !== undefined) return 'Выполняется запрос';
  if (execution?.commandNodeId === node.id && execution.phase === CommandExecutionPhase.PENDING) {
    return 'Выполняется запрос';
  }
  if (execution?.commandNodeId === node.id && execution.phase === CommandExecutionPhase.REJECTED) {
    return 'Ошибка доступа';
  }
  if (node.content.case === 'entry') return node.content.value.description;
  if (node.content.case === 'command') return node.content.value.text;
  return '';
});
watch(() => activeRecordNode.value?.id ?? '', () => {
  recordPageIndex.value = 0;
});
const showBack = computed(() => {
  if (liveTerminal.value?.commandExecution?.phase === CommandExecutionPhase.PENDING ||
      liveTerminal.value?.terminalNavigation?.pending !== undefined) return false;
  if (activeRecordNode.value === null && retainedReturnTarget.value !== null) return true;
  if (activeRecordNode.value?.content.case === 'entry') return true;
  const rootID = liveTerminal.value?.tree?.id;
  const insideFolder = navigation.state.value?.path.some(segment => segment !== rootID) === true;
  if (activeRecordNode.value === null && insideFolder) return true;
  if (!authority.state.value.canControl) return false;
  return activeRecordNode.value !== null;
});
const backLabel = computed(() => retainedReturnTarget.value !== null && activeRecordNode.value === null
  ? `[ НАЗАД В ${retainedReturnTarget.value.terminalName} ]`
  : '[ НАЗАД ]');
const hackingState = computed(() => liveTerminal.value?.hacking === undefined || liveTerminal.value.hacking.solved
  ? null
  : hacking.state.value);
const hackingHeaderState = computed(() => liveTerminal.value?.hacking === undefined ||
  (liveTerminal.value.hacking.solved && !retainSolvedHackingHeader)
  ? null
  : hacking.state.value);
const hackingGenerationKey = computed(() => hackingState.value?.hack.patterns
  .map(pattern => pattern.id).sort().join('|') ?? '');
const hackingContextKey = computed(() => liveTerminal.value?.controllerPresentation?.contextKey ?? '');
const authoritativeHackingHighlight = computed(() => {
  const presentation = liveTerminal.value?.controllerPresentation?.presentation;
  if (presentation?.case !== 'hacking') return '';
  const target = presentation.value.target;
  if (target.case === 'patternId') return `pattern:${target.value}`;
  if (target.case !== 'targetId') return '';
  return target.value.includes(':') ? `filler:${target.value}` : `word:${target.value}`;
});
const hackingHighlight = computed(() => localHackingHighlight.value || authoritativeHackingHighlight.value);
const authoritativeHackingPreview = computed(() => {
  const state = hackingState.value?.hack;
  const presentation = liveTerminal.value?.controllerPresentation?.presentation;
  if (state === undefined || presentation?.case !== 'hacking') return '';
  const target = presentation.value.target;
  if (target.case === 'patternId') {
    const pattern = state.patterns.find(candidate => candidate.id === target.value);
    if (pattern === undefined) return '';
    return state.columns.flatMap(column => [...column.text])
      .slice(pattern.row * 12 + pattern.start, pattern.row * 12 + pattern.end + 1).join('');
  }
  if (target.case !== 'targetId') return '';
  const parts = target.value.split(':');
  if (parts.length === 2) {
    const column = Number(parts[0]);
    const character = Number(parts[1]);
    return state.columns[column]?.text[character] ?? '';
  }
  for (const column of state.columns) {
    const word = column.words.find(candidate => candidate.id === target.value);
    if (word !== undefined) return column.text.slice(word.start, word.start + word.length);
  }
  return '';
});
const menuSelection = computed(() => {
  if (localMenuSelection.value !== '') {
    return localMenuSelection.value === suppressedMenuPresentationID
      ? menuNodes.value[0]?.id ?? ''
      : localMenuSelection.value;
  }
  const presentation = liveTerminal.value?.controllerPresentation?.presentation;
  if (presentation?.case !== 'menu') return '';
  return presentation.value.targetId === suppressedMenuPresentationID
    ? menuNodes.value[0]?.id ?? ''
    : presentation.value.targetId;
});

const syncReveal = (): void => {
  const live = liveTerminal.value;
  const phase = identity.state.value?.phase;
  if (live === null || (phase !== 'controlling' && phase !== 'observing')) {
    typewriter.cancel();
    return;
  }
  const hack = hackingState.value?.hack;
  if (hack !== undefined) {
    const generation = hack.patterns.map(pattern => pattern.id).sort().join('|');
    const total = hack.columns.reduce((count, column) => count + Math.ceil(column.text.length / 12), 0);
    typewriter.start(`hack:${live.terminalId}:${generation}`, total, true);
    return;
  }
  const record = activeRecordNode.value;
  if (record !== null) {
    const text = recordText.value.replace(/\r\n?/gu, '\n');
    const phase = live.commandExecution?.phase ?? (live.terminalNavigation?.pending === undefined ? 0 : 1);
    const animate = navigation.state.value?.commandNodeID === null || live.commandExecution !== undefined;
    typewriter.start(
      `record:${live.terminalId}:${record.id}:${phase}:${text}`,
      text.split('\n').length,
      animate,
    );
    return;
  }
  const folder = currentFolder.value;
  if (folder === null) {
    typewriter.cancel();
    return;
  }
  const contentIdentity = menuNodes.value.map(node => `${node.id}\u0000${node.name}`).join('\u0001');
  typewriter.start(`menu:${live.terminalId}:${folder.id}:${contentIdentity}`, menuNodes.value.length, true);
};

const requestID = (): string => typeof globalThis.crypto?.randomUUID === 'function'
  ? globalThis.crypto.randomUUID()
  : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;

const presentHackingTarget = (target: Readonly<HackingPointerTarget> | null): void => {
  const updatePreview = !presentationAwaitingResult;
  pendingHackingTarget = target;
  if (hackingPreviewFrame === null) {
    hackingPreviewFrame = requestAnimationFrame(() => {
      hackingPreviewFrame = null;
      localHackingHighlight.value = pendingHackingTarget?.key ?? '';
      if (updatePreview) localHackingPreview.value = pendingHackingTarget?.text ?? '';
    });
  }
  const playerState = projection.state.value.playerState;
  const live = liveTerminal.value;
  const currentIdentity = identity.state.value;
  const contextKey = hackingContextKey.value;
  if (target === null || playerState?.broadcastId === undefined || live === null || currentIdentity === null || contextKey === '') return;
  const value = target.action.kind === 'pattern' ? target.action.patternID
    : target.action.kind === 'word' ? target.action.wordID : `${target.action.column}:${target.action.character}`;
  const intent = create(PresentationIntentSchema, {
    broadcastId: playerState.broadcastId,
    contextKey,
    presentation: {
      contextKey,
      presentation: {
        case: 'hacking',
        value: { target: { case: target.action.kind === 'pattern' ? 'patternId' : 'targetId', value } },
      },
    },
    recognitionHandle: currentIdentity.recognitionHandle,
    requestId: requestID(),
    terminalId: live.terminalId,
  });
  if (uplink.offer(intent)) {
    presentationAwaitingResult = true;
    schedulePresentationCue(target.action.kind === 'filler' ? 'single' : 'multiple');
    pendingPresentationTargetKey = `hacking:${target.action.kind === 'pattern' ? 'patternId' : 'targetId'}:${value}`;
  }
};

const presentMenuNode = (node: ContentNode, allowSuppressed = false): void => {
  if (!authority.state.value.canControl || menuSelection.value === node.id) return;
  if (node.id === suppressedMenuPresentationID && !allowSuppressed) return;
  suppressedMenuPresentationID = '';
  localMenuSelection.value = node.id;
  const playerState = projection.state.value.playerState;
  const live = liveTerminal.value;
  const currentIdentity = identity.state.value;
  const contextKey = live?.controllerPresentation?.contextKey ?? '';
  if (playerState?.broadcastId === undefined || live === null || currentIdentity === null || contextKey === '') return;
  if (uplink.offer(create(PresentationIntentSchema, {
    broadcastId: playerState.broadcastId,
    contextKey,
    presentation: {
      contextKey,
      presentation: { case: 'menu', value: { targetId: node.id } },
    },
    recognitionHandle: currentIdentity.recognitionHandle,
    requestId: requestID(),
    terminalId: live.terminalId,
  }))) {
    presentationAwaitingResult = true;
    schedulePresentationCue('menu-focus');
    pendingPresentationTargetKey = `menu:${node.id}`;
  }
};

const navigateBack = (): void => {
  const current = identity.state.value;
  const playerState = projection.state.value.playerState;
  const live = liveTerminal.value;
  if (current === null || playerState?.broadcastId === undefined || live === null) return;
  resetMenuSelectionAfterBack = activeRecordNode.value !== null;
  localMenuSelection.value = menuNodes.value[0]?.id ?? '';
  void actions.begin({
    input: {
      action: { case: 'back', value: {} },
      broadcastId: playerState.broadcastId,
      recognitionHandle: current.recognitionHandle,
      terminalId: live.terminalId,
    },
    kind: 'navigate',
  });
};

const presentRecordPage = (pageIndex: number): void => {
  if (!Number.isSafeInteger(pageIndex) || pageIndex < 0 || pageIndex >= recordPageCount.value) return;
  typewriter.complete();
  recordPageIndex.value = pageIndex;
  const playerState = projection.state.value.playerState;
  const live = liveTerminal.value;
  const currentIdentity = identity.state.value;
  const contextKey = live?.controllerPresentation?.contextKey ?? '';
  if (playerState?.broadcastId === undefined || live === null || currentIdentity === null || contextKey === '') return;
  if (uplink.offer(create(PresentationIntentSchema, {
    broadcastId: playerState.broadcastId,
    contextKey,
    presentation: {
      contextKey,
      presentation: { case: 'page', value: { pageIndex } },
    },
    recognitionHandle: currentIdentity.recognitionHandle,
    requestId: requestID(),
    terminalId: live.terminalId,
  }))) {
    presentationAwaitingResult = true;
    schedulePresentationCue('menu-focus');
    pendingPresentationTargetKey = `page:${pageIndex}`;
  }
};

const activateHackingTarget = (action: HackingAction): void => {
  void hacking.begin(action, hackingContextKey.value).then(accepted => {
    if (!accepted) return;
    pendingHackingTarget = null;
    localHackingHighlight.value = '';
    localHackingPreview.value = '';
    void sound.play('enter');
  });
};

const selectCharacter = (characterID: string): void => {
  const current = identity.state.value;
  const playerState = projection.state.value.playerState;
  if (current === null || playerState?.broadcastId === undefined) return;
  void actions.begin({
    input: {
      broadcastId: playerState.broadcastId,
      characterId: characterID,
      recognitionHandle: current.recognitionHandle,
    },
    kind: 'selectCharacter',
  });
};

const activateNode = (node: ContentNode): void => {
  const current = identity.state.value;
  const playerState = projection.state.value.playerState;
  const live = liveTerminal.value;
  if (current === null || playerState?.broadcastId === undefined || live === null) return;
  suppressedMenuPresentationID = '';
  const action = node.content.case === 'folder'
    ? { case: 'enter' as const, value: { nodeId: node.id } }
    : node.content.case === 'command'
      ? { case: 'command' as const, value: { nodeId: node.id } }
      : { case: 'entry' as const, value: { nodeId: node.id } };
  void actions.begin({
    input: {
      action,
      broadcastId: playerState.broadcastId,
      recognitionHandle: current.recognitionHandle,
      terminalId: live.terminalId,
    },
    kind: 'navigate',
  });
};

const playerNotice = computed(() => {
  if (projection.state.value.playerState?.notice?.kind === PlayerNoticeKind.COMMAND_PERSISTENCE_FAILED) {
    return 'Не удалось сохранить результат; состояние команды не изменено';
  }
  return actions.state.value.error;
});

onMounted(() => {
  const folders: readonly SoundFolder[] = [
    'ambient', 'charscroll', 'enter', 'hack-bad', 'hack-good', 'menu-focus', 'multiple', 'single',
  ];
  for (const folder of folders) void soundManifest.load(folder);
  void lease.start().catch(() => undefined);
});
onScopeDispose(() => {
  if (hackingPreviewFrame !== null) cancelAnimationFrame(hackingPreviewFrame);
  if (presentationCueTimer !== null) clearTimeout(presentationCueTimer);
});
</script>

<template>
  <CrtShell
    :context-key="hackingContextKey"
    :observer-read-only="authority.state.value.observerReadOnly"
    :pending="actions.state.value.pending !== null"
    :revealing="typewriter.state.value.phase === 'revealing'"
  >
    <TerminalChrome
      :intro-text="liveTerminal?.introText ?? ''"
      :visible="liveTerminal !== null && hackingState === null"
    />
    <header id="hackHeader" class="term-header hack-header" :hidden="hackingHeaderState === null">
      <div class="hdr-center">
        <div>ROBCO INDUSTRIES (TM) TERMLINK PROTOCOL</div>
        <div>ВВЕДИТЕ ПАРОЛЬ</div>
        <HackingAttempts
          :attempts-left="hackingHeaderState?.hack.attemptsLeft ?? 0"
          :attempts-max="hackingHeaderState?.hack.attemptsMax ?? 0"
        />
      </div>
    </header>
    <main id="termBody" class="term-body">
      <CharacterSelection
        v-if="identity.state.value !== null"
        v-show="identity.state.value.phase === 'selecting'"
        :pending="actions.state.value.pending?.kind === 'selectCharacter'"
        :roster="identity.state.value.roster"
        @select="selectCharacter"
      />
      <AssignedWaiting v-if="identity.state.value?.phase === 'waiting'" />
      <section v-else id="assignedWaiting" hidden />
      <PlayerNotice :message="playerNotice" />
      <HackingSurface
        :can-control="authority.state.value.canControl && hackingState?.pending === null"
        :context-key="hackingContextKey"
        :generation-key="hackingGenerationKey"
        :hack="hackingState?.hack ?? null"
        :highlighted-key="hackingHighlight"
        :visible-rows="typewriter.state.value.visible"
        @activate="activateHackingTarget"
        @preview="presentHackingTarget"
      >
        <template v-if="hackingState !== null">
          <div class="hack-log-panel">
            <HackingLog :lines="hackingState.hack.log" />
            <HackingInputPreview
              :blocked="hackingState.pending !== null"
              :hover-text="authoritativeHackingPreview || localHackingPreview"
              :typed="typedHackingInput"
            />
          </div>
        </template>
      </HackingSurface>
      <TerminalSurface
        v-if="hackingState === null && liveTerminal !== null && activeRecordNode === null"
        :can-control="authority.state.value.canControl"
        :nodes="menuNodes"
        :observer-read-only="authority.state.value.observerReadOnly"
        :pending="actions.state.value.pending !== null"
        :selected-i-d="menuSelection"
        :visible-count="typewriter.state.value.visible"
        @activate="activateNode"
        @preview="presentMenuNode"
      />
      <div v-else id="termList" hidden />
      <TerminalRecord
        v-if="hackingState === null && activeRecordNode !== null"
        :page-index="recordPageIndex"
        :pending="liveTerminal?.commandExecution?.phase === CommandExecutionPhase.PENDING"
        :text="recordText"
        :title="activeRecordNode.name"
        :visible-lines="typewriter.state.value.visible"
        @page-count="recordPageCount = $event"
      />
      <section v-else id="termEntry" hidden><div id="entryBody" /></section>
      <div
        id="termIdle"
        class="term-idle"
        :hidden="liveTerminal !== null || identity.state.value?.phase === 'selecting' || identity.state.value?.phase === 'waiting'"
      >
        <div>ОЖИДАНИЕ ТРАНСЛЯЦИИ<span class="blink">_</span></div>
      </div>
    </main>
    <TerminalFooter
      :back-label="backLabel"
      :can-control="authority.state.value.canControl && actions.state.value.pending === null"
      :page-count="recordPageCount"
      :page-index="recordPageIndex"
      :show-back="showBack"
      @back="navigateBack"
      @page="presentRecordPage"
    />
    <div id="termOutput" class="term-output" hidden />
    <PlayerStatusLine v-if="identity.state.value !== null" :identity="identity.state.value" />
    <div id="termPrompt" class="term-prompt" :hidden="liveTerminal === null || hackingState !== null">
      <span>&gt; </span><span class="blink">_</span>
    </div>
  </CrtShell>
  <ConnectionOverlay
    :message="overlay.state.value.message"
    :visible="overlay.state.value.visible"
  />
</template>
