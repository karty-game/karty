// Offline authoring only. Runtime stores sector cohorts, never plant positions.
import { readFile, writeFile } from "node:fs/promises";
const worldURL = new URL("../../samples/terrain/levels/ground/world.yaml", import.meta.url);
const groundcover = JSON.parse(await readFile(new URL("./data/groundcover-groups.json", import.meta.url), "utf8"));
const seabed = JSON.parse(await readFile(new URL("./data/seabed-groups.json", import.meta.url), "utf8"));
const edges = [-1024, -64, -32, 0, 32, 64, 1024];
// Integer cohort sizes on 32m square rooms keep the GPU seed prefix exact.
// Seeds were selected on vegetated ground, away from sand, exposed rock and trails.
// This is offline authoring; all roots remain GPU-derived at runtime.
export const TREE_GROUPS = [
  {
    x: 2,
    y: 3,
    preset: "coastal-palm",
    count: 6,
    seed: 10352,
  },
  {
    x: 3,
    y: 3,
    preset: "coastal-palm",
    count: 4,
    seed: 140,
  },
  {
    x: 4,
    y: 3,
    preset: "coastal-palm",
    count: 12,
    seed: 483325,
  },
  {
    x: 1,
    y: 3,
    preset: "coastal-palm",
    count: 8,
    seed: 505753,
  },
  {
    x: 1,
    y: 4,
    preset: "coastal-palm",
    count: 7,
    seed: 58236,
  },
  {
    x: 2,
    y: 4,
    preset: "coastal-palm",
    count: 14,
    seed: 6609,
  },
  {
    x: 3,
    y: 4,
    preset: "coastal-palm",
    count: 11,
    seed: 183908,
  },
  {
    x: 4,
    y: 4,
    preset: "coastal-palm",
    count: 7,
    seed: 38463,
  },
  {
    x: 3,
    y: 4,
    preset: "coastal-broadleaf",
    count: 4,
    seed: 78,
  },
  {
    x: 4,
    y: 4,
    preset: "coastal-broadleaf",
    count: 3,
    seed: 1,
  },
];
const presets = [
  {
    name: "coastal-grass",
    geometry: "crossed_cards",
    texture: "coastal-grass",
    normal_texture: "coastal-grass-normal",
    width: { min: 0.88, max: 1.32 },
    height: { min: 0.38, max: 0.62 },
    wind_amplitude: 0.07,
    wind_frequency: 0.85,
    draw_distance: 28,
  },
  {
    name: "coastal-tall-grass",
    geometry: "crossed_cards",
    texture: "coastal-tall-grass",
    normal_texture: "coastal-tall-grass-normal",
    width: { min: 0.65, max: 1.0 },
    height: { min: 0.85, max: 1.25 },
    wind_amplitude: 0.13,
    wind_frequency: 0.75,
    draw_distance: 36,
  },
  {
    name: "coastal-red-fruit",
    geometry: "crossed_cards",
    texture: "coastal-red-fruit",
    normal_texture: "coastal-red-fruit-normal",
    width: { min: 0.55, max: 0.8 },
    height: { min: 0.65, max: 0.95 },
    wind_amplitude: 0.045,
    wind_frequency: 0.7,
    draw_distance: 80,
  },
  {
    name: "coastal-seagrass",
    geometry: "crossed_cards",
    texture: "coastal-seagrass",
    normal_texture: "coastal-seagrass-normal",
    width: { min: 0.65, max: 1.1 },
    height: { min: 0.35, max: 0.7 },
    wind_amplitude: 0.12,
    wind_frequency: 0.28,
    draw_distance: 28,
  },
  {
    name: "coastal-ribbon-kelp",
    geometry: "crossed_cards",
    texture: "coastal-ribbon-kelp",
    normal_texture: "coastal-ribbon-kelp-normal",
    width: { min: 0.7, max: 1.2 },
    height: { min: 0.9, max: 1.65 },
    wind_amplitude: 0.22,
    wind_frequency: 0.22,
    draw_distance: 38,
  },
  {
    name: "coastal-shrub",
    geometry: "shrub",
    texture: "coastal-broadleaf",
    normal_texture: "coastal-broadleaf-normal",
    width: { min: 1.15, max: 1.8 },
    height: { min: 0.7, max: 1.0 },
    wind_amplitude: 0.045,
    wind_frequency: 0.7,
    draw_distance: 100,
  },
  {
    name: "coastal-palm",
    geometry: "palm",
    texture: "coastal-palm",
    normal_texture: "coastal-palm-normal",
    width: { min: 8, max: 11 },
    height: { min: 5.4, max: 7.8 },
    wind_amplitude: 0.25,
    wind_frequency: 0.55,
    draw_distance: 128,
  },
  {
    name: "coastal-broadleaf",
    geometry: "tree",
    texture: "coastal-broadleaf",
    normal_texture: "coastal-broadleaf-normal",
    width: { min: 8, max: 11 },
    height: { min: 8, max: 11 },
    wind_amplitude: 0.1,
    wind_frequency: 0.6,
    draw_distance: 128,
  },
];
// The public format permits one preset per sector surface. Identical aliases
// let offline seed cohorts share one texture/geometry batch per plant species.
for (const name of ["coastal-grass", "coastal-tall-grass"]) {
  const slots = new Map();
  for (const group of groundcover) {
    if (group.preset !== name) continue;
    const key = `${group.x}/${group.y}`;
    const slot = slots.get(key) || 0;
    slots.set(key, slot + 1);
    if (slot > 0) group.preset = `${name}-${slot}`;
  }
  const template = presets.find((preset) => preset.name === name);
  for (let slot = 1; slot < Math.max(...slots.values()); slot++) presets.push({ ...template, name: `${name}-${slot}` });
}
if (presets.length > 64) throw new Error("Ground cover exceeds the existing preset limit");
const allGroups = [...TREE_GROUPS, ...groundcover, ...seabed];
if (allGroups.length > 1024 || allGroups.reduce((sum, group) => sum + group.count * 2, 0) > 16384)
  throw new Error("Combined land/seabed cover exceeds the existing layer/card limits");
