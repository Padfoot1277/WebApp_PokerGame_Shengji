package bot

import (
	"math/rand"
	"reflect"
	"testing"

	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// The rollout rule must preserve the one-trick first choice on
// equal returns, and every alternative it takes must improve the simulated
// settlement from the acting team's perspective. Check both partnerships.
func TestRuleRolloutImprovesOrKeepsTacticalChoice(t *testing.T) {
	rng := rand.New(rand.NewSource(523))
	improved := 0
	for deal := 0; deal < 60; deal++ {
		deck := rules.NewDoubleDeck()
		rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		var hands [4][]rules.Card
		for i := range hands {
			hands[i] = deck[i*4 : i*4+4]
		}
		st := fullFixture(hands)
		st.Points = 75
		st.Bottom = deck[16:24]
		seat := deal % 4
		st.Trick.LeaderSeat, st.Trick.TurnSeat = seat, seat
		base, err := scoreFullBudget(Observe(st, seat), nil, true)
		if err != nil {
			t.Fatal(err)
		}
		move, err := RulePlay(st)
		if err != nil {
			t.Fatal(err)
		}
		old := learnedAction(base).Payload.(game.PlayCardsPayload)
		newMove := move.Payload.(game.PlayCardsPayload)
		if reflect.DeepEqual(old, newMove) {
			continue
		}
		before, e := ruleReturn(st, old, roomRuleSearch.ExactTail)
		if e != nil {
			t.Fatal(e)
		}
		after, e := ruleReturn(st, newMove, roomRuleSearch.ExactTail)
		if e != nil {
			t.Fatal(e)
		}
		if after.levels <= before.levels {
			t.Fatalf("deal %d seat %d changed on %d -> %d", deal, seat, before.levels, after.levels)
		}
		improved++
	}
	if improved == 0 {
		t.Fatal("no strict improvements exercised")
	}
}

func TestWiderShortlistKeepsAttackersBelowEighty(t *testing.T) {
	deck := rules.NewDoubleDeck()
	var hands [4][]rules.Card
	for seat, ids := range [4][5]int{{16, 51, 61, 15, 77}, {7, 70, 17, 46, 105}, {27, 84, 58, 45, 3}, {12, 40, 14, 83, 42}} {
		for _, id := range ids {
			hands[seat] = append(hands[seat], deck[id])
		}
	}
	st := fullFixture(hands)
	st.Points = 75
	st.Trick.LeaderSeat, st.Trick.TurnSeat = 2, 2
	for _, id := range []int{30, 21, 96, 32, 90, 72, 91, 25} {
		st.Bottom = append(st.Bottom, deck[id])
	}
	for _, tc := range []struct{ limit, id, levels int }{{3, 84, 0}, {5, 45, 1}} {
		action, err := (RuleSearch{MaxHand: 8, Candidates: tc.limit}).Play(st)
		if err != nil {
			t.Fatal(err)
		}
		move := action.Payload.(game.PlayCardsPayload)
		outcome, err := ruleReturn(st, move, 0)
		if err != nil || len(move.CardIDs) != 1 || move.CardIDs[0] != tc.id || outcome.levels != tc.levels {
			t.Fatalf("limit %d chose %v with %d levels, error %v", tc.limit, move, outcome.levels, err)
		}
	}
}

func TestRuleSettlementPointPreference(t *testing.T) {
	best := ruleOutcome{levels: 1, margin: -70}
	for _, tc := range []struct {
		candidate          ruleOutcome
		preferPoints, wins bool
	}{
		{ruleOutcome{2, -90}, true, true},
		{ruleOutcome{0, -10}, true, false},
		{ruleOutcome{1, -65}, true, true},
		{ruleOutcome{1, -65}, false, false},
		{ruleOutcome{1, -70}, true, false},
	} {
		if tc.candidate.better(best, tc.preferPoints) != tc.wins {
			t.Fatalf("candidate %+v vs %+v with preferPoints=%t", tc.candidate, best, tc.preferPoints)
		}
	}
}

func TestRuleDiverseLastCandidate(t *testing.T) {
	cards := []rules.Card{
		{ID: 1, SuitClass: rules.SCS, Rank: rules.R3},
		{ID: 2, SuitClass: rules.SCS, Rank: rules.R4},
		{ID: 3, SuitClass: rules.SCS, Rank: rules.R6},
		{ID: 4, SuitClass: rules.SCS, Rank: rules.R10},
		{ID: 5, SuitClass: rules.SCD, Rank: rules.R8},
	}
	var ranked []tacticalChoice
	for _, c := range cards {
		ranked = append(ranked, tacticalChoice{cards: []rules.Card{c}})
	}
	diversifyLast(ranked, 3)
	if ranked[0].cards[0].ID != 1 || ranked[1].cards[0].ID != 2 || ranked[2].cards[0].ID != 4 {
		t.Fatalf("expected first two unchanged and highest-ranked new signature third: %+v", ranked)
	}
}

// At 75 attacker points the one-trick continuation forecasts 90 for both
// leads. With best play in the last three cards one lead loses a level.
func TestRuleTailForecastForAttackingTeam(t *testing.T) {
	deck := rules.NewDoubleDeck()
	var hands [4][]rules.Card
	for seat, ids := range [4][4]int{{34, 29, 23, 61}, {101, 2, 73, 88}, {10, 25, 100, 72}, {97, 42, 54, 24}} {
		for _, id := range ids {
			hands[seat] = append(hands[seat], deck[id])
		}
	}
	st := fullFixture(hands)
	st.Trick.LeaderSeat, st.Trick.TurnSeat = 1, 1
	st.Points = 75
	for _, id := range []int{20, 69, 57, 45, 89, 106, 36, 75} {
		st.Bottom = append(st.Bottom, deck[id])
	}
	for _, tc := range []struct{ tail, wantID, wantLevels int }{{0, 73, -1}, {3, 2, 0}} {
		action, err := (RuleSearch{MaxHand: 13, Candidates: 5, ExactTail: tc.tail}).Play(st)
		if err != nil {
			t.Fatal(err)
		}
		play := action.Payload.(game.PlayCardsPayload)
		if len(play.CardIDs) != 1 || play.CardIDs[0] != tc.wantID {
			t.Fatalf("tail %d chose %v", tc.tail, play)
		}
		tactical, err := ruleReturn(st, play, 0)
		if err != nil || tactical.levels != 0 || tactical.margin != 90 {
			t.Fatalf("tactical result %+v, err %v", tactical, err)
		}
		exact, err := ruleReturn(st, play, 3)
		if err != nil || exact.levels != tc.wantLevels {
			t.Fatalf("tail result %+v, err %v", exact, err)
		}
	}
}
