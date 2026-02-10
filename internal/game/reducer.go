package game

import (
	"fmt"
	"upgrade-lan/internal/game/rules"
)

type ReduceResult struct {
	State   GameState
	Changed bool
	Notice  string
}

func Reduce(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, *AppError) {
	switch st.Phase {
	case PhaseLobby:
		return reduceLobby(st, uid, typ, payload)
	case PhaseCallTrump:
		return reduceCallTrump(st, uid, typ, payload)
	case PhaseBottom:
		return reduceBottom(st, uid, typ, payload)
	case PhaseTrumpFight:
		return reduceTrumpFight(st, uid, typ, payload)
	case PhasePlayTrick:
		return reducePlayTrick(st, uid, typ, payload)
	default:
		return ReduceResult{State: st, Changed: false}, ErrStateWrongPhase.WithInfof("非法游戏阶段 %s", st.Phase)
	}
}

func reduceLobby(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, *AppError) {
	switch typ {
	case EvSit:
		p := payload.(SitPayload)
		seat := &st.Seats[p.Seat]
		if seat.UID != "" && seat.UID != uid {
			return ReduceResult{State: st}, ErrStateSeatTaken.WithInfof("该座位已有玩家%s", seat.UID)
		}
		// 如果 uid 已经坐在别处，先清掉旧座位
		for i := 0; i < 4; i++ {
			if st.Seats[i].UID == uid && i != p.Seat {
				st.Seats[i] = SeatState{}
			}
		}
		seat.UID = uid
		seat.Online = true
		seat.Ready = false
		st.Version++
		return ReduceResult{State: st, Changed: true, Notice: fmt.Sprintf("玩家%s已坐入%d号位", uid, p.Seat)}, nil

	case EvLeave:
		// uid 离开自己座位
		for i := 0; i < 4; i++ {
			if st.Seats[i].UID == uid {
				st.Seats[i] = SeatState{}
				st.Version++
				return ReduceResult{State: st, Changed: true, Notice: fmt.Sprintf("玩家%s已离开%d号位", uid, i)}, nil
			}
		}
		return ReduceResult{State: st}, ErrStateNotSeated.WithInfof("当前还未就坐")

	case EvReady, EvUnready:
		wantReady := typ == EvReady
		for i := 0; i < 4; i++ {
			if st.Seats[i].UID == uid {
				st.Seats[i].Ready = wantReady
				st.Version++
				rr := ReduceResult{State: st, Changed: true}

				// 自动 start：4 人都 ready
				if allReady(&st) {
					startDeal(&st)
					rr.State = st
					rr.Notice = "所有人已准备，系统已自动发牌"
				}
				return rr, nil
			}
		}
		return ReduceResult{State: st}, ErrStateNotSeated.WithInfof("当前还未就坐")

	case EvStart:
		// 手动 start：仅当 4 人都 ready（可限制房主）
		if !allReady(&st) {
			return ReduceResult{State: st}, ErrStateNotReady.WithInfof("还有人没准备")
		}
		startDeal(&st)
		return ReduceResult{State: st, Changed: true, Notice: "已手动发牌"}, nil

	default:
		return ReduceResult{State: st}, ErrUnknownEvent.WithInfof("未知Phase状态 %s", st.Phase)
	}
}

func startDeal(st *GameState) {
	// 进入dealing
	st.Phase = PhaseDealing

	// 生成两副牌并洗牌发牌
	deck := rules.NewDoubleDeck()
	rules.ShuffleInPlace(deck)
	hands, bottom := rules.Deal(deck)

	// 写入座位手牌
	for i := 0; i < 4; i++ {
		st.Seats[i].Hand = hands[i]
		st.Seats[i].HandCount = len(hands[i]) // 25张
		// 发牌阶段：先按“本队级牌 + 无主花色”排序，便于判断能否定主
		team := st.Seats[i].Team
		level := st.Teams[team].LevelRank
		rules.SortHand(st.Seats[i].Hand, rules.Trump{
			LevelRank:    level,
			HasTrumpSuit: false,
		})
	}

	// 写入底牌
	st.Bottom = bottom
	st.BottomCount = len(bottom) // 8张
	st.BottomOwnerSeat = -1

	// ---- 初始化本小局定主流转 ----
	st.CallPassMask = 0
	st.CallPassCount = 0

	if st.RoundIndex == 0 {
		// 第一小局：抢定主
		st.CallMode = CallModeRace
		st.StarterSeat = -1
		st.CallTurnSeat = -1
	} else {
		// 后续小局：按上一小局结果决定优先权
		st.CallMode = CallModeOrdered
		st.StarterSeat = st.NextStarterSeat
		st.CallTurnSeat = st.StarterSeat
	}

	st.Trump = TrumpState{
		Trump: rules.Trump{
			HasTrumpSuit: false,
			LevelRank:    rules.RPending,
		},
		Locked:     false,
		CallerSeat: -1,
	}

	// 发牌结束后进入下一阶段（定主）
	st.Phase = PhaseCallTrump
	st.Version++
}

