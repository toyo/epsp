package epsp

import (
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/hashicorp/logutils"
)

var (
	logger *log.Logger
	logMu  sync.Mutex
)

func init() {
	logger = log.New(os.Stderr, ``, log.LstdFlags)
	logger.SetOutput(&logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"ECHO", "DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel("WARN"),
		Writer:   os.Stderr,
	})
	SetLogger(logger)
}

// SetLogger は、ロガーを設定します
func SetLogger(l *log.Logger) {
	if l == nil {
		l = log.New(ioutil.Discard, ``, log.LstdFlags) // logger is for debugging in epsp
	}
	logMu.Lock()
	logger = l
	logMu.Unlock()
}

// SetLoggerDebug は、ロガーをデバッグモードにします
func SetLoggerDebug() {
	logger = log.New(os.Stderr, ``, log.LstdFlags)
}

func logf(format string, v ...interface{}) {
	logMu.Lock()
	logger.Printf(format, v...)
	logMu.Unlock()
}

func logln(v ...interface{}) {
	logMu.Lock()
	logger.Print(v...)
	logMu.Unlock()
}
