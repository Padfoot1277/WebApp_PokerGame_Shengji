import type { Card, ViewState } from "../types/view";

export function orderedHandGroups(
  groups: Card[][] | null | undefined,
  trump: ViewState["trump"] | undefined,
): Card[][] {
  const suitKey = (suit: string) => {
    const keys: Record<string, string> = {
      红桃: "H",
      黑桃: "S",
      方块: "D",
      梅花: "C",
      主牌: "trump",
      "♥": "H",
      "♠": "S",
      "♦": "D",
      "♣": "C",
    };
    return keys[suit.replace(/\uFE0F/g, "")] ?? suit;
  };
  const trumpSuit = trump?.hasTrumpSuit ? suitKey(trump.suit ?? "") : "";
  const order =
    trumpSuit === "S"
      ? ["trump", "H", "C", "D"]
      : trumpSuit === "D"
        ? ["trump", "S", "H", "C"]
        : ["trump", "H", "S", "D", "C"];
  const priority = (group: Card[]) => {
    const card = group[0]!;
    const key =
      card.suit_class === "主牌" || isTrumpCard(card, trump)
        ? "trump"
        : suitKey(card.suit_class ?? card.suit);
    const index = order.indexOf(key);
    return index < 0 ? order.length : index;
  };
  return (groups ?? [])
    .filter((group) => group.length > 0)
    .sort((a, b) => priority(a) - priority(b));
}

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
