import { test } from "node:test";
import assert from "node:assert/strict";
import { buildSync } from "esbuild";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
const module = { exports: {} };
vm.runInNewContext(
  buildSync({
    entryPoints: [
      fileURLToPath(new URL("../src/viewport.ts", import.meta.url)),
    ],
    bundle: true,
    format: "cjs",
    write: false,
  }).outputFiles[0].text,
  { module, exports: module.exports },
);
const { viewport } = module.exports;
test("portrait column respects safe areas and capsule on phones and tablets", () => {
  for (const [w, h, inset] of [
    [320, 568, 0],
    [375, 667, 0],
    [390, 844, 0],
    [430, 932, 0],
    [768, 1024, 0],
    [844, 390, 44],
  ]) {
    const safe = { left: inset, right: w - inset, top: 0, bottom: h - 20 },
      capsule = { left: w - 100, right: w - 5, top: 5, bottom: 36 };
    const v = viewport(w, h, safe, capsule);
    assert(v.scale > 0);
    assert.equal(v.width, 390);
    assert(v.height >= 640 - 1e-8);
    assert(v.x >= safe.left);
    assert(v.y >= safe.top);
    assert(v.y >= capsule.bottom);
    assert(v.x + v.width * v.scale <= safe.right + 1e-8);
    assert(v.y + v.height * v.scale <= safe.bottom + 1e-8);
  }
});
