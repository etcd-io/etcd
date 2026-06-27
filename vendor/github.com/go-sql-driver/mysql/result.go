// Go MySQL Driver - A MySQL-Driver for Go's database/sql package
//
// Copyright 2012 The Go-MySQL-Driver Authors. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package mysql

import "database/sql/driver"

// Result exposes data not available through *connection.Result.
//
// This is accessible by executing statements using sql.Conn.Raw() and
// downcasting the returned result:
//
//	res, err := rawConn.Exec(...)
//	res.(mysql.Result).AllRowsAffected()
type Result interface {
	driver.Result
	// AllRowsAffected returns a slice containing the affected rows for each
	// executed statement.
	AllRowsAffected() []int64
	// AllLastInsertIds returns a slice containing the last inserted ID for each
	// executed statement.
	AllLastInsertIds() []int64
}

type mysqlResult struct {
	// One entry in both slices is created for every executed statement result.
	affectedRows []int64
	insertIds    []int64
}

func (res *mysqlResult) LastInsertId() (int64, error) {
	return res.insertIds[len(res.insertIds)-1], nil
}

func (res *mysqlResult) RowsAffected() (int64, error) {
	return res.affectedRows[len(res.affectedRows)-1], nil
}

func (res *mysqlResult) AllLastInsertIds() []int64 {
	return append([]int64{}, res.insertIds...) // defensive copy
}

func (res *mysqlResult) AllRowsAffected() []int64 {
	return append([]int64{}, res.affectedRows...) // defensive copy
}
