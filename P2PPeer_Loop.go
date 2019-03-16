package epsp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func (p *P2PPeer) chanGet(ctx context.Context, retvalch chan []string, errch chan error) {
	for {
		retval, err := p.Get(ctx)
		errch <- err
		retvalch <- retval
		if err != nil {
			if err != io.EOF {
				p.Close()
			}
			break
		}
	}
}

// NetLoop は、接続済みTCP接続からデータの読み書きします
func (p *P2PPeer) NetLoop(ctx context.Context, mypeerid string, agent []string, peers func() []string, p2mpcmd func(peer *P2PPeer, ss []string) (err error)) (err error) {
	timer := time.NewTicker(5 * time.Minute)
	defer timer.Stop()
	retvalch := make(chan []string)
	errch := make(chan error)
	go p.chanGet(ctx, retvalch, errch) // get received value.

outerloop:
	for {
		select {
		case <-timer.C: // timeout
			if err = p.sendPing(); err != nil { // ピアエコー要求
				err = errors.Wrap(err, `ピアエコー要求`)
				break outerloop
			}
		case err = <-errch: // read
			retval := <-retvalch
			if err != nil {
				err = errors.Wrap(err, `行読出エラー`)
				break outerloop
			}
			if err = p.loop(retval, mypeerid, agent, peers, p2mpcmd); err != nil {
				err = errors.Wrap(err, `Loopエラー`)
				break outerloop
			}

			if p.PeerID == `` {
				if err = p.Write(`612`, `1`); err != nil { // ピアID要求
					err = errors.Wrap(err, `ピアIDTCP要求送信エラー`)
					break outerloop
				}
				logln("[ECHO] ピア" + p.GetPeerIDorIPPort() + `: ピアID要求`)
			}
		case <-ctx.Done(): // context close
			err = ctx.Err()
			p.Close()

			break outerloop
		}
	}
	return
}

// loop は、送られてきた文字列に対する処理を行います
func (p *P2PPeer) loop(retval []string, mypeerid string, agent []string, peers func() []string, p2mpcmd func(peer *P2PPeer, ss []string) (err error)) error {
	if len(retval) == 0 {
		return errors.New(`空行`)
	} else if len(retval[0]) == 1 {
		return errors.New(`経由数なし: ` + strings.Join(retval, ` `))
	} else if retval[0][0] == '5' || retval[0] == `615` || retval[0] == `635` { // relay message.
		return errors.Wrap(p2mpcmd(p, retval), `p2mpcmd`)
	} else { // Not relayed.
		return errors.Wrap(p.p2pcmd(agent, mypeerid, peers, retval), `p2pcmd`)
	}
}

func (p *P2PPeer) p2pcmd(myagent []string, mypeerid string, peers func() []string, retval []string) error {
	switch retval[0] {
	case `115`:
		return p.code115(peers())
	case "611":
		return p.code611()
	case "612":
		return p.code612(mypeerid)
	case "614":
		return p.code614(retval, myagent)
	case `631`:
		return p.code631()
	case `632`:
		return p.code632(retval, peers)
	case `634`:
		return p.code634(retval)
	case `694`: // Protocol_version_incompatible
		return p.code694()
	default:
		logln("[ERROR] ピア" + p.GetPeerIDorIPPort() + ": Recv " + strings.Join(retval, ` `))
		return nil
	}
}

func (p *P2PPeer) code115(peers []string) error {
	return p.Write(`235`, `1`, strings.Join(peers, `:`))
}

func (p *P2PPeer) code611() error {
	logln("[ECHO] ピア" + p.GetPeerIDorIPPort() + ": エコー要求受領")
	p.SetPingRecvTime()

	if err := p.Write(`631`, `1`); err != nil {
		return errors.Wrap(err, `エコー返答エラー`)
	}
	logln(`[ECHO] ピア` + p.GetPeerIDorIPPort() + `: エコー返答`)
	return nil
}

func (p *P2PPeer) code612(mypeerid string) error {
	logln("[DEBUG] ピア" + p.GetPeerIDorIPPort() + ": ピアID要求")
	if err := p.Write(`632`, `1`, mypeerid); err != nil {
		return errors.Wrap(err, `ピアID返答エラー`)
	}
	logln(`[DEBUG] ピア` + p.GetPeerIDorIPPort() + `: ピアID返答`)
	if p.GetPingTime() == nil {
		return errors.Wrap(p.sendPing(), `ピアエコー要求`) // ピアエコー要求
	}
	return nil
}

func (p *P2PPeer) code614(retval []string, myagent []string) error {
	p.Agent = strings.Split(retval[2], `:`)
	logln("[DEBUG] ピア" + p.GetPeerIDorIPPort() + ": ピアプロトコルバージョン要求: " + strings.Join(p.Agent, `:`))
	if err := p.Write(`634`, `1`, strings.Join(myagent, `:`)); err != nil {
		return errors.Wrap(err, `ピアプロトコルバージョン返答エラー`)
	}
	logln(`[DEBUG] ピア` + p.GetPeerIDorIPPort() + `: ピアプロトコルバージョン返答: ` + strings.Join(myagent, `:`))
	return nil
}

func (p *P2PPeer) code631() error {
	logln("[ECHO] ピア" + p.GetPeerIDorIPPort() + ": エコー返答受領")
	p.SetPongTime()
	return nil
}

func (p *P2PPeer) code632(retval []string, peers func() []string) error {
	logln("[INFO] ピア" + retval[2] + ": ピアID返答受領" + p.EPSPConn.IPPort)
	if p.PeerID == `` {
		for _, v := range peers() {
			if strings.Split(v, `,`)[2] == retval[2] {
				return fmt.Errorf(`ピアID重複 %s %s`, v, p.GetIPPortPeerID())
			}
		}
		p.PeerID = retval[2]
	} else {
		if p.PeerID != retval[2] {
			return errors.New(`ピアID矛盾` + retval[2])
		}
	}
	if p.GetPingTime() == nil {
		if err := p.sendPing(); err != nil { // ピアエコー要求
			return errors.Wrap(err, `ピアエコー要求`)
		}
	}
	return nil
}

func (p *P2PPeer) code634(retval []string) error {
	logln("[DEBUG] ピア" + p.GetPeerIDorIPPort() + ": ピアプロトコルバージョン受領")
	p.Agent = strings.Split(retval[2], `:`)
	return nil
}

func (p *P2PPeer) code694() error {
	return errors.New(`こちらのプロトコルバージョンが古い`)
}
