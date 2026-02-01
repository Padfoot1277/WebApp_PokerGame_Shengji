package game

import (
	"fmt"
	"log"
	"upgrade-lan/internal/game/rules"
)

type ReduceResult struct {
	State   GameState
	Changed bool
	Notice  string // 可选：给前端提示
}

func Reduce(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, error) {
	switch st.Phase {
	case PhaseLobby:
		return reduceLobby(st, uid, typ, payload)
	case PhaseDealing:
		// 目前 dealing 不响应任何客户端事件（后续再加发牌确认等）
		return ReduceResult{State: st, Changed: false}, fmt.Errorf("phase_forbid: %s", typ)
	case PhaseCallTrump:
		return reduceCallTrump(st, uid, typ, payload)
	case PhaseBottom:
		return reduceBottom(st, uid, typ, payload)
	case PhaseTrumpFight:
		return reduceTrumpFight(st, uid, typ, payload)
	case PhasePlayTrick:
		return reducePlayTrick(st, uid, typ, payload)
	default:
		return ReduceResult{State: st, Changed: false}, fmt.Errorf("unknown_phase")
	}
}

func reduceLobby(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, error) {
	switch typ {
	case EvSit:
		p := payload.(SitPayload)
		if p.Seat < 0 || p.Seat >= 4 {
			return ReduceResult{State: st}, fmt.Errorf("bad_seat")
		}
		seat := &st.Seats[p.Seat]
		if seat.UID != "" && seat.UID != uid {
			return ReduceResult{State: st}, fmt.Errorf("seat_taken")
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
		return ReduceResult{State: st, Changed: true}, nil

	case EvLeave:
		// uid 离开自己座位
		for i := 0; i < 4; i++ {
			if st.Seats[i].UID == uid {
				st.Seats[i] = SeatState{}
				st.Version++
				return ReduceResult{State: st, Changed: true}, nil
			}
		}
		return ReduceResult{State: st}, fmt.Errorf("not_seated")

	case EvReady, EvUnready:
		wantReady := typ == EvReady
		for i := 0; i < 4; i++ {
			if st.Seats[i].UID == uid {
				st.Seats[i].Ready = wantReady
				st.Version++
				rr := ReduceResult{State: st, Changed: true}

				// 自动 start：4 人都 ready
				if allReady(st) {
					st = startDeal(st)
					rr.State = st
					rr.Notice = "所有人已准备，已发牌"
				}
				return rr, nil
			}
		}
		return ReduceResult{State: st}, fmt.Errorf("not_seated")

	case EvStart:
		// 手动 start：仅当 4 人都 ready（可限制房主）// 可去掉，有自动
		if !allReady(st) {
			return ReduceResult{State: st}, fmt.Errorf("not_all_ready")
		}
		st = startDeal(st)
		return ReduceResult{State: st, Changed: true, Notice: "manual_deal"}, nil

	default:
		return ReduceResult{State: st}, fmt.Errorf("unknown_event")
	}
}

func startDeal(st GameState) GameState {
	// 进入dealing
	st.Phase = PhaseDealing
	st.Version++

	// 生成两副牌并洗牌发牌
	deck := rules.NewDoubleDeck()
	rules.ShuffleInPlace(deck)
	hands, bottom := rules.Deal25PlusBottom8(deck)

	// 写入座位手牌
	for i := 0; i < 4; i++ {
		st.Seats[i].Hand = hands[i]
		st.Seats[i].HandCount = len(hands[i]) // 25

		team := st.Seats[i].Team
		level := st.Teams[team].LevelRank

		// 发牌阶段：先按“本队级牌 + 无主花色”排序，便于判断能否定主
		rules.SortHand(st.Seats[i].Hand, rules.SortCtx{
			LevelRank:    level,
			HasTrumpSuit: false,
		})
	}

	// 写入底牌
	st.Bottom = bottom
	st.BottomCount = len(bottom) // 8
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
			LevelRank:    "",
		},
		Locked:     false,
		CallerSeat: -1,
	}

	// 发牌结束后进入下一阶段（定主）
	st.Phase = PhaseCallTrump
	st.Version++
	return st
}

