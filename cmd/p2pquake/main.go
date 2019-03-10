package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/logutils"
	epsp "github.com/toyo/p2pq"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	var (
		d = flag.Bool(`d`, false, `debug flag`)
	)
	flag.Parse()

	if *d {
		logger := log.New(os.Stderr, ``, log.LstdFlags)
		logger.SetOutput(&logutils.LevelFilter{
			Levels:   []logutils.LogLevel{"ECHO", "DEBUG", "INFO", "WARN", "ERROR"},
			MinLevel: logutils.LogLevel("DEBUG"),
			Writer:   os.Stderr,
		})
		epsp.SetLogger(logger)
	}

	peer, err := epsp.NewPeer(
		/* servers */ []string{
			`www.p2pquake.net:6910`, `p2pquake.dnsalias.net:6910`,
			`p2pquake.dyndns.info:6910`, `p2pquake.ddo.jp:6910`},
		/* region */ `250`,
		/* incoming */ 20,
		[]byte(`-----BEGIN PUBLIC KEY-----
MIGdMA0GCSqGSIb3DQEBAQUAA4GLADCBhwKBgQC8p/vth2yb/k9x2/PcXKdb6oI3gAbhvr
/HPTOwla5tQHB83LXNF4Y+Sv/Mu4Uu0tKWz02FrLgA5cuJZfba9QNULTZLTNUgUXIB0m/d
q5Rx17IyCfLQ2XngmfFkfnRdRSK7kGnIXvO2/LOKD50JsTf2vz0RQIdw6cEmdl+Aga7i8Q
IBEQ==
-----END PUBLIC KEY-----`),
		[]byte(`-----BEGIN PUBLIC KEY-----
MIGdMA0GCSqGSIb3DQEBAQUAA4GLADCBhwKBgQDTJKLLO7wjCHz80kpnisqcPDQvA9voNY
5QuAA+bOWeqvl4gmPSiylzQZzldS+n/M5p4o1PRS24WAO+kPBHCf4ETAns8M02MFwxH/Fl
QnbvMfi9zutJkQAu3Hq4293rHz+iCQW/MWYB5IfzFBnWtEdjkhqHsGy6sZMMe+qx/F1rcQ
IBEQ==
-----END PUBLIC KEY-----`),
		usercmd)
	if err != nil {
		log.Fatal(err)
	}

	hs := http.NewServeMux()
	peer.WebSocketAPI(ctx, hs)

	hs.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "html/index.html")
	})

	hs.HandleFunc("/cancel", func(w http.ResponseWriter, r *http.Request) {
		cancel()
	})

	errCh := make(chan error)
	go func() {
		errCh <- http.ListenAndServe(`:6980`, hs)
	}()

	go func() {
		errCh <- peer.Loop(ctx, 6911 /* tcp port */)
	}()

	select {
	case err := <-errCh:
		log.Fatal(err)
	case s := <-c:
		cancel()
		log.Println(`Terminating by`, s)
		time.Sleep(1 * time.Second)
		os.Exit(0)
	case <-ctx.Done():
		cancel()
		log.Println(`Terminating by`, ctx.Err())
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}
}

func usercmd(code string, recvdata ...string) {

	switch code {
	case "551":
		gaiyob, _, err := transform.String(japanese.ShiftJIS.NewDecoder().Transformer, recvdata[2])
		if err != nil {
			log.Println(err)
		}
		syohsaib, _, err := transform.String(japanese.ShiftJIS.NewDecoder().Transformer, recvdata[3])
		if err != nil {
			log.Println(err)
		}

		gaiyo := strings.Split(string(gaiyob), `,`)
		showsai := strings.Split(string(syohsaib), `,`)

		log.Print("地震情報:" + gaiyo[0] + `、震度` + gaiyo[1] + `の地震がありました。` +
			`震源は` + gaiyo[4] + `、深さは` + gaiyo[5] + `、マグニチュードは` + gaiyo[6] + `と推定されます。`)
		switch gaiyo[2] {
		case `0`:
			log.Print(`津波の心配はありません。`)
		case `1`:
			log.Print(`津波に注意してください。`)
		case `2`:
			log.Print(`津波について調査中です。`)
		case `3`:
			log.Print(`津波について不明です。`)
		}
		if gaiyo[7] == `1` {
			log.Print(`震度が訂正されました。`)
		}
		log.Println(strings.Join(showsai, `,`))
	case "555":
		kanchidata := strings.Split(recvdata[5], `,`)
		log.Println("地震感知情報 " + epsp.Area(kanchidata[1]) + `(PubKey:` + recvdata[2] + `)から` + kanchidata[0])
	default:
		log.Println(`未知コード受信 ` + code + ` n ` + strings.Join(recvdata, `:`))
	}
}
