export const seatName = (seat: number) =>
  ["座位 A1", "座位 B1", "座位 A2", "座位 B2"][seat] ?? "未入座";
export function relativeSeat(seat: number, mine: number) {
  if (mine < 0) return seatName(seat);
  return (
    ["我", "下家", "对家", "上家"][(seat - mine + 4) % 4] ?? seatName(seat)
  );
}
export const teamName = (team: number) => (team === 0 ? "A组" : "B组");
