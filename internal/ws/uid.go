package ws

import (
	"strings"
	"unicode"
)

func normalizeAnyUID(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// 移除控制字符
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)

	// 限制长度
	const maxRunes = 24
	rs := []rune(s)
	if len(rs) > maxRunes {
		s = string(rs[:maxRunes])
	}
	return s
}
