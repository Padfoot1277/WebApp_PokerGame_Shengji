package main

import (
	"encoding/json"
	"fmt"
	"os"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
)

func main() {
	var input bot.Input
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fail(err)
	}
	a, err := bot.Decide(input.ViewState, input.History, false)
	if err != nil {
		fail(err)
	}
	if len(os.Args) > 1 && os.Args[1] == "--action" {
		_ = json.NewEncoder(os.Stdout).Encode(a)
		return
	}
	if a.Type != "game.play_cards" {
		fail(fmt.Errorf("not a play action"))
	}
	_ = json.NewEncoder(os.Stdout).Encode(a.Payload.(game.PlayCardsPayload).CardIDs)
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
