package epsp

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Loop は、EPSPサーバと定期的な接続を行うことで、ピアとの接続を維持するメソッドです。
func (peer *Peer) Loop(ctx context.Context, port int) (err error) {
	peerIsRegistered := peer.PeerID != ``

restart:
	for i := 0; ; i++ {

		if i >= len(peer.hosts) {
			i -= len(peer.hosts)
		}
		ctxtimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		peer.EPSPServer, err = NewP2SClient(ctxtimeout, peer.hosts[i], peer.MyAgent)

		if err == nil {

			gotTempPeerID := false
			if peerIsRegistered {
				if err = peer.EPSPServer.Echo(ctx, peer.PeerID, peer.NumOfConnectedPeers()); err != nil {
					logln(`[DEBUG] PeerID expired. P2S Restart`, err)
					peerIsRegistered = false
					peer.EPSPServer.Close(ctx)
					continue restart
				}
				if err = peer.EPSPServer.GetKey(ctx, peer, true); err != nil { // 鍵の再割り当てを要求します。
					logln(`[WARN] GetKey ` + err.Error())
					continue restart
				}
			} else {
				if peer.PeerID, err = peer.EPSPServer.GetTemporaryPeerID(ctx); err != nil {
					logln(`[WARN] GetTemporaryPeerID ` + err.Error())
					peer.EPSPServer.Close(ctx)
					continue restart
				}
				gotTempPeerID = true
			}

			peer.Clients.AddP2PClients(ctx, peer.PeerID, peer.candidatePeers, peer.MyAgent, peer.p2mpcmd, peer.ConnectedIPPortPeersList, peer.incoming)
			peer.candidatePeers = []string{}

			peer.serverIsRunning.Do(func() {
				_, err := peer.Servers.NewP2PServers(ctx, peer.PeerID, peer.MyAgent, port, peer.p2mpcmd, peer.ConnectedIPPortPeersList, peer.incoming)
				if err != nil {
					logln(`[ERROR] NewP2PServers Error`, err)
					return
				}
				if gotTempPeerID {
					if peer.Global, err = peer.EPSPServer.CheckPortOpen(ctx, peer.PeerID, port); err != nil {
						logln(`[WARNING] CheckPortOpen Error`, err)
						return
					}
				}

				logln(`[DEBUG] PortOpen: `, peer.Global, ` Clients: `, peer.Clients.NumOfConnectedPeers())
			})

			if gotTempPeerID ||
				(peer.Servers.NumOfConnectedPeers() != 0 && peer.Clients.NumOfConnectedPeers()*3 < peer.incoming) ||
				(peer.Servers.NumOfConnectedPeers() == 0 && peer.Clients.NumOfConnectedPeers()*3 < peer.incoming*2) {

				var getPeers []string

				if getPeers, err = peer.EPSPServer.GetPeers(ctx, peer.PeerID); err != nil {
					logln(`[WARN] GetPeers ` + err.Error())
					peer.EPSPServer.Close(ctx)
					continue restart
				}
				peer.Clients.AddP2PClients(ctx, peer.PeerID, getPeers, peer.MyAgent, peer.p2mpcmd, peer.ConnectedIPPortPeersList, peer.incoming)
				if err = peer.EPSPServer.TellPeer(peer.Clients, getPeers); err != nil { // 新たに接続出来たピアのIDを通知します。
					logln(`[WARN] TellPeer ` + err.Error())
					peer.EPSPServer.Close(ctx)
					continue restart
				}
			}

			if gotTempPeerID {
				if peer.Global {
					if err = peer.EPSPServer.Regist(ctx, peer.PeerID, port, peer.region, peer.NumOfConnectedPeers(), peer.incoming); err != nil {
						logln(`[WARN] Regist ` + err.Error())
						peer.EPSPServer.Close(ctx)
						continue restart
					}
				} else {
					if err = peer.EPSPServer.Regist(ctx, peer.PeerID, port, peer.region, peer.NumOfConnectedPeers(), 0); err != nil {
						logln(`[WARN] Regist ` + err.Error())
						peer.EPSPServer.Close(ctx)
						continue restart
					}
				}
				peerIsRegistered = true

				if err = peer.EPSPServer.GetKey(ctx, peer, false); err != nil { // 必要に応じて鍵の割り当てを要求します。
					logln(`[WARN] GetKey ` + err.Error())
					peer.EPSPServer.Close(ctx)
					continue restart
				}

				if peer.PeerCountsByRegion == nil || peer.ProtocolTimeDiff == 0 {
					var err error
					if peer.PeerCountsByRegion, err = peer.EPSPServer.PeerCountByRegion(ctx, peer.p2mpcmd); err != nil {
						logln(`[DEBUG] PeerCountByRegion`, err)
					}
				}

				if peer.ProtocolTimeDiff == 0 {
					var t time.Time
					if t, err = peer.EPSPServer.GetTime(ctx); err == nil {
						peer.ProtocolTimeDiff = time.Until(t)
						logln(`[DEBUG] Protocol Time`, t, `Diff=`, peer.ProtocolTimeDiff)
					} else {
						logln(err)
					}
				}
			}
		} else {
			logln(`[WARNING] サーバ`+peer.hosts[i]+`: ESPSサーバ接続エラー`, err)
			peer.serverErrorCount++
			if peer.serverErrorCount <= uint16(len(peer.hosts)) {
				continue restart
			} else {
				logln(`[WARNING] サーバ` + peer.hosts[i] + `: ESPS全サーバ接続エラー`)
				if peer.NumOfConnectedPeers() == 0 {
					return
				}
			}
		}
		peer.serverErrorCount = 0
		peer.EPSPServer.Close(ctx) // close p2s connection

		peer.SaveKey()
		duration := 10 * time.Minute * time.Duration(peer.NumOfConnectedPeers()) / time.Duration(peer.incoming)
		if duration > 1*time.Minute {
			duration = 10 * time.Minute
		}

		timer := time.NewTimer(duration)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			continue restart
		}
	}
}

