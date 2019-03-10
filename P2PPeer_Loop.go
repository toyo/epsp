package epsp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// NetLoop は、接続済みTCP接続からデータの読み書きします
func (p *P2PPeer) NetLoop(ctx context.Context, agent []string, peers func() []string, p2mpcmd func(peer *P2PPeer, ss []string) (err error)) (err error) {
	timer := time.NewTicker(5 * time.Minute)
	defer timer.Stop()
	retvalch := make(chan []string)
	errch := make(chan error)
	go func() {
		for {
			retval, err := p.Get(ctx)
			errch <- err
			retvalch <- retval
			if err != nil {
				break
			}
		}
	}()

outerloop:
	for {
		select {
		case <-timer.C: // timeout
			if err = p.sendPing(); err != nil { // ピアエコー要求
				if err != io.EOF {
					p.Close()
				}
				err = errors.Wrap(err, `ピアエコー要求`)
				break outerloop
			}
		case err = <-errch: // read
			retval := <-retvalch
			if err != nil {
				if err != io.EOF {
					p.Close()
				}
				err = errors.Wrap(err, `行読出エラー`)
				break outerloop
			}
			if err = p.Loop(retval, agent, peers, p2mpcmd); err != nil {
				if err != io.EOF {
					p.Close()
				}
				err = errors.Wrap(err, `Loopエラー`)
				break outerloop
			}

			if p.PeerID == `` {
				if err = p.Write(`612`, `1`); err != nil { // ピアID要求
					if err != io.EOF {
						p.Close()
					}
					err = errors.Wrap(err, `ピアIDTCP要求送信エラー`)
					break outerloop
				}
				logln("[ECHO] ピア" + p.GetPeerIDorIPPort() + `: ピアID要求`)
			}
		case <-ctx.Done():
			err = ctx.Err()
			p.Close()

			break outerloop
		}
	}
	return
}

// Loop は、送られてきた文字列に対する処理を行います
func (p *P2PPeer) Loop(retval []string, agent []string, peers func() []string, p2mpcmd func(peer *P2PPeer, ss []string) (err error)) (err error) {
	if len(retval) == 0 {
		err = errors.New(`空行`)
		return
	} else if len(retval[0]) != 3 {
		err = errors.New(`書式エラー: ` + strings.Join(retval, ` `))
	} else if retval[0] == `115` {
		err = p.Write(`235`, `1`, strings.Join(peers(), `:`))
	} else if retval[0][0] == '5' || retval[0] == `615` || retval[0] == `635` || retval[0] == `115` { // Message relayed.
		err = errors.Wrap(p2mpcmd(p, retval), `code5xx`)
	} else if retval[0][0] == '6' { // Not relayed.
		err = errors.Wrap(p.p2pcmd(agent, p, peers, retval), `code6xx`)
	} else {
		err = errors.New(`Unknown ` + strings.Join(retval, ` `))
	}
	return
}

func (p *P2PPeer) p2pcmd(myagent []string, mypeerid *P2PPeer, peers func() []string, retval []string) (err error) {
	switch retval[0] {
	case "611":
		logln("[ECHO] ピア" + p.GetPeerIDorIPPort() + ": エコー要求受領")
		p.SetPingRecvTime()

		err = p.Write(`631`, `1`)
		if err != nil {
			err = errors.Wrap(err, `エコー返答エラー`)
			return
		}
		logln(`[ECHO] ピア` + p.GetPeerIDorIPPort() + `: エコー返答`)
	case "612":
		logln("[DEBUG] ピア" + p.GetPeerIDorIPPort() + ": ピアID要求")
		err = p.Write(`632`, `1`, mypeerid.GetPeerID())
		if err != nil {
			err = errors.Wrap(err, `ピアID返答エラー`)
			return
		}
		logln(`[DEBUG] ピア` + p.GetPeerIDorIPPort() + `: ピアID返答`)
		if p.GetPingTime() == nil {
			if err = p.sendPing(); err != nil { // ピアエコー要求
				err = errors.Wrap(err, `ピアエコー要求`)
				return
			}
		}
	case "614":
		p.Agent = strings.Split(retval[2], `:`)
		logln("[DEBUG] ピア" + p.GetPeerIDorIPPort() + ": ピアプロトコルバージョン要求: " + strings.Join(p.Agent, `:`))
		err = p.Write(`634`, `1`, strings.Join(myagent, `:`))
		if err != nil {
			err = errors.Wrap(err, `ピアプロトコルバージョン返答エラー`)
			return
		}
		logln(`[DEBUG] ピア` + p.GetPeerIDorIPPort() + `: ピアプロトコルバージョン返答: ` + strings.Join(myagent, `:`))
	case `631`:
		logln("[ECHO] ピア" + p.GetPeerIDorIPPort() + ": エコー返答受領")
		p.SetPongTime()
	case `632`:
		logln("[INFO] ピア" + retval[2] + ": ピアID返答受領" + p.EPSPConn.IPPort)
		if p.PeerID == `` {
			for _, v := range peers() {
				if strings.Split(v, `,`)[2] == retval[2] {
					err = fmt.Errorf(`ピアID重複 %s %s`, v, p.GetIPPortPeerID())
					return
				}
			}
			p.PeerID = retval[2]
		} else {
			if p.PeerID != retval[2] {
				err = errors.New(`ピアID矛盾` + retval[2])
				return
			}
		}
		if p.GetPingTime() == nil {
			if err = p.sendPing(); err != nil { // ピアエコー要求
				err = errors.Wrap(err, `ピアエコー要求`)
				return
			}
		}
	case `634`:
		logln("[DEBUG] ピア" + p.GetPeerIDorIPPort() + ": ピアプロトコルバージョン受領")
		p.Agent = strings.Split(retval[2], `:`)
	case `694`: // Protocol_version_incompatible
		err = errors.New(`こちらのプロトコルバージョンが古い`)
		return
	default:
		logln("[DEBUG] ピア" + p.GetPeerIDorIPPort() + ": Recv " + strings.Join(retval, ` `))
	}
	err = nil
	return
}
