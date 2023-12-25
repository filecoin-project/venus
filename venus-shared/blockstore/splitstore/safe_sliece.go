package splitstore

import "sync"

type SafeSlice[T any] struct {
	data []T
	mux  sync.RWMutex
}

func (s *SafeSlice[T]) Append(v ...T) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.data = append(s.data, v...)
}

func (s *SafeSlice[T]) At(idx int) T {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.data[idx]
}

func (s *SafeSlice[T]) Len() int {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return len(s.data)
}

func (s *SafeSlice[T]) Prototype() []T {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.data
}

func (s *SafeSlice[T]) First() T {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.data[0]
}

func (s *SafeSlice[T]) Last() T {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.data[len(s.data)-1]
}

func (s *SafeSlice[T]) Delete(idx int) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.data = append(s.data[:idx], s.data[idx+1:]...)
}

func (s *SafeSlice[T]) Slice(start, end int) *SafeSlice[T] {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return &SafeSlice[T]{
		data: s.data[start:end],
	}
}

// Range calls f sequentially for each element present in the slice.
// If f returns false, range stops the iteration.
// Should not modify the slice during the iteration. (modify the element is ok)
func (s *SafeSlice[T]) ForEach(f func(int, T) bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	for i, v := range s.data {
		if !f(i, v) {
			return
		}
	}
}

func NewSafeSlice[T any](data []T) *SafeSlice[T] {
	return &SafeSlice[T]{
		data: data,
	}
}

type InfinityChannel[T any] struct {
	data []T
	in   chan T
	out  chan T
}

func NewInfinityChannel[T any]() (in chan<- T, out <-chan T) {

	ch := &InfinityChannel[T]{
		data: make([]T, 0),
		in:   make(chan T),
		out:  make(chan T),
	}

	notClosed := true

	go func() {
		for {
			if notClosed {
				var v T
				if len(ch.data) > 0 {
					select {
					case v, notClosed = <-ch.in:
						if notClosed {
							ch.data = append(ch.data, v)
						}
					case ch.out <- ch.data[0]:
						ch.data = ch.data[1:]
					}
				} else {
					v, notClosed = <-ch.in
					ch.data = append(ch.data, v)
				}
			} else {
				for _, v := range ch.data {
					ch.out <- v
				}
				close(ch.out)
				break
			}
		}
	}()

	return ch.in, ch.out
}

func Map[T, R any](data []T, f func(T) R) []R {
	result := make([]R, len(data))
	for i, v := range data {
		result[i] = f(v)
	}
	return result
}
