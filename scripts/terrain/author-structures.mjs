// Offline sample detail authoring. No engine source or runtime-generated geometry.
// Run from any directory: node scripts/terrain/author-structures.mjs
import { readFile, writeFile } from "node:fs/promises";
import { LIGHTHOUSE, PIER_APPROACH, ROCK_PLACEMENTS } from "./cove-layout.mjs";
const rockHeights = JSON.parse(await readFile(new URL("data/rock-placement-heights.json", import.meta.url), "utf8"));
if (
  !Array.isArray(rockHeights) ||
  rockHeights.length !== ROCK_PLACEMENTS.length ||
  rockHeights.some((h) => !Number.isFinite(h))
) {
  throw new Error("Rock placement heights must match cove-layout.mjs; refresh the retained terrain export.");
}
const worldPath = new URL("../../samples/terrain/levels/ground/world.yaml", import.meta.url);
const solids = [];
const bunkerRoofBottom = 17.65;
const n = (value) => Number(value.toFixed(5));
const plane = (value) => (typeof value === "number" ? { c: n(value) } : value);
const woodUV = { mode: "planar", anchor: "world", scale: { x: 3, y: 3 }, rotation_degrees: 90 };
function prism(id, footprint, bottom, top, material, extra = {}) {
  solids.push({
    id,
    footprint: footprint.map(([x, y]) => ({ x: n(x), y: n(y) })),
    bottom: plane(bottom),
    top: plane(top),
    side_material: material,
    top_material: material,
    bottom_material: material,
    collision: true,
    ...extra,
  });
}
function box(id, x0, y0, x1, y1, bottom, top, material, extra) {
  prism(
    id,
    [
      [x0, y0],
      [x1, y0],
      [x1, y1],
      [x0, y1],
    ],
    bottom,
    top,
    material,
    extra,
  );
}
function post(id, x, y, bottom, top, radius = 0.16) {
  // Squared timbers also keep the authored liquid shoreline decomposition small.
  box(id, x - radius, y - radius, x + radius, y + radius, bottom, top, "pier-wood");
}
// Low concrete observation bunker: actual hollow shell, sea-facing embrasures,
// side doorway and a covered footprint matching the small terrain cutout.
box("bunker-floor", -33.5, -11.5, -25.3, -0.8, 13.45, 14.2, "bunker-concrete");
box("bunker-rear", -33.5, -11.5, -32.45, -2.5, 14.2, 17.65, "bunker-concrete");
box("bunker-south", -33.5, -11.5, -25.5, -10.55, 14.2, 17.65, "bunker-concrete");
box("bunker-north-west", -33.5, -3.45, -29.6, -2.5, 14.2, 17.65, "bunker-concrete");
box("bunker-north-east", -27.6, -3.45, -25.5, -2.5, 14.2, 17.65, "bunker-concrete");
box("bunker-door-lintel", -29.6, -3.45, -27.6, -2.5, 16.85, 17.65, "bunker-concrete");
box("bunker-window-sill", -26.5, -11.5, -25.5, -2.5, 14.2, 15.35, "bunker-concrete");
box("bunker-window-brow", -26.5, -11.5, -25.35, -2.5, 16.2, 17.65, "bunker-concrete");
for (const [i, y] of [-10.55, -7.35, -3.8].entries())
  box(`bunker-window-pier-${i}`, -26.5, y, -25.5, y + 0.4, 15.35, 16.2, "bunker-concrete");
