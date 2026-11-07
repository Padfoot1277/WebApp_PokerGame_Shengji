import type { Move, ViewState } from "../types/view";
export function showingLast(s: ViewState, review = false) {
  return (
    review ||
    ["round_settle", "game_over"].includes(s.phase) ||
    (s.trick.resolved && !s.trick.playedMoves.some(Boolean))
  );
}
export function moveTag(move: Move | null | undefined) {
  if (!move) return null;
  const label =
    move.info === "垫牌" || ["杂牌", "Mix"].includes(move.suitClass)
      ? "垫牌"
      : move.suitClass;
  const icons: Record<string, string> = {
    主牌: "🃏",
    方块: "♦️",
    红桃: "♥️",
    梅花: "♣️",
    黑桃: "♠️",
    垫牌: "🍂",
  };
  return icons[label] ? { label, icon: icons[label] } : null;
}
