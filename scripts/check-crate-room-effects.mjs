// Observe real WASM commands and unmodified Oto output. Inputs are synthesized.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { startVillageServer } from "./willowbrook-server.mjs";

const directory = resolve(process.argv[2] || "samples/crate-room/dist/web");
const output = resolve("dist/validation/crate-room-effects");
await mkdir(output, { recursive: true });
const declarations = await readFile("samples/crate-room/.karty/assets/sounds.go", "utf8");
const sounds = Object.fromEntries(
  [...declarations.matchAll(/Sound(\w+)\s+engine\.SoundID\s*=\s*(\d+)/g)].map((m) => [m[1], Number(m[2])]),
);
for (const name of ["Shoot", "Blast", "CrateHit"]) assert.ok(sounds[name], `missing sound ${name}`);
const digest = async (name) =>
  createHash("sha256")
    .update(await readFile(resolve(directory, name)))
    .digest("hex");
const report = {
  passed: false,
  scope:
    "Actual WASM, WebGL2 and stereo WebAudio in Chromium SwiftShader; synthetic input. No device listening or FPS qualification.",
  hostSha256: await digest("karty-host.wasm"),
  cartridgeSha256: await digest("game.kart"),
  sounds,
  errors: [],
};
const server = await startVillageServer(directory);
const browser = await chromium.launch({
  headless: true,
  args: ["--no-sandbox", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
});
const deadline = setTimeout(() => void browser.close(), 120000);
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
  page.on("pageerror", (error) => report.errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") report.errors.push(message.text());
  });
  await page.addInitScript(() => {
    const e = (globalThis.crateEffects = {
      frame: 0,
      starts: [],
      shots: 0,
      stops: [],
      particles: [],
      particleTextures: [],
      particleState: {},
      peakParticles: 0,
      outputs: [],
      receivers: [],
      live: {},
    });
    const OriginalContext = globalThis.AudioContext;
    globalThis.AudioContext = new Proxy(OriginalContext, {
      construct(target, args) {
        const context = Reflect.construct(target, args);
        const addModule = context.audioWorklet.addModule.bind(context.audioWorklet);
        context.audioWorklet.addModule = async (url, ...options) => {
          const source = await (await fetch(url)).text();
          if (!source.includes("class OtoWorkletProcessor")) throw new Error("unexpected Oto worklet");
          const audit = `
            const originalProcess=OtoWorkletProcessor.prototype.process;
            OtoWorkletProcessor.prototype.process=function(inputs,outputs,parameters){
              const result=originalProcess.call(this,inputs,outputs,parameters);
              this.crateAudit_ ||= {blocks:0,left:0,right:0,peak:0,frames:0,channels:outputs[0].length};
              for(let c=0;c<Math.min(2,outputs[0].length);c++)for(const v of outputs[0][c]){
                this.crateAudit_[c?'right':'left']+=v*v;
                this.crateAudit_.peak=Math.max(this.crateAudit_.peak,Math.abs(v));
              }
              this.crateAudit_.frames+=outputs[0][0].length;
              if(++this.crateAudit_.blocks===16){this.port.postMessage({__crateAudio:true,...this.crateAudit_});this.crateAudit_=null;}
              return result;
            };`;
          const injected = URL.createObjectURL(new Blob([source, audit], { type: "application/javascript" }));
          try {
            return await addModule(injected, ...options);
          } finally {
            URL.revokeObjectURL(injected);
          }
        };
        return context;
      },
    });
    const OriginalNode = globalThis.AudioWorkletNode;
    globalThis.AudioWorkletNode = new Proxy(OriginalNode, {
      construct(target, args) {
        const node = Reflect.construct(target, args);
        node.port.addEventListener("message", (event) => {
          if (event.data?.__crateAudio) {
            e.outputs.push({ ...event.data, frame: e.frame });
            event.stopImmediatePropagation();
          }
        });
        node.port.start();
        return node;
      },
    });
    const hook = (name, visit) => {
      let wrapped;
      Object.defineProperty(globalThis, name, {
        configurable: true,
        get: () => wrapped,
        set(original) {
          wrapped = (...args) => {
            visit(...args);
            return original(...args);
          };
        },
      });
    };
    hook("kartyClientUpdate", (frame) => {
      e.frame = Number(frame);
    });
    hook("kartyHostPlaySound", () => {
      e.shots++;
    });
    hook("kartyHostSetAudioReceiver", (id, x, y) => {
      e.receivers.push({ id, x, y });
    });
    hook("kartyHostSetSoundEmitter", (id, sound, level, source, x, y, z, volume, near, far, flags) => {
      if (!e.live[id]) e.starts.push({ id, sound, x, y, z, volume, near, far, flags, frame: e.frame });
      e.live[id] = true;
    });
    hook("kartyHostStopSoundEmitter", (id) => {
      delete e.live[id];
      e.stops.push(id);
    });
    hook("kartyHostSubmitCommands", (batch) => {
      const view = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
      for (let i = 0, offset = 24; i < view.getUint32(16, true); i++) {
        const tag = view.getUint8(offset),
          p = offset + 3,
          id = view.getUint32(p, true);
        if (tag === 29) {
          e.particles.push(id);
          e.particleTextures.push(view.getUint32(p + 44, true));
          e.particleState[id] = {
            x: view.getFloat32(p + 8, true),
            y: view.getFloat32(p + 12, true),
            z: view.getFloat32(p + 16, true),
            size: 0.05,
            texture: view.getUint32(p + 44, true),
          };
        }
        if (tag === 30 && e.particleState[id])
          Object.assign(e.particleState[id], {
            x: view.getFloat32(p + 4, true),
            y: view.getFloat32(p + 8, true),
            z: view.getFloat32(p + 12, true),
          });
        if (tag === 31 && e.particleState[id])
          Object.assign(e.particleState[id], {
            texture: view.getUint32(p + 4, true),
            size: view.getFloat32(p + 12, true),
          });
        if (tag === 34 || tag === 2) {
          e.particles = e.particles.filter((value) => value !== id);
          delete e.particleState[id];
        }
        e.peakParticles = Math.max(e.peakParticles, e.particles.length);
        offset += 3 + view.getUint16(offset + 1, true);
      }
    });
  });
  await page.goto(`http://127.0.0.1:${server.address().port}/?worldScale=0.5&aa=false&debug=0`);
  await page.waitForFunction(() => globalThis.crateEffects?.receivers.length, null, { timeout: 45000 });
  const frames = async (count) => {
    const first = await page.evaluate(() => crateEffects.frame);
    await page.waitForFunction(
      ({ first, count }) => crateEffects.frame >= first + count,
      { first, count },
      { timeout: 20000 },
    );
  };
  await page.locator("canvas").click({ position: { x: 640, y: 360 } });
  await page.keyboard.press("Escape");
  await page.keyboard.press("r");
  await frames(50);
  await page.keyboard.press("b");
  await page.waitForFunction(() => crateEffects.particles.length === 6, null, { timeout: 10000 });
  await frames(8);
  const earlySmoke = await page.evaluate(() => structuredClone(crateEffects.particleState));
  const mainID = Object.keys(earlySmoke).sort((a, b) => earlySmoke[b].size - earlySmoke[a].size)[0];
  await frames(60);
  const laterSmoke = await page.evaluate((id) => structuredClone(crateEffects.particleState[id]), mainID);
  assert.ok(laterSmoke, "main cloud remains alive after a second");
  assert.ok(laterSmoke.size > earlySmoke[mainID].size, "smoke keeps expanding instead of shrinking away");
  assert.ok(
    Math.hypot(laterSmoke.x - earlySmoke[mainID].x, laterSmoke.y - earlySmoke[mainID].y) > 0.15,
    "smoke keeps drifting outward while fading",
  );
  assert.ok(laterSmoke.texture >= 2 && laterSmoke.texture <= 16, "smoke advances into reduced-opacity atlas regions");
  report.smokeAnimation = { early: earlySmoke[mainID], later: laterSmoke };
  await page.screenshot({ path: resolve(output, "blast.png") });
  // Use a second real blast to capture its larger, fainter late cloud.
  await page.waitForFunction(() => crateEffects.particles.length === 0, null, { timeout: 20000 });
  await page.keyboard.press("r");
  await frames(2);
  await page.keyboard.press("b");
  await page.waitForFunction(() => crateEffects.particles.length === 6, null, { timeout: 10000 });
  await frames(96);
  await page.screenshot({ path: resolve(output, "smoke.png") });
  await page.waitForFunction(() => crateEffects.particles.length === 0, null, { timeout: 20000 });
  await page.keyboard.down("Space");
  await frames(24);
  await page.keyboard.up("Space");
  assert.ok(await page.evaluate(() => crateEffects.shots >= 3), "held fire produces shot sounds");
  await frames(30);
  const first = await page.evaluate(() => structuredClone(crateEffects));
  assert.equal(first.particles.length, 0, "one-shot particles expire");
  assert.ok(
    first.particleTextures.length > 0 && first.particleTextures.every((id) => id === 1),
    "blasts emit only the cartoon smoke texture",
  );
  assert.ok(
    first.starts.some((s) => s.sound === sounds.Blast),
    "blast emits spatial sound at its hit",
  );
  const impacts = first.starts.filter((s) => s.sound === sounds.CrateHit);
  assert.ok(impacts.length, "falling crates produce spatial thuds");
  for (let i = 1; i < impacts.length; i++)
    assert.ok(impacts[i].frame - impacts[i - 1].frame >= 8, "room impact rate limit");
  assert.ok(
    impacts.every((s) => s.volume <= 0.5 && Number.isFinite(s.z)),
    "impact gain is capped and sources include height",
  );
  const pcm = first.outputs.filter((s) => s.peak > 0.00001);
  assert.ok(pcm.length && pcm.every((s) => s.channels === 2), "effects reach actual stereo Oto output");
  assert.ok(
    pcm.some((s) => Math.abs(s.left - s.right) / (s.left + s.right) > 0.05),
    "spatial effects produce unequal left/right energy",
  );
  assert.ok(Math.max(...pcm.map((s) => s.peak)) < 0.98, "mix retains PCM headroom");
  report.firstPass = {
    shots: first.shots,
    impacts,
    blast: first.starts.filter((s) => s.sound === sounds.Blast),
    audibleWindows: pcm.length,
    peak: Math.max(...pcm.map((s) => s.peak)),
  };
  // A rapid burst replaces visual slots and admits at most four blast voices.
  for (let i = 0; i < 8; i++) {
    await page.keyboard.press("b");
    await frames(2);
  }
  const beforeReset = await page.evaluate(() => crateEffects.peakParticles);
  assert.ok(beforeReset <= 24, "repeated blasts retain the 24-particle bound");
  await page.keyboard.press("r");
  await frames(3);
  assert.equal(await page.evaluate(() => crateEffects.particles.length), 0, "reset destroys every particle");
  await frames(100);
  const quietFrom = await page.evaluate(() => crateEffects.starts.length);
  await frames(60);
  assert.equal(await page.evaluate(() => crateEffects.starts.length), quietFrom, "reset stacks settle to silence");
  report.peakParticles = beforeReset;
  const facing = await page.evaluate(() => crateEffects.receivers.at(-1));
  await page.keyboard.down("ArrowRight");
  await frames(20);
  await page.keyboard.up("ArrowRight");
  const turned = await page.evaluate(() => crateEffects.receivers.at(-1));
  assert.ok(Math.hypot(turned.x - facing.x, turned.y - facing.y) > 0.4, "listener basis follows player turning");
  report.listener = { before: facing, after: turned };
  assert.deepEqual(report.errors, []);
  report.passed = true;
} finally {
  clearTimeout(deadline);
  await browser.close();
  await new Promise((done) => server.close(done));
  await writeFile(resolve(output, "results.json"), JSON.stringify(report, null, 2) + "\n");
}
console.log(`Crate room effects passed: ${output}`);
