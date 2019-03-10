package epsp

import (
	"bufio"
	"context"
	"io"
	"math"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// EPSPConn は、EPSPの接続情報を保持します
type EPSPConn struct {
	IPPort       string
	ConnTime     *time.Time
	PingTime     *time.Time
	PongTime     *time.Time
	PingPong     *time.Duration
	PingRecvTime *time.Time
	DiscTime     *time.Time
	LastRXTime   *time.Time
	Agent        []string
	conn         *net.TCPConn
	Tx           uint64
	Rx           uint64
	RxUniq       uint64
	RxDup        uint64
}

// SetConnTime は、現在時刻を接続時間として設定します
func (p *EPSPConn) SetConnTime() {
	p.ConnTime = new(time.Time)
	*p.ConnTime = time.Now()
}

// SetDiscTime は、現在時刻を切断時間として設定します
func (p *EPSPConn) SetDiscTime() {
	p.DiscTime = new(time.Time)
	*p.DiscTime = time.Now()
}

// SetPingTime は、現在時刻をPingした時刻として設定します
func (p *EPSPConn) SetPingTime() {
	if p.PingTime == nil {
		p.PingTime = new(time.Time)
	}
	*p.PingTime = time.Now()
}

// SetPongTime は、現在時刻をPingの返答を受け取った時刻として設定します
func (p *EPSPConn) SetPongTime() {
	if p.PongTime == nil {
		p.PongTime = new(time.Time)
	}
	*p.PongTime = time.Now()
	if p.PingPong == nil {
		p.PingPong = new(time.Duration)
	}
	*p.PingPong = p.PongTime.Sub(*p.PingTime)
}

// SetPingRecvTime は、現在時刻をPingを受け取った時刻として設定します
func (p *EPSPConn) SetPingRecvTime() {
	if p.PingRecvTime == nil {
		p.PingRecvTime = new(time.Time)
	}
	*p.PingRecvTime = time.Now()
}

// SetLastRXTime は、最後にデータを受信した時刻を設定します
func (p *EPSPConn) SetLastRXTime() {
	if p.LastRXTime == nil {
		p.LastRXTime = new(time.Time)
	}
	*p.LastRXTime = time.Now()
}

// GetPingTime は、Pingした時刻を取得します
func (p *EPSPConn) GetPingTime() *time.Time {
	return p.PingTime
}

// GetDiscTime は、Pingした時刻を取得します
func (p *EPSPConn) GetDiscTime() *time.Time {
	return p.DiscTime
}

// GetPingRecv は、Pingを受信した時刻を取得します
func (p *EPSPConn) GetPingRecv() *time.Time {
	return p.PingRecvTime
}

// AddTx はTxを一つ増やします
func (p *EPSPConn) AddTx() {
	p.Tx++
}

// AddRx はRxを一つ増やします
func (p *EPSPConn) AddRx() {
	p.Rx++
}

// AddRxDup はRxDupを一つ増やします
func (p *EPSPConn) AddRxDup() {
	p.RxDup++
}

// AddRxUniq はRxUniqを一つ増やします
func (p *EPSPConn) AddRxUniq() {
	p.RxUniq++
}

// GetRXUniqRate はすべての受信情報のうち、最速だったものの割合の逆数を返します
func (p *EPSPConn) GetRXUniqRate() uint64 {
	if p.RxUniq == 0 {
		return math.MaxUint64
	}
	return (p.RxUniq + p.RxDup) / p.RxUniq
}

// IsConn は、接続中かどうか返します
func (p *EPSPConn) IsConn() bool {
	return p.GetDiscTime() == nil
}

// Get は、データを一行取得し、スペースで区切り、文字列配列を返します
func (p *EPSPConn) Get(ctx context.Context) ([]string, error) {
	if !p.IsConn() {
		return nil, errors.New(`No Connection`)
	}

	readch := make(chan []byte)
	errch := make(chan error)

	go func() {
		var b []byte
		for {
			if bcont, isPrefix, err := bufio.NewReader(p.conn).ReadLine(); err == nil {
				b = append(b, bcont...)
				if !isPrefix {
					errch <- nil
					readch <- b
					p.SetLastRXTime()
					return
				}
			} else {
				errch <- err
				readch <- []byte{}
				return
			}
		}
	}()

	select {
	case err := <-errch:
		if err != nil {
			if err == io.EOF {
				p.Close()
			}
			return nil, errors.Wrap(err, `ReadLine`)
		}
		p.AddRx()
		return strings.SplitN(string(<-readch), ` `, 3), err
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), `ReadLine`)
	}
}

func (p *EPSPConn) Write(strs ...string) error {
	if !p.IsConn() {
		return errors.New(`No Connection`)
	}

	if _, err := p.conn.Write([]byte(strings.Join(strs, ` `) + "\r\n")); err != nil {
		return errors.Wrap(err, `conn.Write`)
	}
	p.AddTx()
	return nil
}

// Close はTCPを終了します
func (p *EPSPConn) Close() {
	if p.conn != nil {
		err := p.conn.Close()
		_ = err
	}
	p.SetDiscTime()
}
