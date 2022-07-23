package logging

/*
   This is a modified zerolog.ConsoleWriter that handles specification the
   "exception" field. The exception field will always be output last, and
   if a sentry-json encoded stack trace is dectected, it will be formatted
   on multiple lines, so a human can read it easily.
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
)

// ExceptionFieldName is the field name used for exception fields.
// In our case, it should be a sentry-formatted exception built from a panic
const ExceptionFieldName = "exception"

const (
	colorBlack = iota + 30 //nolint:deadcode,varcheck
	colorRed
	colorGreen
	colorYellow
	colorBlue //nolint:deadcode,varcheck
	colorMagenta
	colorCyan
	colorWhite //nolint:deadcode,varcheck

	colorBold     = 1
	colorDarkGray = 90
)

var (
	consoleBufPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 100))
		},
	}
)

const (
	consoleDefaultTimeFormat = time.Kitchen
)

// Formatter transforms the input into a formatted string.
type Formatter func(interface{}) string

// ConsoleWriter parses the JSON input and writes it in an
// (optionally) colorized, human-friendly format to Out.
type ConsoleWriter struct {
	// Out is the output destination.
	Out io.Writer

	// NoColor disables the colorized output.
	NoColor bool

	// TimeFormat specifies the format for timestamp in output.
	TimeFormat string

	// PartsOrder defines the order of parts in output.
	PartsOrder []string

	FormatTimestamp     Formatter
	FormatLevel         Formatter
	FormatCaller        Formatter
	FormatMessage       Formatter
	FormatFieldName     Formatter
	FormatFieldValue    Formatter
	FormatErrFieldName  Formatter
	FormatErrFieldValue Formatter
	FormatExcFieldName  Formatter
	FormatExcFieldValue Formatter
}

// NewConsoleWriter creates and initializes a new ConsoleWriter.
func NewConsoleWriter(options ...func(w *ConsoleWriter)) ConsoleWriter {
	w := ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: consoleDefaultTimeFormat,
		PartsOrder: consoleDefaultPartsOrder(),
	}

	for _, opt := range options {
		opt(&w)
	}

	return w
}

// Write transforms the JSON input with formatters and appends to w.Out.
func (w ConsoleWriter) Write(p []byte) (n int, err error) {
	if w.PartsOrder == nil {
		w.PartsOrder = consoleDefaultPartsOrder()
	}

	var buf = consoleBufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		consoleBufPool.Put(buf)
	}()

	var evt map[string]interface{}
	p = decodeIfBinaryToBytes(p)
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()
	err = d.Decode(&evt)
	if err != nil {
		return n, fmt.Errorf("cannot decode event: %w", err)
	}

	for _, p := range w.PartsOrder {
		w.writePart(buf, evt, p)
	}

	w.writeFields(evt, buf)

	err = buf.WriteByte('\n')
	if err != nil {
		return n, err
	}
	_, err = buf.WriteTo(w.Out)
	return len(p), err
}

// writeFields appends formatted key-value pairs to buf.
func (w ConsoleWriter) writeFields(evt map[string]interface{}, buf *bytes.Buffer) {
	var fields = make([]string, 0, len(evt))
	for field := range evt {
		switch field {
		case zerolog.LevelFieldName, zerolog.TimestampFieldName, zerolog.MessageFieldName, zerolog.CallerFieldName:
			continue
		}
		fields = append(fields, field)
	}
	sort.Strings(fields)

	if len(fields) > 0 {
		buf.WriteByte(' ')
	}

	// Move the "error" field to the front, and the "exception" field to the back
	erri := sort.Search(len(fields), func(i int) bool { return fields[i] >= zerolog.ErrorFieldName })
	exci := sort.Search(len(fields), func(i int) bool { return fields[i] >= zerolog.ErrorFieldName })
	hasErr := erri < len(fields) && fields[erri] == zerolog.ErrorFieldName
	hasExc := exci < len(fields) && fields[exci] == ExceptionFieldName

	var xfields = make([]string, 0, len(fields))
	if hasErr {
		xfields = append(xfields, zerolog.ErrorFieldName)
	}
	for i, field := range fields {
		if hasErr && erri == i {
			continue
		}
		if hasExc && exci == i {
			continue
		}
		xfields = append(xfields, field)
	}
	if hasExc {
		xfields = append(xfields, ExceptionFieldName)
	}
	fields = xfields

	for i, field := range fields {
		var fn Formatter
		var fv Formatter

		if field == zerolog.ErrorFieldName { // nolint:gocritic
			if w.FormatErrFieldName == nil {
				fn = consoleDefaultFormatErrFieldName(w.NoColor)
			} else {
				fn = w.FormatErrFieldName
			}

			if w.FormatErrFieldValue == nil {
				fv = consoleDefaultFormatErrFieldValue(w.NoColor)
			} else {
				fv = w.FormatErrFieldValue
			}
		} else if field == ExceptionFieldName {
			if w.FormatExcFieldName == nil {
				fn = consoleDefaultFormatExcFieldName(w.NoColor)
			} else {
				fn = w.FormatExcFieldName
			}

			if w.FormatExcFieldValue == nil {
				fv = consoleDefaultFormatExcFieldValue(w.NoColor)
			} else {
				fv = w.FormatExcFieldValue
			}
		} else {
			if w.FormatFieldName == nil {
				fn = consoleDefaultFormatFieldName(w.NoColor)
			} else {
				fn = w.FormatFieldName
			}

			if w.FormatFieldValue == nil {
				fv = consoleDefaultFormatFieldValue
			} else {
				fv = w.FormatFieldValue
			}
		}

		buf.WriteString(fn(field))

		switch fValue := evt[field].(type) {
		case string:
			if needsQuote(fValue) {
				buf.WriteString(fv(strconv.Quote(fValue)))
			} else {
				buf.WriteString(fv(fValue))
			}
		case json.Number:
			buf.WriteString(fv(fValue))
		default:
			b, err := json.Marshal(fValue)
			if err != nil {
				fmt.Fprintf(buf, colorize("[error: %v]", colorRed, w.NoColor), err)
			} else {
				fmt.Fprint(buf, fv(b))
			}
		}

		if i < len(fields)-1 { // Skip space for last field
			buf.WriteByte(' ')
		}
	}
}

// writePart appends a formatted part to buf.
func (w ConsoleWriter) writePart(buf *bytes.Buffer, evt map[string]interface{}, p string) {
	var f Formatter

	switch p {
	case zerolog.LevelFieldName:
		if w.FormatLevel == nil {
			f = consoleDefaultFormatLevel(w.NoColor)
		} else {
			f = w.FormatLevel
		}
	case zerolog.TimestampFieldName:
		if w.FormatTimestamp == nil {
			f = consoleDefaultFormatTimestamp(w.TimeFormat, w.NoColor)
		} else {
			f = w.FormatTimestamp
		}
	case zerolog.MessageFieldName:
		if w.FormatMessage == nil {
			f = consoleDefaultFormatMessage
		} else {
			f = w.FormatMessage
		}
	case zerolog.CallerFieldName:
		if w.FormatCaller == nil {
			f = consoleDefaultFormatCaller(w.NoColor)
		} else {
			f = w.FormatCaller
		}
	default:
		if w.FormatFieldValue == nil {
			f = consoleDefaultFormatFieldValue
		} else {
			f = w.FormatFieldValue
		}
	}

	var s = f(evt[p])

	if len(s) > 0 {
		buf.WriteString(s)
		if p != w.PartsOrder[len(w.PartsOrder)-1] { // Skip space for last part
			buf.WriteByte(' ')
		}
	}
}

// needsQuote returns true when the string s should be quoted in output.
func needsQuote(s string) bool {
	for i := range s {
		if s[i] < 0x20 || s[i] > 0x7e || s[i] == ' ' || s[i] == '\\' || s[i] == '"' {
			return true
		}
	}
	return false
}

// colorize returns the string s wrapped in ANSI code c, unless disabled is true.
func colorize(s interface{}, c int, disabled bool) string {
	if disabled {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}

// ----- DEFAULT FORMATTERS ---------------------------------------------------

func consoleDefaultPartsOrder() []string {
	return []string{
		zerolog.TimestampFieldName,
		zerolog.LevelFieldName,
		zerolog.CallerFieldName,
		zerolog.MessageFieldName,
	}
}

func consoleDefaultFormatTimestamp(timeFormat string, noColor bool) Formatter {
	if timeFormat == "" {
		timeFormat = consoleDefaultTimeFormat
	}
	return func(i interface{}) string {
		t := "<nil>"
		switch tt := i.(type) {
		case string:
			ts, err := time.Parse(zerolog.TimeFieldFormat, tt)
			if err != nil {
				t = tt
			} else {
				t = ts.Format(timeFormat)
			}
		case json.Number:
			i, err := tt.Int64()
			if err != nil {
				t = tt.String()
			} else {
				var sec, nsec int64 = i, 0
				switch zerolog.TimeFieldFormat {
				case zerolog.TimeFormatUnixMs:
					nsec = int64(time.Duration(i) * time.Millisecond)
					sec = 0
				case zerolog.TimeFormatUnixMicro:
					nsec = int64(time.Duration(i) * time.Microsecond)
					sec = 0
				}
				ts := time.Unix(sec, nsec).UTC()
				t = ts.Format(timeFormat)
			}
		}
		return colorize(t, colorDarkGray, noColor)
	}
}

func consoleDefaultFormatLevel(noColor bool) Formatter {
	return func(i interface{}) string {
		var l string
		if ll, ok := i.(string); ok {
			switch ll {
			case "trace":
				l = colorize("TRC", colorMagenta, noColor)
			case "debug":
				l = colorize("DBG", colorYellow, noColor)
			case "info":
				l = colorize("INF", colorGreen, noColor)
			case "warn":
				l = colorize("WRN", colorRed, noColor)
			case "error":
				l = colorize(colorize("ERR", colorRed, noColor), colorBold, noColor)
			case "fatal":
				l = colorize(colorize("FTL", colorRed, noColor), colorBold, noColor)
			case "panic":
				l = colorize(colorize("PNC", colorRed, noColor), colorBold, noColor)
			default:
				l = colorize("???", colorBold, noColor)
			}
		} else {
			if i == nil {
				l = colorize("???", colorBold, noColor)
			} else {
				l = strings.ToUpper(fmt.Sprintf("%s", i))[0:3]
			}
		}
		return l
	}
}

func consoleDefaultFormatCaller(noColor bool) Formatter {
	return func(i interface{}) string {
		var c string
		if cc, ok := i.(string); ok {
			c = cc
		}
		if len(c) > 0 {
			cwd, err := os.Getwd()
			if err == nil {
				c = strings.TrimPrefix(c, cwd)
				c = strings.TrimPrefix(c, "/")
			}
			c = colorize(c, colorBold, noColor) + colorize(" >", colorCyan, noColor)
		}
		return c
	}
}

func consoleDefaultFormatMessage(i interface{}) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%s", i)
}

func consoleDefaultFormatFieldName(noColor bool) Formatter {
	return func(i interface{}) string {
		return colorize(fmt.Sprintf("%s=", i), colorCyan, noColor)
	}
}

func consoleDefaultFormatFieldValue(i interface{}) string {
	return fmt.Sprintf("%s", i)
}

func consoleDefaultFormatErrFieldName(noColor bool) Formatter {
	return func(i interface{}) string {
		return colorize(fmt.Sprintf("%s=", i), colorRed, noColor)
	}
}

func consoleDefaultFormatErrFieldValue(noColor bool) Formatter {
	return func(i interface{}) string {
		return colorize(fmt.Sprintf("%s", i), colorRed, noColor)
	}
}

func consoleDefaultFormatExcFieldName(noColor bool) Formatter {
	return func(i interface{}) string {
		return colorize(fmt.Sprintf("%s=", i), colorRed, noColor)
	}
}

func consoleDefaultFormatExcFieldValue(noColor bool) Formatter {
	return func(i interface{}) string {
		if b, ok := i.([]byte); ok {
			var e sentry.Exception
			if err := json.Unmarshal(b, &e); err == nil {
				s := ""
				if e.Value != "" {
					s += e.Value
				}
				if e.Type != "" {
					s += "(" + e.Type + ")"
				}
				s += "\n"
				if e.Stacktrace != nil {
					for _, frame := range e.Stacktrace.Frames {
						s += PrintFrame(frame)
					}
					return s
				}
			}
		}
		return colorize(fmt.Sprintf("%s", i), colorRed, noColor)
	}
}

func decodeIfBinaryToBytes(in []byte) []byte {
	return in
}

// PrintFrame prints a sentry frame in a go stack-like manner
func PrintFrame(frame sentry.Frame) string {
	return fmt.Sprintf("%s.%s\n    %s:%d\n", frame.Module, frame.Function, frame.AbsPath, frame.Lineno)
}
