package paths

import (
	"fmt"
	"os"

	gap "github.com/muesli/go-app-paths"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Paths struct {
	Log    string
	Config string
}

func New(name string) (*Paths, error) {
	p := Paths{}
	var err error
	scope := gap.NewScope(gap.User, name)
	logFilename := fmt.Sprintf("%s.log", name)
	p.Log, err = scope.LogPath(logFilename)
	if err != nil {
		return nil, err
	}
	configFilename := fmt.Sprintf("%s.yaml", name)
	p.Config, err = scope.ConfigPath(configFilename)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func pathCreate(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Path does not exist: ", path)
		}
	}
	if fi.IsDir() {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Paths) Logger(fromStdError bool) *zap.Logger {
	if fromStdError {
		return initLoggerToStdErr()
	}
	return initLoggerToFile(p.Log)
}

func initLoggerToStdErr() *zap.Logger {
	stderrSyncer := zapcore.Lock(os.Stderr)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, stderrSyncer, zapcore.DebugLevel)
	return zap.New(core, zap.AddCaller())
}

func initLoggerToFile(filename string) *zap.Logger {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    50, //mb
		MaxBackups: 10,
		MaxAge:     30, //days
		Compress:   false,
	}
	writerSyncer := zapcore.AddSync(lumberJackLogger)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, writerSyncer, zapcore.DebugLevel)
	return zap.New(core, zap.AddCaller())
}
