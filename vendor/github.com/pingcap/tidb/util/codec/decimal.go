// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package codec

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/types"
)

// EncodeDecimal encodes a decimal into a byte slice which can be sorted lexicographically later.
func EncodeDecimal(b []byte, dec *types.MyDecimal, precision, frac int) ([]byte, error) {
	if precision == 0 {
		precision, frac = dec.PrecisionAndFrac()
	}
	b = append(b, byte(precision), byte(frac))
	bin, err := dec.ToBin(precision, frac)
	b = append(b, bin...)
	return b, errors.Trace(err)
}

// DecodeDecimal decodes bytes to decimal.
func DecodeDecimal(b []byte) ([]byte, *types.MyDecimal, int, int, error) {
	// gofail: var errorInDecodeDecimal bool
	// if errorInDecodeDecimal {
	//	 return b, nil, 0, 0, errors.New("gofail error")
	// }

	if len(b) < 3 {
		return b, nil, 0, 0, errors.New("insufficient bytes to decode value")
	}
	precision := int(b[0])
	frac := int(b[1])
	b = b[2:]
	dec := new(types.MyDecimal)
	binSize, err := dec.FromBin(b, precision, frac)
	b = b[binSize:]
	if err != nil {
		return b, nil, precision, frac, errors.Trace(err)
	}
	return b, dec, precision, frac, nil
}
