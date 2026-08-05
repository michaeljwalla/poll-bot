package commands

import (
	"poll-bot/src/aliases"
	"poll-bot/src/authorize"
	"poll-bot/src/commands/alias"
	"poll-bot/src/commands/modrank"
	"poll-bot/src/commands/ping"
	"poll-bot/src/commands/rank"
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
	rank.Register(&handles, &authorize.AuthTable)
	modrank.Register(&handles, &authorize.AuthTable)
	alias.Register(&handles, &aliases.AliasTable)
}
