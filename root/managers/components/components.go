package components

import (
	"sync"
	"sync/atomic"

	"github.com/bwmarrin/discordgo"
)

type grouping = string
type snowflake = string
type callbackId = string
type Callback = func(optIntxnUpdate *discordgo.InteractionCreate) (done bool, err error)

type ComponentCallbacks struct {
	intxnID   snowflake
	callbacks map[callbackId]Callback
	busy      atomic.Bool
}
type GroupMetadata struct {
	FromHandle         string
	NewInvalidatesOld  bool
	InvalidationCloses bool
}
type ComponentCallbackManager struct {
	mutex        sync.RWMutex
	groupMutexes map[grouping]*sync.RWMutex
	data         map[grouping]map[snowflake]*ComponentCallbacks // returns true when interaction should be closed
	// points to interaction; points to set of related callbacks

	inverse map[snowflake]grouping
	//
	rules map[grouping]GroupMetadata
}

func New() *ComponentCallbackManager {
	return &ComponentCallbackManager{
		groupMutexes: make(map[grouping]*sync.RWMutex),
		data:         make(map[grouping]map[snowflake]*ComponentCallbacks),
		inverse:      make(map[snowflake]grouping),
		rules:        make(map[grouping]GroupMetadata),
	}
}

func NewComponentCallbacks(id snowflake, funcs map[callbackId]Callback) *ComponentCallbacks {
	return &ComponentCallbacks{
		intxnID:   id,
		callbacks: funcs,
	}
}
