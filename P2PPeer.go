package epsp

import (
	"strings"

	"github.com/pkg/errors"
)

// P2PPeer は、ピア接続のためのクラスです
type P2PPeer struct {
	PeerID string
	EPSPConn
}

// GetPeerIDorIPPort は、PeerID、IPアドレス、ポートを取得します
func (p *P2PPeer) GetPeerIDorIPPort() string {
	if p != nil {
		if p.PeerID != `` {
			return p.PeerID
		}
		return p.EPSPConn.IPPort
	}
	return `NO PEER`
}

// GetPeerID は、PeerIDを取得します
func (p *P2PPeer) GetPeerID() string {
	if p != nil {
		return p.PeerID
	}
	return ``
}

// WriteTo は、peeridへssを送信します
func (p *P2PPeer) WriteTo(ss ...string) (err error) {
	if p == nil {
		return errors.New(`送信先ピアがありません`)
	}
	if p.IsConn() {
		if err = p.Write(ss...); err == nil {
			logln("[DEBUG] ピア" + p.GetPeerIDorIPPort() + `: 送出 `)
		} else {
			err = errors.Wrap(err, `送出不可`)
		}
	} else {
		err = errors.Wrap(err, `未接続`)
	}
	return
}

// GetIPPortPeerID は、IP,Port,PeerID 形式の文字列を返します
func (p *P2PPeer) GetIPPortPeerID() string {
	boundipport := strings.LastIndex(p.IPPort, `:`) // need to consider ipv6
	return p.IPPort[:boundipport] + `,` + p.IPPort[boundipport+1:] + `,` + p.GetPeerID()
}

// StringAgent は、エージェント名の文字列を返します
func (p P2PPeer) StringAgent() string {
	return strings.Join(p.Agent, `:`)
}

func (p *P2PPeer) sendPing() error {
	if err := p.Write(`611`, `1`); err != nil { // ピアエコー要求
		return errors.Wrap(err, `ピアエコー要求送信不能`)
	}
	logln("[ECHO] ピア" + p.GetPeerIDorIPPort() + `: ピアエコー要求`)
	p.SetPingTime()
	return nil
}