func reduceCallTrump(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, *AppError) {
	seat, err := seatIndexByUID(&st, uid)
	if err != nil {
		return ReduceResult{State: st}, err
	}
	if st.CallMode == CallModeOrdered {
		if seat != st.CallTurnSeat {
			return ReduceResult{State: st}, ErrStateNotYourTurn.WithInfof("请等待%v号位定主", st.CallTurnSeat)
		}
	}
	if st.CallMode == CallModeRace && st.StarterSeat >= 0 {
		return ReduceResult{State: st}, ErrStateNotYourTurn.WithInfof("手慢啦，%v号位已定主", st.StarterSeat)
	}

	switch typ {

	case EvCallPass:
		bit := uint8(1 << uint(seat))
		if (st.CallPassMask & bit) != 0 {
			return ReduceResult{State: st}, ErrDuplicateOps.WithInfo("重复点击跳过")
		}
		st.CallPassMask |= bit
		st.CallPassCount++
		st.Version++
		if st.CallMode == CallModeOrdered {
			st.CallTurnSeat = (st.CallTurnSeat + 1) % 4
		}

		// 四人都 pass -> 硬主
		if st.CallPassCount >= 4 {
			st.Trump.HasTrumpSuit = false
			st.Trump.Locked = false
			st.Trump.CallerSeat = -1

			leaderSeat := 0 // 首局抢定主失败时，先手按 seat0
			if st.CallMode == CallModeOrdered {
				leaderSeat = st.StarterSeat
			}
			st.StarterSeat = leaderSeat
			leaderTeam := st.Seats[leaderSeat].Team
			st.Trump.LevelRank = st.Teams[leaderTeam].LevelRank

			// 修改每张牌的牌域，手牌重排（硬主视角）
			refreshHandSuitClassForUI(&st)
			sortAllHands(&st)

			st.BottomOwnerSeat = -1
			st.Phase = PhasePlayTrick
			st.Trick = TrickState{
				LeaderSeat: st.StarterSeat,
				TurnSeat:   st.StarterSeat,
			}
			st.Version++
			notice := fmt.Sprintf("无人定主，本小局硬主，级牌为%s", st.Trump.LevelRank)
			return ReduceResult{State: st, Changed: true, Notice: notice}, nil
		}

		return ReduceResult{State: st, Changed: true, Notice: fmt.Sprintf("%d号位不定主", seat)}, nil

	case EvCallTrump:
		p := payload.(CallTrumpPayload)
		handIdx := NewCardIndex(st.Seats[seat].Hand)
		joker, ok := handIdx.Get(p.JokerID)
		if !ok {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo("手牌中没有此王牌")
		}
		levelCards := make([]rules.Card, 0, len(p.LevelIDs))
		for _, id := range p.LevelIDs {
			c, ok := handIdx.Get(id)
			if !ok {
				return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo("手牌中没有此级牌")
			}
			levelCards = append(levelCards, c)
		}
		// 校验定主规则
		team := st.Seats[seat].Team
		teamLevel := st.Teams[team].LevelRank
		trumpSuit, locked, err := rules.ValidateCallTrump(teamLevel, joker, levelCards)
		if err != nil {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo(err.Error())
		}
		// 定主成功：写入 Trump
		st.Trump.HasTrumpSuit = true
		st.Trump.Suit = trumpSuit
		st.Trump.LevelRank = teamLevel
		st.Trump.Locked = locked
		st.Trump.CallerSeat = seat
		st.StarterSeat = seat

		// 修改每张牌的牌域，手牌按“本小局最终级牌 + 主花色”重排
		refreshHandSuitClassForUI(&st)
		sortAllHands(&st)

		// 进入下一阶段：坐家收底牌、重扣底牌
		enterBottomPhase(&st, seat)
		notice := fmt.Sprintf("成功定主，主牌为%s，级牌为%s，请扣底牌", st.Trump.LevelRank, st.Trump.Suit)
		if locked {
			notice = fmt.Sprintf("成功定主，主牌为%s（已锁定），级牌为%s，请扣底牌", st.Trump.LevelRank, st.Trump.Suit)
		}
		return ReduceResult{State: st, Changed: true, Notice: notice}, nil

	default:
		return ReduceResult{State: st}, ErrUnknownEvent.WithInfof("非法事件 %s", typ)
	}
}

