// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package f64 implements float64 vector and matrix types.
package f64 // import "golang.org/x/image/math/f64"

// Vec2 is a 2-element vector.
type Vec2 [2]float64

// Vec3 is a 3-element vector.
type Vec3 [3]float64

// Vec4 is a 4-element vector.
type Vec4 [4]float64

// Mat3 is a 3x3 matrix in row major order.
//
// m[3*r + c] is the element in the r'th row and c'th column.
type Mat3 [9]float64

// Mat4 is a 4x4 matrix in row major order.
//
// m[4*r + c] is the element in the r'th row and c'th column.
type Mat4 [16]float64

// Aff3 is a 3x3 affine transformation matrix in row major order, where the
// bottom row is implicitly [0 0 1].
//
// m[3*r + c] is the element in the r'th row and c'th column.
type Aff3 [6]float64

// Aff4 is a 4x4 affine transformation matrix in row major order, where the
// bottom row is implicitly [0 0 0 1].
//
// m[4*r + c] is the element in the r'th row and c'th column.
type Aff4 [12]float64
