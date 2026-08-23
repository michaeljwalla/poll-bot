package polls

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// section related to finalized insertion
var resultsInitQuery = `
PRAGMA journal_mode = 'WAL';
PRAGMA synchronous = 'NORMAL';
CREATE TABLE IF NOT EXISTS results (
    id             BIGINT        NOT NULL,
    channel_id     BIGINT        NOT NULL,
    title          VARCHAR (300) NOT NULL,
    title_override VARCHAR (300),
    active         INTEGER       NOT NULL DEFAULT 1,
    options        BLOB          NOT NULL,
    answers        BLOB          NOT NULL,
    PRIMARY KEY (
        id
    )
);

CREATE TABLE IF NOT EXISTS manual (
    id             BIGINT        NOT NULL,
    title          VARCHAR (300) NOT NULL,
    title_override VARCHAR (300),
    active         INTEGER       NOT NULL DEFAULT 1,
    options        BLOB          NOT NULL,
    answers        BLOB          NOT NULL,
    PRIMARY KEY (
        id
    )
);
`

// results and manual hold the same rows apart from channel_id, which only
// Discord-sourced polls carry. Nothing reads it back off a record, so manual
// rows select a constant rather than the table carrying a dead column.
// Manual rows are hand-entered, so they shadow a results row of the same id.
const finalizedSources = `SELECT id, '' AS channel_id, COALESCE(title_override, title) AS title, active, options, answers FROM manual
	UNION ALL
	SELECT id, channel_id, COALESCE(title_override, title) AS title, active, options, answers FROM results
	WHERE id NOT IN (SELECT id FROM manual)`

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
	Active   bool
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

// Returns finalized records from both the results and manual tables. With no
// ids, returns every row; otherwise filters by id. An id held by both tables
// resolves to the manual row. omitInactive drops rows flagged inactive.
func (man *PollManager) GetFinalized(offset int, limit int, omitInactive bool, ids ...snowflake) ([]*FinalRecord, error) {
	query := `SELECT id, channel_id, title, active, options, answers FROM (` + finalizedSources + `)`
	args := make([]any, 0, len(ids))
	//both predicates apply to the union, so they have to share the one WHERE.
	where := make([]string, 0, 2)
	if omitInactive {
		where = append(where, `active != 0`)
	}
	if len(ids) > 0 {
		placeholders := strings.Repeat("?,", len(ids))
		where = append(where, `id IN (`+placeholders[:len(placeholders)-1]+`)`)
		for _, id := range ids {
			args = append(args, id)
		}
	}
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	//the filter has to land on the union before it can be ordered or paged.
	query += ` ORDER BY id`
	if limit > 0 {
		query += " LIMIT " + strconv.Itoa(limit)
	} else if offset > 0 {
		query += " LIMIT -1" //SQLite rejects a bare OFFSET; -1 means unbounded
	}
	if offset > 0 {
		query += " OFFSET " + strconv.Itoa(offset)
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
			active                   int
			optionsBlob, answersBlob []byte
		)
		ok, err := iter.NextScan(&id, &channelID, &title, &active, &optionsBlob, &answersBlob)
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
			Metadata: Poll{Message: Message{ID: id, ChannelID: channelID}},
			Title:    title,
			Active:   active != 0,
		})
	}
	return out, nil
}

