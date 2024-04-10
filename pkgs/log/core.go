package mlog

import (
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type CoreConfig struct {
	OutputType  string
	OutputPath  string
	Level       string
	EncodeType  string
	EncodeColor bool
}

var (
	coreConfigs []CoreConfig
)

func SetOutputTypes(configs ...CoreConfig) {
	coreConfigs = append(coreConfigs, configs...)
}

func NewCore() zapcore.Core {
	cores := make([]zapcore.Core, 0, len(coreConfigs))
	for _, cfg := range coreConfigs {
		var core zapcore.Core
		switch cfg.OutputType {
		case "file":
			core = FileCore(cfg)
		case "console":
			core = ConsoleCore(cfg)
		}

		if core != nil {
			cores = append(cores, core)
		}
	}

	if len(cores) == 0 {
		cores = append(cores, ConsoleCore(CoreConfig{EncodeColor: true}))
	}
	return zapcore.NewTee(cores...)
}

func ConsoleCore(cfg CoreConfig) zapcore.Core {
	encoderConfig := zapcore.EncoderConfig{
		// Keys can be anything except the empty string.
		TimeKey:          "T",
		LevelKey:         "L",
		NameKey:          "N",
		CallerKey:        "C",
		FunctionKey:      zapcore.OmitKey,
		MessageKey:       "M",
		StacktraceKey:    "S",
		EncodeTime:       zapcore.RFC3339TimeEncoder,
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     zapcore.ShortCallerEncoder,
		ConsoleSeparator: "\t",
	}
	if cfg.EncodeColor {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	level, err := zap.ParseAtomicLevel(cfg.Level)
	if err != nil {
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	switch cfg.EncodeType {
	case "json":
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	out := "stdout"
	if strings.ToLower(cfg.OutputPath) == "stderr" {
		out = "stderr"
	}
	writer, closeFn, err := zap.Open(out)
	if err != nil {
		closeFn()
		return nil
	}
	return zapcore.NewCore(encoder, writer, level)
}

func FileCore(cfg CoreConfig) zapcore.Core {
	encoderConfig := zapcore.EncoderConfig{
		// Keys can be anything except the empty string.
		TimeKey:          "T",
		LevelKey:         "L",
		NameKey:          "N",
		CallerKey:        "",
		FunctionKey:      zapcore.OmitKey,
		MessageKey:       "M",
		StacktraceKey:    "S",
		EncodeTime:       zapcore.RFC3339TimeEncoder,
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     nil,
		ConsoleSeparator: "\t",
	}

	if cfg.EncodeColor {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	level, err := zap.ParseAtomicLevel(cfg.Level)
	if err != nil {
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	switch cfg.EncodeType {
	case "json":
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	os.MkdirAll(filepath.Dir(cfg.OutputPath), 0600)
	writer, _, err := zap.Open(cfg.OutputPath)
	if err != nil {
		return nil
	}
	return zapcore.NewCore(encoder, writer, level)
}
