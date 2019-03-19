package epsp

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/toyo/go-net"
)

// P2SClient は、サーバに対するクライアントです
type P2SClient struct {
	EPSPConn
}

// GetPeerID は、相手のPeerIDを返すものですが、サーバにはPeerIDがないので、IPとポートを返します。
func (p2s P2SClient) GetPeerID() string {
	return p2s.IPPort
}

func (p2s *P2SClient) code211(myagent []string) error {
	logln(`[DEBUG] サーバ` + p2s.EPSPConn.IPPort + `: バージョン要求`)
	if err := p2s.EPSPConn.Write(`131`, `1`, strings.Join(myagent, `:`)); err != nil {
		return errors.Wrap(err, `バージョン要求不能`)
	}
	logln(`[DEBUG] サーバ` + p2s.EPSPConn.IPPort + `: バージョン返信 ` + strings.Join([]string{`131`, `1`, strings.Join(myagent, `:`)}, ` `))
	return nil
}

func (p2s *P2SClient) code212(myagent, retval []string) (myagent0 []string) {
	logln(`[DEBUG] サーバ` + p2s.EPSPConn.IPPort + `: バージョン受領 ` + retval[2])

	p2s.EPSPConn.Agent = strings.Split(retval[2], `:`)
	if myagent[0] > p2s.EPSPConn.Agent[0] {
		myagent[0] = p2s.EPSPConn.Agent[0]
		logln(`[DEBUG] エージェント名変更: ` + strings.Join(myagent, `:`))
	}
	return myagent
}

// NewP2SClient は、サーバ接続用のクライアントです
func NewP2SClient(ctx context.Context, paddr string, myagent0 []string) (p2s *P2SClient, myagent []string, err error) {
	myagent = myagent0
	p2s = new(P2SClient)
	p2s.EPSPConn.IPPort = paddr
	p2s.EPSPConn.conn, err = net.DialContext(ctx, `tcp`, p2s.EPSPConn.IPPort)
	if err != nil {
		err = errors.Wrap(err, `DialContext`)
		p2s = nil
		return
	}
	logln(`[INFO] サーバ` + p2s.EPSPConn.IPPort + `: 接続`)
	p2s.EPSPConn.SetConnTime()

outerloop:
	for {
		var rv string
		if rv, err = p2s.EPSPConn.Get(ctx); err != nil { // バージョン要求がこないよ
			p2s.Close(ctx)
			err = errors.Wrap(err, `バージョン要求なし: `+rv)
			p2s = nil
			return
		}
		retval := strings.SplitN(rv, ` `, 3)
		switch retval[0] {
		case `211`: // バージョン要求
			if err = p2s.code211(myagent); err != nil {
				p2s.Close(ctx)
				p2s = nil
			}
		case `212`: // バージョン受領
			myagent = p2s.code212(myagent, retval)
			break outerloop
		default:
			p2s.Close(ctx)
			err = errors.Wrap(err, `コマンド受領エラー `+strings.Join(retval, ` `))
			p2s = nil
			return
		}
	}
	return
}

// GetTemporaryPeerID は、サーバから暫定ピアIDを取得します
func (p2s P2SClient) GetTemporaryPeerID(ctx context.Context) (peerID string, err error) {
	logln(`[DEBUG] サーバ` + p2s.IPPort + `: ピアID暫定割当要求`)
	if err = p2s.EPSPConn.Write(`113`, `1`); err != nil {
		err = errors.Wrap(err, `ピアID暫定割当要求`)
		return
	}

	var rv string
	if rv, err = p2s.Get(ctx); err != nil {
		err = errors.Wrap(err, "ピアID暫定割当受信")
		return
	}
	retval := strings.SplitN(rv, ` `, 3)

	switch retval[0] {
	case `233`:
		peerID = retval[2]
		logln(`[DEBUG] サーバ` + p2s.IPPort + `: ピアID暫定割当: ` + peerID)
		err = nil
		return
	default:
		err = errors.New(`ピアID暫定割当がこないよ`)
		return
	}
}

