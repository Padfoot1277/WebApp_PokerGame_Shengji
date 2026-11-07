import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import ts from "typescript";
function load(source, extras = {}) {
  const module = { exports: {} };
  const context = vm.createContext({ module, exports: module.exports, ...extras });
  vm.runInContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, context);
  return context;
}
const table = load(readFileSync(new URL("../src/utils/table.ts", import.meta.url), "utf8")).module.exports;
test("settled cards remain until the next lead, with history available afterward", () => {
  const s = { phase: "play_trick", trick: { resolved: true, playedMoves: [null,null,null,null] } };
  assert.equal(table.showingLast(s), true);
  s.trick.resolved = false;
  s.trick.playedMoves[2] = { seat: 2 };
  assert.equal(table.showingLast(s), false);
  assert.equal(table.showingLast(s, true), true);
});
test("padding is one tag even for uniform off-suit cards", () => {
  for (const suitClass of ["杂牌", "Mix", "黑桃"]) {
    assert.equal(table.moveTag({ suitClass, info: "垫牌" }).label, "垫牌");
  }
  assert.equal(table.moveTag({ suitClass: "主牌", info: "" }).label, "主牌");
});
test("hand selection waits for release and scrolling cancels the whole gesture", () => {
  const props = { selectedIds: [] };
  const script = readFileSync(new URL("../src/components/HandArea.vue", import.meta.url), "utf8").match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1];
  const game = { view: null, reviewPrevious: false, pending: false };
  const ctx = load(script, {
    require: name => name === "vue" ? { computed: fn => ({ get value() { return fn(); } }), watch() {} } : name.includes("store") ? { useGameStore: () => game } : {},
    defineProps: () => props,
    defineEmits: () => (name, ids) => { if (name === "update:selectedIds") props.selectedIds = ids; },
  });
  const event = (x,y) => ({ isPrimary: true, button: 0, clientX:x, clientY:y, target: { closest: () => ({ dataset: { cardId: "7" } }) } });
  ctx.e = event(20,20); vm.runInContext("down(e)",ctx);
  assert.equal(props.selectedIds.length, 0);
  ctx.e = event(20,70); vm.runInContext("move(e); up(e)",ctx);
  assert.equal(props.selectedIds.length, 0);
  ctx.e = event(20,20); vm.runInContext("down(e); up(e)",ctx);
  assert.equal(JSON.stringify(props.selectedIds), "[7]");
});
