package completable

import (
	"errors"
	"sort"
	"sync"
	"time"
)

const (
	syncPriority = iota
	asyncPriority
	errPriority
)

var (
	timeoutError = errors.New("task timeout")
)

type CallFunc[T any] func() (T, error)
type ApplyFunc[T, K any] func(T) (K, error)
type CombineFunc[T, K, L any] func(T, K) (L, error)

type IBase interface {
	priority() int
	fire()
	joinAndGet() (any, error)
	checkAndAppend(i IBase) bool
}

type Future[T any] interface {
	IBase
	Get() (T, error)
	GetWithTimeout(timeout time.Duration) (T, error)
}

// futureResult promise err
type futureResult[T any] struct {
	Result T
	Err    error
}

type callFuture[T any] struct {
	sync.Mutex
	result   *futureResult[T]
	done     chan struct{}
	callFunc CallFunc[T]
	isAsync  bool
	arr      []IBase
}

func newCallFuture[T any](c CallFunc[T], isAsync bool) *callFuture[T] {
	return &callFuture[T]{
		Mutex:    sync.Mutex{},
		done:     make(chan struct{}, 1),
		callFunc: c,
		isAsync:  isAsync,
		arr:      make([]IBase, 0),
	}
}

func (c *callFuture[T]) priority() int {
	if c.isAsync {
		return asyncPriority
	}
	return syncPriority
}

func (c *callFuture[T]) fire() {
	if c.isAsync {
		go c.run()
	} else {
		c.run()
	}
}

func (c *callFuture[T]) run() {
	res, err := c.callFunc()
	c.setResult(&futureResult[T]{
		Result: res,
		Err:    err,
	})
	c.completed()
}

func (c *callFuture[T]) joinAndGet() (any, error) {
	return c.Get()
}

func (c *callFuture[T]) setResult(result *futureResult[T]) bool {
	c.Lock()
	defer c.Unlock()
	if c.result != nil {
		return false
	}
	c.result = result
	return true
}

func (c *callFuture[T]) completed() {
	close(c.done)
	c.Lock()
	arr := c.arr[:]
	c.Unlock()
	for _, i := range arr {
		i.fire()
	}
}

func (c *callFuture[T]) getFutureResult() *futureResult[T] {
	c.Lock()
	defer c.Unlock()
	return c.result
}

func (c *callFuture[T]) checkAndAppend(b IBase) bool {
	if b == nil {
		return false
	}
	c.Lock()
	defer c.Unlock()
	if c.result != nil {
		return false
	}
	c.arr = append(c.arr, b)
	// 异步任务优先执行
	sort.SliceStable(c.arr, func(i, j int) bool {
		return c.arr[i].priority() > c.arr[j].priority()
	})
	return true
}

// Get 阻塞获取结果 无限期等待
func (c *callFuture[T]) Get() (T, error) {
	return c.GetWithTimeout(0)
}

// GetWithTimeout 带超时返回结果 超时返回timeoutErr
func (c *callFuture[T]) GetWithTimeout(timeout time.Duration) (T, error) {
	val := c.getFutureResult()
	if val == nil {
		if timeout > 0 {
			timer := time.NewTimer(timeout)
			defer timer.Stop()
			select {
			case <-c.done:
				break
			case <-timer.C:
				var t T
				return t, timeoutError
			}
		} else {
			select {
			case <-c.done:
				break
			}
		}
		val = c.getFutureResult()
	}
	return val.Result, val.Err
}

type knownErrorFuture[T any] struct {
	Result T
	Err    error
}

func newKnownErrorFuture[T any](err error) Future[T] {
	return &knownErrorFuture[T]{
		Err: err,
	}
}

func (f *knownErrorFuture[T]) priority() int {
	return errPriority
}

func (f *knownErrorFuture[T]) fire() {}

func (f *knownErrorFuture[T]) joinAndGet() (any, error) {
	return f.Result, f.Err
}

func (f *knownErrorFuture[T]) Get() (T, error) {
	return f.Result, f.Err
}

func (f *knownErrorFuture[T]) GetWithTimeout(_ time.Duration) (T, error) {
	return f.Result, f.Err
}

func (f *knownErrorFuture[T]) checkAndAppend(_ IBase) bool {
	return false
}