// GetPeers は、サーバから接続可能なピア情報を取得します
func (p2s P2SClient) GetPeers(ctx context.Context, peerID string) (peers []string, err error) {
	err = p2s.EPSPConn.Write(`115`, `1`, peerID)
	if err != nil {
		err = errors.Wrap(err, `接続先ピア情報要求不能`)
		return
	}
	logln(`[DEBUG] サーバ` + p2s.IPPort + `: 接続先ピア情報要求`)

	rv, err := p2s.Get(ctx)
	if err != nil {
		err = errors.Wrap(err, "接続先ピア情報要求受信")
		return
	}

	retval := strings.SplitN(rv, ` `, 3)
	switch retval[0] {
	case `235`:
		peers = strings.Split(retval[2], `:`)
		logln(`[DEBUG] サーバ`+p2s.IPPort+`: 接続先ピア情報の取得:`, len(peers), `peers`)
		return
	default:
		err = errors.New(`接続先ピア情報がこないよ: ` + retval[0])
		return
	}
}

// Regist は、ピアIDの本割り当てを要求します
func (p2s P2SClient) Regist(ctx context.Context, peerID string, port int, region string, numofpeers uint64, incoming uint64) (err error) {
	err = p2s.EPSPConn.Write(`116`, `1`, peerID+`:`+strconv.Itoa(port)+`:`+region+`:`+strconv.FormatUint(numofpeers, 10)+`:`+strconv.FormatUint(incoming, 10))
	if err != nil {
		err = errors.Wrap(err, `ピアID本割当要求不能`)
		return
	}
	logln(`[DEBUG] サーバ` + p2s.IPPort + `: ピアID本割当要求`)

	rv, err := p2s.Get(ctx)
	if err != nil {
		err = errors.Wrap(err, `ピアID本割当受信`)
		return
	}

	retval := strings.SplitN(rv, ` `, 3)
	switch retval[0] {
	case `236`:
		logln(`[DEBUG] サーバ`+p2s.IPPort+`: ピアID本割当完了 参加ピア数: `, retval[2])
		return
	default:
		err = errors.New(`ピアID本割当できないよ: ` + retval[0])
		return
	}
}

// GetKey は、キーを取得します
func (p2s P2SClient) GetKey(ctx context.Context, peer *Peer, echo bool) (err error) {

	if time.Now().After(peer.keyExpire.Add(-30 * time.Minute)) {
		if echo {
			if err = p2s.EPSPConn.Write(`124`, `1`, peer.PeerID+`+`+peer.secKey); err != nil { // 鍵の再割り当てを要求します。
				err = errors.Wrap(err, `鍵再割当要求不能`)
				return
			}
			logln(`[DEBUG] サーバ` + p2s.IPPort + `: 鍵再割当要求`)
		} else {
			if err = p2s.EPSPConn.Write(`117`, `1`, peer.PeerID); err != nil { // 鍵の割り当てを要求します。
				err = errors.Wrap(err, `鍵割当要求不能`)
				return
			}
			logln(`[DEBUG] サーバ` + p2s.IPPort + `: 鍵割当要求`)
		}

		var rv string
		if rv, err = p2s.Get(ctx); err != nil {
			err = errors.Wrap(err, "鍵割当要求応答受信")
			return
		}

		retval := strings.SplitN(rv, ` `, 3)
		switch retval[0] {
		case "237":
			fallthrough
		case "244":
			logln(`[INFO] サーバ` + p2s.IPPort + `: 鍵の取得`)
			keyslice := strings.Split(retval[2], `:`)
			peer.secKey = keyslice[0]
			peer.pubKey = keyslice[1]
			peer.keySig = keyslice[3]

			loc, err := time.LoadLocation("Asia/Tokyo")
			if err != nil {
				loc = time.FixedZone("Asia/Tokyo", 9*60*60)
			}
			peer.keyExpire, err = time.ParseInLocation(`2006/01/02 15-04-05`, keyslice[2], loc)
			if err != nil {
				return err
			}

			logln(`[DEBUG] サーバ`+p2s.IPPort+`: 秘密鍵`, peer.secKey)
			logln(`[DEBUG] サーバ`+p2s.IPPort+`: 公開鍵`, peer.pubKey)
			logln(`[DEBUG] サーバ`+p2s.IPPort+`: 有効期限`, peer.keyExpire)
			logln(`[DEBUG] サーバ`+p2s.IPPort+`: 鍵署名`, peer.keySig)

			peer.SaveKey()

		case "295":
			logln(`[DEBUG] サーバ` + p2s.IPPort + `: キー割当済`)
		default:
			logln(`[DEBUG] サーバ` + p2s.IPPort + `: エラー` + strings.Join(retval, ` `))
		}
	}

	return
}

