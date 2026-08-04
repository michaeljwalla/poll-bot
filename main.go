package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"poll-bot/src/audit"
	"poll-bot/src/bot"
	"poll-bot/src/commands"
	"poll-bot/src/version"
	"regexp"
	"strings"
	"syscall"
)

var logger *audit.Log
var TOKEN string
var aliases map[string]string

var LogFlag = audit.LogFlag

const DATA_PATH = "./data/"
const LOG_PATH = DATA_PATH + "logs/"
const ALIAS_PATH = DATA_PATH + "aliases.json"

var MODE string

func setupLogger() {
	lg, err := audit.New(LOG_PATH+MODE, LogFlag.Default, audit.LogGroup.INIT)
	if err != nil {
		log.Fatal(fmt.Errorf("couldn't init logs: %v", err))
	}
	logger = lg
	logger.Add(fmt.Sprintf("Running in '%s' mode", MODE), audit.LogGroup.INIT)
}
func setEnvironmentVars() {
	MODE = strings.ToUpper(os.Getenv("MODE"))
	if MODE == "" {
		log.Fatal(errors.New("you should provide a MODE"))
	}
	TOKEN = os.Getenv(MODE + "_TOKENID")
	if TOKEN == "" {
		log.Fatal(fmt.Errorf("no token for MODE '%s' (canceled)", MODE))
		return
	}
}
func setAliases() {
	if ALIAS_PATH == "" {
		return
	}
	data, err := os.OpenFile(ALIAS_PATH, os.O_RDONLY, 0644)
	if err != nil {
		logger.Warn("ALIAS_PATH provided but failed to load.", audit.LogGroup.INIT)
		return
	}
	if err := json.NewDecoder(data).Decode(&aliases); err != nil {
		logger.Warn(fmt.Sprintf("Couldn't get aliases: %v", err), audit.LogGroup.INIT)
		return
	}
	logger.Add("Loaded aliases.json", audit.LogGroup.INIT)
}
func init() {
	log.SetFlags(log.Ldate) //only date no time
}
func persist() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
func checkVersion() error {
	logger.Add(version.Format(), audit.LogGroup.INIT)
	source := version.Source()
	switch source {
	case "Unknown":
		return errors.New("no repo provided")
	case "local":
		return nil
	}

	logger.Add("Checking for updates...", audit.LogGroup.INIT)

	resp, err := http.Get(source)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("%v %v", resp.StatusCode, resp.Body)
	}
	defer resp.Body.Close()

	var data string
	if body, err := io.ReadAll(resp.Body); err != nil {
		return err
	} else {
		data = string(body)
	}
	// not making struct for this and not learning go lib for single use
	re := regexp.MustCompile(`(?s)"tag_name":"([^"]+)",.+"prerelease":(true|false)`)
	match := re.FindStringSubmatch(data)
	if match == nil {
		return errors.New("regex capture failed to read")
	}

	var out string
	if version.Version() != match[1] {
		out = "A newer %srelease (%s) is available."
	} else {
		return nil
	}
	var preRelease = ""
	if match[2] == "true" {
		preRelease = "pre"
	}
	logger.Add(fmt.Sprintf(out, preRelease, match[1]), audit.LogGroup.INIT)
	return nil
}
func main() {
	if len(os.Args) > 1 {
		if os.Args[1] == "--version" || os.Args[1] == "-v" {
			fmt.Printf("version: %s\nsource:  %s\n", version.Version(), version.Source())
			os.Exit(0)
		} else {
			fmt.Printf("Run with no commands to start the bot.")
			os.Exit(1)
		}
		return
	}

	setEnvironmentVars()
	setupLogger()
	defer logger.Close() // nolint

	if err := checkVersion(); err != nil {
		logger.Warn(fmt.Sprintf("Couldn't check for updates: %v", err), audit.LogGroup.INIT)
	}
	setAliases()
	//
	commands := commands.MainCommands
	session, err := bot.Start(bot.StartInstructions{
		Token:         TOKEN,
		Commands:      commands,
		Logger:        logger,
		TargetAliases: aliases,
	})
	if err != nil {
		logger.Panic(fmt.Errorf("error creating Discord session: %v", err))
		return
	}
	defer bot.EndSession(session)

	persist()
}
