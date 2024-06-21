package completable

import (
	"errors"
	"sort"
	"sync"
	"time"
)

type anyOfFuture struct {
	sync.Mutex
	result     any
	err        error
	done       bool
	doneChan   chan struct{}
	notifyChan chan IBase
	arr        []IBase
	bases      []IBase
}

func newAnyOfFuture(bases ...IBase) *anyOfFuture {
	return &anyOfFuture{
		Mutex:      sync.Mutex{},
		doneChan:   make(chan struct{}, 1),
		notifyChan: make(chan IBase, len(bases)),
		arr:        make([]IBase, 0),
		bases:      bases,
	}
}

func (f *anyOfFuture) checkAndAppend(i IBase) bool {
	if i == nil {
		return false
	}
	f.Lock()
	defer f.Unlock()
	if f.done {
		return false
	}
	f.arr = append(f.arr, i)
	// 异步任务优先执行
	sort.SliceStable(f.arr, func(i, j int) bool {
		return f.arr[i].priority() > f.arr[j].priority()
	})
	return true
}

func (f *anyOfFuture) notify(b IBase) {
	if b == nil {
		return
	}
	f.Lock()
	defer f.Unlock()
	if !f.done {
		f.notifyChan <- b
	}
}

func (f *anyOfFuture) Get() (any, error) {
	return f.GetWithTimeout(0)
}

func (f *anyOfFuture) joinAndGet() (any, error) {
	return f.Get()
}

func (f *anyOfFuture) getResultAndErr() (any, error) {
	f.Lock()
	defer f.Unlock()
	return f.result, f.err
}

func (f *anyOfFuture) setResultAndErr(result any, err error) {
	f.Lock()
	defer f.Unlock()
	f.result = result
	f.err = err
	f.done = true
}

func (f *anyOfFuture) GetWithTimeout(timeout time.Duration) (any, error) {
	if f.isDone() {
		return f.getResultAndErr()
	}
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil, timeoutError
		case <-f.doneChan:
			return f.getResultAndErr()
		}
	} else {
		select {
		case <-f.doneChan:
			return f.getResultAndErr()
		}
	}
}

func (f *anyOfFuture) fire() {
	defer f.postComplete()
	if len(f.bases) == 0 {
		return
	}
	for {
		select {
		case b, ok := <-f.notifyChan:
			if !ok {
				return
			}
			result, err := b.joinAndGet()
			f.setResultAndErr(result, err)
			return
		case <-f.doneChan:
			return
		}
	}
}

func (f *anyOfFuture) priority() int {
	return syncPriority
}

func (f *anyOfFuture) isDone() bool {
	f.Lock()
	defer f.Unlock()
	return f.done
}

func (f *anyOfFuture) postComplete() {
	close(f.doneChan)
	close(f.notifyChan)
	f.Lock()
	f.done = true
	arr := f.arr[:]
	f.Unlock()
	for _, i := range arr {
		i.fire()
	}
}

func ThenAnyOf(bases ...IBase) Future[any] {
	return thenAnyOf(false, bases...)
}

func ThenAnyOfAsync(bs ...IBase) Future[any] {
	return thenAnyOf(true, bs...)
}

func thenAnyOf(isAsync bool, bs ...IBase) Future[any] {
	if len(bs) == 0 {
		return newKnownErrorFuture[any](errors.New("nil futures"))
	}
	f := newAnyOfFuture(bs...)
	for _, i := range bs {
		if !i.checkAndAppend(&relay{
			f: f,
			b: i,
		}) {
			f.notify(i)
		}
	}
	if isAsync {
		go f.fire()
	} else {
		f.fire()
	}
	return f
}
