package game

import (
	"fmt"
)

// AppError 是整个后端统一使用的错误类型
// - Code: 稳定、可机读，用于前端/UI/测试/replay
// - Msg:  给人看的简要信息（可英文，前端可自行映射文案）
type AppError struct {
	Code  string `json:"code"`
	Msg   string `json:"message"`
	Cause string `json:"info"`
}

func (e *AppError) Error() string {
	if e.Cause != "" {
		return fmt.Sprintf("%s (%s: %s)", e.Cause, e.Code, e.Msg)
	}
	return fmt.Sprintf("%s (%s)", e.Msg, e.Code)
}

// NewErr 创建一个最基础的 AppError
func NewErr(code, msg string) *AppError {
	return &AppError{
		Code: code,
		Msg:  msg,
	}
}

// WithCause 给已有 AppError 附加 Cause
func (e *AppError) WithCause(info string) *AppError {
	if e == nil {
		return nil
	}
	e.Cause = info
	return e
}

func (e *AppError) WithCausef(format string, a ...any) *AppError {
	if e == nil {
		return nil
	}
	e.Cause = fmt.Sprintf(format, a...)
	return e
}

// ---------- 协议 / Payload 校验错误（router 层）----------

var (
	ErrBadJSON        = NewErr("PROTO_BAD_JSON", "invalid json payload")
	ErrUnknownEvent   = NewErr("PROTO_UNKNOWN_EVENT", "unknown event type")
	ErrInvalidPayload = NewErr("PROTO_INVALID_PAYLOAD", "invalid payload")
	ErrDuplicateIDs   = NewErr("PROTO_DUPLICATE_IDS", "duplicate card ids")
	ErrEmptyCards     = NewErr("PROTO_EMPTY_CARDS", "no cards selected")
	ErrSeatRange      = NewErr("PROTO_SEAT_OUT_OF_RANGE", "seat out of range")
	ErrDuplicateOps   = NewErr("PROTO_DUPLICATE_OPS", "duplicate operations")
)

// ---------- 规则拒绝错误（rules 层）----------

var (
	ErrRuleIllegalPlay   = NewErr("RULE_ILLEGAL_PLAY", "illegal play")
	ErrRuleIllegalFollow = NewErr("RULE_ILLEGAL_FOLLOW", "illegal follow")
	ErrRuleIllegalTrump  = NewErr("RULE_ILLEGAL_TRUMP", "非法主牌")
	ErrRuleIncomparable  = NewErr("RULE_INCOMPARABLE", "cards not comparable")
	ErrRuleInvalidLevel  = NewErr("RULE_INVALID_LEVEL", "invalid level card")
	ErrRuleInvalidJoker  = NewErr("RULE_INVALID_JOKER", "invalid joker")
)

// ---------- 状态机错误（game / engine 层）----------

var (
	ErrStateNotYourTurn = NewErr("STATE_NOT_YOUR_TURN", "not your turn")
	ErrStateWrongPhase  = NewErr("STATE_WRONG_PHASE", "operation not allowed in current phase")
	ErrStateNotSeated   = NewErr("STATE_NOT_SEATED", "player not seated")
	ErrStateSeatTaken   = NewErr("STATE_Taken", "这个座位已经有人了")
	ErrStateNotReady    = NewErr("STATE_NOT_READY", "player not ready")
)

// ---------- 系统错误（不可恢复，通常只记日志）----------

var (
	ErrSystem = NewErr("SYS_INTERNAL_ERROR", "internal server error")
)
