package commands

import (
	"poll-bot/root/commands/alias"
	"poll-bot/root/commands/modrank"
	"poll-bot/root/commands/ping"
	"poll-bot/root/commands/rank"
	"poll-bot/root/commands/rate"
	"poll-bot/root/fileio/aliases"
	"poll-bot/root/fileio/authorize"
	"poll-bot/root/types"
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
