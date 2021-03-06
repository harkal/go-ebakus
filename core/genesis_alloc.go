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

package core

// Constants containing the genesis allocation of built-in genesis blocks.
// Their content is an RLP-encoded list of (address, balance) tuples.
// Use mkalloc.go to create/update them.

// nolint: misspell
const mainnetAllocData = "\xf9\x016\xc2\x01\x01\xc2\x02\x01\xc2\x03\x01\xc2\x04\x01\xc2\x05\x01\xc2\x06\x01\xc2\a\x01\xc2\b\x01\xc2\t\x01\u0102\x01\x01\x01\u0102\x01\x02\x01\xe1\x94\x01!{\u45e8\x9a\xd5.b\xcaW\r\x13\xedi^\xec\x8a\u007f\x8b\x1axCy\u065d\xb4 \x00\x00\x00\xe1\x94\x1d\f\xf2\xb3G\x10\xc1\x1a^\xbf\xe0@\xe1_\xfen#:\xb0\u018b$e\\\u01cb8\u05ec\x00\x00\x00\xe2\x94\x1d\xd8;)\xa6D\t7\xab4\xe7\x12\xf90\xf1\x0eG\xa4\xf2\x16\x8c\x02\xc7`\x15j\xb8nH\xdc\x00\x00\x00\xe1\x94Z\xf0\x02\x99/\xb0CV\x06\x04Y\r\x1f{\xa7\xea-\u02f3\xa4\x8b\x1f\x04\xef\x12\xcb\x04\xcf\x15\x80\x00\x00\xe1\x94s_\xe5\xa3\xfb\xe3zM}\xcet\xe1\x81\xfc'q\xa6\xd1\xea\u064b\v\x94\x9d\x85O4\xfe\xce\x00\x00\x00\xe1\x94\xc8;9\xb2F\x1b5\xd3z\xa3\x8a\xe4r\xe8\xf8\x04\xd9|N\xaf\x8b\x01\b\xb2\xa2\u0080)\t@\x00\x00\xe1\x94\xcd\\\xad\u007f'\xbc\xc5\ahy8S\xbf\x178\xae\xe4\x83az\x8b\bE\x95\x16\x14\x01HJ\x00\x00\x00\xe1\x94\xd1Pr\x84\xfa\xf14\xf6\xc3(S3\b\x1eR\x03(\xf0\xa9\u008b\x01\b\xb2\xa2\u0080)\t@\x00\x00"
const testnetAllocData = "\xf8E\xc2\x01\x01\xc2\x02\x01\xc2\x03\x01\xc2\x04\x01\xc2\x05\x01\xc2\x06\x01\xc2\a\x01\xc2\b\x01\xc2\t\x01\u0102\x01\x01\x01\u0102\x01\x02\x01\u07d4\xd5=\u70f1/\x12\xb7\x85#H\xf2\xe3,!\x99\aF\xad\x02\x8965\u026d\xc5\u07a0\x00\x00"
