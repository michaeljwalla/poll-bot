package bot

import (
	"log"
	"poll-bot/internal/commands"

	"github.com/bwmarrin/discordgo"
)

type Commands = commands.Commands

type CommandRegisters struct {
	Reference *Commands
	DGObjects []*discordgo.ApplicationCommand
}
type Session struct {
	DGSession *discordgo.Session
	Registers CommandRegisters
}

// 2. Initialize a new Discord
func Start(token string, commands *Commands) (session *Session, err error) {
	dgSession, err := discordgo.New("Bot " + token)
	if err != nil {
		return
	}

	// set perms "intents" & register gateway handler boilerplate
	dgSession.Identify.Intents = discordgo.IntentsGuilds
	dgSession.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionType(discordgo.InteractionContextGuild) {
			// Handle component interactions or autocomplete here if needed
		}
		if handle, ok := commands.Handlers[i.ApplicationCommandData().Name]; ok {
			handle(s, i)
		}
	})

	// take a wild guess
	err = dgSession.Open()
	if err != nil {
		return
	}

	// register slash commands
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands.Identifiers))
	for idx, cmd := range commands.Identifiers {
		rcmd, err := dgSession.ApplicationCommandCreate(dgSession.State.User.ID, "", cmd)
		if err != nil {
			log.Fatalf("Cannot create '%v' command: %v", cmd.Name, err)
		}
		registeredCommands[idx] = rcmd
	}

	session = &Session{
		DGSession: dgSession,
		Registers: CommandRegisters{
			Reference: commands,
			DGObjects: registeredCommands,
		},
	}

	return
}

func EndSession(session *Session) {
	defer session.DGSession.Close()
	// 8. Clean up and remove commands upon shutdown
	dgSession := session.DGSession

	for _, cmd := range session.Registers.DGObjects {
		err := dgSession.ApplicationCommandDelete(dgSession.State.User.ID, "", cmd.ID)
		if err != nil {
			log.Printf("Cannot delete '%v' command: %v", cmd.Name, err)
		}
	}
}
