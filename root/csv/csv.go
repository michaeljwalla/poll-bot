package csv

import (
	"fmt"
	"os"
	"poll-bot/root/info/unix"
	"poll-bot/root/managers/aliases"
	"poll-bot/root/managers/polls"
	"slices"
	"strings"
	"time"
)

type snowflake = string

var answerMap = map[string]string{
	"⭐⭐⭐⭐⭐": "5",
	"⭐⭐⭐⭐":  "4",
	"⭐⭐⭐":   "3",
	"⭐⭐":    "2",
	"⭐":     "1",
	"N/A":   "N/A",
}

type pollPair struct {
	time  int64
	title string
	votes *int
}

func ToCSV(path string, recs []*polls.FinalRecord, aliases *aliases.AliasManager) (*string, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint
	// const HEADER = "Date, Topic, Total, Users...,  <END>"

	voterNames := make(map[snowflake]string)
	pollOrder := make([]pollPair, 0)
	voterOrder := make([]snowflake, 0)
	mappedRecData := make(map[string]map[snowflake]string) // map[title]map[user]choice

	//prelim validating / building aliases
	for _, rec := range recs {
		thisPoll := make(map[snowflake]string)
		mappedRecData[rec.Title] = thisPoll
		time, err := unix.SnowflakeToTime(rec.Metadata.Message.ID)
		if err != nil {
			continue
		}
		pollVotes := 0
		pollOrder = append(pollOrder, pollPair{
			time: time.Unix(), title: rec.Title, votes: &pollVotes,
		})

		for _, choice := range rec.Answers {
			for _, id := range choice.Voters {
				// ignore bad titles
				choice, ok := answerMap[choice.Title]
				if !ok {
					break
				}

				// add user if unique
				if _, ok := voterNames[id]; !ok {
					if alias := aliases.GetAlias(id); alias != "?" {
						voterNames[id] = alias
					} else {
						voterNames[id] = id
					}
					//insert for ordering
					voterOrder = append(voterOrder, id)
				}

				//insert for building
				thisPoll[id] = choice
				if choice != answerMap["N/A"] {
					pollVotes++
				}
			}
		}
	}

	// lexicographically sort for column consistency
	slices.Sort(voterOrder)
	slices.SortFunc(pollOrder, func(a pollPair, b pollPair) int {
		return int(a.time) - int(b.time)
	})

	//build now
	out := strings.Builder{}
	out.WriteString("Date, Topic, Total, ")
	for _, id := range voterOrder {
		fmt.Fprintf(&out, "%s, ", voterNames[id])
	}
	out.WriteString("<END>")
	//
	for _, pair := range pollOrder {
		time, title, votes := time.Unix(pair.time, 0).Format("01/02/2006"), pair.title, *pair.votes
		fmt.Fprintf(&out, "\n%s, %s, %d, ", time, title, votes)

		thisPoll := mappedRecData[title]
		for _, id := range voterOrder {
			msg, ok := thisPoll[id]
			if !ok {
				msg = "N/A"
			}
			fmt.Fprintf(&out, "%s, ", msg)
		}
		out.WriteString("<END>")
	}
	final := out.String()
	file.Write([]byte(final)) //nolint
	return &final, nil
}