func enterBottomPhase(st *GameState, ownerSeat int) {
	st.BottomOwnerSeat = ownerSeat

	// 把底牌“复制”进坐家手牌 -> 33张
	st.Seats[ownerSeat].Hand = append(st.Seats[ownerSeat].Hand, st.Bottom...)
	st.Seats[ownerSeat].HandCount = len(st.Seats[ownerSeat].Hand) // 33张
	rules.SortHand(st.Seats[ownerSeat].Hand, st.Trump.Trump)

	st.Phase = PhaseBottom
	st.Version++
}

func reduceBottom(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, *AppError) {
	seat, err := seatIndexByUID(&st, uid)
	if err != nil {
		return ReduceResult{State: st}, err
	}
	if seat != st.BottomOwnerSeat {
		return ReduceResult{State: st}, ErrStateNotYourTurn.WithInfof("非底牌所有者")
	}

	switch typ {
	case EvPutBottom:
		p := payload.(PutBottomPayload)
		// 校验8张牌都在坐家手牌里
		hand := st.Seats[seat].Hand
		if len(hand) != 33 {
			return ReduceResult{State: st}, ErrRuleIllegalPlay.WithInfof("当前手牌数为%d，无法扣牌", len(hand))
		}
		newBottom, err := pickCardsFromHand(hand, p.DiscardIDs)
		if err != nil {
			return ReduceResult{State: st}, err
		}
		// 从手牌移除这8张 -> 回到25
		keep := deleteCardsFromHand(hand, p.DiscardIDs)
		st.Seats[seat].Hand = keep
		st.Seats[seat].HandCount = 25
		st.Bottom = newBottom
		st.BottomCount = 8

		// 扣底完成：进入改主/攻主窗口（PhaseTrumpFight）
		st = enterTrumpFight(st)
		return ReduceResult{State: st, Changed: true, Notice: fmt.Sprintf("玩家%d完成扣牌", seat)}, nil

	default:
		return ReduceResult{State: st}, ErrUnknownEvent.WithInfof("非法事件 %s", typ)
	}
}

func enterTrumpFight(st GameState) GameState {
	st.FightPassMask = 0
	st.FightPassCount = 0
	st.Phase = PhaseTrumpFight
	st.Version++
	return st
}

