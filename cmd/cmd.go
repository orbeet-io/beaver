package cmd

import (
	"fmt"
	"os"

	"github.com/orus-io/go-flags"

	beaver "orus.io/orus-io/beaver/lib"
	"orus.io/orus-io/beaver/lib/logging"
)

var (
	Version   = beaver.Version()
	CommitSha = beaver.CommitSha()
	BuildDate = beaver.BuildDate()
)

var (
	Logger         = logging.DefaultLogger(os.Stdout)
	LoggingOptions = logging.MustOptions(logging.NewOptions(&Logger, os.Stdout))

	parser = flags.NewNamedParser("beaver", flags.HelpFlag|flags.PassDoubleDash)
)

func Run() int {
	if _, err := parser.Parse(); err != nil {
		code := 1

		if fe, ok := err.(*flags.Error); ok { //nolint:errorlint
			if fe.Type == flags.ErrHelp {
				code = 0
				// this error actually contains a help message for the user
				// so we print it on the console
				fmt.Println(err)
			} else {
				Logger.Error().Msg(err.Error())
			}
		} else {
			Logger.Err(err).Msg("")
		}

		return code
	}

	return 0
}
