package game

import (
	"fmt"
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

func allReady(st GameState) bool {
	for i := 0; i < 4; i++ {
		if st.Seats[i].UID == "" || !st.Seats[i].Ready {
			return false
		}
	}
	return true
}

func startDeal(st GameState) GameState {
	// 进入dealing
	st.Phase = PhaseDealing
	st.Version++

	// 生成两副牌并洗牌
	deck := rules.NewDoubleDeck()
	rules.ShuffleInPlace(deck)

	// 发 25*4 + 8底牌
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

	// 发牌结束后进入下一阶段（定主）
	st.Phase = PhaseCallTrump
	st.Version++

	return st
}
