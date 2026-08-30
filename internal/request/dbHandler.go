package request

import (
	"ImageCacheProject/internal/db"
	"ImageCacheProject/internal/util"
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

const (
	maxChanCap         = 100
	startQueuePipeMark = 15
	maxSenders         = 4
	maxDBreq           = 10
)

const (
	userDBTopic  = DBTopic(0)
	imageDBTopic = DBTopic(0)
)

type DBTopic int

type Stringer interface {
	toString() string
}

type event struct {
	userID int
	topic  DBTopic
	data   Stringer
	ret    chan error
}
type imgData struct {
	path string
	date int
}

type userData struct {
	password   string
	actionTime int
}

type Subscriber interface {
	ProcessEvent(e event) error
	PushLimit()
	PullLimit()
}

type DBHandler struct {
	eventBus      chan event
	q             *util.Queue[event]
	isPipeRunning atomic.Bool
	ctx           context.Context
	subs          map[DBTopic][]Subscriber
	senderLimit   chan struct{}
}

func (h *DBHandler) pushData(e event) {
	if len(h.eventBus) == maxChanCap {
		h.q.Push(e)
		if h.startPipeCondition() {
			go h.runPipe()
		}
	}
	h.eventBus <- e
}

func (h *DBHandler) startPipeCondition() bool {
	return !h.isPipeRunning.Load() && h.q.GetSize() >= startQueuePipeMark
}

func (h *DBHandler) runPipe() {
	ctx, _ := context.WithTimeout(h.ctx, 10*time.Second)
	h.isPipeRunning.Store(true)
	defer h.isPipeRunning.Store(false)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			for val := h.q.Pop(); val != nil; {
				h.eventBus <- *val
			}
		}
	}

}

func getImgData(path string) (*imgData, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	x, err := exif.Decode(file)
	var tm time.Time
	if err == nil {
		tm, err = x.DateTime()
	}
	if err != nil {
		tm = time.Now()
	}

	dateStr := tm.Format("20060102")
	dateInt, err := strconv.Atoi(dateStr)
	if err != nil {
		return nil, fmt.Errorf("error converting date: %w", err)
	}
	d := &imgData{path, dateInt}
	return d, err
}

// TODO: should work with userID
func (h *DBHandler) Publish(s Stringer, userID int, topic DBTopic) chan error {
	e := convertToEvent(s, userID, topic)
	h.pushData(e)
	return e.ret
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
			h.senderLimit <- struct{}{}
			go func() {
				defer func() { <-h.senderLimit }()
				h.sendToSubs(e)
			}()
		}
	}
}

func (h *DBHandler) sendToSubs(e event) {
	topic := e.topic
	topicSubs := h.subs[topic]
	if topicSubs == nil {
		return
	}
	var wg sync.WaitGroup
	for _, s := range topicSubs {
		s.PushLimit()
		wg.Add(1)
		go func() {
			defer func() { wg.Done(); s.PullLimit() }()
			err := s.ProcessEvent(e)
			e.ret <- err
		}()
	}
	wg.Wait()
	close(e.ret)
}

func (u *userData) toString() string {
	return "{" + u.password + "} {" + strconv.Itoa(u.actionTime) + "}"
}

func (i *imgData) toString() string {
	return "{" + i.path + "} {" + strconv.Itoa(i.date) + "}"
}

func NewDBHandler(ctx context.Context) *DBHandler {
	return &DBHandler{
		eventBus:    make(chan event, maxChanCap),
		q:           util.NewQueue[event](),
		ctx:         ctx,
		subs:        make(map[DBTopic][]Subscriber),
		senderLimit: make(chan struct{}, maxSenders)}

}

func (h *DBHandler) InitDBHandler() {
	go h.sender()
}

func (h *DBHandler) Subscribe(topic DBTopic, s Subscriber) {
	topicSubs := h.subs[topic]
	topicSubs = append(topicSubs, s)
	h.subs[topic] = topicSubs
}

func convertToEvent(s Stringer, userID int, topic DBTopic) event {
	return event{userID, topic, s, make(chan error, 1)}
}

type Wrapper struct {
	dataBase *db.ImageDB
	limit    chan struct{}
}

func (w *Wrapper) PushLimit() {
	w.limit <- struct{}{}
}

func (w *Wrapper) PullLimit() {
	<-w.limit
}

// TODO: implement ProcessEvent
func (w *Wrapper) ProcessEvent(e event) error {
	return nil
}
