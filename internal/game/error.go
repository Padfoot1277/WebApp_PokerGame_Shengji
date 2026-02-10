package game

import (
	"fmt"
)

// AppError 是整个后端统一使用的错误类型
// - Code: 稳定、可机读，用于前端/UI/测试/replay
// - Msg:  给人看的简要信息（可英文，前端可自行映射文案）
type AppError struct {
	Code string `json:"code"`
	Msg  string `json:"message"`
	Info string `json:"info"`
}

func (e *AppError) Error() string {
	if e.Info != "" {
		return fmt.Sprintf("%s (%s)", e.Info, e.Code)
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

// WithInfo 给已有 AppError 附加 Info
func (e *AppError) WithInfo(info string) *AppError {
	if e == nil {
		return nil
	}
	e.Info = info
	return e
}

func (e *AppError) WithInfof(format string, a ...any) *AppError {
	if e == nil {
		return nil
	}
	e.Info = fmt.Sprintf(format, a...)
	return e
}

func (e *AppError) ClearInfo() *AppError {
	if e == nil {
		return nil
	}
	e.Info = ""
	return e
}

// 以下 err 变量会在各处使用，要注意 Info 的更改/清空（使用 WithInfo / ClearInfo），否则会出现错误的提示信息
// ---------- 协议 / Payload 校验错误（router 层）----------
var (
	ErrBadJSON        = NewErr("PROTO_BAD_JSON", "JSON 数据格式错误")
	ErrUnknownEvent   = NewErr("PROTO_UNKNOWN_EVENT", "未知的事件类型")
	ErrInvalidPayload = NewErr("PROTO_INVALID_PAYLOAD", "请求数据不合法")
	ErrDuplicateIDs   = NewErr("PROTO_DUPLICATE_IDS", "存在重复的卡牌 ID")
	ErrEmptyCards     = NewErr("PROTO_EMPTY_CARDS", "未选择任何卡牌")
	ErrSeatRange      = NewErr("PROTO_SEAT_OUT_OF_RANGE", "座位号超出范围")
	ErrDuplicateOps   = NewErr("PROTO_DUPLICATE_OPS", "存在重复的操作")
	ErrWrongCardsNum  = NewErr("PROTO_WRONG_CARD_NUM", "卡牌数量不正确")
)

// ---------- 规则拒绝错误（rules 层）----------
var (
	ErrRuleIllegalPlay   = NewErr("RULE_ILLEGAL_PLAY", "出牌不符合规则")
	ErrRuleIllegalFollow = NewErr("RULE_ILLEGAL_FOLLOW", "跟牌不符合规则")
	ErrRuleIllegalTrump  = NewErr("RULE_ILLEGAL_TRUMP", "主牌使用不合法")
)

// ---------- 状态机错误（game / engine 层）----------
var (
	ErrStateNotYourTurn = NewErr("STATE_NOT_YOUR_TURN", "未轮到你操作")
	ErrStateWrongPhase  = NewErr("STATE_WRONG_PHASE", "当前阶段不允许该操作")
	ErrStateNotSeated   = NewErr("STATE_NOT_SEATED", "玩家尚未入座")
	ErrStateSeatTaken   = NewErr("STATE_TAKEN", "该座位已被占用")
	ErrStateNotReady    = NewErr("STATE_NOT_READY", "玩家尚未准备")
)

// ---------- 系统错误（不可恢复，通常只记日志）----------
var (
	ErrSystem = NewErr("SYS_INTERNAL_ERROR", "服务器内部错误")
	ErrFatal  = NewErr("SYS_FATAL_ERROR", "服务器发生严重错误")
)
