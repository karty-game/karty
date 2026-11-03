// Each active job owns its worker, so cancellation also terminates the codec.
// A bounded queue prevents parallel inflate/output allocations during streaming.
let previous = Promise.resolve();
globalThis.kartyTextureTranscode = (bytes, target, capacity) => {
  let worker;
  let cancelled = false;
  let rejectActive;
  let fetchController;
  const stopWorker = () => {
    const active = worker;
    worker = undefined;
    if (active) {
      active.onmessage = null;
      active.onerror = null;
      active.terminate();
    }
  };
  const promise = previous
    .catch(() => {})
    .then(async () => {
      if (cancelled) throw new Error("Texture job cancelled");
      // The page owns the delivery release pin. A new dedicated worker has no
      // pin, so its own binary fetch would be rejected by the delivery worker.
      fetchController = new AbortController();
      const response = await fetch(new URL("./transcoder.wasm", import.meta.url), {
        redirect: "error",
        signal: fetchController.signal,
      });
      if (!response.ok) throw new Error("Texture codec fetch failed");
      const codec = await readCodec(response);
      fetchController = undefined;
      if (cancelled) throw new Error("Texture job cancelled");
      return new Promise((resolve, reject) => {
        rejectActive = reject;
        worker = new Worker(new URL("./texture-worker.mjs", import.meta.url), { type: "module" });
        const active = worker;
        active.onmessage = ({ data }) => {
          if (worker !== active) return;
          stopWorker();
          if (data.error) reject(new Error(data.error));
          else resolve(new Uint8Array(data.bytes));
        };
        active.onerror = (event) => {
          if (worker !== active) return;
          stopWorker();
          reject(new Error(event.message || "Texture worker failed"));
        };
        worker.postMessage({ bytes: bytes.buffer, codec, target, capacity }, [bytes.buffer, codec]);
      });
    });
  // The serialization tail carries no image result, so a completed job does
  // not retain its transferred output until another texture is requested.
  previous = promise.then(
    () => undefined,
    () => undefined,
  );
  return {
    promise,
    cancel() {
      cancelled = true;
      bytes = null;
      fetchController?.abort();
      stopWorker();
      rejectActive?.(new Error("Texture job cancelled"));
    },
  };
};

async function readCodec(response) {
  const limit = 8 * 1024 * 1024;
  if (Number(response.headers.get("Content-Length")) > limit) {
    await response.body?.cancel();
    throw new Error("Texture codec exceeds its size limit");
  }
  if (!response.body) throw new Error("Texture codec response has no body");
  const reader = response.body.getReader();
  const parts = [];
  let size = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > limit) throw new Error("Texture codec exceeds its size limit");
      parts.push(value);
    }
  } catch (error) {
    await reader.cancel().catch(() => {});
    throw error;
  }
  const result = new Uint8Array(size);
  let offset = 0;
  for (const part of parts) {
    result.set(part, offset);
    offset += part.byteLength;
  }
  return result.buffer;
}