func reduceTrumpFight(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, *AppError) {
	seat, err := seatIndexByUID(&st, uid)
	if err != nil {
		return ReduceResult{State: st}, err
	}

	// 定主者不参与改主/攻主
	if seat == st.BottomOwnerSeat {
		return ReduceResult{State: st}, ErrStateNotYourTurn.WithInfo("请等待其余玩家攻改")
	}

	handIdx := NewCardIndex(st.Seats[seat].Hand)

	switch typ {
	case EvCallPass:
		bit := uint8(1 << uint(seat))
		if (st.FightPassMask & bit) != 0 {
			return ReduceResult{State: st}, ErrDuplicateOps.WithInfo("重复点击跳过")
		}
		st.FightPassMask |= bit
		st.FightPassCount++
		st.Version++
		notice := fmt.Sprintf("玩家%d已选择跳过", seat)
		// 其余三位都跳过，则正式进入出牌阶段
		if st.FightPassCount >= 3 {
			st.Phase = PhasePlayTrick
			st.Trick = TrickState{
				LeaderSeat: st.StarterSeat,
				TurnSeat:   st.StarterSeat,
			}
			st.Version++
			notice = fmt.Sprintf("%s。无人继续改/攻主，进入出牌阶段，由%d号位先手", notice, st.StarterSeat)
			return ReduceResult{State: st, Changed: true, Notice: notice}, nil
		}
		return ReduceResult{State: st, Changed: true, Notice: notice}, nil

	case EvChangeTrump:
		if st.Trump.Locked {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo("主家已锁定花色，不可以改主")
		}
		// 初步校验
		p := payload.(ChangeTrumpPayload)
		joker, ok := handIdx.Get(p.JokerID)
		if !ok {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo("手牌中无此牌")
		}
		c1, ok := handIdx.Get(p.LevelIDs[0])
		if !ok {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo("手牌中无此牌")
		}
		c2, ok := handIdx.Get(p.LevelIDs[1])
		if !ok {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo("手牌中无此牌")
		}
		// 改主规则校验
		trumpSuit, err := rules.ValidateChangeTrump(st.Trump.LevelRank, joker, c1, c2)
		if err != nil {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo(err.Error())
		}
		// 改主成功：只改主花色，不改 LevelRank / StarterSeat
		st.Trump.HasTrumpSuit = true
		st.Trump.Suit = trumpSuit
		refreshHandSuitClassForUI(&st)
		sortAllHands(&st)
		// 改主者成为 bottomOwner，拿当前底牌并扣底
		enterBottomPhase(&st, seat)
		notice := fmt.Sprintf("%d号位改主成功，变为花色%s", seat, st.Trump.Suit)
		return ReduceResult{State: st, Changed: true, Notice: notice}, nil

	case EvAttackTrump:
		// 初步校验
		p := payload.(AttackTrumpPayload)
		j1, ok := handIdx.Get(p.JokerIDs[0])
		if !ok {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo("手牌中无此牌")
		}
		j2, ok := handIdx.Get(p.JokerIDs[1])
		if !ok {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo("手牌中无此牌")
		}
		// 攻主规则校验
		err := rules.ValidateAttackTrump(j1, j2)
		if err != nil {
			return ReduceResult{State: st}, ErrRuleIllegalTrump.WithInfo(err.Error())
		}
		// 攻主成功：进入硬主（只影响主花色体系），不改 LevelRank/StarterSeat。此后不可以再改主
		st.Trump.HasTrumpSuit = false
		st.Trump.Suit = ""
		st.Trump.Locked = true
		refreshHandSuitClassForUI(&st)
		sortAllHands(&st)
		// 攻主者成为 bottomOwner，拿底扣底
		enterBottomPhase(&st, seat)
		notice := fmt.Sprintf("玩家%d攻主成功，本小局硬主", seat)
		return ReduceResult{State: st, Changed: true, Notice: notice}, nil

	default:
		return ReduceResult{State: st}, ErrUnknownEvent.WithInfof("非法事件 %s", typ)
	}
}

