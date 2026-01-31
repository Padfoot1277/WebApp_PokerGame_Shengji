package rules

import "fmt"

func isRedSuit(s Suit) bool   { return s == Heart || s == Diamond }
func isBlackSuit(s Suit) bool { return s == Spade || s == Club }

// 定主条件：
// 红主：红大王 + 红色级牌（♥/♦）
// 黑主：黑小王 + 黑色级牌（♠/♣）
func ValidateCallTrump(teamLevel Rank, joker Card, levelCards []Card) (trumpSuit Suit, locked bool, err error) {
	if len(levelCards) != 1 && len(levelCards) != 2 {
		return "", false, fmt.Errorf("级牌数量有误")
	}

	// 级牌必须是本队级牌
	for _, lc := range levelCards {
		if lc.Kind != KindNormal || lc.Rank != teamLevel {
			return "", false, fmt.Errorf("级牌选择有误，应为%s", string(teamLevel))
		}
	}

	// 级牌花色必须一致（定主花色由级牌决定）
	trumpSuit = levelCards[0].Suit
	for _, lc := range levelCards[1:] {
		if lc.Suit != trumpSuit {
			return "", false, fmt.Errorf("级牌花色不一致")
		}
	}

	// Joker + color constraint
	if joker.Kind == KindJokerBig && !isRedSuit(trumpSuit) {
		return "", false, fmt.Errorf("王和级牌颜色不一致")
	}
	if joker.Kind == KindJokerSmall && !isBlackSuit(trumpSuit) {
		return "", false, fmt.Errorf("王和级牌颜色不一致")
	}

	// 锁主：同色王 + 一对级牌（len==2 且同花色自然成立）
	if len(levelCards) == 2 {
		locked = true
	}
	return trumpSuit, locked, nil
}
