package completable

import (
	"errors"
)

type notifier interface {
	notify(base IBase)
}

type relay struct {
	f notifier
	b IBase
}

func (r *relay) fire() {
	r.f.notify(r.b)
}

func (r *relay) priority() int {
	return syncPriority
}

func (r *relay) checkAndAppend(IBase) bool {
	return false
}

func (r *relay) joinAndGet() (any, error) {
	return nil, nil
}

type allOfFuture struct {
	*anyOfFuture
}

func (f *allOfFuture) fire() {
	defer f.postComplete()
	if len(f.bases) == 0 {
		return
	}
	count := 0
	for {
		select {
		case b, ok := <-f.notifyChan:
			if !ok {
				return
			}
			_, err := b.joinAndGet()
			if err != nil {
				f.setResultAndErr(nil, err)
				return
			}
			count += 1
			if count == len(f.bases) {
				return
			}
		case <-f.doneChan:
			return
		}
	}
}

func ThenAllOf(bases ...IBase) Future[any] {
	return thenAllOf(false, bases...)
}

func ThenAllOfAsync(bases ...IBase) Future[any] {
	return thenAllOf(true, bases...)
}

func thenAllOf(isAsync bool, bases ...IBase) Future[any] {
	if len(bases) == 0 {
		return newKnownErrorFuture[any](errors.New("nil futures"))
	}
	f := &allOfFuture{
		anyOfFuture: newAnyOfFuture(bases...),
	}
	for _, i := range bases {
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
