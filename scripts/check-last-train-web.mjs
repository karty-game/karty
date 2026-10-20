// Actual Yarn-driven cartridge, canvas controls and persistent engine storage.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { mkdir, readFile, realpath, rm, writeFile } from "node:fs/promises";
import { extname, resolve } from "node:path";
import { chromium } from "playwright";

const directory = await realpath(
  resolve(process.argv.slice(2).find((arg) => !arg.startsWith("--")) || "samples/last-train/dist/web"),
);
const output = resolve(process.env.KARTY_LAST_TRAIN_OUTPUT || "dist/validation/last-train/browser");
const audioOnly = process.argv.includes("--audio-only");
const readingOnly = process.argv.includes("--reading-only");
const directionOnly = process.argv.includes("--direction-only");
await mkdir(output, { recursive: true });
await rm(resolve(output, "failure.png"), { force: true });
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");
const report = {
  passed: false,
  backend: "Chromium SwiftShader; full 1280×720 desktop and 390×844 portrait; synthesized mouse/touch",
  cartridgeSHA256: digest(await readFile(resolve(directory, "game.kart"))),
  hostSHA256: digest(await readFile(resolve(directory, "karty-host.wasm"))),
  launcherSHA256: digest(await readFile(resolve(directory, "karty.js"))),
  indexSHA256: digest(await readFile(resolve(directory, "index.html"))),
  checks: [],
  captures: [],
  errors: [],
};
const server = createServer(async (request, response) => {
  try {
    const path = new URL(request.url, "http://localhost").pathname;
    const file = await realpath(resolve(directory, "." + (path === "/" ? "/index.html" : path)));
    if (!file.startsWith(directory + "/")) throw new Error("outside distribution");
    response.writeHead(200, {
      "Content-Type":
        { ".html": "text/html", ".js": "text/javascript", ".wasm": "application/wasm", ".kart": "application/wasm" }[
          extname(file)
        ] || "application/octet-stream",
      "Cache-Control": "no-store",
    });
    response.end(await readFile(file));
  } catch {
    response.writeHead(404).end();
  }
});
await new Promise((done) => server.listen(0, "127.0.0.1", done));
const url = (process.env.KARTY_LAST_TRAIN_URL || `http://127.0.0.1:${server.address().port}`).replace(/\/$/, "");
if (process.env.KARTY_LAST_TRAIN_URL) {
  for (const [file, expected] of [
    ["game.kart", report.cartridgeSHA256],
    ["karty-host.wasm", report.hostSHA256],
    ["karty.js", report.launcherSHA256],
    ["index.html", report.indexSHA256],
  ]) {
    const response = await fetch(url + "/" + file);
    assert.ok(response.ok, `preview ${file}: HTTP ${response.status}`);
    assert.equal(
      digest(Buffer.from(await response.arrayBuffer())),
      expected,
      `preview ${file} differs from intended artifact`,
    );
  }
  report.previewURL = url + "/";
}

