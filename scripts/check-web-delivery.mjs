// Actual Chromium + generated distribution + worker coverage. The tiny WASM
// fixture checks delivery, not graphics; check-web separately runs Go/TinyGo.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { cp, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { startDeliveryServer } from "./check-web-delivery-server.mjs";

const argument = (name, fallback) => {
  const at = process.argv.indexOf(name);
  return at < 0 ? fallback : process.argv[at + 1];
};
const sourceFixtures = resolve(argument("--fixtures", "dist/validation/web-delivery-fixtures"));
const fixtures = await mkdtemp(resolve(tmpdir(), "karty-web-delivery-"));
await cp(sourceFixtures, fixtures, { recursive: true });
const output = resolve(argument("--output", "dist/validation/web-delivery/results.json"));
const readRelease = async (name) => {
  const directory = resolve(fixtures, name);
  const pointer = JSON.parse(await readFile(resolve(directory, "karty-latest.json"), "utf8"));
  const bytes = await readFile(resolve(directory, pointer.manifest));
  assert.equal(createHash("sha256").update(bytes).digest("hex"), pointer.release);
  return { pointer, manifest: JSON.parse(bytes) };
};
const [a, b] = await Promise.all([readRelease("a"), readRelease("b")]);
assert.notEqual(a.pointer.release, b.pointer.release);
const backgrounds = (release) => release.manifest.files.filter((file) => file.role === "background");
const shared = backgrounds(a).find((file) => backgrounds(b).some((other) => other.sha256 === file.sha256));
const old = backgrounds(a).find((file) => !backgrounds(b).some((other) => other.sha256 === file.sha256));
const changed = backgrounds(b).find((file) => !backgrounds(a).some((other) => other.sha256 === file.sha256));
assert.ok(shared && old && changed, "fixture needs shared and changed content");
const server = await startDeliveryServer(fixtures);
const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
const report = {
  passed: false,
  scope: "Real Chromium worker/fetch/cache execution; no graphics or device qualification",
  checks: [],
  failures: [],
};
const contexts = new Set();
const extraServers = [];
const deadline = setTimeout(() => {
  console.error("Web delivery checks exceeded 120 seconds");
  process.exit(1);
}, 120000);
const ready = async (page) => {
  await page.waitForFunction(() => !!globalThis.KartyWebRuntime, null, { timeout: 9000 });
  return page.evaluate(() => globalThis.KartyWebRuntime.ready);
};
const open = async (context, prefix = "/game/") => {
  const page = await context.newPage();
  await page.goto(server.origin + prefix, { waitUntil: "domcontentloaded", timeout: 9000 });
  return { page, state: await ready(page) };
};
const context = async () => {
  const value = await browser.newContext();
  contexts.add(value);
  return value;
};
const status = (page) => page.evaluate(() => globalThis.KartyWebRuntime.status());
const prefetch = (page) => page.evaluate(() => globalThis.KartyWebRuntime.startPrefetch());
const complete = async (page) => {
  await prefetch(page);
  await waitUntil(async () => (await status(page)).ready === true, "background prefetch did not become ready");
  return status(page);
};
const check = async (name, fn) => {
  try {
    await fn();
    report.checks.push(name);
    console.log(`PASS ${name}`);
  } catch (error) {
    report.failures.push({ name, error: String(error.stack || error) });
    if (!process.argv.includes("--keep-going")) throw error;
    console.error(`FAIL ${name}: ${error.message}`);
    server.setOnline(true);
    server.faults.clear();
  }
};
const fetchBytes = (page, path, options = {}) =>
  page.evaluate(
    async ({ url, options }) => {
      const response = await fetch(url, options);
      if (!response.ok) throw new Error(`fetch ${response.status}`);
      return [...new Uint8Array(await response.arrayBuffer())];
    },
    { url: path, options },
  );
const storedState = (page) =>
  page.evaluate(async () => {
    const scope = new URL("./", location.href);
    const cache = await caches.open("karty-web-v1:" + scope.href + ":metadata");
    return (await cache.match(new URL(".karty-cache/state", scope)))?.json();
  });
