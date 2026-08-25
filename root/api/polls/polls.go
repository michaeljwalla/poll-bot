package polls

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"poll-bot/root/api/general"
	"poll-bot/root/csv"
	"poll-bot/root/info/unix"
	"poll-bot/root/managers/polls"
	"slices"
	"strconv"
	"strings"
	"time"
)

var ErrBadPage = errors.New("invalid page number")
var ErrBadOmitInactive = errors.New("omitInactive is not a boolean")
var ErrInvalidVoteChoice = errors.New("no such vote choice")
var ErrBadPollID = errors.New("poll id is not a snowflake")
var ErrDuplicateVoter = errors.New("voter submitted twice for one poll")
var ErrDuplicatePollID = errors.New("poll id submitted twice in one batch")

func getPageContent(w http.ResponseWriter, page int, omitInactive bool) (data *strings.Builder, nextPage int, err error) {
	const PAGE_SIZE = 50
	if page < 0 {
		general.ErrWrite(w, http.StatusBadRequest, ErrBadPage)
		return nil, 0, ErrBadPage
	} else if page == 0 {
		page = 1
	}

	bcp, err := general.GetBCP()
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return nil, 0, err
	}
	// get PAGE_SIZE+1 to check if another page exists
	records, err := bcp.Polls.GetFinalized(PAGE_SIZE*(page-1), PAGE_SIZE+1, omitInactive)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return nil, 0, err
	}
	if len(records) == 0 {
		//an empty page is not an error, but the caller writes this builder
		//unconditionally, so it must not be nil.
		return &strings.Builder{}, 0, nil
	}

	out, err := csv.ToCSV("", records[:min(PAGE_SIZE, len(records))], bcp.Aliases)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return nil, 0, err
	}

	if len(records) > PAGE_SIZE {
		nextPage = page + 1
	}
	return out, nextPage, nil
}

func GetPage(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	params := r.URL.Query()

	//absent is page 1 (aka 0); a present but unparseable value is a client error
	page := 0
	if raw := params.Get("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			general.ErrWrite(w, http.StatusBadRequest, ErrBadPage)
			return
		}
		page = parsed
	}
	//absent means omit them; only an explicit falsey value asks for inactive rows
	omitInactive := true
	if raw := params.Get("omitInactive"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			general.ErrWrite(w, http.StatusBadRequest, ErrBadOmitInactive)
			return
		}
		omitInactive = parsed
	}

	writer, nextPage, err := getPageContent(w, page, omitInactive)
	if err != nil {
		log.Println(err)
		return //getPageContent will handle
	} else if nextPage == 0 && writer.Len() == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	//

	var buf bytes.Buffer
	zipped := gzip.NewWriter(&buf)
	_, err = zipped.Write([]byte(writer.String()))
	zipped.Close() //nolint
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}

	header.Add("Content-Encoding", "gzip")
	header.Add("Content-Type", "text/csv")
	header.Add("X-Next-Page", strconv.Itoa(nextPage))
	header.Add("Content-Length", strconv.Itoa(len(buf.Bytes())))
	//
	_, err = w.Write(buf.Bytes())
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(200)

}

type VoterWeb struct {
	ID     string `json:"id"`
	Choice int    `json:"choice"`
}
type NewPollWeb struct {
	ID    string     `json:"id"`
	Title string     `json:"title"`
	Votes []VoterWeb `json:"votes"`
}

var NO_EXPIRY = time.Time{}

// Choice is an index into this set. The exporter matches these titles
// verbatim (see csv.answerMap), so a manual poll has to speak the same
// vocabulary as a Discord one or it drops out of the CSV.
var choiceOptions = []string{"N/A", "⭐", "⭐⭐", "⭐⭐⭐", "⭐⭐⭐⭐", "⭐⭐⭐⭐⭐"}

// Discord numbers answers within a poll message; a manual poll has no such
// message, so there is no id worth keeping.
const MANUAL_ANSWER_ID = -1

