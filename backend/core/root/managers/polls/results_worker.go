package polls

import (
	"context"
	"errors"
	"fmt"
	"poll-bot/root/managers/audit"
	"sync"
	"sync/atomic"
	"time"
)

type subscription[T any] struct {
	awaiting *atomic.Bool
	ch       *chan T
}
type workerMessage struct {
	status   string
	workerid string
	message  any
}

func (wr *resultsWorker) sendMessage(status string, msg any) {
	wr.status.Store(&workerMessage{
		status:   status,
		workerid: wr.id,
		message:  msg,
	})
	wr.logger.Add(fmt.Sprintf("%s %s: %v", wr.id, status, msg), audit.LogGroup.WORKER)
}

const (
	WORKER_UNSTARTED = "Unstarted"
	WORKER_BUSY      = "Busy"
	WORKER_PAUSED    = "Paused"
	WORKER_STOPPING  = "Stopping"
	WORKER_STOPPED   = "Stopped"
)

type nothing = int

type resultsWorker struct {
	id        string
	man       *PollManager
	max_batch int
	batch     []Poll                         // only read when worker is paused
	status    *atomic.Pointer[workerMessage] // mutate pointer to new message
	sub       subscription[nothing]
	halt      chan nothing
	//
	stopped *atomic.Bool
	stopper *sync.Once
	logger  *audit.Log
}

func newResultsWorker(man *PollManager, id string, max_batch int, l *audit.Log) (*resultsWorker, error) {
	if man == nil {
		return nil, errors.New("no manager assigned, cannot start")
	}
	if max_batch < 1 {
		return nil, errors.New("batch size must be at least 1")
	}
	worker := resultsWorker{
		id:        id,
		man:       man,
		max_batch: max_batch,
		batch:     make([]Poll, 0, max_batch),
		status:    &atomic.Pointer[workerMessage]{},
		sub: subscription[nothing]{
			awaiting: &atomic.Bool{},
			ch:       &man.queueSubCh,
		},
		halt:    make(chan int),
		stopped: &atomic.Bool{},
		stopper: &sync.Once{},
		logger:  l,
	}

	worker.sendMessage(WORKER_UNSTARTED, "")
	return &worker, nil
}

func (wr *resultsWorker) checkForStop() bool {
	if wr.stopped.Load() {
		return true
	}
	//
	select {
	case <-wr.halt:
		wr.stopped.Store(true)
		return true
	default:
		return false
	}

}

// poll only present when it knows how long til expiry
func (wr *resultsWorker) tryAddToBatch() (ok bool, full bool, poll *Poll) {
	if len(wr.batch) == wr.max_batch {
		return true, true, nil
	}

	// the expiry check has to happen inside the take, or another worker can
	// swap the top out between deciding and popping.
	poll, taken, err := wr.man.queue.TryTake(func(p *Poll) bool {
		return p.Expiry == nil || !time.Now().Before(*p.Expiry)
	})
	if err != nil {
		wr.logger.Warn(fmt.Sprintf("while taking from queue: %v", err), audit.LogGroup.WORKER)
	}
	if poll == nil { //queue empty; caller handles batch state
		return false, false, nil
	}
	if !taken { //not expired; caller waits on it
		return false, false, poll
	}
	wr.batch = append(wr.batch, *poll)
	wr.man.set.Remove(poll.Message.ID)
	wr.man.releaseSubscribers()
	return true, len(wr.batch) == wr.max_batch, nil
}

