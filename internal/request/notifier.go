package request

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/SherClockHolmes/webpush-go"
)

var (
	ErrFullSSEChan   = errors.New("users sse chan is full")
	ErrDeprecatedSub = errors.New("webpush subscription is deprecated")
)
var (
	vapidPublicKey  = os.Getenv("VAPID_PUBLIC_KEY")
	vapidPrivateKey = os.Getenv("VAPID_PRIVATE_KEY")
	vapidSubscriber = "https://pcloudcom.tech/vapid_messages"
)

type SubscribeRequest struct {
	Subscription webpush.Subscription `json:"subscription"`
}

type ClientDevices map[string]chan util.Message

type Notifier struct {
	webPdb     WebPushDB
	mu         sync.RWMutex
	sseClients map[int]ClientDevices
}

func (n *Notifier) handleSubscription(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := n.webPdb.AddSub(userID, req.Subscription)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[Push] registered new sub for userID: %d", userID)
	w.WriteHeader(http.StatusCreated)
}

func (n *Notifier) handleStreamConnect(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	/*
	 deviceID is just an identifier
	 if the access token on which deviceID is based expires,
	 it will not affect the device identification, because here it is not
	 used for authentication but for differing connected user devices.
	 It is because the connection does not close when the user access token
	 expires and is fulfilling the purpose of dynamic sort-term connection
	*/
	deviceID, ok := DeviceIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	msgChan := make(chan util.Message, 10)
	n.mu.Lock()
	clientDevices, ok := n.sseClients[userID]
	if !ok {
		clientDevices = make(ClientDevices)
	}
	clientDevices[deviceID] = msgChan
	n.sseClients[userID] = clientDevices
	n.mu.Unlock()

	defer func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		delete(n.sseClients[userID], deviceID)
		if len(n.sseClients[userID]) == 0 {
			delete(n.sseClients, userID)
		}
		close(msgChan)
		fmt.Println("user " + strconv.Itoa(userID) + "closed connection")
	}()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	clientDone := r.Context().Done()

	for {
		select {
		case <-clientDone:
			return

		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()

		case msg, ok := <-msgChan:
			if !ok {
				return
			}
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (n *Notifier) ProcessEvent(e brocker.Event) util.Issue {
	datablob := e.Body
	switch e.Topic {
	case brocker.SendMessage:
		{
			data, ok := datablob.(util.Message)
			if !ok {
				return &util.ErrIssue{brocker.ErrConversion, "unable to convert to util.Message"}
			}
			err := n.sendMessage(data)
			if err != nil {
				return &util.ErrIssue{err, "unable to send message"}
			}
			data.Status = util.Delivered
			e.Forward(data, brocker.ChangeMessageStatus)
		}
	default:
		return &util.ErrIssue{
			Err: brocker.ErrWrongTopic,
			Desc: fmt.Sprintf("sent topic: %d, expected %d or %d",
				e.Topic, brocker.AddUserToGroup, brocker.RemoveUserFromGroup)}
	}
	return nil
}

func (n *Notifier) PushLimit() {}
func (n *Notifier) PullLimit() {}

func (n *Notifier) sendMessage(m util.Message) error {
	target, isOnline := n.sseClients[m.RecipientID]
	var err error
	if !isOnline {
		fmt.Println("sending webpush to " + strconv.Itoa(m.RecipientID))
		sendErr := n.sendWebPushes(m)
		if sendErr != nil {
			err = sendErr
		}
	} else {
		for _, targetChan := range target {
			select {
			case targetChan <- m:
				fmt.Println("message was sent")
			default:
				fmt.Println("unable to send message, chan is full")
				err = ErrFullSSEChan
			}
		}
	}
	return err
}

func (n *Notifier) sendWebPushes(m util.Message) error {
	subs, err := n.webPdb.GetSubs(m.RecipientID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	for _, sub := range subs {
		res := n.sendWebPush(payload, sub)
		if res != nil {
			err = res
		}
	}
	return err
}

func (n *Notifier) sendWebPush(payload []byte, sub webpush.Subscription) error {
	resp, err := webpush.SendNotification(payload, &sub, &webpush.Options{
		Subscriber:      vapidSubscriber,
		VAPIDPublicKey:  vapidPublicKey,
		VAPIDPrivateKey: vapidPrivateKey,
		TTL:             86400,
	})
	if err != nil {
		fmt.Printf("[Push] error calling webpush: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		fmt.Printf("[Push] deprecated subscription, deleting")
		return ErrDeprecatedSub
	}
	return nil
}

func NewNotifier(db WebPushDB) *Notifier {
	return &Notifier{
		webPdb:     db,
		sseClients: make(map[int]ClientDevices),
	}
}