func checkExpired(recvdata []string) (err error) {
	expiredate, err := time.Parse(`2006/01/02 15-04-05`, recvdata[1])
	if err != nil {
		return err
	}
	if expired := time.Now().After(expiredate); expired {
		return errors.New("データ有効期限切れ: " + recvdata[1])
	}
	return nil
}

func (peer *Peer) checkNotDuplicate(from *P2PPeer, recvdata []string) (err error) {
	datasig := recvdata[0]
	if _, dup := peer.sigmap.LoadOrStore(datasig, struct{}{}); dup {
		from.AddRxDup()
		return errors.New("重複: " + recvdata[0])
	}
	from.AddRxUniq()
	return nil
}

func (peer *Peer) checkSignature(cmd string, recvdata []string) (err error) {
	switch cmd[2] {
	case '1': // cmd= 551, 552, 561 サーバ保証用公開鍵
		fallthrough
	case '2':

		dataSig := recvdata[0]
		expDate := recvdata[1]
		dataBody := recvdata[2]

		if err := DataSignatureCheck(peer.serverKey, dataSig, expDate, dataBody); err != nil {
			return errors.Wrap(err, `データ署名NG`)
		}

	case '5': // cmd= 555, 556 ピア保証用公開鍵
		fallthrough
	case '6':

		pubKey := recvdata[2]
		keySig := recvdata[3]
		keyExpDate := recvdata[4]

		if peerPubKey, err := KeySignatureCheck(peer.peerKey, pubKey, keySig, keyExpDate); err == nil {
			dataSig := recvdata[0]
			expDate := recvdata[1]
			dataBody := recvdata[5]

			if err = DataSignatureCheck(peerPubKey, dataSig, expDate, dataBody); err != nil {
				return errors.Wrap(err, `鍵署名OK、データ署名NG`)
			}
		} else {
			return errors.Wrap(err, `鍵署名NG`)
		}
	}
	return nil
}

func (peer *Peer) cmd561(recvdata []string) {
	peer.PeerCountsByRegion = NewPeerCount(recvdata[2])
	peer.SaveKey()
}

