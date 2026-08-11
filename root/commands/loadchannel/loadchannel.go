package loadchannel

import (
	"fmt"
	"poll-bot/root/info/unix"
	"poll-bot/root/managers/authorize"
	"poll-bot/root/managers/components"
	"poll-bot/root/managers/polls"
	"poll-bot/root/types"
	"sync/atomic"

	"github.com/bwmarrin/discordgo"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.MANAGE,
}

const FETCHING_TIMEOUT_S = 30
const INTERACTION_IDLE_TIMEOUT_S = 90

func buttons(loadBegin string, stop string, id [2]string, disabled [2]bool) *[]discordgo.MessageComponent {
	return &[]discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    loadBegin,
					Style:    discordgo.PrimaryButton,
					CustomID: id[0],
					Disabled: disabled[0],
				},
				discordgo.Button{
					Label:    stop,
					Style:    discordgo.SecondaryButton,
					CustomID: id[1],
					Disabled: disabled[1],
				},
			},
		},
	}
}

func timestamp(i string) uint64 {
	return uint64(unix.IdToEpochMillis(i) / 1000)
}
func expired(a uint64, b uint64, expiryDiff int) bool {
	return a+uint64(expiryDiff) < b
}
func canceledResponse(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	str := "-# This interaction has expired or been canceled."
	emptyComponents := make([]discordgo.MessageComponent, 0)
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    &str,
		Components: &emptyComponents,
	})
	return err
}
func genInteractionExpireMessage(i *discordgo.InteractionCreate) (timeout string, expiry string) {
	timestamp := timestamp(i.ID)
	expiry = fmt.Sprintf(`interaction expires in <t:%d:R>`, timestamp+INTERACTION_IDLE_TIMEOUT_S)
	timeout = fmt.Sprintf(`timeout <t:%d:R>`, timestamp+FETCHING_TIMEOUT_S)
	return
}
func genContinueMessage(i *discordgo.InteractionCreate) (message *string) {
	timeout, expiry := genInteractionExpireMessage(i)
	str := fmt.Sprintf(`### fetching polls... %s
-# -> %s
-# -> if you're on mobile, the timestamps update on-relaunch`,
		timeout, expiry)
	return &str
}
func genStartupMessage(i *discordgo.InteractionCreate) (message *string) {
	_, expiry := genInteractionExpireMessage(i)

	str := fmt.Sprintf(`### To begin fetching polls, please press %s.
The oldest fetched poll will be linked for reference how far back has been indexed.
-# -> %s`, "`Start`", expiry)
	return &str
}
func genSuccessQueryMessage(i *discordgo.InteractionCreate, num_polls int, num_messages int, msg *discordgo.Message) (message *string) {
	_, expiry := genInteractionExpireMessage(i)
	msg_link := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, msg.ChannelID, msg.ID)

	str := fmt.Sprintf(`### Fetched %d messages and found %d polls.
Oldest message %s from <t:%d:f>
-# -> %s`, num_messages, num_polls,
		msg_link, timestamp(msg.ID),
		expiry)
	return &str
}

