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

var MODE string

var f = fmt.Sprintf

func setupLogger() {
	lg, err := audit.New(LOG_PATH+MODE, LogFlag.Default, audit.LogGroup.INIT)
	if err != nil {
		log.Fatal(f("Couldn't init logs: %v", err))
	}
	logger = lg
	logger.Add(fmt.Sprintf("Running in '%s' mode", MODE), audit.LogGroup.INIT)
}
func setEnvironmentVars() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Couldn't load .env", audit.LogGroup.INIT)
		return
	}

	MODE = strings.ToUpper(os.Getenv("MODE"))
	if MODE == "" {
		MODE = "DEV"
	}
	TOKEN = os.Getenv(MODE + "_TOKENID")
	if TOKEN == "" {
		log.Fatal(fmt.Sprintf("No token for MODE '%s' (canceled)", MODE), audit.LogGroup.INIT)
		return
	}
	return
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
func main() {
	setEnvironmentVars()

	setupLogger()
	defer logger.Close()

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
		logger.Panic(f("Error creating Discord session: %v", err))
		return
	}
	defer bot.EndSession(session)

	persist()
}
