// Copyright 2019 The ebakus/node Authors
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

// +build !go1.7,amd64,!gccgo,!appengine

package blake2b

import "golang.org/x/sys/cpu"

func init() {
	useSSE4 = cpu.X86.HasSSE41
}

//go:noescape
func fSSE4(h *[8]uint64, m *[16]uint64, c0, c1 uint64, flag uint64, rounds uint64)

func f(h *[8]uint64, m *[16]uint64, c0, c1 uint64, flag uint64, rounds uint64) {
	if useSSE4 {
		fSSE4(h, m, c0, c1, flag, rounds)
	} else {
		fGeneric(h, m, c0, c1, flag, rounds)
	}
}
