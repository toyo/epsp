package epsp

import (
	"context"
	"net"
	traditionalnet "net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// P2PPeers is array of *P2PPeer
type P2PPeers []*P2PPeer

// NewP2PServers は、P2PServerを立ち上げます
func (pps *P2PPeers) NewP2PServers(ctx context.Context, mypeerid string, myagent []string, port int, codep2mp func(from *P2PPeer, retval []string) error, ConnectedIPPortPeersList func() []string, incoming uint64) (global bool, err error) {

	laddr, err := traditionalnet.ResolveTCPAddr("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		err = errors.Wrap(err, "ResolveTCPAddr")
		return
	}
	laddr.IP = nil

	l, err := net.ListenTCP(`tcp`, laddr)
	if err != nil {
		err = errors.Wrap(err, "ListenTCP")
		return
	}

	logln(`[DEBUG] LitenTCP: `, laddr, len(laddr.IP), strings.Join(myagent, `:`))

	pschan := make(chan *P2PPeer)
	go func(l *traditionalnet.TCPListener) {
		for {
			if ps, err := NewP2PServer(ctx, l, myagent); err != nil {
				logln(`[WARN] `, err)
			} else {
				pschan <- ps
			}
		}
	}(l)

	go func() {
		var mu sync.Mutex
		timer := time.NewTicker(1 * time.Minute)
		for {
			select {
			case ps := <-pschan:
				go func() {
					mu.Lock()
					*pps = append(*pps, ps)
					mu.Unlock()
					err = ps.NetLoop(ctx, mypeerid, myagent, ConnectedIPPortPeersList, codep2mp)
					if err != nil {
						logln(`[INFO] ピア`, ps.PeerID+`: サーバ通信異常終了 `+strings.Join(ps.Agent, `:`), err)
					} else {
						logln(`[INFO] ピア`, ps.PeerID+`: サーバ通信正常終了 `+strings.Join(ps.Agent, `:`))
					}
				}()
			case <-timer.C:
				mu.Lock()
				pps.deleteClosedFromList()
				pps.deleteManyDuplicatePeer(incoming, 100)
				mu.Unlock()
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	}()

	global = len(laddr.IP) != 0
	err = nil
	return
}

// AddP2PClients は、P2PClientsを追加します
func (pps *P2PPeers) AddP2PClients(ctx context.Context, mypeerid string, otherPeers []string, myagent []string, codep2mp func(from *P2PPeer, retval []string) error, ConnectedIPPortPeersList func() []string, incoming uint64) {

	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := range otherPeers {
		wg.Add(1)
		go func(i int) {
			pc, err := NewP2PClient(ctx, otherPeers[i], ConnectedIPPortPeersList)
			if err != nil {
				logln(`[INFO] ピア`+pc.GetPeerIDorIPPort()+`: 接続失敗 `, err)
				wg.Done()
			} else {
				mu.Lock()
				*pps = append(*pps, pc)
				mu.Unlock()
				wg.Done()
				err = pc.NetLoop(ctx, mypeerid, myagent, ConnectedIPPortPeersList, codep2mp)
				if err != nil {
					logln(`[INFO] ピア`, pc.PeerID+`: クライアント通信異常終了 `+strings.Join(pc.Agent, `:`), err)
				} else {
					logln(`[INFO] ピア`, pc.PeerID+`: クライアント通信正常終了 `+strings.Join(pc.Agent, `:`))
				}
			}
		}(i)
	}
	wg.Wait()
	pps.deleteClosedFromList()
	pps.deleteManyDuplicatePeer(incoming, 10)

}

func (pps *P2PPeers) deleteClosedFromList() {
	// Delete connection after 3min.
	for i := 0; i < len(*pps); i++ {
		if !(*pps)[i].IsConn() && time.Since(*(*pps)[i].GetDiscTime()) > 1*time.Minute {
			*pps = append((*pps)[:i], (*pps)[i+1:]...)
			i--
		}
	}
}

func (pps *P2PPeers) deleteManyDuplicatePeer(incoming, rxdup uint64) {
	// Close connection to the peers who send many duplicate
	for i := range *pps {
		if (*pps)[i].RxDup > rxdup && (*pps)[i].IsConn() {
			if (*pps)[i].GetRXUniqRate() > incoming/2 {
				(*pps)[i].Close()
				logln(`[INFO] ピア` + (*pps)[i].PeerID + `: 重複過多、終了`)
			}
		}
	}
}

// NumOfConnectedPeers は、接続中ピアの数を返します
func (pps *P2PPeers) NumOfConnectedPeers() (n uint64) {
	for i := range *pps {
		if (*pps)[i].IsConn() {
			n++
		}
	}
	return n
}

// ConnectedPeersList は、接続中ピアのピアIDのリストを返します
func (pps *P2PPeers) ConnectedPeersList() (ss []string) {
	for i := range *pps {
		if (*pps)[i].IsConn() && (*pps)[i].PeerID != `` {
			ss = append(ss, (*pps)[i].PeerID)
		}
	}
	return
}

// ConnectedIPPortPeersList は、接続中ピアのIPアドレス,ポート,ピアIDのリストを返します
func (pps *P2PPeers) ConnectedIPPortPeersList() (ss []string) {
	for i := range *pps {
		if (*pps)[i].IsConn() {
			ss = append(ss, (*pps)[i].GetIPPortPeerID())
		}
	}
	return
}
