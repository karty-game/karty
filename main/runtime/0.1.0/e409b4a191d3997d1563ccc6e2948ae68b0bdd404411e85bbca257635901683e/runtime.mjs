import "./texture-codec.mjs";
import { loadCartridge } from "./client.mjs";

// A timed-out Go await detaches before releasing its js.Func. The promise can
// still settle later without calling back into a released Go function.
globalThis.kartyWebGPUPromiseGate = (promise, callback, disposeLate = false) => {
  let active = callback;
  promise.then(
    (value) => {
      const handler = active;
      active = null;
      if (handler) handler(true, value);
      else if (disposeLate && typeof value?.destroy === "function") value.destroy();
    },
    (error) => {
      const handler = active;
      active = null;
      if (handler) handler(false, String(error?.message ?? error));
    },
  );
  return () => {
    active = null;
  };
};

export async function startHost({ sha256, onStage = () => {}, onError = () => {} }) {
  const started = performance.now();
  globalThis.kartyWebGPUStartup = { stage: "fetch", heartbeat: 0 };
  const heartbeat = setInterval(() => globalThis.kartyWebGPUStartup.heartbeat++, 25);
  const stage = (name) => {
    globalThis.kartyWebGPUStartup.stage = name;
    onStage(name);
  };
  const fail = (error) => {
    globalThis.kartyWebGPUStartupError ??= String(error?.message ?? error);
    onError(error);
  };
  try {
    const bytes = await fetchWASM();
    stage("validate");
    const pin = sha256;
    if (!/^[a-f0-9]{64}$/.test(pin)) throw new Error("Missing valid pinned WASM SHA-256");
    const digest = await crypto.subtle.digest("SHA-256", bytes);
    const actual = Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, "0")).join("");
    if (actual !== pin) throw new Error("WASM SHA-256 does not match this distribution");
    if (typeof Go !== "function") throw new Error("Go runtime glue is unavailable");
    const go = new Go();
    go.argv = ["karty-host-webgpu"];
    stage("compile");
    const module = await WebAssembly.compile(bytes);
    stage("instantiate");
    const instance = await WebAssembly.instantiate(module, go.importObject);
    stage("go-run");
    void go.run(instance).then(() => fail(new Error("Go host exited")), fail);
    const deadline = performance.now() + 9000;
    while (!globalThis.kartyWebGPUReady && !globalThis.kartyWebGPUStartupError) {
      if (performance.now() >= deadline) throw new Error("Go host did not expose its entry point within 9000 ms");
      await new Promise((resolve) => setTimeout(resolve, 10));
    }
    if (globalThis.kartyWebGPUStartupError) throw new Error(globalThis.kartyWebGPUStartupError);
    if (typeof globalThis.kartyWebGPURun !== "function") throw new Error("Go host exposed no rendering entry point");
    globalThis.kartyWebGPUStartup.stage = "ready";
    globalThis.kartyWebGPUStartup.elapsedMs = performance.now() - started;
    globalThis.kartyWebGPUStartup.wasmSHA256 = actual;
    globalThis.kartyWebGPUMountDebug?.();
  } finally {
    clearInterval(heartbeat);
  }
}

async function fetchWASM() {
  for (const name of ["karty-host.wasm", "runtime.bin"]) {
    const url = new URL(name, import.meta.url);
    if (url.origin !== location.origin) throw new Error("WASM must use the page origin");
    globalThis.kartyWebGPUStartup.wasmURL = url.pathname;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 9000);
    try {
      const response = await fetch(url, { cache: "no-store", redirect: "error", signal: controller.signal });
      if (!response.ok) throw new Error(`WASM request returned HTTP ${response.status}`);
      return await response.arrayBuffer();
    } catch (error) {
      // This byte-identical alias preserves the experiment's mobile startup
      // path. Content, hash and compilation errors never trigger a retry.
      if (!(error instanceof TypeError) || name === "runtime.bin") throw error;
    } finally {
      clearTimeout(timer);
      controller.abort();
    }
  }
  throw new Error("WASM response unavailable");
}

// Cartridge clients publish one shared guest ABI bridge. Keep its complete
// replacement lifetime exclusive with lighting checks, including fetch/compile
// waits. A failed request must not poison later operations in request order.
let operation = Promise.resolve();
function serialize(work) {
  const next = operation.then(work);
  operation = next.catch(() => {});
  return next;
}

