package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"poll-bot/root/bot"
	"poll-bot/root/commands"
	"poll-bot/root/info/version"
	"poll-bot/root/managers/aliases"
	"poll-bot/root/managers/audit"
	"poll-bot/root/managers/authorize"
	"poll-bot/root/managers/components"
	"poll-bot/root/managers/polls"
	"poll-bot/root/managers/web"
	"strconv"
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
	DATA_PATH         = "./data/"
	LOG_PATH          = DATA_PATH + "logs/"
	ALIAS_PATH        = DATA_PATH + "aliases.db"
	DISCORD_AUTH_PATH = DATA_PATH + "discord_auth.db"
	POLLS_PATH        = DATA_PATH + "polls/"
	//
	WEB_ROOT_PATH = "" // you should point this port to the root externally
	WEB_AUTH_PATH = DATA_PATH + "web_auth.db"
)

type serverInfo struct {
	port         int
	signingToken string
}

func setEnvironmentVars() (token string, mode string, server *serverInfo) {
	mode = strings.ToUpper(os.Getenv("MODE"))
	if mode == "" {
		log.Fatal(errors.New("you should provide a MODE"))
	}
	token = os.Getenv(mode + "_TOKENID")
	if token == "" {
		log.Fatal(fmt.Errorf("no token for MODE '%s' (canceled)", mode))
		return
	}
	tryPort := os.Getenv("SERVER_PORT")
	if tryPort != "" {
		port, err := strconv.Atoi(tryPort)
		if err != nil {
			log.Fatal(fmt.Errorf("invalid SERVER_PORT %s: %v", tryPort, err))
		}
		signingSecret := os.Getenv("JWT_SECRET")
		if signingSecret == "" {
			log.Fatal(fmt.Errorf("SERVER_PORT present without JWT_SECRET"))
		}
		server = &serverInfo{port, signingSecret}
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

func getManAliases() *aliases.AliasManager {
	man, err := aliases.New(ALIAS_PATH)
	if err != nil {
		logger.Panic(fmt.Sprintf("Couldn't init aliases: %v", err), audit.LogGroup.INIT)
		return nil
	}
	return man
}
func getManAuth() *authorize.AuthManager {
	man, err := authorize.New(DISCORD_AUTH_PATH)
	if err != nil {
		logger.Panic(fmt.Sprintf("Couldn't init auth manager: %v", err), audit.LogGroup.INIT)
		return nil
	}
	return man
}
func getManPolls() *polls.PollManager {
	man, err := polls.New(POLLS_PATH)
	if err != nil {
		logger.Panic(fmt.Sprintf("Couldn't init poll manager: %v", err), audit.LogGroup.INIT)
		return nil
	}
	return man
}
func getManWeb(info *serverInfo) *web.WebManager {
	if info == nil {
		return nil
	}
	//
	man, err := web.New(info.port, WEB_ROOT_PATH, WEB_AUTH_PATH, []byte(info.signingToken))
	if err != nil {
		logger.Panic(fmt.Sprintf("Couldn't init poll manager: %v", err), audit.LogGroup.INIT)
		return nil
	}
	logger.Add(fmt.Sprintf("Starting webserver on port %d:%s", info.port, WEB_ROOT_PATH))
	return man
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

	token, mode, server := setEnvironmentVars()
	setupLogger(mode)
	defer logger.Close() // nolint

	checkForUpdates()

	commands := commands.Register(commands.RegisterReqs{
		Aliases:    getManAliases(),
		Auth:       getManAuth(),
		Polls:      getManPolls(),
		Components: components.New(),
		WebManager: getManWeb(server),
	})

	//

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
