import { appendHeaders, Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
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

export class LatestPresentationMailbox {
  #closed = false;
  #pending = null;
  #waiter = null;

  offer(value) {
    if (this.#closed) return false;
    if (this.#waiter) {
      const resolve = this.#waiter;
      this.#waiter = null;
      resolve({ value, done: false });
    } else {
      this.#pending = value;
    }
    return true;
  }

  close() {
    if (this.#closed) return;
    this.#closed = true;
    this.#pending = null;
    if (this.#waiter) {
      const resolve = this.#waiter;
      this.#waiter = null;
      resolve({ value: undefined, done: true });
    }
  }

  [Symbol.asyncIterator]() {
    return {
      next: () => {
        if (this.#pending !== null) {
          const value = this.#pending;
          this.#pending = null;
          return Promise.resolve({ value, done: false });
        }
        if (this.#closed) return Promise.resolve({ value: undefined, done: true });
        return new Promise(resolve => { this.#waiter = resolve; });
      },
    };
  }
}

export function supportsPresentationRequestStreaming(scope = globalThis) {
  if (scope.location?.protocol !== 'https:' || !scope.isSecureContext ||
      typeof scope.ReadableStream !== 'function' ||
      typeof scope.Request !== 'function' || typeof scope.fetch !== 'function') {
    return false;
  }
  try {
    const request = new scope.Request(scope.location.origin, {
      method: 'POST',
      body: new scope.ReadableStream(),
      duplex: 'half',
    });
    return request.body !== null && request.duplex === 'half';
  } catch {
    return false;
  }
}

export function createPresentationUplinkTransport({
  baseUrl,
  fallback,
  method: uplinkMethod,
  fetch: fetchImpl = globalThis.fetch,
}) {
  return {
    unary: (...args) => fallback.unary(...args),
    stream: (method, signal, timeoutMs, headers, input, contextValues) => {
      if (method !== uplinkMethod) {
        return fallback.stream(method, signal, timeoutMs, headers, input, contextValues);
      }
      return presentationStream({ baseUrl, fetchImpl, method, signal, timeoutMs, headers, input });
    },
  };
}

async function presentationStream({ baseUrl, fetchImpl, method, signal, timeoutMs, headers, input }) {
  const { serialize, parse } = createClientMethodSerializers(method, true);
  const iterator = input[Symbol.asyncIterator]();
  const body = new ReadableStream({
    async pull(controller) {
      try {
        const next = await iterator.next();
        if (next.done) {
          controller.close();
        } else {
          controller.enqueue(encodeEnvelope(0, serialize(create(method.input, next.value))));
        }
      } catch (error) {
        controller.error(error);
      }
    },
    async cancel(reason) {
      await iterator.return?.(reason);
    },
  });
  const response = await fetchImpl(createMethodUrl(baseUrl, method), {
    method: 'POST',
    headers: requestHeader(method.methodKind, true, timeoutMs, headers, false),
    signal,
    body,
    duplex: 'half',
    credentials: 'same-origin',
    redirect: 'error',
  });
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
    stream: true,
    service: method.parent,
    method,
    header: response.headers,
    trailer,
    message: parseResponseBody(response.body, parse, trailer, response.headers, signal),
  };
}

async function* parseResponseBody(body, parse, trailer, header, signal) {
  const reader = createEnvelopeReadableStream(body).getReader();
  let endStreamReceived = false;
  for (;;) {
    const next = await reader.read();
    if (next.done) break;
    const { flags, data } = next.value;
    if ((flags & compressedFlag) === compressedFlag) {
      throw new ConnectError('unsupported compressed streaming response', Code.Internal);
    }
    if ((flags & endStreamFlag) === endStreamFlag) {
      endStreamReceived = true;
      const endStream = endStreamFromJson(data);
      if (endStream.error) {
        header.forEach((value, key) => endStream.error.metadata.append(key, value));
        throw endStream.error;
      }
      endStream.metadata.forEach((value, key) => trailer.set(key, value));
      continue;
    }
    yield parse(data);
  }
  signal?.throwIfAborted?.();
  if (!endStreamReceived) throw new ConnectError('missing end-stream response', Code.Internal);
}
