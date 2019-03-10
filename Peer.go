package epsp

import (
	"crypto/rsa"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// Peer はピアに関するデータを保持します。
type Peer struct {
	PeerID             string
	BootTime           time.Time
	ProtocolTimeDiff   time.Duration
	MyAgent            []string
	EPSPServer         *P2SClient
	Clients            P2PPeers
	Servers            P2PPeers
	hosts              []string
	region             string
	incoming           uint64
	serverKey          *rsa.PublicKey
	peerKey            *rsa.PublicKey
	keyfilename        string
	secKey             string
	pubKey             string
	keyExpire          time.Time
	keySig             string
	PeerCountsByRegion PeerCounts
	sigmap             sync.Map
	traceecho          sync.Map
	serverIsRunning    sync.Once
	serverErrorCount   uint16
	candidatePeers     []string
	Global             bool

	usercmd func(code string, retval ...string)
}

// NewPeer は、Peerのコンストラクタです
func NewPeer(hosts []string, region string, incoming uint64, serverKey, peerKey []byte, usercmd func(code string, retval ...string)) (*Peer, error) {

	peer := new(Peer)
	peer.MyAgent = []string{`0.34r`, `toyokun_at_gmail_dot_com`, `Alpha0000`}
	peer.hosts = hosts
	peer.usercmd = usercmd
	peer.region = region
	peer.incoming = incoming
	peer.Clients = make([]*P2PPeer, 0)
	peer.Servers = make([]*P2PPeer, 0)

	if len(hosts) == 0 {
		return nil, errors.New(`No hosts`)
	}

	peer.keyfilename = os.TempDir() + `/` + strings.Replace(hosts[0], `:`, `P`, -1) + `.json`

	var err error
	if peer.serverKey, err = DecryptKey(serverKey); err != nil {
		return nil, errors.Wrap(err, `サーバ公開鍵`)
	}

	if peer.peerKey, err = DecryptKey(peerKey); err != nil {
		return nil, errors.Wrap(err, `ピア公開鍵`)
	}

	peer.candidatePeers, err = peer.LoadKey()
	if err != nil {
		logln(`[WARN] LoadKey ` + err.Error())
	}
	peer.BootTime = time.Now()

	return peer, nil

}

// NumOfConnectedPeers は、接続中ピアの数を返します
func (peer *Peer) NumOfConnectedPeers() (n uint64) {
	return peer.Clients.NumOfConnectedPeers() + peer.Servers.NumOfConnectedPeers()
}

// WriteExceptFrom は、from以外へssを送信します
func (peer *Peer) WriteExceptFrom(from *P2PPeer, ss ...string) {
	for i := range peer.Clients {
		if from != nil && peer.Clients[i].GetPeerID() == from.GetPeerID() {
			logln("[DEBUG] ピア" + peer.Clients[i].GetPeerID() + `: 送出しない `)
		} else if peer.Clients[i].IsConn() {
			if err := peer.Clients[i].Write(ss...); err == nil {
				logln("[DEBUG] ピア" + peer.Clients[i].GetPeerID() + `: 送出 `)
			} else {
				logln("[WARN] ピア" + peer.Clients[i].GetPeerID() + `: 送出不可 ` + err.Error())
			}
		} else {
			logln("[DEBUG] ピア" + peer.Clients[i].GetPeerID() + `: 未接続`)
		}
	}
	for i := range peer.Servers {
		if from != nil && peer.Servers[i].GetPeerID() == from.GetPeerID() {
			logln("[DEBUG] ピア" + peer.Servers[i].GetPeerID() + `: 送出しない `)
		} else if peer.Servers[i].IsConn() {
			if err := peer.Servers[i].Write(ss...); err == nil {
				logln("[DEBUG] ピア" + peer.Servers[i].GetPeerID() + `: 送出 `)
			} else {
				logln("[WARN] ピア" + peer.Servers[i].GetPeerID() + `: 送出不可 ` + err.Error())
			}
		} else {
			logln("[DEBUG] ピア" + peer.Servers[i].GetPeerID() + `: 未接続`)
		}
	}
}

// PeerIDToP2PPeer は、peeridに対応するP2PPeerを返します。対応するP2PPeerがなければnilを返します。
func (peer *Peer) PeerIDToP2PPeer(peerid string) *P2PPeer {
	for i := range peer.Clients {
		if peer.Clients[i].PeerID == peerid {
			return peer.Clients[i]
		}
	}
	for i := range peer.Servers {
		if peer.Servers[i].PeerID == peerid {
			return peer.Servers[i]
		}
	}
	return nil
}

// ConnectedPeersList は、接続中ピアのリストを返します
func (peer *Peer) ConnectedPeersList() (ss []string) {
	ss = append(ss, peer.Clients.ConnectedPeersList()...)
	ss = append(ss, peer.Servers.ConnectedPeersList()...)
	return
}

// ConnectedIPPortPeersList は、接続中ピアのIPアドレス,ポート,ピアIDのリストを返します
func (peer *Peer) ConnectedIPPortPeersList() (ss []string) {
	ss = append(ss, peer.Clients.ConnectedIPPortPeersList()...)
	ss = append(ss, peer.Servers.ConnectedIPPortPeersList()...)
	return
}
