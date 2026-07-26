package bot

import (
	"fmt"
	"poll-bot/internal/audit"
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
	//
	Logger        *audit.Log
	TargetAliases map[string]string
}

type StartInstructions struct {
	Token         string
	Commands      *Commands
	Logger        *audit.Log
	TargetAliases map[string]string
}

func resolveAlias(s string, as map[string]string) string {
	if value, ok := as[s]; ok {
		return value
	}
	return "?"
}

// 2. Initialize a new Discord
func Start(instr StartInstructions) (session *Session, err error) {
	token, commands, logger, aliases := instr.Token, instr.Commands, instr.Logger, instr.TargetAliases
	dgSession, err := discordgo.New("Bot " + token)
	if err != nil {
		return
	}

	logger.Add("Begin bot init", audit.LogGroup.BOT)
	// set perms "intents" & register gateway handler boilerplate
	dgSession.Identify.Intents = discordgo.IntentsGuilds
	dgSession.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionType(discordgo.InteractionContextGuild) {
			// Handle component interactions or autocomplete here if needed
		}
		commandData := i.ApplicationCommandData()
		if handle, ok := commands.Handlers[commandData.Name]; ok {
			id := i.Member.User.ID
			alias := resolveAlias(id, aliases)

			logger.Add(fmt.Sprintf("%v %33s) | /%s", id, "("+alias, commandData.Name), audit.LogGroup.BOT, audit.LogGroup.INTERACT)
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
			logger.Panic(fmt.Sprintf("Cannot create '%v' command: %v", cmd.Name, err), audit.LogGroup.BOT)
		}
		registeredCommands[idx] = rcmd
	}

	session = &Session{
		DGSession: dgSession,
		Registers: CommandRegisters{
			Reference: commands,
			DGObjects: registeredCommands,
		},
		Logger:        logger,
		TargetAliases: aliases,
	}
	logger.Add("Init success; bot online", audit.LogGroup.BOT)
	return
}

func EndSession(session *Session) {
	session.Logger.Add("Ending session...", audit.LogGroup.BOT)
	defer func() {
		session.DGSession.Close()
		session.Logger.Add("Session ended\n", audit.LogGroup.BOT)
	}()
	// 8. Clean up and remove commands upon shutdown
	dgSession := session.DGSession

	for _, cmd := range session.Registers.DGObjects {
		err := dgSession.ApplicationCommandDelete(dgSession.State.User.ID, "", cmd.ID)
		if err != nil {
			session.Logger.Warn(fmt.Sprintf("Cannot delete '%v' command: %v", cmd.Name, err), audit.LogGroup.BOT)
		}
	}
}
