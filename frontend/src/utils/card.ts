export function suitToSymbol(suit: string): { symbol: string; color: 'red' | 'black' | 'joker' } {
    switch (suit) {
        case 'S':
            return { symbol: '♠️', color: 'black' }
        case 'C':
            return { symbol: '♣️', color: 'black' }
        case 'H':
            return { symbol: '♥️', color: 'red' }
        case 'D':
            return { symbol: '♦️', color: 'red' }
        case 'SJ':
            return { symbol: '🃏', color: 'joker' }
        case 'BJ':
            return { symbol: '👑', color: 'joker' }
        case '小王':
            return { symbol: '🃏', color: 'joker' }
        case '大王':
            return { symbol: '👑', color: 'joker' }
        default:
            return { symbol: suit, color: 'black' }
    }
}
