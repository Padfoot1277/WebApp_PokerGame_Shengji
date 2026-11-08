package bot

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func TestPublicMemoryAndVoid(t *testing.T) {
	s := phaseState()
	s.MyHand = [][]rules.Card{{card(13, rules.RA)}}
	lead := card(14, rules.RK)
	discard := rules.Card{ID: 27, Suit: rules.Club, Rank: rules.RK, SuitClass: rules.SCC}
	history := []PublicTrick{{Leader: 0, Plays: [4]*game.PlayedMove{0: {Move: game.Move{Cards: []rules.Card{lead}}, SuitClass: rules.SCH}, 1: {Move: game.Move{Cards: []rules.Card{discard}}, SuitClass: rules.SCC}}}}
	k := knowledge(s, history, handOf(s))
	if !k.void[1][rules.SCH] {
		t.Fatal("failed to infer void")
	}
	for _, c := range k.unknown {
		if c.ID == lead.ID || c.ID == discard.ID || c.ID == 13 {
			t.Fatal("known card still unseen")
		}
	}
}
func TestSafeThrowRejectsBeatableComponent(t *testing.T) {
	tr := rules.Trump{LevelRank: rules.R2}
	h := []rules.Card{card(13, rules.RA), card(14, rules.RQ)}
	k := tableKnowledge{unknown: []rules.Card{card(15, rules.RK)}}
	if got := safeThrow(h, k, tr, rules.SCH); got != nil {
		t.Fatal("Q can be stopped by K")
	}
	k.unknown = nil
	if got := safeThrow(h, k, tr, rules.SCH); len(got) != 2 {
		t.Fatal("proven safe throw missing")
	}
}
func TestThreatAccountsForVoidAndRank(t *testing.T) {
	s := phaseState()
	s.Seats[1].HandCount = 8
	s.Trump.HasTrumpSuit = true
	s.Trump.Suit = rules.Spade
	h := []rules.Card{card(13, rules.RA)}
	k := knowledge(s, nil, h)
	blocks, _ := rules.DecomposeThrow(h, s.Trump.Trump, rules.SCH)
	before := k.threat(s, 1, blocks, rules.SCH)
	k.void[1][rules.SCH] = true
	after := k.threat(s, 1, blocks, rules.SCH)
	if after <= before {
		t.Fatalf("known void should increase ruff danger: %f -> %f", before, after)
	}
}
func TestSeededFullRounds(t *testing.T) {
	start := time.Now()
	moves := 0
	for seed := int64(0); seed < 16; seed++ {
		deck := rules.NewDoubleDeck()
		rng := rand.New(rand.NewSource(seed))
		rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		tr := rules.Trump{LevelRank: []rules.Rank{rules.R2, rules.R5, rules.R10, rules.RA}[seed%4], HasTrumpSuit: seed%3 != 0, Suit: rules.Heart}
		deck = classified(deck, tr)
		hands, bottom := rules.Deal(deck)
		st := game.GameState{Phase: game.PhasePlayTrick, CallerSeat: 0, BottomOwnerSeat: 0, Bottom: bottom, Trump: game.TrumpState{Trump: tr, CallerSeat: 0}, Trick: game.TrickState{LeaderSeat: int(seed % 4), TurnSeat: int(seed % 4), BiggerSeat: -1}}
		for i := 0; i < 4; i++ {
			st.Seats[i] = game.SeatState{UID: fmt.Sprint(i), Team: i % 2, Hand: hands[i], HandCount: 25}
			st.Teams[i%2].LevelRank = tr.LevelRank
		}
		history := []PublicTrick{}
		for turn := 0; st.Phase == game.PhasePlayTrick; turn++ {
			if turn >= 100 {
				t.Fatalf("seed %d did not finish", seed)
			}
			seat := st.Trick.TurnSeat
			view := game.MakeView(st, fmt.Sprint(seat))
			cards, err := chooseWithHistory(view, history)
			if err != nil {
				t.Fatalf("seed %d turn %d: %v", seed, turn, err)
			}
			ids := []int{}
			for _, c := range cards {
				ids = append(ids, c.ID)
			}
			index := st.TrickIndex
			res, appErr := game.Reduce(st, fmt.Sprint(seat), game.EvPlayCards, game.PlayCardsPayload{CardIDs: ids})
			if appErr != nil {
				t.Fatalf("seed %d turn %d: %v", seed, turn, appErr)
			}
			st = res.State
			moves++
			if st.TrickIndex != index {
				history = append(history, PublicTrick{st.Trick.LastLeaderSeat, st.Trick.LastPlays})
			}
		}
		for _, seat := range st.Seats {
			if len(seat.Hand) != 0 {
				t.Fatalf("seed %d left cards", seed)
			}
		}
	}
	t.Logf("16 complete rounds, %d legal moves, elapsed %s", moves, time.Since(start))
}