const rooms = [],
  connections = [];
const id = (x, y) => `cove-${x}-${y}`;
for (let y = 0; y < 6; y++)
  for (let x = 0; x < 6; x++) {
    const x0 = edges[x],
      y0 = edges[y],
      x1 = edges[x + 1],
      y1 = edges[y + 1];
    const points = [
      { x: x0, y: y0 },
      { x: x1, y: y0 },
      { x: x1, y: y1 },
      { x: x0, y: y1 },
    ];
    const boundary = points.map((start, edge) => ({
      id: `edge-${edge}`,
      start,
      end: points[(edge + 1) % 4],
      material: "seabed",
      ...([y === 0, x === 5, y === 5, x === 0][edge] ? { sky: true } : {}),
    }));
    const groups = allGroups.filter((group) => group.x === x && group.y === y);
    const room = {
      id: id(x, y),
      floor: { c: -32 },
      ceiling: { c: 128 },
      floor_material: "seabed",
      ceiling_material: "seabed",
      sky: true,
      boundary,
    };
    if (groups.length)
      room.vegetation_layers = groups.map(({ preset, count, seed }) => ({
        preset,
        density: count / ((x1 - x0) * (y1 - y0)),
        seed,
        surface: "floor",
      }));
    rooms.push(room);
    if (x < 5)
      connections.push({
        id: `cove-link-east-${x}-${y}`,
        a: { room: id(x, y), edge: "edge-1" },
        b: { room: id(x + 1, y), edge: "edge-3" },
      });
    if (y < 5)
      connections.push({
        id: `cove-link-north-${x}-${y}`,
        a: { room: id(x, y), edge: "edge-2" },
        b: { room: id(x, y + 1), edge: "edge-0" },
      });
  }
let world = await readFile(worldURL, "utf8");
world = world.replace(/^version: \d+$/m, "version: 20");
const roomStart = world.indexOf("\nrooms:\n"),
  solidsStart = world.indexOf("\nsolids:\n");
if (roomStart < 0 || solidsStart <= roomStart) throw new Error("Cannot locate cove room block");
world =
  world.slice(0, roomStart) +
  "\nrooms:\n" +
  rooms.map((room) => `  - ${JSON.stringify(room)}\n`).join("") +
  "connections:\n" +
  connections.map((connection) => `  - ${JSON.stringify(connection)}\n`).join("") +
  world.slice(solidsStart);
const vegetationStart = world.indexOf("\nvegetation:\n"),
  uvStart = world.indexOf("\nuv:\n");
if (uvStart < 0) throw new Error("Cannot locate world UV settings");
if (vegetationStart >= 0) world = world.slice(0, vegetationStart) + world.slice(uvStart);
const uvIndex = world.indexOf("\nuv:\n");
world =
  world.slice(0, uvIndex) +
  "\nvegetation:\n  version: 5\n  presets:\n" +
  presets.map((preset) => `    - ${JSON.stringify(preset)}\n`).join("") +
  world.slice(uvIndex);
await writeFile(worldURL, world);
console.log(
  `Authored ${rooms.length} connected cove sectors, ${TREE_GROUPS.length} GPU tree layers and ${TREE_GROUPS.reduce((n, g) => n + g.count, 0)} seeded trees.`,
);
console.log(`Added ${groundcover.length} authored grass/shrub cohorts; roots and wind remain GPU-derived.`);
