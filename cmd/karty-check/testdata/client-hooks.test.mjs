// One room/actor, no runtime baker: execute generated actions in the production
// browser host and observe actual guest commands without replacing host work.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, resolve, sep } from "node:path";
import { chromium } from "playwright";

const directory = resolve(process.argv[2]);
const mime = new Map([
  [".html", "text/html"],
  [".js", "text/javascript"],
  [".wasm", "application/wasm"],
]);
const requests = [];
const server = createServer(async (request, response) => {
  try {
    const pathname = new URL(request.url, "http://localhost").pathname;
    const path = resolve(directory, pathname === "/" ? "index.html" : pathname.slice(1));
    if (!path.startsWith(directory + sep)) throw new Error("path outside fixture");
    const body = await readFile(path);
    requests.push(pathname);
    response.writeHead(200, { "Content-Type": mime.get(extname(path)) || "application/octet-stream" });
    response.end(body);
  } catch {
    response.writeHead(404);
    response.end();
  }
});
await new Promise((accept, reject) => {
  server.once("error", reject);
  server.listen(0, "127.0.0.1", accept);
});
let browser, browserServer, deadline;
try {
  browserServer = await chromium.launchServer({
    headless: true,
    timeout: 9000,
    args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  browser = await chromium.connect(browserServer.wsEndpoint(), { timeout: 9000 });
  // Browser.close can wait behind a blocked GPU compiler. Keep ownership of
  // this browser's process group so a stalled fixture cannot leak Chromium.
  deadline = setTimeout(() => {
    console.error("Crafted browser fixture exceeded its 9-second execution budget");
    void browserServer.kill();
  }, 9000);
  const page = await browser.newPage({ viewport: { width: 320, height: 240 } });
  page.setDefaultTimeout(9000);
  page.setDefaultNavigationTimeout(9000);
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
    else if (process.env.KARTY_TRACE_SHADERS) console.log(message.text());
  });
  if (process.env.KARTY_TRACE_SHADERS)
    await page.addInitScript(() => {
      const sources = new Map(),
        programs = new Map();
      const proto = WebGL2RenderingContext.prototype;
      const source = proto.shaderSource,
        attach = proto.attachShader,
        link = proto.linkProgram,
        status = proto.getProgramParameter;
      proto.shaderSource = function (shader, text) {
        sources.set(shader, text);
        return source.call(this, shader, text);
      };
      proto.attachShader = function (program, shader) {
        programs.set(program, [...(programs.get(program) || []), sources.get(shader)]);
        return attach.call(this, program, shader);
      };
      proto.linkProgram = function (program) {
        console.log("link shader " + (programs.get(program) || []).map((text) => text?.slice(0, 160)).join(" | "));
        return link.call(this, program);
      };
      proto.getProgramParameter = function (program, name) {
        const result = status.call(this, program, name);
        if (name === this.LINK_STATUS) console.log("linked shader " + result);
        return result;
      };
    });
  await page.addInitScript(() => {
    globalThis.fixtureCommands = [];
    let submit;
    Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
      configurable: true,
      get: () => submit,
      set: (value) => {
        submit = (batch) => {
          const view = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
          for (let index = 0, offset = 24; index < view.getUint32(16, true); index++) {
            const tag = view.getUint8(offset),
              length = view.getUint16(offset + 1, true);
            const payload = offset + 3;
            if (tag === 32) {
              assertSingleTag(view.getUint8(payload + 4));
              const count = view.getUint8(payload + 8);
              const text = new TextDecoder().decode(batch.subarray(payload + 9, payload + 9 + count));
              globalThis.fixtureCommands.push({ tag: text, id: view.getUint32(payload, true) });
            }
            offset += 3 + length;
          }
          return value(batch);
        };
      },
    });
    function assertSingleTag(count) {
      if (count !== 1) throw new Error("fixture expected one actor tag");
    }
  });
  await page.goto(`http://127.0.0.1:${server.address().port}/?aa=off`);
  await page.waitForFunction(() => globalThis.fixtureCommands.some((command) => command.tag === "second-mount"), null, {
    timeout: 9000,
  });
  const mounts = await page.evaluate(() =>
    globalThis.fixtureCommands.filter((command) => command.tag.endsWith("-mount")),
  );
  assert.deepEqual(
    mounts.map((command) => command.tag),
    ["first-mount", "second-mount"],
  );
  assert.notEqual(mounts[0].id, mounts[1].id, "released actor reference reused");
  await page.keyboard.down("a");
  await page.waitForFunction(() => globalThis.fixtureCommands.some((command) => command.tag === "key-hook"));
  await page.waitForFunction(() => globalThis.fixtureCommands.some((command) => command.tag === "transform-hook"));
  await page.keyboard.up("a");
  await page.waitForFunction(() => globalThis.fixtureCommands.some((command) => command.tag === "key-up-hook"));
  const canvas = page.locator("canvas");
  await canvas.waitFor({ state: "visible" });
  await canvas.click({ position: { x: 20, y: 20 } });
  await page.waitForFunction(() => globalThis.fixtureCommands.some((command) => command.tag === "pointer-hook"));
  const input = await page.evaluate(() =>
    globalThis.fixtureCommands.filter((command) => command.tag.endsWith("-hook")),
  );
  assert.ok(
    input.every((command) => command.id === mounts[1].id),
    "hook targeted an actor from the released mount",
  );
  const before = await page.evaluate(() => Number(document.documentElement.dataset.kartyFrame));
  await page.waitForFunction((frame) => Number(document.documentElement.dataset.kartyFrame) > frame + 2, before);
  assert.equal(await page.getAttribute("html", "data-karty-state"), "running");
  assert.deepEqual(errors, []);
  assert.equal(new Set(requests.filter((path) => path.endsWith(".kld"))).size, 1, "fixture fetched unrelated levels");
  console.log(
    "Crafted browser hooks fixture passed: typed input, accepted transform callback, authored condition/wait/action and fresh remount references",
  );
} finally {
  clearTimeout(deadline);
  await browserServer?.kill();
  await new Promise((accept) => server.close(accept));
}
