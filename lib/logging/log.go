package logging

import (
	"fmt"
	"io"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
)

// NewOptions creates a new Options struct for obtaining a logger
func NewOptions(log *zerolog.Logger, output io.Writer) (*Options, error) {
	var o Options
	if err := o.Setup(log, output); err != nil {
		return nil, err
	}
	return &o, nil
}

// MustOptions panic if err is not nil
func MustOptions(o *Options, err error) *Options {
	if err != nil {
		panic(err)
	}
	return o
}

// DefaultLogger ...
func DefaultLogger(output io.Writer) zerolog.Logger {
	return zerolog.
		New(output).
		With().
		Timestamp().
		Logger().
		Level(zerolog.WarnLevel)
}

// Options holds the logging options
type Options struct {
	Level   func(string) error `long:"level" env:"LEVEL" ini-name:"log-level" choice:"trace" choice:"debug" choice:"info" choice:"warn" choice:"error" choice:"fatal" choice:"panic" choice:"auto" default:"auto" description:"log level. 'auto' selects 'info' when stdout is a tty, 'error' otherwise."`
	Format  func(string) error `long:"format" env:"FORMAT" ini-name:"log-format" choice:"json" choice:"pretty" choice:"auto" default:"auto" description:"Logs format. 'auto' selects 'pretty' if stdout is a tty."`
	Verbose func()             `short:"v" long:"verbose" no-ini:"t" description:"Increase log verbosity. Can be repeated"`

	logFinalOutput io.Writer                   `no-flag:"t"`
	logOutput      io.Writer                   `no-flag:"t"`
	logWrappers    []func(io.Writer) io.Writer `no-flag:"t"`
	log            *zerolog.Logger             `no-flag:"t"`
}

// Logger returns the latest configured logger
func (o *Options) Logger() zerolog.Logger {
	return *o.log
}

func (o *Options) resetOutput() {
	out := o.logOutput
	for _, wrapper := range o.logWrappers {
		out = wrapper(out)
	}
	*o.log = o.log.Output(out)
}

// AddLogWrapper adds a log wrapper to the stack
func (o *Options) AddLogWrapper(wrapper func(io.Writer) io.Writer) {
	o.logWrappers = append(o.logWrappers, wrapper)
	o.resetOutput()
}

// SetMinLoggingLevel makes sure the logging level is not under a given value
func (o *Options) SetMinLoggingLevel(level zerolog.Level) {
	if level < o.log.GetLevel() {
		*o.log = log.Level(level)
	}
}

// Setup ...
func (o *Options) Setup(log *zerolog.Logger, output io.Writer) error {
	var logLevelAutoLocked = false

	o.logFinalOutput = output

	o.log = log
	*o.log = DefaultLogger(output)

	o.Format = func(format string) error {
		if format == "auto" {
			if outputFile, hasFd := o.logFinalOutput.(interface{ Fd() uintptr }); hasFd && term.IsTerminal(int(outputFile.Fd())) {
				format = "pretty"
			} else {
				format = "json"
			}
		}
		switch format {
		case "pretty":
			o.logOutput = ConsoleWriter{Out: o.logFinalOutput}
		case "json":
			o.logOutput = o.logFinalOutput
		default:
			return fmt.Errorf("invalid log-format: %s", format)
		}
		o.resetOutput()
		return nil
	}
	o.Verbose = func() {
		*o.log = o.log.Level(o.log.GetLevel() - zerolog.Level(1))
	}
	o.Level = func(value string) error {
		if value == "auto" {
			if logLevelAutoLocked {
				// The current call is at best redondant, at worse called by
				// default after some potential --verbose that would be ignored
				return nil
			}
			if outputFile, hasFd := o.logFinalOutput.(interface{ Fd() uintptr }); hasFd && term.IsTerminal(int(outputFile.Fd())) {
				value = "info"
			} else {
				value = "warn"
			}
		}

		level, err := zerolog.ParseLevel(value)
		if err != nil {
			return err
		}
		*o.log = o.log.Level(level)
		return nil
	}
	if err := o.Format("auto"); err != nil {
		return err
	}
	if err := o.Level("auto"); err != nil {
		return err
	}
	logLevelAutoLocked = true
	return nil
}