func reduceCallTrump(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, error) {
	seat, ok := seatIndexByUID(st, uid)
	if !ok {
		return ReduceResult{State: st}, fmt.Errorf("未确认座位号")
	}
	if st.CallMode == CallModeOrdered {
		if seat != st.CallTurnSeat {
			return ReduceResult{State: st}, fmt.Errorf("请等待%v号位定主", st.CallTurnSeat)
		}
	}
	if st.CallMode == CallModeRace && st.StarterSeat >= 0 {
		return ReduceResult{State: st}, fmt.Errorf("手慢啦，%v号位已定主", st.StarterSeat)
	}

	switch typ {

	case EvCallPass:
		bit := uint8(1 << uint(seat))
		if (st.CallPassMask & bit) != 0 {
			return ReduceResult{State: st}, fmt.Errorf("重复点击跳过")
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

			// 首局抢定主失败时，先手按 seat0
			leaderSeat := 0
			if st.CallMode == CallModeOrdered {
				leaderSeat = st.StarterSeat
			}
			st.StarterSeat = leaderSeat
			leaderTeam := st.Seats[leaderSeat].Team
			st.Trump.LevelRank = st.Teams[leaderTeam].LevelRank

			// 重排（硬主视角：每人按自己队级牌 + 无主）
			st = sortAllHands(st)

			st.BottomOwnerSeat = -1
			st.Phase = PhasePlayTrick
			st.Trick = TrickState{
				LeaderSeat: st.StarterSeat,
				TurnSeat:   st.StarterSeat,
				Lead:       LeadMove{Seat: -1},
				Follows:    [3]FollowMove{{Seat: -1}, {Seat: -1}, {Seat: -1}},
			}
			st.Version++
			return ReduceResult{State: st, Changed: true, Notice: "硬主"}, nil
		}

		return ReduceResult{State: st, Changed: true}, nil

	case EvCallTrump:
		p := payload.(CallTrumpPayload)
		// 初步校验，必须包含手牌中的 jokerId + 1/2 张 levelIds
		if p.JokerID < 0 || len(p.LevelIDs) == 0 {
			return ReduceResult{State: st}, fmt.Errorf("缺少王牌/级牌")
		}
		hand := st.Seats[seat].Hand
		joker, ok := findCardByID(hand, p.JokerID)
		if !ok {
			return ReduceResult{State: st}, fmt.Errorf("手牌中没有此王牌")
		}
		levelCards := make([]rules.Card, 0, len(p.LevelIDs))
		for _, id := range p.LevelIDs {
			c, ok := findCardByID(hand, id)
			if !ok {
				return ReduceResult{State: st}, fmt.Errorf("手牌中没有此级牌")
			}
			levelCards = append(levelCards, c)
		}
		// 校验定主规则
		team := st.Seats[seat].Team
		teamLevel := st.Teams[team].LevelRank
		trumpSuit, locked, err := rules.ValidateCallTrump(teamLevel, joker, levelCards)
		if err != nil {
			return ReduceResult{State: st}, err
		}
		// 定主成功：写入 Trump
		st.Trump.HasTrumpSuit = true
		st.Trump.Suit = trumpSuit
		st.Trump.LevelRank = teamLevel
		st.Trump.Locked = locked
		st.Trump.CallerSeat = seat
		st.StarterSeat = seat

		// 全员按“本小局最终级牌 + 主花色”重排
		st = sortAllHands(st)

		// 进入下一阶段：坐家收底牌、重扣底牌
		st = enterBottomPhase(st, seat)
		return ReduceResult{State: st, Changed: true,
			Notice: "成功定主，级牌为" + string(st.Trump.LevelRank) + "，主牌为" + string(st.Trump.Suit) + "，请扣底牌",
		}, nil

	default:
		return ReduceResult{State: st}, fmt.Errorf("非正常游戏阶段: %s", typ)
	}
}

func enterBottomPhase(st GameState, ownerSeat int) GameState {
	st.BottomOwnerSeat = ownerSeat

	// 把底牌“复制”进坐家手牌 -> 33张
	st.Seats[ownerSeat].Hand = append(st.Seats[ownerSeat].Hand, st.Bottom...)
	st.Seats[ownerSeat].HandCount = len(st.Seats[ownerSeat].Hand) // 33
	rules.SortHand(st.Seats[ownerSeat].Hand, rules.SortCtx{
		LevelRank:    st.Trump.LevelRank,
		HasTrumpSuit: st.Trump.HasTrumpSuit,
		TrumpSuit:    st.Trump.Suit,
	})

	st.Phase = PhaseBottom
	st.Version++
	return st
}

func reduceBottom(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, error) {
	seat, ok := seatIndexByUID(st, uid)
	if !ok {
		return ReduceResult{State: st}, fmt.Errorf("未确认座位号")
	}
	if seat != st.BottomOwnerSeat {
		return ReduceResult{State: st}, fmt.Errorf("非底牌所有者")
	}

	switch typ {
	case EvPutBottom:
		p := payload.(PutBottomPayload)
		if len(p.DiscardIDs) != 8 {
			return ReduceResult{State: st}, fmt.Errorf("牌数错误，应为8张")
		}
		seen := map[int]bool{}
		for _, id := range p.DiscardIDs {
			if seen[id] {
				return ReduceResult{State: st}, fmt.Errorf("不可重复选择牌")
			}
			seen[id] = true
		}
		// 校验这些牌都在坐家33张手牌里
		hand := st.Seats[seat].Hand
		newBottom, err := checkAllInHand(p.DiscardIDs, hand)
		if err != nil {
			return ReduceResult{State: st}, err
		}
		// 从手牌移除这8张 -> 回到25
		keep := deleteCardFromHand(hand, newBottom)
		if len(keep) != 25 {
			log.Fatalf("扣牌处理错误，手牌不足25张")
		}
		st.Seats[seat].Hand = keep
		st.Seats[seat].HandCount = 25
		st.Bottom = newBottom
		st.BottomCount = 8

		// 扣底完成：进入改主/攻主窗口（PhaseTrumpFight）
		st = enterTrumpFight(st)
		return ReduceResult{State: st, Changed: true, Notice: "完成扣牌"}, nil

	default:
		return ReduceResult{State: st}, fmt.Errorf("phase_forbid: %s", typ)
	}
}

