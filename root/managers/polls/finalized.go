package polls

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
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
	Options  []string
	Metadata Poll
	Title    string
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

// Returns finalized records from the results DB. With no ids, returns
// every row; otherwise filters by id.
func (man *PollManager) GetFinalized(ids ...snowflake) ([]*FinalRecord, error) {
	query := `SELECT id, channel_id, title, options, answers FROM results`
	args := make([]any, 0, len(ids))
	if len(ids) > 0 {
		placeholders := strings.Repeat("?,", len(ids))
		query += ` WHERE id IN (` + placeholders[:len(placeholders)-1] + `)`
		for _, id := range ids {
			args = append(args, id)
		}
	}
	query += `;`

	iter, err := man.finalized.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer iter.Close() //nolint

	var out []*FinalRecord
	for {
		var (
			id, channelID            snowflake
			title                    string
			optionsBlob, answersBlob []byte
		)
		ok, err := iter.NextScan(&id, &channelID, &title, &optionsBlob, &answersBlob)
		if err != nil {
			return out, err
		}
		if !ok {
			break
		}
		var options []string
		if err := json.Unmarshal(optionsBlob, &options); err != nil {
			return out, fmt.Errorf("decoding options for %s: %v", id, err)
		}
		var answers []RecordAnswer
		if err := json.Unmarshal(answersBlob, &answers); err != nil {
			return out, fmt.Errorf("decoding answers for %s: %v", id, err)
		}
		ptrs := make([]*RecordAnswer, len(answers))
		for i := range answers {
			ptrs[i] = &answers[i]
		}
		out = append(out, &FinalRecord{
			Answers:  ptrs,
			Options:  options,
			Metadata: Poll{Message: message{ID: id, ChannelID: channelID}},
			Title:    title,
		})
	}
	return out, nil
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
		//Discord finalizes results slightly after expiry; if not yet
		//finalized, requeue with a small delay and skip this record.
		if pd.Results == nil || !pd.Results.Finalized {
			later := time.Now().Add(30 * time.Second)
			poll.Expiry = &later
			poll.realMessage = nil //force refetch on next attempt
			man.Push(poll)         //nolint
			err := errors.New("poll not finalized by Discord yet")
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
