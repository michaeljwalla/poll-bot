package credentials

import (
	"poll-bot/root/managers/authorize"
	"poll-bot/root/managers/components"
	"poll-bot/root/managers/web"
	"poll-bot/root/types"

	"github.com/bwmarrin/discordgo"
)

type session = *discordgo.Session
type intxn = *discordgo.InteractionCreate
type bcpackage = *types.BotCommandPackage
type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback
type choice = discordgo.ApplicationCommandOptionChoice

type webman = web.WebManager //spiderman

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.MANAGER,
}

type callback = func(session, intxn, bcpackage, *webman) error
type subCommandMap = map[string]callback

var subcommands = subCommandMap{
	"set":  cmd_set_credentials,
	"drop": cmd_drop_credentials,
}

type field struct {
	Label, CustomID, Placeholder string
}
type iState struct {
	manager *web.WebManager
	// modal has no message. use state.message
	interaction intxn

	// nil means create message, otherwise edit
	message *discordgo.Message
}

type registerCallback = func(intxn) (bool, error)
type stateCallback = func(session, *iState) (bool, error)

// inject state via wrapper func in order to isolate interactions
func shareState(s session, state *iState, f stateCallback) registerCallback {
	return func(i intxn) (bool, error) {
		state.interaction = i
		return f(s, state)
	}
}
func newInjector(s session, state *iState) func(stateCallback) registerCallback {
	return func(f stateCallback) registerCallback {
		return shareState(s, state, f)
	}
}

func makeFields(fields []field) []discordgo.MessageComponent {
	if fields == nil {
		return nil
	}
	out := make([]discordgo.MessageComponent, len(fields))
	for i, f := range fields {
		out[i] = discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.TextInput{
					CustomID:    f.CustomID,
					Label:       f.Label,
					Style:       discordgo.TextInputShort,
					Placeholder: f.Placeholder,
					Required:    true,
					MaxLength:   50,
				},
			},
		}
	}
	return out
}
func getSubcommands(subcommands subCommandMap) []*choice {
	choices := make([]*choice, len(subcommands))

	i := 0
	for name := range subcommands {
		choices[i] = &choice{
			Name:  name,
			Value: name,
		}
		i++
	}
	return choices
}

// map is already reference-like but just for continuity
func Register(bcp *types.BotCommandPackage, web *webman) {
	bcp.Components.AddGroup("cred-set", &components.GroupMetadata{
		NewInvalidatesOld:  true,
		InvalidationCloses: false,
		FromHandle:         "credentials",
	})

	(*bcp.Handles)["credentials"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
			Name:        "credentials",
			Description: "Actions related to web credentials. You probably can't use this.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "Choose one",
					Required:    true,
					Choices:     getSubcommands(subcommands),
				},
			},
		},
		Metadata: metadata,
		Callback: func(s session, i intxn) error {
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			name := optionMap["action"]
			callback, ok := subcommands[name.StringValue()]

			if !ok {
				return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "I don't know that action.",
					},
				})
			}
			return callback(s, i, bcp, web)
		},
	}

}
