// Measure original production worklet output. No gain/sample/device hooks alter
// playback; only ordinary music selection and real pointer burst input are used.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { mkdir, readFile, realpath, writeFile } from "node:fs/promises";
import { dirname, extname, resolve } from "node:path";
import { chromium } from "playwright";

const directory = await realpath(resolve(process.argv[2] || "samples/media-lab/dist/web"));
const wasm = resolve(process.argv[3] || resolve(directory, "karty-host.wasm"));
const output = resolve(process.env.KARTY_AUDIO_QUALITY_OUTPUT || "dist/validation/media-lab/audio");
await mkdir(output, { recursive: true });
const server = createServer(async (request, response) => {
  try {
    const path = decodeURIComponent(new URL(request.url, "http://localhost").pathname);
    let file;
    if (path === "/karty-host.wasm") file = wasm;
    else if (path === "/wasm_exec.js") file = resolve(dirname(wasm), "wasm_exec.js");
    else {
      file = await realpath(resolve(directory, "." + (path === "/" ? "/index.html" : path)));
      if (!file.startsWith(directory + "/")) throw new Error("outside distribution");
    }
    const bytes = await readFile(file);
    response.writeHead(200, {
      "Content-Type":
        { ".html": "text/html", ".js": "text/javascript", ".wasm": "application/wasm", ".kart": "application/wasm" }[
          extname(file)
        ] || "application/octet-stream",
      "Content-Length": bytes.length,
      "Cache-Control": "no-store",
    });
    response.end(bytes);
  } catch {
    response.writeHead(404).end();
  }
});
await new Promise((done) => server.listen(0, "127.0.0.1", done));
let browser, deadline;
try {
  browser = await chromium.launch({
    headless: true,
    timeout: 9000,
    args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  deadline = setTimeout(() => void browser.close(), 45000);
  const page = await browser.newPage({ viewport: { width: 960, height: 540 } });
  const profiler = process.env.KARTY_AUDIO_PROFILE ? await page.context().newCDPSession(page) : null;
  if (profiler) {
    await profiler.send("Profiler.enable");
    await profiler.send("Profiler.start");
  }
  page.setDefaultTimeout(30000);
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  await page.addInitScript(() => {
    const evidence = (globalThis.audioQuality = { contexts: [], outputs: [], effects: [], tasks: [] });
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries())
        evidence.tasks.push({
          start: entry.startTime,
          duration: entry.duration,
          endContext: evidence.contexts[0]?.currentTime,
          at: performance.now(),
        });
    }).observe({ entryTypes: ["longtask"] });
    const Context = globalThis.AudioContext;
    globalThis.AudioContext = new Proxy(Context, {
      construct(target, args) {
        const context = Reflect.construct(target, args);
        evidence.contexts.push(context);
        const addModule = context.audioWorklet.addModule.bind(context.audioWorklet);
        context.audioWorklet.addModule = async (url, ...options) => {
          const source = await (await fetch(url)).text();
          if (!source.includes("class OtoWorkletProcessor")) throw new Error("unexpected audio worklet");
          const audit = `
            const original=OtoWorkletProcessor.prototype.process;
            OtoWorkletProcessor.prototype.process=function(inputs,outputs,parameters){
              const underrun=this.buf_.length<outputs[0][0].length*this.channelCount_;
              const result=original.call(this,inputs,outputs,parameters);
              this.quality_ ||= {start:currentFrame/sampleRate,blocks:0,peak:0,jump:0,over:0,underruns:0};
              this.lastQuality_ ||= [0,0];
              if(underrun)this.quality_.underruns++;
              for(let c=0;c<outputs[0].length;c++)for(const value of outputs[0][c]){
                this.quality_.peak=Math.max(this.quality_.peak,Math.abs(value));
                this.quality_.jump=Math.max(this.quality_.jump,Math.abs(value-this.lastQuality_[c]));
                this.lastQuality_[c]=value;
                if(Math.abs(value)>1)this.quality_.over++;
              }
              if(++this.quality_.blocks===32){this.port.postMessage({__audioQuality:true,...this.quality_});this.quality_=null;}
              return result;
            };`;
          const injected = URL.createObjectURL(new Blob([source, audit], { type: "text/javascript" }));
          try {
            return await addModule(injected, ...options);
          } finally {
            URL.revokeObjectURL(injected);
          }
        };
        return context;
      },
    });
    const Node = globalThis.AudioWorkletNode;
    globalThis.AudioWorkletNode = new Proxy(Node, {
      construct(target, args) {
        const node = Reflect.construct(target, args);
        node.port.addEventListener("message", (event) => {
          if (event.data?.__audioQuality) {
            evidence.outputs.push(event.data);
            event.stopImmediatePropagation();
          }
        });
        node.port.start();
        return node;
      },
    });
    let play;
    Object.defineProperty(globalThis, "kartyHostPlaySound", {
      configurable: true,
      get: () => play,
      set(original) {
        play = (...args) => {
          evidence.effects.push({ args, time: evidence.contexts[0]?.currentTime });
          return original(...args);
        };
      },
    });
  });
  await page.goto(`http://127.0.0.1:${server.address().port}/?worldScale=0.5&aa=false`);
  await page.waitForFunction(() => typeof kartyHostPlayMusic === "function" && document.querySelector("canvas")?.width);
  await page.keyboard.press("Shift");
  await page.waitForFunction(() => audioQuality.contexts.some((context) => context.state === "running"));
  await page.evaluate(() => kartyHostPlayMusic(1, 1, 500));
  await page.waitForFunction(() => audioQuality.outputs.some((block) => block.peak > 0.05));
  await page.waitForTimeout(1500);
  const baselineStart = await page.evaluate(() => audioQuality.contexts[0].currentTime);
  await page.waitForTimeout(5000);
  const baselineEnd = await page.evaluate(() => audioQuality.contexts[0].currentTime);
  for (let burst = 0; burst < 2; burst++) {
    const box = await page.locator("canvas").boundingBox();
    await page.mouse.move(box.x + (box.width * 450) / 960, box.y + (box.height * 450) / 540);
    await page.mouse.down();
    await page.waitForTimeout(150);
    await page.mouse.up();
    await page.waitForTimeout(1400);
    assert.ok(
      (await page.evaluate(() => audioQuality.effects.length)) >= (burst + 1) * 36,
      "normal burst input did not reach the cartridge",
    );
  }
  const report = await page.evaluate(
    ({ baselineStart, baselineEnd }) => ({
      contexts: audioQuality.contexts.map((context) => ({ state: context.state, sampleRate: context.sampleRate })),
      outputs: audioQuality.outputs.filter((block) => block.start >= baselineStart),
      music: audioQuality.outputs.filter(
        (block) => block.start >= baselineStart && block.start + 4096 / 48000 <= baselineEnd,
      ),
      effects: audioQuality.effects,
      tasks: audioQuality.tasks,
    }),
    { baselineStart, baselineEnd },
  );
  report.errors = errors;
  if (profiler) {
    const { profile } = await profiler.send("Profiler.stop");
    await writeFile(resolve(output, "media-lab.cpuprofile"), JSON.stringify(profile));
  }
  report.provenance = {};
  for (const [name, file] of [
    ["cartridge", resolve(directory, "game.kart")],
    ["host", wasm],
  ]) {
    report.provenance[name] = createHash("sha256")
      .update(await readFile(file))
      .digest("hex");
  }
  await writeFile(resolve(output, "media-lab.json"), JSON.stringify(report, null, 2) + "\n");
  assert.deepEqual(errors, []);
  assert.ok(report.music.length >= 40, "insufficient music output to cross its loop");
  assert.equal(
    report.music.reduce((sum, block) => sum + block.underruns, 0),
    0,
    "music starved the worklet",
  );
  assert.ok(
    Math.max(...report.music.map((block) => block.jump)) < 0.08,
    "music has a discontinuous loop or playback gap",
  );
  assert.equal(
    report.outputs.reduce((sum, block) => sum + block.over, 0),
    0,
    "sample mix exceeds full scale",
  );
  assert.equal(
    report.outputs.reduce((sum, block) => sum + block.underruns, 0),
    0,
    "effect bursts starved the worklet",
  );
  const peak = Math.max(...report.outputs.map((block) => block.peak));
  assert.ok(peak > 0.1 && peak <= 0.95, `mixed output lacks signal/headroom: ${peak}`);
  console.log(
    JSON.stringify({
      peak,
      musicBlocks: report.music.length,
      musicUnderruns: 0,
      effectStarts: report.effects.length,
      provenance: report.provenance,
    }),
  );
} finally {
  clearTimeout(deadline);
  await browser?.close();
  await new Promise((done) => server.close(done));
}
