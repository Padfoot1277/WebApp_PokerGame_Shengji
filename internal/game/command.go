package game

import "upgrade-lan/internal/game/rules"

// Commands are validated at the engine boundary, including callers outside the transport.
func validateCommand(phase Phase, typ ClientEventType, payload any) *AppError {
	allowed := map[Phase][]ClientEventType{
		PhaseLobby:     {EvSit, EvLeave, EvReady, EvUnready, EvStart},
		PhaseCallTrump: {EvCallTrump, EvCallPass}, PhaseBottom: {EvPutBottom},
		PhaseTrumpFight: {EvCallPass, EvChangeTrump, EvAttackTrump},
		PhasePlayTrick:  {EvPlayCards}, PhaseRoundSettle: {EvStartNextRound},
	}
	if typ == EvSurrender || typ == EvCancelSurrender {
		if phase == PhaseBottom || phase == PhaseTrumpFight || phase == PhasePlayTrick {
			return nil
		}
		return ErrStateWrongPhase.WithInfo("定主后至小局结束前可以申请弃局")
	}
	found := false
	for _, event := range allowed[phase] {
		if event == typ {
			found = true
			break
		}
	}
	if !found {
		return ErrStateWrongPhase
	}
	switch typ {
	case EvSit:
		if p, ok := payload.(SitPayload); ok {
			return p.Validate()
		}
	case EvCallTrump:
		if p, ok := payload.(CallTrumpPayload); ok {
			return p.Validate()
		}
	case EvChangeTrump:
		if p, ok := payload.(ChangeTrumpPayload); ok {
			return p.Validate()
		}
	case EvAttackTrump:
		if p, ok := payload.(AttackTrumpPayload); ok {
			return p.Validate()
		}
	case EvPutBottom:
		if p, ok := payload.(PutBottomPayload); ok {
			return p.Validate()
		}
	case EvPlayCards:
		if p, ok := payload.(PlayCardsPayload); ok {
			return p.Validate()
		}
	default:
		return nil
	}
	return ErrInvalidPayload
}

// Copy all mutable storage before applying a command. Rejected commands leave the input intact.
func cloneState(st GameState) GameState {
	for i := range st.Seats {
		st.Seats[i].Hand = append([]rules.Card(nil), st.Seats[i].Hand...)
	}
	st.Bottom = append([]rules.Card(nil), st.Bottom...)
	st.BottomReveal = append([]rules.Card(nil), st.BottomReveal...)
	used := make(map[int]bool, len(st.UsedTrumpIDs))
	for id, v := range st.UsedTrumpIDs {
		used[id] = v
	}
	st.UsedTrumpIDs = used
	for i := 0; i < 4; i++ {
		if p := st.Trick.Plays[i]; p != nil {
			cp := *p
			cp.Move = cloneMove(p.Move)
			st.Trick.Plays[i] = &cp
		}
		if p := st.Trick.LastPlays[i]; p != nil {
			cp := *p
			cp.Move = cloneMove(p.Move)
			st.Trick.LastPlays[i] = &cp
		}
	}
	if p := st.Trick.Throw; p != nil {
		cp := *p
		cp.IntentMove = cloneMove(p.IntentMove)
		st.Trick.Throw = &cp
	}
	return st
}

func useTrumpCards(st *GameState, ids []int) *AppError {
	if err := validateUnique(ids); err != nil {
		return err
	}
	for _, id := range ids {
		if st.UsedTrumpIDs[id] {
			return ErrRuleIllegalTrump.WithInfo("这张牌已用于本局亮主，不可重复使用")
		}
	}
	if st.UsedTrumpIDs == nil {
		st.UsedTrumpIDs = make(map[int]bool)
	}
	for _, id := range ids {
		st.UsedTrumpIDs[id] = true
	}
	return nil
}

// Rank is the primary order across structures; ties prefer the smaller structure, then card ID.
func weakerBlock(a, b rules.Block) bool {
	if a.RankValue != b.RankValue {
		return a.RankValue < b.RankValue
	}
	if len(a.Cards) != len(b.Cards) {
		return len(a.Cards) < len(b.Cards)
	}
	return a.Cards[0].ID < b.Cards[0].ID
}

func reduceSurrender(st GameState, uid string, typ ClientEventType) (ReduceResult, *AppError) {
	seat, err := seatIndexByUID(&st, uid)
	if err != nil {
		return ReduceResult{State: st}, err
	}
	bit := uint8(1 << uint(seat))
	if typ == EvCancelSurrender {
		st.SurrenderMask &^= bit
	} else {
		st.SurrenderMask |= bit
	}
	st.Version++
	notice := "弃局意愿已更新；同队两人同意后，按当前已结算分数结束本局"
	teamMask := uint8(0b0101 << uint(seat%2))
	if st.SurrenderMask&teamMask == teamMask {
		st.Surrendered = true
		st.Trick.TurnSeat = -1
		notice = "同队两人同意弃局。" + settleRoundEnd(&st)
	}
	return ReduceResult{State: st, Changed: true, Notice: notice}, nil
}

func SeatName(seat int) string {
	if seat < 0 || seat > 3 {
		return "未入座"
	}
	return []string{"东席", "南席", "西席", "北席"}[seat]
}
