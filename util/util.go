// Copyright 2022-2023 Tigris Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
