package ws

import (
	"strings"
	"testing"
)

func TestSessionIdentityAndTampering(t *testing.T) {
	a, token, err := session("", "同名")
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := session("", "同名")
	if a.UID == b.UID {
		t.Fatal("same nickname shares identity")
	}
	restored, _, err := session(token, "")
	if err != nil || restored.UID != a.UID {
		t.Fatal("session not restored")
	}
	if _, _, err = session(strings.Replace(token, ".", ".x", 1), ""); err == nil {
		t.Fatal("tampered token accepted")
	}
}
func TestAuthenticatedNicknameChangeKeepsSeatIdentity(t *testing.T) {
	id, token, err := session("", "原昵称")
	if err != nil {
		t.Fatal(err)
	}
	updated, newToken, err := session(token, "新昵称")
	if err != nil || updated.UID != id.UID || updated.Name != "新昵称" || newToken == token {
		t.Fatal("nickname update lost identity")
	}
	restored, _, err := session(newToken, "")
	if err != nil || restored.Name != "新昵称" {
		t.Fatal("new nickname token invalid")
	}
}

func TestRecoveredSeatReceivesValidIdentity(t *testing.T) {
	c := &Conn{uid: "new", name: "牌友", allowNicknameRecovery: true}
	if !c.RecoverSeat("original") {
		t.Fatal("recovery failed")
	}
	id, _, err := session(c.Token(), "牌友")
	if err != nil || id.UID != "original" || c.UID() != "original" {
		t.Fatal("recovered identity invalid")
	}
}
