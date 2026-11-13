package arena

import (
	"fmt"
	"math/rand"

	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// Match plays from level 2 to A. Each leg from the same seed consumes exactly
// one independent deal seed per round, even when the strategies make the
// calling windows, plays, or number of rounds diverge. The first-round race
// uses a seeded permutation as arrival order for the four preplay policies.
func Match(seed int64, decide func(game.GameState) ([]int, error)) (game.GameState, error) {
	deals := rand.New(rand.NewSource(seed))
	teams := [2]game.TeamState{{LevelRank: rules.R2}, {LevelRank: rules.R2}}
	starter := 0
	for index := 0; ; index++ {
		dealSeed := deals.Int63()
		rng := rand.New(rand.NewSource(dealSeed))
		deck := shuffledDeck(rng)
		var order []int
		if index == 0 {
			order = rng.Perm(4)
		}
		st, err := openRound(dealSeed, deck, index, teams, starter, order)
		if err != nil {
			return st, fmt.Errorf("match seed %d round %d: %w", seed, index+1, err)
		}
		final, err := Play(st, decide, nil)
		if err != nil {
			return final, fmt.Errorf("match seed %d round %d: %w", seed, index+1, err)
		}
		if final.Phase == game.PhaseGameOver {
			return final, nil
		}
		teams, starter = final.Teams, final.NextStarterSeat
	}
}
