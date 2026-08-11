package components

import (
	"errors"
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// Data Getters
func (man *ComponentCallbackManager) GroupOf(intxnID snowflake) (val grouping, ok bool) {
	man.mutex.RLock()
	defer man.mutex.RUnlock()
	val, ok = man.inverse[intxnID]
	return
}
func (man *ComponentCallbackManager) GetMetadata(group grouping) (*GroupMetadata, error) {
	man.mutex.RLock()
	g, ok := man.rules[group]
	man.mutex.RUnlock()
	if !ok {
		return nil, errors.New("no group metadata present for " + group)
	}
	return &g, nil
}

// private helpers
func doOnce(f func()) func() {
	once := sync.Once{}
	return func() {
		once.Do(f)
	}
}

// applies on register, not on groupadd
func (man *ComponentCallbackManager) applyRules(group grouping) error {
	man.mutex.RLock()
	unlock := doOnce(man.mutex.RUnlock)
	defer unlock()

	rules, ok := man.rules[group]
	if !ok {
		return nil
	}
	unlock()
	if rules.NewInvalidatesOld {
		man.mutex.Lock()
		defer man.mutex.Unlock()
		man.groupMutexes[group].Lock()
		defer man.groupMutexes[group].Unlock()
		//
		for id := range man.data[group] {
			if rules.InvalidationCloses {
				if _, err := man.data[group][id].callbacks["close"](nil); err != nil {
					return fmt.Errorf("while applying rules -> closing invalidated group %s ID %s: %v", group, id, err)
				}
			}
			delete(man.inverse, id)
			delete(man.data[group], id)
		}
	}
	return nil
}

// always runs with context that it can manage its own area
func (man *ComponentCallbackManager) close(group grouping, c *ComponentCallbacks, i *discordgo.InteractionCreate) error {
	man.mutex.Lock()
	defer man.mutex.Unlock()
	//
	_, err := c.callbacks["close"](i)
	if err != nil {
		return fmt.Errorf("(failed) while closing group %s ID %s: err", group, c.intxnID)
	}
	delete(man.inverse, c.intxnID)
	delete(man.data[group], c.intxnID)
	return nil
}

// public setters / do-ers
func (man *ComponentCallbackManager) Register(group grouping, c *ComponentCallbacks) error {
	//get grouping
	man.mutex.RLock()
	g, ok := man.data[group]
	man.mutex.RUnlock()
	if !ok {
		return errors.New("no such grouping: " + group)
	}
	if err := man.applyRules(group); err != nil {
		return err
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

func (man *ComponentCallbackManager) AddGroup(group grouping, rules *GroupMetadata) (exists bool) {
	man.mutex.Lock()
	defer man.mutex.Unlock()
	//
	if _, ok := man.data[group]; ok {
		return true
	}
	man.data[group] = make(map[snowflake]*ComponentCallbacks)
	man.groupMutexes[group] = &sync.RWMutex{}
	if rules != nil {
		man.rules[group] = *rules
	}
	return false
}

func (man *ComponentCallbackManager) TryRun(i *discordgo.InteractionCreate) (busy bool, err error) {
	//find group
	man.mutex.RLock()
	unlockMan := doOnce(man.mutex.RUnlock)
	defer unlockMan()

	groupstr, ok := man.inverse[i.Member.User.ID]

	if !ok {
		return false, fmt.Errorf("Unregistered/Ungrouped interaction ID: %s", i.Member.User.ID)
	}
	//get refs from man
	gmutex := man.groupMutexes[groupstr]
	group := man.data[groupstr]
	unlockMan()

	// get refs from groupset
	gmutex.RLock()
	unlockGroup := doOnce(gmutex.RUnlock)
	defer unlockGroup() //overkill
	c, ok := group[i.Member.User.ID]
	unlockGroup()

	if !ok {
		return false, fmt.Errorf("no registered callback for group %s ID %s", groupstr, i.Member.User.ID)
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

	done, err := callback(i)
	if err != nil {
		return false, err
	}

	//this way you can have a component explicitly for closing,
	//or another callback can signify the close should happen.
	if done && cid != "close" {
		return false, man.close(groupstr, c, i)
	}
	return false, nil
}
