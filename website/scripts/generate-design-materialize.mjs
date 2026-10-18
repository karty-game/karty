import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { promisify } from "node:util";

// Run from the repository root with the executable selected by the SDK.
// The SVG embeds unedited native output, so it needs no external image requests.
if (process.argv.length !== 3 || !process.argv[2]) {
  throw new Error("Usage: generate-design-materialize.mjs /path/to/pinned/materialize-cli");
}

const execute = promisify(execFile);
const tool = resolve(process.argv[2]);
const source = "samples/hangar/levels/hangar/brick.png";
const output = "website/assets/design-materialize.svg";
const directory = await mkdtemp(join(tmpdir(), "karty-doc-materialize-"));
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");
const argumentsFor = (destination) => [
  resolve(source),
  "--output",
  destination,
  "--only",
  "height,normal,ao,metallic,smoothness",
  "--normal-format",
  "opengl",
  "--format",
  "png",
  "--preset",
  "brick",
  "--verbose",
];

try {
  const { stdout, stderr } = await execute(tool, argumentsFor(directory), {
    timeout: 90000,
    maxBuffer: 1024 * 1024,
  });
  const maps = [
    ["Source colour", source],
    ["Normal", join(directory, "brick_normal.png")],
    ["Height", join(directory, "brick_height.png")],
    ["Ambient occlusion", join(directory, "brick_ao.png")],
    ["Metallic", join(directory, "brick_metallic.png")],
    ["Smoothness", join(directory, "brick_smoothness.png")],
  ];
  const panels = await Promise.all(maps.map(async ([label, file]) => ({ label, bytes: await readFile(file) })));
  const width = panels[0].bytes.readUInt32BE(16);
  const height = panels[0].bytes.readUInt32BE(20);
  const records = [];
  let svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 440 680" role="img" aria-labelledby="title description">
<title id="title">From one brick texture to material maps</title>
<desc id="description">An authored brick colour image beside five actual Materialize outputs: normal, height, ambient occlusion, metallic and smoothness. Each is ${width} by ${height} pixels. These are map views, not a rendered surface.</desc>
<rect width="440" height="680" rx="16" fill="#111f26"/>
<g font-family="system-ui,sans-serif" text-anchor="middle">
<text x="220" y="32" fill="#edf4f5" font-size="20" font-weight="600">One texture, five generated maps</text>
<text x="220" y="57" fill="#93a8b2" font-size="14">Materialize · brick preset · ${width} × ${height}</text>
`;
  for (const [index, panel] of panels.entries()) {
    if (
      panel.bytes.subarray(0, 8).toString("hex") !== "89504e470d0a1a0a" ||
      panel.bytes.readUInt32BE(16) !== width ||
      panel.bytes.readUInt32BE(20) !== height
    ) {
      throw new Error(`Unexpected image format or dimensions: ${panel.label}`);
    }
    const x = 12 + (index % 2) * 212;
    const y = 76 + Math.floor(index / 2) * 198;
    svg += `<rect x="${x}" y="${y}" width="204" height="188" rx="10" fill="#172630" stroke="${index === 0 ? "#51816c" : "#283d48"}"/>
<text x="${x + 102}" y="${y + 25}" fill="#edf4f5" font-size="16">${panel.label}</text>
<image x="${x + 30}" y="${y + 36}" width="144" height="144" style="image-rendering:pixelated" href="data:image/png;base64,${panel.bytes.toString("base64")}"/>
`;
    records.push({ label: panel.label, sha256: hash(panel.bytes), width, height });
  }
  svg += "</g></svg>\n";
  await writeFile(output, svg);
  const { stdout: version } = await execute(tool, ["--version"]);
  const record = {
    kind: "native material generation, not a game screenshot",
    generator: "website/scripts/generate-design-materialize.mjs",
    source,
    tool: { version: version.trim(), sha256: hash(await readFile(tool)) },
    arguments: argumentsFor("TEMP_OUTPUT").map((value) => (value === resolve(source) ? source : value)),
    backend: (stdout + stderr).split("\n").find((line) => line.startsWith("GPU:")) ?? "not reported",
    environment: Object.fromEntries(
      ["MATERIALIZE_GPU_BACKEND", "LIBGL_ALWAYS_SOFTWARE", "EGL_PLATFORM"].map((name) => [
        name,
        process.env[name] ?? "unset",
      ]),
    ),
    note: "Unedited 128-pixel source and native map PNGs embedded in an SVG grid. The game pipeline separately normalizes materials to 256 pixels, packs channels and builds mip levels. No host rendering or final lighting is captured.",
    panels: records,
    svg: { file: output, sha256: hash(svg) },
  };
  await writeFile("website/assets/design-materialize.generation.json", JSON.stringify(record, null, 2) + "\n");
  console.log(`Wrote ${output}\n${record.backend}`);
} finally {
  await rm(directory, { recursive: true, force: true });
}
