// Focused visual direction checks, using the ordinary cartridge/control fixture.
// Loaded by check-last-train-web.mjs --direction-only; never injects game state.
import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import { resolve } from "node:path";

export async function runDirectionChecks(context) {
  const { fixture, open, state, advance, advanceTo, advanceToChoices, choose, toolbar, status, capture, report } =
    context;
  const phone = process.argv.includes("--phone-only");
  const prefix = phone ? "direction-phone" : "direction";

  await fixture(async (page) => {
    await open(page);
    const sceneClick = () => context.click(page, phone ? 150 : 500, phone ? 330 : 300);
    const partial = await state(page);
    assert.equal(partial.revealing, true, "opening should still be revealing before scene click");
    await sceneClick();
    const revealed = await state(page);
    assert.equal(revealed.text, partial.text, "scene click skipped the revealing line");
    assert.equal(revealed.display, revealed.text, "scene click did not finish the line");
    assert.equal(revealed.revealing, false);
    await sceneClick();
    const progressed = await state(page);
    assert.notEqual(progressed.text, partial.text, "second scene click did not advance");
    assert.ok(progressed.text.startsWith("Westbridge, eleven forty-seven."), "scene click advanced more than once");
    await advance(page);
    assert.ok(
      (await state(page)).text.startsWith("In your coat pocket:"),
      "dialogue icon also triggered the scene target",
    );
    report.checks.push(
      `${prefix}: scene click/tap reveals once, advances one line, and does not double-handle the dialogue icon`,
    );
    await capture(page, `${prefix}-rain`);
    const rain = phone ? null : await visibleMotion(page);
    if (rain) {
      assert.ok(rain.changedPixels > 40, "platform environment did not visibly animate");
      assert.equal(rain.backgroundTransforms, 0, "rain motion moved the station architecture");
      report.checks.push(
        `${prefix}: rendered rain changes (${rain.changedPixels} pixels) while station architecture stays fixed`,
      );
    } else {
      report.checks.push(
        `${prefix}: responsive phone platform composition (continuous ambience uses desktop sprite stage only)`,
      );
    }

    let entrance;
    if (phone) {
      await advanceTo(page, "You have barely settled into your seat");
      entrance = await recordPhoneEntrance(page, () => advance(page));
      console.log("Last Train phone entrance pixels:", entrance);
      assert.ok(entrance.firstToMiddle > 30, "phone portrait did not move during its entrance");
      assert.ok(entrance.middleToLast > 30, "phone portrait jumped instead of completing its entrance");
      report.checks.push(`${prefix}: responsive portrait visibly progresses through its entrance`);
    }
    await advanceTo(page, "My name is Mara.");
    await capture(page, `${prefix}-conductor`);
    const medium = await character(page, "mara");
    if (phone) await phoneStage(page, { frame: "medium", images: [1], reduced: false });
    assert.match(medium.asset, /worried/, "the conductor's guarded expression was not rendered");
    await settle(page);
    const actorIDs = phone ? await actorInstances(page) : null;
    await advanceTo(page, "Her thumb finds the small key");
    await capture(page, `${prefix}-key-steady-shot`);
    await settle(page);
    const close = await character(page, "mara");
    if (phone) await phoneStage(page, { frame: "medium", images: [1], reduced: false });
    assert.ok(Math.abs(close.scaleX / medium.scaleX - 1) < 0.01, "same character received a zoom transition");
    assert.equal(close.id, medium.id, "same character was recreated");
    assert.equal(close.alpha, 1, "same character replayed its entrance");
    if (phone) assert.deepEqual(await actorInstances(page), actorIDs, "same character remounted its phone portrait");
    report.checks.push(`${prefix}: same character keeps its shot and portrait instance without a repeated entrance`);

    // A checkpoint carries persistent composition and expression, never the
    // momentary progress of a nod or a rain streak.
    await toolbar(page, 0);
    await status(page, "Checkpoint saved");
    const savedText = (await state(page)).text;
    await advanceTo(page, "Rain threads the glass");
    await toolbar(page, 1);
    await status(page, "Checkpoint restored");
    assert.equal((await state(page)).text, savedText);
    await settle(page);
    const restored = await character(page, "mara");
    if (phone) await phoneStage(page, { frame: "medium", images: [1], reduced: false });
    assert.equal(restored.asset, close.asset);
    assert.ok(Math.abs(restored.scaleX - close.scaleX) < 0.01, "loaded run lost its steady framing");
    await capture(page, `${prefix}-restored-direction`);
    report.checks.push(`${prefix}: save/load restores expression and camera composition`);

    // The optional motion control changes a lifetime preference. It must stop
    // ambient movement while keeping the same story and emotional expression.
    await toolbar(page, 5);
    await context.motion(page);
    await toolbar(page, 1);
    await status(page, "Checkpoint restored");
    await settle(page);
    const still = await visibleMotion(page);
    if (phone) await phoneStage(page, { frame: "medium", images: [1], reduced: true });
    assert.equal(still.transformedEntities, 0, "reduced motion still emits changing sprite transforms");
    assert.equal(still.changedPixels, 0, "reduced motion still moves the rendered scene");
    assert.equal((await character(page, "mara")).asset, close.asset);
    assert.equal((await state(page)).text, savedText);
    report.checks.push(`${prefix}: reduced motion settles the scene without changing its story or expression`);
    report.direction ||= {};
    report.direction[phone ? "phone" : "desktop"] = { rain, entrance, medium, close, restored, reduced: still };
  }, phone);

  // Keep the dining route in a separate browser lifetime, with the same strict
  // 120-second deadline as the shared fixture on software graphics.
  await fixture(async (page) => {
    await open(page);
    await advanceTo(page, "Rain threads the glass");
    await advance(page);
    await choose(page, 3);
    await advanceTo(page, "You bring them together");
    await capture(page, `${prefix}-dining-tableau`);
    await settle(page);
    const tableau = await Promise.all(["mara", "elias", "iris"].map((name) => character(page, name)));
    if (phone) await phoneStage(page, { ensemble: true, images: [1, 6, 9] });
    assert.equal(new Set(tableau.map((v) => v.id)).size, 3, "dining does not show all three passengers");
    assert.equal(new Set(tableau.map((v) => Math.round(v.x))).size, 3, "dining passengers overlap at one position");
    await advanceToChoices(page);
    const choicesBefore = await state(page);
    await context.click(page, phone ? 150 : 500, phone ? 330 : 300);
    const choicesAfter = await state(page);
    assert.equal(choicesAfter.text, choicesBefore.text);
    assert.deepEqual(choicesAfter.choices, choicesBefore.choices, "scene click bypassed a choice");
    report.checks.push(`${prefix}: scene click/tap leaves choices awaiting an explicit selection`);
    await choose(page, 2);
    await advanceTo(page, "We have a schedule to keep");
    await capture(page, `${prefix}-timetable-reaction`);
    assert.match((await character(page, "mara")).asset, /worried/);
    if (phone) await phoneStage(page, { frame: "medium", images: [1], reduced: false });
    await advanceTo(page, "There is always next Thursday");
    await capture(page, `${prefix}-elias-reaction`);
    assert.match((await character(page, "elias")).asset, /wistful/);
    if (phone) await phoneStage(page, { frame: "medium", images: [4], reduced: false });
    report.checks.push(`${prefix}: three-person dining tableau and distinct ending reactions render`);
    report.direction[phone ? "phone" : "desktop"].tableau = tableau;
  }, phone);
}

