package utils

import (
        "k8s.io/klog/v2"
)

type Logger struct {
}

func NewLogger(prefix string, flags int) *Logger {
        return &Logger{}
}

func (l *Logger) Trace(format string, v ...interface{}) {
        klog.V(4).Infof("[TRACE] "+format, v...)
}

func (l *Logger) Tracef(format string, v ...interface{}) {
        klog.V(4).Infof("[TRACE] "+format, v...)
}

func (l *Logger) Debug(format string, v ...interface{}) {
        klog.V(4).Infof("[DEBUG] "+format, v...)
}

func (l *Logger) Debugf(format string, v ...interface{}) {
        klog.V(4).Infof("[DEBUG] "+format, v...)
}

func (l *Logger) Info(format string, v ...interface{}) {
        klog.Infof("[INFO] "+format, v...)
}

func (l *Logger) Infomation(format string, v ...interface{}) {
        klog.Infof("[INFO] "+format, v...)
}

func (l *Logger) Infof(format string, v ...interface{}) {
        klog.Infof("[INFO] "+format, v...)
}

func (l *Logger) Warn(format string, v ...interface{}) {
        klog.Warningf("[WARN] "+format, v...)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
         klog.Warningf("[WARN] "+format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
        klog.Errorf("[ERROR] "+format, v...)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
        klog.Errorf("[ERROR] "+format, v...)
}

func (l *Logger) Fatal(format string, v ...interface{}) {
        klog.Fatalf("[FATAL] "+format, v...)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
        klog.Fatalf("[FATAL] "+format, v...)
}

func (l *Logger) LogDebug(format string, v ...interface{}) {
        klog.V(4).Infof("[DEBUG] "+format, v...)
}