try {
  if (directionOnly)
    await (
      await import("./check-last-train-direction.mjs")
    ).runDirectionChecks({
      fixture,
      open,
      state,
      advance,
      advanceTo,
      advanceToChoices,
      choose,
      toolbar,
      status,
      capture,
      report,
      motion,
      ready,
      begin,
      click,
    });
  else if (readingOnly)
    await fixture(async (page) => {
      await open(page);
      const initial = await state(page);
      assert.equal(initial.speaker, "");
      assert.equal(initial.narration, true);
      assert.ok(["»", "▾"].includes(initial.prompt));
      const buttons = await buttonBands(page);
      await page.waitForFunction(() => {
        const d = trainDialogue();
        return (
          typeof d.revealCount === "number" &&
          d.display?.length > 35 &&
          d.display.length < d.text.length &&
          !/\s$/.test(d.display) &&
          !/\s/.test(d.text[d.display.length])
        );
      });
      const partial = await state(page);
      assert.ok(!/\s$/.test(partial.display), "character reveal waited for a complete word");
      report.revealCounts = await page.evaluate(() => {
        const dialogue = trainDialogue();
        return trainProbe.revealUpdates.filter((entry) => entry.text === dialogue.text).map((entry) => entry.count);
      });
      assert.ok(report.revealCounts.includes(1), "first character was not independently revealed");
      assert.ok(
        report.revealCounts.some((count, index, values) => index > 0 && count === values[index - 1] + 1),
        "reveal did not advance one Unicode scalar at a time",
      );
      const partialImage = await canvasImage(page);
      assert.equal((await state(page)).revealing, true);
      await click(page, buttons[0].x, buttons[0].y);
      assert.equal((await state(page)).text, partial.text);
      assert.equal((await state(page)).display, partial.text);
      const fullImage = await canvasImage(page);
      report.wrapping = await page.evaluate(
        async ({ partialImage, fullImage }) => {
          async function pixels(url) {
            const bitmap = await createImageBitmap(await (await fetch(url)).blob());
            const canvas = new OffscreenCanvas(bitmap.width, bitmap.height),
              ctx = canvas.getContext("2d");
            ctx.drawImage(bitmap, 0, 0);
            return {
              width: bitmap.width,
              height: bitmap.height,
              data: ctx.getImageData(0, 0, bitmap.width, bitmap.height).data,
            };
          }
          const a = await pixels(partialImage),
            b = await pixels(fullImage);
          function narrationBounds(image) {
            const ruleColor = [184, 145, 75];
            const isRule = (x, y) => {
              const at = (y * image.width + x) * 4;
              return ruleColor.every((value, channel) => Math.abs(image.data[at + channel] - value) < 3);
            };
            let rule = -1;
            for (let y = Math.floor(image.height * 0.4); y < image.height; y++) {
              let count = 0;
              for (let x = 30; x < image.width - 30; x += 3) if (isRule(x, y)) count++;
              if (count > (image.width - 60) / 6) {
                rule = y;
                break;
              }
            }
            if (rule < 0) throw new Error("dialogue rule missing");
            let left = 0,
              right = 0,
              run = 0;
            for (let x = 0; x <= image.width; x++) {
              if (x < image.width && isRule(x, rule)) continue;
              if (x - run > right - left) [left, right] = [run, x];
              run = x + 1;
            }
            if (right - left < image.width / 2) throw new Error("dialogue rule too short");
            // NovelFrame paints #0a1523f2 around the prose. Probe both padding
            // stripes, beyond the rule's ends, so glyphs cannot obscure its top.
            // Account for every possible backdrop under the translucent panel.
            const panelColor = [10, 21, 35],
              panelAlpha = 242 / 255;
            const probes = [left - 4, left - 2, right + 2, right + 4];
            const paintedPanel = (y) =>
              probes.filter((x) => {
                if (x < 0 || x >= image.width) return false;
                const at = (y * image.width + x) * 4;
                return panelColor.every((value, channel) => {
                  const minimum = value * panelAlpha;
                  return (
                    image.data[at + channel] >= minimum - 2 &&
                    image.data[at + channel] <= minimum + 255 * (1 - panelAlpha) + 2
                  );
                });
              }).length >= 3;
            let top = rule - 3;
            if (!paintedPanel(top)) throw new Error("dialogue panel padding missing");
            while (top > 0 && paintedPanel(top - 1)) top--;
            if (rule - top < 30) throw new Error("dialogue prose region too short");
            return { left, right, top, bottom: rule - 3, rule };
          }
          const partialBounds = narrationBounds(a),
            bounds = narrationBounds(b);
          let ink = 0,
            fullInk = 0,
            moved = 0;
          for (let y = bounds.top; y < bounds.bottom; y++)
            for (let x = bounds.left; x < bounds.right; x++) {
              const at = (y * a.width + x) * 4;
              if ([212, 195, 166].every((v, c) => Math.abs(b.data[at + c] - v) < 8)) fullInk++;
              if ([212, 195, 166].every((v, c) => Math.abs(a.data[at + c] - v) < 8)) {
                ink++;
                if ([0, 1, 2].some((c) => Math.abs(a.data[at + c] - b.data[at + c]) > 12)) moved++;
              }
            }
          return { ink, fullInk, moved, bounds, partialBounds };
        },
        { partialImage, fullImage },
      );
      assert.deepEqual(
        report.wrapping.partialBounds,
        report.wrapping.bounds,
        "narration panel changed bounds during reveal",
      );
      assert.ok(report.wrapping.ink > 20, "no revealed narration ink observed");
      assert.equal(report.wrapping.moved, 0, "already revealed text changed pixel position");
      assert.ok(report.wrapping.fullInk > report.wrapping.ink, "unrevealed glyphs were already painted");
      for (const [name, url] of [
        ["reading-partial", partialImage],
        ["reading-narration", fullImage],
      ]) {
        const bytes = Buffer.from(url.split(",")[1], "base64");
        await writeFile(resolve(output, name + ".png"), bytes);
        report.captures.push({ name, sha256: digest(bytes) });
      }
      await advanceTo(page, "Forgive me.");
      assert.equal((await state(page)).speaker, "Conductor");
      await capture(page, "conductor-before-introduction");
      await advanceTo(page, "My name is Mara.");
      assert.equal((await state(page)).speaker, "Conductor");
      await advance(page);
      await advance(page);
      assert.equal((await state(page)).speaker, "Mara");
      await capture(page, "mara-after-introduction");
      await advanceToChoices(page);
      assert.ok((await state(page)).choices.every((v) => !v.text.includes("Elias") && !v.text.includes("Iris")));
      await choose(page, 2);
      assert.equal((await state(page)).speaker, "Traveller");
      await capture(page, "traveller-before-introduction");
      await advanceTo(page, "My name is Iris.");
      assert.equal((await state(page)).speaker, "Traveller");
      await advance(page);
      assert.ok(!(await state(page)).text.includes("Elias"));
      await advance(page);
      assert.equal((await state(page)).speaker, "Iris");
      await advanceToChoices(page);
      await choose(page, 0);
      await advanceToChoices(page);
      await choose(page, 1);
      assert.equal((await state(page)).speaker, "Older passenger");
      await capture(page, "passenger-before-introduction");
      await advanceTo(page, "Elias. Retired");
      assert.equal((await state(page)).speaker, "Older passenger");
      await advance(page);
      assert.equal((await state(page)).speaker, "Elias");
      await page.keyboard.press("Escape");
      await page.waitForFunction(() => Object.values(trainProbe.instances).some((v) => v.name === "dialogue.history"));
      assert.ok(
        await page.evaluate(() =>
          Object.values(trainProbe.instances)
            .filter((v) => v.name === "dialogue.history-entry")
            .every((v) => v.values[1]?.text !== "Narrator"),
        ),
      );
      await capture(page, "reading-history", false);
      report.checks.push(
        "narration without a nameplate, distinct prose typography, icon prompts, fixed revealed-character pixels, introductions before names and anonymous history",
      );
    }, process.argv.includes("--phone-only"));
  if (audioOnly)
    await fixture(async (page) => {
      await open(page);
      // Stay on the platform: no rail ambience or Foley can mask the score.
      await page.waitForFunction(() => trainAudio.frames >= 48000 * 90 || trainAudio.gaps.length > 0, null, {
        timeout: 105000,
      });
      const audio = await page.evaluate(() => trainAudio);
      report.audio = audio;
      assert.ok(audio.peak > 0.01 && audio.peak < 0.5, "score output silent or over its headroom");
      assert.ok(audio.maxZeroRun < 4800, "score dropped to digital silence for 100 ms");
      await capture(page, "score-loop");
      await toolbar(page, 3);
      await status(page, "Music off");
      await page.waitForTimeout(3500);
      const silent = await page.evaluate(() => trainAudio.windows.slice(-3));
      assert.ok(
        silent.every((v) => v.peak === 0),
        "music toggle left audible output",
      );
      report.checks.push(
        "actual 90-second WebAudio score playback across the 83.478-second loop, bounded peak, no 100 ms digital dropout and music-off silence",
      );
    });
  if (process.argv.includes("--pacing-only"))
    await fixture(async (page) => {
      await page.goto(url + "/?debug=0");
      await ready(page);
      assert.equal((await state(page)).title, true);
      await capture(page, "opening-title");
      await page.waitForTimeout(600);
      assert.equal((await state(page)).title, true, "opening advanced without the player");
      await begin(page);
      const before = await buttonBands(page);
      await toolbar(page, 5);
      await begin(page);
      await page.waitForFunction(() => {
        const v = trainDialogue();
        return v.display?.length > 0 && v.display.length < v.text.length;
      });
      const partial = await state(page);
      const imageURL = await page.evaluate(
        () =>
          new Promise((done) =>
            requestAnimationFrame(() => done(document.querySelector("canvas").toDataURL("image/png"))),
          ),
      );
      const revealBytes = Buffer.from(imageURL.split(",")[1], "base64");
      await writeFile(resolve(output, "opening-reveal.png"), revealBytes);
      report.captures.push({ name: "opening-reveal", sha256: digest(revealBytes) });
      await click(page, before[0].x, before[0].y);
      const complete = await state(page);
      assert.equal(complete.text, partial.text, "reveal click skipped the line");
      assert.equal(complete.display, complete.text);
      const after = await buttonBands(page);
      assert.equal(before[0].y, after[0].y, "reveal moved the Continue button");
      await capture(page, "opening-platform");
      // Boarding takes several player-controlled beats before Mara introduces the case.
      await advanceTo(page, "The whistle sounds");
      if ((await state(page)).revealing) await clickDialogue(page, 0);
      await clickDialogue(page, 0);
      await page.waitForFunction(() =>
        Object.values(trainProbe.instances).some((v) => v.name === "ui.game-screen" && v.visible[31]),
      );
      await page.waitForTimeout(450);
      const curtain = await state(page);
      assert.equal(curtain.cover, true);
      await capture(page, "boarding-card");
      await page.waitForFunction(() => trainDialogue().text?.startsWith("Warmth"));
      await capture(page, "boarding-carriage");
      await advanceTo(page, "My name is Mara");
      await capture(page, "mara-introduction");
      report.checks.push(
        "player-started title, Unicode-safe progressive reveal, reveal-before-advance, stable layout and timed boarding transition",
      );
    }, process.argv.includes("--phone-only"));
  if (
    !directionOnly &&
    !audioOnly &&
    !readingOnly &&
    !process.argv.includes("--phone-only") &&
    !process.argv.includes("--pacing-only")
  )
    await fixture(async (page) => {
      await open(page);
      const first = (await state(page)).text;
      await toolbar(page, 0);
      await status(page, "Checkpoint saved");
      await toolbar(page, 3);
      await status(page, "Music off");
      await toolbar(page, 4);
      await status(page, "Train ambience off");
      // No run save after changing preferences: profile must persist independently.
      await page.reload();
      await ready(page);
      await toolbar(page, 1);
      await status(page, "Checkpoint restored");
      assert.equal((await state(page)).text, first);
      await toolbar(page, 3);
      await status(page, "Music on");
      await toolbar(page, 4);
      await status(page, "Train ambience on");
      report.checks.push(
        "profile preferences survive reload without saving a run, and old run load keeps current preferences",
      );

      await selectSlot(page, "2");
      await toolbar(page, 1);
      await status(page, "No checkpoint in slot 2");
      await advance(page);
      const second = (await state(page)).text;
      assert.notEqual(second, first);
      await toolbar(page, 0);
      await status(page, "Checkpoint saved");
      await selectSlot(page, "3");
      await toolbar(page, 1);
      await status(page, "No checkpoint in slot 3");
      await advance(page);
      const third = (await state(page)).text;
      assert.notEqual(third, second);
      await toolbar(page, 0);
      await status(page, "Checkpoint saved");
      await selectSlot(page, "1");
      await toolbar(page, 1);
      await status(page, "Checkpoint restored");
      assert.equal((await state(page)).text, first);
      await selectSlot(page, "2");
      await toolbar(page, 1);
      await status(page, "Checkpoint restored");
      assert.equal((await state(page)).text, second);
      await selectSlot(page, "3");
      await toolbar(page, 1);
      await status(page, "Checkpoint restored");
      assert.equal((await state(page)).text, third);
      await toolbar(page, 5);
      await begin(page);
      assert.equal((await state(page)).text, first);
      await toolbar(page, 1);
      await status(page, "Checkpoint restored");
      assert.equal((await state(page)).text, third);
      report.checks.push("three independent run slots retain distinct story positions across load and restart");
      await capture(page, "run-slots");
    });
  for (const ending of directionOnly ||
  audioOnly ||
  readingOnly ||
  process.argv.includes("--phone-only") ||
  process.argv.includes("--pacing-only")
    ? []
    : process.argv.includes("--quick")
      ? ["timetable"]
      : ["truth", "kindness", "timetable"]) {
    let investigation;
    await fixture(async (page) => {
      await open(page);
      await capture(page, `${ending}-platform`);
      await advanceTo(page, "Rain threads the glass");
      await advance(page);
      if (ending !== "timetable") {
        await choose(page, 0); // Mara
        await capture(page, "mara");
        await advanceToChoices(page);
        await choose(page, 1); // Trust rather than value
        await advanceToChoices(page);
        await choose(page, 1); // Elias
        await capture(page, "elias");
        await advanceToChoices(page);
        await choose(page, 0); // North Quay
        await advanceToChoices(page);
        await choose(page, 2); // Iris
        await capture(page, "iris");
        await advanceToChoices(page);
        await choose(page, 0); // Listen
        await advanceToChoices(page);
        if (ending === "truth") {
          const before = await state(page);
          await toolbar(page, 0);
          await status(page, "Checkpoint saved");
          const stored = await page.evaluate(() =>
            Object.entries(localStorage).filter(([key]) => key.startsWith("karty.save.v1/")),
          );
          assert.equal(stored.length, 1);
          await page.reload();
          await ready(page);
          assert.equal((await state(page)).title, true);
          await toolbar(page, 1);
          await status(page, "Checkpoint restored");
          const restored = await state(page);
          assert.equal(restored.text, before.text);
          assert.deepEqual(restored.choices, before.choices);
          report.checks.push("choice checkpoint survives a complete page/guest restart");
          // Identical saves intentionally skip storage I/O. Move to another
          // story state so the injected failure exercises a real changed write.
          await choose(page, 3);
          assert.notEqual((await state(page)).text, before.text);
          // A failed write must keep the previous successful checkpoint.
          await page.evaluate(() => {
            globalThis.originalSetItem = Storage.prototype.setItem;
            Storage.prototype.setItem = () => {
              throw new DOMException("injected full storage", "QuotaExceededError");
            };
          });
          await toolbar(page, 0);
          await status(page, "Could not save");
          assert.deepEqual(
            await page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith("karty.save.v1/"))),
            stored,
          );
          await page.evaluate(() => {
            Storage.prototype.setItem = globalThis.originalSetItem;
          });
          await toolbar(page, 1);
          await status(page, "Checkpoint restored");
          const afterFailedWrite = await state(page);
          assert.equal(afterFailedWrite.text, before.text);
          assert.deepEqual(afterFailedWrite.choices, before.choices);
          report.checks.push("injected browser quota failure preserves the previous checkpoint");
        }
      }
      // Transfer a real engine-written checkpoint into a new browser lifetime.
      // Investigation and conclusion retain the same strict fixture deadline.
      await selectSlot(page, "2");
      await toolbar(page, 0);
      await status(page, "Checkpoint saved");
      investigation = {
        state: await state(page),
        entries: await page.evaluate(() =>
          Object.entries(localStorage).filter(([key]) => key.startsWith("karty.save.v1/")),
        ),
      };
      assert.ok(investigation.entries.length > 0, "investigation checkpoint was not persisted");
    });
    await fixture(async (page) => {
      await page.addInitScript((entries) => {
        // Seed only the fresh browser. Later page reloads must retain the
        // lifetime profile and slots written by the actual running game.
        if (!Object.keys(localStorage).some((key) => key.startsWith("karty.save.v1/")))
          for (const [key, value] of entries) localStorage.setItem(key, value);
      }, investigation.entries);
      await open(page);
      await selectSlot(page, "2");
      await toolbar(page, 1);
      await status(page, "Checkpoint restored");
      const resumed = await state(page);
      assert.equal(resumed.text, investigation.state.text);
      assert.deepEqual(resumed.choices, investigation.state.choices);
      report.checks.push(`${ending}: investigation checkpoint survives closing and reopening the browser`);
      await choose(page, 3); // Dining car
      await capture(page, `${ending}-dining`);
      await advanceToChoices(page);
      const options = (await state(page)).choices;
      assert.equal(options.length, 4);
      assert.equal(options[0].enabled, ending !== "timetable");
      assert.equal(options[1].enabled, ending !== "timetable");
      if (ending === "timetable") {
        const before = (await state(page)).text;
        await clickDialogue(page, 0);
        assert.equal((await state(page)).text, before, "disabled choice advanced the story");
        assert.equal((await state(page)).choices.length, 4);
      }
      await capture(page, `${ending}-choices`);
      await choose(page, { truth: 0, kindness: 1, timetable: 2 }[ending]);
      const titles = { truth: "THE RETURN JOURNEY", kindness: "A LIGHT LEFT ON", timetable: "THE EMPTY PLATFORM" };
      await advanceTo(page, titles[ending]);
      await capture(page, `${ending}-ending`);
      await advance(page);
      await status(page, "End of the line");
      report.checks.push(`${ending}: actual Yarn conditions, commands, scene changes and ending`);
      if (ending === "truth") {
        let text = (await state(page)).text;
        await toolbar(page, 2);
        await page.waitForFunction(() =>
          Object.values(trainProbe.instances).some((v) => v.name === "dialogue.history"),
        );
        await capture(page, "conversation-log");
        await page.keyboard.press("Escape");
        await page.waitForFunction(
          () => !Object.values(trainProbe.instances).some((v) => v.name === "dialogue.history"),
        );
        assert.equal((await state(page)).text, text);
        await toolbar(page, 3);
        await status(page, "Music off");
        await toolbar(page, 4);
        await status(page, "Train ambience off");
        const audio = await page.evaluate(() => trainAudio);
        assert.ok(
          audio.blocks > 0 && audio.peak > 0.001 && audio.peak <= 1,
          "actual WebAudio output was silent or clipped",
        );
        assert.ok(audio.sounds >= 2, "ticket punch and station whistle were not dispatched");
        report.audio = audio;
        report.checks.push(
          "nonzero actual Oto worklet output and both ending sound effects; no physical listening qualification",
        );
        await selectSlot(page, "3");
        await selectSlot(page, "1");
        await toolbar(page, 1);
        await status(page, "Checkpoint restored");
        await profileLabel(page, "Slot 1 · Endings 1/3");
        await toolbar(page, 3);
        await status(page, "Music on");
        await toolbar(page, 4);
        await status(page, "Train ambience on");
        await page.reload();
        await ready(page);
        await profileLabel(page, "Slot 1 · Endings 1/3");
        await toolbar(page, 1);
        await status(page, "Checkpoint restored");
        await profileLabel(page, "Slot 1 · Endings 1/3");
        text = (await state(page)).text;
        report.checks.push(
          "unlocked ending survives older-run load, restart and page reload independently of run slots",
        );
        // Corrupt external data is rejected while retaining the current scene.
        await page.evaluate(() =>
          localStorage.setItem(
            Object.keys(localStorage).find((key) => key.startsWith("karty.save.v1/")),
            "corrupt",
          ),
        );
        await toolbar(page, 1);
        await status(page, "Could not load");
        assert.equal((await state(page)).text, text);
        await page.evaluate(() =>
          Object.defineProperty(globalThis, "localStorage", {
            configurable: true,
            get() {
              throw new DOMException("injected denied storage", "SecurityError");
            },
          }),
        );
        await toolbar(page, 1);
        await status(page, "save storage unavailable");
        assert.equal((await state(page)).text, text);
        report.checks.push("conversation log, audio switches, corrupt-save and denied-storage retention");
      }
    });
  }
  if (
    !directionOnly &&
    !audioOnly &&
    !readingOnly &&
    !process.argv.includes("--quick") &&
    !process.argv.includes("--pacing-only")
  )
    await fixture(async (page) => {
      await open(page);
      await capture(page, "phone-platform");
      await advanceToChoices(page);
      await choose(page, 2);
      assert.equal((await state(page)).speaker, "Traveller");
      await capture(page, "phone-iris");
      await advanceToChoices(page);
      await choose(page, 0);
      await advanceToChoices(page);
      await choose(page, 3);
      await advanceToChoices(page);
      await capture(page, "phone-choices");
      assert.equal((await state(page)).choices.length, 4);
      report.checks.push("portrait touch navigation, blonde Iris and four visible choice buttons");
    }, true);
  assert.ok(report.checks.length > 0, "no browser checks executed");
  report.passed = true;
} finally {
  server.closeAllConnections();
  await new Promise((done) => server.close(done));
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
}
console.log(`Last Train passed: ${report.checks.length} checks, ${report.captures.length} captures`);

