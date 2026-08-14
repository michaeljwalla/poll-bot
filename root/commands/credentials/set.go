package credentials

import (
	"fmt"
	"poll-bot/root/managers/components"
	"time"

	"github.com/bwmarrin/discordgo"
)

func iModalSubmit(s session, state *iState) (bool, error) {
	i := state.interaction

	msg, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Content: "I'm validating this information...",
	})
	if err != nil {
		return false, err
	}
	state.message = msg

	// VA:ODATE OIT HERE
	return true, err
}
func iModalClose(s session, state *iState) (bool, error) {
	i := state.interaction
	msg := state.message

	time.Sleep(time.Duration(3) * time.Second)
	if msg == nil {
		_, err := s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
			Content: "Something has gone terribly wrong (jk)",
		})
		return true, err
	}

	message := fmt.Sprintf("<@%s>, I created credentials for `%s`. You should be able to sign in now.", i.Member.User.ID, "John Doe")
	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:      msg.ID,
		Channel: msg.ChannelID,
		Content: &message,
	})
	return true, err
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
