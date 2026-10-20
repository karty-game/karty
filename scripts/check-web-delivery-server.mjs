// Loopback-only fixture server for the maintained browser delivery checks.
// Compression is ordinary HTTP gzip: the distribution has no sidecar files.
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, relative, resolve } from "node:path";
import { gzipSync } from "node:zlib";

const types = new Map([
  [".html", "text/html; charset=utf-8"],
  [".js", "text/javascript; charset=utf-8"],
  [".json", "application/json"],
  [".wasm", "application/wasm"],
  [".kart", "application/wasm"],
  [".kld", "application/wasm"],
]);

export async function startDeliveryServer(fixtures) {
  const releases = new Map([
    ["/game/", "a"],
    ["/other/", "a"],
  ]);
  const faults = new Map();
  const requests = [];
  const pending = new Set();
  const workerRevisions = new Map();
  let online = true;
  let active = 0;
  let peak = 0;
  const server = createServer(async (request, response) => {
    const url = new URL(request.url, "http://fixture.invalid");
    const prefix = [...releases.keys()].find((candidate) => url.pathname.startsWith(candidate));
    const name = prefix ? decodeURIComponent(url.pathname.slice(prefix.length)) || "index.html" : "";
    const entry = { path: url.pathname, query: url.search, active: ++active, aborted: false };
    requests.push(entry);
    peak = Math.max(peak, active);
    response.once("close", () => {
      active--;
      entry.aborted = !response.writableFinished;
    });
    const finish = (status, body) => {
      response.writeHead(status, { "Content-Type": "text/plain", "Cache-Control": "no-store" });
      response.end(body);
    };
    try {
      if (!online) return response.destroy();
      if (!prefix || /\.(br|gz)$/.test(name)) return finish(404, "missing");
      const fault = faults.get(url.pathname);
      if (fault?.redirect) {
        entry.redirect = fault.redirect;
        entry.status = fault.status || 302;
        response.writeHead(entry.status, {
          Location: fault.redirect,
          "Cache-Control": "no-store",
          "Access-Control-Allow-Origin": "*",
        });
        return response.end();
      }
      const root = resolve(fixtures, releases.get(prefix));
      const file = resolve(root, name);
      if (relative(root, file).startsWith("..")) return finish(403, "forbidden");
      let body = await readFile(file);
      if (name === "karty-sw.js" && workerRevisions.has(prefix)) {
        entry.workerRevision = workerRevisions.get(prefix);
        body = Buffer.concat([
          body,
          Buffer.from("\n// Browser fixture worker revision " + entry.workerRevision + "\n"),
        ]);
      }
      if (fault?.status) return finish(fault.status, "fixture failure");
      if (fault?.kind === "corrupt") {
        body = Buffer.from(body);
        body[body.length - 1] ^= 1;
      }
      if (fault?.kind === "truncated") body = body.subarray(0, Math.max(0, body.length - 1));
      if (fault?.kind === "oversized") body = Buffer.concat([body, Buffer.from([0])]);
      if (fault?.headers) {
        entry.headersBlocked = true;
        await new Promise((done) => {
          pending.add(done);
          fault.release = () => {
            pending.delete(done);
            done();
          };
          response.once("close", fault.release);
        });
      }
      if (response.destroyed) return;
      const encoded = gzipSync(body);
      response.writeHead(200, {
        "Content-Type": types.get(extname(file)) || "application/octet-stream",
        "Content-Encoding": "gzip",
        "Content-Length": encoded.length,
        "Cache-Control":
          name === "karty-latest.json" || name.endsWith(".html") || name === "karty-sw.js"
            ? "no-store"
            : "public, max-age=31536000, immutable",
        Vary: "Accept-Encoding",
        "Access-Control-Allow-Origin": "*",
      });
      entry.encodedBytes = encoded.length;
      entry.decodedBytes = body.length;
      if (fault?.body) {
        const split = Math.max(11, Math.floor(encoded.length / 2));
        response.write(encoded.subarray(0, split));
        entry.bodyBlocked = true;
        await new Promise((done) => {
          pending.add(done);
          fault.release = () => {
            pending.delete(done);
            done();
          };
          response.once("close", fault.release);
        });
        if (!response.destroyed) response.end(encoded.subarray(split));
      } else response.end(encoded);
    } catch (error) {
      if (!response.headersSent) finish(error.code === "ENOENT" ? 404 : 500, String(error));
      else response.destroy(error);
    }
  });
  await new Promise((ready, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", ready);
  });
  return {
    origin: `http://127.0.0.1:${server.address().port}`,
    requests,
    faults,
    setRelease: (prefix, release) => releases.set(prefix, release),
    setWorkerRevision: (prefix, revision) => workerRevisions.set(prefix, revision),
    setOnline: (value) => {
      online = value;
    },
    peak: () => peak,
    close: async () => {
      for (const done of pending) done();
      server.closeAllConnections();
      await new Promise((done) => server.close(done));
    },
  };
}