// Chamfered plan and a thick projecting roof avoid an ordinary flat house silhouette.
prism(
  "bunker-roof",
  [
    [-33.8, -11.9],
    [-26, -11.9],
    [-24.85, -10.8],
    [-24.85, -3.2],
    [-26, -2.1],
    [-33.8, -2.1],
  ],
  bunkerRoofBottom,
  18.55,
  "bunker-concrete",
);
// Close exposed heightfield cut faces above the roof. The lighthouse apron is
// higher than this bunker, so horizontal roof overlap alone cannot seal it.
const bunkerEdges = JSON.parse(await readFile(new URL("data/bunker-terrain-edges.json", import.meta.url), "utf8"));
for (const [index, edge] of bunkerEdges.entries()) {
  let a = edge.a.slice(),
    b = edge.b.slice();
  if (Math.max(a[2], b[2]) <= bunkerRoofBottom) continue;
  if (Math.min(a[2], b[2]) < bunkerRoofBottom) {
    const t = (bunkerRoofBottom - a[2]) / (b[2] - a[2]);
    const intersection = a.map((value, axis) => value + t * (b[axis] - value));
    if (a[2] < bunkerRoofBottom) a = intersection;
    else b = intersection;
  }
  const axis = a[0] === b[0] ? 1 : 0,
    slope = (b[2] - a[2]) / (b[axis] - a[axis]),
    top = { [axis === 0 ? "a" : "b"]: slope, c: a[2] - slope * a[axis] + 0.01 },
    [ix, iy] = edge.inward,
    footprint = [
      [a[0], a[1]],
      [b[0], b[1]],
      [b[0] + ix * 0.12, b[1] + iy * 0.12],
      [a[0] + ix * 0.12, a[1] + iy * 0.12],
    ];
  if ((b[0] - a[0]) * iy - (b[1] - a[1]) * ix < 0) footprint.reverse();
  prism(`bunker-terrain-stitch-${index}`, footprint, bunkerRoofBottom - 0.05, top, "coastal-rock");
}
// The visual cover sits just over the graded terrain bearing surface.
box("bunker-landing-cover", -30, -3.45, -26, -0.8, { b: 0.16, c: 14.28 }, { b: 0.16, c: 14.63 }, "bunker-concrete", {
  collision: false,
});
// A narrow flight climbs the existing north rock shelf; no new terrain hills.
for (let i = 0; i < 30; i++) {
  const y0 = -0.8 + i * 0.45,
    top = 14.5 - i * 0.27;
  box(`bunker-step-${i}`, -29.1, y0, -27.45, y0 + 0.455, top - 0.4, top, "bunker-concrete", { collision: false });
}
// Continuous narrow walking strip beside the treads uses existing slope collision.
box(
  "bunker-access-ramp",
  -27.45,
  -0.8,
  -26.65,
  12.7,
  { b: -0.6, c: 13.37 },
  { b: -0.6, c: 14.022 },
  "bunker-concrete",
  {
    collision: false,
  },
);
for (const y of [1.5, 5.5, 9.5])
  box(`bunker-access-support-${y}`, -28.9, y, -26.65, y + 0.35, 5.4, 13.37 - 0.6 * y, "bunker-concrete");
// Timber jetty: one modest 3.6m-wide deck with a roofed outer landing.
// Planks are real shallow solids; grain and joints keep their scale close up.
for (let i = 0; i < 68; i++) {
  const y = -4.6 + i * 0.3,
    top = 1.1 + (i % 7 === 0 ? 0.008 : 0);
  box(`pier-walk-plank-${i}`, 9.6, y, 13.2, y + 0.288, 0.89, top, "pier-wood", { top_uv: woodUV, collision: false });
}
for (let i = 0; i < 17; i++) {
  const y = -9.7 + i * 0.3;
  box(`pier-landing-plank-${i}`, 9.6, y, 13.2, y + 0.288, 0.89, 1.1, "pier-wood", { top_uv: woodUV, collision: false });
}
// Continuous bearing deck under the visible boards avoids tiny collision risers.
const { x0: pierX0, x1: pierX1, deckEndY: pierDeckEnd, deckHeight: pierDeckHeight } = PIER_APPROACH;
box("pier-walk-bearing", pierX0, -9.7, pierX1, pierDeckEnd, 0.73, pierDeckHeight - 0.01, "pier-wood", {
  top_uv: woodUV,
});
// A gentle boarding ramp continues into the sand rather than presenting the
// bearing slab's vertical end to a walking body. Its landward end is buried.
const { rampStartY: pierRampStart, rampEndY: pierRampEnd, toeHeight: pierToeHeight } = PIER_APPROACH,
  pierRampSlope = (pierToeHeight - pierDeckHeight) / (pierRampEnd - pierRampStart),
  pierRampBase = pierDeckHeight - pierRampSlope * pierRampStart;
