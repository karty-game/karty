import assert from "node:assert/strict";
import { webcrypto, createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import { runInNewContext } from "node:vm";

const script = await readFile(new URL("./browser.js", import.meta.url), "utf8");
function uiAssetBundle(entries) {
  const parts = entries.map(({ name, bytes }) => {
    const key = Buffer.from(name),
      header = Buffer.alloc(8);
    header.writeUInt16LE(key.length);
    header.writeUInt32LE(bytes.length, 4);
    return Buffer.concat([header, key, bytes]);
  });
  const header = Buffer.alloc(16);
  header.write("KTYA");
  header.writeUInt16LE(1, 4);
  header.writeUInt32LE(entries.length, 8);
  header.writeUInt32LE(16 + parts.reduce((sum, part) => sum + part.length, 0), 12);
  return Buffer.concat([header, ...parts]);
}
function uiBytes(templates = [], fonts = []) {
  const templateBytes = uiAssetBundle(templates),
    fontBytes = uiAssetBundle(fonts);
  const header = Buffer.alloc(12);
  header.write("KDU1");
  header.writeUInt32LE(templateBytes.length, 4);
  header.writeUInt32LE(fontBytes.length, 8);
  return Buffer.concat([header, templateBytes, fontBytes]);
}

const emptyUI = uiBytes();
const uiResource = (bytes) => ({
  artifact: `__karty_dev/ui/${hash(bytes)}.bin`,
  contentSha256: hash(bytes),
  size: bytes.length,
});
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");
function generation(value = "a", bytes = Buffer.from("level")) {
  const digest = hash(bytes);
  return {
    revision: value.charCodeAt(0),
    ui: uiResource(emptyUI),
    id: value.repeat(64),
    runtimeIdentity: "d".repeat(64),
    baseURL: `__karty_dev/generations/${value.repeat(64)}/`,
    levels: [
      {
        name: "levels.room",
        kind: "world",
        contentSha256: digest,
        size: bytes.length,
        envelopeVersion: 1,
        artifact: `content/${digest}.kld`,
      },
    ],
  };
}
function fixture({
  initial = null,
  mounts = [],
  pending = 0,
  body = Buffer.from("level"),
  stageError = "",
  onFetch,
  onStage,
  autoAck = true,
  responseFor,
  withoutBridge = false,
  reloadTarget = null,
} = {}) {
  let now = 0,
    next = 0;
  const timers = new Map(),
    streams = [],
    navigations = [],
    staged = [],
    uiStaged = [],
    catalogs = [],
    fetched = [],
    elements = [];
  const bridgeStatus = { mounts, pending, staged: 0, applied: 0, error: "" };
  const schedule = (callback, wait, interval = false) => {
    const id = ++next;
    timers.set(id, { callback, due: now + wait, period: interval ? wait : 0 });
    return id;
  };
  class EventSource {
    constructor(url) {
      this.url = url.href;
      this.listeners = new Map();
      streams.push(this);
    }
    close() {
      this.closed = true;
    }
    addEventListener(name, callback) {
      this.listeners.set(name, callback);
    }
    snapshot(next = null, state = "ready", message = "") {
      this.listeners.get("snapshot")({
        data: JSON.stringify({ schema: "karty.dev@1", session: "session", state, message, generation: next }),
      });
    }
  }
  const storage = new Map();
  if (reloadTarget) storage.set("karty.dev.reload:http://localhost/proxy/35009/", reloadTarget);
  const context = {
    URL,
    EventSource,
    Uint8Array,
    AbortController,
    crypto: webcrypto,
    document: {
      body: { appendChild: (node) => elements.push(node) },
      createElement: () => ({ style: {}, setAttribute() {} }),
    },
    location: { search: "?debug=0", hash: "", replace: (url) => navigations.push(url) },
    sessionStorage: {
      getItem: (key) => storage.get(key),
      setItem: (key, value) => storage.set(key, value),
      removeItem: (key) => storage.delete(key),
    },
    kartyDevelopmentBoot: { appRoot: "http://localhost/proxy/35009/", generation: initial },
    kartyDevelopment: {
      schema: "karty.dev@1",
      status: () => bridgeStatus,
      stageUI: (revision, bytes) => {
        uiStaged.push({ revision, bytes });
        bridgeStatus.staged = revision;
        bridgeStatus.error = "";
        if (autoAck && !stageError) bridgeStatus.applied = revision;
        onStage?.(bridgeStatus);
        return stageError;
      },
      stage: (revision, handle, bytes) => {
        staged.push({ revision, handle, bytes });
        bridgeStatus.staged = revision;
        bridgeStatus.error = "";
        if (autoAck && !stageError) bridgeStatus.applied = revision;
        onStage?.(bridgeStatus);
        return stageError;
      },
    },
    kartyConfigureProject: (_, catalog) => catalogs.push(catalog),
    fetch: async (url, { signal }) => {
      fetched.push(url.href);
      onFetch?.(bridgeStatus);
      const responseBody = responseFor ? await responseFor(url, signal) : body;
      let sent = false;
      return {
        ok: true,
        body: {
          getReader: () => ({
            read: async () => (sent ? { done: true } : ((sent = true), { value: responseBody, done: false })),
            cancel: async () => {},
            releaseLock() {},
          }),
        },
      };
    },
    Date: { now: () => now },
    Math: { min: Math.min, max: Math.max, round: Math.round, random: () => 0.5 },
    setTimeout: schedule,
    setInterval: (callback, wait) => schedule(callback, wait, true),
    clearTimeout: (id) => timers.delete(id),
    clearInterval: (id) => timers.delete(id),
  };
  if (withoutBridge) {
    delete context.kartyDevelopment;
    delete context.kartyConfigureProject;
  }
  runInNewContext(script, context);
  return {
    streams,
    client: context.kartyDevelopmentClient,
    navigations,
    staged,
    uiStaged,
    catalogs,
    fetched,
    elements,
    bridgeStatus,
    advance(milliseconds) {
      const end = now + milliseconds;
      for (;;) {
        const pending = [...timers.entries()]
          .filter(([, timer]) => timer.due <= end)
          .sort((a, b) => a[1].due - b[1].due)[0];
        if (!pending) break;
        const [id, timer] = pending;
        now = timer.due;
        if (timer.period) timer.due += timer.period;
        else timers.delete(id);
        timer.callback();
      }
      now = end;
    },
    async settle() {
      for (let index = 0; index < 20; index++) await new Promise((resolve) => setTimeout(resolve, 1));
    },
  };
}

test("HTTP 524 errors explicitly recreate streams with bounded backoff", () => {
  const { streams, client, advance } = fixture();
  assert.equal(streams[0].url, "http://localhost/proxy/35009/__karty_dev/events");
  for (const wait of [1000, 2000, 4000, 8000, 16000, 30000, 30000]) {
    const current = streams.at(-1),
      count = streams.length;
    current.onerror();
    current.onerror();
    assert.equal(current.closed, true);
    advance(wait - 1);
    assert.equal(streams.length, count);
    advance(1);
    assert.equal(streams.length, count + 1);
  }
  streams.at(-1).snapshot();
  streams.at(-1).onerror();
  const count = streams.length;
  advance(1000);
  assert.equal(streams.length, count + 1);
  client.disconnect();
});

test("named heartbeat sustains inactivity and watchdog detects silently cut streams", () => {
  const { streams, client, advance } = fixture();
  const first = streams[0];
  first.onopen();
  for (let index = 0; index < 5; index++) {
    advance(15000);
    first.listeners.get("heartbeat")();
  }
  assert.equal(streams.length, 1);
  assert.equal(client.state.heartbeats, 5);
  advance(45000);
  assert.equal(first.closed, true);
  assert.equal(client.state.retryReason, "heartbeat timeout");
  advance(1000);
  assert.equal(streams.length, 2);
  client.disconnect();
});

test("shutdown cancels retries and obsolete callbacks", () => {
  const { streams, client, advance } = fixture();
  const first = streams[0];
  first.onerror();
  advance(1000);
  first.onopen();
  first.snapshot();
  assert.equal(client.state.connections, 0);
  streams[1].onerror();
  client.disconnect();
  advance(120000);
  assert.equal(streams.length, 2);
  assert.equal(client.state.connection, "closed");
});

test("one changed mounted level verifies bytes, patches and switches future catalog atomically", async () => {
  const bytes = Buffer.from("edited level"),
    initial = generation(),
    next = generation("b", bytes);
  const f = fixture({ initial, mounts: [{ handle: 7, name: "levels.room" }], body: bytes });
  f.streams[0].snapshot(next);
  await f.settle();
  assert.equal(f.staged.length, 1);
  assert.equal(f.staged[0].handle, 7);
  assert.deepEqual(Buffer.from(f.staged[0].bytes), bytes);
  assert.equal(
    f.catalogs[0]["levels.room"].artifact,
    `http://localhost/proxy/35009/${next.baseURL}${next.levels[0].artifact}`,
  );
  assert.equal(f.client.state.applied, next.id);
  assert.equal(f.navigations.length, 0);
  f.streams[0].snapshot(next);
  await f.settle();
  assert.equal(f.staged.length, 1);
  f.client.disconnect();
});

test("unmounted edited levels update future catalog without downloading or reloading", async () => {
  const f = fixture({ initial: generation() });
  const next = generation("b", Buffer.from("edited level"));
  f.streams[0].snapshot(next);
  await f.settle();
  assert.equal(f.catalogs.length, 1);
  assert.equal(f.fetched.length, 0);
  assert.equal(f.navigations.length, 0);
  f.client.disconnect();
});

test("a changed level mounted during fetch or acknowledgement forces restart", async () => {
  for (const phase of ["fetch", "stage", "released"]) {
    const body = Buffer.from("edited level"),
      initial = generation(),
      next = generation("b", body);
    initial.levels.push({ ...initial.levels[0], name: "levels.other" });
    next.levels.push({ ...next.levels[0], name: "levels.other" });
    const mutate = (status) => {
      if (phase === "released") status.mounts = [];
      else status.mounts.push({ handle: 2, name: "levels.other" });
    };
    const f = fixture({
      initial,
      mounts: [{ handle: 1, name: "levels.room" }],
      body,
      onFetch: phase === "fetch" ? mutate : undefined,
      onStage: phase !== "fetch" ? mutate : undefined,
    });
    f.streams[0].snapshot(next);
    await f.settle();
    assert.equal(f.catalogs.length, 0, `catalog changed after ${phase} mount race`);
    assert.equal(f.navigations.length, 1, phase);
    assert.equal(f.staged.length, phase === "fetch" ? 0 : 1);
  }
});

test("pending loads and changed runtime use one controlled reload", async () => {
  for (const pending of [0, 1]) {
    const f = fixture({ initial: generation(), pending });
    const next = generation("b");
    if (!pending) next.runtimeIdentity = "e".repeat(64);
    f.streams[0].snapshot(next);
    await f.settle();
    assert.deepEqual(f.navigations, ["http://localhost/proxy/35009/?debug=0"]);
    assert.equal(f.staged.length, 0);
  }
});

test("integrity failures never stage partial data and fall back to restart", async () => {
  for (const body of [Buffer.from("corrupt"), Buffer.from("edited level plus overflow")]) {
    const f = fixture({ initial: generation(), mounts: [{ handle: 1, name: "levels.room" }], body });
    f.streams[0].snapshot(generation("b", Buffer.from("edited level")));
    await f.settle();
    assert.equal(f.staged.length, 0);
    assert.equal(f.navigations.length, 1);
  }
});

test("build diagnostic is plain text and retains running generation", () => {
  const initial = generation(),
    f = fixture({ initial });
  f.streams[0].snapshot(initial, "error", "<script>broken build</script>");
  assert.equal(f.elements[0].textContent, "<script>broken build</script>");
  assert.equal(f.client.state.applied, initial.id);
  assert.equal(f.navigations.length, 0);
  f.client.disconnect();
});

test("UI and font resources stage through the shared revision without navigation", async () => {
  const body = uiBytes(
    [{ name: "hud", bytes: Buffer.from("compiled templates") }],
    [{ name: "karty.font/body", bytes: Buffer.from("font bytes") }],
  );
  const initial = generation(),
    next = generation("b");
  next.ui = uiResource(body);
  const f = fixture({ initial, body, mounts: [{ handle: 7, name: "levels.room" }] });
  f.streams[0].snapshot(next);
  await f.settle();
  assert.equal(f.uiStaged.length, 1);
  assert.deepEqual(Buffer.from(f.uiStaged[0].bytes), body);
  assert.equal(f.staged.length, 0);
  assert.equal(f.client.state.applied, next.id);
  assert.equal(f.navigations.length, 0);
  assert.equal(f.fetched[0], `http://localhost/proxy/35009/${next.baseURL}${next.ui.artifact}`);
  f.client.disconnect();
});

test("simultaneous UI and mounted-level changes restart without partial staging", async () => {
  const body = Buffer.from("new UI"),
    initial = generation(),
    next = generation("b", Buffer.from("edited level"));
  next.ui = uiResource(body);
  const f = fixture({ initial, body, mounts: [{ handle: 1, name: "levels.room" }] });
  f.streams[0].snapshot(next);
  await f.settle();
  assert.equal(f.staged.length, 0);
  assert.equal(f.uiStaged.length, 0);
  assert.equal(f.navigations.length, 1);
});

test("invalid UI bytes or rejected staging retain the old generation until restart", async () => {
  for (const rejection of ["integrity", "host"]) {
    const body = Buffer.from("new UI"),
      initial = generation(),
      next = generation("b");
    next.ui = uiResource(body);
    const f = fixture({
      initial,
      body: rejection === "integrity" ? Buffer.from("corrupt") : body,
      stageError: rejection === "host" ? "UI topology changed" : "",
    });
    f.streams[0].snapshot(next);
    await f.settle();
    assert.equal(f.uiStaged.length, rejection === "integrity" ? 0 : 1);
    assert.equal(f.client.state.applied, initial.id);
    assert.equal(f.navigations.length, 1);
  }
});

function resourceEdit(kind, value = "b") {
  const initial = generation(),
    editedBytes =
      kind === "level" ? Buffer.from(`edited level ${value}`) : uiBytes([{ name: "hud", bytes: Buffer.from(value) }]);
  const next = generation(value, kind === "level" ? editedBytes : Buffer.from("level"));
  if (kind === "ui") next.ui = uiResource(editedBytes);
  const reverted = { ...initial, revision: next.revision + 1 };
  return { initial, next, reverted, editedBytes, initialBytes: kind === "level" ? Buffer.from("level") : emptyUI };
}

test("returning to the applied generation aborts obsolete level and UI fetches", async () => {
  for (const kind of ["level", "ui"]) {
    const edit = resourceEdit(kind);
    let pendingSignal;
    const f = fixture({
      initial: edit.initial,
      mounts: [{ handle: 1, name: "levels.room" }],
      responseFor: async (_, signal) => {
        pendingSignal = signal;
        return new Promise((_, reject) =>
          signal.addEventListener("abort", () => reject(new Error("aborted obsolete fetch")), { once: true }),
        );
      },
    });
    f.streams[0].snapshot(edit.next);
    await f.settle();
    assert.ok(pendingSignal);
    f.streams[0].snapshot(edit.reverted);
    await f.settle();
    assert.equal(pendingSignal.aborted, true);
    assert.equal(f.staged.length + f.uiStaged.length, 0);
    assert.equal(f.client.state.applied, edit.initial.id);
    assert.equal(f.catalogs.length, 0);
    assert.equal(f.navigations.length, 0);
    f.client.disconnect();
  }
});

test("already staged level and UI edits acknowledge before reconciling a rapid undo", async () => {
  for (const kind of ["level", "ui"]) {
    const edit = resourceEdit(kind);
    const f = fixture({
      initial: edit.initial,
      mounts: [{ handle: 1, name: "levels.room" }],
      autoAck: false,
      responseFor: async (url) => (url.href.includes(hash(edit.editedBytes)) ? edit.editedBytes : edit.initialBytes),
    });
    f.streams[0].snapshot(edit.next);
    await f.settle();
    const stages = kind === "level" ? f.staged : f.uiStaged;
    assert.equal(stages.length, 1);
    f.streams[0].snapshot(edit.reverted);
    await f.settle();
    assert.equal(stages.length, 1, "the queued host transaction must be joined");
    assert.equal(f.catalogs.length, 0, "an obsolete generation must never publish its future catalog");
    f.bridgeStatus.applied = f.bridgeStatus.staged;
    f.advance(50);
    await f.settle();
    assert.equal(stages.length, 2, "undo must restore resources applied by the obsolete transaction");
    assert.deepEqual(Buffer.from(stages[1].bytes), edit.initialBytes);
    assert.equal(f.client.state.applied, edit.initial.id);
    f.bridgeStatus.applied = f.bridgeStatus.staged;
    f.advance(50);
    await f.settle();
    assert.equal(f.catalogs.length, 1);
    assert.equal(f.navigations.length, 0);
    assert.equal(f.client.state.generation.revision, edit.reverted.revision);
    f.client.disconnect();
  }
});

test("superseded rejected transactions retain the applied resources; lost acknowledgements restart latest", async () => {
  for (const kind of ["level", "ui"]) {
    for (const failure of ["rejected", "timeout"]) {
      const edit = resourceEdit(kind);
      const f = fixture({
        initial: edit.initial,
        mounts: [{ handle: 1, name: "levels.room" }],
        autoAck: false,
        body: edit.editedBytes,
      });
      f.streams[0].snapshot(edit.next);
      await f.settle();
      f.streams[0].snapshot(edit.reverted);
      if (failure === "rejected") f.bridgeStatus.error = "transaction rejected without publication";
      f.advance(failure === "timeout" ? 15000 : 50);
      await f.settle();
      assert.equal(f.staged.length + f.uiStaged.length, 1);
      assert.equal(f.client.state.applied, edit.initial.id);
      assert.equal(f.navigations.length, failure === "timeout" ? 1 : 0);
      f.client.disconnect();
    }
  }
});

test("a rejected obsolete host transaction does not block a newer level or UI target", async () => {
  for (const kind of ["level", "ui"]) {
    const edit = resourceEdit(kind),
      newest = resourceEdit(kind, "c");
    const f = fixture({
      initial: edit.initial,
      mounts: [{ handle: 1, name: "levels.room" }],
      autoAck: false,
      responseFor: async (url) => (url.href.includes(hash(edit.editedBytes)) ? edit.editedBytes : newest.editedBytes),
    });
    f.streams[0].snapshot(edit.next);
    await f.settle();
    f.streams[0].snapshot(newest.next);
    f.bridgeStatus.error = "old transaction rejected";
    f.advance(50);
    await f.settle();
    const stages = kind === "level" ? f.staged : f.uiStaged;
    assert.equal(stages.length, 2);
    assert.deepEqual(Buffer.from(stages[1].bytes), newest.editedBytes);
    f.bridgeStatus.applied = f.bridgeStatus.staged;
    f.advance(50);
    await f.settle();
    assert.equal(f.client.state.applied, newest.next.id);
    assert.equal(f.catalogs.length, 1);
    assert.equal(f.navigations.length, 0);
    f.client.disconnect();
  }
});

test("the initial snapshot is a no-op before the host and launcher are ready", async () => {
  const initial = generation(),
    f = fixture({ initial, withoutBridge: true });
  f.streams[0].snapshot(initial);
  await f.settle();
  assert.equal(f.navigations.length, 0);
  assert.equal(f.client.state.error, "");
  assert.equal(f.client.state.applied, initial.id);
  assert.equal(f.fetched.length, 0);
  f.client.disconnect();
});

test("a successful target boot clears its old reload guard for a later lost acknowledgement", async () => {
  const edit = resourceEdit("level");
  const f = fixture({
    initial: edit.initial,
    reloadTarget: edit.initial.id,
    mounts: [{ handle: 1, name: "levels.room" }],
    autoAck: false,
    body: edit.editedBytes,
  });
  f.streams[0].snapshot(edit.next);
  await f.settle();
  f.streams[0].snapshot(edit.reverted);
  f.advance(15000);
  await f.settle();
  assert.equal(f.navigations.length, 1);
  f.client.disconnect();
});

test("a reload that still serves older HTML retains its loop guard", async () => {
  const initial = generation(),
    next = generation("b");
  next.runtimeIdentity = "e".repeat(64);
  const f = fixture({ initial, reloadTarget: next.id });
  f.streams[0].snapshot(next);
  await f.settle();
  assert.equal(f.navigations.length, 0);
  assert.match(f.client.state.error, /Refresh the preview to retry/);
  f.client.disconnect();
});
