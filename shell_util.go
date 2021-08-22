package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type connection struct {
	ws       *websocket.Conn
	send     chan []byte
	userType string
}

type subscription struct {
	room   string
	conn   *connection
	authed bool
}

type message struct {
	data   []byte
	roomID string
	sender *connection
}

type socketHub struct {
	broadcastMessage chan message
	registerSocket   chan subscription
	unregisterSocket chan subscription
	roomList         map[string]map[*connection]bool
}

func (c *connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return c.ws.WriteMessage(mt, payload)
}

func (s *subscription) writePump() {
	c := s.conn
	ticker := time.NewTicker(10 * time.Second)

	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.write(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (s *subscription) readPump() {
	c := s.conn

	defer func() {
		h.unregisterSocket <- *s
		c.ws.Close()
	}()

	c.ws.SetReadLimit(512)
	c.ws.SetReadDeadline(time.Now().Add(15 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(15 * time.Second))
		return nil
	})

	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			return
		}

		strMsg := string(msg)
		if s.authed {
			m := message{msg, s.room, c}
			h.broadcastMessage <- m
		} else {
			fmt.Println("CONN: " + strMsg)
			if strings.Contains(strMsg, "READER") {
				s.authed = true
				s.conn.userType = "reader"
				fmt.Println(strMsg)
			} else if strings.Contains(strMsg, "WRITER") {
				s.authed = true
				s.conn.userType = "writer"
			}
		}
	}
}

var h = socketHub{
	broadcastMessage: make(chan message),
	registerSocket:   make(chan subscription),
	unregisterSocket: make(chan subscription),
	roomList:         make(map[string]map[*connection]bool),
}

func (h *socketHub) run() {
	for {
		select {

		case s := <-h.registerSocket:
			connectionList := h.roomList[s.room]
			if connectionList == nil {
				connectionList = make(map[*connection]bool)
				h.roomList[s.room] = connectionList
			} else if len(connectionList) == 2 {
				s.conn.send <- []byte("Room Full")
				close(s.conn.send)
				s.conn.ws.Close()
			}

			h.roomList[s.room][s.conn] = true

		case s := <-h.unregisterSocket:
			connectionList := h.roomList[s.room]
			if connectionList != nil {
				if _, ok := connectionList[s.conn]; ok {
					delete(connectionList, s.conn)
					close(s.conn.send)

					if len(connectionList) == 0 {
						delete(h.roomList, s.room)
					}
				}
			}

		case m := <-h.broadcastMessage:
			connectionList := h.roomList[m.roomID]
			for c := range connectionList {
				if c == m.sender {
					continue
				}

				select {
				case c.send <- m.data:
				default:
					close(c.send)
					delete(connectionList, c)

					if len(connectionList) == 0 {
						delete(h.roomList, m.roomID)
					}
				}
			}

		}
	}
}