// The graded heightfield supplies collision through the entire overlap and
// ramp; separate slab end faces must not obstruct the approach.
box("pier-beach-ramp-landing", pierX0, pierDeckEnd, pierX1, pierRampStart, 0.74, pierDeckHeight, "pier-wood", {
  top_uv: woodUV,
  collision: false,
});
box(
  "pier-beach-ramp-bearing",
  pierX0,
  pierRampStart,
  pierX1,
  pierRampEnd,
  { b: pierRampSlope, c: pierRampBase - 0.34 },
  { b: pierRampSlope, c: pierRampBase },
  "pier-wood",
  { top_uv: woodUV, collision: false },
);
for (let i = 0; i < 18; i++) {
  const y = pierDeckEnd + i * 0.3,
    slope = i < 8 ? 0 : pierRampSlope,
    base = i < 8 ? pierDeckHeight : pierRampBase;
  box(
    `pier-beach-ramp-plank-${i}`,
    pierX0,
    y,
    pierX1,
    y + 0.288,
    { b: slope, c: base - 0.18 },
    { b: slope, c: base + 0.02 },
    "pier-wood",
    { top_uv: woodUV, collision: false },
  );
}

// Two longitudinal stringers and crossbeams make the deck visibly supported.
for (const x of [9.9, 12.65]) box(`pier-stringer-${x}`, x, -9.5, x + 0.22, 15.85, 0.53, 0.89, "pier-wood");
for (let i = 0; i < 7; i++) {
  const y = -4.5 + i * 3.2;
  box(`pier-crossbeam-${i}`, 9.45, y - 0.16, 13.35, y + 0.16, 0.45, 0.72, "pier-wood");
  for (const [j, x] of [9.5, 13.3].entries()) post(`pier-piling-${i}-${j}`, x, y, -9, 1.48 + (i % 3) * 0.07);
}
for (const y of [-9.4, -4.8]) {
  box(`pier-landing-crossbeam-${y}`, 9.4, y - 0.18, 13.4, y + 0.18, 0.43, 0.75, "pier-wood");
  for (const x of [9.5, 13.3]) post(`pier-landing-pile-${x}-${y}`, x, y, -9, 1.65, 0.19);
}
// Open canopy: four uprights, beams, and a simple pitched timber roof.
for (const x of [9.75, 13.05]) for (const y of [-9.05, -5.1]) post(`pier-canopy-post-${x}-${y}`, x, y, 1.1, 4.1, 0.13);
for (const x of [9.6, 13.0]) box(`pier-canopy-side-${x}`, x, -9.25, x + 0.22, -4.9, 3.8, 4.08, "pier-wood");
for (const y of [-9.17, -5.08]) box(`pier-canopy-tie-${y}`, 9.5, y, 13.3, y + 0.2, 3.85, 4.08, "pier-wood");
const ridgeX = 11.4;
box("pier-roof-west", 9.1, -9.8, ridgeX, -4.35, { a: 0.5, c: -0.63 }, { a: 0.5, c: -0.45 }, "pier-wood", {
  top_uv: woodUV,
});
box("pier-roof-east", ridgeX, -9.8, 13.7, -4.35, { a: -0.5, c: 10.77 }, { a: -0.5, c: 10.95 }, "pier-wood", {
  top_uv: woodUV,
});
box("pier-roof-ridge", 11.3, -9.85, 11.5, -4.3, 5.12, 5.3, "pier-wood");
// Sparse guardrails leave the landing and boarding side open.
for (const x of [9.52, 13.17])
  for (const y of [2, 8, 14]) {
    post(`pier-rail-upright-${x}-${y}`, x, y, 0.8, 2.08, 0.07);
  }
for (const x of [9.48, 13.13])
  for (const [i, y] of [2, 8].entries()) {
    box(`pier-rail-${x}-${i}`, x, y, x + 0.11, y + 6, 1.96, 2.07, "pier-wood");
  }
// A loose, weathered board on the beach, with one end partly bedded in sand.
const boardAngle = 0.37,
  boardX = 16,
  boardY = 16;
