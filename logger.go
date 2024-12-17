package logger

// This is a logging module that enforces structured logging and emits prom metrics.

import (
	"os"
	"time"

	zaplogfmt "github.com/sykesm/zap-logfmt"
	// zap json

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	sugar      *zap.SugaredLogger
	sugarCallerSkip1 *zap.SugaredLogger
	debugmode  bool
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
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	level := zapcore.InfoLevel
	if os.Getenv("VERBOSE") != "" {
		level = zapcore.DebugLevel
		debugmode = true
	}
	var logger *zap.Logger
	var loggerSkip *zap.Logger
	if os.Getenv("PRODUCTION") == "" {
		logger = zap.New(zapcore.NewCore(
			zaplogfmt.NewEncoder(config),
			os.Stdout,
			level,
		), zap.AddCaller(), zap.AddCallerSkip(1))
		loggerSkip = zap.New(zapcore.NewCore(
			zaplogfmt.NewEncoder(config),
			os.Stdout,
			level,
		), zap.AddCaller(), zap.AddCallerSkip(2))
	} else {
		logger = zap.New(zapcore.NewCore(
			zapcore.NewJSONEncoder(config),
			os.Stdout,
			level,
		), zap.AddCaller(), zap.AddCallerSkip(1))
		loggerSkip = zap.New(zapcore.NewCore(
			zapcore.NewJSONEncoder(config),
			os.Stdout,
			level,
		), zap.AddCaller(), zap.AddCallerSkip(2))
	}

	defer logger.Sync()
	defer loggerSkip.Sync()
	sugar = logger.Sugar()
	sugarCallerSkip1 = loggerSkip.Sugar()
}

// SetNamespace sets the namespace and subsystem for the logger metrics.
// This should be called before any logging is done.
// Namespace is the first part of the metric name, and subsystem is the second.
// Namespace should be your application name, and subsystem should be the
// component of your application that is doing the logging.
func SetNamespace(namespace, subsystem string) {
	logCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "logs_total",
			Help:      "Number of logs emitted with a type label",
			Namespace: namespace,
			Subsystem: subsystem,
		},
		[]string{"type"},
	)

}

func Debug(msg string, keysAndValues ...interface{}) {
	sugar.Debugw(msg, keysAndValues...)
	if debugmode {
		logCounter.WithLabelValues("Debug").Inc()
	}
}

func Info(msg string, keysAndValues ...interface{}) {
	sugar.Infow(msg, keysAndValues...)
	logCounter.WithLabelValues("Info").Inc()
}

func Error(msg string, keysAndValues ...interface{}) {
	sugar.Errorw(msg, keysAndValues...)
	logCounter.WithLabelValues("Error").Inc()
}

func ErrorSkipOne(msg string, keysAndValues ...interface{}) {
	sugarCallerSkip1.Errorw(msg, keysAndValues...)
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
