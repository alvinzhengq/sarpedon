package main

import (
	"net/http"
	"time"
	"fmt"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func serveWs(w http.ResponseWriter, r *http.Request, roomID string) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
	}

	c := &connection{send: make(chan []byte, 256), ws: ws}
	s := subscription{roomID, c, false}

	ticker := time.NewTicker(time.Second * 1)
	done := make(chan bool)

	go s.writePump()
	go s.readPump()
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !s.authed {
					ws.Close()
				} else {
					for wsC := range h.roomList[roomID] {
						if wsC.userType == s.conn.userType {
							ws.Close()
							ticker.Stop()
							done <- true

							return
						}
					}

					h.registerSocket <- s
				}
				
				ticker.Stop()
				done <- true
			}
		}
	}()
}
