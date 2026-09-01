import { create } from '@bufbuild/protobuf';

import {
  ActionReason,
  PresentationUplinkRequestSchema,
  type ActionResult,
  type PresentationIntent,
  type PresentationUplinkRequest,
  type PresentationUplinkResult,
  type SetPresentationRequest,
} from '../../gen/fallout/terminal/player/v1/player_pb.js';
import type { PlayerRPCAdapter } from './player-rpc.js';

export interface PresentationStreamingScope {
  readonly Request?: typeof Request;
  readonly ReadableStream?: typeof ReadableStream;
  readonly fetch?: typeof fetch;
  readonly isSecureContext?: boolean;
  readonly location?: Pick<Location, 'origin' | 'protocol'>;
}

export type ValidatedPresentationResult =
  | { readonly kind: 'action'; readonly action: ActionResult }
  | { readonly kind: 'ready' };

export interface PresentationResultExpectation {
  readonly clientInstanceID: string;
  readonly generation: number;
  readonly requestIDs: ReadonlySet<string>;
}

export interface PresentationUplinkTransport {
  readonly streaming: boolean;
  fallback(input: SetPresentationRequest, signal: AbortSignal): Promise<ActionResult>;
  open(input: Parameters<PlayerRPCAdapter['presentationUplink']>[0], signal: AbortSignal): Promise<boolean>;
}

export function supportsPresentationRequestStreaming(scope: PresentationStreamingScope = globalThis): boolean {
  if (scope.location?.protocol !== 'https:' || scope.isSecureContext !== true ||
      typeof scope.ReadableStream !== 'function' || typeof scope.Request !== 'function' ||
      typeof scope.fetch !== 'function') return false;
  try {
    const request = new scope.Request(scope.location.origin, {
      body: new scope.ReadableStream(),
      duplex: 'half',
      method: 'POST',
    } as RequestInit);
    return request.body !== null && (request as Request & { readonly duplex?: string }).duplex === 'half';
  } catch {
    return false;
  }
}

export function createPresentationOpen(
  clientInstanceID: string,
  generation: number,
  recognitionHandle: string,
): PresentationUplinkRequest {
  return create(PresentationUplinkRequestSchema, {
    payload: {
      case: 'open',
      value: { clientInstanceId: clientInstanceID, recognitionHandle, uplinkGeneration: BigInt(generation) },
    },
  });
}

export function createPresentationIntent(intent: PresentationIntent): PresentationUplinkRequest {
  return create(PresentationUplinkRequestSchema, { payload: { case: 'intent', value: intent } });
}

export function validatePresentationResult(
  result: PresentationUplinkResult,
  expected: PresentationResultExpectation,
): ValidatedPresentationResult | null {
  if (!Number.isSafeInteger(expected.generation) || expected.generation <= 0 ||
      result.clientInstanceId !== expected.clientInstanceID || result.uplinkGeneration !== BigInt(expected.generation)) return null;
  if (result.payload.case === 'ready') return Object.freeze({ kind: 'ready' });
  if (result.payload.case !== 'action') return null;
  const action = result.payload.value;
  if (!expected.requestIDs.has(action.requestId) || action.revision < 0n ||
      action.revision > BigInt(Number.MAX_SAFE_INTEGER) ||
      action.reason < ActionReason.ACCEPTED || action.reason > ActionReason.DUPLICATE ||
      action.accepted !== (action.reason === ActionReason.ACCEPTED)) return null;
  return Object.freeze({ action, kind: 'action' });
}

export function createPresentationUplinkTransport(
  rpc: Pick<PlayerRPCAdapter, 'presentationUplink' | 'setPresentation'>,
  scope: PresentationStreamingScope = globalThis,
): PresentationUplinkTransport {
  const streaming = supportsPresentationRequestStreaming(scope);
  return Object.freeze({
    async fallback(input: SetPresentationRequest, signal: AbortSignal): Promise<ActionResult> {
      return rpc.setPresentation(input, { signal });
    },
    async open(input: Parameters<PlayerRPCAdapter['presentationUplink']>[0], signal: AbortSignal): Promise<boolean> {
      if (!streaming || signal.aborted) return false;
      try {
        const response = await rpc.presentationUplink(input, { signal });
        return !signal.aborted && response.$typeName === 'fallout.terminal.player.v1.PresentationUplinkResponse';
      } catch {
        return false;
      }
    },
    streaming,
  });
}
