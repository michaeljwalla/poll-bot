package polls

import (
	"errors"
	"slices"
	"time"
)

// TODO make set.Set an interface so you can optionally make it async safe

// pushing once is O(logn)
//
// pushing multiple initiates re-heapify O(n)
func (man *PollManager) Push(values ...Poll) (dupes int, err error) {
	if len(values) == 0 {
		return 0, nil
	}
	// rid dupes (in-flight/queued via set, already-finalized via DB)
	pruned := make([]Poll, 0, len(values))
	var dropped int
	for _, poll := range values {
		if !man.set.TryInsert(poll.Message.ID) {
			dropped++
			continue
		}
		if man.hasFinalized(poll.Message.ID) {
			man.set.Remove(poll.Message.ID) //undo insert
			dropped++
			continue
		}
		pruned = append(pruned, poll)
	}
	//
	if dropped == len(values) {
		return dropped, nil
	}

	pre, hadPre := man.queue.Peek()
	if err := man.queue.Merge(pruned...); err != nil {
		return -1, err
	}
	// release subs if top changes
	post, _ := man.queue.Peek()
	if !hadPre || pre.Message.ID != post.Message.ID {
		man.releaseSubscribers()
	}

	return dropped, nil
}

func (man *PollManager) Path() string {
	return man.path
}

// just { Message: } is enough
func (man *PollManager) Has(poll Poll) bool {
	if man.set.Has(poll.Message.ID) {
		return true
	}
	return man.hasFinalized(poll.Message.ID)
}

// checks the results DB for a prior finalization of this id
func (man *PollManager) hasFinalized(id snowflake) bool {
	iter, err := man.finalized.Query(`SELECT 1 FROM results WHERE id = ? LIMIT 1;`, id)
	if err != nil {
		return false
	}
	defer iter.Close() //nolint
	var one int
	found, _ := iter.NextScan(&one)
	return found
}

//	func (man *PollManager) Peek() (Poll, bool) {
//		return man.queue.Peek()
//	}
func (man *PollManager) GetTopOrdered() ([]Poll, bool) {
	iTop := 0
	top, ok := man.queue.At(iTop)
	if !ok {
		return nil, false
	}
	data := make([]Poll, 0, 3)
	data = append(data, top)
	//
	left, ok := man.queue.Left(iTop)
	if ok {
		data = append(data, left)
	}
	right, ok := man.queue.Right(iTop)
	if ok {
		data = append(data, right)
	}
	slices.SortFunc(data, func(a Poll, b Poll) int {
		timeA := time.Time{}
		timeB := time.Time{}
		if a.Expiry != nil {
			timeA = *a.Expiry
		}
		if b.Expiry != nil {
			timeB = *b.Expiry
		}
		return timeA.Compare(timeB)
	})
	return data, true
}

// drops every queued poll whose Discord message is gone
func (man *PollManager) CleanQueue() (dropped int, err error) {
	man.queue.Lock()
	defer man.queue.Unlock()
	//
	data := make([]Poll, 0, man.queue.LenLocked())
	stale := make([]snowflake, 0)
	for poll := range man.queue.IterLocked() {
		if _, err := man.GetData(&poll); errors.Is(err, ErrMessageNotFound) {
			stale = append(stale, poll.Message.ID)
			continue
		}
		data = append(data, poll)
	}
	if len(stale) == 0 { //nothing to drop, so nothing to rewrite
		return 0, nil
	}
	if err := man.queue.ClearLocked(); err != nil {
		return 0, err
	}
	if err := man.queue.MergeLocked(data...); err != nil {
		return 0, err
	}
	// the ids live in the set only to dedupe pushes; a message that no longer
	// exists must not keep occupying its slot
	for _, id := range stale {
		man.set.Remove(id)
	}
	return len(stale), nil
}