func TestCheapestWinningCardAtLastSeat(t *testing.T) {
	s := phaseState()
	s.MySeat = 3
	s.Seats[3].Team = 1
	s.Seats[0].Team = 0
	s.MyHand = [][]rules.Card{{card(15, rules.RJ), card(16, rules.RQ), card(17, rules.RA)}}
	lead := card(18, rules.R10)
	blocks, _ := rules.DecomposeThrow([]rules.Card{lead}, s.Trump.Trump, rules.SCH)
	s.Trick = game.TrickState{LeaderSeat: 0, BiggerSeat: 0, TurnSeat: 3}
	for seat := 0; seat < 3; seat++ {
		c := card(20+seat, rules.R3)
		if seat == 0 {
			c = lead
		}
		s.Trick.Plays[seat] = &game.PlayedMove{SuitClass: rules.SCH, Move: game.Move{Cards: []rules.Card{c}, Blocks: blocks}}
	}
	got, err := choose(s)
	if err != nil || got[0].Rank != rules.RJ {
		t.Fatalf("should use J instead of Q/A: %v %v", got, err)
	}
}
func TestProtectExposedPartner(t *testing.T) {
	s := phaseState()
	s.MySeat = 2
	s.Seats[2].Team = 0
	s.Seats[3].Team = 1
	s.Seats[3].HandCount = 8
	s.MyHand = [][]rules.Card{{card(15, rules.R4), card(16, rules.R10), card(17, rules.RA)}}
	lead := card(18, rules.R9)
	blocks, _ := rules.DecomposeThrow([]rules.Card{lead}, s.Trump.Trump, rules.SCH)
	s.Trick = game.TrickState{LeaderSeat: 0, BiggerSeat: 0, TurnSeat: 2}
	s.Trick.Plays[0] = &game.PlayedMove{SuitClass: rules.SCH, Move: game.Move{Cards: []rules.Card{lead}, Blocks: blocks}}
	s.Trick.Plays[1] = &game.PlayedMove{SuitClass: rules.SCH, Move: game.Move{Cards: []rules.Card{card(20, rules.R5)}}}
	got, err := choose(s)
	if err != nil || got[0].Rank != rules.RA {
		t.Fatalf("should protect partner instead of feeding exposed 9: %v %v", got, err)
	}
}
func TestStructuredFollowSeeds(t *testing.T) {
	tr := rules.Trump{LevelRank: rules.R2}
	hand := []rules.Card{}
	for _, c := range rules.NewDoubleDeck() {
		if c.Suit == rules.Heart {
			hand = append(hand, c)
		}
	}
	hand = classified(hand, tr)
	// Two pairs plus singles requires global resource allocation.
	selected := []rules.Card{hand[1], hand[14], hand[5], hand[18], hand[9]}
	blocks, _ := rules.DecomposeThrow(selected, tr, rules.SCH)
	lead := &game.PlayedMove{SuitClass: rules.SCH, Move: game.Move{Cards: selected, Blocks: blocks}}
	seeds := structuredFollows(hand, lead, tr)
	if len(seeds) == 0 {
		t.Fatal("no structural seed found")
	}
	for _, cards := range seeds {
		if err := rules.ValidateHideCheck(hand, cards, blocks, tr); err != nil {
			t.Fatal(err)
		}
	}
}
