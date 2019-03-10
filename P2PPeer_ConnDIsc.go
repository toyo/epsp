package epsp

import (
	"context"
	traditionalnet "net"
	"strings"
	"time"

	"github.com/pkg/errors"
	net "github.com/toyo/go-net"
)

// NewP2PServer は、ピアからの接続を待ちます
func NewP2PServer(ctx context.Context, l *traditionalnet.TCPListener, myagent []string, code5xx func(p *P2PPeer, ss []string) (err error)) (ps *P2PPeer, err error) {
	ps = new(P2PPeer)
	ps.conn, err = l.AcceptTCP()
	if err != nil {
		if ne, ok := err.(traditionalnet.Error); ok {
			if ne.Temporary() {
				err = errors.Wrap(err, "AcceptTCP")
				return
			}
			err = errors.Wrap(err, "AcceptTCP0")
			return
		}
		err = errors.Wrap(err, "AcceptTCP1")
		return
	}

	ps.IPPort = ps.conn.RemoteAddr().String()
	logln(`[INFO] ピア` + ps.IPPort + `: TCP接続受理`)
	ps.EPSPConn.SetConnTime()

	if err = ps.Write(`614`, `1`, strings.Join(myagent, `:`)); err != nil { // バージョン要求
		ps.Close()
		return
	}
	logln("[DEBUG] ピア" + ps.IPPort + `: バージョン要求` + ` ` + strings.Join(myagent, `:`))

	return
}

// NewP2PClient は、他のピアと接続します。
func NewP2PClient(ctx context.Context, ipportpeerid string, connectedIPPortPeersList func() []string, code5xx func(from *P2PPeer, ss []string) (err error)) (pc *P2PPeer, err error) {
	ipportpeerids := strings.Split(ipportpeerid, `,`)

	for _, connedctedipportpeerid := range connectedIPPortPeersList() {
		connedctedipportpeerids := strings.Split(connedctedipportpeerid, `,`)
		if ipportpeerids[2] == connedctedipportpeerids[2] {
			err = errors.New(`PeerID重複: ` + ipportpeerid + ` == ` + connedctedipportpeerid)
			return
		}
	}

	pc = new(P2PPeer)
	pc.IPPort = ipportpeerids[0] + `:` + ipportpeerids[1]
	pc.PeerID = ipportpeerids[2]

	ctxtimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pc.EPSPConn.conn, err = net.DialContext(ctxtimeout, `tcp`, pc.EPSPConn.IPPort)
	if err != nil {
		err = errors.Wrap(err, `TCP接続不可エラー`)
		pc.Close()
		return
	}
	logln("[INFO] ピア", pc.PeerID, ": TCP接続完了 ", pc.EPSPConn.IPPort)
	pc.SetConnTime()

	return
}

// Close は、ピア接続を終了します
func (p *P2PPeer) Close() {
	if p != nil {
		p.EPSPConn.Close()
	}
}
