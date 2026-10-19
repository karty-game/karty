import assert from "node:assert/strict";

// Observe the actual guest lifecycle, not a separately initialized map fixture.
export function assertQuestAnchors(state, stations, sceneryLayer) {
  const sprites = Object.values(state.questSprites);
  assert.equal(sprites.length, 3, "the loaded village must create two signs and a chest");
  assert.ok(sceneryLayer >= 0, "authored scenery layer is missing");
  for (const [index, station] of stations.entries()) {
    const height = index === 2 ? 48 : 64;
    const sprite = sprites.find(
      (sprite) => Math.abs(sprite.x - station.x) < 0.01 && Math.abs(sprite.y + height / 2 - station.y) < 0.01,
    );
    assert.ok(
      sprite,
      `${station.name} did not spawn at its authored anchor (${station.x}, ${station.y}): ${JSON.stringify(sprites)}`,
    );
    assert.equal(sprite.name, index === 2 ? "treasure-closed" : "clue-note");
    assert.equal(sprite.layer, sceneryLayer * 100000 + Math.trunc(station.y * 2));
  }
}

// Compare opaque artwork pixels with the real framebuffer at its projected
// location. Protocol records alone cannot establish that an object is visible.
export async function checkQuestPixels(game, station, index, source) {
  const result = await game.evaluate(
    async ({ station, index, source }) => {
      const bitmap = await createImageBitmap(await (await fetch("data:image/png;base64," + source)).blob());
      const image = new OffscreenCanvas(bitmap.width, bitmap.height);
      const ctx = image.getContext("2d");
      ctx.drawImage(bitmap, 0, 0);
      const expected = ctx.getImageData(0, 0, image.width, image.height).data;
      return new Promise((resolve) =>
        requestAnimationFrame(() => {
          const canvas = document.querySelector("canvas"),
            gl = canvas.getContext("webgl2");
          const pixels = new Uint8Array(canvas.width * canvas.height * 4);
          gl.readPixels(0, 0, canvas.width, canvas.height, gl.RGBA, gl.UNSIGNED_BYTE, pixels);
          const camera = villageEvidence.camera;
          const size = index === 2 ? 48 : 64;
          const scale = (camera[2] * canvas.height) / 720;
          let sampled = 0,
            matched = 0;
          for (let y = 1; y < image.height - 1; y += 2) {
            for (let x = 1; x < image.width - 1; x += 2) {
              const at = (y * image.width + x) * 4;
              // Exclude antialiased edges; retain opaque details throughout the face.
              if ([0, -4, 4, -image.width * 4, image.width * 4].some((delta) => expected[at + delta + 3] < 250))
                continue;
              const px = Math.floor(
                canvas.width / 2 + (station.x - size / 2 + ((x + 0.5) * size) / image.width - camera[0]) * scale,
              );
              const py = Math.floor(
                canvas.height / 2 + (station.y - size + ((y + 0.5) * size) / image.height - camera[1]) * scale,
              );
              if (px < 2 || px >= canvas.width - 2 || py < 2 || py >= canvas.height - 2) continue;
              sampled++;
              let found = false;
              for (let dy = -1; dy <= 1; dy++)
                for (let dx = -1; dx <= 1; dx++) {
                  const actual = ((canvas.height - 1 - py - dy) * canvas.width + px + dx) * 4;
                  if ([0, 1, 2].every((channel) => Math.abs(pixels[actual + channel] - expected[at + channel]) <= 28))
                    found = true;
                }
              if (found) matched++;
            }
          }
          resolve({ sampled, matched, fraction: matched / Math.max(1, sampled), camera });
        }),
      );
    },
    { station, index, source },
  );
  assert.ok(
    result.sampled >= 40 && result.fraction >= 0.6,
    `${station.name} artwork is not visibly painted at its authored location: ${JSON.stringify(result)}`,
  );
  return result;
}
