// Loopback-only preview. Pin the complete distribution so rebuilds cannot mix levels.
import { createServer } from "node:http";
import { readFile, readdir, realpath } from "node:fs/promises";
import { createHash } from "node:crypto";
import { resolve, extname } from "node:path";
import { pathToFileURL } from "node:url";
import { gzipSync } from "node:zlib";

export async function startVillageServer(directory, port = 0) {
  directory = await realpath(directory);
  const files = new Map();
  const mime = {
    ".html": "text/html; charset=utf-8",
    ".js": "text/javascript",
    ".wasm": "application/wasm",
    ".kart": "application/wasm",
    ".kld": "application/wasm",
  };
  for (const item of await readdir(directory, { recursive: true, withFileTypes: true })) {
    if (!item.isFile()) continue;
    const file = resolve(item.parentPath, item.name);
    const name = file.slice(directory.length + 1);
    const body = await readFile(file);
    files.set("/" + name, {
      body,
      gzip: [".wasm", ".kart", ".kld"].includes(extname(file)) ? gzipSync(body, { level: 6 }) : null,
      type: mime[extname(file)] || "application/octet-stream",
      hash: createHash("sha256").update(body).digest("hex"),
    });
  }
  const server = createServer((request, response) => {
    const url = new URL(request.url, "http://localhost");
    let path;
    try {
      path = decodeURIComponent(url.pathname);
    } catch {
      response.writeHead(400).end();
      return;
    }
    if (path === "/") path = "/index.html";
    const file = files.get(path);
    if (!file || !["GET", "HEAD"].includes(request.method)) {
      response.writeHead(404).end();
      return;
    }
    const compressed = file.gzip && /(?:^|[,\s])gzip(?:[,\s;]|$)/i.test(request.headers["accept-encoding"] || "");
    const body = compressed ? file.gzip : file.body;
    response.writeHead(200, {
      "Content-Type": file.type,
      "Content-Length": body.length,
      "Cache-Control": "no-store",
      ETag: '"' + file.hash + '"',
      Vary: "Accept-Encoding",
      ...(compressed ? { "Content-Encoding": "gzip" } : {}),
    });
    response.end(request.method === "HEAD" ? undefined : body);
  });
  await new Promise((done, reject) => {
    server.once("error", reject);
    server.listen(port, "127.0.0.1", done);
  });
  return server;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const server = await startVillageServer(
    process.argv[2] || "samples/willowbrook/dist/web",
    Number(process.argv[3] || 39718),
  );
  console.log(
    JSON.stringify({
      port: server.address().port,
      pid: process.pid,
      binding: "127.0.0.1",
      snapshot: true,
      url: `http://127.0.0.1:${server.address().port}/`,
    }),
  );
}
