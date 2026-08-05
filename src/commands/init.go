package commands

import (
	"poll-bot/src/commands/ping"
	"poll-bot/src/commands/rate"
	"poll-bot/src/types"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var handles = map[string]CommandInfo{}

var CommandPackage = types.BotCommandPackage{
	Handles: &handles,
}

func init() {
	ping.Register(&handles)
	rate.Register(&handles)
}
