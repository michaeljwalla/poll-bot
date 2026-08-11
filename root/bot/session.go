package bot

import (
	"fmt"
	"poll-bot/root/managers/audit"
	"poll-bot/root/managers/authorize"
	"poll-bot/root/types"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type StartInstructions = types.StartInstructions
type Session = types.Session

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

func ack(s *discordgo.Session, i *discordgo.InteractionCreate, logger *audit.Log, value discordgo.InteractionResponseType) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: value,
	})
	if err != nil {
		logger.Warn(fmt.Sprintf("failed to defer: %s", err), audit.LogGroup.BOT, audit.LogGroup.INTERACT)
		return
	}
}

// 2. Initialize a new Discord
// TODO aliases and auth are now in Commands. fix the type errors
func Start(instr StartInstructions) (session *Session, err error) {
	token, handles, logger, aliases, auth, components :=
		instr.Token, instr.Commands.Handles, instr.Logger, instr.Commands.Aliases, instr.Commands.Auth, instr.Commands.Components
	dgSession, err := discordgo.New("Bot " + token)
	if err != nil {
		return
	}

	//setup bcp/managers
	instr.Commands.Polls.SetSession(dgSession)

	logger.Add("Begin bot init", audit.LogGroup.BOT)
	// set perms "intents" & register gateway handler boilerplate
	dgSession.Identify.Intents = discordgo.IntentsGuilds
	dgSession.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			// ack(s, i, logger, discordgo.InteractionResponseDeferredChannelMessageWithSource)
			//
			commandData := i.ApplicationCommandData()
			var statusChar string
			if handle, ok := (*handles)[commandData.Name]; ok {
				id := i.Member.User.ID

				alias := aliases.GetAlias(id)
				cmd := rebuildCommand(&commandData)
				var callback types.EventCallback
				if !auth.CanUse(id, handle.Metadata.MinTrustLevel) {
					statusChar = "❌"
					callback = authorize.PermissionsErrorIntercept // TODO
				} else {
					statusChar = "✅"
					callback = handle.Callback
				}
				logger.Add(fmt.Sprintf("%v %33s %02d %s| %s", id, alias, auth.GetRank(id), statusChar, cmd), audit.LogGroup.BOT, audit.LogGroup.INTERACT)

				if err := callback(s, i); err != nil {
					logger.Warn(fmt.Sprintf("While handling %s: %v", rebuildCommand(&commandData), err), audit.LogGroup.BOT, audit.LogGroup.INTERACT)
				}
			}
		case discordgo.InteractionMessageComponent:
			ack(s, i, logger, discordgo.InteractionResponseDeferredMessageUpdate)
			//
			id := i.Member.User.ID
			group, ok := components.GroupOf(id)
			if !ok {
				logger.Warn("no group assigned to this interaction.", audit.LogGroup.BOT, audit.LogGroup.INTERACT)
				return
			}

			alias := aliases.GetAlias(id)

			componentData := i.MessageComponentData()
			var statusChar string
			metadata, err := components.GetMetadata(group)
			if err != nil {
				logger.Warn(err, audit.LogGroup.BOT, audit.LogGroup.INTERACT)
				return
			}
			canUse := auth.CanUse(id, (*handles)[metadata.FromHandle].Metadata.MinTrustLevel)
			if !canUse {
				statusChar = "❌"
			} else {
				statusChar = "✅"
			}
			logger.Add(fmt.Sprintf("%v %33s %02d %s| -> %s", id, alias, auth.GetRank(id), statusChar, componentData.CustomID), audit.LogGroup.BOT, audit.LogGroup.INTERACT)
			if !canUse {
				return
			}
			if busy, err := components.TryRun(i); err != nil {
				logger.Warn(fmt.Sprintf("While handling Component %s: %v", componentData.CustomID, err), audit.LogGroup.BOT, audit.LogGroup.INTERACT)
			} else if busy {
				logger.Warn("Component command ignored (busy)", audit.LogGroup.BOT, audit.LogGroup.INTERACT)
			}
		}
	})

	// take a wild guess
	err = dgSession.Open()
	if err != nil {
		return
	}

	// register slash commands
	var registeredCommands []*discordgo.ApplicationCommand
	{
		payload := make([]*discordgo.ApplicationCommand, len(*handles))
		i := 0
		for _, cmd := range *handles {
			payload[i] = cmd.DGInfo
			i++
		}
		registeredCommands, err = dgSession.ApplicationCommandBulkOverwrite(dgSession.State.User.ID, "", payload)
		if err != nil {
			logger.Panic(fmt.Sprintf("Failed to create commands: %v", err), audit.LogGroup.BOT)
		}
	}

	session = &Session{
		DGSession: dgSession,
		Registers: registeredCommands,
		Logger:    logger,
		Package:   types.BotCommandPackage{Handles: handles},
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
}
