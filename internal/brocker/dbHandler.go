package brocker

import (
	"ImageCacheProject/internal/util"
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

const (
	maxChanCap         = 100
	startQueuePipeMark = 15
	maxSenders         = 4
)

var (
	ErrUnknownDBTopic = errors.New("unknown topic")
	ErrNoDataInEvent  = errors.New("event has no Stringer data")
)

type DBTopic int

func (d *DBTopic) GetIntValue() int {
	return int(*d)
}

const (
	InsertNewToken = DBTopic(0)
	RevokeToken    = DBTopic(1)
	InsertNewImage = DBTopic(2)
	RemoveImage    = DBTopic(3)
)

type Event struct {
	topic DBTopic
	data  any
}

type Subscriber interface {
	ProcessEvent(e Event) util.Issue
	PushLimit()
	PullLimit()
}

type DBHandler struct {
	inChan    chan Event
	eventBus  chan Event
	q         *util.LinkedQueue[Event]
	isRunning atomic.Bool
	ctx       context.Context
	subs      map[DBTopic][]Subscriber
	errChan   chan<- util.Issue
}

func (h *DBHandler) runPipe() {
	h.isRunning.Store(true)
	defer h.isRunning.Store(false)

	var nextItem *Event

	for {
		if nextItem == nil {
			nextItem = h.q.Pop()
		}

		if nextItem == nil {
			select {
			case <-h.ctx.Done():
				return
			case val := <-h.inChan:
				h.q.Push(val)
			}
		} else {
			select {
			case <-h.ctx.Done():
				return
			case val := <-h.inChan:
				h.q.Push(val)
			case h.eventBus <- *nextItem:
				nextItem = nil
			}
		}
	}
}

func (h *DBHandler) Publish(s any, topic DBTopic) {
	e := convertToEvent(s, topic)
	h.inChan <- e
}

func (h *DBHandler) sender() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case e, ok := <-h.eventBus:
			if !ok {
				return
			}
			go h.sendToSubs(e)
		}
	}
}

func (h *DBHandler) sendToSubs(e Event) {
	topic := e.topic
	topicSubs := h.subs[topic]
	if topicSubs == nil {
		return
	}
	var wg sync.WaitGroup
	for _, s := range topicSubs {
		wg.Add(1)
		go func() {
			s.PushLimit()
			defer func() { wg.Done(); s.PullLimit() }()
			h.errChan <- s.ProcessEvent(e)
		}()
	}
	wg.Wait()
}

func NewDBHandler(ctx context.Context, errChan chan<- util.Issue) *DBHandler {
	return &DBHandler{
		inChan:   make(chan Event, maxChanCap),
		eventBus: make(chan Event, maxChanCap),
		q:        util.NewQueue[Event](),
		ctx:      ctx,
		subs:     make(map[DBTopic][]Subscriber),
		errChan:  errChan}

}

func (h *DBHandler) InitDBHandler() {
	go h.runPipe()
	go h.sender()
}

func (h *DBHandler) Subscribe(topic DBTopic, s Subscriber) {
	topicSubs := h.subs[topic]
	topicSubs = append(topicSubs, s)
	h.subs[topic] = topicSubs
}

func convertToEvent(s any, topic DBTopic) Event {
	return Event{topic, s}
}

func (e *Event) GetTopic() DBTopic {
	return e.topic
}

func (e *Event) GetData() any {
	return e.data
}