// Echo は、エコーを送信し、返信を受け取ります
func (p2s *P2SClient) Echo(ctx context.Context, peerID string, peercount uint64) (err error) {

	if err = p2s.EPSPConn.Write(`123`, `1`, peerID+":"+strconv.FormatUint(peercount, 10)); err == nil {
		logln(`[DEBUG] サーバ` + p2s.IPPort + `: エコー要求送信 PeerID: ` + peerID)
		p2s.SetPingTime()
		var rv string
		ctxtimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if rv, err = p2s.Get(ctxtimeout); err == nil {
			retval := strings.SplitN(rv, ` `, 3)
			switch retval[0] {
			case `243`:
				logln(`[DEBUG] サーバ` + p2s.IPPort + `: エコー返信`)
				p2s.SetPongTime()
				return nil
			case `299`: // エコー時のIPアドレスが参加時と変わっている場合、コード299が返されることがあります。この場合、一旦ネットワークから切断し、参加しなおしてください。
				return errors.New(`IPアドレスが変わったかも`)
			default:
				return errors.New(`エコーがこないよ ` + retval[0])
			}
		} else {
			return errors.Wrap(err, `エコー返信受信`)
		}
	} else {
		return errors.Wrap(err, `エコー要求送信不能`)
	}
}

// CheckPortOpen は、ポート開放をサーバに確認します
func (p2s P2SClient) CheckPortOpen(ctx context.Context, peerID string, peercount int) (open bool, err error) {

	if err = p2s.EPSPConn.Write(`114`, `1`, peerID+":"+strconv.Itoa(peercount)); err != nil {
		err = errors.Wrap(err, `ポート開放確認不能`)
		return
	}
	logln(`[DEBUG] サーバ` + p2s.IPPort + `: ポート開放確認`)
	var rv string
	rv, err = p2s.Get(ctx)
	if err != nil {
		err = errors.Wrap(err, `ポート開放確認不能`)
		return
	}

	retval := strings.SplitN(rv, ` `, 3)
	switch retval[0] {
	case `234`:
		switch retval[2] {
		case `1`:
			logln(`[DEBUG] サーバ` + p2s.IPPort + `: ポート開放成功`)
			open = true
		case `0`:
			logln(`[DEBUG] サーバ` + p2s.IPPort + `: ポート開放失敗`)
		}
		err = nil
		return
	default:
		err = errors.New(`ポート開放確認がこないよ ` + retval[0])
		return
	}
}

