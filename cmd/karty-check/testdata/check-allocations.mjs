// Diagnostic runner for a temporary cartridge built with karty_alloccheck.
// Usage: node check-allocations.mjs /path/to/game.kart /path/to/engine/protocol.go
import { readFile } from "node:fs/promises";
import assert from "node:assert/strict";

const protocolSource = await readFile(process.argv[3], "utf8");
const protocolVersion = Number(protocolSource.match(/protocolVersion\s*=\s*uint16\((\d+)\)/)?.[1]);
assert.ok(Number.isInteger(protocolVersion) && protocolVersion > 0, "SDK must declare a wire version");

const { instance } = await WebAssembly.instantiate(await readFile(process.argv[2]), {
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
  karty: { log: () => {}, play_sound: () => {}, action: () => {}, submit_commands: () => {} },
});
const guest = instance.exports;
guest._initialize();
guest.karty_register();
guest.initialize();

function update(frame) {
  const pointer = guest.event_buffer() >>> 0;
  const view = new DataView(guest.memory.buffer, pointer, 29);
  view.setUint32(0, 0x4259544b, true);
  view.setUint16(4, protocolVersion, true);
  view.setUint8(6, 1);
  view.setUint8(7, 0);
  view.setBigUint64(8, BigInt(frame), true);
  view.setUint32(16, 1, true);
  view.setUint32(20, 5, true);
  view.setUint8(24, frame % 2 === 0 ? 1 : 2);
  view.setUint16(25, 2, true);
  view.setUint16(27, 1, true);
  guest.update(BigInt(frame), 29);
}

// Warm runtime and stack initialization before sampling guest heap counters.
for (let frame = 0; frame < 100; frame++) update(frame);
const allocations = guest.karty_allocations();
const bytes = guest.karty_allocated_bytes();
for (let frame = 100; frame < 1100; frame++) update(frame);
const allocationDelta = guest.karty_allocations() - allocations;
const byteDelta = guest.karty_allocated_bytes() - bytes;
console.log(`Guest: ${allocationDelta} allocations, ${byteDelta} bytes across 1000 updates`);
assert.equal(allocationDelta, 0n);
assert.equal(byteDelta, 0n);
guest.shutdown();
