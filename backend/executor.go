// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import (
	"errors"
	"math/rand"
	"net/http"
)

var (
	ErrEmptyQuery          = errors.New("empty query")
	ErrGetBucket           = errors.New("can't get bucket")
	ErrGetMeasurement      = errors.New("can't get measurement")
	ErrBackendsUnavailable = errors.New("backends unavailable")
	ErrGetBackends         = errors.New("can't get backends")
)

func QueryWithBucketMeasurement(w http.ResponseWriter, req *http.Request, ip *Proxy, org, bucket, meas string) (err error) {
	// all circles -> backend by key(org,bucket,meas) -> query
	key := GetKey(org, bucket, meas)

	// pass non-active, rewriting or write-only.
	perms := rand.Perm(len(ip.Circles))
	for _, p := range perms {
		be := ip.Circles[p].GetBackend(key)
		if !be.IsActive() || be.IsRewriting() || be.IsWriteOnly() {
			continue
		}
		err = be.Query(req, w)
		if err == nil {
			return
		}
	}

	// pass non-active, non-writing (excluding rewriting and write-only).
	backends := ip.GetBackends(key)
	for _, be := range backends {
		if !be.IsActive() || !(be.IsRewriting() || be.IsWriteOnly()) {
			continue
		}
		err = be.Query(req, w)
		if err == nil {
			return
		}
	}

	if err != nil {
		return err
	}
	return ErrBackendsUnavailable
}