async function fixture(run, phone = false) {
  const browser = await chromium.launch({
    headless: true,
    args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  const deadline = setTimeout(() => void browser.close(), 120000);
  try {
    const page = await browser.newPage({
      viewport: phone ? { width: 390, height: 844 } : { width: 1280, height: 720 },
      hasTouch: phone,
      isMobile: phone,
    });
    page.trainTouch = phone;
    if (phone) page.trainTouchSession = await page.context().newCDPSession(page);
    page.setDefaultTimeout(15000);
    const messages = [];
    page.on("console", (message) => {
      messages.push(message.text());
      if (messages.length > 40) messages.shift();
    });
    page.on("pageerror", (error) => report.errors.push(String(error)));
    await page.addInitScript(() => {
      // Observe the SDK's KTYM v2 metadata; this helper runs only in the checker.
      const metadata = (bytes) => {
        const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
        let at = 8,
          nodes = 0;
        const fail = () => {
          throw new Error("Invalid KTYM v2 observation");
        };
        const take = (size) => {
          if (size < 0 || size > bytes.length - at) fail();
          const start = at;
          at += size;
          return start;
        };
        const count = () => view.getUint32(take(4), true);
        const text = () => {
          const size = count(),
            start = take(size);
          return new TextDecoder("utf-8", { fatal: true }).decode(bytes.subarray(start, start + size));
        };
        const value = (depth) => {
          if (depth > 16 || ++nodes > 2048) fail();
          switch (view.getUint8(take(1))) {
            case 0:
              return null;
            case 1:
              return false;
            case 2:
              return true;
            case 3:
              return Number(view.getBigInt64(take(8), true));
            case 4:
              return Number(view.getBigUint64(take(8), true));
            case 5:
              return view.getFloat64(take(8), true);
            case 6:
              return text();
            case 7: {
              const length = count(),
                start = take(length);
              return bytes.subarray(start, start + length);
            }
            case 9: {
              const length = count();
              if (length > 2048) fail();
              return Array.from({ length }, () => value(depth + 1));
            }
            case 8: {
              const length = count(),
                result = Object.create(null);
              if (length > 2048) fail();
              for (let i = 0; i < length; i++) {
                const key = text();
                if (Object.hasOwn(result, key)) fail();
                result[key] = value(depth + 1);
              }
              return result;
            }
            default:
              return fail();
          }
        };
        if (
          bytes.length < 13 ||
          new TextDecoder().decode(bytes.subarray(0, 4)) !== "KTYM" ||
          view.getUint16(4, true) !== 2 ||
          view.getUint16(6, true) !== 0
        )
          fail();
        const result = value(0);
        if (at !== bytes.length) fail();
        return result;
      };
      globalThis.trainProbe = {
        instances: {},
        revealUpdates: [],
        sprites: {},
        textures: {},
        packs: {},
        requests: [],
        frame: 0,
      };
      let update;
      Object.defineProperty(globalThis, "kartyClientUpdate", {
        configurable: true,
        get: () => update,
        set(original) {
          update = (frame, bytes) => {
            trainProbe.frame++;
            const v = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
            for (let at = 24; at < bytes.length;) {
              const tag = v.getUint8(at),
                size = v.getUint16(at + 1, true),
                p = at + 3;
              if (tag === 4) {
                const h = v.getUint32(p + 4, true),
                  n = v.getUint16(p + 8, true);
                const m = metadata(bytes.subarray(p + 10, p + 10 + n));
                trainProbe.packs[h] = { name: m.name, frame: trainProbe.frame };
                for (const t of m.kartyTextures || []) trainProbe.textures[h + ":" + t.id] = "train." + t.name;
              }
              at = p + size;
            }
            return original(frame, bytes);
          };
        },
      });
      globalThis.trainDialogue = () => {
        const v = Object.values(trainProbe.instances).find((v) => v.name === "dialogue.dialogue");
        const narration = v?.visible[1] === false;
        const element = narration ? 6 : 4;
        const full = v?.values[element]?.text;
        const count = v?.reveals?.[element];
        return {
          speaker: narration ? "" : v?.values[1]?.text,
          text: v?.values[narration ? 5 : 3]?.text,
          display:
            count === undefined
              ? full
              : Array.from(full || "")
                  .slice(0, count)
                  .join(""),
          revealCount: count,
          prompt: v?.values[11]?.text,
          enabled: v?.values[11]?.enabled,
          narration,
          status: v?.values[12]?.text,
        };
      };
      globalThis.trainAudio = {
        blocks: 0,
        peak: 0,
        sounds: 0,
        frames: 0,
        maxZeroRun: 0,
        maxStableZeroRun: 0,
        windows: [],
        gaps: [],
        stableGaps: [],
      };
      const OriginalContext = globalThis.AudioContext;
      globalThis.AudioContext = new Proxy(OriginalContext, {
        construct(target, args) {
          const context = Reflect.construct(target, args);
          const addModule = context.audioWorklet.addModule.bind(context.audioWorklet);
          context.audioWorklet.addModule = async (url, ...options) => {
            const source = await (await fetch(url)).text();
            if (!source.includes("class OtoWorkletProcessor")) throw new Error("unexpected Oto worklet");
            // Observe the original output after mixing, without changing samples.
            const audit = `
              const trainProcess = OtoWorkletProcessor.prototype.process;
              OtoWorkletProcessor.prototype.process = function(inputs, outputs, parameters) {
                const result = trainProcess.call(this, inputs, outputs, parameters);
                this.trainPeak_ ||= 0;
                this.trainFrames_ = (this.trainFrames_ || 0) + outputs[0][0].length;
                for (let i = 0; i < outputs[0][0].length; i++) {
                  let zero = true;
                  for (const channel of outputs[0]) {
                    this.trainPeak_ = Math.max(this.trainPeak_, Math.abs(channel[i]));
                    if (channel[i] !== 0) zero = false;
                  }
                  if (!zero) this.trainHeard_ = true;
                  this.trainZero_ = zero && this.trainHeard_ ? (this.trainZero_ || 0) + 1 : 0;
                  if (this.trainZero_ === 4800) this.port.postMessage({trainAudioGap: true, frame: this.trainFrames_ - outputs[0][0].length + i - 4800});
                  this.trainMaxZero_ = Math.max(this.trainMaxZero_ || 0, this.trainZero_);
                  const frame = this.trainFrames_ - outputs[0][0].length + i;
                  if (frame >= 48000 * 3) {
                    this.trainStableZero_ = zero ? (this.trainStableZero_ || 0) + 1 : 0;
                    this.trainStableMax_ = Math.max(this.trainStableMax_ || 0, this.trainStableZero_);
                    if (this.trainStableZero_ === 4800) this.port.postMessage({trainAudioStableGap: true, frame: frame - 4800});
                  }
                }
                if (((this.trainBlocks_ = (this.trainBlocks_ || 0) + 1) % 64) === 0) {
                  this.port.postMessage({trainAudioAudit: true, peak: this.trainPeak_, frames: this.trainFrames_, maxZeroRun: this.trainMaxZero_ || 0, maxStableZeroRun: this.trainStableMax_ || 0});
                  this.trainPeak_ = 0;
                }
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
      globalThis.AudioWorkletNode = new Proxy(globalThis.AudioWorkletNode, {
        construct(target, args) {
          const node = Reflect.construct(target, args);
          node.port.addEventListener("message", (event) => {
            if (event.data?.trainAudioStableGap) {
              trainAudio.stableGaps.push(event.data.frame / 48000);
              event.stopImmediatePropagation();
            }
            if (event.data?.trainAudioGap) {
              trainAudio.gaps.push(event.data.frame / 48000);
              event.stopImmediatePropagation();
            }
            if (event.data?.trainAudioAudit) {
              trainAudio.blocks += 64;
              trainAudio.peak = Math.max(trainAudio.peak, event.data.peak);
              trainAudio.frames = event.data.frames;
              trainAudio.maxZeroRun = event.data.maxZeroRun;
              trainAudio.maxStableZeroRun = event.data.maxStableZeroRun;
              trainAudio.windows.push({ frames: event.data.frames, peak: event.data.peak });
              event.stopImmediatePropagation();
            }
          });
          node.port.start();
          return node;
        },
      });
      let playSound;
      Object.defineProperty(globalThis, "kartyHostPlaySound", {
        configurable: true,
        get: () => playSound,
        set(original) {
          playSound = (...args) => {
            trainAudio.sounds++;
            return original(...args);
          };
        },
      });
      let submit;
      const remove = (id) => {
        for (const [child, value] of Object.entries(trainProbe.instances))
          if (value.parent === id) remove(Number(child));
        delete trainProbe.instances[id];
      };
      Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
        configurable: true,
        get: () => submit,
        set(original) {
          submit = (bytes) => {
            const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
            const text = (at, length) => new TextDecoder().decode(bytes.subarray(at, at + length));
            for (let at = 24; at < bytes.length;) {
              const tag = view.getUint8(at),
                size = view.getUint16(at + 1, true),
                p = at + 3;
              const id = view.getUint32(p, true);
              if (tag === 1) {
                const n = view.getUint16(p + 4, true);
                trainProbe.sprites[id] = {
                  id,
                  asset: text(p + 6, n),
                  x: view.getFloat32(p + 6 + n, true),
                  y: view.getFloat32(p + 10 + n, true),
                  rotation: 0,
                  scaleX: 1,
                  scaleY: 1,
                  alpha: 1,
                };
              }
              if (tag === 12)
                trainProbe.requests.push({
                  id,
                  name: text(p + 6, view.getUint16(p + 4, true)),
                  frame: trainProbe.frame,
                });
              if (tag === 16)
                trainProbe.sprites[id] = {
                  id,
                  asset: trainProbe.textures[view.getUint32(p + 4, true) + ":" + view.getUint32(p + 8, true)],
                  x: view.getFloat32(p + 12, true),
                  y: view.getFloat32(p + 16, true),
                  rotation: 0,
                  scaleX: 1,
                  scaleY: 1,
                  alpha: 1,
                };
              if (tag === 2) delete trainProbe.sprites[id];
              const sprite = trainProbe.sprites[id];
              if (sprite && tag === 3) {
                for (const [n, key] of ["x", "y", "rotation", "scaleX", "scaleY"].entries())
                  sprite[key] = view.getFloat32(p + 4 + n * 4, true);
              }
              if (sprite && tag === 4) {
                sprite.x += view.getFloat32(p + 4, true);
                sprite.y += view.getFloat32(p + 8, true);
              }
              if (sprite && tag === 5) sprite.asset = text(p + 6, view.getUint16(p + 4, true));
              if (sprite && tag === 17)
                sprite.asset = trainProbe.textures[view.getUint32(p + 4, true) + ":" + view.getUint32(p + 8, true)];
              if (sprite && tag === 6) sprite.alpha = view.getFloat32(p + 4, true);
              if (tag === 18 || tag === 23) {
                const offset = tag === 18 ? 8 : 20;
                trainProbe.instances[view.getUint32(p, true)] = {
                  name: text(p + offset + 2, view.getUint16(p + offset, true)),
                  parent: tag === 23 ? view.getUint32(p + 4, true) : 0,
                  slot: tag === 23 ? view.getUint32(p + 8, true) : 0,
                  handle: tag === 23 ? view.getUint32(p + 16, true) : view.getUint32(p + 4, true),
                  values: {},
                  reveals: {},
                  visible: {},
                };
              }
              const instance = trainProbe.instances[view.getUint32(p, true)];
              if (tag === 20 && instance)
                instance.values[view.getUint32(p + 4, true)] = {
                  text: text(p + 11, view.getUint16(p + 9, true)),
                  enabled: view.getUint8(p + 8) === 1,
                };
              if (tag === 59 && instance) {
                const element = view.getUint32(p + 4, true),
                  count = view.getUint32(p + 8, true);
                instance.reveals[element] = count;
                if (trainProbe.revealUpdates.length < 4096)
                  trainProbe.revealUpdates.push({ element, count, text: instance.values[element]?.text });
              }
              if (tag === 26 && instance) instance.visible[view.getUint32(p + 4, true)] = view.getUint8(p + 8) === 1;
              if (tag === 19 || tag === 25) remove(view.getUint32(p, true));
              at = p + size;
            }
            return original(bytes);
          };
        },
      });
    });
    try {
      await run(page);
    } catch (error) {
      console.error("Last Train browser console:", messages);
      console.error("Last Train state at failure:", await state(page).catch(() => null));
      await page.screenshot({ path: resolve(output, "failure.png"), fullPage: true, timeout: 9000 }).catch(() => {});
      throw error;
    }
    assert.deepEqual(report.errors, []);
  } finally {
    clearTimeout(deadline);
    await browser.close();
  }
}
async function canvasImage(page) {
  return page.evaluate(
    () =>
      new Promise((done) => requestAnimationFrame(() => done(document.querySelector("canvas").toDataURL("image/png")))),
  );
}
async function open(page) {
  await page.goto(url + "/?debug=0");
  await ready(page);
  await begin(page);
}
async function ready(page) {
  await page.waitForFunction(
    () =>
      document.documentElement.dataset.kartyState === "running" &&
      Object.values(trainProbe.instances).some(
        (v) =>
          (v.name === "dialogue.dialogue" && trainDialogue().text) ||
          (v.name === "ui.game-screen" && v.visible[6] === false),
      ),
  );
  await page.waitForTimeout(150);
}
async function state(page) {
  return page.evaluate(() => {
    const dialogue = trainDialogue(),
      instances = Object.values(trainProbe.instances),
      root = instances.find((v) => v.name === "ui.game-screen");
    return {
      speaker: dialogue.speaker,
      text: dialogue.text,
      display: dialogue.display,
      narration: dialogue.narration,
      revealCount: dialogue.revealCount,
      prompt: dialogue.prompt,
      revealing: dialogue.prompt === "»",
      paused: dialogue.enabled === false,
      title: root?.visible[6] === false,
      cover: root?.visible[31] === true,
      status: dialogue.status || root?.values[29]?.text,
      choices: instances
        .filter((v) => v.name === "dialogue.choice")
        .map((v) => v.values[1])
        .filter(Boolean),
    };
  });
}
async function status(page, text) {
  await page.waitForFunction((text) => {
    const instances = Object.values(trainProbe.instances);
    if (instances.some((v) => v.name === "dialogue.dialogue")) return trainDialogue().status?.includes(text);
    return instances.some((v) => v.name === "ui.game-screen" && v.values[29]?.text.includes(text));
  }, text);
}
async function advance(page) {
  const previous = await state(page);
  assert.equal(previous.choices.length, 0);
  if (previous.revealing) await clickDialogue(page, 0);
  await page.waitForFunction(() =>
    Object.values(trainProbe.instances).some((v) => v.name === "dialogue.dialogue" && trainDialogue().enabled),
  );
  await clickDialogue(page, 0);
  // A pause or chapter curtain keeps the old text mounted while input is
  // disabled. Wait for the resulting story state before clicking the toolbar.
  await page.waitForFunction((text) => {
    const dialogue = trainDialogue();
    const choices = Object.values(trainProbe.instances).some(
      (v) => v.name === "dialogue.choice" && v.values[1]?.enabled,
    );
    return (
      (dialogue.enabled && dialogue.text && dialogue.text !== text) ||
      choices ||
      dialogue.status?.includes("End of the line")
    );
  }, previous.text);
}

async function advanceTo(page, text) {
  for (let n = 0; n < 48; n++) {
    const current = await state(page);
    if (current.text.includes(text)) return;
    assert.equal(current.choices.length, 0, `unexpected choices before ${text}`);
    await advance(page);
  }
  throw new Error(`missing story line: ${text}`);
}
async function advanceToChoices(page) {
  for (let n = 0; n < 32; n++) {
    if ((await state(page)).choices.length) return;
    await advance(page);
  }
  throw new Error("no choices reached");
}
async function choose(page, index) {
  const current = await state(page);
  assert.ok(current.choices[index]?.enabled, `unavailable choice ${index}: ${JSON.stringify(current)}`);
  await clickDialogue(page, index);
  await page.waitForFunction(
    (text) =>
      Object.values(trainProbe.instances).some(
        (v) => v.name === "dialogue.dialogue" && trainDialogue().text && trainDialogue().text !== text,
      ),
    current.text,
  );
}
async function clickDialogue(page, index) {
  const current = await state(page);
  // Desktop copy fits the same minimum-height text area for short lines. Cache
  // those observed framebuffer positions; portrait wrapping is measured anew.
  const key =
    !page.trainTouch && current.text?.length < 180
      ? `${current.narration}:${current.choices.length}:${Boolean(current.status)}`
      : null;
  page.trainButtons ||= new Map();
  let bands = key && page.trainButtons.get(key);
  if (!bands) {
    bands = await buttonBands(page);
    if (key) page.trainButtons.set(key, bands);
  }
  assert.ok(bands[index], `missing canvas button ${index}: ${JSON.stringify(bands)}`);
  if (current.revealing && !current.choices.length) {
    const now = await state(page);
    assert.equal(now.text, current.text, "line changed while locating the reveal control");
    if (!now.revealing) return;
    // A release-triggered canvas click takes 150 ms. Let an almost complete
    // line finish naturally instead of accidentally advancing it on release.
    if (now.text.length - now.display.length < 14) {
      await page.waitForFunction(() => trainDialogue().display === trainDialogue().text);
      return;
    }
  }
  await click(page, bands[index].x, bands[index].y);
}
async function click(page, x, y) {
  if (page.trainTouch) {
    await page.trainTouchSession.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [{ id: 1, x, y }],
    });
    await page.waitForTimeout(150);
    await page.trainTouchSession.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  } else {
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.waitForTimeout(150);
    await page.mouse.up();
  }
  await page.waitForTimeout(200);
  if (!page.trainTouch) {
    await page.mouse.move(5, 200);
    await page.evaluate(
      () =>
        new Promise((done) => {
          let n = 4;
          const step = () => (--n ? requestAnimationFrame(step) : done());
          requestAnimationFrame(step);
        }),
    );
  }
}
async function profileLabel(page, text) {
  await page.waitForFunction(
    (expected) =>
      Object.values(trainProbe.instances).some((v) => Object.values(v.values).some((value) => value.text === expected)),
    text,
  );
}
async function selectSlot(page, slot) {
  // Slot cycles 1 -> 2 -> 3 -> 1 in the title panel.
  await click(page, 150, 115);
  await status(page, `Slot ${slot} selected`);
}
async function begin(page) {
  if (!(await state(page)).title) return;
  await openingButton(page, true);
  await page.waitForFunction(() =>
    Object.values(trainProbe.instances).some((v) => v.name === "dialogue.dialogue" && trainDialogue().text),
  );
}
async function openingButton(page, primary, skip = 0) {
  const deadline = Date.now() + 9000;
  let point;
  while (!point && Date.now() < deadline) {
    const bytes = await page.screenshot({ fullPage: true, timeout: Math.max(1, deadline - Date.now()) });
    point = await page.evaluate(
      async ({ encoded, primary, skip }) => {
        const bitmap = await createImageBitmap(await (await fetch("data:image/png;base64," + encoded)).blob());
        const canvas = new OffscreenCanvas(bitmap.width, bitmap.height),
          ctx = canvas.getContext("2d");
        ctx.drawImage(bitmap, 0, 0);
        const data = ctx.getImageData(0, 0, bitmap.width, bitmap.height).data;
        const colors = primary
          ? [
              [100, 80, 57],
              [97, 76, 49],
            ]
          : [[23, 37, 54]];
        let start = -1;
        for (let y = 0; y <= bitmap.height; y++) {
          let matches = 0;
          if (y < bitmap.height)
            for (let x = Math.floor(bitmap.width * 0.1); x < bitmap.width * 0.9; x += 3) {
              const at = (y * bitmap.width + x) * 4;
              if (colors.some((color) => color.every((v, c) => Math.abs(data[at + c] - v) < 3))) matches++;
            }
          const match = matches > Math.min(55, bitmap.width * 0.1);
          if (match && start < 0) start = y;
          if (!match && start >= 0) {
            if (y - start >= 30) {
              if (skip-- === 0) return { x: bitmap.width / 2, y: (y + start) / 2 };
            }
            start = -1;
          }
        }
        return null;
      },
      { encoded: bytes.toString("base64"), primary, skip },
    );
    if (!point) await page.waitForTimeout(100);
  }
  assert.ok(point, "missing opening button");
  await click(page, point.x, point.y);
}
async function toolbar(page, index) {
  if (index === 1 && (await state(page)).title) {
    await openingButton(page, false);
    return;
  }
  const { width } = page.viewportSize();
  // Consumer toolbar: six equal minimum-width buttons in one horizontal row.
  await click(page, width === 390 ? 37 + index * 62.5 : 849 + index * 70.5, width === 390 ? 165 : 49);
}
async function capture(page, name, complete = true) {
  if (complete && (await state(page)).revealing) await clickDialogue(page, 0);
  await page.mouse.move(5, 200);
  const bytes = await page.screenshot({ fullPage: true, timeout: 9000 });
  await writeFile(resolve(output, name + ".png"), bytes);
  report.captures.push({ name, sha256: digest(bytes) });
  if (name === "mara" && process.argv.includes("--capture-preview")) {
    const viewport = page.viewportSize();
    await page.setViewportSize({ width: 960, height: 540 });
    await page.waitForTimeout(200);
    await page.screenshot({
      path: resolve("samples/last-train/preview.jpg"),
      type: "jpeg",
      quality: 85,
      fullPage: true,
      timeout: 9000,
    });
    await page.setViewportSize(viewport);
    await page.waitForTimeout(200);
  }
}
async function buttonBands(page) {
  const bytes = await page.screenshot({ fullPage: true, timeout: 9000 });
  return page.evaluate(async (encoded) => {
    const bitmap = await createImageBitmap(await (await fetch("data:image/png;base64," + encoded)).blob());
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height),
      ctx = canvas.getContext("2d");
    ctx.drawImage(bitmap, 0, 0);
    const pixels = ctx.getImageData(0, 0, bitmap.width, bitmap.height).data;
    const colors = [
      [23, 37, 54],
      [25, 40, 57],
      [16, 28, 41],
      [97, 76, 49],
      [76, 60, 42],
    ];
    // Choices span the dialogue; the advance indicator is a separate 48 px target.
    for (const compact of [false, true]) {
      const bands = [];
      let start = -1;
      const narrow = bitmap.width < 600;
      for (let y = Math.floor(bitmap.height * 0.3); y <= bitmap.height; y++) {
        let matches = 0;
        const left = compact ? bitmap.width - (narrow ? 35 : 60) : narrow ? 26 : Math.floor(bitmap.width * 0.15);
        const right = compact ? left + 3 : narrow ? 31 : bitmap.width * 0.8;
        if (y < bitmap.height)
          for (let x = left; x < right; x += compact || narrow ? 1 : 3) {
            const at = (y * bitmap.width + x) * 4;
            if (colors.some((color) => color.every((v, c) => Math.abs(pixels[at + c] - v) < 3))) matches++;
          }
        const button = compact ? matches >= 2 : narrow ? matches >= 3 : matches > bitmap.width * 0.15;
        if (button && start < 0) start = y;
        if (!button && start >= 0) {
          if (y - start >= 20)
            bands.push({ x: compact ? bitmap.width - (narrow ? 46 : 76) : bitmap.width * 0.5, y: (y + start) * 0.5 });
          start = -1;
        }
      }
      if (bands.length) return bands;
    }
    return [];
  }, bytes.toString("base64"));
}

async function motion(page) {
  assert.equal((await state(page)).title, true);
  await openingButton(page, false, 2);
  await page.waitForFunction(() =>
    Object.values(trainProbe.instances).some(
      (v) => v.name === "ui.game-screen" && Object.values(v.values).some((value) => value.text === "Motion: Reduced"),
    ),
  );
}
