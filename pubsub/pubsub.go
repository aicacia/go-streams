package pubsub

import "sync"

const subscriber_channel_size = 1000

type Subscriber[T any] struct {
	pubsub *PubSub[T]
	C      chan *T
}

func (s *Subscriber[T]) Close() {
	s.pubsub.Close(s)
}

type PubSub[T any] struct {
	mutex    sync.RWMutex
	channels []*Subscriber[T]
}

func NewPubSub[T any]() PubSub[T] {
	return PubSub[T]{
		mutex: sync.RWMutex{},
	}
}

func (ps *PubSub[T]) Publish(value *T) {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()
	for _, channel := range ps.channels {
		if len(channel.C) < cap(channel.C) {
			channel.C <- value
		}
	}
}

func (ps *PubSub[T]) Subscribe() *Subscriber[T] {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	subscriber := Subscriber[T]{
		pubsub: ps,
		C:      make(chan *T, subscriber_channel_size),
	}
	ps.channels = append(ps.channels, &subscriber)
	return &subscriber
}

func (ps *PubSub[T]) Close(s *Subscriber[T]) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	close(s.C)
	ps.channels = findAndDelete(ps.channels, s)
}

func findAndDelete[T any](slice []T, item T) []T {
	index := 0
	for _, value := range slice {
		if &value != &item {
			slice[index] = value
			index++
		}
	}
	return slice[:index]
}