const waitUntil = async (predicate, message) => {
  const end = Date.now() + 9000;
  while (!(await predicate())) {
    assert.ok(Date.now() < end, message);
    await new Promise((done) => setTimeout(done, 10));
  }
};
try {
  const session = await context();
  const first = await open(session);
  await check("initial selection verifies and pins release A", async () => {
    assert.equal(first.state.release, a.pointer.release);
    const selected = await status(first.page);
    assert.ok(selected);
  });
  await check("background prefetch finishes with native HTTP gzip", async () => {
    await complete(first.page);
    assert.ok(server.requests.some((entry) => entry.path === "/game/" + shared.url && entry.encodedBytes));
    assert.equal(
      server.requests.some((entry) => /\.(br|gz)$/.test(entry.path)),
      false,
    );
  });
  const sharedCount = server.requests.filter((entry) => entry.path === "/game/" + shared.url).length;
  const independent = await open(session, "/other/");
  await complete(independent.page);
  const otherCacheNames = await independent.page.evaluate(async () => (await caches.keys()).sort());
  server.setRelease("/game/", "b");
  let second;
  await check("online startup selects B while open A remains pinned", async () => {
    second = await open(session);
    assert.equal(second.state.release, b.pointer.release);
    assert.equal((await status(first.page)).release, a.pointer.release);
    assert.deepEqual(await fetchBytes(first.page, old.url), [...(await readFile(resolve(fixtures, "a", old.url)))]);
  });
  await check("failed B prefetch preserves complete A for offline restart", async () => {
    server.faults.set("/game/" + changed.url, { status: 503 });
    await second.page.evaluate(() => {
      globalThis.deliveryEvents = [];
      window.addEventListener("karty-web-status", (event) => globalThis.deliveryEvents.push(event.detail));
    });
    await prefetch(second.page).catch(() => {});
    await second.page.waitForFunction(() => globalThis.deliveryEvents.some((event) => event.error), null, {
      timeout: 9000,
    });
    server.setOnline(false);
    const offline = await open(session);
    assert.equal(offline.state.release, a.pointer.release);
    await offline.page.close();
    server.setOnline(true);
    server.faults.clear();
  });
  await check("B completion reuses unchanged content without downloading it", async () => {
    await complete(second.page);
    report.afterBCompletion = await storedState(second.page);
    assert.equal(server.requests.filter((entry) => entry.path === "/game/" + shared.url).length, sharedCount);
  });
  await check("live A and B pins recover after worker restart and missing pin metadata", async () => {
    await first.page.evaluate(async () => {
      const scope = new URL("./", location.href);
      const cache = await caches.open("karty-web-v1:" + scope.href + ":metadata");
      for (const key of await cache.keys()) if (key.url.includes("/.karty-cache/pin/")) await cache.delete(key);
    });
    const cdp = await session.newCDPSession(first.page);
    await cdp.send("ServiceWorker.enable");
    await cdp.send("ServiceWorker.stopAllWorkers");
    await cdp.detach();
    assert.deepEqual(await fetchBytes(first.page, old.url), [...(await readFile(resolve(fixtures, "a", old.url)))]);
    assert.deepEqual(await fetchBytes(second.page, changed.url), [
      ...(await readFile(resolve(fixtures, "b", changed.url))),
    ]);
    assert.equal((await status(first.page)).release, a.pointer.release);
    assert.equal((await status(second.page)).release, b.pointer.release);
  });
  await check("new worker waits for explicit activation and preserves live A and B pins", async () => {
    await second.page.evaluate(() => {
      globalThis.deliveryOriginalWorker = navigator.serviceWorker.controller;
    });
    server.setWorkerRevision("/game/", 2);
    await second.page.evaluate(async () => {
      await (await navigator.serviceWorker.getRegistration()).update();
    });
    await waitUntil(
      () =>
        second.page.evaluate(
          async () => (await navigator.serviceWorker.getRegistration()).waiting?.state === "installed",
        ),
      "updated worker did not wait for activation",
    );
    assert.equal(
      await second.page.evaluate(() => navigator.serviceWorker.controller === globalThis.deliveryOriginalWorker),
      true,
    );
    await second.page.evaluate(() => globalThis.KartyWebRuntime.applyUpdate());
    assert.equal(
      await second.page.evaluate(() => navigator.serviceWorker.controller === globalThis.deliveryOriginalWorker),
      false,
    );
    assert.equal((await status(first.page)).release, a.pointer.release);
    assert.equal((await status(second.page)).release, b.pointer.release);
    assert.deepEqual(await fetchBytes(first.page, old.url), [...(await readFile(resolve(fixtures, "a", old.url)))]);
    assert.deepEqual(await fetchBytes(second.page, changed.url), [
      ...(await readFile(resolve(fixtures, "b", changed.url))),
    ]);
  });
  await check("fully downloaded B survives worker termination and offline reload", async () => {
    report.beforeRestart = await storedState(second.page);
    await first.page.close();
    await second.page.close();
    const cdp = await session.newCDPSession(independent.page);
    await cdp.send("ServiceWorker.enable");
    await cdp.send("ServiceWorker.stopAllWorkers");
    await cdp.detach();
    server.setOnline(false);
    const offline = await open(session);
    report.afterRestart = {
      ready: offline.state,
      state: await offline.page.evaluate(async () => {
        const scope = new URL("./", location.href);
        const cache = await caches.open("karty-web-v1:" + scope.href + ":metadata");
        return (await cache.match(new URL(".karty-cache/state", scope)))?.json();
      }),
    };
    assert.equal(offline.state.release, b.pointer.release);
    assert.deepEqual(await fetchBytes(offline.page, changed.url), [
      ...(await readFile(resolve(fixtures, "b", changed.url))),
    ]);
    const obsolete = await offline.page.evaluate(async (hash) => {
      const scope = new URL("./", location.href);
      const cache = await caches.open("karty-web-v1:" + scope.href + ":content");
      return !!(await cache.match(new URL(".karty-cache/sha256/" + hash, scope)));
    }, old.sha256);
    assert.equal(obsolete, false, "unreferenced A content remains cached after B is complete");
    await offline.page.close();
    server.setOnline(true);
  });
  await check("another game's cache remains usable across release cleanup", async () => {
    server.setOnline(false);
    await independent.page.reload({ waitUntil: "domcontentloaded", timeout: 9000 });
    assert.equal((await ready(independent.page)).release, a.pointer.release);
    assert.deepEqual(await fetchBytes(independent.page, shared.url), [
      ...(await readFile(resolve(fixtures, "a", shared.url))),
    ]);
    const after = await independent.page.evaluate(() => caches.keys());
    assert.ok(otherCacheNames.some((name) => after.includes(name)));
    server.setOnline(true);
  });
  await check("confirmed B startup corruption blocks launch instead of silently selecting A", async () => {
    const isolated = await context();
    server.setRelease("/game/", "a");
    const previous = await open(isolated);
    await complete(previous.page);
    server.setRelease("/game/", "b");
    server.faults.set("/game/game.kart", { kind: "corrupt" });
    const updating = await isolated.newPage();
    await updating.goto(server.origin + "/game/", { waitUntil: "domcontentloaded", timeout: 9000 });
    await assert.rejects(ready(updating), /integrity|failed|bounds/i);
    assert.equal((await status(previous.page)).release, a.pointer.release);
    await updating.close();
    server.faults.clear();
    server.setOnline(false);
    const offline = await open(isolated);
    assert.equal(offline.state.release, a.pointer.release);
    server.setOnline(true);
    await isolated.close();
    contexts.delete(isolated);
  });
  for (const kind of ["corrupt", "truncated", "oversized"]) {
    await check(`${kind} background bytes are rejected before cache insertion`, async () => {
      server.setRelease("/game/", "b");
      const isolated = await context();
      const opened = await open(isolated);
      server.faults.set("/game/" + changed.url, { kind });
      await opened.page.evaluate(() => {
        globalThis.deliveryEvents = [];
        window.addEventListener("karty-web-status", (event) => globalThis.deliveryEvents.push(event.detail));
      });
      await prefetch(opened.page);
      await opened.page.waitForFunction(() => globalThis.deliveryEvents.some((event) => event.error), null, {
        timeout: 9000,
      });
      const inserted = await opened.page.evaluate(async (hash) => {
        const cache = await caches.open("karty-web-v1:" + new URL("./", location.href).href + ":content");
        return !!(await cache.match(new URL(".karty-cache/sha256/" + hash, location.href)));
      }, changed.sha256);
      assert.equal(inserted, false);
      server.faults.clear();
      await complete(opened.page);
      await isolated.close();
      contexts.delete(isolated);
    });
  }
  for (const phase of ["headers", "body"]) {
    await check(`foreground cancellation during ${phase} aborts the network request`, async () => {
      const isolated = await context();
      const opened = await open(isolated);
      const path = "/game/" + changed.url;
      const start = server.requests.length;
      const fault = { [phase]: true };
      server.faults.set(path, fault);
      // Chromium maps an errored custom response body to TypeError when
      // arrayBuffer consumes it. Compare with the same browser's native stream
      // behavior rather than requiring a name it does not preserve.
      const expectedError =
        phase === "headers"
          ? "AbortError"
          : await opened.page.evaluate(async () => {
              const controller = new AbortController();
              const stream = new ReadableStream({
                start(body) {
                  controller.signal.addEventListener("abort", () => body.error(controller.signal.reason));
                },
              });
              const pending = new Response(stream).arrayBuffer().then(
                () => "unexpected success",
                (error) => error.name,
              );
              controller.abort();
              return pending;
            });
      assert.notEqual(expectedError, "unexpected success");
      await opened.page.evaluate((url) => {
        globalThis.deliveryAbort = new AbortController();
        globalThis.deliverySettled = null;
        globalThis.deliveryPending = fetch(url, { signal: globalThis.deliveryAbort.signal })
          .then((response) => response.arrayBuffer())
          .then(
            () => (globalThis.deliverySettled = { name: "unexpected success" }),
            (error) =>
              (globalThis.deliverySettled = {
                name: error.name,
                message: error.message,
                aborted: globalThis.deliveryAbort.signal.aborted,
              }),
          );
      }, changed.url);
      await waitUntil(
        () => server.requests.slice(start).some((entry) => entry.path === path && entry[phase + "Blocked"]),
        "delayed request never reached the tested phase",
      );
      const result = await opened.page.evaluate(async () => {
        const before = globalThis.deliverySettled;
        globalThis.deliveryAbort.abort();
        return {
          before,
          result: await globalThis.deliveryPending,
        };
      });
      (report.cancelEvidence ||= {})[phase] = result;
      assert.equal(result.before, null, "request failed before cancellation");
      assert.equal(result.result.name, expectedError);
      assert.equal(result.result.aborted, true);
      await waitUntil(
        () => server.requests.slice(start).some((entry) => entry.path === path && entry.aborted),
        "cancellation did not reach server",
      );
      fault.release?.();
      server.faults.clear();
      assert.deepEqual(await fetchBytes(opened.page, changed.url), [
        ...(await readFile(resolve(fixtures, "b", changed.url))),
      ]);
      await isolated.close();
      contexts.delete(isolated);
    });
  }
  await check("foreground streams incrementally and interrupts background download", async () => {
    const isolated = await context();
    const opened = await open(isolated);
    const [background, foreground] = backgrounds(b);
    assert.ok(background && foreground);
    const path = "/game/" + background.url;
    const start = server.requests.length;
    const fault = { body: true };
    server.faults.set(path, fault);
    await prefetch(opened.page);
    await waitUntil(
      () => server.requests.slice(start).some((entry) => entry.path === path && entry.bodyBlocked),
      "background body never started",
    );
    server.faults.delete(path);
    assert.deepEqual(await fetchBytes(opened.page, foreground.url), [
      ...(await readFile(resolve(fixtures, "b", foreground.url))),
    ]);
    await waitUntil(
      () => server.requests.slice(start).some((entry) => entry.path === path && entry.aborted),
      "foreground failed to interrupt background transfer",
    );
    fault.release?.();
    await complete(opened.page);
    await isolated.close();
    contexts.delete(isolated);
  });
  await check("foreground response exposes a chunk before the whole media file arrives", async () => {
    const isolated = await context();
    const opened = await open(isolated);
    const fault = { body: true };
    server.faults.set("/game/" + changed.url, fault);
    const prefix = await opened.page.evaluate(async (url) => {
      const response = await fetch(url);
      globalThis.deliveryReader = response.body.getReader();
      const first = await globalThis.deliveryReader.read();
      return { size: first.value?.byteLength, done: first.done };
    }, changed.url);
    assert.ok(prefix.size > 0 && prefix.size < changed.size);
    assert.equal(prefix.done, false);
    fault.release?.();
    server.faults.clear();
    await opened.page.evaluate(async () => {
      while (!(await globalThis.deliveryReader.read()).done) {}
    });
    await isolated.close();
    contexts.delete(isolated);
  });
  await check("save-data pauses automatic prefetch and explicit consent can resume", async () => {
    const isolated = await context();
    await isolated.addInitScript(() => {
      Object.defineProperty(navigator, "connection", {
        configurable: true,
        value: { saveData: true, effectiveType: "4g" },
      });
    });
    const opened = await open(isolated);
    const start = server.requests.length;
    const paused = await prefetch(opened.page);
    assert.equal(paused.status.policyPaused, true);
    assert.equal(
      server.requests.slice(start).some((entry) => backgrounds(b).some((file) => entry.path === "/game/" + file.url)),
      false,
    );
    await opened.page.evaluate(() => globalThis.KartyWebRuntime.startPrefetch({ force: true }));
    await waitUntil(async () => (await status(opened.page)).ready === true, "explicit download did not finish");
    await isolated.close();
    contexts.delete(isolated);
  });
  await check("real browser storage quota prevents incomplete offline readiness", async () => {
    const isolated = await context();
    const opened = await open(isolated);
    await opened.page.evaluate(() => {
      globalThis.deliveryEvents = [];
      window.addEventListener("karty-web-status", (event) => globalThis.deliveryEvents.push(event.detail));
    });
    const cdp = await isolated.newCDPSession(opened.page);
    await cdp.send("Storage.overrideQuotaForOrigin", { origin: server.origin, quotaSize: 1 });
    await prefetch(opened.page);
    await opened.page.waitForFunction(() => globalThis.deliveryEvents.some((event) => event.error), null, {
      timeout: 9000,
    });
    const state = await status(opened.page);
    assert.equal(state.ready, false);
    const errors = await opened.page.evaluate(() =>
      globalThis.deliveryEvents.filter((event) => event.error).map((event) => event.error),
    );
    assert.match(errors.join(" "), /storage|quota/i);
    await cdp.send("Storage.overrideQuotaForOrigin", { origin: server.origin });
    await cdp.detach();
    await isolated.close();
    contexts.delete(isolated);
  });
  await check("storage quota before first launch preserves verified online play", async () => {
    const isolated = await context();
    const blank = await isolated.newPage();
    const cdp = await isolated.newCDPSession(blank);
    await cdp.send("Storage.overrideQuotaForOrigin", { origin: server.origin, quotaSize: 1 });
    const opened = await open(isolated);
    assert.equal(opened.state.launch, true);
    assert.equal(opened.state.release, b.pointer.release);
    assert.equal(opened.state.onlineOnly, true);
    assert.equal((await status(opened.page)).ready, false);
    assert.deepEqual(await fetchBytes(opened.page, "game.kart"), [
      ...(await readFile(resolve(fixtures, "b", "game.kart"))),
    ]);
    await cdp.send("Storage.overrideQuotaForOrigin", { origin: server.origin });
    await complete(opened.page);
    await cdp.send("ServiceWorker.enable");
    await cdp.send("ServiceWorker.stopAllWorkers");
    server.setOnline(false);
    await opened.page.reload({ waitUntil: "domcontentloaded", timeout: 9000 });
    assert.equal((await ready(opened.page)).release, b.pointer.release);
    server.setOnline(true);
    await cdp.detach();
    await isolated.close();
    contexts.delete(isolated);
  });
  await check("quota during A to B update retains the confirmed B release after retry", async () => {
    const isolated = await context();
    server.setRelease("/game/", "a");
    const previous = await open(isolated);
    await complete(previous.page);
    const cdp = await isolated.newCDPSession(previous.page);
    await cdp.send("Storage.overrideQuotaForOrigin", { origin: server.origin, quotaSize: 1 });
    server.setRelease("/game/", "b");
    const updating = await open(isolated);
    assert.equal(updating.state.release, b.pointer.release);
    assert.equal(updating.state.onlineOnly, true);
    assert.equal((await status(previous.page)).release, a.pointer.release);
    await cdp.send("Storage.overrideQuotaForOrigin", { origin: server.origin });
    await complete(updating.page);
    assert.equal((await storedState(updating.page)).completeRelease, b.pointer.release);
    await previous.page.close();
    const restarted = await isolated.newCDPSession(updating.page);
    await restarted.send("ServiceWorker.enable");
    await restarted.send("ServiceWorker.stopAllWorkers");
    server.setOnline(false);
    await updating.page.reload({ waitUntil: "domcontentloaded", timeout: 9000 });
    assert.equal((await ready(updating.page)).release, b.pointer.release);
    assert.deepEqual(await fetchBytes(updating.page, changed.url), [
      ...(await readFile(resolve(fixtures, "b", changed.url))),
    ]);
    server.setOnline(true);
    await restarted.detach();
    await isolated.close();
    contexts.delete(isolated);
  });
  await check("pausing background prefetch aborts a request waiting for headers", async () => {
    const isolated = await context();
    const opened = await open(isolated);
    const file = backgrounds(b)[0];
    const path = "/game/" + file.url;
    const start = server.requests.length;
    const fault = { headers: true };
    server.faults.set(path, fault);
    await prefetch(opened.page);
    await waitUntil(
      () => server.requests.slice(start).some((entry) => entry.path === path && entry.headersBlocked),
      "background request never started",
    );
    await opened.page.evaluate(() => globalThis.KartyWebRuntime.pausePrefetch());
    await waitUntil(
      () => server.requests.slice(start).some((entry) => entry.path === path && entry.aborted),
      "pausing failed to cancel the background request",
    );
    fault.release?.();
    server.faults.clear();
    await complete(opened.page);
    await isolated.close();
    contexts.delete(isolated);
  });
  await check("cross-origin CDN host with a version query and unknown decoded size works offline", async () => {
    const cdn = await startDeliveryServer(fixtures);
    const mirror = await startDeliveryServer(fixtures);
    extraServers.push(cdn, mirror);
    report.cdn = { requests: cdn.requests, mirrorRequests: mirror.requests };
    cdn.setRelease("/runtime/", "b");
    mirror.setRelease("/mirror/", "b");
    cdn.faults.set("/runtime/karty-host.wasm", {
      redirect: mirror.origin + "/mirror/karty-host.wasm?v=shared-runtime",
    });
    const manifest = structuredClone(b.manifest);
    const host = manifest.files.find((file) => file.url === "karty-host.wasm");
    assert.ok(host);
    host.url = cdn.origin + "/runtime/karty-host.wasm?v=shared-runtime";
    host.size = 0;
    const bytes = Buffer.from(JSON.stringify(manifest));
    const release = createHash("sha256").update(bytes).digest("hex");
    const manifestPath = "karty-releases/" + release + ".json";
    await cp(resolve(fixtures, "b"), resolve(fixtures, "cdn"), { recursive: true });
    await writeFile(resolve(fixtures, "cdn", manifestPath), bytes);
    await writeFile(
      resolve(fixtures, "cdn", "karty-latest.json"),
      JSON.stringify({ version: 1, release, manifest: manifestPath }),
    );
    server.setRelease("/cdn-game/", "cdn");
    const isolated = await context();
    const opened = await open(isolated, "/cdn-game/");
    assert.equal(opened.state.release, release);
    await complete(opened.page);
    const state = await status(opened.page);
    assert.equal(state.totalBytesKnown, true);
    assert.ok(
      cdn.requests.some((entry) => entry.path === "/runtime/karty-host.wasm" && entry.query === "?v=shared-runtime"),
    );
    assert.ok(cdn.requests.some((entry) => entry.path === "/runtime/karty-host.wasm" && entry.status === 302));
    assert.ok(
      mirror.requests.some((entry) => entry.path === "/mirror/karty-host.wasm" && entry.query === "?v=shared-runtime"),
    );
    await opened.page.evaluate(() => globalThis.KartyWebRuntime.pausePrefetch());
    const stopped = await isolated.newCDPSession(opened.page);
    await stopped.send("ServiceWorker.enable");
    await stopped.send("ServiceWorker.stopAllWorkers");
    await stopped.detach();
    await opened.page.evaluate(async (hash) => {
      const scope = new URL("./", location.href);
      const cache = await caches.open("karty-web-v1:" + scope.href + ":content");
      await cache.delete(new URL(".karty-cache/sha256/" + hash, scope));
    }, host.sha256);
    assert.equal((await status(opened.page)).ready, false);
    const foregroundStart = cdn.requests.length;
    const mirrorStart = mirror.requests.length;
    assert.deepEqual(await fetchBytes(opened.page, host.url, { cache: "no-store" }), [
      ...(await readFile(resolve(fixtures, "b", "karty-host.wasm"))),
    ]);
    assert.ok(
      cdn.requests
        .slice(foregroundStart)
        .some((entry) => entry.path === "/runtime/karty-host.wasm" && entry.status === 302),
    );
    assert.ok(
      mirror.requests
        .slice(mirrorStart)
        .some((entry) => entry.path === "/mirror/karty-host.wasm" && entry.query === "?v=shared-runtime"),
    );
    await complete(opened.page);
    const cdp = await isolated.newCDPSession(opened.page);
    await cdp.send("ServiceWorker.enable");
    await cdp.send("ServiceWorker.stopAllWorkers");
    await cdp.detach();
    await opened.page.close();
    server.setOnline(false);
    cdn.setOnline(false);
    mirror.setOnline(false);
    const offline = await open(isolated, "/cdn-game/");
    assert.equal(offline.state.release, release);
    assert.deepEqual(await fetchBytes(offline.page, host.url), [
      ...(await readFile(resolve(fixtures, "b", "karty-host.wasm"))),
    ]);
    server.setOnline(true);
    cdn.setOnline(true);
    mirror.setOnline(true);
    report.cdn = { requests: cdn.requests, mirrorRequests: mirror.requests };
    await isolated.close();
    contexts.delete(isolated);
  });
  report.requests = server.requests;
  report.peakNetworkConcurrency = server.peak();
  report.passed = report.failures.length === 0;
} finally {
  report.requests = server.requests;
  clearTimeout(deadline);
  for (const value of contexts) await value.close();
  await browser.close();
  await server.close();
  for (const extra of extraServers) await extra.close();
  await rm(fixtures, { recursive: true, force: true });
  await mkdir(resolve(output, ".."), { recursive: true });
  await writeFile(output, JSON.stringify(report, null, 2) + "\n");
}
assert.equal(report.failures.length, 0, "web delivery checks failed; see results.json");
console.log(`Web delivery: ${report.checks.length} checks passed; ${output}`);
