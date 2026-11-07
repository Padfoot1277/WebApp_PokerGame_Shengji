import type { Card, ViewState } from "../types/view";
export function unavailableCard(
  s: ViewState | null,
  card: Card | undefined,
  selected: number[] = [],
): boolean {
  if (!s || !card || s.mySeat < 0) return true;
  const hand = (s.myHand ?? []).flat();
  const joker = card.suit === "BJ" || card.suit === "SJ";
  if (s.phase === "bottom") return s.bottomOwnerSeat !== s.mySeat;
  if (s.phase === "call_trump" || s.phase === "trump_fight") {
    if (
      s.phase === "call_trump" &&
      (s.callPassedSeats[s.mySeat] ||
        (s.callMode !== "race" && s.callTurnSeat !== s.mySeat))
    )
      return true;
    if (s.phase === "trump_fight" && s.fightTurnSeat !== s.mySeat) return true;
    if (s.usedTrumpIds.includes(card.id)) return true;
    if (joker) return false;
    if (s.phase === "trump_fight" && s.trump.locked) return true;
    return (
      card.rank !==
      (s.phase === "call_trump"
        ? s.teams[s.mySeat % 2]?.levelRank
        : s.trump.levelRank)
    );
  }
  if (s.phase !== "play_trick" || s.trick.turnSeat !== s.mySeat) return true;
  const lead = s.trick.playedMoves[s.trick.leaderSeat];
  if (lead) {
    const matching = hand.filter((c) => c.suit_class === lead.suitClass).length;
    return (
      matching >= lead.followMove.cards.length &&
      card.suit_class !== lead.suitClass
    );
  }
  const first = hand.find((c) => selected.includes(c.id));
  return (
    !!first &&
    first.suit_class !== card.suit_class &&
    !selected.includes(card.id)
  );
}
export function floatingFeedback(
  before: ViewState | null,
  next: ViewState,
): string[] {
  if (
    !before ||
    before.roomId !== next.roomId ||
    before.roundIndex !== next.roundIndex ||
    next.version <= before.version
  )
    return [];
  const messages: string[] = [];
  if (next.points > before.points)
    messages.push(`打家 +${next.points - before.points} 分`);
  const a = before.trump,
    b = next.trump;
  if (
    next.phase !== "call_trump" &&
    (a.hasTrumpSuit !== b.hasTrumpSuit ||
      a.suit !== b.suit ||
      a.locked !== b.locked ||
      before.bottomOwnerSeat !== next.bottomOwnerSeat)
  ) {
    const action =
      before.phase === "call_trump"
        ? "定主"
        : !b.hasTrumpSuit
          ? "攻主"
          : "改主";
    messages.push(
      `${action} · ${b.hasTrumpSuit ? b.suit : "硬主"}${b.locked && b.hasTrumpSuit ? " · 锁主" : ""}`,
    );
  } else if (before.phase === "bottom" && next.phase !== "bottom")
    messages.push("扣底完成");
  if (next.bottomRevealed && !before.bottomRevealed && next.bottomAward > 0)
    messages.push(
      `末墩抠底 · ${next.bottomPoints} × ${next.bottomMul} = ${next.bottomAward} 分`,
    );
  if (
    ["round_settle", "game_over"].includes(next.phase) &&
    !["round_settle", "game_over"].includes(before.phase)
  ) {
    messages.push(
      next.phase === "game_over"
        ? "升 A 获胜 · 整场结束"
        : `本局结束 · ${next.roundResultLabel}`,
    );
  }
  const moves = next.trick.resolved
    ? next.trick.lastMoves
    : next.trick.playedMoves;
  if (
    moves.some(
      (m, seat) =>
        m?.info?.startsWith("甩牌被阻止") &&
        m.followMove.cards.map((c) => c.id).join() !==
          (
            before.trick.playedMoves[seat] ?? before.trick.lastMoves[seat]
          )?.followMove.cards
            .map((c) => c.id)
            .join(),
    )
  )
    messages.push("甩牌被阻止，改出最低牌力组合");
  return messages.slice(0, 2);
}
