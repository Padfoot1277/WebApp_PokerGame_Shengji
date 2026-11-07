import { test } from "node:test";
import assert from "node:assert/strict";
import { buildSync } from "esbuild";
import vm from "node:vm";
import { fileURLToPath } from "node:url";
function source(name) {
  const module = { exports: {} };
  vm.runInNewContext(
    buildSync({
      entryPoints: [
        fileURLToPath(new URL(`../src/${name}.ts`, import.meta.url)),
      ],
      bundle: true,
      platform: "node",
      format: "cjs",
      write: false,
    }).outputFiles[0].text,
    { module, exports: module.exports },
  );
  return module.exports;
}
const { Motion } = source("motion"),
  { readPreferences } = source("preferences");
test("preferences default enabled and tolerate damaged or old storage", () => {
  for (const value of [null, "true", 1, { sound: "true" }]) {
    const p = readPreferences(value);
    assert.equal(p.sound, true);
    assert.equal(p.vibration, true);
    assert.equal(p.reducedMotion, false);
  }
  assert.equal(readPreferences({ sound: false, vibration: false }).sound, false);
  assert.equal(readPreferences({ sound: false, vibration: false }).vibration, false);
  assert.equal(
    readPreferences({ sound: true, reducedMotion: true }).reducedMotion,
    true,
  );
});
test("motion has one bounded frame job and no idle loop", () => {
  let now = 0,
    draws = 0,
    id = 0;
  const jobs = new Map();
  const m = new Motion(
    () => draws++,
    () => now,
    (f) => {
      jobs.set(++id, f);
      return id;
    },
    (i) => jobs.delete(i),
  );
  for (let i = 0; i < 100; i++) m.start(`select:${i}`, 5000);
  assert.equal(jobs.size, 1);
  assert.equal(m.active("select:0"), false);
  now = 1200;
  const [key, job] = [...jobs][0];
  jobs.delete(key);
  job();
  assert.equal(draws, 1);
  assert.equal(jobs.size, 0);
  m.start("deal", 400);
  m.suspend();
  assert.equal(jobs.size, 0);
  assert.equal(m.progress("deal"), 1);
  m.start("hidden", 400);
  m.resume();
  assert.equal(jobs.size, 0);
  m.start("turn", 500);
  m.setEnabled(false);
  assert.equal(jobs.size, 0);
  m.start("reduced", 500);
  assert.equal(jobs.size, 0);
  m.setEnabled(true);
  assert.equal(jobs.size, 0);
});
