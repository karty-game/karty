(() => {
  "use strict";
  const boot = globalThis.kartyDevelopmentBoot;
  const appRoot = new URL(boot.appRoot);
  const state = {
    schema: "karty.dev@1",
    connection: "connecting",
    connections: 0,
    retries: 0,
    heartbeats: 0,
    error: "",
    applied: boot.generation?.id || null,
    generation: boot.generation || null,
  };
  let applied = boot.generation?.id ? boot.generation : null;
  let wanted = null,
    source = null,
    retryTimer = null,
    stopped = false,
    attempts = 0,
    lastActivity = Date.now(),
    ticket = 0,
    localRevision = 0;
  let activeFetch = null,
    desired = null,
    applying = false,
    hostStagePending = false;
  const levelResources = new Map();
  let uiResourceHash = applied?.ui?.contentSha256;
  const reloadKey = "karty.dev.reload:" + appRoot.href;
  try {
    // A matching boot proves the previous navigation reached its target. Keep
    // guarding stale HTML, while allowing a later independent recovery.
    if (applied?.id && sessionStorage.getItem(reloadKey) === applied.id) sessionStorage.removeItem(reloadKey);
  } catch (_) {
    /* Storage may be unavailable. */
  }
  const delay = () => new Promise((accept) => setTimeout(accept, 50));
  let panel;
  function diagnostic(message) {
    state.error = message;
    if (!panel && document.body) {
      panel = document.createElement("pre");
      panel.id = "karty-development-status";
      panel.setAttribute("role", "status");
      panel.style.cssText =
        "position:fixed;left:12px;top:12px;right:12px;z-index:2147483647;max-height:35vh;overflow:auto;white-space:pre-wrap;background:#10131cee;color:#edf1fa;padding:12px;font:13px monospace;border:1px solid #526581;pointer-events:none";
      document.body.appendChild(panel);
    }
    if (panel) {
      panel.textContent = message;
      panel.hidden = !message;
    }
  }
  function retry(reason) {
    if (stopped || retryTimer !== null) return;
    source?.close();
    source = null;
    state.connection = "reconnecting";
    state.retryReason = reason;
    state.retries++;
    const wait = Math.min(
      30000,
      Math.round(Math.min(1000 * 2 ** Math.min(attempts++, 5), 30000) * (0.8 + Math.random() * 0.4)),
    );
    retryTimer = setTimeout(() => {
      retryTimer = null;
      connect();
    }, wait);
  }
  function connect() {
    if (stopped) return;
    state.connection = "connecting";
    lastActivity = Date.now();
    const stream = new EventSource(new URL("__karty_dev/events", appRoot));
    source = stream;
    stream.onopen = () => {
      if (source === stream && !stopped) {
        state.connections++;
        state.connection = "open";
        lastActivity = Date.now();
      }
    };
    stream.onerror = () => {
      if (source === stream) retry("event stream error");
    };
    stream.addEventListener("heartbeat", () => {
      if (source === stream && !stopped) {
        lastActivity = Date.now();
        state.heartbeats++;
      }
    });
    stream.addEventListener("snapshot", (event) => {
      if (source !== stream || stopped) return;
      lastActivity = Date.now();
      attempts = 0;
      try {
        snapshot(JSON.parse(event.data));
      } catch (error) {
        diagnostic(String(error.message));
      }
    });
  }
  const watchdog = setInterval(() => {
    if (source && Date.now() - lastActivity >= 45000) retry("heartbeat timeout");
  }, 5000);
  function disconnect() {
    stopped = true;
    ticket++;
    activeFetch?.abort();
    clearInterval(watchdog);
    if (retryTimer !== null) clearTimeout(retryTimer);
    source?.close();
    source = null;
    state.connection = "closed";
  }
  globalThis.kartyDevelopmentClient = { state, disconnect };
  globalThis.addEventListener?.("pagehide", disconnect, { once: true });
  connect();
  function valid(next) {
    return (
      next &&
      Number.isSafeInteger(next.revision) &&
      next.revision > 0 &&
      /^[a-f0-9]{64}$/.test(next.id) &&
      /^[a-f0-9]{64}$/.test(next.runtimeIdentity) &&
      next.baseURL === `__karty_dev/generations/${next.id}/` &&
      next.ui &&
      /^[a-f0-9]{64}$/.test(next.ui.contentSha256) &&
      next.ui.artifact === `__karty_dev/ui/${next.ui.contentSha256}.bin` &&
      Number.isSafeInteger(next.ui.size) &&
      next.ui.size > 0 &&
      next.ui.size <= 32 * 1024 * 1024 + 12 &&
      Array.isArray(next.levels) &&
      next.levels.length <= 4096 &&
      next.levels.every(
        (level) =>
          typeof level.name === "string" &&
          typeof level.kind === "string" &&
          /^[a-f0-9]{64}$/.test(level.contentSha256) &&
          Number.isSafeInteger(level.size) &&
          level.size > 0 &&
          level.size <= 32 * 1024 * 1024 &&
          level.artifact === `content/${level.contentSha256}.kld`,
      )
    );
  }
  function snapshot(update) {
    if (update.schema !== state.schema || typeof update.session !== "string")
      throw new Error("Unsupported development stream");
    state.server = update;
    diagnostic(update.state === "error" ? update.message : update.state === "building" ? update.message : "");
    const next = update.generation;
    if (!next) return;
    if (!valid(next)) throw new Error("Invalid development generation");
    const key = `${update.session}:${next.revision}:${next.id}`;
    if (wanted === key) return;
    wanted = key;
    const current = ++ticket;
    activeFetch?.abort();
    desired = { next, current };
    void reconcile();
  }
  async function reconcile() {
    if (applying) return;
    applying = true;
    try {
      while (desired && !stopped) {
        const job = desired;
        desired = null;
        try {
          await apply(job.next, job.current);
        } catch (error) {
          if (stopped) return;
          if (hostStagePending) {
            // A queued transaction cannot be canceled from the browser. If its
            // acknowledgement is lost, restart into the latest complete build.
            hostStagePending = false;
            reload(desired?.next ?? job.next, "Live update acknowledgement was lost");
            return;
          }
          if (job.current !== ticket) continue;
          state.patchError = String(error.message);
          reload(job.next, "Live update requires a game restart");
        }
      }
    } finally {
      applying = false;
    }
  }

  function reload(next, reason) {
    // Remember a failed target across navigation to prevent repeated reloads
    // when a host cannot accept a replacement or the page is still starting.
    try {
      if (sessionStorage.getItem(reloadKey) === next.id) {
        diagnostic(reason + ". Refresh the preview to retry.");
        return;
      }
      sessionStorage.setItem(reloadKey, next.id);
    } catch (_) {
      /* Storage may be unavailable; each boot still pins next.id. */
    }
    disconnect();
    const target = new URL(appRoot);
    target.search = location.search;
    target.hash = location.hash;
    location.replace(target.href);
  }
  function catalog(next) {
    const base = new URL(next.baseURL, appRoot);
    return Object.fromEntries(
      next.levels.map((level) => [level.name, { ...level, artifact: new URL(level.artifact, base).href }]),
    );
  }
  function sameChangedMounts(status, changedNames, expected) {
    const current = status.mounts.filter((mount) => changedNames.has(mount.name));
    return (
      current.length === expected.length &&
      expected.every((mount) => current.some((item) => item.handle === mount.handle && item.name === mount.name))
    );
  }
  async function apply(next, current) {
    if (
      applied?.id === next.id &&
      levelResources.size === 0 &&
      uiResourceHash === next.ui.contentSha256 &&
      !hostStagePending
    ) {
      // Joining earlier work happens in reconcile before this shortcut. The
      // initial SSE snapshot can arrive before the host/launcher has started.
      applied = next;
      state.generation = next;
      return;
    }
    if (!applied || applied.runtimeIdentity !== next.runtimeIdentity) {
      reload(next, "Game code changed");
      return;
    }
    // Launcher and host must both be ready. Updates during startup restart into
    // the complete current generation instead of racing its pending requests.
    const bridge = globalThis.kartyDevelopment;
    if (bridge?.schema !== state.schema || typeof globalThis.kartyConfigureProject !== "function") {
      reload(next, "Runtime does not support live updates");
      return;
    }
    let status = bridge.status();
    if (status.pending) {
      reload(next, "A level is loading");
      return;
    }
    const previous = new Map(applied.levels.map((level) => [level.name, level]));
    const nextLevels = new Map(next.levels.map((level) => [level.name, level]));
    for (const [handle, resource] of levelResources) {
      if (!status.mounts.some((mount) => mount.handle === handle && mount.name === resource.name))
        levelResources.delete(handle);
    }
    const actualLevelHash = (mount) =>
      levelResources.get(mount.handle)?.contentSha256 ?? previous.get(mount.name)?.contentSha256;
    const changed = next.levels.filter((level) => previous.get(level.name)?.contentSha256 !== level.contentSha256);
    const changedNames = new Set(changed.map((level) => level.name));
    const live = status.mounts.filter((mount) => actualLevelHash(mount) !== nextLevels.get(mount.name)?.contentSha256);
    for (const mount of live) changedNames.add(mount.name);
    const expectedMounts = status.mounts.filter((mount) => changedNames.has(mount.name));
    const uiChanged = uiResourceHash !== next.ui.contentSha256;
    if (uiChanged && live.length) {
      reload(next, "UI and mounted level resources changed together");
      return;
    }
    if (live.length > 1) {
      reload(next, "Several mounted levels changed");
      return;
    }
    if (live.length) {
      const level = nextLevels.get(live[0].name);
      const controller = new AbortController();
      activeFetch = controller;
      const deadline = setTimeout(() => controller.abort(), 30000);
      let bytes;
      try {
        bytes = await artifact(new URL(level.artifact, new URL(next.baseURL, appRoot)), level, controller.signal);
      } finally {
        clearTimeout(deadline);
      }
      if (current !== ticket || stopped) return;
      status = bridge.status();
      if (status.pending || !sameChangedMounts(status, changedNames, expectedMounts))
        throw new Error("Mounted levels changed while preparing the update");
      localRevision = Math.max(localRevision, status.staged || 0, status.applied || 0) + 1;
      const error = bridge.stage(localRevision, live[0].handle, bytes);
      if (error) throw new Error(error);
      hostStagePending = true;
      const acknowledgedBy = Date.now() + 15000;
      while (bridge.status().applied < localRevision) {
        if (stopped) return;
        status = bridge.status();
        if (status.error) {
          hostStagePending = false;
          throw new Error(status.error);
        }
        if (Date.now() >= acknowledgedBy) throw new Error("Live update acknowledgement timed out");
        await delay();
      }
      hostStagePending = false;
      levelResources.set(live[0].handle, { name: live[0].name, contentSha256: level.contentSha256 });
      if (current !== ticket || stopped) return;
    }
    if (uiChanged) {
      if (typeof bridge.stageUI !== "function") {
        reload(next, "Runtime does not support live UI updates");
        return;
      }
      const controller = new AbortController();
      activeFetch = controller;
      const deadline = setTimeout(() => controller.abort(), 30000);
      let bytes;
      try {
        bytes = await artifact(new URL(next.ui.artifact, new URL(next.baseURL, appRoot)), next.ui, controller.signal);
      } finally {
        clearTimeout(deadline);
      }
      if (current !== ticket || stopped) return;
      status = bridge.status();
      if (status.pending || !sameChangedMounts(status, changedNames, expectedMounts))
        throw new Error("Mounted levels changed while preparing UI resources");
      localRevision = Math.max(localRevision, status.staged || 0, status.applied || 0) + 1;
      const error = bridge.stageUI(localRevision, bytes);
      if (error) throw new Error(error);
      hostStagePending = true;
      const acknowledgedBy = Date.now() + 15000;
      while (bridge.status().applied < localRevision) {
        if (stopped) return;
        status = bridge.status();
        if (status.error) {
          hostStagePending = false;
          throw new Error(status.error);
        }
        if (Date.now() >= acknowledgedBy) throw new Error("Live UI acknowledgement timed out");
        await delay();
      }
      hostStagePending = false;
      uiResourceHash = next.ui.contentSha256;
    }
    if (current !== ticket || stopped) return;
    status = bridge.status();
    if (status.pending || !sameChangedMounts(status, changedNames, expectedMounts))
      throw new Error("Mounted levels changed before publication");
    globalThis.kartyConfigureProject({}, catalog(next));
    applied = next;
    levelResources.clear();
    state.applied = next.id;
    state.generation = next;
    state.error = "";
    if (state.server?.state === "ready") diagnostic("");
  }
  async function artifact(url, level, signal) {
    const response = await fetch(url, { signal });
    if (!response.ok || !response.body?.getReader) throw new Error("Could not fetch the updated level");
    const bytes = new Uint8Array(level.size),
      reader = response.body.getReader();
    let offset = 0;
    try {
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        if (value.byteLength > bytes.length - offset) {
          await reader.cancel();
          throw new Error("Level exceeds its declared size");
        }
        bytes.set(value, offset);
        offset += value.byteLength;
      }
    } finally {
      reader.releaseLock();
    }
    const hash = [...new Uint8Array(await crypto.subtle.digest("SHA-256", bytes))]
      .map((value) => value.toString(16).padStart(2, "0"))
      .join("");
    if (offset !== level.size || hash !== level.contentSha256) throw new Error("Updated level integrity check failed");
    return bytes;
  }
})();
