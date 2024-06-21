package completable

import "errors"

func ThenApply[T, K any](f Future[T], fn ApplyFunc[T, K]) Future[K] {
	return thenApply(f, fn, false)
}

func ThenApplyAsync[T, K any](f Future[T], fn ApplyFunc[T, K]) Future[K] {
	return thenApply(f, fn, true)
}

func thenApply[T, K any](f Future[T], fn ApplyFunc[T, K], isAsync bool) Future[K] {
	if f == nil {
		return newKnownErrorFuture[K](errors.New("nil futures"))
	}
	if fn == nil {
		return newKnownErrorFuture[K](errors.New("nil apply fn"))
	}
	cf := newCallFuture[K](func() (K, error) {
		fret, err := f.Get()
		if err != nil {
			var k K
			return k, err
		}
		return fn(fret)
	}, isAsync)
	if f.checkAndAppend(cf) {
		return cf
	}
	fret, err := f.Get()
	if err != nil {
		return newKnownErrorFuture[K](err)
	}
	return call(func() (K, error) {
		return fn(fret)
	}, isAsync)
}
