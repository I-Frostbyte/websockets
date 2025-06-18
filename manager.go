package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/gorilla/websocket"
)

var (
	websocketUpgrader = websocket.Upgrader{
		CheckOrigin: checkOrigin,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

type Manager struct {
	clients ClientList
	sync.RWMutex

	otps RetentionMap

	handlers map[string]EventHandler
}

func NewManager(ctx context.Context) *Manager {
	m := &Manager{
		clients: make(ClientList),
		handlers: make(map[string]EventHandler),
		otps: NewRetentionMap(ctx, 5*time.Second),
	}

	m.setupEventHandlers()
	return m
}

func (m *Manager) setupEventHandlers() {
	m.handlers[EventSendMessage] = SendMessage
}

func SendMessage(event Event, c *Client) error {
	fmt.Println(event)
	return nil
}

func (m *Manager) routeEvent(event Event, c *Client) error {
	// Check if the event type is part of the handler
	if handler, ok := m.handlers[event.Type]; ok {
		if err := handler(event, c); err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("there is no such event type")
	}
}

func (m *Manager) serveWS(w http.ResponseWriter, r *http.Request) {
	// Verifying the otp before connection.
	otp := r.URL.Query().Get("otp")
	if otp == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !m.otps.VerifyOTP(otp) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	log.Println("new connection")

	// upgrade regular http connection into websocket
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := NewClient(conn, m)

	m.addClient(client)

	// Start client processes
	go client.readMessages()
	go client.writeMessages()
}

// loginHandler takes the requests to the login endpoint and processes them.
// This is where the user is authenticated, the otp is created and the key is sent back.
// The key in the header shall then serve to grant the user continous access to the websocket.
func (m *Manager) loginHandler(responseWriter http.ResponseWriter, httpRequest *http.Request) {
	type userLoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req userLoginRequest

	if err := json.NewDecoder(httpRequest.Body).Decode(&req); err != nil {
		http.Error(responseWriter, err.Error(), http.StatusBadRequest)
		return
	}

	// hardcoded authentication, can be replaced later with proper db interaction.
	if req.Username == "gilbert" && req.Password == "insider" {
		type response struct {
			OTP string `json:"otp"`
		}

		otp := m.otps.NewOTP()

		resp := response{
			OTP: otp.Key,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			log.Println(err)
			return
		}

		responseWriter.WriteHeader(http.StatusOK)
		responseWriter.Write(data)
		return

	}

	responseWriter.WriteHeader(http.StatusUnauthorized)
}

func (m *Manager) addClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	m.clients[client] = true
}

func (m *Manager) removeClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.clients[client]; ok {
		client.connection.Close()
		delete(m.clients, client)
	}
}

// checkOrigin is built to prevent Cross Site Request Forgery (CRSF).
// It will take an http.Request and return a bool.
func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	switch origin {
	case "http://localhost:9001":
		return true
	default:
		return false
	}
}