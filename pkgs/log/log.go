package mlog

import (
	"go.uber.org/zap"
)

type Logger struct {
	*zap.SugaredLogger
}

func New(name string) *Logger {
	logger := zap.New(NewCore(), zap.AddCaller())
	return &Logger{logger.Sugar().Named(name)}
}
