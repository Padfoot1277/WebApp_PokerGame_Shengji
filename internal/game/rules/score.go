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
