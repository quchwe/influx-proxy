// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import mapset "github.com/deckarep/golang-set"

var SupportCmds = mapset.NewSet(
	"show measurements",
	"show series",
	"show field keys",
	"show tag keys",
	"show tag values",
	"show retention policies",
	"show stats",
	"show databases",
	"create database",
	"delete from",
	"drop series from",
	"drop measurement",
)
