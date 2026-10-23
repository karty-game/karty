// Keep the established task name while measuring the sample-owned elevator.
// Character policy allocations belong to the package; the engine tests its motor.
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { copyFile, mkdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const stage = resolve("dist/validation/hangar-elevator-allocations");
const source = resolve("samples/hangar/internal/elevator");
await mkdir(resolve(stage, "internal/elevator"), { recursive: true });
await mkdir(resolve(stage, "app"), { recursive: true });
await copyFile(resolve(source, "schedule.go"), resolve(stage, "internal/elevator/schedule.go"));
await copyFile(resolve(source, "testdata/allocations.go.txt"), resolve(stage, "app/main.go"));
await writeFile(resolve(stage, "go.mod"), "module example.com/hangar\n\ngo 1.27.0\n");
const build = spawnSync(
  "tinygo",
  ["build", "-target=wasi", "-buildmode=c-shared", "-scheduler=none", "-o", "guest.wasm", "./app"],
  { cwd: stage, env: { ...process.env, GOWORK: "off" }, stdio: "inherit" },
);
if (build.error) throw build.error;
assert.equal(build.status, 0, "TinyGo must compile the actual elevator schedule");
const module = await WebAssembly.compile(await readFile(resolve(stage, "guest.wasm")));
const { exports: guest } = await WebAssembly.instantiate(module, {
  wasi_snapshot_preview1: {
    fd_write: () => 0,
    proc_exit: (code) => {
      throw new Error(`guest exited: ${code}`);
    },
    random_get: () => 0,
    sched_yield: () => 0,
    poll_oneoff: () => 0,
    clock_time_get: () => 0,
  },
});
guest._initialize();
function tick(frame) {
  const height = guest.tick(BigInt(frame));
  assert.ok(height >= 0.1999 && height <= 3.4001, "elevator leaves its authored bounds");
}
for (let frame = 0; frame < 100; frame++) tick(frame);
const allocations = guest.allocations(),
  bytes = guest.allocated_bytes();
for (let frame = 100; frame < 1100; frame++) tick(frame);
const result = {
  name: "elevator",
  updates: 1000,
  allocations: String(guest.allocations() - allocations),
  allocatedBytes: String(guest.allocated_bytes() - bytes),
};
console.log(JSON.stringify(result));
assert.equal(result.allocations, "0", "elevator must not allocate");
assert.equal(result.allocatedBytes, "0", "elevator must not allocate bytes");
const control = guest.allocations(),
  controlBytes = guest.allocated_bytes();
guest.negative_control();
assert.ok(guest.allocations() > control && guest.allocated_bytes() > controlBytes, "heap counter negative control");
await writeFile(resolve(stage, "report.json"), JSON.stringify({ passed: true, results: [result] }, null, 2) + "\n");
