package ws

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
)

var ErrTooManyConnections = errors.New("too many websocket connections for user")

type Broadcaster interface {
	PublishToUser(userID int64, event models.Event) error
	PublishGlobal(event models.Event) error
}

type registration struct {
	client *Client
	result chan error
}

type Hub struct {
	logger                *slog.Logger
	mu                    sync.RWMutex
	register              chan registration
	unregister            chan *Client
	users                 map[int64]map[*Client]struct{}
	maxConnectionsPerUser int
}

func NewHub(logger *slog.Logger, maxConnectionsPerUser int) *Hub {
	return &Hub{
		logger:                logger,
		register:              make(chan registration),
		unregister:            make(chan *Client),
		users:                 make(map[int64]map[*Client]struct{}),
		maxConnectionsPerUser: maxConnectionsPerUser,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case req := <-h.register:
			h.mu.Lock()
			clients := h.users[req.client.userID]
			if len(clients) >= h.maxConnectionsPerUser {
				h.mu.Unlock()
				req.result <- ErrTooManyConnections
				close(req.result)
				continue
			}
			if clients == nil {
				clients = make(map[*Client]struct{})
				h.users[req.client.userID] = clients
			}
			clients[req.client] = struct{}{}
			h.mu.Unlock()
			req.result <- nil
			close(req.result)
		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.users[client.userID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					client.shutdown()
				}
				if len(clients) == 0 {
					delete(h.users, client.userID)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Register(client *Client) error {
	result := make(chan error, 1)
	h.register <- registration{client: client, result: result}
	return <-result
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) PublishToUser(userID int64, event models.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h.mu.RLock()
	clients := make([]*Client, 0, len(h.users[userID]))
	for client := range h.users[userID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- payload:
		case <-client.done:
		default:
			client.shutdown()
			h.Unregister(client)
		}
	}
	return nil
}

func (h *Hub) PublishGlobal(event models.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h.mu.RLock()
	clients := make([]*Client, 0)
	for _, group := range h.users {
		for client := range group {
			clients = append(clients, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- payload:
		case <-client.done:
		default:
			client.shutdown()
			h.Unregister(client)
		}
	}
	return nil
}
