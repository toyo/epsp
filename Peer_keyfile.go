package epsp

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

type keyFile struct {
	PeerID            string
	SecKey            string
	PubKey            string
	Expire            time.Time
	KeySig            string
	Global            bool
	PeerCountByRegion PeerCounts
	Peers             []string
}

// SaveKey は、キーをセーブするメソッドです。
func (peer *Peer) SaveKey() {
	var k keyFile
	k.Expire = peer.keyExpire
	k.KeySig = peer.keySig
	k.PeerID = peer.PeerID
	k.PubKey = peer.pubKey
	k.SecKey = peer.secKey
	k.Global = peer.Global
	k.PeerCountByRegion = peer.PeerCountsByRegion

	maxRxUniq := uint64(0)
	for i := range peer.Clients {
		rxUniq := peer.Clients[i].RxUniq
		if maxRxUniq < rxUniq {
			maxRxUniq = rxUniq
		}
	}

	for i := range peer.Clients {
		if maxRxUniq < 10 || peer.Clients[i].RxUniq <= maxRxUniq/10 { // maxRxUniq < 10 or save only useful peers.
			k.Peers = append(k.Peers, strings.Replace(peer.Clients[i].IPPort, `:`, `,`, 1)+`,`+peer.Clients[i].PeerID)
		}
	}

	keyfile, err := os.Create(peer.keyfilename)
	if err != nil {
		logln(`[WARN] SaveKey OpenFile` + err.Error())
		return
	}
	if err = json.NewEncoder(keyfile).Encode(k); err != nil {
		logln(`[WARN] SaveKey Encode` + err.Error())
		return
	}
}

// LoadKey は、キーをロードするメソッドです。
func (peer *Peer) LoadKey() (peers []string, err error) {
	var k keyFile
	keyfile, err := os.Open(peer.keyfilename)
	if err != nil {
		return nil, err
	}
	if err = json.NewDecoder(keyfile).Decode(&k); err != nil {
		return nil, err
	}
	logln(`[INFO] ノード: 証明書有効期限`, k.Expire)
	peer.keyExpire = k.Expire
	peer.keySig = k.KeySig
	peer.PeerID = k.PeerID
	peer.pubKey = k.PubKey
	peer.secKey = k.SecKey
	peer.Global = k.Global
	peer.PeerCountsByRegion = k.PeerCountByRegion
	return k.Peers, nil
}
