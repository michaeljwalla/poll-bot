package commands

import (
	"poll-bot/root/commands/alias"
	"poll-bot/root/commands/loadchannel"
	"poll-bot/root/commands/modrank"
	"poll-bot/root/commands/ping"
	"poll-bot/root/commands/rank"
	"poll-bot/root/commands/rate"
	"poll-bot/root/commands/status"
	"poll-bot/root/managers/aliases"
	"poll-bot/root/managers/authorize"
	"poll-bot/root/managers/components"
	"poll-bot/root/managers/polls"
	"poll-bot/root/types"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

type RegisterReqs struct {
	Aliases    *aliases.AliasManager
	Auth       *authorize.AuthManager
	Polls      *polls.PollManager
	Components *components.ComponentCallbackManager
}

var registers []func(*types.BotCommandPackage)

func init() {
	registers = []func(*types.BotCommandPackage){
		alias.Register,
		modrank.Register,
		ping.Register,
		rank.Register,
		rate.Register,
		status.Register,
		loadchannel.Register,
	}
}
func Register(reqs RegisterReqs) *types.BotCommandPackage {
	handles := make(map[string]CommandInfo)
	bcp := types.BotCommandPackage{
		Handles:    &handles,
		Aliases:    reqs.Aliases,
		Auth:       reqs.Auth,
		Polls:      reqs.Polls,
		Components: reqs.Components,
	}

	for _, reg := range registers {
		reg(&bcp)
	}
	return &bcp
}
