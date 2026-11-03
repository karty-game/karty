// Browser adapter for the actual generated WASI reactor. No gameplay is
// implemented here: all initialize/update/shutdown calls enter the cartridge.
export async function loadCartridge(bytes) {
  if (typeof globalThis.kartyWebGPUValidateCartridge !== "function")
    throw new Error("Cartridge validator is unavailable");
  let client;
  let initialized = false;
  let memoryBuffer, memoryBytes, memoryView;
  let commandView, commandPointer, commandLength;
  const decoder = new TextDecoder();
  const memory = () => {
    const buffer = client.exports.memory.buffer;
    if (buffer !== memoryBuffer) {
      memoryBuffer = buffer;
      memoryBytes = new Uint8Array(buffer);
      memoryView = undefined;
      commandView = undefined;
    }
    return memoryBytes;
  };
  const view = () => {
    memory();
    return (memoryView ??= new DataView(memoryBuffer));
  };
  const checkedMemory = (pointer, length, maximum = 65536) => {
    pointer >>>= 0;
    length >>>= 0;
    const data = memory();
    if (length > maximum || pointer > data.length || length > data.length - pointer)
      throw new Error("Invalid cartridge memory range");
    return data;
  };
  const range = (pointer, length, maximum = 65536) => {
    const data = checkedMemory(pointer, length, maximum);
    pointer >>>= 0;
    return data.subarray(pointer, pointer + (length >>> 0));
  };
  const host = (name, ...args) => {
    const callback = globalThis[name];
    if (typeof callback !== "function") throw new Error(`Host service ${name} is unavailable`);
    return callback(...args);
  };
  const empty = (count, size) => {
    range(count, 4);
    range(size, 4);
    view().setUint32(count, 0, true);
    view().setUint32(size, 0, true);
    return 0;
  };
  const wasi = {
    args_sizes_get: empty,
    environ_sizes_get: empty,
    args_get: () => 0,
    environ_get: () => 0,
    fd_close: () => 8,
    fd_prestat_get: () => 8,
    fd_prestat_dir_name: () => 8,
    fd_fdstat_get(fd, pointer) {
      if (fd >>> 0 > 2) return 8;
      range(pointer, 24).fill(0);
      memory()[pointer] = 2;
      view().setBigUint64(pointer + 8, fd === 0 ? 2n : 64n, true);
      return 0;
    },
    fd_fdstat_set_flags: (fd, flags) => (fd >>> 0 > 2 ? 8 : flags & ~4 ? 58 : 0),
    fd_read(fd, pointers, count, read) {
      if (fd !== 0) return 8;
      count >>>= 0;
      if (count > 1024) return 28;
      range(read, 4);
      range(pointers, (count >>> 0) * 8, 8192);
      for (let i = 0; i < count; i++)
        range(view().getUint32(pointers + i * 8, true), view().getUint32(pointers + i * 8 + 4, true), 1 << 20);
      view().setUint32(read, 0, true);
      return 0;
    },
    fd_write(fd, pointers, count, written) {
      if (fd !== 1 && fd !== 2) return 8;
      count >>>= 0;
      if (count > 1024) return 28;
      range(written, 4);
      range(pointers, (count >>> 0) * 8, 8192);
      let total = 0;
      const parts = [];
      for (let i = 0; i < count; i++) {
        const part = range(view().getUint32(pointers + i * 8, true), view().getUint32(pointers + i * 8 + 4, true));
        total += part.length;
        if (total > 65536) throw new Error("Cartridge diagnostic output exceeds its bound");
        parts.push(decoder.decode(part));
      }
      view().setUint32(written, total, true);
      if (total) console.log(parts.join(""));
      return 0;
    },
    proc_exit(code) {
      throw new Error(`Cartridge exited (${code})`);
    },
    sched_yield: () => 0,
    poll_oneoff: () => 52, // Blocking guest scheduling is outside the frame ABI.
    random_get(pointer, length) {
      const destination = range(pointer, length, 1 << 20);
      for (let offset = 0; offset < destination.length; offset += 65536)
        crypto.getRandomValues(destination.subarray(offset, offset + 65536));
      return 0;
    },
    clock_time_get(clock, _precision, pointer) {
      if (clock !== 0 && clock !== 1) return 28;
      range(pointer, 8);
      view().setBigUint64(pointer, BigInt(Math.floor((clock === 0 ? Date.now() : performance.now()) * 1e6)), true);
      return 0;
    },
  };
  const karty = {
    submit_commands(pointer, length) {
      const data = checkedMemory(pointer, length);
      pointer >>>= 0;
      length >>>= 0;
      if (!commandView || commandPointer !== pointer || commandLength !== length) {
        commandView = data.subarray(pointer, pointer + length);
        commandPointer = pointer;
        commandLength = length;
      }
      host("kartyHostSubmitCommands", commandView);
    },
    log(pointer, length) {
      host("kartyHostLog", decoder.decode(range(pointer, length)));
    },
    action: (id) => host("kartyHostAction", id),
    access_save(operation, keyPointer, keyLength, pointer, length) {
      const key = decoder.decode(range(keyPointer, keyLength, 64));
      if (!/^[A-Za-z0-9_-]{1,64}$/.test(key) || operation < 1 || operation > 3 || (operation === 3 && length !== 0))
        return -1;
      return host("kartyHostAccessSave", operation, key, range(pointer, length, 48 * 1024));
    },
  };
  for (const [name, callback] of Object.entries({
    play_sound: "kartyHostPlaySound",
    play_sfx_from: "kartyHostPlaySFXFrom",
    set_sound_emitter: "kartyHostSetSoundEmitter",
    stop_sound_emitter: "kartyHostStopSoundEmitter",
    set_audio_receiver: "kartyHostSetAudioReceiver",
    clear_audio_receiver: "kartyHostClearAudioReceiver",
    play_music: "kartyHostPlayMusic",
    stop_music: "kartyHostStopMusic",
    play_environment: "kartyHostPlayEnvironment",
    stop_environment: "kartyHostStopEnvironment",
    play_video: "kartyHostPlayVideo",
    stop_video: "kartyHostStopVideo",
  }))
    karty[name] = (...args) => host(callback, ...args);
  const module = await WebAssembly.compile(bytes);
  await globalThis.kartyWebGPUValidateCartridge(bytes);
  const imports = { wasi_snapshot_preview1: wasi, karty };
  for (const item of WebAssembly.Module.imports(module)) {
    if (
      item.kind !== "function" ||
      !Object.hasOwn(imports, item.module) ||
      !Object.hasOwn(imports[item.module], item.name)
    )
      throw new Error(`Unsupported cartridge import ${item.module}.${item.name}`);
  }
  client = await WebAssembly.instantiate(module, { wasi_snapshot_preview1: wasi, karty });
  for (const name of ["karty_register", "initialize", "event_buffer", "update", "shutdown"]) {
    if (typeof client.exports[name] !== "function") throw new Error(`Missing cartridge export ${name}`);
  }
  if (!(client.exports.memory instanceof WebAssembly.Memory)) throw new Error("Cartridge memory is unavailable");
  globalThis.kartyClientInitialize = () => {
    if (initialized) throw new Error("Cartridge already initialized");
    client.exports._initialize?.();
    client.exports.karty_register();
    initialized = true;
    client.exports.initialize();
  };
  globalThis.kartyClientUpdate = (frame, events) => {
    if (!initialized) throw new Error("Cartridge is not initialized");
    const guestFrame = BigInt(frame);
    const pointer = client.exports.event_buffer() >>> 0;
    checkedMemory(pointer, events.length).set(events, pointer);
    client.exports.update(guestFrame, events.length);
  };
  globalThis.kartyClientShutdown = () => {
    if (initialized) {
      client.exports.shutdown();
      initialized = false;
    }
  };
  return { module, instance: client };
}
