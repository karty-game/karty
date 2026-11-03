self.onmessage = async ({ data: { bytes, codec, target, capacity } }) => {
  try {
    if (
      !(bytes instanceof ArrayBuffer) ||
      !(codec instanceof ArrayBuffer) ||
      codec.byteLength > 8 * 1024 * 1024 ||
      bytes.byteLength > 64 * 1024 * 1024 ||
      !Number.isInteger(capacity) ||
      capacity < 0 ||
      capacity > 256 * 1024 * 1024 ||
      !Number.isInteger(target) ||
      target < 0 ||
      target > 4
    )
      throw new Error("Invalid texture job");
    const { instance } = await WebAssembly.instantiate(codec, {
      wasi_snapshot_preview1: {
        proc_exit(code) {
          throw new Error(`Texture codec exit ${code}`);
        },
        fd_close() {
          return 8;
        },
        fd_seek() {
          return 8;
        },
        fd_write() {
          return 8;
        },
        fd_fdstat_get() {
          return 8;
        },
        clock_time_get() {
          return 52;
        },
        environ_sizes_get() {
          return 0;
        },
        environ_get() {
          return 0;
        },
      },
    });
    const api = instance.exports;
    api._initialize?.();
    const input = api.malloc(bytes.byteLength);
    const output = capacity ? api.malloc(capacity) : 0;
    if (!input || (capacity && !output)) throw new Error("Texture codec allocation failed");
    new Uint8Array(api.memory.buffer, input, bytes.byteLength).set(new Uint8Array(bytes));
    const written = api.transcode(input, bytes.byteLength, target, output, capacity);
    if (written < 0 || written !== capacity) throw new Error(`Texture codec error ${written}`);
    const result = new Uint8Array(api.memory.buffer, output, capacity).slice();
    self.postMessage({ bytes: result.buffer }, [result.buffer]);
  } catch (error) {
    self.postMessage({ error: String(error?.message ?? error) });
  }
};
