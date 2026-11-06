import type { Card, ViewState } from "../../frontend/src/types/view";

export const isJoker = (c: Card) => c.suit === "SJ" || c.suit === "BJ";
export function selectionError(
  type: string,
  cards: Card[],
  s: ViewState,
): string | null {
  if (type === "game.play_cards") {
    if (!cards.length) return "请先选择手牌";
    const lead = s.trick.playedMoves[s.trick.leaderSeat];
    if (!lead) {
      if (new Set(cards.map((c) => c.suit_class)).size > 1)
        return "先手出牌须来自同一牌域（主牌或同一副花色）";
      return null;
    }
    const required = lead.followMove.cards.length;
    if (cards.length !== required)
      return `本墩需要跟 ${required} 张，你选了 ${cards.length} 张`;
    const available = (s.myHand ?? [])
      .flat()
      .filter((c) => c.suit_class === lead.suitClass).length;
    const followed = cards.filter(
      (c) => c.suit_class === lead.suitClass,
    ).length;
    if (followed < Math.min(required, available))
      return `请优先跟足${lead.suitClass}，不足时须全部跟出`;
    // Structure legality remains authoritative on the server.
    return null;
  }
  if (type === "game.put_bottom")
    return cards.length === 8
      ? null
      : `扣底须选 8 张，当前已选 ${cards.length} 张`;
  if (cards.some((c) => s.usedTrumpIds.includes(c.id)))
    return "选中了本局已亮过的牌，请换用未亮过的实体牌";
  const jokers = cards.filter(isJoker),
    levels = cards.filter((c) => !isJoker(c));
  if (type === "game.attack_trump") {
    return cards.length === 2 &&
      jokers.length === 2 &&
      jokers[0]!.suit === jokers[1]!.suit
      ? null
      : "攻主须选一对大王或一对小王";
  }
  const call = type === "game.call_trump";
  if (jokers.length !== 1 || !(call ? [1, 2] : [2]).includes(levels.length))
    return call
      ? "定主须选一张王和一张级牌；同花色一对级牌可锁主"
      : "改主须选一张王和同花色一对本局级牌";
  const rank = call ? s.teams[s.mySeat % 2]!.levelRank : s.trump.levelRank;
  if (levels.some((c) => c.rank !== rank)) return `请选择 ${rank} 级牌`;
  if (levels.some((c) => c.suit !== levels[0]!.suit))
    return "两张级牌必须是同一花色";
  const red = /[♥♦]/.test(levels[0]!.suit);
  if (red !== (jokers[0]!.suit === "BJ"))
    return "大王配红色级牌，小王配黑色级牌";
  return null;
}

export function selectionSummary(cards: Card[], s: ViewState): string {
  const points = cards.reduce(
    (sum, c) =>
      sum + (c.rank === "5" ? 5 : ["10", "K"].includes(c.rank) ? 10 : 0),
    0,
  );
  if (s.phase === "bottom")
    return `扣底 ${cards.length} / 8 张 · ${points} 分${cards.length === 8 ? " · 可以确认" : ""}`;
  const lead =
    s.phase === "play_trick" ? s.trick.playedMoves[s.trick.leaderSeat] : null;
  return `已选 ${cards.length}${lead ? " / " + lead.followMove.cards.length : ""} 张 · ${points} 分`;
}
