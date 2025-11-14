package utils

import (
	"log"
	"os"
)

type Logger struct {
	logger *log.Logger
}

func NewLogger(prefix string, flags int) *Logger {
	return &Logger{
		logger: log.New(os.Stderr, prefix, flags),
	}
}

func (l *Logger) Trace(format string, v ...interface{}) {
	l.logger.Printf("[TRACE] "+format, v...)
}

func (l *Logger) Tracef(format string, v ...interface{}) {
	l.logger.Printf("[TRACE] "+format, v...)
}

func (l *Logger) Debug(format string, v ...interface{}) {
	l.logger.Printf("[DEBUG] "+format, v...)
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	l.logger.Printf("[DEBUG] "+format, v...)
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.logger.Printf("[INFO] "+format, v...)
}

func (l *Logger) Infomation(format string, v ...interface{}) {
	l.logger.Printf("[INFO] "+format, v...)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.logger.Printf("[INFO] "+format, v...)
}

func (l *Logger) Warn(format string, v ...interface{}) {
	l.logger.Printf("[WARN] "+format, v...)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.logger.Printf("[WARN] "+format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.logger.Printf("[ERROR] "+format, v...)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.logger.Printf("[ERROR] "+format, v...)
}

func (l *Logger) Fatal(format string, v ...interface{}) {
	l.logger.Fatalf("[FATAL] "+format, v...)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.logger.Printf("[FATAL] "+format, v...)
}

func (l *Logger) LogDebug(format string, v ...interface{}) {
	l.logger.Printf("[DEBUG] "+format, v...)
}
