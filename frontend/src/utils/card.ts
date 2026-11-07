export function suitToSymbol(suit: string): {
  symbol: string;
  color: "red" | "black" | "joker";
} {
  switch (suit) {
    case "♠️":
    case "S":
      return { symbol: "♠️", color: "black" };
    case "♣️":
    case "C":
      return { symbol: "♣️", color: "black" };
    case "♥️":
    case "H":
      return { symbol: "♥️", color: "red" };
    case "♦️":
    case "D":
      return { symbol: "♦️", color: "red" };
    case "SJ":
      return { symbol: "🎩", color: "joker" };
    case "BJ":
      return { symbol: "👑", color: "joker" };
    case "小王":
      return { symbol: "🎩", color: "joker" };
    case "大王":
      return { symbol: "👑", color: "joker" };
    default:
      return { symbol: suit, color: "black" };
  }
}

export function isTrumpCard(
  card: { suit: string; rank: string; suit_class?: string },
  trump:
    { levelRank: string; hasTrumpSuit: boolean; suit?: string } | undefined,
) {
  if (!trump || trump.levelRank === "Pending") return false;
  if (
    card.suit_class === "主牌" ||
    ["BJ", "SJ", "大王", "小王"].includes(card.suit) ||
    card.rank === trump.levelRank
  )
    return true;
  const suits: Record<string, string> = {
    黑桃: "S",
    红桃: "H",
    梅花: "C",
    方块: "D",
    "♠": "S",
    "♥": "H",
    "♣": "C",
    "♦": "D",
  };
  const key = (s: string) => suits[s.replace(/\uFE0F/g, "")] || s;
  return trump.hasTrumpSuit && key(card.suit) === key(trump.suit || "");
}
