export type Phase =
  | "lobby"
  | "dealing"
  | "call_trump"
  | "bottom"
  | "trump_fight"
  | "play_trick"
  | "round_settle"
  | "game_over";
export interface Card {
  id: number;
  suit: string;
  rank: string;
  suit_class?: string;
}
export interface Seat {
  uid: string;
  name: string;
  online: boolean;
  ready: boolean;
  team: number;
  handCount: number;
}
export interface Block {
  type: string;
  suitClass: string;
  ranks: number;
  tractor_len: number;
  cards: Card[];
}
export interface Move {
  seat: number;
  suitClass: string;
  followMove: { blocks: Block[][] | null; cards: Card[]; actualIds: number[] };
  info: string;
}
export interface ViewState {
  roomId: string;
  phase: Phase;
  version: number;
  seats: Seat[];
  teams: { levelRank: string }[];
  roundIndex: number;
  callMode: "race" | "ordered";
  callPassedSeats: boolean[];
  starterSeat: number;
  callTurnSeat: number;
  callPassCount: number;
  fightPassedSeats: boolean[];
  fightPassCount: number;
  fightTurnSeat: number;
  bottomOwnerSeat: number;
  trickIndex: number;
  trump: {
    hasTrumpSuit: boolean;
    suit?: string;
    levelRank: string;
    locked: boolean;
    callerSeat: number;
  };
  trick: {
    leaderSeat: number;
    lastLeaderSeat?: number;
    turnSeat: number;
    playedMoves: (Move | null)[];
    lastMoves: (Move | null)[];
    biggerSeat: number;
    resolved: boolean;
    winnerSeat: number;
  };
  points: number;
  hideRecord: boolean;
  record: {
    spade: SuitRecord;
    heart: SuitRecord;
    club: SuitRecord;
    diamond: SuitRecord;
    bigJoker: number;
    smallJoker: number;
  };
  bottomRevealed: boolean;
  bottomReveal?: Card[];
  bottomPoints: number;
  bottomMul: number;
  bottomAward: number;
  roundPointsFinal: number;
  roundResultLabel: string;
  callerDelta: number;
  defenderDelta: number;
  nextStarterSeat: number;
  mySeat: number;
  myHand: Card[][] | null;
  myBottom: Card[] | null;
  usedTrumpIds: number[];
  surrenderVotes: boolean[];
  surrendered: boolean;
  winnerTeam: number;
}
interface SuitRecord {
  a: number;
  k: number;
  ten: number;
  num: number;
}