// error channel non-blocking
// int channel blocking
func (man *PollManager) Insert(mode fetchMode, done chan int, cher chan<- error, p ...Poll) error {
	valueBuilder := strings.Builder{}
	valueSet := "(?, ?, ?, ?, ?, ?, ?)"
	valueBuilder.Write([]byte(`REPLACE INTO results
		(id, channel_id, title, title_override, active, options, answers)
		VALUES `))
	records := make([]any, 0, len(p)*7)
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
		//finalized, requeue and skip this record.
		if pd.Results == nil || !pd.Results.Finalized {
			//the live message is authoritative: whatever we had stored may be
			//stale, clobbered by an earlier retry, or absent entirely.
			if pd.Expiry != nil {
				poll.Expiry = pd.Expiry
			}
			//only ever push the retry forward. Overwriting a future expiry
			//with now+RETRY_DELAY strands the poll in a RETRY_DELAY loop for
			//as long as it has left to run, and loses its real deadline.
			retry := time.Now().Add(RETRY_DELAY)
			if poll.Expiry == nil || poll.Expiry.Before(retry) {
				poll.Expiry = &retry
			}
			poll.realMessage = nil //force refetch on next attempt
			man.Push(poll)         //nolint
			err := fmt.Errorf("poll %s not finalized by Discord yet, retrying at %s",
				poll.Message.ID, poll.Expiry.Format(time.RFC3339))
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

		// id, channel_id, title, title_override, active, optionblob, answerblob
		//Discord owns the title, so there is no override until someone sets one.
		records = append(records,
			&poll.realMessage.ID,
			&poll.realMessage.ChannelID,
			&pd.Question.Text,
			nil,
			1,
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

// Writes records straight into the manual table. Unlike Insert there is no
// fetch mode and no progress/error channels: nothing here talks to Discord,
// so there is no rate limit to pace against and a failure is just an error.
// Rows are keyed by id, so re-posting the same id overwrites it.
func (man *PollManager) InsertManual(records ...*FinalRecord) error {
	if len(records) == 0 {
		return nil
	}
	valueBuilder := strings.Builder{}
	valueSet := "(?, ?, ?, ?, ?, ?)"
	valueBuilder.WriteString(`REPLACE INTO manual
		(id, title, title_override, active, options, answers)
		VALUES `)
	args := make([]any, 0, len(records)*6)
	rowCount := 0 //rows actually appended (nil records are skipped)

	for _, rec := range records {
		if rec == nil {
			continue
		}
		id := rec.Metadata.Message.ID
		if id == "" {
			return fmt.Errorf("manual insertion: record %q has no id", rec.Title)
		}
		options := rec.Options
		if options == nil {
			options = []string{}
		}
		answers := rec.Answers
		if answers == nil {
			answers = []*RecordAnswer{}
		}

		jsonOptions, err := json.Marshal(options)
		if err != nil {
			return fmt.Errorf("manual insertion marshaling %v: %v", options, err)
		}
		jsonAnswers, err := json.Marshal(answers)
		if err != nil {
			return fmt.Errorf("manual insertion marshaling %v: %v", answers, err)
		}

		if rowCount > 0 {
			valueBuilder.WriteString(", ")
		}
		valueBuilder.WriteString(valueSet)
		// id, title, title_override, active, optionblob, answerblob
		//a hand-entered title is itself the override; title keeps a copy so
		//clearing the override later still leaves the row with a name.
		args = append(args, id, rec.Title, rec.Title, 1, jsonOptions, jsonAnswers)
		rowCount++
	}
	if rowCount == 0 {
		return nil
	}
	valueBuilder.WriteString(";")
	_, err := man.finalized.Exec(valueBuilder.String(), args...)
	return err
}
func (man *PollManager) Drop(ids ...snowflake) error {
	const query = `DELETE FROM results WHERE id in (%s); DELETE FROM manual WHERE id in (%s);`
	builder := strings.Builder{}
	for i, id := range ids {
		fmt.Fprintf(&builder, "'%s'", id)
		if i != len(ids)-1 {
			builder.WriteString(", ")
		}
	}
	out := builder.String()
	_, err := man.finalized.Exec(fmt.Sprintf(query, out, out))
	return err
}

// Flips the active flag on existing rows. An id may live in either table, so
// both are updated; ids matching nothing are silently no-ops.
func (man *PollManager) SetActive(active bool, ids ...snowflake) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	filter := ` WHERE id IN (` + placeholders[:len(placeholders)-1] + `);`

	flag := 0
	if active {
		flag = 1
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, flag)
	for _, id := range ids {
		args = append(args, id)
	}

	for _, table := range [...]string{"manual", "results"} {
		if _, err := man.finalized.Exec(`UPDATE `+table+` SET active = ?`+filter, args...); err != nil {
			return fmt.Errorf("setting active on %s: %v", table, err)
		}
	}
	return nil
}
