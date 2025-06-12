package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

var (
	// pongWait is the duration for how long we will await a pong from a client before dropping the connection.
	pongWait = 10 * time.Second

	// pingInterval is how often we will send pings to the client. It needs to always be smaller than the pongWait.
	pingInterval = (pongWait * 9) / 10
)

type ClientList map[*Client]bool

type Client struct {
	connection *websocket.Conn
	manager    *Manager

	// egress is used to avoid concurrent writes on the websocket connection
	egress chan Event
}

func NewClient(conn *websocket.Conn, manager *Manager) *Client {
	return &Client{
		connection: conn,
		manager:    manager,
		egress:     make(chan Event),
	}
}

func (c *Client) readMessages() {
	defer func() {
		// cleanup connection
		c.manager.removeClient(c)
	}()

	// SetReadDeadline from the gorilla package allows us to set a time for how long we should wait
	if err := c.connection.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Println(err)
		return
	}

	// SetPongHandler applies a certain handler when a pong is triggered.
	c.connection.SetPongHandler(c.pongHandler)

	for {
		_, payload, err := c.connection.ReadMessage()

		if err != nil {
			// CloseGoingAway and CloseAbnormalClosure are checks to ensure the err is not triggered by client or server.
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}

		var request Event

		if err := json.Unmarshal(payload, &request); err != nil {
			log.Printf("error marshalling event :%v", err)
			break
		}

		if err := c.manager.routeEvent(request, c); err != nil {
			log.Println("error handling message: ", err)
		}
	}
}

// Websocket has a few events
// ClosedEvent
// ErrorEvent
// MessageEvent
// OpenEvent

func (c *Client) writeMessages() {
	defer func() {
		c.manager.removeClient(c)
	}()

	ticker := time.NewTicker(pingInterval)

	for {
		select {
		case message, ok := <-c.egress:
			if !ok {
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					log.Println("connection closed: ", err)
				}
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				log.Println(err)
				return
			}

			if err := c.connection.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("failed to send message: %v", err)
			}
			log.Println("message sent")
		// This case is for when we receive a tick from the ticker.
		case <-ticker.C:
			log.Println("ping")

			// Send a Ping to the Client
			if err := c.connection.WriteMessage(websocket.PingMessage, []byte(``)); err != nil {
				log.Println("writemsg err: ", err)
				return
			}
		}
	}
}

func (c *Client) pongHandler(pongMsg string) error {
	log.Println("pong")
	// We have to reset the timeline so as to enable subsequent pings and pongs
	return c.connection.SetReadDeadline(time.Now().Add(pongWait))
}
