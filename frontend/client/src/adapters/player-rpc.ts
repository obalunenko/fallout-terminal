import {
  create,
  toBinary,
  type DescMessage,
  type MessageInitShape,
  type MessageShape,
} from '@bufbuild/protobuf';
import { createClient, type CallOptions, type Client, type Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

import { ActivatePatternRequestSchema, GuessRequestSchema } from '../../gen/fallout/terminal/player/v1/hacking_pb.js';
import { NavigateRequestSchema } from '../../gen/fallout/terminal/player/v1/navigation_pb.js';
import {
  ActionReason,
  PlayerService,
  PresentationUplinkRequestSchema,
  SelectCharacterRequestSchema,
  SetPresentationRequestSchema,
  SubscribeRequestSchema,
  type ActionResult,
  type PresentationUplinkResponse,
  type SubscriptionMessage,
} from '../../gen/fallout/terminal/player/v1/player_pb.js';
import {
  SoundCategory,
  SoundManifestRequestSchema,
  type SoundManifestResponse,
} from '../../gen/fallout/terminal/player/v1/sound_pb.js';
import type { ControllerTerminalPresentation } from '../../gen/fallout/terminal/player/v1/terminal_pb.js';
import { createPlayerTransport } from './presentation-stream-transport.js';

const MAX_PUBLIC_MESSAGE_BYTES = 4 << 10;
const MAX_SAFE_REVISION = BigInt(Number.MAX_SAFE_INTEGER);
const MAX_PRESENTATION_PAGE_INDEX = 10_000;

const playerMethodNames = [
  'subscribe',
  'selectCharacter',
  'navigate',
  'guess',
  'activatePattern',
  'setPresentation',
  'presentationUplink',
  'soundManifest',
] as const satisfies readonly (keyof typeof PlayerService.method)[];

export const PLAYER_RPC_CONTRACTS = Object.freeze(playerMethodNames.map(localName => {
  const method = PlayerService.method[localName];
  return Object.freeze({
    cardinality: method.methodKind,
    localName,
    procedure: `/${PlayerService.typeName}/${method.name}`,
  });
}));

export class PlayerRPCValidationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'PlayerRPCValidationError';
  }
}

export type PlayerServiceClient = Client<typeof PlayerService>;
type SubscribeInput = Parameters<PlayerServiceClient['subscribe']>[0];
type SelectCharacterInput = Parameters<PlayerServiceClient['selectCharacter']>[0];
type NavigateInput = Parameters<PlayerServiceClient['navigate']>[0];
type GuessInput = Parameters<PlayerServiceClient['guess']>[0];
type ActivatePatternInput = Parameters<PlayerServiceClient['activatePattern']>[0];
type SetPresentationInput = Parameters<PlayerServiceClient['setPresentation']>[0];
type PresentationUplinkInput = Parameters<PlayerServiceClient['presentationUplink']>[0];
type SoundManifestInput = Parameters<PlayerServiceClient['soundManifest']>[0];

export interface PlayerRPCAdapter {
  subscribe(input: SubscribeInput, options?: CallOptions): AsyncIterable<SubscriptionMessage>;
  selectCharacter(input: SelectCharacterInput, options?: CallOptions): Promise<ActionResult>;
  navigate(input: NavigateInput, options?: CallOptions): Promise<ActionResult>;
  guess(input: GuessInput, options?: CallOptions): Promise<ActionResult>;
  activatePattern(input: ActivatePatternInput, options?: CallOptions): Promise<ActionResult>;
  setPresentation(input: SetPresentationInput, options?: CallOptions): Promise<ActionResult>;
  presentationUplink(input: PresentationUplinkInput, options?: CallOptions): Promise<PresentationUplinkResponse>;
  soundManifest(input: SoundManifestInput, options?: CallOptions): Promise<SoundManifestResponse>;
}

export interface PlayerRPCAdapterOptions {
  readonly baseUrl?: string;
  readonly client?: PlayerServiceClient;
  readonly transport?: Transport;
}

type PublicFieldName =
  | 'action target'
  | 'broadcast ID'
  | 'character ID'
  | 'generation ID'
  | 'recognition handle'
  | 'request ID'
  | 'terminal ID';

const publicFieldLimits: Readonly<Record<PublicFieldName, number>> = Object.freeze({
  'action target': 256,
  'broadcast ID': 128,
  'character ID': 256,
  'generation ID': 128,
  'recognition handle': 128,
  'request ID': 128,
  'terminal ID': 256,
});

