package logger

import (
	"log"
	"os"
)

type Logger struct {
	*log.Logger
}

func New(env string) *Logger {
	prefix := "[starline:" + env + "] "
	return &Logger{Logger: log.New(os.Stdout, prefix, log.LstdFlags|log.Lmicroseconds)}
}

func (l *Logger) Infof(format string, args ...any) {
	l.Printf("INFO "+format, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.Printf("ERROR "+format, args...)
}
