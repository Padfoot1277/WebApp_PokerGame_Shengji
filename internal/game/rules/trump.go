package rules

import "fmt"

// ValidateCallTrump 定主条件：
// 红主：红大王 + 红色级牌（♥/♦）
// 黑主：黑小王 + 黑色级牌（♠/♣）
func ValidateCallTrump(teamLevel Rank, joker Card, levelCards []Card) (trumpSuit Suit, locked bool, err error) {
	// 级牌必须是本队级牌
	for _, lc := range levelCards {
		if !isNormal(lc) || lc.Rank != teamLevel {
			return "", false, fmt.Errorf("级牌选择有误，应为%s", string(teamLevel))
		}
	}
	// 级牌花色必须一致（定主花色由级牌决定）
	trumpSuit = levelCards[0].Suit
	if len(levelCards) == 2 && levelCards[1].Suit != trumpSuit {
		return "", false, fmt.Errorf("级牌花色不一致")
	}
	// Joker + color constraint
	if IsBigJoker(joker) && !isRedSuit(trumpSuit) {
		return "", false, fmt.Errorf("王和级牌颜色不一致")
	}
	if IsSmallJoker(joker) && !isBlackSuit(trumpSuit) {
		return "", false, fmt.Errorf("王和级牌颜色不一致")
	}
	// 锁主：同色王 + 一对级牌（len==2 且同花色自然成立）
	if len(levelCards) == 2 {
		locked = true
	}
	return trumpSuit, locked, nil
}

func ValidateChangeTrump(LevelRank Rank, joker Card, c1 Card, c2 Card) (trumpSuit Suit, err error) {
	if !isNormal(c1) || !isNormal(c2) {
		return "", fmt.Errorf("改主牌非法")
	}
	if c1.Rank != LevelRank || c2.Rank != LevelRank {
		return "", fmt.Errorf("改主牌非法")
	}
	if c1.Suit != c2.Suit {
		return "", fmt.Errorf("改主牌非法")
	}
	if !IsBigJoker(joker) && !IsSmallJoker(joker) {
		return "", fmt.Errorf("改主牌非法")
	}
	if IsBigJoker(joker) && !(c1.Suit == Heart || c1.Suit == Diamond) {
		return "", fmt.Errorf("改主牌非法")
	}
	if IsSmallJoker(joker) && !(c1.Suit == Spade || c1.Suit == Club) {
		return "", fmt.Errorf("改主牌非法")
	}
	return c1.Suit, nil
}

func ValidateAttackTrump(j1 Card, j2 Card) (err error) {
	if j1.Suit != j2.Suit || (!IsBigJoker(j1) && !IsSmallJoker(j1)) {
		return fmt.Errorf("攻主牌非法")
	}
	return nil
}
