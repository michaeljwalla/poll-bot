package polls

import (
	"encoding/json"
	"fmt"
	"strings"
)

// section related to finalized insertion
var resultsInitQuery = `
PRAGMA journal_mode = 'WAL';
PRAGMA synchronous = 'NORMAL';

CREATE TABLE IF NOT EXISTS results (
    id         BIGINT        NOT NULL,
    channel_id BIGINT        NOT NULL,
    title      VARCHAR (300) NOT NULL,
    options    BLOB          NOT NULL,
    answers    BLOB          NOT NULL,
    PRIMARY KEY (
        id
    )
);
`

type RecordAnswer struct {
	Title  string
	ID     int
	Voters []snowflake
}
type FinalRecord struct {
	Answers  []*RecordAnswer
	Metadata Poll
}

const (
	WRITE_FAILED = iota
	WRITE_SUCCESS
	FETCH_CONTINUE
	FETCH_STOP
)

func (man *PollManager) Insert(mode fetchMode, done chan int, p ...Poll) error {
	valueBuilder := strings.Builder{}
	valueSet := "(?, ?, ?, ?, ?)"
	valueBuilder.Write([]byte(`REPLACE INTO results
		(id, channel_id, title, options, answers)
		VALUES `))
	records := make([]any, 0, len(p)*4)
	s := man.session.Load()
	for i, poll := range p {
		//load message
		poll.LiveMessage(mode, s)
		pd := poll.realMessage.Poll
		//
		options := make([]string, 0, len(pd.Answers))
		answers := make([]RecordAnswer, 0, len(pd.Answers))

		for _, ans := range pd.Answers {
			options = append(options, ans.Media.Text)

			pdVoters, err := s.PollAnswerVoters(poll.realMessage.ChannelID, poll.realMessage.ID, ans.AnswerID)
			if err != nil {
				return fmt.Errorf("while fetching voters: %v", err)
			}
			voters := make([]snowflake, 0, len(pdVoters))
			for _, voter := range pdVoters {
				voters = append(voters, voter.ID)
			}
			answers = append(answers, RecordAnswer{
				Title:  ans.Media.Text,
				ID:     ans.AnswerID,
				Voters: voters,
			})
		}

		jsonOptions, err := json.Marshal(options)
		jsonAnswers, err := json.Marshal(answers)
		if err != nil {
			return fmt.Errorf("insertion marshaling %v: %v", options, err)
		}

		// id, channel_id, title,optionblob, answerblob
		records = append(records,
			&poll.realMessage.ID,
			&poll.realMessage.ChannelID,
			&pd.Question.Text,
			&jsonOptions,
			&jsonAnswers,
		)
		valueBuilder.Write([]byte(valueSet))
		if i == len(p)-1 {
			valueBuilder.Write([]byte(";"))
		} else {
			valueBuilder.Write([]byte(", "))
		}
		done <- i
		if next := <-done; next == FETCH_STOP {
			break
		} else if next == FETCH_CONTINUE {
			continue
		}
	}
	_, err := man.finalized.Exec(valueBuilder.String())
	if err != nil {
		done <- WRITE_FAILED
	} else {
		done <- WRITE_SUCCESS
	}
	return err
}
