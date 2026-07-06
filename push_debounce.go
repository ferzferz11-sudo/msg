package main

import (
	"fmt"
	"sync"
	"time"
)

type PushDebouncer struct {
	mu      sync.Mutex
	pending map[string]*pendingPush
	sendFn  func(targets []pushTarget, title, body, roomID string)
}

type pendingPush struct {
	sender   string
	senderID string
	targets  []pushTarget
	messages []string
	timer    *time.Timer
}

func NewPushDebouncer(sendFn func([]pushTarget, string, string, string)) *PushDebouncer {
	return &PushDebouncer{
		pending: make(map[string]*pendingPush),
		sendFn:  sendFn,
	}
}

func (d *PushDebouncer) Enqueue(targets []pushTarget, sender, senderID, text, roomID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if existing, ok := d.pending[roomID]; ok {
		existing.messages = append(existing.messages, text)
		existing.timer.Reset(3 * time.Second)
		return
	}

	pp := &pendingPush{
		sender:   sender,
		senderID: senderID,
		targets:  targets,
		messages: []string{text},
	}
	pp.timer = time.AfterFunc(3*time.Second, func() {
		d.flush(roomID)
	})
	d.pending[roomID] = pp
}

func (d *PushDebouncer) flush(roomID string) {
	d.mu.Lock()
	pp, ok := d.pending[roomID]
	if !ok {
		d.mu.Unlock()
		return
	}
	delete(d.pending, roomID)
	d.mu.Unlock()

	var body string
	switch len(pp.messages) {
	case 1:
		body = pp.messages[0]
	case 2, 3:
		body = pp.messages[0]
		for _, m := range pp.messages[1:] {
			body += "\n" + m
		}
	default:
		body = pp.messages[0]
		for _, m := range pp.messages[1:3] {
			body += "\n" + m
		}
		body += fmt.Sprintf("\n...и %d сообщений", len(pp.messages)-3)
	}

	d.sendFn(pp.targets, pp.sender, body, roomID)
}
