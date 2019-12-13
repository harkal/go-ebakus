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

package abi

import (
	"errors"
	"fmt"
	"reflect"
)

// Table represents a go struct and is used so as to convert data passed from solidity.
// Input specifies the required struct fields.
type Table struct {
	Name   string
	Inputs Arguments
}

// Unpack performs the operation hexdata -> Go format
func (table Table) GetTableInstance() (interface{}, error) {
	var abi2interfaceReflect []reflect.StructField
	for i, arg := range table.Inputs {
		if ToCamelCase(arg.Name) == "" {
			return nil, errors.New("abi: purely anonymous or underscored field is not supported")
		}

		sf := reflect.StructField{
			Name:  ToCamelCase(arg.Name),
			Type:  arg.Type.Type,
			Index: []int{i},
		}
		abi2interfaceReflect = append(abi2interfaceReflect, sf)
	}

	st := reflect.StructOf(abi2interfaceReflect)
	so := reflect.New(st)
	return so.Interface(), nil
}

// Unpack performs the operation hexdata -> Go format
func (table Table) UnpackSingle(v interface{}, field string, data []byte) (interface{}, error) {
	if reflect.Ptr != reflect.ValueOf(v).Kind() {
		return nil, fmt.Errorf("abi: Unpack(non-pointer %T)", v)
	}

	for _, arg := range table.Inputs {
		if arg.Name != field {
			continue
		}

		elem := reflect.ValueOf(v).Elem()

		marshalledValue, err := toGoType(0, arg.Type, data)
		if err != nil {
			return nil, err
		}

		fieldName := ToCamelCase(field)
		if fieldName == "" {
			return nil, errors.New("abi: purely anonymous or underscored field is not supported")
		}

		elem.FieldByName(fieldName).Set(reflect.ValueOf(marshalledValue))

		return marshalledValue, nil
	}

	return nil, nil
}

// Pack performs the operation Go format -> Hexdata
func (table Table) Pack(args ...interface{}) ([]byte, error) {
	v := args[0]

	value := reflect.Indirect(reflect.ValueOf(v))

	// variable input is the output appended at the end of packed
	// output. This is used for strings and bytes types input.
	var variableInput []byte

	// input offset is the bytes offset for packed output
	inputOffset := 0
	for _, input := range table.Inputs {
		inputOffset += getTypeSize(input.Type)
	}

	var ret []byte
	for _, input := range table.Inputs {
		fv := value.FieldByName(input.Name)
		packed, err := input.Type.pack(fv)
		if err != nil {
			return nil, err
		}
		// check for dynamic types
		if isDynamicType(input.Type) {
			// set the offset
			ret = append(ret, packNum(reflect.ValueOf(inputOffset))...)
			// calculate next offset
			inputOffset += len(packed)
			// append to variable input
			variableInput = append(variableInput, packed...)
		} else {
			// append the packed value to the input
			ret = append(ret, packed...)
		}
	}
	// append the variable input at the end of the packed input
	ret = append(ret, variableInput...)

	return ret, nil
}
