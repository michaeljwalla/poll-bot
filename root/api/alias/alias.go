package alias

import (
	"bytes"
	"encoding/json"
	"net/http"
	"poll-bot/root/api/general"
)

type AliasWeb struct {
	ID    string `json:"id"`
	Alias string `json:"alias"`
}

func GetAliases(w http.ResponseWriter, r *http.Request) {
	bcp, err := general.GetBCP()
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	aliases := bcp.Aliases

	aliasList := make([]AliasWeb, 0)
	for uid, alias := range aliases.Iter() {
		aliasList = append(aliasList, AliasWeb{ID: uid, Alias: alias})
	}
	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(&aliasList)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}

	header := w.Header()
	header.Add("Content-Type", "application/json")
	w.Write(buf.Bytes())
	w.WriteHeader(http.StatusOK)
}

func SetAliases(w http.ResponseWriter, r *http.Request) {
	bcp, err := general.GetBCP()
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	aliases := bcp.Aliases

	var updates []AliasWeb
	err = json.NewDecoder(r.Body).Decode(&updates)
	if err != nil {
		general.ErrWrite(w, http.StatusBadRequest, err)
		return
	}

	for _, alias := range updates {
		err = aliases.SetAlias(alias.ID, alias.Alias)
		if err != nil {
			break
		}
	}
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
	}
	w.WriteHeader(http.StatusOK)
}

func DropAliases(w http.ResponseWriter, r *http.Request) {
	bcp, err := general.GetBCP()
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	aliases := bcp.Aliases

	var updates []string
	err = json.NewDecoder(r.Body).Decode(&updates)
	if err != nil {
		general.ErrWrite(w, http.StatusBadRequest, err)
		return
	}

	for _, alias := range updates {
		err = aliases.DropAlias(alias)
		if err != nil {
			break
		}
	}
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
	}
	w.WriteHeader(http.StatusOK)
}
