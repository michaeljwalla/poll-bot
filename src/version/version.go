package version

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

var (
	version = "vUnknown"
	source  = "Unknown"
)

func Format() string {
	return fmt.Sprintf("Running Poll-Bot %s", version)
}
func Version() string {
	return version
}
func Source() string {
	return source
}

func TryForUpdates() (fetch func() (message string, err error), err error) {
	switch source {
	case "Unknown":
		err = errors.New("no repo provided")
		return
	case "local":
		return
	}
	fetch = getUpdateStatus
	return
}

func getUpdateStatus() (message string, err error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := client.Get(source)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("%v %v", resp.StatusCode, resp.Body)
		return
	}
	defer resp.Body.Close()

	var data string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	} else {
		data = string(body)
	}

	// not making struct for this and not learning go lib for single use
	re := regexp.MustCompile(`(?s)"tag_name":"([^"]+)",.+"prerelease":(true|false)`)
	match := re.FindStringSubmatch(data)
	if match == nil {
		err = errors.New("regex capture failed to read")
		return
	}

	var out string
	if version != match[1] {
		out = "A new %srelease (%s) is available."
	} else {
		message = "Up to date"
		return
	}
	var preRelease = ""
	if match[2] == "true" {
		preRelease = "(pre)"
	}

	message = fmt.Sprintf(out, preRelease, match[1])
	return
}
