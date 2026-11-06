package rules

import (
	"testing"
	"time"
)

func cardsFor(ranks ...Rank) []Card {
	deck := NewDoubleDeck()
	out := []Card{}
	for _, r := range ranks {
		for _, c := range deck {
			if c.Suit == Spade && c.Rank == r {
				c.SuitClass = SCS
				out = append(out, c)
			}
		}
	}
	return out
}
func TestCallRequiresJoker(t *testing.T) {
	deck := NewDoubleDeck()
	if _, _, err := ValidateCallTrump(R2, deck[0], []Card{deck[12]}); err == nil {
		t.Fatal("non-joker accepted")
	}
}
func TestLongTractorFollow(t *testing.T) {
	trump := Trump{LevelRank: R2}
	lead, _ := DecomposeThrow(cardsFor(R3, R4, R5, R6), trump, SCS)
	hand := cardsFor(R3, R4, R5, R6, RA)
	valid := cardsFor(R3, R4, R5, R6)
	if err := ValidateHideCheck(hand, valid, lead, trump); err != nil {
		t.Fatal(err)
	}
	invalid := cardsFor(R3, R4, R5, RA)
	if err := ValidateHideCheck(hand, invalid, lead, trump); err == nil {
		t.Fatal("hidden four-pair tractor accepted")
	}
}
func TestFollowDoesNotDependOnIDs(t *testing.T) {
	trump := Trump{LevelRank: R2}
	hand := cardsFor(R3, R4, R5, R6, RA)
	reqs := []reqBlock{{bt: BlockTractor, tractorLen: 2, needCards: 4}, {bt: BlockPair, needCards: 2}}
	before, err := bestFollowProfile(hand, reqs, trump, SCS)
	if err != nil {
		t.Fatal(err)
	}
	for i := range hand {
		hand[i].ID = 107 - hand[i].ID
	}
	after, _ := bestFollowProfile(hand, reqs, trump, SCS)
	if compareProfile(before, after) != 0 {
		t.Fatal("card IDs affect follow")
	}
}
func TestRemovedLevelDoesNotBreakOrdinaryTractor(t *testing.T) {
	trump := Trump{LevelRank: R4}
	blocks, err := FindBlocksInHand(cardsFor(R3, R5), trump, SCS, BlockTractor, 2)
	if err != nil || len(blocks) != 1 {
		t.Fatal("3 and 5 must be adjacent when 4 is level")
	}
}
func TestRankCapsAtAce(t *testing.T) {
	if AddRank(RK, 3) != RA {
		t.Fatal("rank wrapped")
	}
}
func TestFollowSearchBound(t *testing.T) {
	start := time.Now()
	cards := cardsFor(R3, R4, R5, R6, R7, R8, R9, R10, RJ, RQ, RK, RA)
	_, err := bestFollowProfile(cards, []reqBlock{{bt: BlockTractor, tractorLen: 10, needCards: 20}}, Trump{LevelRank: R2}, SCS)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("search duration", time.Since(start))
}
func TestWinningMovePreservesLeadShape(t *testing.T) {
	trump := Trump{LevelRank: R2}
	lead, _ := DecomposeThrow(cardsFor(R3, R5), trump, SCS)
	matched, ok, err := WinningBlocks(lead, cardsFor(R6, R7), trump)
	if err != nil || !ok {
		t.Fatal("winning pair assignment failed", err)
	}
	if len(matched) != 1 || len(matched[0]) != 2 || matched[0][0].Type != BlockPair {
		t.Fatal("follower changed lead shape")
	}
	if _, ok, err := WinningBlocks(matched, cardsFor(R9, RJ), trump); err != nil || !ok {
		t.Fatal("later player cannot beat matching pairs", err)
	}
}
