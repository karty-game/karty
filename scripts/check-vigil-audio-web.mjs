// Real cartridge/WASM/WebAudio spatial mix. The observer measures original Oto
// output samples; it never changes game commands, audio samples or mixer gains.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { mkdir, readFile, realpath, writeFile } from "node:fs/promises";
import { extname, resolve } from "node:path";
import { chromium } from "playwright";

const projectDirectory = await realpath(resolve(process.env.KARTY_VIGIL_PROJECT || "samples/vigil"));
const directory = await realpath(resolve(process.argv[2] || resolve(projectDirectory, "dist/web")));
const output = resolve("dist/validation/vigil/audio");
const animation = JSON.parse(await readFile(resolve(projectDirectory, "levels/crypt/walk.generated.json"), "utf8"));
const declarations = await readFile(resolve(projectDirectory, ".karty/assets/sounds.go"), "utf8");
const sounds = Object.fromEntries(
  [...declarations.matchAll(/Sound(\w+)\s+engine\.SoundID\s*=\s*(\d+)/g)].map((match) => [match[1], Number(match[2])]),
);
for (const name of ["EmberCrackle", "FireballCast", "FireballImpact", "ShrineChime", "StoneFootstep"])
  assert.ok(sounds[name], `build Vigil audio assets first: ${name}`);
await mkdir(output, { recursive: true });
const digest = async (name) =>
  createHash("sha256")
    .update(await readFile(resolve(directory, name)))
    .digest("hex");
