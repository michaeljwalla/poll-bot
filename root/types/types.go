package types

import (
	"poll-bot/root/managers/aliases"
	"poll-bot/root/managers/audit"
	"poll-bot/root/managers/authorize"
	"poll-bot/root/managers/components"
	"poll-bot/root/managers/polls"

	"github.com/bwmarrin/discordgo"
)

type CommandMetadata struct {
	MinTrustLevel authorize.Rank
}
type EventCallback = func(s *discordgo.Session, i *discordgo.InteractionCreate) error

type CommandInfo struct {
	DGInfo   *discordgo.ApplicationCommand
	Metadata CommandMetadata
	Callback EventCallback
}

type BotCommandPackage struct {
	Handles    *map[string]CommandInfo
	Aliases    *aliases.AliasManager
	Auth       *authorize.AuthManager
	Polls      *polls.PollManager
	Components *components.ComponentCallbackManager
}

type SessionCommandRegisters struct {
	Reference BotCommandPackage
}
type Session struct {
	DGSession *discordgo.Session
	Registers []*discordgo.ApplicationCommand
	//
	Logger  *audit.Log
	Package BotCommandPackage
}

type StartInstructions struct {
	Token    string
	Commands *BotCommandPackage
	Logger   *audit.Log
}
