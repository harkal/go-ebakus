// Copyright 2019 The ebakus/go-ebakus Authors
// This file is part of the ebakus/go-ebakus library.
//
// The ebakus/go-ebakus library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The ebakus/go-ebakus library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the ebakus/go-ebakus library. If not, see <http://www.gnu.org/licenses/>.

// Package testlog provides a log handler for unit tests.
package testlog

import (
	"testing"

	"github.com/ebakus/go-ebakus/log"
)

// Logger returns a logger which logs to the unit test log of t.
func Logger(t *testing.T, level log.Lvl) log.Logger {
	l := log.New()
	l.SetHandler(Handler(t, level))
	return l
}

// Handler returns a log handler which logs to the unit test log of t.
func Handler(t *testing.T, level log.Lvl) log.Handler {
	return log.LvlFilterHandler(level, &handler{t, log.TerminalFormat(false)})
}

type handler struct {
	t   *testing.T
	fmt log.Format
}

func (h *handler) Log(r *log.Record) error {
	h.t.Logf("%s", h.fmt.Format(r))
	return nil
}