func AddPolls(w http.ResponseWriter, r *http.Request) {
	var pollData []NewPollWeb
	err := json.NewDecoder(r.Body).Decode(&pollData)
	if err != nil {
		general.ErrWrite(w, http.StatusBadRequest, err)
		return
	}

	var records = make([]*polls.FinalRecord, 0, len(pollData))
	//a repeated id would silently REPLACE its way down to whichever copy
	//landed last, so the batch would report more polls than it stored.
	seenPolls := make(map[string]bool, len(pollData))
	for _, p := range pollData {
		//the id doubles as the poll's date downstream, so a value the
		//exporter cannot decode would silently drop the whole row.
		if _, err := unix.SnowflakeToTime(p.ID); err != nil {
			general.ErrWrite(w, http.StatusBadRequest, ErrBadPollID)
			return
		}
		if seenPolls[p.ID] {
			general.ErrWrite(w, http.StatusBadRequest, ErrDuplicatePollID)
			return
		}
		seenPolls[p.ID] = true

		//every option gets an answer, voted or not, so a manual record has
		//the same shape as one Insert builds from a Discord poll.
		answers := make([]*polls.RecordAnswer, len(choiceOptions))
		for i, option := range choiceOptions {
			answers[i] = &polls.RecordAnswer{Title: option, ID: MANUAL_ANSWER_ID}
		}
		// validate vote
		//one voter under two choices lands in two answers, which inflates the
		//exported vote total and then keeps only the higher-indexed choice.
		seenVoters := make(map[string]bool, len(p.Votes))
		for _, vote := range p.Votes {
			if vote.Choice < 0 || vote.Choice >= len(choiceOptions) {
				general.ErrWrite(w, http.StatusBadRequest, ErrInvalidVoteChoice)
				return
			}
			if seenVoters[vote.ID] {
				general.ErrWrite(w, http.StatusBadRequest, ErrDuplicateVoter)
				return
			}
			seenVoters[vote.ID] = true

			answer := answers[vote.Choice]
			answer.Voters = append(answer.Voters, vote.ID)
		}

		records = append(records, &polls.FinalRecord{
			Answers:  answers,
			Options:  slices.Clone(choiceOptions),
			Metadata: polls.Poll{Message: polls.Message{ID: p.ID}, Expiry: &NO_EXPIRY},
			Title:    p.Title,
		})
	}

	//nothing is written until the whole batch validates, so a bad record
	//partway through cannot leave half a request in the table.
	bcp, err := general.GetBCP()
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	if err := bcp.Polls.InsertManual(records...); err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

type SetActiveWeb struct {
	ID     string `json:"id"`
	Active bool   `json:"active"`
}

func SetActive(w http.ResponseWriter, r *http.Request) {
	bcp, err := general.GetBCP()
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	var setters []SetActiveWeb
	err = json.NewDecoder(r.Body).Decode(&setters)
	//
	if err != nil {
		general.ErrWrite(w, http.StatusBadRequest, err)
		return
	}

	var active, inactive = make([]string, 0), make([]string, 0)
	for _, setter := range setters {
		if setter.Active {
			active = append(active, setter.ID)
		} else {
			inactive = append(inactive, setter.ID)
		}
	}
	err = bcp.Polls.SetActive(false, inactive...)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	err = bcp.Polls.SetActive(true, active...)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(200)
}

type SetTitleWeb struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func SetTitle(w http.ResponseWriter, r *http.Request) {
	bcp, err := general.GetBCP()
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	var setters []SetTitleWeb
	err = json.NewDecoder(r.Body).Decode(&setters)
	if err != nil {
		general.ErrWrite(w, http.StatusBadRequest, err)
		return
	}

	//a repeated id would silently keep whichever copy landed last in the map
	//below, so the batch would report more titles set than it stored.
	overrides := make(map[string]string, len(setters))
	for _, setter := range setters {
		if _, err := unix.SnowflakeToTime(setter.ID); err != nil {
			general.ErrWrite(w, http.StatusBadRequest, ErrBadPollID)
			return
		}
		if _, seen := overrides[setter.ID]; seen {
			general.ErrWrite(w, http.StatusBadRequest, ErrDuplicatePollID)
			return
		}
		overrides[setter.ID] = setter.Title
	}

	if err := bcp.Polls.SetTitleOverride(overrides); err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(200)
}

func DropPolls(w http.ResponseWriter, r *http.Request) {
	bcp, err := general.GetBCP()
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	var ids []string
	err = json.NewDecoder(r.Body).Decode(&ids)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	err = bcp.Polls.Drop(ids...)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