async function settle(page) {
  await page.waitForTimeout(900);
}

async function character(page, name) {
  const sprites = await page.evaluate(() => Object.values(trainProbe.sprites));
  const result = sprites.find((v) => v.asset.includes(name) && v.alpha > 0.1);
  assert.ok(result, `${name} missing from visible sprite stage: ${JSON.stringify(sprites)}`);
  return result;
}

async function visibleMotion(page) {
  const before = await sample(page);
  await page.waitForTimeout(400);
  const after = await sample(page);
  const transformedEntities = Object.entries(after.sprites).filter(([id, value]) => {
    const old = before.sprites[id];
    return (
      old && ["x", "y", "rotation", "scaleX", "scaleY", "alpha"].some((key) => Math.abs(value[key] - old[key]) > 0.0001)
    );
  }).length;
  const backgrounds = Object.entries(before.sprites).filter(([, value]) =>
    /train\.(platform|carriage|dining)$/.test(value.asset),
  );
  assert.ok(backgrounds.length > 0, "no architectural background was sampled");
  let backgroundTransforms = 0;
  for (const [id, value] of backgrounds) {
    const current = after.sprites[id];
    assert.ok(current && current.asset === value.asset, "background changed owner during an ambient sample");
    for (const key of ["x", "y", "rotation", "scaleX", "scaleY"])
      if (current[key] !== value[key]) backgroundTransforms++;
  }
  assert.equal(backgroundTransforms, 0, "painted architecture translated, rotated or zoomed during an ambient sample");
  const changedPixels = await imageDifference(page, before.image, after.image);
  return { changedPixels, transformedEntities, backgroundTransforms };
}

