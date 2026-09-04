package polls

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var ErrMessageNotFound = errors.New("message missing")

// MessageNotFound carries the id alongside the sentinel. A caller dropping a
// poll has to name which one, and by the time it decides the message is gone
// there is nothing left to look the id up on: the error is all it holds.
type MessageNotFound struct {
	ID    snowflake
	cause error
}

func (e *MessageNotFound) Error() string {
	return fmt.Sprintf("message %s not found: %v", e.ID, e.cause)
}

// matches ErrMessageNotFound while Unwrap still exposes the Discord error, so
// a caller can test for the condition and report the cause separately.
func (e *MessageNotFound) Is(target error) bool { return target == ErrMessageNotFound }
func (e *MessageNotFound) Unwrap() error        { return e.cause }

// notFound tags a Discord error that means the message is gone. Every fetch
// against a deleted message 404s the same way, so the recognition lives here
// rather than being restated at each call site.
func notFound(id snowflake, err error) error {
	if err != nil && strings.HasPrefix(err.Error(), "HTTP 404") {
		return &MessageNotFound{ID: id, cause: err}
	}
	return err
}

func (man *PollManager) GetData(data *Poll) (*discordgo.Message, error) {
	if s := man.session.Load(); s != nil {
		return data.LiveMessage(CACHEDFETCH, s)
	}
	return nil, errors.New("no session assigned to PollManager")
}
