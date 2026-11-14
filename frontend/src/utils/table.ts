import type { Move, ViewState } from "../types/view";
// Row-major cells; clockwise around the grid follows the real seat indices.
export const lobbySeatOrder = [0, 1, 3, 2] as const;
export const seatOffsets = [2, 3, 1, 0] as const;
export function leaderSeats(s: ViewState, review = false): number[] {
  if (["dealing", "call_trump"].includes(s.phase)) {
    return s.callMode === "race" ? [0, 1, 2, 3] : [s.starterSeat];
  }
  if (["bottom", "trump_fight"].includes(s.phase)) return [s.starterSeat];
  if (["play_trick", "round_settle", "game_over"].includes(s.phase)) {
    const seat = showingLast(s, review)
      ? s.trick.lastLeaderSeat
      : s.trick.leaderSeat;
    return seat === undefined ? [] : [seat];
  }
  return [];
}
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
  const labels: Record<string, string> = {
    主牌: "主",
    方块: "副",
    红桃: "副",
    梅花: "副",
    黑桃: "副",
    垫牌: "杂",
  };
  return labels[label] ? { label: labels[label] } : null;
}
