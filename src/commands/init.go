package commands

import (
	"poll-bot/src/commands/ping"
	"poll-bot/src/commands/rate"

	"github.com/bwmarrin/discordgo"
)

var infos = []*discordgo.ApplicationCommand{}

func GetCommands() []*discordgo.ApplicationCommand {
	return infos
}

func Register(info *discordgo.ApplicationCommand, cb func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	infos = append(infos, info)
	callbacks[info.Name] = cb
}

var callbacks = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error{}

func GetCallbacks() map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return callbacks
}

type Package struct {
	Identifiers *[]*discordgo.ApplicationCommand
	Handlers    *map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error
}

var CommandPackage = Package{
	Identifiers: &infos,
	Handlers:    &callbacks,
}

func init() {
	ping.Register(&infos, &callbacks)
	rate.Register(&infos, &callbacks)
}