const boardCorners = [
  [-1.3, -0.2],
  [1.3, -0.2],
  [1.3, 0.2],
  [-1.3, 0.2],
].map(([x, y]) => [
  boardX + x * Math.cos(boardAngle) - y * Math.sin(boardAngle),
  boardY + x * Math.sin(boardAngle) + y * Math.cos(boardAngle),
]);
prism("beach-loose-board", boardCorners, 0.75, 0.86, "pier-wood", { top_uv: woodUV, collision: false });
// Lighthouse: a compact painted masonry tower and open framed lantern.
const { x: lx, y: ly, z: lz } = LIGHTHOUSE;
function ring(radius, sides = 12, cx = lx, cy = ly) {
  return Array.from({ length: sides }, (_, i) => [
    cx + radius * Math.cos((i * Math.PI * 2) / sides),
    cy + radius * Math.sin((i * Math.PI * 2) / sides),
  ]);
}
prism("lighthouse-plinth", ring(2.9), lz - 0.45, lz + 0.3, "bunker-concrete");
prism("lighthouse-tower", ring(2.05), lz + 0.3, lz + 9.4, "lighthouse-white");
prism("lighthouse-red-band", ring(2.07), lz + 3.8, lz + 5.5, "lighthouse-red");
prism("lighthouse-balcony", ring(2.65), lz + 9.4, lz + 9.75, "lighthouse-white");
prism("lighthouse-lantern-base", ring(1.65, 8), lz + 9.75, lz + 10.05, "lighthouse-metal");
for (let i = 0; i < 8; i++) {
  const theta = (i * Math.PI) / 4,
    x = lx + 1.6 * Math.cos(theta),
    y = ly + 1.6 * Math.sin(theta);
  box(`lighthouse-lantern-post-${i}`, x - 0.07, y - 0.07, x + 0.07, y + 0.07, lz + 9.95, lz + 11.8, "lighthouse-metal");
  const a = theta,
    b = theta + Math.PI / 4;
  const quad = (r0, r1) => [
    [lx + r0 * Math.cos(a), ly + r0 * Math.sin(a)],
    [lx + r1 * Math.cos(a), ly + r1 * Math.sin(a)],
    [lx + r1 * Math.cos(b), ly + r1 * Math.sin(b)],
    [lx + r0 * Math.cos(b), ly + r0 * Math.sin(b)],
  ];
  prism(`lighthouse-handrail-${i}`, quad(2.43, 2.53), lz + 10.8, lz + 10.9, "lighthouse-metal");
  box(
    `lighthouse-baluster-${i}`,
    lx + 2.48 * Math.cos(a) - 0.04,
    ly + 2.48 * Math.sin(a) - 0.04,
    lx + 2.48 * Math.cos(a) + 0.04,
    ly + 2.48 * Math.sin(a) + 0.04,
    lz + 9.75,
    lz + 10.8,
    "lighthouse-metal",
  );
  const mid = theta + Math.PI / 8,
    g = -1.3 / (2.1 * Math.cos(Math.PI / 8)),
    pa = g * Math.cos(mid),
    pb = g * Math.sin(mid),
    pc = lz + 13.1 - pa * lx - pb * ly;
  prism(
    `lighthouse-roof-${i}`,
    [
      [lx, ly],
      [lx + 2.1 * Math.cos(a), ly + 2.1 * Math.sin(a)],
      [lx + 2.1 * Math.cos(b), ly + 2.1 * Math.sin(b)],
    ],
    { a: pa, b: pb, c: pc - 0.16 },
    { a: pa, b: pb, c: pc },
    "lighthouse-red",
  );
}
prism("lighthouse-beacon", ring(0.4, 8), lz + 10.65, lz + 11.3, "lighthouse-lamp", { collision: false });
box("lighthouse-door", lx - 0.52, ly + 1.96, lx + 0.52, ly + 2.09, lz + 0.3, lz + 2.4, "pier-wood");
box("lighthouse-door-brow", lx - 0.65, ly + 1.9, lx + 0.65, ly + 2.18, lz + 2.4, lz + 2.55, "lighthouse-white");
// Three actual reusable source prefabs, each built from irregular fractured solids.
const prefabs = [];
function rockPrefab(id, build) {
  const start = solids.length;
  build();
  prefabs.push({ id, solids: solids.splice(start) });
}
function stoneFace(id, vertices) {
  const [origin, a, b] = vertices;
  const ax = a[0] - origin[0],
    ay = a[1] - origin[1];
  const bx = b[0] - origin[0],
    by = b[1] - origin[1];
  const det = ax * by - bx * ay;
  const pa = ((a[2] - origin[2]) * by - (b[2] - origin[2]) * ay) / det;
  const pb = (ax * (b[2] - origin[2]) - bx * (a[2] - origin[2])) / det;
  prism(
    id,
    vertices.map(([x, y]) => [x, y]),
    -0.45,
    { a: pa, b: pb, c: origin[2] - pa * origin[0] - pb * origin[1] },
    "coastal-rock",
  );
}
function facetedStone(id, points, crown, edgeHeights) {
  // A broad, broken cap and steep chipped shoulders, never a common apex.
  // The cap is a slanted convex polygon; each shoulder has two planar fractures.
  const top = { a: 0.055, b: -0.045, c: crown[2] - 0.055 * crown[0] + 0.045 * crown[1] };
  const inner = points.map(([x, y]) => {
    const ix = crown[0] + (x - crown[0]) * 0.56;
    const iy = crown[1] + (y - crown[1]) * 0.56;
    return [ix, iy, top.a * ix + top.b * iy + top.c];
  });
  prism(
    `${id}-cap`,
    inner.map(([x, y]) => [x, y]),
    -0.45,
    top,
    "coastal-rock",
  );
  for (let i = 0; i < points.length; i++) {
    const next = (i + 1) % points.length;
    const a = [...points[i], edgeHeights[i]],
      b = [...points[next], edgeHeights[next]];
    stoneFace(`${id}-shoulder-${i}-a`, [a, b, inner[next]]);
    stoneFace(`${id}-shoulder-${i}-b`, [a, inner[next], inner[i]]);
  }
}
rockPrefab("coastal-slab", () =>
  facetedStone(
    "slab",
    [
      [-1.3, -0.65],
      [0.45, -0.9],
      [1.25, -0.25],
      [0.8, 0.7],
      [-0.7, 0.85],
    ],
    [-0.1, 0.05, 0.48],
    [0.1, 0.23, 0.08, 0.16, 0.12],
  ),
);
rockPrefab("coastal-boulder", () =>
  facetedStone(
    "boulder",
    [
      [-1, -0.75],
      [0.4, -1],
      [1, 0.05],
      [0.55, 0.9],
      [-0.7, 0.7],
    ],
    [0.05, -0.1, 0.92],
    [0.2, 0.35, 0.18, 0.25, 0.15],
  ),
);
rockPrefab("coastal-split-stone", () => {
  facetedStone(
    "west",
    [
      [-1, -0.55],
      [-0.15, -0.8],
      [0.05, 0.25],
      [-0.45, 0.65],
      [-1.1, 0.15],
    ],
    [-0.55, -0.1, 0.95],
    [0.12, 0.25, 0.1, 0.25, 0.15],
  );
  facetedStone(
    "east",
    [
      [-0.12, -0.65],
      [0.55, -0.75],
      [1.1, 0.1],
      [0.4, 0.75],
      [0.02, 0.25],
    ],
    [0.4, 0.05, 0.65],
    [0.1, 0.15, 0.08, 0.12, 0.1],
  );
});
// Deliberate clusters beside the paths; keep travel corridors and doorways open.
const placements = ROCK_PLACEMENTS;
const instances = placements.map(([prefab, x, y, yaw, scale], i) => ({
  id: `cove-rock-${i}`,
  prefab,
  transform: { translation: { x, y, z: rockHeights[i] }, yaw_degrees: yaw, scale },
}));