func reducePlayTrick(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, *AppError) {
	// 合法校验
	seat, err := seatIndexByUID(&st, uid)
	if err != nil {
		return ReduceResult{State: st}, err
	}
	if seat != st.Trick.TurnSeat {
		return ReduceResult{State: st}, ErrStateNotYourTurn.WithInfof("请先等待玩家%v出牌", st.Trick.TurnSeat)
	}
	if st.Trick.Plays[seat] != nil {
		return ReduceResult{State: st}, ErrStateNotYourTurn.WithInfo("已出过牌，请等待本回合结束")
	}
	p := payload.(PlayCardsPayload)
	// 根据cardID，从手牌中获取card
	selected, err := pickCardsFromHand(st.Seats[seat].Hand, p.CardIDs)
	if err != nil {
		return ReduceResult{State: st}, err
	}
	switch typ {
	case EvPlayCards:
		currentMove := PlayedMove{Seat: seat}
		if seat == st.Trick.LeaderSeat {
			// 先手约束：必须全是同SuitClass（同副花色 或 全主）
			sc, ok := rules.ComputeSuitClassAllSame(selected)
			if !ok {
				return ReduceResult{State: st}, ErrRuleIllegalPlay.WithInfo("所出牌的牌域不一致")
			}
			// 将所出牌解析为多组Block，同一种牌型归于一组
			blockGroups, decomposeErr := rules.DecomposeThrow(selected, st.Trump.Trump, sc)
			if decomposeErr != nil {
				return ReduceResult{State: st}, ErrRuleIllegalPlay.WithInfo(decomposeErr.Error())
			}
			currentMove.SuitClass = sc
			currentMove.Move = Move{
				Blocks:  blockGroups,
				CardIDs: p.CardIDs,
				Cards:   selected,
			}
			// 如果有多种牌型，或者有多个同牌型，则视为甩牌，需进行判定并调整leadMove
			if len(blockGroups) > 1 || len(blockGroups[0]) > 1 {
				throwMove := ThrowMove{
					IsThrow:    true,
					IntentMove: cloneMove(currentMove.Move),
				}
				// 甩牌判定，若失败则更新selected，blockGroups
				throwOK, actualMove, info, err := canonicalizeLead(st, seat, sc, throwMove.IntentMove)
				if err != nil {
					return ReduceResult{State: st}, err
				}
				throwMove.ThrowOK = throwOK
				st.Trick.Throw = &throwMove
				// 若甩牌失败，重新修改出牌
				if !throwOK {
					currentMove.Move = actualMove
					currentMove.Info = info
				}
			}
		} else {
			// 跟牌判定，暂时跳过

		}
		// 从手牌移除实际出的牌
		st.Seats[seat].Hand = deleteCardsFromHand(st.Seats[seat].Hand, currentMove.CardIDs)
		st.Seats[seat].HandCount = len(st.Seats[seat].Hand)
		// 更新trick
		st.Trick.Plays[seat] = &currentMove
		st.Trick.TurnSeat = (seat + 1) % 4
		st.Version++
		notice := fmt.Sprintf("玩家%d已出牌", seat)
		if currentMove.Info != "" {
			notice += fmt.Sprintf("【%s】", currentMove.Info)
		}
		return ReduceResult{State: st, Changed: true, Notice: notice}, nil

	default:
		return ReduceResult{State: st}, ErrUnknownEvent.WithInfof("非法事件 %s", typ)
	}
}

// canonicalizeLead 对先手选牌进行甩牌判定
// 返回：是否甩牌成功、最终出牌、失败原因，错误
func canonicalizeLead(st GameState, leaderSeat int, sc rules.SuitClass, intentMove Move) (bool, Move, string, *AppError) {
	// 对每一种牌型（每个组），取甩牌中该组最小的，与对手最大比
	groups := intentMove.Blocks
	for off := 1; off <= 3; off++ {
		def := (leaderSeat + off) % 4
		for _, g := range groups {
			if len(g) == 0 {
				continue
			}
			throwMin := g[len(g)-1]
			defHand := st.Seats[def].Hand
			defBlocks, err := rules.FindBlocksInHand(defHand, st.Trump.Trump, sc, throwMin.Type, throwMin.TractorLen)
			if err != nil {
				return false, Move{}, "", ErrSystem.WithInfo(err.Error())
			}
			if len(defBlocks) == 0 {
				continue
			}
			defMax := defBlocks[0]
			// 同type、同suitClass，比较大小
			res, err := rules.CompareTwoBlocks(throwMin, defMax)
			if err != nil {
				return false, Move{}, "", ErrSystem.WithInfo(err.Error())
			}
			if res < 0 {
				// 甩牌失败：最终出牌结果=该组最小的 throwMin
				return false, Move{
					Blocks:  [][]rules.Block{{throwMin}},
					Cards:   throwMin.Cards,
					CardIDs: getIDs(throwMin.Cards),
				}, fmt.Sprintf("甩牌失败，原计划甩出%v张%v", len(intentMove.CardIDs), sc), nil
			}
		}
	}
	// 无人可压，甩牌成功
	return true, intentMove, "甩牌成功", nil
}
