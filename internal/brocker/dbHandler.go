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
	ErrConversion     = errors.New("unable to convert data type")
	ErrWrongTopic     = errors.New("send data from wrong topic")
)

type Topic int

const (
	InsertNewToken      Topic = 0
	RevokeToken         Topic = 1
	InsertNewImage      Topic = 2
	RemoveImage         Topic = 3
	AddUserToGroup      Topic = 4
	RemoveUserFromGroup Topic = 5
	AddMessage          Topic = 6
	ChangeMessageStatus Topic = 7
	SendMessage         Topic = 8
	InsertNewDevice     Topic = 9
)

type Event struct {
	Topic   Topic
	Body    any
	Forward func(data any, topic Topic)
}

type Subscriber interface {
	ProcessEvent(e Event) util.Issue
	PushLimit()
	PullLimit()
}

type Handler struct {
	subMu     sync.RWMutex
	inChan    chan Event
	eventBus  chan Event
	q         *util.LinkedQueue[Event]
	isRunning atomic.Bool
	ctx       context.Context
	subs      map[Topic][]Subscriber
	errChan   chan<- util.Issue
}

func (h *Handler) runPipe() {
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

func (h *Handler) Publish(body any, topic Topic) {
	e := h.convertToEvent(body, topic)
	h.inChan <- e
}

func (h *Handler) sender() {
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

func (h *Handler) sendToSubs(e Event) {
	topic := e.Topic
	h.subMu.RLock()
	var wg sync.WaitGroup
	func() {
		defer h.subMu.RUnlock()
		topicSubs := h.subs[topic]
		if topicSubs == nil {
			return
		}
		for _, s := range topicSubs {
			wg.Add(1)
			go func() {
				s.PushLimit()
				defer func() { wg.Done(); s.PullLimit() }()
				res := s.ProcessEvent(e)
				if res != nil && res.GetErr() != nil {
					h.errChan <- res
				}
			}()
		}
	}()
	wg.Wait()
}

func NewDBHandler(ctx context.Context, errChan chan<- util.Issue) *Handler {
	return &Handler{
		inChan:   make(chan Event, maxChanCap),
		eventBus: make(chan Event, maxChanCap),
		q:        util.NewQueue[Event](),
		ctx:      ctx,
		subs:     make(map[Topic][]Subscriber),
		errChan:  errChan}

}

func (h *Handler) InitDBHandler() {
	go h.runPipe()
	go h.sender()
}

func (h *Handler) Subscribe(topic Topic, s Subscriber) {
	h.subMu.Lock()
	defer h.subMu.Unlock()
	topicSubs := h.subs[topic]
	topicSubs = append(topicSubs, s)
	h.subs[topic] = topicSubs
}

func (h *Handler) convertToEvent(s any, topic Topic) Event {
	return Event{topic, s, h.Publish}
}
