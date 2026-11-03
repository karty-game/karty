import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { runInNewContext } from "node:vm";

const html = await readFile(process.argv[2] || new URL("../templates/index-webgpu.html.tmpl", import.meta.url), "utf8");
const inline = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].map((match) => match[1]).join("\n");
const settle = () => new Promise((resolve) => setImmediate(resolve));
assert.match(html, /class="card"/);
assert.match(html, /Powered by Karty/);
assert.ok(
  html.indexOf('id="offline-download"') < html.indexOf("</main>"),
  "undocked offline controls stay inside startup UI",
);
assert.match(html, /#fullscreen\[data-docked\]/);
assert.doesNotMatch(html, /MutationObserver/);

function fixture(mode = "success", prefetchFails = false) {
  const listeners = new Map();
  const scripts = [];
  const timers = new Map();
  const nodes = new Map();
  let select,
    reloads = 0,
    focused = 0,
    prefetches = 0;
  for (const id of ["world", "loading", "status", "progress", "error-detail", "retry", "game-title", "fullscreen"]) {
    nodes.set(id, {
      textContent: id === "game-title" ? "A test game" : "Loading",
      hidden: ["error-detail", "retry", "fullscreen"].includes(id),
      attributes: {},
      handlers: new Map(),
      setAttribute(name, value) {
        this.attributes[name] = value;
      },
      addEventListener(name, callback) {
        this.handlers.set(name, callback);
      },
      focus() {
        focused++;
      },
    });
  }
  const ready = new Promise((resolve, reject) => {
    select = { resolve, reject };
  });
  const document = {
    title: "Loading",
    fullscreenEnabled: true,
    fullscreenElement: null,
    documentElement: {
      dataset: {},
      async requestFullscreen() {
        document.fullscreenElement = document.documentElement;
      },
    },
    async exitFullscreen() {
      document.fullscreenElement = null;
    },
    addEventListener(name, callback) {
      listeners.set(name, callback);
    },
    getElementById: (id) => nodes.get(id),
    createElement: () => ({}),
    body: {
      appendChild(script) {
        scripts.push(script);
        queueMicrotask(() => {
          if (mode === "module-error" && script.type === "module") script.onerror();
          else script.onload();
        });
      },
    },
  };
  const environment = {
    document,
    window: {
      addEventListener(name, callback) {
        listeners.set(name, callback);
      },
    },
    location: {
      reload() {
        reloads++;
      },
    },
    setTimeout(callback) {
      timers.set(1, callback);
      return 1;
    },
    clearTimeout(id) {
      timers.delete(id);
    },
    KartyWebRuntime: {
      ready,
      preparing: true,
      startPrefetch() {
        prefetches++;
        return prefetchFails ? Promise.reject(new Error("offline unavailable")) : Promise.resolve();
      },
    },
    btoa: (value) => Buffer.from(value, "binary").toString("base64"),
  };
  runInNewContext(inline, environment);
  return {
    environment,
    document,
    nodes,
    scripts,
    timers,
    select(cached = false) {
      environment.KartyWebRuntime.preparing = false;
      if (mode === "delivery-error") select.reject(new Error("Pinned release unavailable"));
      else
        select.resolve({
          status: { cachedFiles: cached ? 11 : 0 },
          startupFiles: ["wasm_exec.js", "bootstrap.mjs"].map((url) => ({ url, sha256: "ab".repeat(32) })),
        });
    },
    event(name, detail) {
      listeners.get(name)?.({ detail });
    },
    prefetches: () => prefetches,
    focused: () => focused,
    reloads: () => reloads,
  };
}

for (const mode of ["success", "delivery-error", "module-error"]) {
  const run = fixture(mode);
  assert.equal(run.scripts.length, 0, "runtime must wait for release selection");
  assert.equal(run.nodes.get("loading").hidden, false);
  assert.equal(run.prefetches(), 0, "offline downloads wait for the presented game");
  run.select();
  await settle();
  if (mode === "delivery-error") {
    assert.equal(run.scripts.length, 0);
    assert.equal(run.document.documentElement.dataset.kartyState, "error");
    assert.match(run.nodes.get("error-detail").textContent, /Pinned release unavailable/);
  } else {
    assert.deepEqual(
      run.scripts.map((script) => script.src),
      ["wasm_exec.js", "bootstrap.mjs"],
    );
    assert.equal(run.scripts[0].type, undefined);
    assert.equal(run.scripts[1].type, "module");
    assert.ok(run.scripts.every((script) => script.integrity.startsWith("sha256-")));
    if (mode === "module-error") {
      assert.equal(run.document.documentElement.dataset.kartyState, "error");
      assert.match(run.nodes.get("error-detail").textContent, /Could not load bootstrap.mjs/);
    }
  }
  if (mode !== "success") {
    assert.equal(run.nodes.get("loading").hidden, false);
    assert.equal(run.nodes.get("progress").hidden, true);
    assert.equal(run.nodes.get("retry").hidden, false);
    const first = run.nodes.get("error-detail").textContent;
    run.event("karty-host-error", { message: "secondary error" });
    run.event("karty-host-ready");
    assert.equal(run.nodes.get("error-detail").textContent, first);
    assert.equal(run.nodes.get("loading").hidden, false, "late ready must not hide a real failure");
    run.nodes.get("retry").onclick();
    assert.equal(run.reloads(), 1);
  }
}

for (const cached of [false, true]) {
  const run = fixture("success", true);
  const originalElement = run.document.getElementById;
  run.document.getElementById = (id) => (id === "fullscreen" ? null : originalElement(id));
  run.select(cached);
  await settle();
  assert.equal(run.nodes.get("loading").hidden, false, "script evaluation alone is not a presented game");
  run.event("karty-host-stage", { stage: "compile" });
  assert.equal(run.nodes.get("status").textContent, "Preparing the engine…");
  assert.equal(run.prefetches(), 0);
  run.event("karty-host-ready");
  await settle();
  assert.equal(run.nodes.get("loading").hidden, true);
  assert.equal(run.nodes.get("loading").attributes["aria-busy"], "false");
  assert.equal(run.nodes.get("fullscreen").hidden, false);
  assert.equal(run.document.title, "A test game");
  assert.equal(run.focused(), 1);
  assert.equal(run.prefetches(), 1);
  assert.equal(
    run.document.documentElement.dataset.kartyState,
    "running",
    "background cache failures cannot stop gameplay",
  );
  assert.equal(run.timers.size, 0);
  run.event("karty-host-ready");
  await settle();
  assert.equal(run.prefetches(), 1, "duplicate lifecycle signals cannot restart downloads or move focus");
  assert.equal(run.focused(), 1);
  await run.nodes.get("fullscreen").handlers.get("click")();
  run.event("fullscreenchange");
  assert.equal(run.nodes.get("fullscreen").attributes["aria-label"], "Exit fullscreen");
  await run.nodes.get("fullscreen").handlers.get("click")();
  run.event("fullscreenchange");
  assert.equal(run.nodes.get("fullscreen").attributes["aria-label"], "Fullscreen");
}

{
  const run = fixture();
  run.select(true);
  await settle();
  run.event("karty-host-ready");
  assert.equal(run.nodes.get("loading").hidden, true, "current host readiness event opens the game");
  run.event("karty-host-error", { message: "Device lost" });
  assert.equal(run.nodes.get("loading").hidden, false);
  assert.equal(run.nodes.get("error-detail").textContent, "Device lost");
}
{
  const run = fixture();
  const watchdog = run.timers.get(1);
  watchdog();
  assert.equal(run.nodes.get("retry").hidden, true, "an actively preparing release is not a stalled launcher");
  run.environment.KartyWebRuntime.preparing = false;
  watchdog();
  assert.equal(run.nodes.get("retry").hidden, false);
  assert.match(run.nodes.get("error-detail").textContent, /launcher did not start/);
}