function assertPublicField(name: PublicFieldName, value: unknown): asserts value is string {
  if (typeof value !== 'string' || value === '' || value.trim() !== value) {
    throw new PlayerRPCValidationError(`${name} must be a nonblank string`);
  }
  const bytes = new TextEncoder().encode(value);
  if (bytes.byteLength > publicFieldLimits[name]) {
    throw new PlayerRPCValidationError(`${name} exceeds ${publicFieldLimits[name]} bytes`);
  }
  for (const character of value) {
    const point = character.codePointAt(0) ?? 0;
    if (point < 0x21 || point > 0x7e) {
      throw new PlayerRPCValidationError(`${name} contains an invalid character`);
    }
  }
}

function assertOptionalPublicField(name: PublicFieldName, value: unknown): void {
  if (value !== undefined) assertPublicField(name, value);
}

function assertMessageSize<D extends DescMessage>(schema: D, input: MessageInitShape<D>): MessageShape<D> {
  const message = create(schema, input);
  if (toBinary(schema, message).byteLength > MAX_PUBLIC_MESSAGE_BYTES) {
    throw new PlayerRPCValidationError(`public player request exceeds ${MAX_PUBLIC_MESSAGE_BYTES} bytes`);
  }
  return message;
}

function assertCommonMutation(input: {
  readonly broadcastId?: unknown;
  readonly recognitionHandle?: unknown;
  readonly requestId?: unknown;
}): void {
  assertPublicField('recognition handle', input.recognitionHandle);
  assertPublicField('request ID', input.requestId);
  assertPublicField('broadcast ID', input.broadcastId);
}

function assertTerminalMutation(input: {
  readonly broadcastId?: unknown;
  readonly recognitionHandle?: unknown;
  readonly requestId?: unknown;
  readonly terminalId?: unknown;
}): void {
  assertCommonMutation(input);
  assertPublicField('terminal ID', input.terminalId);
}

function assertPresentation(presentation: ControllerTerminalPresentation | undefined, contextKey: string): void {
  if (presentation === undefined || presentation.contextKey !== contextKey) {
    throw new PlayerRPCValidationError('presentation context precondition does not match');
  }
  const variant = presentation.presentation;
  if (variant.case === 'none') return;
  if (variant.case === 'menu') {
    assertPublicField('action target', variant.value.targetId);
    return;
  }
  if (variant.case === 'page') {
    if (!Number.isSafeInteger(variant.value.pageIndex) || variant.value.pageIndex < 0 ||
        variant.value.pageIndex > MAX_PRESENTATION_PAGE_INDEX) {
      throw new PlayerRPCValidationError('presentation page index is invalid');
    }
    return;
  }
  if (variant.case === 'hacking') {
    const target = variant.value.target;
    if (target.case !== 'targetId' && target.case !== 'patternId') {
      throw new PlayerRPCValidationError('hacking presentation target is required');
    }
    assertPublicField('action target', target.value);
    return;
  }
  throw new PlayerRPCValidationError('presentation variant is required');
}

function assertActionResult(result: ActionResult, requestId: string): ActionResult {
  if (result.requestId !== requestId || result.revision < 0n || result.revision > MAX_SAFE_REVISION ||
      result.reason < ActionReason.ACCEPTED || result.reason > ActionReason.DUPLICATE ||
      result.accepted !== (result.reason === ActionReason.ACCEPTED)) {
    throw new PlayerRPCValidationError('action result is invalid');
  }
  return result;
}

function validatedBaseURL(value: string): string {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new PlayerRPCValidationError('player RPC base URL is invalid');
  }
  if ((url.protocol !== 'http:' && url.protocol !== 'https:') || url.username !== '' || url.password !== '' ||
      url.search !== '' || url.hash !== '') {
    throw new PlayerRPCValidationError('player RPC base URL is invalid');
  }
  return url.href.replace(/\/$/, '');
}

