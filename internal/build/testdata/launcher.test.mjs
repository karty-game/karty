// Executes the launcher source or a staged launcher with a host test double.
// This validates the JS/WASM bridge, not browser rendering.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { runInNewContext } from "node:vm";
import { createHash } from "node:crypto";

const mock = process.argv[2] === "--mock";
const directory = mock ? null : process.argv[2];
const source = await readFile(mock ? process.argv[3] : join(directory, "karty.js"), "utf8");
if (!mock) {
  const html = await readFile(join(directory, "index.html"), "utf8");
  const scripts = [...html.matchAll(/<script\s+src="([^"]+)"\s*>/g)].map(match => match[1]);
  assert.deepEqual(scripts.map(path => path.split("?")[0]), ["wasm_exec.js", "karty.js"], "runtime must load before the launcher");
  for (const path of scripts) {
    const [name, query] = path.split("?");
    const bytes = await readFile(join(directory, name));
    assert.equal(query, "v=" + createHash("sha256").update(bytes).digest("hex").slice(0, 16));
  }
  for (const text of [html, source]) {
    for (const warning of ["DO NOT EDIT", "DO NOT MODIFY", "WILL BE OVERWRITTEN", "Source of truth:", "Regenerate with:"]) assert.ok(text.includes(warning));
  }
}
let imports;
let submissions = [];
let registered = false;
let started = false;
let memory;
let eventPointer = 65536;
let resolveDone, rejectDone;
const done = new Promise((resolve, reject) => { resolveDone = resolve; rejectDone = reject; });
const timer = setTimeout(() => rejectDone(new Error("launcher did not start host")), 10000);
const elements = new Map(["loading", "status", "progress", "retry", "error-detail", "game-title", "fullscreen"].map(id => [id, {
  hidden: ["retry", "error-detail", "fullscreen"].includes(id),
  textContent: "",
  attributes: {},
  setAttribute(key, value) { this.attributes[key] = value; },
  addEventListener() {},
}]));
const failure = process.env.KARTY_TEST_HTTP_FAILURE === "1";
const fallback = process.env.KARTY_TEST_WASM_FALLBACK === "1";
const soundSections = process.env.KARTY_TEST_SOUND_SECTIONS || "present";
assert.ok(["present", "absent", "duplicate"].includes(soundSections));
const soundFailure = mock && soundSections === "duplicate";
const expectedFailure = failure || soundFailure;
const mockSoundBundle = new Uint8Array([0x4b, 0x54, 0x59, 0x53, 1, 0, 0, 0]).buffer;
// The mock uses its fixture wire version; staged guests use their pinned SDK.
const protocolSource = mock ? null : await readFile(join(directory, "../../.karty/engine/protocol.go"), "utf8");
const protocolVersion = mock ? 5 : Number(protocolSource.match(/protocolVersion\s*=\s*uint16\((\d+)\)/)?.[1]);
assert.ok(Number.isInteger(protocolVersion) && protocolVersion > 0, "SDK must declare a wire version");

// Exercise the inline fallback independently: the launcher may never execute.
const shell = await readFile(new URL("../templates/index.html.tmpl", import.meta.url), "utf8");
const bootstrap = shell.match(/<script>([\s\S]*?)<\/script>/)[1];
for (const scenario of ["syntax", "download", "rejection", "timeout"]) {
  const nodes = new Map();
  const listeners = {};
  let timeout;
  const window = { addEventListener: (name, listener) => { listeners[name] = listener; } };
  const documentElement = { dataset: {} };
  runInNewContext(bootstrap, {
    window,
    document: { documentElement, getElementById: id => {
      if (!nodes.has(id)) nodes.set(id, { hidden: true, setAttribute() {} });
      return nodes.get(id);
    } },
    setTimeout: callback => { timeout = callback; },
  });
  if (scenario === "syntax") listeners.error({ message: "SyntaxError", filename: "karty.js", lineno: 7 });
  if (scenario === "download") listeners.error({ target: { tagName: "SCRIPT", src: "wasm_exec.js" } });
  if (scenario === "rejection") listeners.unhandledrejection({ reason: new Error("startup failed") });
  if (scenario === "timeout") timeout();
  const firstError = nodes.get("error-detail").textContent;
  timeout();
  assert.equal(nodes.get("error-detail").textContent, firstError, "timeout must not hide the original failure");
  assert.equal(nodes.get("loading").hidden, false);
  assert.equal(nodes.get("retry").hidden, false);
  assert.ok(nodes.get("error-detail").textContent.length > 0);
}