export function checkLighting(mode = "hardware") {
  return serialize(async () => {
    globalThis.kartyWebGPUStop?.();
    return globalThis.kartyWebGPURun(mode);
  });
}
globalThis.kartyWebGPUCheckLighting = checkLighting;

export function play(value, mode = "hardware") {
  return serialize(() => playOwned(value, mode));
}

async function playOwned(value, mode) {
  if (mode !== "hardware" && mode !== "software") throw new Error("Invalid WebGPU backend");
  const url = new URL(value, location.href);
  if (url.origin !== location.origin || !["http:", "https:"].includes(url.protocol))
    throw new Error("Cartridge must use this page's origin");
  globalThis.kartyWebGPUStop?.();
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 9000);
  let bytes;
  try {
    bytes = await fetchCartridge(url, controller.signal);
  } finally {
    clearTimeout(timer);
  }
  await loadCartridge(bytes);
  await globalThis.kartyWebGPUPlay(bytes, url.href, mode);
}

async function fetchCartridge(url, signal) {
  const response = await fetch(url, { cache: "no-store", signal });
  if (!response.ok || Number(response.headers.get("Content-Length")) > 64 * 1024 * 1024)
    throw new Error("Cartridge request failed or exceeds its size limit");
  const parts = [];
  let size = 0;
  const reader = response.body.getReader();
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > 64 * 1024 * 1024) throw new Error("Cartridge exceeds its size limit");
      parts.push(value);
    }
  } catch (error) {
    await reader.cancel();
    throw error;
  }
  const bytes = new Uint8Array(size);
  let offset = 0;
  for (const part of parts) {
    bytes.set(part, offset);
    offset += part.byteLength;
  }
  return bytes;
}
globalThis.kartyWebGPULoadCartridge = play;
// Appended to runtime.mjs by the host packager; this remains host-owned UI.
const debugWidgets = new WeakMap();

export function debugMetrics(value = {}) {
  if (value.status !== "running") return "Waiting for the game…";
  const ms = (number) => Number(number || 0).toFixed(2);
  return [
    Number(value.fps || 0).toFixed(0) + " FPS · WebGPU · " + value.textureFormat,
    "CPU ms: update " + ms(value.updateMS) + " · prepare " + ms(value.prepareMS) + " · submit " + ms(value.submitMS),
    value.width + " × " + value.height + " · " + value.samples + "× MSAA",
    value.draws + " draws · " + value.instances + " instances · " + value.meshes + " meshes",
    (value.cells || 0) + " resident cells · " + (value.cellCommands || 0) + " cell commands",
    "Cell uploads total: " +
      (value.cellBufferBytes || 0) +
      " buffer B · " +
      (value.cellTextureBytes || 0) +
      " lightmap B · " +
      (value.cellRecordBytes || 0) +
      " dirty-record B",
    "Instances last frame: " +
      value.validatedRecords +
      " records validated · " +
      value.writeBytes +
      " bytes / " +
      value.writeRanges +
      " writes",
    "Prepared: " +
      value.worldRecords +
      " world · " +
      value.actorRecords +
      " actor · " +
      value.instanceSnapshots +
      " snapshots",
    "Visibility: " + value.visibilityFrames + " traversals · " + value.coverageSectors + " coverage sectors",
    "Selection: " + value.drawRecords + " records · " + value.membershipScans + " membership scans",
    "Pending content: " + value.pendingContent,
    [value.vendor, value.device].filter(Boolean).join(" "),
    "CPU spans include host work; GPU elapsed timing is unavailable.",
  ].join("\n");
}

