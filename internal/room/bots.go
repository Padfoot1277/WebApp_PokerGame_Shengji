package room

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/transport"
)

// Bots are state-machine actors, not socket clients. Computation happens outside
// the room loop, with a global bound and version checking before committing.
var botWorkers = make(chan struct{}, 4)

type botResult struct {
	version int64
	uid     string
	action  bot.Action
	err     error
}

func (r *Room) changeBot(c transport.Client, add bool) *game.AppError {
	if r.state.Phase != game.PhaseLobby {
		return game.ErrStateWrongPhase.WithInfo("仅可在准备界面增减陪练")
	}
	if add {
		empty := -1
		order := uint64(0)
		for i, s := range r.state.Seats {
			if s.UID == "" && empty < 0 {
				empty = i
			}
			if s.BotOrder > order {
				order = s.BotOrder
			}
		}
		if empty < 0 {
			return game.ErrInvalidPayload.WithInfo("房间已满，没有空位")
		}
		r.state.Seats[empty] = game.SeatState{UID: "bot:" + rand.Text(), Name: fmt.Sprintf("陪练%d", order+1), Team: game.TeamOfSeat(empty), IsBot: true, BotOrder: order + 1, Online: true}
		r.notice = "已增加一位陪练"
	} else {
		first := -1
		for i, s := range r.state.Seats {
			if s.IsBot && (first < 0 || s.BotOrder < r.state.Seats[first].BotOrder) {
				first = i
			}
		}
		if first < 0 {
			return game.ErrInvalidPayload.WithInfo("房间内没有陪练")
		}
		r.state.Seats[first] = game.SeatState{Team: game.TeamOfSeat(first)}
		r.notice = "已移除最先添加的陪练"
	}
	r.state.Version++
	return nil
}
func (r *Room) rememberTrick() {
	if r.historyRound != r.state.RoundIndex {
		r.botHistory = nil
		r.historyRound = r.state.RoundIndex
		r.historyTrick = 0
	}
	if r.state.TrickIndex > r.historyTrick {
		r.botHistory = append(r.botHistory, bot.PublicTrick{Leader: r.state.Trick.LastLeaderSeat, Plays: r.state.Trick.LastPlays})
		r.historyTrick = r.state.TrickIndex
	}
}
func (r *Room) scheduleBot(now time.Time) {
	if r.botBusy || now.Before(r.botNext) || len(r.conns) == 0 || r.loadErr != nil {
		return
	}
	r.rememberTrick()
	// Rotate across seats so all eligible bots get a chance in race/lobby phases.
	for off := 0; off < 4; off++ {
		seat := (r.botCursor + off) % 4
		s := r.state.Seats[seat]
		if !s.IsBot {
			continue
		}
		eligible := false
		switch r.state.Phase {
		case game.PhaseLobby:
			eligible = !s.Ready
		case game.PhaseCallTrump:
			eligible = r.state.CallPassMask&(1<<uint(seat)) == 0 && (r.state.CallMode == game.CallModeRace || r.state.CallTurnSeat == seat)
		case game.PhaseTrumpFight:
			eligible = r.state.FightTurnSeat == seat && r.state.FightPassMask&(1<<uint(seat)) == 0
		case game.PhaseBottom:
			eligible = r.state.BottomOwnerSeat == seat
		case game.PhasePlayTrick:
			eligible = r.state.Trick.TurnSeat == seat
		case game.PhaseRoundSettle:
			eligible = r.state.NextStarterSeat == seat
		}
		if (r.state.Phase == game.PhaseBottom || r.state.Phase == game.PhaseTrumpFight || r.state.Phase == game.PhasePlayTrick) && r.state.SurrenderMask&(1<<uint((seat+2)%4)) != 0 && r.state.SurrenderMask&(1<<uint(seat)) == 0 {
			eligible = true
		}
		if !eligible || r.botBlocked == r.state.Version && r.botBlockedUID == s.UID {
			continue
		}
		select {
		case botWorkers <- struct{}{}:
		default:
			return
		}
		r.botBusy = true
		r.botCursor = (seat + 1) % 4
		table := bot.Observe(r.state, seat)
		view := table.View
		history := append([]bot.PublicTrick(nil), r.botHistory...)
		go func() {
			result := botResult{version: view.Version, uid: s.UID}
			defer func() {
				if recovered := recover(); recovered != nil {
					result.err = fmt.Errorf("bot policy panic: %v", recovered)
				}
				<-botWorkers
				select {
				case r.botResults <- result:
				case <-r.done:
				}
			}()
			result.action, result.err = bot.DecideFull(table, history)
		}()
		return
	}
}
func (r *Room) applyBot(result botResult) {
	r.botBusy = false
	r.botNext = time.Now().Add(time.Second)
	if result.version != r.state.Version {
		return
	}
	if result.err != nil {
		r.blockBot(result, result.err)
		return
	}
	if result.action.Type == "" {
		return
	}
	res, err := game.Reduce(r.state, result.uid, game.ClientEventType(result.action.Type), result.action.Payload)
	if err != nil {
		r.blockBot(result, err)
		return
	}
	if !res.Changed {
		return
	}
	before := r.state
	r.state = res.State
	if err := r.save(); err != nil {
		r.state = before
		r.blockBot(result, err)
		return
	}
	r.rememberTrick()
	if res.Notice != "" {
		for c := range r.conns {
			_ = c.SendJSON(game.NoticeMsg{Type: "notice", Message: res.Notice})
		}
	}
	r.broadcastSnapshot()
}
func (r *Room) blockBot(result botResult, err error) {
	r.botBlocked = result.version
	r.botBlockedUID = result.uid
	slog.Warn("bot action paused", "room", r.id, "event", result.action.Type, "error", err)
	for c := range r.conns {
		_ = c.SendJSON(game.NoticeMsg{Type: "notice", Message: "陪练暂时无法完成当前动作，请重连后重试"})
	}
}
