export const seatName = (seat: number) =>
  ["东席", "南席", "西席", "北席"][seat] ?? "未入座";
export function relativeSeat(seat: number, mine: number) {
  if (mine < 0) return seatName(seat);
  return (
    ["我", "下家", "对家", "上家"][(seat - mine + 4) % 4] ?? seatName(seat)
  );
}
export const teamName = (team: number) => (team === 0 ? "东西队" : "南北队");
