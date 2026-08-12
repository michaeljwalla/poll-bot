package loadchannel

import (
	"fmt"
	"poll-bot/root/managers/components"
	"poll-bot/root/managers/polls"
	"poll-bot/root/types"
	"sync/atomic"

	"github.com/bwmarrin/discordgo"
)

const (
	CONTINUE = "load-continue"
	STOP     = "load-stop"
	CLOSE    = "load-close"
)

func list[T any](values ...T) []T {
	return values
}

type snowflake = string
type intxn = discordgo.InteractionCreate
type session = discordgo.Session

type registerCallback = func(*intxn) (bool, error)
type stateCallback = func(*session, *iState) (bool, error)

type query struct {
	polls    []*discordgo.Message //use the message bc it holds more metadata
	searched int
	oldest   *discordgo.Message
	ignored  int //dupes
}
type iState struct {
	stopped      atomic.Bool
	interaction  *intxn
	query        query
	lastInteract atomic.Uint64
	manager      *polls.PollManager
}

// inject state via wrapper func in order to isolate interactions
func shareState(s *discordgo.Session, state *iState, f stateCallback) registerCallback {
	return func(i *intxn) (bool, error) {
		state.interaction = i
		return f(s, state)
	}
}
func newInjector(s *discordgo.Session, state *iState) func(stateCallback) registerCallback {
	return func(f stateCallback) registerCallback {
		return shareState(s, state, f)
	}
}

func defaultRetry(s *session, i *intxn) (*discordgo.Message, error) {
	str := "I couldn't fetch the messages here, sorry."
	components := buttons("Retry", "Stop & Save", list(CONTINUE, STOP), list(false, false))
	return s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    &str,
		Components: components,
	})
}

func (state *iState) insertPolls(msgs []*discordgo.Message) {
	query := &state.query
	for _, msg := range msgs {
		query.searched++
		query.oldest = msg
		if msg.Poll == nil {
			continue
		} else if state.manager.Has(polls.Poll{
			Message: msg,
		}) {
			query.ignored++
			continue
		}
		query.polls = append(query.polls, msg)
	}
}
func iLoadContinue(s *session, state *iState) (bool, error) {
	i := state.interaction
	stopped := &state.stopped
	query := &state.query

	lastIntxn, thisIntxn := state.lastInteract.Load(), timestamp(i.ID)
	//
	if stopped.Load() {
		return false, canceledResponse(s, i)
	} else if expired(thisIntxn, lastIntxn, INTERACTION_IDLE_TIMEOUT_S) /*|| !lastIntxnUpdate.CompareAndSwap(lastIntxn, thisIntxn)*/ {
		return true, canceledResponse(s, i) //close
	}

	//pass next round to load-continue
	components := buttons("Continue", "Stop", list("load-continue", "load-stop"), list(true, true))
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    genContinueMessage(i),
		Components: components,
	}); err != nil {
		_, errRetry := defaultRetry(s, i)
		return false, fmt.Errorf("edit error: %v\nretry error?: %v", err, errRetry)
	}

	//query "loop"
	var id snowflake
	if query.oldest == nil {
		id = i.ID //default to self
	} else {
		id = query.oldest.ID
	}
	msgs, err := s.ChannelMessages(i.ChannelID, 100, id, "", "")
	if err != nil {
		_, errRetry := defaultRetry(s, i)
		return false, fmt.Errorf("query error: %v\nretry error?: %v", err, errRetry)
	}

	//check stopped/expired after fetch
	if stopped.Load() {
		return false, nil
	} else if expired(thisIntxn, lastIntxn, FETCHING_TIMEOUT_S) {
		return true, nil
	}

	//filters here
	state.insertPolls(msgs)

	//update buttons to allow continuation
	components = buttons("Continue", "Stop & Save", list("load-continue", "load-stop"), list(false, false))
	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    genSuccessQueryMessage(state),
		Components: components,
	})
	return false, err
}
func iLoadStop(s *session, state *iState) (bool, error) {
	i := state.interaction
	stopped := &state.stopped
	if stopped.Load() {
		return true, canceledResponse(s, i)
	}
	return true, nil
}
func iClose(s *session, state *iState) (bool, error) {
	i := state.interaction

	if !state.stopped.CompareAndSwap(false, true) || i == nil { //acts as a debounce too
		return true, nil
	}
	query := &state.query
	//attempt to save data
	if len(query.polls) == 0 {
		return true, canceledResponse(s, i)
	}

	//notify & clear options
	pollsBuffer := make([]polls.Poll, len(query.polls))
	for j, msg := range query.polls {
		pollsBuffer[j] = polls.Poll{
			Expiry:  msg.Poll.Expiry,
			Message: msg,
			Guild:   i.GuildID,
		}
	}

	var msg string
	dropped, err := state.manager.Push(pollsBuffer...)
	if err != nil {
		msg = fmt.Sprintf("Couldn't push to heap: %v", err)
	} else {
		if dropped > 0 {
			msg += fmt.Sprintf("Dropped %d duplicates.\n", dropped) //more like a sanity check
		}
		msg += fmt.Sprintf("Added %d polls to the queue heap for processing", len(query.polls))
	}
	msg += "\n-# This interaction has ended."
	// TODO
	// make a translator func for discordgo.Message bc its unnecessarily large
	// for this use case.
	// write to external data that isn't the queue. may need to do
	// some form of chunking.

	// to remove options
	empty := list[discordgo.MessageComponent]()
	_, errSend := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    &msg,
		Components: &empty,
	})
	//
	if err != nil {
		return true, fmt.Errorf("in close() of loadchannel: %v", err)
	} else if errSend != nil {
		return true, fmt.Errorf("in close() of loadchannel on message: %v", errSend)
	} else {
		return true, nil
	}
}
func createInteractionSession(bcp *types.BotCommandPackage, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	//default inits
	thisState := iState{
		interaction: i,
		manager:     bcp.Polls,
	}
	thisState.lastInteract.Store(timestamp(i.ID))
	inject := newInjector(s, &thisState)
	//busy atomic in manager should help w debounce
	return bcp.Components.Register("load", components.NewComponentCallbacks(i.Member.User.ID, map[string]components.Callback{
		"load-continue": inject(iLoadContinue),
		"load-stop":     inject(iLoadStop),
		"close":         inject(iClose),
	}))
}
