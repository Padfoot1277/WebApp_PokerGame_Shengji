import type { ViewState } from "../../frontend/src/types/view";
export const seatName = (seat: number) =>
  ["东席", "南席", "西席", "北席"][seat] ?? "未入座";
export const relative = (seat: number, mine: number) =>
  mine < 0
    ? seatName(seat)
    : ["我", "下家", "对家", "上家"][(seat - mine + 4) % 4];
export function seatRole(s: ViewState, seat: number): string {
  const own =
    s.mySeat >= 0
      ? seat % 2 === s.mySeat % 2
        ? "我方"
        : "对方"
      : seat % 2 === 0
        ? "东西队"
        : "南北队";
  if (["lobby", "dealing", "call_trump"].includes(s.phase)) return own;
  const caller = s.trump.callerSeat >= 0 ? s.trump.callerSeat : s.starterSeat;
  return `${own} · ${seat % 2 === caller % 2 ? "坐家" : "打家"}`;
}
export function turn(s: ViewState) {
  switch (s.phase) {
    case "call_trump":
      return s.callTurnSeat;
    case "bottom":
      return s.bottomOwnerSeat;
    case "trump_fight":
      return s.fightTurnSeat;
    case "play_trick":
      return s.trick.turnSeat;
    default:
      return -1;
  }
}
export function guide(s: ViewState): string {
  if (s.mySeat < 0) return "正在观战 · 仅展示公开出牌";
  const name = (i: number) =>
    `${relative(i, s.mySeat)}（${s.seats[i]?.name || seatName(i)}）`;
  switch (s.phase) {
    case "call_trump":
      return s.callPassedSeats[s.mySeat]
        ? "已跳过，等待其他人定主"
        : s.callMode === "race" || turn(s) === s.mySeat
          ? `定主：一张同色王 + 本队级牌 ${s.teams[s.mySeat % 2]?.levelRank}；一对级牌可锁主`
          : `等待${name(turn(s))}定主`;
    case "bottom":
      return turn(s) === s.mySeat
        ? "从 33 张手牌中选 8 张扣底，其余留作手牌"
        : `等待${name(turn(s))}扣底`;
    case "trump_fight":
      return turn(s) === s.mySeat
        ? "轮到你攻改：王加一对级牌改主，一对同类王攻主，或跳过"
        : `等待${name(turn(s))}攻改`;
    case "play_trick": {
      const lead = s.trick.playedMoves[s.trick.leaderSeat];
      return turn(s) !== s.mySeat
        ? `等待${name(turn(s))}出牌`
        : lead
          ? `请跟 ${lead.followMove.cards.length} 张${lead.suitClass}，尽量满足对子 / 拖拉机结构`
          : "轮到你先手：可出单张、对子、拖拉机或甩牌";
    }
    case "round_settle":
      return `本局 ${s.roundPointsFinal} 分 · ${s.roundResultLabel} · 坐家 +${s.callerDelta} / 打家 +${s.defenderDelta} · ${name(s.nextStarterSeat)}开下一局`;
    case "game_over":
      return `${s.winnerTeam === 0 ? "东西队" : "南北队"}升到 A，整局获胜！可返回大厅创建新房间`;
    default:
      return "选择座位，四人准备后自动开始";
  }
}