func enterTrumpFight(st GameState) GameState {
	st.FightPassMask = 0
	st.FightPassCount = 0
	st.Phase = PhaseTrumpFight
	st.Version++
	return st
}

func reduceTrumpFight(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, error) {
	seat, ok := seatIndexByUID(st, uid)
	if !ok {
		return ReduceResult{State: st}, fmt.Errorf("未确定座位")
	}

	// 坐家不能“跳过改主/攻主”，也不参与计数
	if seat == st.BottomOwnerSeat {
		return ReduceResult{State: st}, fmt.Errorf("phase_forbid: %s", typ)
	}

	switch typ {
	case EvCallPass:
		bit := uint8(1 << uint(seat))
		if (st.FightPassMask & bit) != 0 {
			return ReduceResult{State: st}, fmt.Errorf("已点击跳过")
		}
		st.FightPassMask |= bit
		st.FightPassCount++
		st.Version++
		// 其余三位都跳过
		if st.FightPassCount >= 3 {
			st.Phase = PhasePlayTrick
			st.Trick = TrickState{
				LeaderSeat: st.StarterSeat,
				TurnSeat:   st.StarterSeat,
				Lead:       LeadMove{Seat: -1},
				Follows:    [3]FollowMove{{Seat: -1}, {Seat: -1}, {Seat: -1}},
			}
			st.Version++
			return ReduceResult{State: st, Changed: true, Notice: "无人继续改/攻主，进入出牌阶段"}, nil
		}
		return ReduceResult{State: st, Changed: true}, nil

	case EvChangeTrump:
		if st.Trump.Locked {
			return ReduceResult{State: st}, fmt.Errorf("主家已锁定花色，不可以改主")
		}
		// 初步校验
		p := payload.(ChangeTrumpPayload)
		if p.JokerID <= 0 || len(p.LevelIDs) != 2 {
			return ReduceResult{State: st}, fmt.Errorf("改主牌非法")
		}
		hand := st.Seats[seat].Hand
		joker, ok := findCardByID(hand, p.JokerID)
		if !ok {
			return ReduceResult{State: st}, fmt.Errorf("无此改主牌")
		}
		c1, ok := findCardByID(hand, p.LevelIDs[0])
		if !ok {
			return ReduceResult{State: st}, fmt.Errorf("无此改主牌")
		}
		c2, ok := findCardByID(hand, p.LevelIDs[1])
		if !ok {
			return ReduceResult{State: st}, fmt.Errorf("无此改主牌")
		}
		// 改主规则校验
		trumpSuit, err := rules.ValidateChangeTrump(st.Trump.LevelRank, joker, c1, c2)
		if err != nil {
			return ReduceResult{State: st}, err
		}
		// 改主成功：只改主花色，不改 LevelRank / StarterSeat
		st.Trump.HasTrumpSuit = true
		st.Trump.Suit = trumpSuit
		st = sortAllHands(st)
		// 改主者成为 bottomOwner，拿当前底牌并扣底
		st = enterBottomPhase(st, seat)
		return ReduceResult{State: st, Changed: true, Notice: "改主成功，变为花色" + string(st.Trump.Suit)}, nil

	case EvAttackTrump:
		// 初步校验
		p := payload.(AttackTrumpPayload)
		if len(p.JokerIDs) != 2 {
			return ReduceResult{State: st}, fmt.Errorf("攻主牌非法")
		}
		hand := st.Seats[seat].Hand
		j1, ok := findCardByID(hand, p.JokerIDs[0])
		if !ok {
			return ReduceResult{State: st}, fmt.Errorf("无此攻主牌")
		}
		j2, ok := findCardByID(hand, p.JokerIDs[1])
		if !ok {
			return ReduceResult{State: st}, fmt.Errorf("无此攻主牌")
		}
		// 攻主规则校验
		err := rules.ValidateAttackTrump(j1, j2)
		if err != nil {
			return ReduceResult{State: st}, err
		}
		// 攻主成功：进入硬主（只影响主花色体系），不改 LevelRank/StarterSeat
		st.Trump.HasTrumpSuit = false
		st.Trump.Suit = ""
		st = sortAllHands(st)
		// 攻主者成为 bottomOwner，拿底扣底
		st = enterBottomPhase(st, seat)
		return ReduceResult{State: st, Changed: true, Notice: "攻主成功，本小局硬主"}, nil

	default:
		return ReduceResult{State: st}, fmt.Errorf("phase_forbid: %s", typ)
	}
}

