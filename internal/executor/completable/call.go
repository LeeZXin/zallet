package completable

import "errors"

func Call[T any](c CallFunc[T]) Future[T] {
	return call(c, false)
}

func CallAsync[T any](c CallFunc[T]) Future[T] {
	return call(c, true)
}

func call[T any](c CallFunc[T], isAsync bool) Future[T] {
	if c == nil {
		return newKnownErrorFuture[T](errors.New("nil futures"))
	}
	f := newCallFuture[T](c, isAsync)
	f.fire()
	return f
}