const report = {
  passed: false,
  deadlineMs: 120000,
  cartridgeSha256: await digest("game.kart"),
  hostSha256: await digest("karty-host.wasm"),
  backend:
    "Chromium SwiftShader, worldScale=0.25, aa=false; actual stereo Oto worklet output, synthesized keyboard/mouse; no physical-device listening/FPS qualification",
  sounds,
  measurements: [],
  checks: [],
  errors: [],
};
const server = createServer(async (request, response) => {
  try {
    const path = new URL(request.url, "http://localhost").pathname;
    const file = await realpath(resolve(directory, "." + (path === "/" ? "/index.html" : path)));
    if (!file.startsWith(directory + "/")) throw new Error("outside distribution");
    response.writeHead(200, {
      "Content-Type":
        {
          ".html": "text/html",
          ".js": "text/javascript",
          ".wasm": "application/wasm",
          ".kart": "application/wasm",
          ".kld": "application/wasm",
        }[extname(file)] || "application/octet-stream",
      "Cache-Control": "no-store",
    });
    response.end(await readFile(file));
  } catch {
    response.writeHead(404).end();
  }
});
await new Promise((done) => server.listen(0, "127.0.0.1", done));
const browser = await chromium.launch({
  headless: true,
  args: ["--no-sandbox", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
});
const deadline = setTimeout(() => void browser.close(), 120000);
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
  page.on("pageerror", (error) => report.errors.push(String(error)));
  await page.addInitScript(
    ({ rootTexture }) => {
      const evidence = (globalThis.vigilAudio = {
        contexts: [],
        outputs: [],
        emitters: {},
        starts: [],
        stops: [],
        receivers: [],
        music: [],
        positions: {},
        camera: 0,
        root: 0,
        updates: 0,
      });
      const OriginalContext = globalThis.AudioContext;
      globalThis.AudioContext = new Proxy(OriginalContext, {
        construct(target, args) {
          const context = Reflect.construct(target, args);
          evidence.contexts.push(context);
          const addModule = context.audioWorklet.addModule.bind(context.audioWorklet);
          context.audioWorklet.addModule = async (url, ...options) => {
            const source = await (await fetch(url)).text();
            if (!source.includes("class OtoWorkletProcessor")) throw new Error("unexpected Oto audio worklet");
            const audit = `
            const originalProcess=OtoWorkletProcessor.prototype.process;
            OtoWorkletProcessor.prototype.process=function(inputs,outputs,parameters){
              const result=originalProcess.call(this,inputs,outputs,parameters);
              this.audit_ ||= {blocks:0,frames:0,channels:outputs[0].length,left:0,right:0,hpLeft:0,hpRight:0,peak:0};
              this.hpAudit_ ||= Array.from({length:2},()=>Array.from({length:4},()=>[0,0]));
              const alpha=Math.exp(-2*Math.PI*1800/sampleRate);
              for(let c=0;c<outputs[0].length;c++)for(const value of outputs[0][c]){
                if(c>1)continue;
                let filtered=value;
                for(const stage of this.hpAudit_[c]){const next=alpha*(stage[1]+filtered-stage[0]);stage[0]=filtered;stage[1]=next;filtered=next;}
                this.audit_[c===0?'left':'right']+=value*value;
                this.audit_[c===0?'hpLeft':'hpRight']+=filtered*filtered;
                this.audit_.peak=Math.max(this.audit_.peak,Math.abs(value));
              }
              this.audit_.frames+=outputs[0][0].length;
              if(++this.audit_.blocks===16){this.port.postMessage({__vigilAudioAudit:true,...this.audit_,contextTime:(currentFrame+outputs[0][0].length)/sampleRate});this.audit_=null;}
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
            if (event.data?.__vigilAudioAudit) {
              evidence.outputs.push(event.data);
              event.stopImmediatePropagation();
            }
          });
          node.port.start();
          return node;
        },
      });
      const now = () => evidence.contexts[0]?.currentTime ?? 0;
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
      hook("kartyHostSetSoundEmitter", (id, sound, level, source, x, y, z, volume, near, far, flags) => {
        const record = {
          id,
          sound,
          level,
          source,
          position: [x, y, z],
          volume,
          near,
          far,
          flags,
          time: now(),
          receiver: evidence.receivers.at(-1),
          rootPosition: evidence.positions[evidence.root]?.slice(),
          cameraPosition: evidence.positions[evidence.camera]?.slice(),
        };
        if (!evidence.emitters[id]) evidence.starts.push(record);
        evidence.emitters[id] = record;
      });
      hook("kartyHostStopSoundEmitter", (id) => {
        evidence.stops.push({ id, time: now() });
        delete evidence.emitters[id];
      });
      hook("kartyHostSetAudioReceiver", (id, x, y) => evidence.receivers.push({ id, right: [x, y], time: now() }));
      hook("kartyHostPlayMusic", (id, loop, crossfade) => evidence.music.push({ id, loop, crossfade, time: now() }));
      const records = (batch, visit) => {
        const b = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
        for (let i = 0, o = 24; i < b.getUint32(16, true); i++) {
          const tag = b.getUint8(o),
            length = b.getUint16(o + 1, true);
          visit(tag, b, o + 3);
          o += 3 + length;
        }
      };
      hook("kartyHostSubmitCommands", (batch) =>
        records(batch, (tag, b, p) => {
          const id = b.getUint32(p, true);
          if (tag === 27) {
            evidence.camera = id;
            evidence.positions[id] = [12, 16, 20].map((offset) => b.getFloat32(p + offset, true));
          }
          if (tag === 28) evidence.positions[id] = [8, 12, 16].map((offset) => b.getFloat32(p + offset, true));
          if (tag === 27 || tag === 28) {
            const settings = p + (tag === 27 ? 8 : 4);
            evidence.height = b.getFloat32(settings + 28, true);
          }
          if (tag === 29) {
            evidence.positions[id] = [8, 12, 16].map((offset) => b.getFloat32(p + offset, true));
            if (b.getUint32(p + 44, true) === rootTexture) evidence.root = id;
          }
          if (tag === 30) evidence.positions[id] = [4, 8, 12].map((offset) => b.getFloat32(p + offset, true));
        }),
      );
      // Accepted transforms are independent evidence of the actual audio provider.
      hook("kartyClientUpdate", (_frame, batch) => {
        evidence.updates++;
        records(batch, (tag, b, p) => {
          if (tag === 11 || tag === 19) {
            const id = b.getUint32(p + (tag === 19 ? 4 : 0), true);
            const pose = p + (tag === 19 ? 20 : 4);
            evidence.positions[id] = [0, 4, 8].map((offset) => b.getFloat32(pose + offset, true));
          }
        });
      });
    },
    { rootTexture: animation.rootTexture },
  );
  await page.goto(`http://127.0.0.1:${server.address().port}/?worldScale=0.25&aa=false&debug=0`);
  await page.waitForFunction(
    () =>
      globalThis.vigilAudio?.root &&
      vigilAudio.receivers.length &&
      globalThis.kartyWorldRenderer?.stats().cameras.some((camera) => camera.device?.frames >= 5),
    null,
    { timeout: 45000 },
  );
  await page.locator("canvas").click({ position: { x: 640, y: 420 } });
  await page.keyboard.press("Escape");
  await page.waitForFunction(
    () =>
      vigilAudio.contexts.length &&
      vigilAudio.contexts.every((context) => context.state === "running") &&
      vigilAudio.outputs.length >= 8,
    null,
    { timeout: 15000 },
  );
  const frames = async (count) => {
    const first = await page.evaluate(() => vigilAudio.updates);
    await page.waitForFunction(
      ({ first, count }) => vigilAudio.updates >= first + count,
      { first, count },
      { timeout: 20000 },
    );
  };
  const press = async (key, count = 4) => {
    await page.keyboard.down(key);
    await frames(count);
    await page.keyboard.up(key);
    await frames(4);
  };
  const audioWait = async (seconds) => {
    const first = await page.evaluate(() => vigilAudio.contexts[0].currentTime);
    await page.waitForFunction(
      ({ first, seconds }) => vigilAudio.contexts[0].currentTime >= first + seconds,
      { first, seconds },
      { timeout: 15000 },
    );
  };
  const measure = async (label) => {
    const start = await page.evaluate(() => vigilAudio.contexts[0].currentTime);
    await audioWait(0.9);
    const data = await page.evaluate(
      ({ start, label }) => {
        const blocks = vigilAudio.outputs.filter(
          (block) => block.contextTime >= start + 0.1 && block.contextTime <= start + 0.85,
        );
        const total = (field) => blocks.reduce((sum, block) => sum + block[field], 0);
        const count = total("frames"),
          context = vigilAudio.contexts[0];
        return {
          label,
          start,
          count,
          sampleRate: context.sampleRate,
          channels: [...new Set(blocks.map((block) => block.channels))],
          left: total("left") / count,
          right: total("right") / count,
          hpLeft: total("hpLeft") / count,
          hpRight: total("hpRight") / count,
          peak: Math.max(...blocks.map((block) => block.peak)),
          root: vigilAudio.positions[vigilAudio.root],
          camera: vigilAudio.positions[vigilAudio.camera],
          cameraHeight: vigilAudio.height,
          receiver: vigilAudio.receivers.at(-1),
        };
      },
      { start, label },
    );
    assert.ok(data.count > data.sampleRate * 0.5, "real worklet output window is incomplete");
    assert.deepEqual(data.channels, [2]);
    report.measurements.push(data);
    return data;
  };
  await press("r");
  await audioWait(1.3);
  const bed = await measure("quiet-church-bed");
  assert.ok(bed.left + bed.right > 1e-7, "streaming church ambience was silent after activation");
  assert.ok(Math.abs(bed.left - bed.right) / (bed.left + bed.right) < 0.02, "mono church bed unexpectedly panned");
  assert.ok(
    (await page.evaluate(() => vigilAudio.music)).some((call) => call.loop === 1),
    "church bed was not looped",
  );
  const castStart = await page.evaluate(() => vigilAudio.starts.length);
  await press("ArrowLeft", 2);
  await press("f");
  await page.waitForFunction(
    ({ first, sound }) => vigilAudio.starts.slice(first).some((start) => start.sound === sound),
    { first: castStart, sound: sounds.FireballImpact },
    { timeout: 15000 },
  );
  await audioWait(0.8);
  assert.ok(
    (await page.evaluate((first) => vigilAudio.starts.slice(first), castStart)).some(
      (start) => start.sound === sounds.FireballCast,
    ),
    "normal F cast did not play its world sound",
  );
  await press("r");
  const walk = async (goal) => {
    const pixel = await page.evaluate(([x, y]) => {
      const [cx, cy] = vigilAudio.positions[vigilAudio.camera],
        rect = document.querySelector("canvas").getBoundingClientRect(),
        scale = rect.height / vigilAudio.height;
      return [
        rect.left + rect.width / 2 + ((x - cx - 18 - y + cy + 18) / Math.SQRT2) * scale,
        rect.top + rect.height / 2 - (((x - cx - 18 + y - cy - 18) * 0.5) / Math.SQRT2) * scale,
      ];
    }, goal);
    assert.ok(
      pixel[0] > 5 && pixel[0] < 1275 && pixel[1] > 110 && pixel[1] < 600,
      `audio route covered/offscreen: ${goal} -> ${pixel}`,
    );
    await page.keyboard.down("Shift");
    await page.mouse.move(...pixel);
    await page.mouse.down();
    await frames(3);
    await page.mouse.up();
    try {
      await page.waitForFunction(
        ([x, y]) =>
          Math.hypot(vigilAudio.positions[vigilAudio.root][0] - x, vigilAudio.positions[vigilAudio.root][1] - y) < 0.15,
        goal,
        { timeout: 15000 },
      );
    } catch (error) {
      report.routeFailure = {
        goal,
        pixel,
        state: await page.evaluate(() => ({
          root: vigilAudio.positions[vigilAudio.root],
          camera: vigilAudio.positions[vigilAudio.camera],
          height: vigilAudio.height,
          updates: vigilAudio.updates,
        })),
      };
      console.log("Audio route failure", JSON.stringify(report.routeFailure));
      await page.screenshot({ path: resolve(output, "route-failure.png") });
      throw error;
    }
    await page.keyboard.up("Shift");
    await frames(4);
  };
  for (const goal of [
    [11.75, 8.25],
    [11.75, 12.25],
    [7.75, 12.25],
    [3.75, 12.25],
  ])
    await walk(goal);
  await press("e");
  await page.waitForFunction((sound) => vigilAudio.starts.some((start) => start.sound === sound), sounds.EmberCrackle, {
    timeout: 10000,
  });
  await audioWait(2.4);
  await walk([3.75, 11.75]);
  await audioWait(0.8);
  const left = await measure("shrine-screen-left");
  await walk([1.75, 13.25]);
  await audioWait(0.8);
  const right = await measure("shrine-screen-right");
  await page.setViewportSize({ width: 390, height: 844 });
  await frames(5);
  await audioWait(0.3);
  const portrait = await measure("portrait-shrine-screen-right");
  assert.deepEqual(portrait.root, right.root, "viewport resize moved the physical audio listener");
  assert.ok(
    portrait.hpRight > portrait.hpLeft * 1.35,
    "portrait resize reversed or flattened actual screen-right audio",
  );
  await page.setViewportSize({ width: 1280, height: 720 });
  await frames(5);
  await walk([3.75, 12.25]);
  await walk([7.75, 12.25]);
  await audioWait(0.8);
  const far = await measure("shrine-distant");
  const evidence = await page.evaluate(() => ({
    starts: vigilAudio.starts,
    stops: vigilAudio.stops,
    receivers: vigilAudio.receivers,
    music: vigilAudio.music,
    contexts: vigilAudio.contexts.map((context) => ({ state: context.state, sampleRate: context.sampleRate })),
  }));
  report.evidence = evidence;
  for (const name of ["FireballCast", "FireballImpact", "ShrineChime", "StoneFootstep", "EmberCrackle"])
    assert.ok(
      evidence.starts.some((start) => start.sound === sounds[name]),
      `ordinary play did not produce ${name}`,
    );
  const source = evidence.starts.find((start) => start.sound === sounds.EmberCrackle);
  assert.ok(source.flags & 1, "shrine crackle did not loop");
  assert.ok(source.flags & 2, "shrine crackle did not retain geometry obstruction");
  const spatial = (measurement) => {
    const delta = source.position.map((value, axis) => value - measurement.root[axis]),
      distance = Math.hypot(...delta),
      cameraDistance = Math.hypot(...source.position.map((value, axis) => value - measurement.camera[axis]));
    return { distance, cameraDistance, pan: (delta[0] - delta[1]) / Math.SQRT2 / distance };
  };
  report.spatial = { left: spatial(left), right: spatial(right), portrait: spatial(portrait), far: spatial(far) };
  for (const value of [left, right, portrait, far]) {
    assert.ok(
      Math.abs(value.receiver.right[0] - Math.SQRT1_2) < 0.0001 &&
        Math.abs(value.receiver.right[1] + Math.SQRT1_2) < 0.0001,
      "receiver right vector does not match dimetric screen right",
    );
    assert.notEqual(value.receiver.id, await page.evaluate(() => vigilAudio.camera), "camera selected as listener");
  }
  assert.ok(
    report.spatial.left.pan < -0.6 && report.spatial.right.pan > 0.6,
    "normal route did not straddle the sound along screen right",
  );
  assert.ok(
    Math.abs(report.spatial.left.distance - report.spatial.right.distance) < 0.2,
    "pan comparison changed source distance",
  );
  assert.ok(
    left.hpLeft > left.hpRight * 1.35 && right.hpRight > right.hpLeft * 1.35,
    "actual high-band WebAudio energy did not reverse screen-left/right dominance",
  );
  const nearEnergy = (left.hpLeft + left.hpRight + right.hpLeft + right.hpRight) / 2,
    farEnergy = far.hpLeft + far.hpRight;
  assert.ok(
    nearEnergy > (bed.hpLeft + bed.hpRight) * 2 + 1e-10,
    "spatial crackle was not measurable above the quiet music bed",
  );
  assert.ok(farEnergy < nearEnergy * 0.6, "actual output did not attenuate with player-to-source distance");
  assert.ok(
    report.spatial.left.distance < source.far &&
      report.spatial.right.distance < source.far &&
      report.spatial.left.cameraDistance > source.far &&
      report.spatial.right.cameraDistance > source.far,
    "route did not distinguish player-root distance from camera distance",
  );
  const receiver = await page.evaluate(() => ({ root: vigilAudio.root, receiver: vigilAudio.receivers.at(-1).id }));
  assert.equal(receiver.receiver, receiver.root, "receiver must be the physical shadow root");
  await press("r");
  await audioWait(0.3);
  assert.ok(
    (await page.evaluate(() => vigilAudio.stops)).some((stop) => stop.id === source.id),
    "reset did not stop shrine loop",
  );
  assert.deepEqual(report.errors, []);
  report.analysis =
    "High-band energy applies an identical four-stage 1.8 kHz high-pass only to observed output, separating crackle from the harmonic church bed; all samples sent to the real device remain unchanged.";
  report.checks.push(
    "actual WASM sound/music imports and running stereo WebAudio output",
    "ordinary movement/cast/impact/shrine actions play all five sounds",
    "shadow-root receiver and dimetric camera-right stereo agree",
    "equal-distance shrine positions reverse actual output channel energy",
    "390x844 portrait resize preserves the physical listener, screen-right basis and actual channel dominance",
    "actual crackle output attenuates with player distance while offset camera remains beyond sound range",
    "restart releases shrine loop",
  );
  report.passed = true;
  console.log(
    JSON.stringify(
      { passed: true, spatial: report.spatial, measurements: report.measurements, checks: report.checks },
      null,
      2,
    ),
  );
} finally {
  clearTimeout(deadline);
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  await browser.close();
  server.closeAllConnections();
  await new Promise((done) => server.close(done));
}
