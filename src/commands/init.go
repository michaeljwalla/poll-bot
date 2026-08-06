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

type RegisterReqs struct {
	Aliases *aliases.AliasTable
	Auth    *authorize.AuthTable
}

func Register(reqs RegisterReqs) *types.BotCommandPackage {
	handles := make(map[string]CommandInfo)
	bcp := types.BotCommandPackage{
		Handles: &handles,
		Aliases: reqs.Aliases,
		Auth:    reqs.Auth,
	}

	ping.Register(&bcp)
	rate.Register(&bcp)
	rank.Register(&bcp)
	modrank.Register(&bcp)
	alias.Register(&bcp)
	return &bcp
}
