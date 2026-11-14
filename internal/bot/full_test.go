package bot

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func realCard(s rules.Suit, r rules.Rank, copy int) rules.Card {
	for _, c := range rules.NewDoubleDeck() {
		if c.Suit == s && c.Rank == r && c.ID/54 == copy {
			return c
		}
	}
	panic("missing card")
}
func fullFixture(hands [4][]rules.Card) game.GameState {
	st := game.GameState{Phase: game.PhasePlayTrick, CallerSeat: 0, BottomOwnerSeat: 0, Trump: game.TrumpState{Trump: rules.Trump{HasTrumpSuit: true, Suit: rules.Spade, LevelRank: rules.R2}, CallerSeat: 0}, Trick: game.TrickState{LeaderSeat: 0, TurnSeat: 0, BiggerSeat: -1}}
	for i, h := range hands {
		st.Seats[i] = game.SeatState{UID: fmt.Sprint(i), Team: i % 2, Hand: classified(h, st.Trump.Trump), HandCount: len(h)}
		st.Teams[i%2].LevelRank = rules.R2
	}
	return st
}
func playChosen(t *testing.T, st game.GameState, seat int) (game.GameState, []rules.Card) {
	t.Helper()
	a, err := DecideFull(Observe(st, seat), nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Type != "game.play_cards" {
		t.Fatalf("wrong action %+v", a)
	}
	ids := a.Payload.(game.PlayCardsPayload).CardIDs
	cards := []rules.Card{}
	for _, c := range st.Seats[seat].Hand {
		for _, id := range ids {
			if id == c.ID {
				cards = append(cards, c)
			}
		}
	}
	res, gameErr := game.Reduce(st, fmt.Sprint(seat), game.EvPlayCards, a.Payload)
	if gameErr != nil {
		t.Fatal(gameErr)
	}
	return res.State, cards
}
func TestFullStateRunsPointsAndDucksWithAttackers(t *testing.T) {
	h := rules.Heart
	st := fullFixture([4][]rules.Card{
		{realCard(h, rules.RA, 0), realCard(rules.Diamond, rules.R4, 0), realCard(rules.Spade, rules.R3, 0)},
		{realCard(h, rules.R3, 0), realCard(h, rules.RK, 0), realCard(rules.Diamond, rules.R6, 0)},
		{realCard(h, rules.R10, 0), realCard(h, rules.R4, 0), realCard(h, rules.RQ, 0)},
		{realCard(h, rules.R5, 0), realCard(h, rules.RQ, 1), realCard(rules.Diamond, rules.R7, 0)},
	})
	res, err := game.Reduce(st, "0", game.EvPlayCards, game.PlayCardsPayload{CardIDs: []int{realCard(h, rules.RA, 0).ID}})
	if err != nil {
		t.Fatal(err)
	}
	st = res.State
	st, cards := playChosen(t, st, 1)
	if len(cards) != 1 || cards[0].Rank != rules.R3 {
		t.Fatalf("attacker should duck and preserve K: %v", cards)
	}
	_, cards = playChosen(t, st, 2)
	if len(cards) != 1 || cards[0].Rank != rules.R10 {
		t.Fatalf("seated partner should run 10 under safe A: %v", cards)
	}
}
func TestFullStateSmallTrumpTransfersLead(t *testing.T) {
	st := fullFixture([4][]rules.Card{
		{realCard(rules.Spade, rules.R3, 0), realCard(rules.Heart, rules.R4, 0)},
		{realCard(rules.Spade, rules.RK, 0), realCard(rules.Heart, rules.RA, 0)},
		{realCard(rules.BigJoker, rules.RBJ, 0), realCard(rules.Heart, rules.R3, 0)},
		{realCard(rules.Spade, rules.RQ, 0), realCard(rules.Heart, rules.RK, 0)},
	})
	st, cards := playChosen(t, st, 0)
	if len(cards) != 1 || cards[0].Suit != rules.Spade || cards[0].Rank != rules.R3 {
		t.Fatalf("expected small trump to partner: %v", cards)
	}
	for seat := 1; seat < 4; seat++ {
		st, _ = playChosen(t, st, seat)
	}
	if st.Trick.LeaderSeat != 2 {
		t.Fatalf("partner did not take lead: %d", st.Trick.LeaderSeat)
	}
}
func TestFullSnapshotIsPrivateAndDetached(t *testing.T) {
	st := fullFixture([4][]rules.Card{{realCard(rules.Heart, rules.RA, 0)}, {realCard(rules.Heart, rules.R3, 0)}, {realCard(rules.Heart, rules.RK, 0)}, {realCard(rules.Heart, rules.R4, 0)}})
	table := Observe(st, 0)
	st.Seats[1].Hand[0].Rank = rules.RBJ
	if table.Hands[1][0].Rank != rules.R3 {
		t.Fatal("worker shares live hand")
	}
	raw, err := json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	var exposed map[string]json.RawMessage
	_ = json.Unmarshal(raw, &exposed)
	if _, ok := exposed["Hands"]; ok {
		t.Fatal("global hands serializable")
	}
	view := game.MakeView(st, "0")
	raw, _ = json.Marshal(view)
	var decoded game.ViewState
	_ = json.Unmarshal(raw, &decoded)
	if len(handOf(decoded)) != 1 || handOf(decoded)[0].Rank != rules.RA {
		t.Fatal("client view changed")
	}
}
func TestFullStateSeededRounds(t *testing.T) {
	started := time.Now()
	moves := 0
	for seed := int64(0); seed < 6; seed++ {
		deck := rules.NewDoubleDeck()
		rng := rand.New(rand.NewSource(seed))
		rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		hands, bottom := rules.Deal(deck)
		st := fullFixture(hands)
		st.Bottom = bottom
		if seed%2 == 0 {
			st.Trump.HasTrumpSuit = false
			st.Trump.Suit = rules.SuitAttack
			for i := range st.Seats {
				st.Seats[i].Hand = classified(hands[i], st.Trump.Trump)
			}
		}
		for turn := 0; st.Phase == game.PhasePlayTrick; turn++ {
			if turn >= 100 {
				t.Fatal("round did not finish")
			}
			st, _ = playChosen(t, st, st.Trick.TurnSeat)
			moves++
		}
	}
	t.Logf("6 full-state rounds, %d legal moves in %s", moves, time.Since(started))
}

func TestFullStateDoesNotOvertrumpSafePartner(t *testing.T) {
	st := fullFixture([4][]rules.Card{
		{realCard(rules.Heart, rules.RA, 0), realCard(rules.Club, rules.R4, 0)},
		{realCard(rules.Heart, rules.RK, 0), realCard(rules.Club, rules.R6, 0)},
		{realCard(rules.BigJoker, rules.RBJ, 0), realCard(rules.Diamond, rules.R10, 0)},
		{realCard(rules.Heart, rules.R3, 0), realCard(rules.Club, rules.R7, 0)},
	})
	res, err := game.Reduce(st, "0", game.EvPlayCards, game.PlayCardsPayload{CardIDs: []int{realCard(rules.Heart, rules.RA, 0).ID}})
	if err != nil {
		t.Fatal(err)
	}
	st = res.State
	res, err = game.Reduce(st, "1", game.EvPlayCards, game.PlayCardsPayload{CardIDs: []int{realCard(rules.Heart, rules.RK, 0).ID}})
	if err != nil {
		t.Fatal(err)
	}
	st = res.State
	_, cards := playChosen(t, st, 2)
	if len(cards) != 1 || cards[0].Rank != rules.R10 || cards[0].Suit != rules.Diamond {
		t.Fatalf("should pad 10, not waste a joker: %v", cards)
	}
}

func TestFullStateKeepsJokerWhenPartnerAlreadyWins(t *testing.T) {
	st := fullFixture([4][]rules.Card{
		{realCard(rules.Heart, rules.RA, 0), realCard(rules.Heart, rules.R4, 0), realCard(rules.Heart, rules.R6, 0), realCard(rules.Heart, rules.R7, 0), realCard(rules.Heart, rules.R8, 0), realCard(rules.Heart, rules.R3, 1)},
		{realCard(rules.Heart, rules.R3, 0), realCard(rules.Heart, rules.R9, 0), realCard(rules.Heart, rules.RJ, 0), realCard(rules.Heart, rules.RQ, 0), realCard(rules.Heart, rules.RK, 0), realCard(rules.Heart, rules.RJ, 1)},
		{realCard(rules.BigJoker, rules.RBJ, 0), realCard(rules.Diamond, rules.RA, 0), realCard(rules.Diamond, rules.RA, 1), realCard(rules.Club, rules.RA, 0), realCard(rules.Club, rules.R3, 0), realCard(rules.Spade, rules.R3, 0)},
		{realCard(rules.Heart, rules.R4, 1), realCard(rules.Heart, rules.R6, 1), realCard(rules.Heart, rules.R7, 1), realCard(rules.Heart, rules.R8, 1), realCard(rules.Heart, rules.R9, 1), realCard(rules.Heart, rules.RQ, 1)},
	})
	for seat, rank := range []rules.Rank{rules.RA, rules.R3} {
		next, err := ruleStep(st, game.PlayCardsPayload{CardIDs: []int{realCard(rules.Heart, rank, 0).ID}})
		if err != nil {
			t.Fatalf("seat %d: %v", seat, err)
		}
		st = next
	}
	_, cards := playChosen(t, st, 2)
	if len(cards) != 1 || cards[0].Suit != rules.Club || cards[0].Rank != rules.R3 {
		t.Fatalf("keep joker and controls under partner's safe ace: %v", cards)
	}
}

func TestFullStateKeepsControlInsteadOfCreatingUselessVoid(t *testing.T) {
	st := fullFixture([4][]rules.Card{
		{realCard(rules.Heart, rules.R3, 0), realCard(rules.Heart, rules.R6, 0), realCard(rules.Heart, rules.R7, 0), realCard(rules.Heart, rules.R8, 0)},
		{realCard(rules.BigJoker, rules.RBJ, 0), realCard(rules.Club, rules.R6, 0), realCard(rules.Club, rules.R7, 0), realCard(rules.Club, rules.R8, 0)},
		{realCard(rules.Heart, rules.R4, 0), realCard(rules.Heart, rules.R9, 0), realCard(rules.Heart, rules.RJ, 0), realCard(rules.Heart, rules.RQ, 0)},
		{realCard(rules.Diamond, rules.RA, 0), realCard(rules.Club, rules.R3, 0), realCard(rules.Club, rules.R4, 0), realCard(rules.Spade, rules.R3, 0)},
	})
	for _, card := range []rules.Card{realCard(rules.Heart, rules.R3, 0), realCard(rules.BigJoker, rules.RBJ, 0), realCard(rules.Heart, rules.R4, 0)} {
		next, err := ruleStep(st, game.PlayCardsPayload{CardIDs: []int{card.ID}})
		if err != nil {
			t.Fatal(err)
		}
		st = next
	}
	_, cards := playChosen(t, st, 3)
	if len(cards) != 1 || cards[0].Suit != rules.Club || cards[0].Rank != rules.R3 {
		t.Fatalf("discard low club under safe partner, retain ace for later: %v", cards)
	}
}
func TestFullBottomClearsWeakShortSuitAndKeepsControls(t *testing.T) {
	for _, kind := range []string{"weak", "ace", "pair"} {
		h := []rules.Card{}
		for _, suit := range []rules.Suit{rules.Spade, rules.Club, rules.Diamond} {
			n := 0
			for _, c := range rules.NewDoubleDeck() {
				if c.Suit == suit && n < 10 {
					h = append(h, c)
					n++
				}
			}
		}
		side := []rules.Card{realCard(rules.Heart, rules.R3, 0), realCard(rules.Heart, rules.R4, 0), realCard(rules.Heart, rules.R6, 0)}
		if kind == "ace" {
			side[2] = realCard(rules.Heart, rules.RA, 0)
		}
		if kind == "pair" {
			side[1] = realCard(rules.Heart, rules.RK, 0)
			side[2] = realCard(rules.Heart, rules.RK, 1)
		}
		h = append(h, side...)
		st := fullFixture([4][]rules.Card{h, {realCard(rules.Heart, rules.RA, 1)}, {realCard(rules.Heart, rules.RQ, 0)}, {realCard(rules.Heart, rules.RJ, 0)}})
		st.Phase = game.PhaseBottom
		ids, err := bottomFull(Observe(st, 0))
		if err != nil || len(ids) != 8 {
			t.Fatalf("%s: %v %v", kind, ids, err)
		}
		discard := map[int]bool{}
		for _, id := range ids {
			if discard[id] {
				t.Fatal("duplicate bottom")
			}
			discard[id] = true
		}
		if !discard[side[0].ID] {
			t.Fatalf("%s: weak short suit not reduced: %v", kind, ids)
		}
		if kind == "weak" {
			for _, c := range side {
				if !discard[c.ID] {
					t.Fatalf("weak suit should clear: %v", ids)
				}
			}
		}
		if kind == "ace" && discard[side[2].ID] {
			t.Fatalf("discarded safe A for void: %v", ids)
		}
		if kind == "pair" && (discard[side[1].ID] || discard[side[2].ID]) {
			t.Fatalf("broke protected high pair: %v", ids)
		}
	}
}
func TestFullThrowUsesSeparateHands(t *testing.T) {
	st := fullFixture([4][]rules.Card{
		{realCard(rules.Heart, rules.RK, 0), realCard(rules.Heart, rules.RK, 1), realCard(rules.Heart, rules.RA, 0)},
		{realCard(rules.Heart, rules.RA, 1)},
		{realCard(rules.Heart, rules.RQ, 0)},
		{realCard(rules.Heart, rules.RJ, 0)},
	})
	table := Observe(st, 0)
	cards := safeThrowFull(table.Hands[0], 0, table, rules.SCH)
	if len(cards) != 3 {
		t.Fatalf("safe A+KK throw missing: %v", cards)
	}
}

func TestFullStateRuffDeniesEnemyPointFeed(t *testing.T) {
	st := fullFixture([4][]rules.Card{
		{realCard(rules.Heart, rules.RK, 0), realCard(rules.Heart, rules.R4, 0)},
		{realCard(rules.Heart, rules.RA, 0), realCard(rules.Heart, rules.R3, 0)},
		{realCard(rules.Spade, rules.R3, 0), realCard(rules.Club, rules.R4, 0)},
		{realCard(rules.BigJoker, rules.RBJ, 0), realCard(rules.Club, rules.R5, 0)},
	})
	res, err := game.Reduce(st, "0", game.EvPlayCards, game.PlayCardsPayload{CardIDs: []int{realCard(rules.Heart, rules.RK, 0).ID}})
	if err != nil {
		t.Fatal(err)
	}
	st = res.State
	res, err = game.Reduce(st, "1", game.EvPlayCards, game.PlayCardsPayload{CardIDs: []int{realCard(rules.Heart, rules.RA, 0).ID}})
	if err != nil {
		t.Fatal(err)
	}
	st = res.State
	_, cards := playChosen(t, st, 2)
	if len(cards) != 1 || cards[0].Suit != rules.Spade {
		t.Fatalf("ruff should deny free 5-point feed and force a joker: %v", cards)
	}
	// A ruff need not win to be useful: it forces the last attacker to spend a
	// joker rather than discard a 5 onto the already-winning attacking A.
	st, _ = playChosen(t, st, 2)
	st, cards = playChosen(t, st, 3)
	if st.Points != 10 || cards[0].Suit != rules.BigJoker {
		t.Fatalf("expected forced cover, not free point feed: %v points=%d", cards, st.Points)
	}
}
func TestFullStateAttackersCollectPartnerPoints(t *testing.T) {
	st := fullFixture([4][]rules.Card{
		{realCard(rules.Heart, rules.RK, 0), realCard(rules.Heart, rules.R4, 0)},
		{realCard(rules.Heart, rules.RA, 0), realCard(rules.Heart, rules.R3, 0)},
		{realCard(rules.Heart, rules.RQ, 0), realCard(rules.Heart, rules.R5, 0)},
		{realCard(rules.Heart, rules.R10, 0), realCard(rules.Heart, rules.R6, 0)},
	})
	for seat, rank := range []rules.Rank{rules.RK, rules.RA, rules.RQ} {
		res, err := game.Reduce(st, fmt.Sprint(seat), game.EvPlayCards, game.PlayCardsPayload{CardIDs: []int{realCard(rules.Heart, rank, 0).ID}})
		if err != nil {
			t.Fatal(err)
		}
		st = res.State
	}
	st, cards := playChosen(t, st, 3)
	if len(cards) != 1 || cards[0].Rank != rules.R10 || st.Points != 20 {
		t.Fatalf("attacking partner should feed 10 to winning A: %v points=%d", cards, st.Points)
	}
}

func TestFullStatePadsAndSavesLastTrumpWithMoreCards(t *testing.T) {
	st := fullFixture([4][]rules.Card{
		{realCard(rules.Heart, rules.RK, 0), realCard(rules.Heart, rules.R4, 0), realCard(rules.Heart, rules.R6, 0), realCard(rules.Diamond, rules.R4, 0)},
		{realCard(rules.Heart, rules.RA, 0), realCard(rules.Heart, rules.R3, 0), realCard(rules.Heart, rules.R7, 0), realCard(rules.Diamond, rules.R6, 0)},
		{realCard(rules.Spade, rules.R3, 0), realCard(rules.Club, rules.R4, 0), realCard(rules.Club, rules.R6, 0), realCard(rules.Diamond, rules.R7, 0)},
		{realCard(rules.BigJoker, rules.RBJ, 0), realCard(rules.Club, rules.R3, 0), realCard(rules.Club, rules.R7, 0), realCard(rules.Diamond, rules.R8, 0)},
	})
	for seat, rank := range []rules.Rank{rules.RK, rules.RA} {
		res, err := game.Reduce(st, fmt.Sprint(seat), game.EvPlayCards, game.PlayCardsPayload{CardIDs: []int{realCard(rules.Heart, rank, 0).ID}})
		if err != nil {
			t.Fatal(err)
		}
		st = res.State
	}
	_, cards := playChosen(t, st, 2)
	if len(cards) != 1 || cards[0].SuitClass == rules.SCTrump {
		t.Fatalf("save the last trump for future tricks and pad a side card: %v", cards)
	}
}

func TestFullRolesAndBottomFollowOriginalCaller(t *testing.T) {
	st := fullFixture([4][]rules.Card{})
	st.BottomOwnerSeat = 1
	if table := Observe(st, 1); table.CallerTeam != 0 || !table.CanDig {
		t.Fatal("changing bottom owner changed seated team or suit-trump bottom eligibility")
	}
	st.Trump.HasTrumpSuit = false
	st.Trump.Suit = rules.SuitAttack
	if table := Observe(st, 1); table.CallerTeam != 0 || table.CanDig {
		t.Fatal("attacking-team hard trump must disable bottom capture, not change seated team")
	}
	st.BottomOwnerSeat = 2
	if !Observe(st, 2).CanDig {
		t.Fatal("seated-team attack should allow bottom capture")
	}
	st.BottomOwnerSeat = -1
	if Observe(st, 0).CanDig {
		t.Fatal("no caller bottom should not be captured")
	}
}