// PeerCountByRegion は、地域ごとのピア数を取得します
func (p2s P2SClient) PeerCountByRegion(ctx context.Context, code5xx func(from *P2PPeer, retval []string) error) (peerCountByName PeerCounts, err error) {

	if err = p2s.EPSPConn.Write(`127`, `1`); err != nil {
		err = errors.Wrap(err, `各地域ピア数要求不能`)
		return
	}
	logln(`[DEBUG] サーバ` + p2s.IPPort + `: 各地域ピア数要求`)

	var rv string
	rv, err = p2s.Get(ctx)
	if err != nil {
		err = errors.Wrap(err, "サーバ")
		return
	}
	retval := strings.SplitN(rv, ` `, 3)

	switch retval[0] {
	case `247`:
		logln(`[DEBUG] サーバ` + p2s.IPPort + `: 各地域ピア数受信`)
		err = nil
		peerCountByName = NewPeerCount(retval[2])
		return
	default:
		err = errors.New(`[DEBUG] サーバ` + p2s.IPPort + `: 各地域ピア数がこないよ`)
		return
	}

}

// GetTime は、プロトコル時刻を取得します
func (p2s P2SClient) GetTime(ctx context.Context) (t time.Time, err error) {

	err = p2s.EPSPConn.Write(`118`, `1`)
	if err != nil {
		err = errors.Wrap(err, `プロトコル時刻要求不能`)
		return
	}
	logln(`[DEBUG] サーバ` + p2s.IPPort + `: プロトコル時刻要求`)

	rv, err := p2s.Get(ctx)
	if err != nil {
		err = errors.Wrap(err, `プロトコル時刻取得不能`)
		return
	}

	retval := strings.SplitN(rv, ` `, 3)
	if retval[0] != "238" {
		err = errors.New(`プロトコル時刻がこないよ[` + retval[0] + `]`)
		return
	}
	logln(`[DEBUG] サーバ` + p2s.IPPort + `: プロトコル時刻返戻`)

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.FixedZone("Asia/Tokyo", 9*60*60)
	}
	t, err = time.ParseInLocation(`2006/01/02 15-04-05`, retval[2], loc)

	return
}

// Close は、サーバとの接続を終了します
func (p2s *P2SClient) Close(ctx context.Context) {
	var err error
	if p2s == nil {
		logln(`[INFO] サーバ` + p2s.IPPort + `: P2SClient close 通信の終了済`)
		return
	}

	if err = p2s.EPSPConn.Write(`119`, `1`); err != nil { // 通信の終了を要求します。
		logln(`[WARN] サーバ` + p2s.IPPort + `: P2SClient close 通信の終了要求不能`)
		p2s.EPSPConn.Close()
		return
	}
	logln(`[DEBUG] サーバ` + p2s.IPPort + `: 通信の終了要求`)

	var rv string
	if rv, err = p2s.Get(ctx); err != nil {
		logln(`[WARN] サーバ` + p2s.IPPort + `: P2SClient close 通信の終了不着`)
		p2s.EPSPConn.Close()
		return
	}

	retval := strings.SplitN(rv, ` `, 3)

	if retval[0] != "239" {
		logln(`[WARN] サーバ` + p2s.IPPort + `: P2SClient close 通信の終了がこないよ`)
		p2s.EPSPConn.Close()
		return
	}
	logln(`[INFO] サーバ` + p2s.IPPort + `: 通信の終了`)
	p2s.EPSPConn.Close()
}

// TellPeer は、ピアとの接続状況を、サーバに伝えます
func (p2s P2SClient) TellPeer(ps P2PPeers, limited []string) (err error) {
	var peerlists []string

	for j := range limited {
		for i := range ps {
			if ps[i].IsConn() && ps[i].PeerID == strings.Split(limited[j], `,`)[2] {
				peerlists = append(peerlists, ps[i].PeerID)
				break
			}
		}
	}

	if len(peerlists) != 0 {
		peerlist := strings.Join(peerlists, `:`)
		logln(`[DEBUG] サーバ` + p2s.IPPort + `: Send Peerlist: ` + peerlist)
		err = p2s.EPSPConn.Write(`155`, `1`, peerlist)
		if err != nil {
			err = errors.Wrap(err, `Send Peerlist不能`)
			return
		}
	}
	return
}
