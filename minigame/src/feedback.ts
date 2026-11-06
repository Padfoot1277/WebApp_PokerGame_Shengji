import type { Move, ViewState } from "../../frontend/src/types/view";
import { turn } from "./model";

export type Feedback =
  | { kind: "hand"; ids: number[] }
  | { kind: "move" | "ready"; seat: number }
  | { kind: "turn" | "trick" | "celebrate" };
const moveKey = (move: Move | null | undefined) =>
  move?.followMove.cards
    .map((c) => c.id)
    .sort((a, b) => a - b)
    .join(",") ?? "";
const ended = (s: ViewState) =>
  s.phase === "round_settle" || s.phase === "game_over";
function myAction(s: ViewState): string {
  if (s.mySeat < 0 || turn(s) !== s.mySeat) return "";
  if (
    s.phase === "call_trump" &&
    (s.callMode === "race" || s.callPassedSeats[s.mySeat])
  )
    return "";
  if (s.phase === "trump_fight" && s.fightPassedSeats[s.mySeat]) return "";
  return `${s.roundIndex}:${s.phase}:${s.trickIndex}`;
}
export function myUpgrade(s: ViewState) {
  if (s.mySeat < 0) return 0;
  const caller = s.trump.callerSeat >= 0 ? s.trump.callerSeat : s.starterSeat;
  return s.mySeat % 2 === caller % 2 ? s.callerDelta : s.defenderDelta;
}
export function lastTrickSummary(s: ViewState): string {
  if (!s.trick.lastMoves.every(Boolean) || s.trick.winnerSeat < 0) return "";
  const points = s.trick.lastMoves
    .flatMap((m) => m!.followMove.cards)
    .reduce(
      (total, c) =>
        total +
        (c.rank === "5" ? 5 : c.rank === "10" || c.rank === "K" ? 10 : 0),
      0,
    );
  if (!points) return "本墩无分牌";
  const caller = s.trump.callerSeat >= 0 ? s.trump.callerSeat : s.starterSeat;
  return s.trick.winnerSeat % 2 === caller % 2
    ? `坐家守住 ${points} 分`
    : `打家收得 ${points} 分`;
}
/** Compare authoritative snapshots, not clicks/acks/draw calls. Reset on every hello. */
export class FeedbackTracker {
  private previous: ViewState | null = null;
  reset() {
    this.previous = null;
  }
  observe(next: ViewState): Feedback[] {
    const before = this.previous;
    if (
      before &&
      next.roomId === before.roomId &&
      next.version < before.version
    )
      return [];
    this.previous = next;
    if (
      !before ||
      before.roomId !== next.roomId ||
      before.mySeat !== next.mySeat
    )
      return [];
    const events: Feedback[] = [];
    const sameRound = before.roundIndex === next.roundIndex;
    if (next.roundIndex < before.roundIndex) return [];
    const oldHand = new Set((before.myHand ?? []).flat().map((c) => c.id));
    const added = (next.myHand ?? [])
      .flat()
      .filter((c) => !sameRound || !oldHand.has(c.id))
      .map((c) => c.id);
    if (added.length) events.push({ kind: "hand", ids: added });
    if (next.phase === "lobby" && before.phase === "lobby")
      next.seats.forEach((p, seat) => {
        const old = before.seats[seat];
        if (p.uid && (p.uid !== old?.uid || (p.ready && !old?.ready)))
          events.push({ kind: "ready", seat });
      });
    if (sameRound) {
      // The fourth move is delivered in lastMoves when the server settles a trick.
      const moves = next.trick.resolved
        ? next.trick.lastMoves
        : next.trick.playedMoves;
      moves.forEach((move, seat) => {
        const key = moveKey(move);
        if (
          key &&
          key !== moveKey(before.trick.playedMoves[seat]) &&
          key !== moveKey(before.trick.lastMoves[seat])
        )
          events.push({ kind: "move", seat });
      });
      if (
        next.trick.resolved &&
        next.trick.lastMoves.every(Boolean) &&
        next.trick.lastMoves.map(moveKey).join("|") !==
          before.trick.lastMoves.map(moveKey).join("|")
      )
        events.push({ kind: "trick" });
    }
    if (myAction(next) && myAction(next) !== myAction(before))
      events.push({ kind: "turn" });
    if (
      ended(next) &&
      (!sameRound || !ended(before)) &&
      !next.surrendered &&
      next.mySeat >= 0 &&
      (next.phase === "game_over"
        ? next.mySeat % 2 === next.winnerTeam
        : myUpgrade(next) > 0)
    )
      events.push({ kind: "celebrate" });
    return events;
  }
}
