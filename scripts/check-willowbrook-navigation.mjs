import assert from "node:assert/strict";

export async function checkNavigation({ game, page, state, frames, press, clickWorld, arrive, capture, report }) {
  const initial = await state();
  assert.equal(initial.worldConfigurations, 1, "the village must configure its engine navigation world");
  assert.ok(
    initial.worldObstacles > 10 && initial.worldObstacles <= 128,
    "authored colliders were not registered within the host bound",
  );
  const routedClick = async (target) => {
    await frames(30);
    const before = await state();
    await clickWorld(target);
    await game.waitForFunction((count) => villageEvidence.pathQueries.length > count, before.pathQueries.length, {
      timeout: 3000,
    });
    const query = (await state()).pathQueries.at(-1);
    await game.waitForFunction(
      (request) => villageEvidence.pathResults.some((result) => result.request === request),
      query.request,
      { timeout: 15000 },
    );
    const result = (await state()).pathResults.find((result) => result.request === query.request);
    assert.equal(result.status, 1, `engine path request failed: ${JSON.stringify({ target, result })}`);
    assert.ok(result.expanded <= 4096 && result.points.length >= 1 && result.points.length <= 128);
    const end = result.points.at(-1);
    assert.ok(
      Math.hypot(end[0] - query.destination[0], end[1] - query.destination[1]) < 0.0001,
      "host silently changed the requested destination",
    );
    return { before, destination: (await state()).pointerDowns.at(-1) };
  };
  const route = async (name, target) => {
    const { before, destination } = await routedClick(target);
    await arrive(destination);
    await frames(8);
    const after = await state();
    const samples = after.collisionPath.filter((point) => point.frame > before.frame);
    assert.ok(samples.length > 10, "missing real accepted movement samples");
    (report.routes ??= []).push({ name, from: before.position, requested: target, target: destination, samples });
    await capture("navigation-" + name);
    return { before, samples };
  };
  await page.keyboard.down("Shift");
  const house = await route("around-cottage", { x: 1160, y: 1440 });
  const dx = 1160 - house.before.position[0],
    dy = 1440 - house.before.position[1];
  assert.ok(
    house.samples.some(
      (point) =>
        Math.abs(dx * (point.y - house.before.position[1]) - dy * (point.x - house.before.position[0])) /
          Math.hypot(dx, dy) >
        20,
    ),
    "cottage route did not detour around the wall",
  );
  report.checks.push(
    "ordinary ground click requests engine A* and reaches the far side of the cottage by a real detour",
  );
  await press("r", 1);
  const bridge = await route("across-bridge", { x: 2250, y: 1380 });
  const crossing = bridge.samples.filter((point) => point.x > 2062 && point.x < 2162);
  assert.ok(
    crossing.length > 10 && crossing.every((point) => point.y >= 1439.5 && point.y <= 1474.5),
    "route crossed water instead of the visible bridge deck",
  );
  const invalid = await state();
  await clickWorld({ x: 2112, y: 1376 });
  await frames(40);
  assert.ok(
    Math.hypot((await state()).position[0] - invalid.position[0], (await state()).position[1] - invalid.position[1]) <
      0.1,
    "blocked water click moved the player",
  );
  await route("return-over-bridge", { x: 1936, y: 1380 });
  report.checks.push("ordinary clicks cross the bridge in both directions and reject a destination inside the river");
  await press("r", 1);
  await route("upstairs", { x: 1728, y: 1128 });
  report.checks.push("ordinary click reaches the narrow upper stair landing through engine navigation");
  await press("r", 1);
  await routedClick({ x: 1160, y: 1440 });
  await frames(10);
  // The camera keeps following during a replacement tap. Assert the actual
  // click destination sent to the engine, rather than a prior screen projection.
  const replacement = await routedClick({ x: 1680, y: 1360 });
  await arrive(replacement.destination);
  await frames(40);
  assert.ok(
    Math.hypot(
      (await state()).position[0] - replacement.destination.x,
      (await state()).position[1] - replacement.destination.y,
    ) < 2,
    "superseded route resumed",
  );
  await press("r", 1);
  await route("wall-approach", { x: 1840, y: 1240 });
  await press("w", 20);
  const contact = (await state()).position;
  assert.ok(contact[1] >= 1226 && contact[1] < 1228, "keyboard did not reach the physical cliff wall");
  await press("w", 10);
  assert.ok(
    Math.hypot((await state()).position[0] - contact[0], (await state()).position[1] - contact[1]) < 0.1,
    "wall-contact fixture did not stop against the physical collider",
  );
  await route("from-wall-contact", { x: 1936, y: 1226.5 });
  assert.ok(
    (await state()).position[1] < 1227.5,
    "route stopped at the planner connector instead of the clicked ground",
  );
  report.checks.push(
    "click navigation escapes physical wall contact and follows collision-safe connectors back to the clicked ground",
  );
  await page.keyboard.up("Shift");
  await frames(4);
  await press("r", 1);
  await page.setViewportSize({ width: 390, height: 844 });
  await route("portrait-touch", { x: 1760, y: 1410 });
  report.checks.push("a new click supersedes the old route; portrait touch reaches its exact ground destination");
}
