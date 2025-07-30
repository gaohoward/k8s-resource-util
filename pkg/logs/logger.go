package logs

import (
	"fmt"
	"io"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// MAIN_LOGGER_NAME is used to print to the console
	MAIN_LOGGER_NAME = "main"
	// APP_LOGGER_NAME is used to log to UI components
	IN_APP_LOGGER_NAME = "in-app"
)

// appLoggers are used to log to UI components
var appLoggers = make(map[string]*zap.Logger)
var loggerLock = sync.RWMutex{}

func NewAppLogger(name string, sink ...io.Writer) (*zap.Logger, error) {

	loggerLock.Lock()
	defer loggerLock.Unlock()

	if _, ok := appLoggers[name]; ok {
		return nil, fmt.Errorf("logger already exists %v", name)
	}

	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var cores []zapcore.Core

	for _, s := range sink {
		writer := zapcore.AddSync(s)
		core := zapcore.NewCore(zapcore.NewJSONEncoder(cfg), writer, zapcore.DebugLevel)
		cores = append(cores, core)
	}

	var logger *zap.Logger
	if len(cores) == 0 {
		logger = zap.Must(zap.NewDevelopment())
		if os.Getenv("APP_ENV") == "production" {
			logger = zap.Must(zap.NewProduction())
		}
	} else {
		logger = zap.New(zapcore.NewTee(cores...))
	}
	appLoggers[name] = logger

	return logger, nil
}

func GetLogger(name string) *zap.Logger {
	if l, ok := appLoggers[name]; ok {
		return l
	}
	fmt.Printf("No logger named %s found\n", name)
	fakeLogger, _ := NewAppLogger(name+"-fake", nil)
	return fakeLogger
}
