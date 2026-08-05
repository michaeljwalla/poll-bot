package bot

import (
	"fmt"
	"poll-bot/src/audit"
	"poll-bot/src/authorize"
	"poll-bot/src/types"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type StartInstructions = types.StartInstructions
type Session = types.Session
type CommandRegisters = types.SessionCommandRegisters

func resolveAlias(s string, as map[string]string) string {
	if value, ok := as[s]; ok {
		return value
	}
	return "?"
}

func getCommandValue(opt *discordgo.ApplicationCommandInteractionDataOption) any {
	switch opt.Type {
	case discordgo.ApplicationCommandOptionString:
		return fmt.Sprintf("%q", opt.StringValue())
	case discordgo.ApplicationCommandOptionInteger:
		return opt.IntValue()
	case discordgo.ApplicationCommandOptionBoolean:
		return opt.BoolValue()
	case discordgo.ApplicationCommandOptionNumber:
		return opt.FloatValue()
	default:
		return opt.Value
	}
}
func rebuildCommand(c *discordgo.ApplicationCommandInteractionData) string {
	args := make([]string, len(c.Options)+1)
	args[0] = fmt.Sprintf("/%s", c.Name)
	for i, option := range c.Options {
		args[i+1] = fmt.Sprintf("%s:%v", option.Name, getCommandValue(option))
	}
	return strings.Join(args, " ")
}

// 2. Initialize a new Discord
func Start(instr StartInstructions) (session *Session, err error) {
	token, handles, logger, aliases, auth := instr.Token, instr.Commands.Handles, instr.Logger, instr.TargetAliases, instr.Authorizations
	dgSession, err := discordgo.New("Bot " + token)
	if err != nil {
		return
	}

	logger.Add("Begin bot init", audit.LogGroup.BOT)
	// set perms "intents" & register gateway handler boilerplate
	dgSession.Identify.Intents = discordgo.IntentsGuilds
	dgSession.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// if i.Type == discordgo.InteractionType(discordgo.InteractionContextGuild) {
		// 	// Handle component interactions or autocomplete here if needed
		// }
		commandData := i.ApplicationCommandData()
		if handle, ok := (*handles)[commandData.Name]; ok {
			id := i.Member.User.ID

			alias := resolveAlias(id, aliases)
			cmd := rebuildCommand(&commandData)
			var callback types.EventCallback
			if !authorize.CanUse(id, handle.Metadata.MinTrustLevel, auth) {
				logger.Add(fmt.Sprintf("%v %33s) %02d❌| %s", id, "("+alias, authorize.GetRank(id, auth), cmd), audit.LogGroup.BOT, audit.LogGroup.INTERACT)
				callback = nil // TODO
			} else {
				logger.Add(fmt.Sprintf("%v %33s) %02d✅| %s", id, "("+alias, authorize.GetRank(id, auth), cmd), audit.LogGroup.BOT, audit.LogGroup.INTERACT)
				callback = handle.Callback
			}

			if err := callback(s, i); err != nil {
				logger.Warn(fmt.Sprintf("While handling %s: %v", rebuildCommand(&commandData), err))
			}
		}
	})

	// take a wild guess
	err = dgSession.Open()
	if err != nil {
		return
	}

	// register slash commands
	registeredCommands := make([]*discordgo.ApplicationCommand, len(*handles))
	cur := 0
	for _, cmd := range *handles {
		rcmd, err := dgSession.ApplicationCommandCreate(dgSession.State.User.ID, "", cmd.DGInfo)
		if err != nil {
			logger.Panic(fmt.Sprintf("Cannot create '%v' command: %v", cmd.DGInfo.Name, err), audit.LogGroup.BOT)
		}
		registeredCommands[cur] = rcmd
		cur++
	}

	session = &Session{
		DGSession: dgSession,
		Registers: CommandRegisters{
			Reference: types.BotCommandPackage{Handles: handles},
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
		if err := session.DGSession.Close(); err != nil {
			session.Logger.Warn(fmt.Sprintf("Error while closing DGSession: %v", err))
		}
		session.Logger.Add("Session ended", audit.LogGroup.BOT)
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
