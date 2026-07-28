package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"poll-bot/src/audit"
	"poll-bot/src/bot"
	"poll-bot/src/commands"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
)

var logger *audit.Log
var TOKEN string
var aliases map[string]string

var LogFlag = audit.LogFlag

const DATA_PATH = "./data/"
const LOG_PATH = DATA_PATH + "/logs/"
const ALIAS_PATH = DATA_PATH + "aliases.json"

var f = fmt.Sprintf

func setupLogger() {
	lg, err := audit.New(LOG_PATH, LogFlag.Default, audit.LogGroup.INIT)
	if err != nil {
		log.Fatal(f("Couldn't init logs: %v", err))
	}
	logger = lg
}
func setEnvironmentVars() {
	if err := godotenv.Load(); err != nil {
		logger.Panic("Couldn't load .env", audit.LogGroup.INIT)
		return
	}

	tokenHeader := strings.ToUpper(os.Getenv("MODE"))
	if tokenHeader == "" {
		tokenHeader = "DEV"
	}
	TOKEN = os.Getenv(tokenHeader + "_TOKEN")
	if TOKEN == "" {
		logger.Panic(fmt.Sprintf("No token for mode '%s' (canceled)", tokenHeader), audit.LogGroup.INIT)
		return
	} else {
		logger.Add(fmt.Sprintf("Running in '%s' mode", tokenHeader), audit.LogGroup.INIT)
	}
	logger.Add("Loaded environment variables", audit.LogGroup.INIT)
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
	//
	setupLogger()
	ok := false
	defer func() {
		if !ok {
			logger.Close()
		}
	}()
	//below here uses logger
	setEnvironmentVars()
	setAliases()
	ok = true
}
func persist() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
func main() {
	//init()
	defer logger.Close()
	//
	commands := commands.MainCommands
	session, err := bot.Start(bot.StartInstructions{
		Token:         TOKEN,
		Commands:      commands,
		Logger:        logger,
		TargetAliases: aliases,
	})
	if err != nil {
		logger.Panic(f("Error creating Discord session: %v", err))
		return
	}
	defer bot.EndSession(session)

	persist()
}
