package polls

import (
	"bytes"
	"compress/gzip"
	"errors"
	"log"
	"net/http"
	"poll-bot/root/api/general"
	"poll-bot/root/csv"
	"strconv"
	"strings"
)

var ErrBadPage = errors.New("invalid page number")

func getPageContent(w http.ResponseWriter, page int) (data *strings.Builder, nextPage int, err error) {
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
	records, err := bcp.Polls.GetFinalized(PAGE_SIZE*(page-1), PAGE_SIZE+1)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return nil, 0, err
	}
	if len(records) == 0 {
		return nil, 0, nil
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

	// default to page 1 (aka 0)
	page, err := strconv.Atoi(params.Get("page"))
	writer, nextPage, err := getPageContent(w, page)
	if err != nil {
		log.Println(err)
		return //getPageContent will handle
	}
	//

	var buf bytes.Buffer
	zipped := gzip.NewWriter(&buf)
	zipped.Write([]byte(writer.String()))
	zipped.Close()

	header.Add("Content-Encoding", "gzip")
	header.Add("Content-Type", "text/csv")
	header.Add("X-Next-Page", strconv.Itoa(nextPage))
	header.Add("Content-Length", strconv.Itoa(len(buf.Bytes())))
	//
	w.Write(buf.Bytes())
	w.WriteHeader(200)

}
