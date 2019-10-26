package pubsub

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
)

// FakeMessage is a simple pubsub message
type FakeMessage struct {
	peerID peer.ID
	data   []byte
}

// GetFrom returns the message's sender ID
func (m *FakeMessage) GetFrom() peer.ID {
	return m.peerID
}

// GetData returns the message's payload
func (m *FakeMessage) GetData() []byte {
	return m.data
}

// FakeSubscription is a fake pubsub subscription.
type FakeSubscription struct {
	topic       string
	pending     chan Message
	err         error
	cancelled   bool
	awaitCancel sync.WaitGroup
}

// NewFakeSubscription builds a new fake subscription to a topic.
func NewFakeSubscription(topic string, bufSize int) *FakeSubscription {
	sub := &FakeSubscription{
		topic:       topic,
		pending:     make(chan Message, bufSize),
		awaitCancel: sync.WaitGroup{},
	}
	sub.awaitCancel.Add(1)
	return sub
}

// Subscription interface

// Topic returns this subscription's topic.
func (s *FakeSubscription) Topic() string {
	return s.topic
}

// Next returns the next messages from this subscription.
func (s *FakeSubscription) Next(ctx context.Context) (Message, error) {
	if s.err != nil {
		return nil, s.err
	}
	select {
	case msg := <-s.pending:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Cancel cancels this subscription, after which no subsequently posted messages will be received.
func (s *FakeSubscription) Cancel() {
	if s.cancelled {
		panic("subscription already cancelled")
	}
	s.cancelled = true
	s.awaitCancel.Done()
}

// Manipulators

// Post posts a new message to this subscription.
func (s *FakeSubscription) Post(msg Message) {
	if s.err != nil {
		panic("subscription has failed")
	}
	if !s.cancelled {
		s.pending <- msg
	}
}

// Fail causes subsequent reads from this subscription to fail.
func (s *FakeSubscription) Fail(err error) {
	if err != nil {
		panic("error is nil")
	}
	if !s.cancelled {
		s.err = err
	}
}

// AwaitCancellation waits for the subscription to be canceled by the subscriber.
func (s *FakeSubscription) AwaitCancellation() {
	s.awaitCancel.Wait()
}
