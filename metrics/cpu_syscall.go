// Copyright 2018 The ebakus/node Authors
// This file is part of the ebakus/node library.
//
// The ebakus/node library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The ebakus/node library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the ebakus/node library. If not, see <http://www.gnu.org/licenses/>.

// +build !windows

package metrics

import (
	"syscall"

	"github.com/ebakus/go-ebakus/log"
)

// getProcessCPUTime retrieves the process' CPU time since program startup.
func getProcessCPUTime() int64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		log.Warn("Failed to retrieve CPU time", "err", err)
		return 0
	}
	return int64(usage.Utime.Sec+usage.Stime.Sec)*100 + int64(usage.Utime.Usec+usage.Stime.Usec)/10000 //nolint:unconvert
}