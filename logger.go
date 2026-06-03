package logger

// This is a logging module that enforces structured logging and emits prom metrics.

import (
	"io"
	"os"
	"sync"
	"time"

	zaplogfmt "github.com/sykesm/zap-logfmt"
	// zap json

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	sugar            *zap.SugaredLogger
	sugarCallerSkip1 *zap.SugaredLogger
	sugarDisk        *zap.SugaredLogger
	sugarDiskSkipOne *zap.SugaredLogger
	logToDisk        bool
	debugmode        bool
	logCounter       = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logger_logs_total",
			Help: "Number of logs emitted with a type label",
		},
		[]string{"type"},
	)
	broadcast = &broadcastSyncer{}
)

// broadcastSyncer is a zapcore.WriteSyncer that fans out writes to registered
// io.Writer subscribers. It is safe for concurrent use.
type broadcastSyncer struct {
	mu          sync.RWMutex
	subscribers map[int]io.Writer
	nextID      int
}

// Write sends p to all registered subscribers. Errors from individual
// subscribers are silently ignored so that a slow or broken subscriber
// cannot disrupt the main log pipeline.
func (b *broadcastSyncer) Write(p []byte) (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, w := range b.subscribers {
		w.Write(p) //nolint:errcheck
	}
	return len(p), nil
}

// Sync is a no-op; subscribers are responsible for their own flushing.
func (b *broadcastSyncer) Sync() error { return nil }

// RegisterWriter adds w as a log subscriber. Every log line written by the
// logger will be copied to w as JSON. The returned function removes the
// subscription when called.
func RegisterWriter(w io.Writer) func() {
	broadcast.mu.Lock()
	defer broadcast.mu.Unlock()
	if broadcast.subscribers == nil {
		broadcast.subscribers = make(map[int]io.Writer)
	}
	id := broadcast.nextID
	broadcast.nextID++
	broadcast.subscribers[id] = w
	return func() {
		broadcast.mu.Lock()
		defer broadcast.mu.Unlock()
		delete(broadcast.subscribers, id)
	}
}

func init() {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(time.RFC3339Nano))
	}
	// config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	level := zapcore.InfoLevel
	if os.Getenv("VERBOSE") != "" {
		level = zapcore.DebugLevel
		debugmode = true
	}
	var encoder zapcore.Encoder

	if os.Getenv("PRODUCTION") == "" {
		encoder = zaplogfmt.NewEncoder(config)
	} else {
		encoder = zapcore.NewJSONEncoder(config)
	}

	// JSON encoder for the broadcast syncer — subscribers always receive JSON.
	broadcastEncoder := zapcore.NewJSONEncoder(config)
	broadcastCore := zapcore.NewCore(broadcastEncoder, broadcast, zapcore.DebugLevel)

	stdoutCore := zapcore.NewCore(encoder, os.Stdout, level)
	stdoutCoreSkip := zapcore.NewCore(encoder, os.Stdout, level)

	logger := zap.New(zapcore.NewTee(stdoutCore, broadcastCore), zap.AddCaller(), zap.AddCallerSkip(1))
	loggerSkip := zap.New(zapcore.NewTee(stdoutCoreSkip, broadcastCore), zap.AddCaller(), zap.AddCallerSkip(2))

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

func SetLogToDisk(filename string) {
	logToDisk = true
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(time.RFC3339Nano))
	}
	level := zapcore.InfoLevel
	if os.Getenv("VERBOSE") != "" {
		level = zapcore.DebugLevel
		debugmode = true
	}
	// set up the WriteSyncer for the file
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		sugar.Fatalw("Failed to open log file", "error", err)
	}
	// create a zapcore.WriteSyncer that writes to the file
	fileSyncer := zapcore.AddSync(file)
	// create a zapcore.Core that writes to the file

	var encoder zapcore.Encoder
	if os.Getenv("PRODUCTION") == "" {
		encoder = zaplogfmt.NewEncoder(config)
	} else {
		encoder = zapcore.NewJSONEncoder(config)
	}

	diskLogger := zap.New(zapcore.NewCore(
		encoder,
		fileSyncer,
		level,
	), zap.AddCaller(), zap.AddCallerSkip(1))
	sugarDisk = diskLogger.Sugar()

	diskLoggerSkip := zap.New(zapcore.NewCore(
		zaplogfmt.NewEncoder(config),
		fileSyncer,
		level,
	), zap.AddCaller(), zap.AddCallerSkip(2))
	sugarDiskSkipOne = diskLoggerSkip.Sugar()
}

func Debug(msg string, keysAndValues ...interface{}) {
	sugar.Debugw(msg, keysAndValues...)
	if debugmode {
		logCounter.WithLabelValues("Debug").Inc()
	}
	if logToDisk {
		sugarDisk.Debugw(msg, keysAndValues...)
	}
}

func DebugSkipOne(msg string, keysAndValues ...interface{}) {
	sugarCallerSkip1.Debugw(msg, keysAndValues...)
	if debugmode {
		logCounter.WithLabelValues("Debug").Inc()
	}
	if logToDisk {
		sugarDiskSkipOne.Debugw(msg, keysAndValues...)
	}
}

func Info(msg string, keysAndValues ...interface{}) {
	sugar.Infow(msg, keysAndValues...)
	logCounter.WithLabelValues("Info").Inc()
	if logToDisk {
		sugarDisk.Infow(msg, keysAndValues...)
	}
}

func InfoSkipOne(msg string, keysAndValues ...interface{}) {
	sugarCallerSkip1.Infow(msg, keysAndValues...)
	logCounter.WithLabelValues("Info").Inc()
	if logToDisk {
		sugarDiskSkipOne.Infow(msg, keysAndValues...)
	}
}

func Error(msg string, keysAndValues ...interface{}) {
	sugar.Errorw(msg, keysAndValues...)
	logCounter.WithLabelValues("Error").Inc()
	if logToDisk {
		sugarDisk.Errorw(msg, keysAndValues...)
	}
}

func ErrorSkipOne(msg string, keysAndValues ...interface{}) {
	sugarCallerSkip1.Errorw(msg, keysAndValues...)
	logCounter.WithLabelValues("Error").Inc()
	if logToDisk {
		sugarDiskSkipOne.Errorw(msg, keysAndValues...)
	}
}

func Fatal(msg string, keysAndValues ...interface{}) {
	logCounter.WithLabelValues("Fatal").Inc()
	if logToDisk {
		sugarDisk.Fatalw(msg, keysAndValues...)
	}
	// We're doing this although it's not useful.
	sugar.Fatalw(msg, keysAndValues...)
}

func Sync() {
	sugar.Sync()
}
