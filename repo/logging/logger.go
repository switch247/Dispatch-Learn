package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

type Logger struct {
	mu     sync.Mutex
	level  Level
	logger *log.Logger
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func Init(level Level, w io.Writer) {
	once.Do(func() {
		if w == nil {
			w = os.Stdout
		}
		defaultLogger = &Logger{
			level:  level,
			logger: log.New(w, "", 0),
		}
	})
}

func Default() *Logger {
	if defaultLogger == nil {
		Init(INFO, os.Stdout)
	}
	return defaultLogger
}

func (l *Logger) log(level Level, module, submodule, msg string) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	ts := time.Now().UTC().Format(time.RFC3339)
	redacted := redactSensitive(msg)

	var prefix string
	if submodule != "" {
		prefix = fmt.Sprintf("%s [%s][%s][%s] ", ts, levelNames[level], module, submodule)
	} else {
		prefix = fmt.Sprintf("%s [%s][%s] ", ts, levelNames[level], module)
	}
	l.logger.Print(prefix + redacted)
}

func (l *Logger) Info(module, submodule, msg string)  { l.log(INFO, module, submodule, msg) }
func (l *Logger) Debug(module, submodule, msg string) { l.log(DEBUG, module, submodule, msg) }
func (l *Logger) Warn(module, submodule, msg string)  { l.log(WARN, module, submodule, msg) }
func (l *Logger) Error(module, submodule, msg string) { l.log(ERROR, module, submodule, msg) }
func (l *Logger) Fatal(module, submodule, msg string) {
	l.log(FATAL, module, submodule, msg)
	os.Exit(1)
}

// Convenience package-level functions
func Info(module, submodule, msg string)  { Default().Info(module, submodule, msg) }
func Debug(module, submodule, msg string) { Default().Debug(module, submodule, msg) }
func Warn(module, submodule, msg string)  { Default().Warn(module, submodule, msg) }
func Error(module, submodule, msg string) { Default().Error(module, submodule, msg) }

var sensitivePatterns = []string{"password", "token", "secret", "ssn", "credit_card"}

func redactSensitive(msg string) string {
	lower := strings.ToLower(msg)
	for _, p := range sensitivePatterns {
		if strings.Contains(lower, p) {
			msg = strings.ReplaceAll(msg, msg, "[REDACTED-SENSITIVE-DATA]")
			return msg
		}
	}
	return msg
}