const debugCSS = [
  "#karty-debug-widget{position:fixed;right:max(12px,env(safe-area-inset-right));bottom:max(12px,env(safe-area-inset-bottom));z-index:30;max-width:calc(100vw - 24px);max-height:calc(100dvh - 24px);overflow:auto;border:1px solid #344354;border-radius:8px;background:#0f141bf2;color:#c1ceda;font:12px/1.6 ui-monospace,monospace;display:none}",
  "html[data-karty-state=running] #karty-debug-widget,html[data-karty-state=error] #karty-debug-widget{display:block}",
  "#karty-debug-widget[data-open=true]{width:400px}",
  "#karty-debug-widget *{box-sizing:border-box}",
  "#karty-debug-widget [hidden]{display:none!important}",
  "#karty-debug-controls{display:flex;align-items:center;justify-content:flex-end;gap:4px;padding:4px;background:#0f141b;position:sticky;top:0;z-index:1}",
  "#karty-debug-tabs{display:flex;flex:1;min-width:0}",
  "#karty-debug-widget .karty-control{position:static;min-width:44px;min-height:44px;padding:8px 10px;border:0;border-radius:4px;background:transparent;color:inherit;font:inherit;cursor:pointer;touch-action:manipulation}",
  "#karty-debug-widget .karty-control:focus-visible{outline:2px solid #c1ceda;outline-offset:-2px}",
  "#karty-debug-widget [aria-selected=true]{background:#334454}",
  "#karty-debug-metrics,#karty-debug-log-lines{padding:10px 12px;margin:0;white-space:pre-wrap;overflow-wrap:anywhere;font:inherit}",
  "#karty-debug-log-lines{max-height:45dvh;overflow:auto;user-select:text}",
  "#karty-debug-log-controls{display:flex;align-items:center;gap:6px;padding:4px 12px}",
  "#karty-debug-log-level{min-height:44px;background:#16202a;color:inherit;border:1px solid #344354;border-radius:4px;font:inherit}",
  "#karty-debug-footer{padding:8px 12px;border-top:1px solid #344354}",
  "#karty-debug-footer #offline-download[data-docked]{position:static!important;min-width:0;max-width:none;padding:0;border:0;background:transparent;color:inherit;font:inherit}",
  "#karty-debug-footer #offline-download p{margin:0 0 8px}",
  "#karty-debug-footer #offline-action{min-height:44px;font:inherit}",
  "#karty-debug-footer progress{width:100%;height:6px;accent-color:#c1ceda}",
].join("\n");

