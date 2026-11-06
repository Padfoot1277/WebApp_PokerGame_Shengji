import { test } from "node:test";
import assert from "node:assert/strict";
import { buildSync } from "esbuild";
import vm from "node:vm";
import { fileURLToPath } from "node:url";
const module = { exports: {} };
vm.runInNewContext(
  buildSync({
    entryPoints: [
      fileURLToPath(new URL("../src/selection.ts", import.meta.url)),
    ],
    bundle: true,
    platform: "node",
    format: "cjs",
    write: false,
  }).outputFiles[0].text,
  { module, exports: module.exports },
);
const { selectionError: error, selectionSummary: summary } = module.exports;
const card = (id, suit, rank, sc = "主") => ({
  id,
  suit,
  rank,
  suit_class: sc,
});
const base = () => ({
  phase: "call_trump",
  mySeat: 0,
  teams: [{ levelRank: "2" }, { levelRank: "3" }],
  trump: { levelRank: "4" },
  usedTrumpIds: [],
  myHand: [],
  trick: { leaderSeat: 0, playedMoves: [null, null, null, null] },
});
test("trump selection explains wrong rank, color, pair and reused cards", () => {
  const s = base(),
    j = card(1, "BJ", "大王"),
    red = card(2, "♥️", "2"),
    black = card(3, "♠️", "2");
  assert.equal(error("game.call_trump", [j, red], s), null);
  assert.match(error("game.call_trump", [j, black], s), /颜色|红色/);
  assert.match(error("game.call_trump", [j, card(4, "♥️", "3")], s), /2/);
  assert.match(
    error("game.call_trump", [j, red, card(5, "♦️", "2")], s),
    /同一花色/,
  );
  s.usedTrumpIds = [1];
  assert.match(error("game.call_trump", [j, red], s), /亮过/);
  assert.match(
    error("game.attack_trump", [j, card(7, "SJ", "小王")], base()),
    /一对/,
  );
  assert.equal(
    error("game.attack_trump", [j, card(8, "BJ", "大王")], base()),
    null,
  );
  assert.equal(
    error(
      "game.change_trump",
      [j, card(9, "♦️", "4"), card(10, "♦️", "4")],
      base(),
    ),
    null,
  );
});
test("follow count and suit must be satisfied before sending; server owns shape rules", () => {
  const s = base(),
    h = card(1, "♥️", "A", "红桃"),
    other = card(2, "♠️", "K", "黑桃");
  s.myHand = [[h, other]];
  s.trick.playedMoves[0] = { suitClass: "红桃", followMove: { cards: [h] } };
  assert.match(error("game.play_cards", [], s), /先选择/);
  assert.match(error("game.play_cards", [h, other], s), /需要跟 1/);
  assert.match(error("game.play_cards", [other], s), /跟足/);
  assert.equal(error("game.play_cards", [h], s), null);
  s.myHand = [[other]];
  assert.equal(error("game.play_cards", [other], s), null);
  s.trick.playedMoves[0] = null;
  assert.match(error("game.play_cards", [h, other], s), /同一牌域/);
});
test("burying requires eight cards and previews point cost", () => {
  const s = base();
  s.phase = "bottom";
  const cards = Array.from({ length: 8 }, (_, i) =>
    card(i, "♠️", i === 0 ? "K" : "2"),
  );
  assert.equal(error("game.put_bottom", cards, s), null);
  assert.match(error("game.put_bottom", cards.slice(1), s), /7/);
  assert.match(summary(cards, s), /8 \/ 8 张 · 10 分/);
});
