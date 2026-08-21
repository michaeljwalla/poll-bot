package polls

import (
	"net/http"
)

func GetPage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
