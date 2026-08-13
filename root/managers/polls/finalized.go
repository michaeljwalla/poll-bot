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
	INS_NEVER = iota //negatives are indices
	INS_WRITE_FAILED
	INS_WRITE_SUCCESS
	INS_FETCH_CONTINUE
	INS_FETCH_STOP
	INS_ERR_NEW
	INS_ERR_RECOVER_BREAK_IGNORE
	INS_ERR_STOP
	INS_ERR_SHUTDOWN
)

// any of the ERR_* excl. ERR_NEW
func handle_insertion_err(err error, ch chan int, cher chan<- error) (doBreak bool, doReturn bool) {
	cher <- err
	ch <- INS_ERR_NEW
	next := <-ch

	switch next {
	case INS_ERR_STOP:
		return true, false
	case INS_ERR_SHUTDOWN: //instant give up
		return false, true
	default: //ERR IGNORE
		return false, false
	}
}

// error channel non-blocking
// int channel blocking
func (man *PollManager) Insert(mode fetchMode, done chan int, cher chan<- error, p ...Poll) error {
	valueBuilder := strings.Builder{}
	valueSet := "(?, ?, ?, ?, ?)"
	valueBuilder.Write([]byte(`REPLACE INTO results
		(id, channel_id, title, options, answers)
		VALUES `))
	records := make([]any, 0, len(p)*4)
	rowCount := 0 //rows actually appended (i can advance past skipped items)

	channeled := done != nil && cher != nil

	s := man.session.Load()
	for i, poll := range p {
		//load message
		if _, err := poll.LiveMessage(mode, s); err != nil {
			if !channeled {
				return err
			}
			if doBreak, doReturn := handle_insertion_err(err, done, cher); doReturn {
				return err
			} else if doBreak {
				break
			} else {
				continue
			}
		}
		pd := poll.realMessage.Poll
		//
		options := make([]string, 0, len(pd.Answers))
		answers := make([]RecordAnswer, 0, len(pd.Answers))

		for _, ans := range pd.Answers {
			options = append(options, ans.Media.Text)

			pdVoters, err := s.PollAnswerVoters(poll.realMessage.ChannelID, poll.realMessage.ID, ans.AnswerID)
			if err != nil {
				err := fmt.Errorf("while fetching voters: %v", err)
				if !channeled {
					return err
				} else {
					if doBreak, doReturn := handle_insertion_err(err, done, cher); doReturn {
						return err
					} else if doBreak {
						break
					} else {
						continue
					}
				}
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
		if err != nil {
			err := fmt.Errorf("insertion marshaling %v: %v", options, err)
			if doBreak, doReturn := handle_insertion_err(err, done, cher); doReturn {
				return err
			} else if doBreak {
				break
			} else {
				continue //skip
			}
		}
		jsonAnswers, err := json.Marshal(answers)
		if err != nil {
			err := fmt.Errorf("insertion marshaling %v: %v", answers, err)
			if doBreak, doReturn := handle_insertion_err(err, done, cher); doReturn {
				return err
			} else if doBreak {
				break
			} else {
				continue //skip
			}
		}

		// id, channel_id, title,optionblob, answerblob
		records = append(records,
			&poll.realMessage.ID,
			&poll.realMessage.ChannelID,
			&pd.Question.Text,
			&jsonOptions,
			&jsonAnswers,
		)
		if rowCount > 0 {
			valueBuilder.Write([]byte(", "))
		}
		valueBuilder.Write([]byte(valueSet))
		rowCount++
		if channeled {
			done <- -i //send negative signifies index
			if next := <-done; next == INS_FETCH_STOP {
				break
			} else if next == INS_FETCH_CONTINUE {
				continue
			}
		}

	}
	if rowCount == 0 {
		if channeled {
			done <- INS_WRITE_SUCCESS
		}
		return nil
	}
	valueBuilder.Write([]byte(";"))
	_, err := man.finalized.Exec(valueBuilder.String(), records...)
	if channeled {
		if err != nil {
			cher <- err
			done <- INS_WRITE_FAILED
		} else {
			done <- INS_WRITE_SUCCESS
		}
	}
	return err
}
