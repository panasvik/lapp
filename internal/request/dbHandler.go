package request

import (
	"ImageCacheProject/internal/db"
	"ImageCacheProject/internal/util"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

type DBTopic int

const (
	userDBTopic  = DBTopic(0)
	imageDBTopic = DBTopic(0)
)

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
	inChan    chan event
	eventBus  chan event
	q         *util.LinkedQueue[event]
	isRunning atomic.Bool
	ctx       context.Context
	subs      map[DBTopic][]Subscriber
}

func (h *DBHandler) runPipe() {
	h.isRunning.Store(true)
	defer h.isRunning.Store(false)

	var nextItem *event

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

func makeImgData(path string) (*imgData, error) {
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
	imgName := filepath.Base(path)
	d := &imgData{imgName, dateInt}
	return d, err
}

// Publish TODO: should work with userID
func (h *DBHandler) Publish(s Stringer, userID int, topic DBTopic) chan error {
	e := convertToEvent(s, userID, topic)
	h.inChan <- e
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
			go h.sendToSubs(e)
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
		wg.Add(1)
		go func() {
			s.PushLimit()
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
		inChan:   make(chan event, 1),
		eventBus: make(chan event, maxChanCap),
		q:        util.NewQueue[event](),
		ctx:      ctx,
		subs:     make(map[DBTopic][]Subscriber)}

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

func convertToEvent(s Stringer, userID int, topic DBTopic) event {
	return event{userID, topic, s, make(chan error, 1)}
}

type Wrapper struct {
	// TODO: convert dataBase type to an interface for universality
	dataBase *db.ImageDB
	limit    chan struct{}
}

func (w *Wrapper) PushLimit() {
	w.limit <- struct{}{}
}

func (w *Wrapper) PullLimit() {
	<-w.limit
}

func (w *Wrapper) ProcessEvent(e event) error {
	q := e.data.toString()
	imgd, err := formImgData(q)
	if err != nil {
		return err
	}
	return w.dataBase.InsertData(imgd.path, e.userID, imgd.date)
}

func formImgData(s string) (imgData, error) {
	data := make([]string, 2)
	currWord := 0
	isCurWordOpen := false
	for _, c := range s {
		if c == '{' {
			isCurWordOpen = true
			continue
		} else if c == '}' {
			currWord++
			isCurWordOpen = false
		}
		if isCurWordOpen {
			data[currWord] = data[currWord] + string(c)
		}

	}
	path := data[0]
	date, err := strconv.Atoi(data[1])
	if err != nil {
		return imgData{"", 0}, err
	}

	return imgData{path, date}, nil
}
