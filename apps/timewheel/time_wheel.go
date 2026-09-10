// Package timewheel implements a hierarchical timing wheel: an in-process
// wheel built on a time ticker plus a circular array of slots, and a
// distributed wheel built on redis zsets with Lua scripts.
package timewheel

import (
	"container/list"
	"sync"
	"time"
)

// taskElement is a task scheduled on the local time wheel.
type taskElement struct {
	task func()
	key  string
	// executeAt is the deadline at which the task should run. The slot
	// position and cycle are derived from it inside the run loop.
	executeAt time.Time
	// pos is the slot index and cycle the number of full wheel rotations to
	// wait for before execution.
	pos   int
	cycle int
}

// TimeWheel is an in-process timing wheel. Create one with NewTimeWheel and
// stop it with Stop. It is safe for concurrent use: AddTask and RemoveTask may
// be called from any goroutine.
type TimeWheel struct {
	sync.Once
	interval         time.Duration
	ticker           *time.Ticker
	stopc            chan struct{}
	addTaskCh        chan *taskElement
	removeTaskCh     chan string
	slots            []*list.List
	curSlot          int
	keyToTaskElement map[string]*list.Element
}

// NewTimeWheel creates and starts a timing wheel with slotNum slots that
// advances one slot every interval. Non-positive arguments fall back to
// defaults (10 slots, 1s interval).
func NewTimeWheel(slotNum int, interval time.Duration) *TimeWheel {
	if slotNum <= 0 {
		slotNum = 10
	}
	if interval <= 0 {
		interval = time.Second
	}

	t := TimeWheel{
		interval:         interval,
		ticker:           time.NewTicker(interval),
		stopc:            make(chan struct{}),
		keyToTaskElement: make(map[string]*list.Element),
		slots:            make([]*list.List, 0, slotNum),
		addTaskCh:        make(chan *taskElement),
		removeTaskCh:     make(chan string),
	}
	for i := 0; i < slotNum; i++ {
		t.slots = append(t.slots, list.New())
	}
	go t.run()
	return &t
}

// Stop stops the wheel. It is idempotent.
func (t *TimeWheel) Stop() {
	t.Do(func() {
		t.ticker.Stop()
		close(t.stopc)
	})
}

// AddTask schedules task to run when executeAt is reached, identified by key.
// Adding a task whose key was added before replaces the earlier task.
func (t *TimeWheel) AddTask(key string, task func(), executeAt time.Time) {
	t.addTaskCh <- &taskElement{
		key:       key,
		task:      task,
		executeAt: executeAt,
	}
}

// RemoveTask cancels the pending task registered under key, if any.
func (t *TimeWheel) RemoveTask(key string) {
	t.removeTaskCh <- key
}

// run is the single goroutine that owns the wheel state (slots, curSlot and
// the key index). Keeping all mutations here avoids data races.
func (t *TimeWheel) run() {
	defer func() {
		// Never let a panic in the loop crash the process; the wheel stops
		// serving in the worst case.
		_ = recover()
	}()

	for {
		select {
		case <-t.stopc:
			return
		case <-t.ticker.C:
			t.tick()
		case task := <-t.addTaskCh:
			t.addTask(task)
		case removeKey := <-t.removeTaskCh:
			t.removeTask(removeKey)
		}
	}
}

// tick executes the tasks in the current slot and advances the wheel.
func (t *TimeWheel) tick() {
	slot := t.slots[t.curSlot]
	defer t.circularIncr()
	t.execute(slot)
}

// execute runs the due tasks in l and removes them from the wheel.
func (t *TimeWheel) execute(l *list.List) {
	for e := l.Front(); e != nil; {
		taskElement, ok := e.Value.(*taskElement)
		if !ok {
			e = e.Next()
			continue
		}
		if taskElement.cycle > 0 {
			taskElement.cycle--
			e = e.Next()
			continue
		}

		// Run the task, then remove it from the wheel.
		go func() {
			defer func() {
				// A panicking task must not crash the process.
				_ = recover()
			}()
			taskElement.task()
		}()

		next := e.Next()
		l.Remove(e)
		delete(t.keyToTaskElement, taskElement.key)
		e = next
	}
}

// getPosAndCycle maps executeAt to a slot position and the number of full
// wheel rotations after which it becomes due. It must only be called from the
// run goroutine, the sole owner of curSlot.
func (t *TimeWheel) getPosAndCycle(executeAt time.Time) (int, int) {
	delay := max(time.Until(executeAt),
		// Tasks scheduled in the past or within the next tick run on the
		// next tick. Clamping also keeps the slot index non-negative.
		t.interval)
	wheelDuration := len(t.slots) * int(t.interval)
	cycle := int(delay) / wheelDuration
	pos := (t.curSlot + int(delay)/int(t.interval)) % len(t.slots)
	return pos, cycle
}

func (t *TimeWheel) addTask(task *taskElement) {
	pos, cycle := t.getPosAndCycle(task.executeAt)
	task.pos = pos
	task.cycle = cycle

	// A task re-added under the same key replaces the pending one.
	t.removeTask(task.key)

	slot := t.slots[pos]
	t.keyToTaskElement[task.key] = slot.PushBack(task)
}

func (t *TimeWheel) removeTask(key string) {
	eTask, ok := t.keyToTaskElement[key]
	if !ok {
		return
	}
	delete(t.keyToTaskElement, key)
	task, ok := eTask.Value.(*taskElement)
	if !ok {
		return
	}
	t.slots[task.pos].Remove(eTask)
}

func (t *TimeWheel) circularIncr() {
	t.curSlot = (t.curSlot + 1) % len(t.slots)
}