async function imageDifference(page, a, b) {
  return page.evaluate(
    async ({ a, b }) => {
      async function pixels(url) {
        const bitmap = await createImageBitmap(await (await fetch(url)).blob());
        const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
        const ctx = canvas.getContext("2d");
        ctx.drawImage(bitmap, 0, 0);
        return {
          width: bitmap.width,
          height: bitmap.height,
          pixels: ctx.getImageData(0, 0, bitmap.width, bitmap.height).data,
        };
      }
      const first = await pixels(a),
        second = await pixels(b);
      let count = 0;
      // Exclude the complete lower dialogue and top toolbar. The remaining pixels
      // belong to artwork, rather than a typing cursor or transient button hover.
      for (let y = Math.floor(first.height * 0.27); y < first.height * 0.48; y++)
        for (let x = 10; x < first.width - 10; x++) {
          const at = (y * first.width + x) * 4;
          if ([0, 1, 2].some((c) => Math.abs(first.pixels[at + c] - second.pixels[at + c]) > 4)) count++;
        }
      return count;
    },
    { a, b },
  );
}

async function sample(page) {
  return page.evaluate(
    () =>
      new Promise((done) =>
        requestAnimationFrame(() =>
          done({
            image: document.querySelector("canvas").toDataURL("image/png"),
            sprites: structuredClone(trainProbe.sprites),
          }),
        ),
      ),
  );
}

async function recordPhoneEntrance(page, trigger) {
  await page.evaluate(() => {
    globalThis.trainEntrance = { images: [], complete: false };
    let began;
    function frame(now) {
      const active = trainDialogue().text?.startsWith("Forgive me.");
      if (active) {
        began ??= now;
        const elapsed = now - began;
        const next = [0, 100, 200, 300, 400, 500, 650, 800, 1000][trainEntrance.images.length];
        if (elapsed >= next) trainEntrance.images.push(document.querySelector("canvas").toDataURL("image/png"));
        if (trainEntrance.images.length === 9) {
          trainEntrance.complete = true;
          return;
        }
      }
      requestAnimationFrame(frame);
    }
    requestAnimationFrame(frame);
  });
  await trigger();
  await page.waitForFunction(() => trainEntrance.complete);
  const images = await page.evaluate(() => trainEntrance.images);
  const output = resolve(process.env.KARTY_LAST_TRAIN_OUTPUT || "dist/validation/last-train/browser");
  for (let n = 0; n < images.length; n++)
    await writeFile(resolve(output, `phone-entrance-${n}.png`), Buffer.from(images[n].split(",")[1], "base64"));
  const frameDiffs = [];
  for (let n = 1; n < images.length; n++) frameDiffs.push(await imageDifference(page, images[n - 1], images[n]));
  // Protocol notifications arrive before their frame is painted. Use the first
  // two actual visual changes, rather than guessing which clock sample lands
  // inside the entrance. A single instant replacement still fails this check.
  const significant = frameDiffs.filter((count) => count > 30);
  return {
    firstToMiddle: significant[0] || 0,
    middleToLast: significant[1] || 0,
    frameDiffs,
  };
}

// These IDs come from the generated clients for the handwritten TrainStage,
// TrainActor and TrainPortrait components. Child components are removed when
// their conditional branch is inactive, so only live portrait images count.
// Initial visibility is true; the SDK emits a record only when it changes.
async function phoneStage(page, { frame, ensemble = false, images, reduced }) {
  const rendered = await page.evaluate(() => {
    const instances = Object.values(trainProbe.instances);
    return {
      stage: instances.find((v) => v.name === "ui.train-stage")?.visible,
      actors: instances.filter((v) => v.name === "ui.train-actor").map((v) => v.visible),
      images: instances
        .filter((v) => v.name.startsWith("art.") && trainProbe.instances[v.parent]?.name === "ui.train-portrait")
        .map((v) => v.slot)
        .sort((a, b) => a - b),
    };
  });
  assert.ok(rendered.stage, "responsive phone stage was not mounted");
  for (const id of [4, 6, 8]) assert.equal(rendered.stage[id] !== false, ensemble, `phone ensemble column ${id}`);
  if (frame) assert.equal(rendered.stage[10] !== false, !ensemble, "phone stable single-character container");
  assert.deepEqual(rendered.images, images, "phone expression images differ from the authored stage");
  if (reduced !== undefined) {
    assert.equal(rendered.actors.length, 1);
    const actor = rendered.actors[0];
    const still = [1, 3, 5].some((id) => actor[id] !== false);
    const animated = [7, 9, 11].some((id) => actor[id] !== false);
    assert.equal(still, reduced, "phone still portrait branch");
    assert.equal(animated, !reduced, "phone animated portrait branch");
  }
  return rendered;
}

async function actorInstances(page) {
  return page.evaluate(() =>
    Object.entries(trainProbe.instances)
      .filter(([, value]) => ["ui.train-actor", "ui.train-portrait"].includes(value.name))
      .map(([id]) => id)
      .sort(),
  );
}
