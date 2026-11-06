const suits = ["♠️", "♥️", "♣️", "♦️"],
  ranks = ["A", "K", "Q", "J", "10", "9", "8", "7", "6", "5", "4", "3", "2"];
export const deck = Array.from({ length: 108 }, (_, id) => {
  const n = id % 54,
    suit = n >= 52 ? (n === 52 ? "SJ" : "BJ") : suits[Math.floor(n / 13)],
    rank = n >= 52 ? (n === 52 ? "小王" : "大王") : ranks[n % 13];
  return {
    id,
    suit,
    rank,
    suit_class:
      n >= 52 || rank === "2" || suit === "♥️"
        ? "主"
        : suit === suits[0]
          ? "黑桃"
          : suit === suits[2]
            ? "梅花"
            : "方块",
  };
});
export function scene(phase = "play_trick", overrides = {}) {
  const hand = [...deck.slice(0, 13), ...deck.slice(54, 66)];
  const s = {
    roomId: "F8K2Q6",
    phase,
    version: 1,
    seats: ["春风", "夏雨", "秋月", "冬雪"].map((name, i) => ({
      uid: `qa${i}`,
      name,
      online: true,
      ready: true,
      team: i % 2,
      handCount: 25,
    })),
    teams: [{ levelRank: "4" }, { levelRank: "3" }],
    roundIndex: 2,
    callMode: "ordered",
    callTurnSeat: 0,
    callPassedSeats: [false, false, true, false],
    callPassCount: 1,
    starterSeat: 0,
    fightTurnSeat: 0,
    fightPassedSeats: [false, false, true, false],
    fightPassCount: 1,
    bottomOwnerSeat: 0,
    trickIndex: 6,
    trump: {
      hasTrumpSuit: true,
      suit: "♥️",
      levelRank: "4",
      locked: false,
      callerSeat: 0,
    },
    trick: {
      leaderSeat: 0,
      turnSeat: 0,
      playedMoves: [null, null, null, null],
      lastMoves: [null, null, null, null],
      biggerSeat: 1,
      resolved: false,
      winnerSeat: 1,
    },
    points: 65,
    hideRecord: false,
    record: {
      spade: { a: 1, k: 0, ten: 1, num: 8 },
      heart: { a: 0, k: 1, ten: 1, num: 6 },
      club: { a: 1, k: 1, ten: 0, num: 7 },
      diamond: { a: 0, k: 0, ten: 0, num: 4 },
      bigJoker: 0,
      smallJoker: 1,
    },
    bottomRevealed: false,
    bottomReveal: [],
    bottomPoints: 20,
    bottomMul: 2,
    bottomAward: 40,
    roundPointsFinal: 125,
    roundResultLabel: "过大关",
    callerDelta: 0,
    defenderDelta: 1,
    nextStarterSeat: 0,
    mySeat: 0,
    myHand: [hand],
    myBottom: deck.slice(25, 33),
    usedTrumpIds: [12, 53],
    surrenderVotes: [false, false, false, false],
    surrendered: false,
    winnerTeam: -1,
    ...overrides,
  };
  if (phase === "bottom") {
    s.myHand = [[...hand, ...deck.slice(25, 33)]];
    s.seats[0].handCount = 33;
  }
  if (phase === "lobby") {
    s.myHand = null;
    s.roundIndex = 0;
    s.points = 0;
    s.teams = [{ levelRank: "2" }, { levelRank: "2" }];
    s.seats = s.seats.map((p, i) => ({
      ...p,
      handCount: 0,
      uid: i < 2 ? p.uid : "",
      name: i < 2 ? p.name : "",
      ready: i === 1,
    }));
  }
  if (["round_settle", "game_over"].includes(phase)) {
    s.myHand = null;
    s.points = s.roundPointsFinal;
    s.bottomRevealed = true;
    s.bottomReveal = deck.slice(25, 33);
    s.seats = s.seats.map((p) => ({ ...p, handCount: 0 }));
  }
  if (phase === "game_over" && s.winnerTeam >= 0)
    s.teams[s.winnerTeam].levelRank = "A";
  return s;
}