function commands(frame) {
  assert.equal(submissions.length, 1, "one batch per lifecycle");
  const bytes = submissions[0];
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  assert.equal(view.getUint32(0, true), 0x4259544b);
  assert.equal(view.getUint16(4, true), protocolVersion);
  assert.equal(view.getUint8(6), 2);
  assert.equal(view.getBigUint64(8, true), frame);
  assert.equal(view.getUint32(20, true), bytes.length - 24);
  let offset = 24;
  const tags = [];
  for (let i = 0; i < view.getUint32(16, true); i++) {
    tags.push(view.getUint8(offset));
    offset += 3 + view.getUint16(offset + 1, true);
    assert.ok(offset <= bytes.length);
  }
  assert.equal(offset, bytes.length);
  submissions = [];
  return tags;
}

const context = {
  document: {
    documentElement: { dataset: {} },
    getElementById: id => elements.get(id),
    addEventListener() {},
    querySelector: () => ({ focus() {} }),
    fullscreenEnabled: true,
  },
  Uint8Array, DataView, TextDecoder, BigInt,
  crypto: globalThis.crypto,
  console: { warn: console.warn, error: (...args) => {
    if (expectedFailure) setTimeout(resolveDone, 0);
    else rejectDone(new Error(args.map(String).join(" ")));
  } },
  fetch: async (url) => ({
    ok: !failure,
    status: failure ? 503 : 200,
    headers: { get: () => fallback ? "application/octet-stream" : "application/wasm" },
    url,
    arrayBuffer: async () => {
      if (mock) return new TextEncoder().encode(url).buffer;
      const path = new URL(url, "https://karty.test/").pathname.slice(1);
      const bytes = await readFile(join(directory, path));
      return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
    },
  }),
  URL,
  WebAssembly: {
    compile: async bytes => mock ? { bytes } : WebAssembly.compile(bytes),
    Module: {
      customSections: (module, name) => {
        if (!mock) return WebAssembly.Module.customSections(module, name);
        if (name === "karty.ui.v1" || name === "karty.videos.v1" || name === "karty.audio-streams.v1") return [];
        if (name === "karty.sounds.v1") {
          if (soundSections === "absent") return [];
          if (soundSections === "duplicate") return [mockSoundBundle, mockSoundBundle.slice(0)];
          return [mockSoundBundle];
        }
        if (name === "karty.assets.v1") {
          const bundle = new Uint8Array(16);
          bundle.set([0x4b, 0x54, 0x59, 0x41, 1, 0]);
          new DataView(bundle.buffer).setUint32(12, bundle.length, true);
          return [bundle.buffer];
        }
        assert.equal(name, "karty.manifest.v1");
        const project = new TextEncoder().encode("Karty");
        const compiler = new TextEncoder().encode("tinygo");
        const bundle = new Uint8Array(20 + project.length + compiler.length);
        bundle.set([0x4b, 0x54, 0x59, 0x4d, 1, 0, project.length, 0, compiler.length, 0]);
        const view = new DataView(bundle.buffer);
        view.setUint32(12, 960, true); view.setUint32(16, 540, true);
        bundle.set(project, 20); bundle.set(compiler, 20 + project.length);
        return [bundle.buffer];
      },
    },
    instantiate: async (module, loadedImports) => {
      if (mock && module.bytes) return (await context.WebAssembly.instantiateStreaming({ url: "game.kart" }, loadedImports)).instance;
      if (loadedImports.hostTest) {
        if (!mock) assert.ok(WebAssembly.validate(module));
        return { instance: {} };
      }
      imports = loadedImports;
      return WebAssembly.instantiate(module, imports);
    },
    instantiateStreaming: async (responsePromise, loadedImports) => {
      const response = await responsePromise;
      const path = new URL(response.url, "https://karty.test/").pathname;
      if (path === "/karty-host.wasm") {
        if (!mock) assert.ok(WebAssembly.validate(await readFile(join(directory, path.slice(1)))));
        return { instance: {} };
      }
      assert.equal(path, "/game.kart", "launcher requested an unexpected cartridge URL");
      imports = loadedImports;
      if (!mock) return WebAssembly.instantiate(await readFile(join(directory, path.slice(1))), imports);
      memory = new WebAssembly.Memory({ initial: 2 });
      const submit = (frame) => {
        const bytes = new Uint8Array(memory.buffer, 0, 24);
        bytes.fill(0);
        const view = new DataView(memory.buffer);
        view.setUint32(0, 0x4259544b, true); view.setUint16(4, protocolVersion, true);
        view.setUint8(6, 2); view.setBigUint64(8, frame, true);
        imports.karty.submit_commands(0, 24);
      };
      return { instance: { exports: {
        memory,
        _initialize: () => { started = true; },
        karty_register: () => { assert.ok(started); registered = true; },
        initialize: () => { assert.ok(registered); submit(0n); },
        event_buffer: () => eventPointer,
        update: (frame, length) => {
          const view = new DataView(memory.buffer, eventPointer, length);
          assert.equal(view.getBigUint64(8, true), frame);
          submit(frame);
        },
        shutdown: () => submit(2n),
      } } };
    },
  },
  Go: class {
    importObject = { hostTest: true };
    run() {
      try {
        if (mock) {
          if (soundSections === "present") assert.equal(context.kartySoundBundle, mockSoundBundle, "sound bundle must be installed before host startup");
          if (soundSections === "absent") assert.equal(context.kartySoundBundle, null, "absent sound section must clear the global bundle");
          assert.equal(context.kartyAudioStreamBundle, null, "absent streaming audio section must clear the global bundle");
          assert.equal(context.kartyAudioStreamBaseURL, "http://localhost/game.kart?v=@@CLIENT_HASH@@");
        }
        context.kartyHostLog = () => {};
        context.kartyHostPlaySound = () => {};
        context.kartyHostPlaySFXFrom = () => {};
        context.kartyHostSetAudioReceiver = () => {};
        context.kartyHostClearAudioReceiver = () => {};
        context.kartyHostPlayMusic = () => {};
        context.kartyHostStopMusic = () => {};
        context.kartyHostPlayEnvironment = () => {};
        context.kartyHostStopEnvironment = () => {};
        context.kartyHostLoopSound = () => {};
        context.kartyHostStopSound = () => {};
        context.kartyHostAction = () => {};
        context.kartyHostSubmitCommands = (bytes) => submissions.push(Uint8Array.from(bytes));
        context.kartyClientInitialize();
        const initial = commands(0n);
        if (!mock) for (const tag of [1, 8, 10]) assert.ok(initial.includes(tag), `missing create tag ${tag}`);
        for (const frame of [1n, 2n]) {
          const pointer = frame === 2n;
          const events = new Uint8Array(pointer ? 36 : 29);
          const view = new DataView(events.buffer);
          view.setUint32(0, 0x4259544b, true); view.setUint16(4, protocolVersion, true); view.setUint8(6, 1);
          view.setBigUint64(8, frame, true); view.setUint32(16, 1, true); view.setUint32(20, events.length - 24, true);
          view.setUint8(24, pointer ? 3 : 1); view.setUint16(25, pointer ? 9 : 2, true);
          if (pointer) { view.setFloat32(27, 20, true); view.setFloat32(31, 30, true); view.setUint8(35, 1); }
          else view.setUint16(27, 1, true);
          context.kartyClientUpdate(frame, events);
          assert.equal(elements.get("loading").hidden, true);
          assert.equal(elements.get("loading").attributes["aria-busy"], "false");
          assert.equal(elements.get("fullscreen").hidden, false);
          const tags = commands(frame);
          if (!mock) { assert.ok(tags.includes(4)); if (pointer) assert.ok(tags.includes(9)); }
        }
        assert.throws(() => context.kartyClientUpdate(3n, new Uint8Array(65537)), /invalid client event buffer/);
        imports.karty.submit_commands(0xffffffff, 32);
        assert.equal(submissions[0].length, 0, "invalid guest range must reach host validation as rejected input");
        submissions = [];
        if (mock) {
          eventPointer = 0xffffffff;
          assert.throws(() => context.kartyClientUpdate(3n, new Uint8Array(24)), /invalid client event buffer/);
        }
        context.kartyClientShutdown(); commands(2n);
        resolveDone();
        return new Promise(() => {}); // A running Go host does not exit.
      } catch (error) { rejectDone(error); }
    }
  },
};

try {
  for (const name of ["status", "client", "go", "element"]) {
    Object.defineProperty(context, name, { value: "existing page global", configurable: false });
  }
  runInNewContext(source, context, { filename: "karty.js" });
  await done;
  if (expectedFailure) {
    assert.equal(elements.get("loading").hidden, false);
    assert.equal(elements.get("retry").hidden, false);
    assert.equal(elements.get("progress").hidden, true);
    if (failure) assert.match(elements.get("error-detail").textContent, /HTTP 503/);
    if (soundFailure) {
      assert.match(elements.get("error-detail").textContent, /sound section is duplicated/);
      assert.equal(context.kartySoundBundle, undefined, "duplicate sound sections must not publish a bundle");
    }
    assert.equal(registered, false);
  }
  console.log(`Launcher checks passed (${mock ? `fixture, sounds ${soundSections}` : "actual TinyGo cartridge"}; host rendering mocked).`);
} finally { clearTimeout(timer); }
