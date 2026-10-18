// Regenerate the runtime compatibility fixture with the actual pinned compiler.
import { spawnSync } from "node:child_process";
import { mkdtemp, open, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
const directory = "packages/dialogue/testdata";
const source = await readFile(`${directory}/compatibility.yarn`);
const name = Buffer.from("compatibility.yarn");
const header = Buffer.alloc(14);
header.write("KPR1");
header.writeUInt32LE(1, 4);
header.writeUInt16LE(name.length, 8);
header.writeUInt32LE(source.length, 10);
const request = Buffer.concat([header, name, source]);
// A regular file gives the .NET reader an unambiguous EOF through the Go wrapper.
// Keep the compiler's trailing-byte validation instead of relying on socket EOF.
const scratch = await mkdtemp(join(tmpdir(), "karty-yarn-fixtures-"));
let command;
try {
  const path = join(scratch, "request.bin");
  await writeFile(path, request);
  const input = await open(path, "r");
  try {
    command = spawnSync("go", ["run", "-buildvcs=false", "./cmd/yarn-processor"], {
      cwd: "packages/dialogue",
      stdio: [input.fd, "pipe", "pipe"],
      env: {
        ...process.env,
        GOWORK: "off",
        DOTNET_NOLOGO: "1",
        DOTNET_CLI_TELEMETRY_OPTOUT: "1",
        DOTNET_SKIP_FIRST_TIME_EXPERIENCE: "1",
      },
    });
  } finally {
    await input.close();
  }
} finally {
  await rm(scratch, { recursive: true, force: true });
}
if (command.error) throw command.error;
if (command.status !== 0) throw new Error(command.stderr.toString());
const response = command.stdout;
if (response.toString("ascii", 0, 4) !== "KPR1") throw new Error("Invalid compiler envelope");
let at = 8;
for (let n = 0; n < response.readUInt32LE(4); n++) {
  const nameSize = response.readUInt16LE(at),
    size = response.readUInt32LE(at + 2);
  at += 6;
  const path = response.toString("utf8", at, at + nameSize);
  at += nameSize;
  if (!["story.yarnc", "strings.bin"].includes(path) || at + size > response.length)
    throw new Error("Invalid fixture output");
  await writeFile(`${directory}/${path}`, response.subarray(at, at + size));
  at += size;
}
if (at !== response.length) throw new Error("Trailing compiler bytes");
console.log("Compiled official Yarn Spinner 3.2.1 compatibility fixtures");