const begin = "  # BEGIN COVE STRUCTURES",
  end = "# END COVE STRUCTURES";
let source = await readFile(worldPath, "utf8");
const block = begin + "\n" + solids.map((s) => "  - " + JSON.stringify(s)).join("\n") + "\n" + end + "\n";
if (source.includes(begin))
  source = source.slice(0, source.indexOf(begin)) + block + source.slice(source.indexOf(end) + end.length + 1);
else source = source.replace("fluids:\n", block + "fluids:\n");
const detailBegin = "# BEGIN COVE ROCK PREFABS",
  detailEnd = "# END COVE ROCK PREFABS";
const detailBlock =
  detailBegin +
  "\nprefabs:\n" +
  prefabs.map((p) => "  - " + JSON.stringify(p)).join("\n") +
  "\ninstances:\n" +
  instances.map((p) => "  - " + JSON.stringify(p)).join("\n") +
  "\n" +
  detailEnd +
  "\n";
if (source.includes(detailBegin))
  source =
    source.slice(0, source.indexOf(detailBegin)) +
    detailBlock +
    source.slice(source.indexOf(detailEnd) + detailEnd.length + 1);
else source += "\n" + detailBlock;
await writeFile(worldPath, source);
console.log(
  `Authored ${solids.length} cove solids, ${prefabs.length} rock prefabs and ${instances.length} rock instances.`,
);