// map is already reference-like but just for continuity
func Register(bcp *types.BotCommandPackage) {

	bcp.Components.AddGroup("load", &components.GroupMetadata{
		NewInvalidatesOld:  true,
		InvalidationCloses: false,
		FromHandle:         "load",
	})

	//states are messed up a bit, this will help with wrong-intxn stuff
	lastIntxnUpdate := atomic.Uint64{}
	(*bcp.Handles)["load"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
			Name:        "load",
			Description: "Discover rating polls in this channel. Works in increments.",
		},
		Metadata: metadata,
		Callback: func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			// channel := i.ChannelID
			stopped := atomic.Bool{}

			lastIntxnUpdate.Store(timestamp(i.ID))

			uid := i.Member.User.ID

			var pollsFound []*discordgo.Message //use the message bc it holds more metadata
			var messages_searched int
			var oldest_message *discordgo.Message
			//busy atomic in manager should help w debounce
			err := bcp.Components.Register("load", components.NewComponentCallbacks(uid, map[string]components.Callback{
				"load-start": func(i *discordgo.InteractionCreate) (bool, error) {
					lastIntxn, thisIntxn := lastIntxnUpdate.Load(), timestamp(i.ID)
					if stopped.Load() {
						return false, canceledResponse(s, i)
					} else if expired(thisIntxn, lastIntxn, INTERACTION_IDLE_TIMEOUT_S) || !lastIntxnUpdate.CompareAndSwap(lastIntxn, thisIntxn) {
						return true, canceledResponse(s, i) //close
					}

					//pass next round to load-continue
					components := buttons("Continue", "Stop & Save", [2]string{"load-continue", "load-stop"}, [2]bool{true, true})
					_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
						Content:    genContinueMessage(i),
						Components: components,
					})

					//query "loop"
					var msgs []*discordgo.Message
					if err == nil {
						msgs, err = s.ChannelMessages(i.ChannelID, 100, i.ID, "", "")
					}

					if err != nil {
						str := "I couldn't fetch the messages here, sorry."
						components := buttons("Retry", "Stop & Save", [2]string{"load-continue", "load-stop"}, [2]bool{false, false})
						_, errMsg := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
							Content:    &str,
							Components: components,
						})
						return false, fmt.Errorf("query error: %v\nmessage error(?): %v", err, errMsg)
					}
					//check stopped/expired after fetch
					if stopped.Load() {
						return false, nil // already called prior, should be impossible here actually
					} else if expired(thisIntxn, lastIntxn, FETCHING_TIMEOUT_S) {
						return true, nil //close
					}
					//filter
					for _, msg := range msgs {
						messages_searched++
						oldest_message = msg
						if msg.Poll == nil {
							continue
						}
						pollsFound = append(pollsFound, msg)
					}

					//update buttons to allow continuation
					components = buttons("Continue", "Stop & Save", [2]string{"load-continue", "load-stop"}, [2]bool{false, false})
					_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
						Content:    genSuccessQueryMessage(i, len(pollsFound), messages_searched, oldest_message),
						Components: components,
					})
					return false, err
				},
				"load-continue": func(i *discordgo.InteractionCreate) (bool, error) {
					lastIntxn, thisIntxn := lastIntxnUpdate.Load(), timestamp(i.ID)
					if stopped.Load() {
						return false, canceledResponse(s, i)
					} else if expired(thisIntxn, lastIntxn, INTERACTION_IDLE_TIMEOUT_S) {
						return true, canceledResponse(s, i) //close
					}
					components := buttons("Continue", "Stop", [2]string{"load-continue", "load-stop"}, [2]bool{true, true})
					_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
						Content:    genContinueMessage(i),
						Components: components,
					})

					//query "loop"
					var msgs []*discordgo.Message
					if err == nil {
						msgs, err = s.ChannelMessages(i.ChannelID, 100, i.ID, "", "")
					}

					if err != nil {
						str := "I couldn't fetch the messages here, sorry."
						components := buttons("Retry", "Stop & Save", [2]string{"load-continue", "load-stop"}, [2]bool{false, false})
						_, errMsg := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
							Content:    &str,
							Components: components,
						})
						return false, fmt.Errorf("query error: %v\nmessage error(?): %v", err, errMsg)
					}
					//check stopped/expired after fetch
					if stopped.Load() {
						return false, nil // already called prior, should be impossible here actually
					} else if expired(thisIntxn, lastIntxn, FETCHING_TIMEOUT_S) {
						return true, nil //close
					}
					//filter
					for _, msg := range msgs {
						messages_searched++
						oldest_message = msg
						if msg.Poll == nil {
							continue
						}
						pollsFound = append(pollsFound, msg)
					}

					components = buttons("Continue", "Stop & Save", [2]string{"load-continue", "load-stop"}, [2]bool{false, false})
					_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
						Content:    genSuccessQueryMessage(i, len(pollsFound), messages_searched, oldest_message),
						Components: components,
					})

					return false, err
				},
				"load-stop": func(i *discordgo.InteractionCreate) (bool, error) {
					lastIntxn, thisIntxn := lastIntxnUpdate.Load(), timestamp(i.ID)
					if stopped.Load() {
						return true, canceledResponse(s, i)
					} else if !lastIntxnUpdate.CompareAndSwap(lastIntxn, thisIntxn) {
						return true, nil //close
					}

					return true, nil
				},
				//close only gets nil, pass through local state to access
				"close": func(i *discordgo.InteractionCreate) (bool, error) {
					if !stopped.CompareAndSwap(false, true) || i == nil { //acts as a debounce too
						return true, nil
					}
					//attempt to save data
					if len(pollsFound) == 0 {
						return true, canceledResponse(s, i)
					}

					//notify & clear options
					pushErrs := make([]error, 0, len(pollsFound))
					for _, msg := range pollsFound {
						err := bcp.Polls.Push(polls.Poll{
							Message: msg,
							Expiry:  msg.Poll.Expiry,
							Guild:   i.GuildID,
						})
						if err != nil {
							pushErrs = append(pushErrs, err)
						}
					}
					errWrite := bcp.Polls.Write()

					var msg string
					if errWrite == nil {
						if len(pushErrs) == 0 {
							msg = fmt.Sprintf("Added %d polls to the queue heap for processing... no errors!", len(pollsFound)-len(pushErrs))
						} else {
							msg = fmt.Sprintf("Added %d polls to the queue heap for processing... %d others couldn't be pushed.", len(pollsFound)-len(pushErrs), len(pushErrs))
						}
					} else {
						msg = fmt.Sprintf("Couldn't sync file automatically, this session may not save.\n%d polls added to the queue heap... %d others couldn't be pushed.", len(pollsFound)-len(pushErrs), len(pushErrs))
					}
					emptyComponents := make([]discordgo.MessageComponent, 0)
					// TODO
					// make a translator func for discordgo.Message bc its unnecessarily large
					// for this use case.
					// filter dupes (may need a filedict alongside the heap!)
					// write to external data that isn't the queue. may need to do
					// some form of chunking.
					// also need to store each message ID too... maybe JUST store the message
					// id instead of poll id.

					_, errSend := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
						Content:    &msg,
						Components: &emptyComponents,
					})
					//
					if len(pushErrs) > 0 || errSend != nil || errWrite != nil {
						return true, fmt.Errorf("in close() of loadchannel:\n\terr send?: %v\n\terr write?: %v\n\terrs mempushed?: %d", errWrite, errSend, len(pushErrs))
					} else {
						return true, nil
					}
				},
			}))
			if err != nil {
				return err
			}
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    *genStartupMessage(i),
					Components: *buttons("Start", "Stop", [2]string{"load-start", "load-stop"}, [2]bool{false, false}),
				},
			})
		},
	}

}
