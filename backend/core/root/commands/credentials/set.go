package credentials

import (
	"fmt"
	"poll-bot/root/managers/components"

	"github.com/bwmarrin/discordgo"
)

func iModalSubmit(s session, state *iState) (bool, error) {
	i := state.interaction
	web := state.manager

	msg, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Content: "I'm validating this information...",
	})
	if err != nil {
		return false, err
	}
	state.message = msg

	data := i.ModalSubmitData()
	idField := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	passField := data.Components[1].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value

	err = web.ModifyUser(idField, passField, 0)

	var message string
	if err != nil {
		message = fmt.Sprintf("<@%s>, I couldn't create credentials: %v", i.Member.User.ID, err)

	} else {
		message = fmt.Sprintf("<@%s>, I created credentials for `%s`. You should be able to sign in now.", i.Member.User.ID, idField)
	}
	_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:      msg.ID,
		Channel: msg.ChannelID,
		Content: &message,
	})
	return true, err
}
func iModalClose(s session, state *iState) (bool, error) {
	return true, nil
}

func createCredentialSetSession(bcp bcpackage, man *webman, s session, i intxn) error {
	//default inits
	thisState := iState{
		interaction: i,
		manager:     man,
	}
	inject := newInjector(s, &thisState)
	//busy atomic in manager should help w debounce
	return bcp.Components.Register("cred-set", components.NewComponentCallbacks(i.Member.User.ID, map[string]components.Callback{
		"cred-set-modal-submit": inject(iModalSubmit),
		"cred-set-close":        inject(iModalClose),
	}))
}
func cmd_set_credentials(s session, i intxn, bcp bcpackage, web *webman) error {
	if err := createCredentialSetSession(bcp, web, s, i); err != nil {
		return err
	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "cred-set-modal-submit",
			Title:    "New Credentials Set",
			Components: makeFields([]field{
				{"Id", "cred-set-id", ""},
				{"Password", "cred-set-pass", ""},
			}),
		}})
	return err
}