export function createPlayerRPCAdapter(options: PlayerRPCAdapterOptions = {}): PlayerRPCAdapter {
  const baseUrl = validatedBaseURL(options.baseUrl ?? window.location.origin);
  const fallbackTransport = createConnectTransport({ baseUrl, useBinaryFormat: true });
  const client = options.client ?? createClient(
    PlayerService,
    options.transport ?? createPlayerTransport(baseUrl, fallbackTransport),
  );

  return Object.freeze({
    subscribe(input: SubscribeInput, callOptions?: CallOptions): AsyncIterable<SubscriptionMessage> {
      const request = assertMessageSize(SubscribeRequestSchema, input);
      assertOptionalPublicField('recognition handle', request.recognitionHandle);
      assertOptionalPublicField('generation ID', request.clientInstanceId);
      return client.subscribe(request, callOptions);
    },
    async selectCharacter(input: SelectCharacterInput, callOptions?: CallOptions): Promise<ActionResult> {
      const request = assertMessageSize(SelectCharacterRequestSchema, input);
      assertCommonMutation(request);
      assertPublicField('character ID', request.characterId);
      return assertActionResult(await client.selectCharacter(request, callOptions), request.requestId);
    },
    async navigate(input: NavigateInput, callOptions?: CallOptions): Promise<ActionResult> {
      const request = assertMessageSize(NavigateRequestSchema, input);
      assertTerminalMutation(request);
      if (request.action.case !== 'back') {
        if (request.action.case !== 'enter' && request.action.case !== 'command' && request.action.case !== 'entry') {
          throw new PlayerRPCValidationError('navigate action is required');
        }
        assertPublicField('action target', request.action.value.nodeId);
      }
      return assertActionResult(await client.navigate(request, callOptions), request.requestId);
    },
    async guess(input: GuessInput, callOptions?: CallOptions): Promise<ActionResult> {
      const request = assertMessageSize(GuessRequestSchema, input);
      assertTerminalMutation(request);
      if (request.target.case === 'wordId') {
        assertPublicField('action target', request.target.value);
      } else if (request.target.case === 'filler') {
        const { character, column } = request.target.value;
        if (!Number.isInteger(column) || column < 0 || column > 1 ||
            !Number.isInteger(character) || character < 0 || character >= 192) {
          throw new PlayerRPCValidationError('filler target is outside the public board');
        }
      } else {
        throw new PlayerRPCValidationError('guess target is required');
      }
      return assertActionResult(await client.guess(request, callOptions), request.requestId);
    },
    async activatePattern(input: ActivatePatternInput, callOptions?: CallOptions): Promise<ActionResult> {
      const request = assertMessageSize(ActivatePatternRequestSchema, input);
      assertTerminalMutation(request);
      assertPublicField('action target', request.patternId);
      return assertActionResult(await client.activatePattern(request, callOptions), request.requestId);
    },
    async setPresentation(input: SetPresentationInput, callOptions?: CallOptions): Promise<ActionResult> {
      const request = assertMessageSize(SetPresentationRequestSchema, input);
      assertTerminalMutation(request);
      assertPublicField('action target', request.contextKey);
      assertPresentation(request.presentation, request.contextKey);
      return assertActionResult(await client.setPresentation(request, callOptions), request.requestId);
    },
    async presentationUplink(input: PresentationUplinkInput, callOptions?: CallOptions): Promise<PresentationUplinkResponse> {
      async function* validatedFrames() {
        for await (const frame of input) {
          const request = assertMessageSize(PresentationUplinkRequestSchema, frame);
          if (request.payload.case === 'open') {
            assertPublicField('generation ID', request.payload.value.clientInstanceId);
            assertPublicField('recognition handle', request.payload.value.recognitionHandle);
            if (request.payload.value.uplinkGeneration <= 0n || request.payload.value.uplinkGeneration > MAX_SAFE_REVISION) {
              throw new PlayerRPCValidationError('uplink generation is invalid');
            }
          } else if (request.payload.case === 'intent') {
            const intent = request.payload.value;
            assertTerminalMutation(intent);
            assertPublicField('action target', intent.contextKey);
            assertPresentation(intent.presentation, intent.contextKey);
          } else {
            throw new PlayerRPCValidationError('presentation uplink payload is required');
          }
          yield request;
        }
      }
      return client.presentationUplink(validatedFrames(), callOptions);
    },
    async soundManifest(input: SoundManifestInput, callOptions?: CallOptions): Promise<SoundManifestResponse> {
      const request = assertMessageSize(SoundManifestRequestSchema, input);
      if (!Number.isInteger(request.category) || request.category < SoundCategory.AMBIENT ||
          request.category > SoundCategory.CHARSCROLL) {
        throw new PlayerRPCValidationError('sound category is invalid');
      }
      const result = await client.soundManifest(request, callOptions);
      if (result.category !== request.category || !Array.isArray(result.assets)) {
        throw new PlayerRPCValidationError('sound manifest result is invalid');
      }
      return result;
    },
  });
}
