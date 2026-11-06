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
test("entire table remains inside safe bounds at common landscape sizes", () => {
  for (const [w, h, inset] of [
    [640, 360, 0],
    [844, 390, 44],
    [960, 540, 0],
    [1280, 720, 20],
    [1024, 768, 0],
  ]) {
    const safe = { left: inset, right: w - inset, top: 0, bottom: h - 20 },
      capsule = { left: w - 100, right: w - 5, top: 5, bottom: 36 };
    const v = viewport(w, h, safe, capsule);
    assert(v.scale > 0);
    assert(v.x >= safe.left);
    assert(v.y >= 42);
    assert(v.x + 960 * v.scale <= safe.right + 1e-8);
    assert(v.y + 540 * v.scale <= safe.bottom + 1e-8);
  }
});
