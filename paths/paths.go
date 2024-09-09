package paths

import (
	"bytes"
	"fmt"
	"io"
	"os"

	gap "github.com/muesli/go-app-paths"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v2"
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

func (p *Paths) LoadConfig(c map[interface{}]interface{}) (map[interface{}]interface{}, error) {
	f, err := os.Open(p.Config)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(buf.Bytes(), &c)
	if err != nil {
		return nil, err
	}
	return c, nil
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
