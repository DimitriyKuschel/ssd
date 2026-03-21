package providers

import (
	"fmt"
	"github.com/rs/zerolog"
	"os"
	"path/filepath"
	"ssd/internal/structures"
)

type TypeEnum string

const (
	TypeGet  TypeEnum = "get"
	TypePost          = "post"
	TypeApp           = "app"
)

type Level string

const (
	TraceLevel Level = "trace"
	DebugLevel       = "debug"
	InfoLevel        = "info"
	WarnLevel        = "warn"
	ErrorLevel       = "error"
	FatalLevel       = "fatal"
	PanicLevel       = "panic"
)

type Logger interface {
	Errorf(t TypeEnum, format string, args ...any)
	Warnf(t TypeEnum, format string, args ...any)
	Debugf(t TypeEnum, format string, args ...any)
	Infof(t TypeEnum, format string, args ...any)
	Fatalf(t TypeEnum, format string, args ...any)
	Close()
}

func GetLogTypeByRequestType(rType string) TypeEnum {
	lType := TypeGet
	if rType == "POST" {
		lType = TypePost
	}
	return lType
}

func NewLogProvider(conf *structures.Config) (Logger, error) {
	log := Zerolog{conf: conf}
	err := log.init()
	if err != nil {
		return nil, err
	}
	return &log, nil
}

type Type struct {
	logger zerolog.Logger
	file   *os.File
}

type Zerolog struct {
	conf    *structures.Config
	loggers map[TypeEnum]Type
}

func (z *Zerolog) init() error {

	level, err := zerolog.ParseLevel(z.conf.Logger.Level)
	if err == nil {
		zerolog.SetGlobalLevel(level)
	}

	if z.conf.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	zerolog.TimeFieldFormat = "02/Jan/2006:15:04:05"

	z.loggers = make(map[TypeEnum]Type)
	for _, t := range []TypeEnum{TypeGet, TypeApp, TypePost} {

		p := filepath.Clean(z.conf.Logger.Dir + "/" + string(t) + ".log")
		file, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.FileMode(z.conf.Logger.Mode))
		if err != nil {
			z.Close()
			return fmt.Errorf("Can`t open/create \"%s\" log file | File [%s] | %v\n", t, p, err)
		}

		var logger zerolog.Logger
		if z.conf.Debug {
			consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
			logger = zerolog.New(zerolog.MultiLevelWriter(zerolog.SyncWriter(consoleWriter), file)).With().Timestamp().Logger()
		} else {
			logger = zerolog.New(file).With().Timestamp().Logger()
		}

		z.loggers[t] = Type{
			logger: logger,
			file:   file,
		}
	}
	return nil
}

func (z *Zerolog) Errorf(t TypeEnum, format string, args ...any) {
	logger := z.loggers[t].logger
	logger.Error().Msgf(format, args...)
}

func (z *Zerolog) Warnf(t TypeEnum, format string, args ...any) {
	logger := z.loggers[t].logger
	z.write(logger.Warn(), format, args...)
}

func (z *Zerolog) Debugf(t TypeEnum, format string, args ...any) {
	logger := z.loggers[t].logger
	z.write(logger.Debug(), format, args...)
}

func (z *Zerolog) Infof(t TypeEnum, format string, args ...any) {
	logger := z.loggers[t].logger
	z.write(logger.Info(), format, args...)
}

func (z *Zerolog) Fatalf(t TypeEnum, format string, args ...any) {
	logger := z.loggers[t].logger
	z.write(logger.Fatal(), format, args...)
}

func (z *Zerolog) write(event *zerolog.Event, format string, args ...any) {
	if len(args) == 0 {
		event.Msg(format)
		return
	}
	event.Msgf(format, args...)
}

func (z *Zerolog) Close() {
	for _, logger := range z.loggers {
		err := logger.file.Close()
		if err != nil {
			fmt.Printf("error via file %s close %s\n", logger.file.Name(), err)
		}
	}
}