func (wr *resultsWorker) waitForPoll(p *Poll) {
	if wr.checkForStop() {
		return
	}
	// if no poll passed, basic weight
	if p == nil {
		wr.sendMessage(WORKER_PAUSED, "Empty queue.")
		select {
		case *wr.sub.ch <- 0:
		case <-wr.halt:
			wr.stopped.Store(true)
		}
		return
	}

	wr.sendMessage(WORKER_PAUSED, "Awaiting top expiry / new top.")

	ctx, cancel := context.WithDeadline(context.Background(), *p.Expiry)
	defer cancel()

	select {
	case *wr.sub.ch <- 0:
	case <-ctx.Done():
	case <-wr.halt:
		wr.stopped.Store(true)
	}
}
func (wr *resultsWorker) handleInsertStatus(ch chan int, cher <-chan error) (ok bool, idx int) {
	var status int
	var err error
	select {
	case status = <-ch:
	case err = <-cher:
		status = <-ch
	}
	switch status {
	case INS_ERR_NEW:
		// A poll whose message Discord no longer has can never be finalized,
		// so retrying it is a permanent loop rather than a slow recovery. It
		// is reported here, at the moment of the decision, because the batch
		// counts printed at the end cannot distinguish a drop from a write.
		var gone *MessageNotFound
		if errors.As(err, &gone) {
			//the title lives on the message, which is exactly what is missing,
			//so the id is all there is to name it by.
			wr.sendMessage(WORKER_BUSY, fmt.Sprintf("I dropped poll %s due to: %v", gone.ID, gone.cause))
			ch <- INS_ERR_RECOVER_DROP
			return wr.handleInsertStatus(ch, cher)
		}
		wr.logger.Warn(fmt.Sprintf("couldn't gen a record: %v", err), audit.LogGroup.WORKER)
		ch <- INS_ERR_RECOVER_BREAK_IGNORE
		// Insert goroutine keeps going; wait for its next status so it
		// doesn't deadlock trying to write to `done` with no reader.
		return wr.handleInsertStatus(ch, cher)
	case INS_WRITE_FAILED:
		wr.logger.Warn(fmt.Sprintf("results db write failed: %v", err), audit.LogGroup.WORKER)
		// TODO
		return false, status
	case INS_WRITE_SUCCESS:
		//TODO
		return false, status
	default: //index -i
		ch <- INS_FETCH_CONTINUE
		return true, -status
	}
}
func (wr *resultsWorker) Start() {
	if wr.status.Load().status != WORKER_UNSTARTED {
		return
	}
	man := wr.man
	wr.sendMessage(WORKER_BUSY, "Begin loop")

	var finalIndex int
	for !wr.checkForStop() {

		//subscribes only when failed to add & batch is not empty
		if added, full, poll := wr.tryAddToBatch(); !added && len(wr.batch) == 0 {
			wr.waitForPoll(poll)
			continue
		} else if added && !full {
			continue //continue add
		}

		// else buffer is ready
		ch, cher := make(chan int), make(chan error)
		go man.Insert(CACHEDFETCH, ch, cher, wr.batch...) //nolint

		batchSize := len(wr.batch)
		wr.sendMessage(WORKER_BUSY, fmt.Sprintf("I got %d polls.", batchSize))
		for !wr.checkForStop() {
			cont, i := wr.handleInsertStatus(ch, cher)
			if !cont {
				var toRequeue []Poll
				switch i {
				case INS_WRITE_FAILED:
					toRequeue = wr.batch //none made it to db, requeue all
					wr.sendMessage(WORKER_BUSY, fmt.Sprintf("I failed completely. Returned %d polls", batchSize))
				case INS_WRITE_SUCCESS:
					toRequeue = wr.batch[finalIndex:] //unwritten tail only
					wr.sendMessage(WORKER_BUSY, fmt.Sprintf("I wrote %d/%d polls.", finalIndex, batchSize))
				}
				// via Push, not queue.Merge: Insert re-queues polls Discord
				// has not finalized yet, so the tail can contain ids that are
				// already back in the heap. Merge bypasses man.set and would
				// seat a second copy of each.
				man.Push(toRequeue...) //nolint
				clear(wr.batch)
				wr.batch = wr.batch[:0] //reset len, keep original cap
				finalIndex = 0
				break
			}
			finalIndex = i + 1 //cutoff: count of items prepared so far
			// wr.sendMessage(WORKER_BUSY, fmt.Sprintf("Building rows from batch... %d / %d", finalIndex, batchSize))
		}
	}
	//do cleanup
	wr.sendMessage(WORKER_STOPPING, "Cleaning up")
	man.Push(wr.batch[finalIndex:]...) //nolint
	//signal safe to mutate
	wr.halt <- 0
	wr.sendMessage(WORKER_STOPPED, "Done")
}

// blocking function. stopped is set by checkForStop
func (wr *resultsWorker) Stop(timeout time.Duration) error {
	var err error
	//safe to terminate immediately if not mutating []Poll
	wr.stopper.Do(func() {
		status := wr.status.Load().status
		switch status {
		case WORKER_BUSY:
		case WORKER_PAUSED:
			wr.man.releaseSubscribers()
		default:
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		//send stop
		select {
		case wr.halt <- 0:
		case <-ctx.Done(): //timeout
			err = ctx.Err()
			return
		}
		//get ack
		select {
		case <-wr.halt:
		case <-ctx.Done(): //timeout
			err = ctx.Err()
			return
		}
		//return values
		_, err = wr.man.Push(wr.batch...)
	})
	return err
}