func (peer *Peer) cmd615(from *P2PPeer, recvdata []string, hops string) error {
	if recvdata[0] == peer.PeerID {
		return nil // do nothing because 615 from me.
	}
	if _, ok := peer.traceecho.LoadOrStore(recvdata[1], from); !ok {
		// 過去の調査エコーバッファと比較し、新規エコーだった場合のみ処理を続けます。
		// 「一意な数」と「送信元（ソケット番号など、後で送り返しするために必要な値）」を新たにバッファに追加します。
		err := from.WriteTo(`635`, `1`, strings.Join(recvdata, `:`)+`:`+peer.PeerID+`:`+strings.Join(peer.ConnectedPeersList(), `,`)+`:`+hops)
		// 送信元に対し、「調査エコーリプライ(コード635)」を送信します。
		if err != nil {
			from.Close()
			return errors.New(`635 Transmit error`)
		}
	}
	return nil
}

func (peer *Peer) cmd635(retval []string) (sent bool, err error) {
	recvdata := strings.Split(retval[2], `:`)

	if recvdata[0] == peer.PeerID {
		go peer.usercmd(retval[0], recvdata...)
		return true, nil // do usercmd because 635 for me.
	}
	origp, ok := peer.traceecho.Load(recvdata[1])
	if !ok { // 一致するバッファがあった場合のみ処理を続けます。
		return false, nil
	}
	origpeer, ok := origp.(*P2PPeer)
	if !ok {
		return false, errors.New(`[ERROR] Type assertion on 635`)
	}
	logln(`[DEBUG] ピア` + peer.PeerID + `: ユニキャスト送信:` + origpeer.GetPeerID() + ` ` + strings.Join(retval[:2], ` `))
	err = origpeer.WriteTo(retval...)
	// 過去の調査エコーバッファで記憶されている「送信元」に対し、調査エコーリプライをリレーします。
	if err == nil {
		return true, nil
	}
	// 送信元との接続が切断されている場合は、接続中の全てのピアに対してリレーします。
	return false, nil
}

func (peer *Peer) mpReSent(from *P2PPeer, retval []string) error {
	if hops, err := strconv.ParseUint(retval[1], 10, 64); err == nil {
		if peer.PeerCountsByRegion.NumOfAllPeers() >= hops {
			retval[1] = strconv.FormatUint(hops+1, 10) // Hop count add
			logln(`[DEBUG] ピア` + peer.PeerID + `: マルチキャスト送信:` + strings.Join(retval[:2], ` `))
			go peer.WriteExceptFrom(from, retval...)
		} else {
			return errors.Errorf(`総参加ピア数(%d) < 経由数(%d)`, peer.PeerCountsByRegion.NumOfAllPeers(), hops)
		}
	} else {
		return errors.New(`経由数書式異常 ` + strings.Join(retval, ` `))
	}
	return nil
}

func (peer *Peer) p2mpcmd(from *P2PPeer, retval []string) error {
	recvdata := strings.Split(retval[2], `:`)

	if (retval[0][0] == '5' || retval[0][0] == '6') && !(retval[0] == `615` || retval[0] == `635`) {
		if err := checkExpired(recvdata); err != nil {
			return errors.Wrap(err, `ピア`+from.GetPeerIDorIPPort()+": "+retval[0]+` `+retval[1])
		}
		if err := peer.checkNotDuplicate(from, recvdata); err != nil {
			return errors.Wrap(err, `ピア`+from.GetPeerIDorIPPort()+": "+retval[0]+` `+retval[1])
		}
	}

	if retval[0][0] == '5' {
		if err := peer.checkSignature(retval[0], recvdata); err != nil {
			return errors.Wrap(err, `ピア`+from.GetPeerIDorIPPort()+": "+retval[0]+` `+retval[1])
		}
	}

	switch retval[0] {
	case `561`:
		peer.cmd561(recvdata)
	case `635`:
		sent, err := peer.cmd635(retval)
		if err != nil {
			return err
		}
		if sent {
			return nil
		}
	case `615`:
		if err := peer.cmd615(from, recvdata, retval[1]); err != nil {
			return err
		}
	default:
		go peer.usercmd(retval[0], recvdata...)
	}

	return peer.mpReSent(from, retval)
}