export function installDebugWidget({
  document = globalThis.document,
  window = globalThis.window,
  bridge = (...args) => globalThis.kartyHostDebug?.(...args),
} = {}) {
  if (!document?.body || !window) return null;
  if (debugWidgets.has(document)) return debugWidgets.get(document);
  const make = (tag, id, text) => {
    const element = document.createElement(tag);
    if (id) element.id = id;
    if (text) element.textContent = text;
    return element;
  };
  const button = (id, label) => {
    const element = make("button", id, label);
    element.type = "button";
    element.className = "karty-control";
    return element;
  };
  const panel = make("section", "karty-debug-widget");
  panel.setAttribute("aria-label", "Debug tools");
  const style = make("style");
  style.textContent = debugCSS;
  const controls = make("div", "karty-debug-controls");
  const tabs = make("div", "karty-debug-tabs");
  tabs.setAttribute("role", "tablist");
  const statsTab = button("karty-debug-tab-stats", "Stats");
  const logsTab = button("karty-debug-tab-logs", "Logs");
  for (const tab of [statsTab, logsTab]) tab.setAttribute("role", "tab");
  tabs.append(statsTab, logsTab);
  const toggle = button("karty-debug-toggle", "Debug");
  toggle.setAttribute("aria-controls", "karty-debug-content");
  controls.append(tabs, toggle);
  const content = make("div", "karty-debug-content");
  const metrics = make("pre", "karty-debug-metrics");
  const copy = button("karty-debug-copy", "Copy diagnostics");
  const stats = make("div", "karty-debug-stats");
  stats.append(metrics, copy);
  const logs = make("div", "karty-debug-logs");
  const logControls = make("div", "karty-debug-log-controls");
  const level = make("select", "karty-debug-log-level");
  level.setAttribute("aria-label", "Minimum log severity");
  for (const value of ["INFO", "WARN", "ERROR"]) {
    const option = make("option", null, value);
    option.value = value;
    level.append(option);
  }
  const clear = button("karty-debug-log-clear", "Clear");
  logControls.append(level, clear);
  const lines = make("pre", "karty-debug-log-lines");
  logs.append(logControls, lines);
  content.append(stats, logs);
  const footer = make("div", "karty-debug-footer");
  panel.append(style, controls, content, footer);
  document.body.append(panel);

  let opened = false;
  let activeTab = "stats";
  let interval;
  let revision = -1;
  let lastStats = {};
  const focus = () => bridge("focus", opened && panel.contains(document.activeElement));
  const refresh = () => {
    if (!opened) return;
    lastStats = bridge("snapshot") || {};
    metrics.textContent = debugMetrics(lastStats);
    if (activeTab === "logs") {
      const result = bridge("logs", level.value || "INFO", revision);
      if (result && !result.unchanged) {
        revision = result.revision;
        lines.textContent = result.records.map((entry) => entry.text).join("\n") || "No messages.";
      }
    }
  };
  const select = (name) => {
    activeTab = name;
    stats.hidden = name !== "stats";
    logs.hidden = name !== "logs";
    statsTab.setAttribute("aria-selected", name === "stats");
    logsTab.setAttribute("aria-selected", name === "logs");
    refresh();
  };
  const visible = (value) => {
    opened = Boolean(value);
    panel.dataset.open = String(opened);
    content.hidden = tabs.hidden = footer.hidden = !opened;
    toggle.textContent = opened ? "Close" : "Debug";
    toggle.setAttribute("aria-expanded", opened);
    toggle.setAttribute("aria-label", opened ? "Hide debug panel" : "Show debug panel");
    bridge("visible", opened);
    if (interval !== undefined) window.clearInterval(interval);
    interval = undefined;
    if (opened) {
      document.exitPointerLock?.();
      toggle.focus();
      refresh();
      interval = window.setInterval(refresh, 250);
    } else {
      bridge("focus", false);
      if (panel.contains(document.activeElement)) document.getElementById("world")?.focus();
    }
  };
  const dock = () => {
    const fullscreen = document.getElementById("fullscreen");
    if (fullscreen) {
      fullscreen.classList.add("karty-control");
      controls.insertBefore(fullscreen, toggle);
    } else if (document.fullscreenEnabled && !document.getElementById("karty-debug-fullscreen")) {
      const fallback = button("karty-debug-fullscreen", "Fullscreen");
      fallback.addEventListener("click", () => {
        const operation = document.fullscreenElement
          ? document.exitFullscreen()
          : document.documentElement.requestFullscreen();
        operation?.catch(() => {});
      });
      controls.insertBefore(fallback, toggle);
    }
    const offline = document.getElementById("offline-download");
    if (offline) {
      offline.setAttribute("data-docked", "");
      footer.append(offline);
    }
    bridge("visible", opened);
  };
  toggle.addEventListener("click", () => visible(!opened));
  statsTab.addEventListener("click", () => select("stats"));
  logsTab.addEventListener("click", () => select("logs"));
  clear.addEventListener("click", () => {
    bridge("clear-logs");
    revision = -1;
    refresh();
  });
  level.addEventListener("change", () => {
    revision = -1;
    refresh();
  });
  copy.addEventListener("click", async () => {
    try {
      await window.navigator.clipboard.writeText(JSON.stringify(lastStats, null, 2));
      copy.textContent = "Copied";
    } catch {
      copy.textContent = "Copy unavailable";
    }
  });
  for (const name of [
    "pointerdown",
    "pointerup",
    "pointermove",
    "mousedown",
    "mouseup",
    "touchstart",
    "touchend",
    "touchmove",
    "click",
    "keydown",
    "keyup",
    "wheel",
  ])
    panel.addEventListener(name, (event) => event.stopPropagation());
  panel.addEventListener("focusin", focus);
  panel.addEventListener("focusout", () => queueMicrotask(focus));
  document.addEventListener(
    "keydown",
    (event) => {
      if (event.code === "F3" && !event.repeat) {
        event.preventDefault();
        event.stopImmediatePropagation();
        visible(!opened);
      } else if (event.code === "Escape" && opened) {
        event.preventDefault();
        event.stopImmediatePropagation();
        visible(false);
        document.getElementById("world")?.focus();
      }
    },
    true,
  );
  window.addEventListener("karty-host-ready", dock);
  if (document.documentElement.dataset.kartyState === "running") dock();
  select("stats");
  visible(new URLSearchParams(window.location.search).get("debug") === "1");
  const api = { visible, refresh, select, panel };
  debugWidgets.set(document, api);
  globalThis.kartyDebugWidget = (value) => {
    if (typeof value === "boolean") visible(value);
    return { ...bridge("snapshot"), visible: opened };
  };
  return api;
}

globalThis.kartyWebGPUMountDebug = () => installDebugWidget();
