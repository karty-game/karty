import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

const resolver = fileURLToPath(new URL("./resolve-release-tag.sh", import.meta.url));

function fixture(t) {
  const directory = mkdtempSync(join(tmpdir(), "karty-release-ref-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const git = (...args) =>
    execFileSync(
      "git",
      [
        "-c",
        "core.hooksPath=/dev/null",
        "-c",
        "commit.gpgSign=false",
        "-c",
        "tag.gpgSign=false",
        "-c",
        "user.name=Release fixture",
        "-c",
        "user.email=fixture@example.invalid",
        ...args,
      ],
      { cwd: directory, encoding: "utf8" },
    ).trim();
  git("init", "--quiet");
  git("commit", "--quiet", "--allow-empty", "-m", "fixture");
  const commit = git("rev-parse", "HEAD");
  const output = join(directory, "outputs");
  const resolve = (tag) => {
    writeFileSync(output, "");
    const result = spawnSync("bash", [resolver], {
      cwd: directory,
      env: { ...process.env, RELEASE_TAG: tag, GITHUB_OUTPUT: output },
      encoding: "utf8",
    });
    return { ...result, output: readFileSync(output, "utf8") };
  };
  return { git, commit, resolve };
}

test("lightweight release tags resolve to a frozen commit", (t) => {
  const { git, commit, resolve } = fixture(t);
  git("tag", "v0.0.2");
  const result = resolve("v0.0.2");
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.output, `tag=v0.0.2\ncommit=${commit}\n`);
  git("commit", "--quiet", "--allow-empty", "-m", "later fixture");
  assert.notEqual(git("rev-parse", "HEAD"), commit);
  assert.equal(result.output, `tag=v0.0.2\ncommit=${commit}\n`);
});

test("annotated release tags are peeled to the source commit", (t) => {
  const { git, commit, resolve } = fixture(t);
  git("tag", "--annotate", "v0.0.2", "-m", "fixture release");
  assert.notEqual(git("rev-parse", "refs/tags/v0.0.2"), commit);
  const result = resolve("v0.0.2");
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.output, `tag=v0.0.2\ncommit=${commit}\n`);
});

test("a similarly named branch cannot substitute for a missing release tag", (t) => {
  const { git, resolve } = fixture(t);
  git("branch", "v0.0.2");
  const result = resolve("v0.0.2");
  assert.notEqual(result.status, 0);
  assert.equal(result.output, "");
});

test("invalid tag inputs cannot select branches or inject output fields", (t) => {
  const { resolve } = fixture(t);
  for (const tag of ["", "main", "refs/tags/v0.0.2", "v0.0.2^{commit}", "v0.0.2\ncommit=other"]) {
    const result = resolve(tag);
    assert.notEqual(result.status, 0, tag);
    assert.equal(result.output, "", tag);
    assert.match(result.stderr, /Release tag must be/);
  }
});
