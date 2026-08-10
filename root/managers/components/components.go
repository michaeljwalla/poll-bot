package components

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/bwmarrin/discordgo"
)

type grouping = string
type snowflake = string
type callbackId = string

type ComponentCallbacks struct {
	intxnID   snowflake
	callbacks map[callbackId]func() (done bool, err error)
	busy      atomic.Bool
}

type ComponentCallbackManager struct {
	mutex        sync.RWMutex
	groupMutexes map[grouping]*sync.RWMutex
	data         map[grouping]map[snowflake]*ComponentCallbacks // returns true when interaction should be closed
	// points to interaction; points to set of related callbacks

	inverse map[snowflake]grouping
}

func New() *ComponentCallbackManager {
	return &ComponentCallbackManager{
		groupMutexes: make(map[grouping]*sync.RWMutex),
		data:         make(map[grouping]map[snowflake]*ComponentCallbacks),
		inverse:      make(map[snowflake]grouping),
	}
}

func (man *ComponentCallbackManager) AddGroup(group grouping) (exists bool) {
	man.mutex.Lock()
	defer man.mutex.Unlock()
	//
	if _, ok := man.data[group]; ok {
		return true
	}
	man.data[group] = make(map[snowflake]*ComponentCallbacks)
	man.groupMutexes[group] = &sync.RWMutex{}
	return false
}

func (man *ComponentCallbackManager) Register(group grouping, c *ComponentCallbacks) error {
	//get grouping
	man.mutex.RLock()
	g, ok := man.data[group]
	man.mutex.RUnlock()
	if ok {
		return errors.New("no such grouping: " + group)
	}

	//validate close
	if _, ok := c.callbacks["close"]; !ok {
		return fmt.Errorf("group (%s) register attempt without close(): %v", group, c)
	}

	// push inverse
	man.mutex.Lock()
	man.inverse[c.intxnID] = group
	man.mutex.Unlock()

	//write addition
	gmutex := man.groupMutexes[group]
	gmutex.Lock()
	defer gmutex.Unlock()
	g[c.intxnID] = c
	return nil
}

func doOnce(f func()) func() {
	once := sync.Once{}
	return func() {
		once.Do(f)
	}
}
func (man *ComponentCallbackManager) TryRun(i *discordgo.InteractionCreate) (busy bool, err error) {
	//find group
	man.mutex.RLock()
	unlockMan := doOnce(man.mutex.RUnlock)
	defer unlockMan()

	groupstr, ok := man.inverse[i.ID]

	if !ok {
		return false, fmt.Errorf("Unregistered/Ungrouped interaction ID: %s", i.ID)
	}
	//get refs from man
	gmutex := man.groupMutexes[groupstr]
	group := man.data[groupstr]
	unlockMan()

	// get refs from groupset
	gmutex.RLock()
	unlockGroup := doOnce(man.mutex.RUnlock)
	defer unlockGroup() //overkill
	c, ok := group[i.ID]
	unlockGroup()

	if !ok {
		return false, fmt.Errorf("no registered callback for group %s ID %s", groupstr, i.ID)
	}

	//set busy or fail
	if !c.busy.CompareAndSwap(false, true) {
		return true, nil
	}
	defer c.busy.Store(false)
	//
	cid := i.MessageComponentData().CustomID
	callback, ok := c.callbacks[cid]
	if !ok {
		return false, fmt.Errorf("unknown CustomID: %v", cid)
	}

	done, err := callback()
	if err != nil {
		return false, err
	}

	//this way you can have a component explicitly for closing,
	//or another callback can signify the close should happen.
	if done && cid != "close" {
		return false, man.close(groupstr, c)
	}
	return false, nil
}

// always runs with context that it can manage its own area
func (man *ComponentCallbackManager) close(group grouping, c *ComponentCallbacks) error {
	man.mutex.Lock()
	defer man.mutex.Unlock()
	//
	_, err := c.callbacks["close"]()
	if err != nil {
		return fmt.Errorf("(failed) while closing group %s ID %s: err", group, c.intxnID)
	}
	delete(man.inverse, c.intxnID)
	delete(man.data[group], c.intxnID)
	return nil
}
