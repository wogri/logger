package logger

// This is a logging module that enforces structured logging and emits prom metrics.

import (
	"os"
	"time"

	zaplogfmt "github.com/sykesm/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	sugar   *zap.SugaredLogger
  logCounter = promauto.NewCounterVec(
    prometheus.CounterOpts{
      Name: "logger_logs_total",
      Help: "Number of logs emitted with a type label",
    },
    []string{"type"},
  )

)

func init() {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(time.RFC3339Nano))
	}
	level := zapcore.InfoLevel
	if os.Getenv("VERBOSE") != "" {
		level = zapcore.DebugLevel
	}
	logger := zap.New(zapcore.NewCore(
		zaplogfmt.NewEncoder(config),
		os.Stdout,
		level,
	), zap.AddCaller(), zap.AddCallerSkip(1))

	defer logger.Sync()
	sugar = logger.Sugar()
}

func Debug(msg string, keysAndValues ...interface{}) {
	sugar.Debugw(msg, keysAndValues...)
  logCounter.WithLabelValues("Debug").Inc()
}

func Info(msg string, keysAndValues ...interface{}) {
	sugar.Infow(msg, keysAndValues...)
  logCounter.WithLabelValues("Info").Inc()
}

func Error(msg string, keysAndValues ...interface{}) {
	sugar.Errorw(msg, keysAndValues...)
  logCounter.WithLabelValues("Error").Inc()
}

func Fatal(msg string, keysAndValues ...interface{}) {
	sugar.Fatalw(msg, keysAndValues...)
  // We're doing this although it's not useful.
  logCounter.WithLabelValues("Fatal").Inc()
}

func Sync() {
	sugar.Sync()
}
