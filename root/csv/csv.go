package csv

import (
	"fmt"
	"os"
	"poll-bot/root/info/unix"
	"poll-bot/root/managers/aliases"
	"poll-bot/root/managers/polls"
	"regexp"
	"slices"
	"strings"
	"time"
)

type snowflake = string

var answerMap = map[string]string{
	"⭐⭐⭐⭐⭐":   "5",
	"⭐⭐⭐⭐":    "4",
	"⭐⭐⭐":     "3",
	"⭐⭐":      "2",
	"⭐":       "1",
	"N/A":     "N/A",
	"NO DATA": "N/A",
}

type pollPair struct {
	time  int64
	title string
	votes *int
}

func ToCSV(path string, recs []*polls.FinalRecord, aliases *aliases.AliasManager) (*strings.Builder, error) {
	var file *os.File
	var err error
	if path != "" {
		file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return nil, err
		}
		defer file.Close() //nolint
	}
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

		match := regexp.MustCompile(`^(⭐+|N/A)(?: \(.*\))?$`)
		for _, choice := range rec.Answers {
			segments := match.FindStringSubmatch(choice.Title)
			if len(segments) <= 1 { //def not one
				pollVotes = 0
				break
			}
			title := strings.ToUpper(segments[1])
			for _, id := range choice.Voters {
				// ignore bad titles & crop comment
				choice, ok := answerMap[title]
				if !ok {
					continue
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
		//likely not a rating
		if pollVotes == 0 {
			delete(mappedRecData, rec.Title)
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
		thisPoll, ok := mappedRecData[pair.title]
		if !ok { //zero-vote poll, dropped upstream
			continue
		}
		time, title, votes := time.Unix(pair.time, 0).Format("01/02/2006"), pair.title, *pair.votes
		fmt.Fprintf(&out, "\n%s, %s, %d, ", time, title, votes)

		for _, id := range voterOrder {
			msg, ok := thisPoll[id]
			if !ok {
				msg = "N/A"
			}
			fmt.Fprintf(&out, "%s, ", msg)
		}
		out.WriteString("<END>")
	}
	if file != nil {
		_, err := file.WriteString(out.String())
		return &out, err
	}
	return &out, nil
}
