package polls

import (
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
	// rid dupes
	pruned := make([]Poll, 0, len(values))
	var dropped int
	for _, poll := range values {
		if !man.set.TryInsert(poll.Message.ID) {
			dropped++
			continue
		}
		pruned = append(pruned, poll)
	}
	//
	if dropped == len(values) {
		return dropped, nil
	}
	return dropped, man.queue.Merge(pruned...)
}
func (man *PollManager) Pop() (poll Poll, err error) {
	poll, err = man.queue.Pop()
	if err != nil {
		return
	}
	//rid entry
	man.set.Remove(poll.Message.ID)
	return
}

// just { Message: } is enough
func (man *PollManager) Has(poll Poll) bool {
	return man.set.Has(poll.Message.ID)
}
func (man *PollManager) Peek() (Poll, bool) {
	return man.queue.Peek()
}
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