func reducePlayTrick(st GameState, uid string, typ ClientEventType, payload any) (ReduceResult, error) {
	// 初步校验
	if typ != EvPlayCards {
		return ReduceResult{State: st}, fmt.Errorf("phase_forbid: %s", typ)
	}
	seat, ok := seatIndexByUID(st, uid)
	if !ok {
		return ReduceResult{State: st}, fmt.Errorf("未选择座位")
	}
	if seat != st.Trick.TurnSeat || seat != st.Trick.LeaderSeat {
		return ReduceResult{State: st}, fmt.Errorf("非出牌者，请等待玩家%v出牌", st.Trick.TurnSeat)
	}
	if st.Trick.Lead.Seat != -1 {
		return ReduceResult{State: st}, fmt.Errorf("已出过牌，请等待本回合结束")
	}
	p := payload.(PlayCardsPayload)
	if len(p.CardIDs) == 0 {
		return ReduceResult{State: st}, fmt.Errorf("出牌数为0")
	}
	// 校验所出牌都在手牌里、无重复
	selected, err := checkAllInHand(p.CardIDs, st.Seats[seat].Hand)
	if err != nil {
		return ReduceResult{State: st}, err
	}
	// 先手约束：必须全是同SuitClass（同副花色 或 全主）
	sc, ok := rules.ComputeSuitClassAllSame(selected, st.Trump.Trump)
	if !ok {
		return ReduceResult{State: st}, fmt.Errorf("出牌花色不一致")
	}
	// 将所出牌解析为多组Block，同一种牌型归于一组
	blockGroups, err := rules.DecomposeThrow(selected, st.Trump.Trump, sc)
	if err != nil {
		return ReduceResult{State: st}, err
	}
	// 先默认leadMove为基础牌型
	leadMove := LeadMove{
		Seat:      seat,
		SuitClass: sc,
		ActualMove: Move{
			Blocks:  blockGroups,
			CardIDs: p.CardIDs,
			Cards:   selected,
		},
	}
	// 如果有多种牌型，或者有多个同牌型，则视为甩牌，需进行判定并调整leadMove
	if len(blockGroups) > 1 || len(blockGroups[0]) > 1 {
		leadMove.IsThrow = true
		leadMove.IntentMove = cloneMove(leadMove.ActualMove)
		// 甩牌判定，若失败则更新selected，blockGroups
		throwOK, actualMove, info, err := canonicalizeLead(st, seat, sc, leadMove.IntentMove)
		if err != nil {
			return ReduceResult{State: st}, err
		}
		leadMove.ThrowOK = throwOK
		// 若甩牌失败，重新修改出牌
		if !throwOK {
			leadMove.ActualMove = actualMove
			leadMove.Info = info
		}
	}
	// 从手牌移除实际出的牌
	st.Seats[seat].Hand = deleteCardFromHand(st.Seats[seat].Hand, leadMove.ActualMove.Cards)
	st.Seats[seat].HandCount = len(st.Seats[seat].Hand)
	// 更新trick, 记录 lead
	st.Trick.Lead = leadMove
	st.Trick.TurnSeat = (seat + 1) % 4
	st.Version++
	return ReduceResult{State: st, Changed: true, Notice: "已出牌" + leadMove.Info}, nil
}

// canonicalizeLead 对先手选牌进行甩牌判定
// 返回：是否甩牌成功、最终出牌blocks、cards与id、失败原因
func canonicalizeLead(st GameState, leaderSeat int, sc rules.SuitClass, intentMove Move) (bool, Move, string, error) {
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
				return false, Move{}, "", err
			}
			if len(defBlocks) == 0 {
				continue
			}
			defMax := defBlocks[0]
			// 同type同suitClass比较
			if rules.CompareTwoBlocks(throwMin, defMax) < 0 {
				// 甩牌失败：最终出牌结果=该组最小的 throwMin
				return false, Move{
					Blocks:  [][]rules.Block{{throwMin}},
					Cards:   throwMin.Cards,
					CardIDs: getIDs(throwMin.Cards),
				}, fmt.Sprintf("甩牌失败，原计划甩出%v张%v牌", len(intentMove.CardIDs), sc), nil
			}
		}
	}
	// 无人可压，甩牌成功
	return true, intentMove, "甩牌成功", nil
}
