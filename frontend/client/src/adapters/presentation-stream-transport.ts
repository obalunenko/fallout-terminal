import { create, type DescMessage, type DescMethodStreaming, type MessageInitShape } from '@bufbuild/protobuf';
import { appendHeaders, Code, ConnectError, type StreamResponse, type Transport } from '@connectrpc/connect';
import {
  compressedFlag,
  createClientMethodSerializers,
  createEnvelopeReadableStream,
  createMethodUrl,
  encodeEnvelope,
} from '@connectrpc/connect/protocol';
import {
  endStreamFlag,
  endStreamFromJson,
  errorFromJson,
  requestHeader,
  trailerDemux,
  validateResponse,
} from '@connectrpc/connect/protocol-connect';

import { PlayerService } from '../../gen/fallout/terminal/player/v1/player_pb.js';

type StreamingRequestInit = RequestInit & { readonly duplex: 'half' };

async function* parseResponseBody<Output extends DescMessage>(
  body: ReadableStream<Uint8Array>,
  parse: (data: Uint8Array) => ReturnType<typeof create<Output>>,
  trailer: Headers,
  header: Headers,
  signal: AbortSignal | undefined,
): AsyncIterable<ReturnType<typeof create<Output>>> {
  const reader = createEnvelopeReadableStream(body).getReader();
  let endStreamReceived = false;
  for (;;) {
    const next = await reader.read();
    if (next.done) break;
    const { data, flags } = next.value;
    if ((flags & compressedFlag) === compressedFlag) {
      throw new ConnectError('unsupported compressed streaming response', Code.Internal);
    }
    if ((flags & endStreamFlag) === endStreamFlag) {
      endStreamReceived = true;
      const endStream = endStreamFromJson(data);
      if (endStream.error !== undefined) {
        header.forEach((value, key) => endStream.error?.metadata.append(key, value));
        throw endStream.error;
      }
      endStream.metadata.forEach((value, key) => trailer.set(key, value));
      continue;
    }
    yield parse(data);
  }
  signal?.throwIfAborted();
  if (!endStreamReceived) throw new ConnectError('missing end-stream response', Code.Internal);
}

async function presentationStream<Input extends DescMessage, Output extends DescMessage>(
  baseUrl: string,
  method: DescMethodStreaming<Input, Output>,
  signal: AbortSignal | undefined,
  timeoutMs: number | undefined,
  headers: HeadersInit | undefined,
  input: AsyncIterable<MessageInitShape<Input>>,
  fetcher: typeof fetch,
): Promise<StreamResponse<Input, Output>> {
  const { parse, serialize } = createClientMethodSerializers(method, true);
  const iterator = input[Symbol.asyncIterator]();
  const body = new ReadableStream<Uint8Array>({
    async cancel(reason) { await iterator.return?.(reason); },
    async pull(controller) {
      try {
        const next = await iterator.next();
        if (next.done) controller.close();
        else controller.enqueue(encodeEnvelope(0, serialize(create(method.input, next.value))));
      } catch (error) {
        controller.error(error);
      }
    },
  });
  const init: StreamingRequestInit = {
    body,
    credentials: 'same-origin',
    duplex: 'half',
    headers: requestHeader(method.methodKind, true, timeoutMs, headers, false),
    method: 'POST',
    redirect: 'error',
    ...(signal === undefined ? {} : { signal }),
  };
  const response = await fetcher(createMethodUrl(baseUrl, method), init);
  const validation = validateResponse(method.methodKind, true, response.status, response.headers);
  if (validation.isUnaryError) {
    throw errorFromJson(
      await response.json(),
      appendHeaders(...trailerDemux(response.headers)),
      validation.unaryError,
    );
  }
  if (response.body === null) throw new ConnectError('missing response body', Code.Internal);
  const trailer = new Headers();
  return {
    header: response.headers,
    message: parseResponseBody(response.body, parse, trailer, response.headers, signal),
    method,
    service: method.parent,
    stream: true,
    trailer,
  };
}

export function createPlayerTransport(
  baseUrl: string,
  fallback: Transport,
  fetcher: typeof fetch = globalThis.fetch,
): Transport {
  return {
    stream(method, signal, timeoutMs, headers, input, contextValues) {
      const uplink = PlayerService.method.presentationUplink;
      if (method.name !== uplink.name || method.parent.typeName !== uplink.parent.typeName) {
        return fallback.stream(method, signal, timeoutMs, headers, input, contextValues);
      }
      return presentationStream(baseUrl, method, signal, timeoutMs, headers, input, fetcher);
    },
    unary(method, signal, timeoutMs, headers, input, contextValues) {
      return fallback.unary(method, signal, timeoutMs, headers, input, contextValues);
    },
  };
}
