package log

import (
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var g *Logger = NewLogger()

const (
	Ldebug = iota
	Linfo
	Lwarn
	Lerror
	Lpanic
	Lfatal
)

func WithId(id uint32) *Logger {
	return &Logger{
		logger: g.logger.With(zap.String("id", strconv.FormatUint(uint64(id), 10))),
	}
}

// SetDebugLevel 调整为 debug 模式，打印更多的日志
func SetDebugLevel() {
	g.SetDebugLevel()
}

// SetInfoLevel 调整为 info 模式，只打印必要的日志
func SetInfoLevel() {
	g.SetInfoLevel()
}

func Fatalf(format string, v ...interface{}) {
	g.Fatalf(format, v...)
}

func Fatal(v ...interface{}) {
	g.Fatal(v...)
}

func Errorf(format string, v ...interface{}) {
	g.Errorf(format, v...)
}

func Error(v ...interface{}) {
	g.Error(v...)
}

func Warnf(format string, v ...interface{}) {
	g.Warnf(format, v...)
}

func Warn(v ...interface{}) {
	g.Warn(v...)
}

func Panicf(format string, v ...interface{}) {
	g.Panicf(format, v...)
}

func Panic(v ...interface{}) {
	g.Panic(v...)
}

func Infof(format string, v ...interface{}) {
	g.Infof(format, v...)
}

func Info(v ...interface{}) {
	g.Info(v...)
}

func Debugf(format string, v ...interface{}) {
	g.Debugf(format, v...)
}

func Debug(v ...interface{}) {
	g.Debug(v...)
}

// implementation
type Logger struct {
	logger *zap.Logger
}

func newLogger(debug bool) *zap.Logger {

	consoleDebugging := zapcore.AddSync(os.Stderr)

	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	var core zapcore.Core
	if debug == true {
		core = zapcore.NewCore(consoleEncoder, consoleDebugging, zap.NewAtomicLevelAt(zapcore.DebugLevel))
	} else {
		core = zapcore.NewCore(consoleEncoder, consoleDebugging, zap.NewAtomicLevelAt(zapcore.InfoLevel))

	}

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2))
	return logger
}

func NewLogger() *Logger {

	logger := newLogger(false)

	zap.ReplaceGlobals(logger)
	return &Logger{logger: logger}
}

func (l *Logger) SetDebugLevel() {
	logger := newLogger(true)
	zap.ReplaceGlobals(logger)
	l.logger = logger
}

func (l *Logger) SetInfoLevel() {
	logger := newLogger(false)
	zap.ReplaceGlobals(logger)
	l.logger = logger
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	zap.S().Errorf(format, v...)
}

func (l *Logger) Error(v ...interface{}) {
	zap.S().Error(v...)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	zap.S().Fatalf(format, v...)
}

func (l *Logger) Fatal(v ...interface{}) {
	zap.S().Fatal(v...)
}

func (l *Logger) Panicf(format string, v ...interface{}) {
	zap.S().Panicf(format, v...)
}

func (l *Logger) Panic(v ...interface{}) {
	zap.S().Panic(v...)
}

func (l *Logger) Printf(format string, v ...interface{}) {
	zap.S().Infof(format, v...)
}

func (l *Logger) Println(v ...interface{}) {
	zap.S().Info(v...)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	zap.S().Infof(format, v...)
}

func (l *Logger) Info(v ...interface{}) {
	zap.S().Info(v...)
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	zap.S().Debugf(format, v...)
}

func (l *Logger) Debug(v ...interface{}) {
	zap.S().Debug(v...)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	zap.S().Warnf(format, v...)
}

func (l *Logger) Warn(v ...interface{}) {
	zap.S().Warn(v...)
}
