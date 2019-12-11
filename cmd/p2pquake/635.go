package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type peer635 struct {
	Hops            uint64
	ConnectedPeerID []string
}

// Handler635 は、調査エコーの相手を保持します
type Handler635 struct {
	p635  map[string]peer635
	mutex *sync.RWMutex
}

// NewHandler635 は、Handler635 のコンストラクタです
func NewHandler635() (h *Handler635) {
	h = new(Handler635)
	h.clean635()
	h.mutex = new(sync.RWMutex)
	return
}

func (h *Handler635) clean635() {
	h.p635 = make(map[string]peer635)
}

func (h Handler635) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type node struct {
		Label string `json:"label"`
	}
	type link struct {
		Source   string `json:"source"`
		Target   string `json:"target"`
		Distance uint64 `json:"distance"`
	}

	var retjson struct {
		Nodes []node
		Links []link
	}

	h.mutex.RLock()
	nodes := make(map[string]uint64)
	for id, v := range h.p635 {
		nodes[id] = v.Hops
		for _, pid := range v.ConnectedPeerID {
			if n, ok := nodes[pid]; !ok || n > v.Hops {
				nodes[pid] = v.Hops + 1
			}
		}
	}
	for i := range nodes {
		retjson.Nodes = append(retjson.Nodes, node{Label: i})
	}

	for id, v := range h.p635 {
		for _, peerid := range v.ConnectedPeerID {
			retjson.Links = append(retjson.Links, link{Source: id, Target: peerid, Distance: 1})
		}
	}
	h.mutex.RUnlock()

	err := json.NewEncoder(w).Encode(&retjson)
	_ = err
}

func (h *Handler635) cmd635(recvdata ...string) {
	log.Println("635 " + strings.Join(recvdata, `:`))
	reporterpeerid := recvdata[2]
	connectedpeerids := strings.Split(recvdata[3], `,`)
	hops, err := strconv.ParseUint(recvdata[4], 10, 64)
	if err != nil {
		return
	}
	h.mutex.Lock()
	h.p635[reporterpeerid] = peer635{
		Hops:            hops,
		ConnectedPeerID: connectedpeerids,
	}
	h.mutex.Unlock()
}
