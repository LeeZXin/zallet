package completable

import "errors"

func ThenCombine[T, K, L any](t Future[T], k Future[K], c CombineFunc[T, K, L]) Future[L] {
	return thenCombine(t, k, c, false)
}

func ThenCombineAsync[T, K, L any](t Future[T], k Future[K], c CombineFunc[T, K, L]) Future[L] {
	return thenCombine(t, k, c, true)
}

func thenCombine[T, K, L any](t Future[T], k Future[K], c CombineFunc[T, K, L], isAsync bool) Future[L] {
	if t == nil || k == nil {
		return newKnownErrorFuture[L](errors.New("nil futures"))
	}
	if c == nil {
		return newKnownErrorFuture[L](errors.New("nil combine func"))
	}
	return thenApply(ThenAllOf(t, k), func(any) (L, error) {
		tret, _ := t.Get()
		kret, _ := k.Get()
		return c(tret, kret)
	}, isAsync)
}
