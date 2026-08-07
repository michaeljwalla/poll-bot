package types

import (
	"poll-bot/src/aliases"
	"poll-bot/src/audit"
	"poll-bot/src/authorize"

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
	Handles *map[string]CommandInfo
	Aliases *aliases.AliasTable
	Auth    *authorize.AuthTable
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
