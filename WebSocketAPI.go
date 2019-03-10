package epsp

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketAPI is handler for websocket api.
func (peer *Peer) WebSocketAPI(ctx context.Context, hs *http.ServeMux) {
	hs.HandleFunc(`/ws/Clients`, func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logln(`Upgrade`, err)
			return
		}

		var lastclients []byte
		ticker := time.NewTicker(3 * time.Second)
		for {
			select {
			case <-ticker.C:
				clients, err := json.Marshal(peer.Clients)
				if err != nil {
					logln(`Marshal`, err)
				}
				if !bytes.Equal(clients, lastclients) {
					if err = conn.WriteMessage(websocket.TextMessage, clients); err != nil {
						//logln(`WriteMessage`, err)
						err = conn.Close()
						if err != nil {
							logln(`Close`, err)
						}
						return
					}
					lastclients = clients

				}
			case <-ctx.Done():
				err = conn.Close()
				if err != nil {
					logln(`Close`, err)
				}
				return
			}
		}
	})

	hs.HandleFunc(`/ws/Servers`, func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logln(`Upgrade`, err)
			return
		}

		var lastservers []byte
		ticker := time.NewTicker(3 * time.Second)
		for {
			select {
			case <-ticker.C:
				servers, err := json.Marshal(peer.Servers)
				if err != nil {
					logln(`Marshal`, err)
				}
				if !bytes.Equal(servers, lastservers) {
					if err = conn.WriteMessage(websocket.TextMessage, servers); err != nil {
						//logln(`WriteMessage`, err)
						err = conn.Close()
						if err != nil {
							logln(`Close`, err)
						}
						return
					}
					lastservers = servers
				}
			case <-ctx.Done():
				err = conn.Close()
				if err != nil {
					logln(`Close`, err)
				}
				return
			}
		}
	})

	hs.HandleFunc(`/ws/PeerCountByRegion`, func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(`Upgrade`, err)
			return
		}

		var lastbs []byte
		ticker := time.NewTicker(3 * time.Second)
		for {
			select {
			case <-ticker.C:
				bs := peer.PeerCountsByRegion.GoogleChart()
				if !bytes.Equal(bs, lastbs) {
					if err = conn.WriteMessage(websocket.TextMessage, bs); err != nil {
						//log.Println(`WriteMessage`, err)
						err = conn.Close()
						if err != nil {
							logln(`Close`, err)
						}
						return
					}
					lastbs = bs
				}
			case <-ctx.Done():
				err = conn.Close()
				if err != nil {
					logln(`Close`, err)
				}
				return
			}
		}
	})
}
