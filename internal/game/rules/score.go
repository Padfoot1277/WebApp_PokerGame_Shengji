package rules

func TrickPoints(cards []Card) int {
	sum := 0
	for _, c := range cards {
		switch c.Rank {
		case R5:
			sum += 5
		case R10:
			sum += 10
		case RK:
			sum += 10
		}
	}
	return sum
}

func DigMultiplierByWinnerMove(winType BlockType) int {
	switch winType {
	case BlockSingle:
		return 1
	case BlockPair:
		return 2
	case BlockTractor:
		return 4
	default:
		return 1
	}
}

func AddRank(r Rank, delta int) Rank {
	if delta <= 0 {
		return r
	}
	// 级牌升级序列（不含大小王）
	seq := []Rank{R2, R3, R4, R5, R6, R7, R8, R9, R10, RJ, RQ, RK, RA}
	// Pending：保持不变（如果你希望 Pending + delta 从 R2 开始，可在这里改）
	if r == RPending || r == RBJ || r == RSJ {
		return r
	}
	// 找到当前 rank 在序列中的位置
	idx := -1
	for i, v := range seq {
		if v == r {
			idx = i
			break
		}
	}
	if idx == -1 {
		return r
	}
	nidx := idx + delta
	if nidx >= len(seq) {
		nidx = 0 // 无限模式
	}
	return seq[nidx]
}
