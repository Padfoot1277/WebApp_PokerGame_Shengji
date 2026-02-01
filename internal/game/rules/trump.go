package rules

import "fmt"

// ValidateCallTrump 定主条件：
// 红主：红大王 + 红色级牌（♥/♦）
// 黑主：黑小王 + 黑色级牌（♠/♣）
func ValidateCallTrump(teamLevel Rank, joker Card, levelCards []Card) (trumpSuit Suit, locked bool, err error) {
	if len(levelCards) != 1 && len(levelCards) != 2 {
		return "", false, fmt.Errorf("级牌数量有误")
	}
	if len(levelCards) == 2 && levelCards[0].ID == levelCards[1].ID {
		return "", false, fmt.Errorf("重复选择相同级牌")
	}
	// 级牌必须是本队级牌
	for _, lc := range levelCards {
		if !isNormal(lc) || lc.Rank != teamLevel {
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
	if isBigJoker(joker) && !isRedSuit(trumpSuit) {
		return "", false, fmt.Errorf("王和级牌颜色不一致")
	}
	if isSmallJoker(joker) && !isBlackSuit(trumpSuit) {
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
	if c1.ID == c2.ID {
		return "", fmt.Errorf("重复选择相同级牌")
	}
	if c1.Rank != LevelRank || c2.Rank != LevelRank {
		return "", fmt.Errorf("改主牌非法")
	}
	if c1.Suit != c2.Suit {
		return "", fmt.Errorf("改主牌非法")
	}
	if !isBigJoker(joker) && !isSmallJoker(joker) {
		return "", fmt.Errorf("改主牌非法")
	}
	if joker.Color == Red && !(c1.Suit == Heart || c1.Suit == Diamond) {
		return "", fmt.Errorf("改主牌非法")
	}
	if joker.Color == Black && !(c1.Suit == Spade || c1.Suit == Club) {
		return "", fmt.Errorf("改主牌非法")
	}
	return c1.Suit, nil
}

func ValidateAttackTrump(j1 Card, j2 Card) (err error) {
	if j1.ID == j2.ID {
		return fmt.Errorf("攻主牌重复")
	}
	if j1.Kind != j2.Kind || (!isBigJoker(j1) && !isSmallJoker(j1)) {
		return fmt.Errorf("攻主牌非法")
	}
	return nil
}
