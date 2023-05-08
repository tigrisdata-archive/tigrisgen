package util

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
)

func Must[T any](v T, err error) T {
	if err != nil {
		Fatal("%v", err.Error())
	}

	return v
}

// Fatal can be overridden in tests.
var Fatal = FatalDef

func FatalDef(format string, args ...any) {
	log.Error().CallerSkipFrame(1).Msgf(format, args...)
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
