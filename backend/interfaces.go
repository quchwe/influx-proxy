// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import "net/http"

type Querier interface {
	Query(w http.ResponseWriter, req *http.Request) (err error)
}

type BackendAPI interface { // nolint:golint
	Querier
	IsActive() (b bool)
	IsRewriting() (b bool)
	IsWriteOnly() (b bool)
	Ping() (version string, err error)
	Write(p []byte) (err error)
	Close() (err error)
	QuerySink(req *http.Request) (qr *QueryResult)
}
