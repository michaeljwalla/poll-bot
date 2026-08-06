package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"poll-bot/src/aliases"
	"poll-bot/src/audit"
	"poll-bot/src/authorize"
	"poll-bot/src/bot"
	"poll-bot/src/commands"
	"poll-bot/src/version"
	"strings"
	"syscall"
)

// some globals
var (
	logger *audit.Log
)

var (
	LogFlag  = audit.LogFlag
	LogGroup = audit.LogGroup
)

const (
	DATA_PATH  = "./data/"
	LOG_PATH   = DATA_PATH + "logs/"
	ALIAS_PATH = DATA_PATH + "aliases.json"
	AUTH_PATH  = DATA_PATH + "auth.json"
)

func setEnvironmentVars() (token string, mode string) {
	mode = strings.ToUpper(os.Getenv("MODE"))
	if mode == "" {
		log.Fatal(errors.New("you should provide a MODE"))
	}
	token = os.Getenv(mode + "_TOKENID")
	if token == "" {
		log.Fatal(fmt.Errorf("no token for MODE '%s' (canceled)", mode))
		return
	}
	return
}

func setupLogger(mode string) {
	lg, err := audit.New(LOG_PATH+mode, LogFlag.Default, audit.LogGroup.INIT)
	if err != nil {
		log.Fatal(fmt.Errorf("couldn't init logs: %v", err))
	}
	logger = lg
	logger.Add(fmt.Sprintf("Running in '%s' mode", mode), audit.LogGroup.INIT)
}

func setAliases() *aliases.AliasTable {
	table, err := aliases.New(ALIAS_PATH)
	if err != nil {
		logger.Panic(fmt.Sprintf("Couldn't get aliases: %v", err), audit.LogGroup.INIT)
		return nil
	}
	return table
}
func setAuth() *authorize.AuthTable {
	table, err := authorize.New(ALIAS_PATH)
	if err != nil {
		logger.Panic(fmt.Sprintf("Couldn't get auth: %v", err), audit.LogGroup.INIT)
		return nil
	}
	return table
}
func init() {
	log.SetFlags(log.Ldate) //only date no time
}
func persist() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func checkForUpdates() {
	logger.Add(version.Format(), audit.LogGroup.INIT)
	//
	fetch, err := version.TryForUpdates()
	if err != nil {
		logger.Warn(fmt.Sprintf("Failed to check for updates: %v", err), audit.LogGroup.INIT)
	}
	if fetch == nil {
		return
	}
	//
	logger.Add("Checking for updates...", audit.LogGroup.INIT)
	message, err := fetch()
	if err != nil {
		logger.Add(fmt.Sprintf("Failed to check for updates: %v", err), audit.LogGroup.INIT)
	} else if message != "" {
		logger.Add(message, audit.LogGroup.INIT)
	}
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

	token, mode := setEnvironmentVars()
	setupLogger(mode)
	defer logger.Close() // nolint

	checkForUpdates()

	commands := commands.Register(commands.RegisterReqs{
		Aliases: setAliases(),
		Auth:    setAuth(),
	})
	session, err := bot.Start(bot.StartInstructions{
		Token:    token,
		Commands: commands,
		Logger:   logger,
	})
	if err != nil {
		logger.Panic(fmt.Errorf("error creating Discord session: %v", err))
		return
	}
	defer bot.EndSession(session)

	persist()
}
