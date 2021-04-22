// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import (
	"bytes"
	"os"
	"testing"
)

func readAndProcess(t *testing.T, fb *FileBackend, s string, l int64) {
	p, err := fb.Read()
	if err != nil {
		t.Errorf("error: %s", err)
		return
	}

	if !bytes.Equal(p, []byte(s)) {
		t.Errorf("error: %s", err)
		return
	}

	err = fb.UpdateMeta()
	if err != nil {
		t.Errorf("error: %s", err)
		return
	}

	fi, err := os.Stat("../data/test/testbk.dat")
	if err != nil {
		t.Errorf("error: %s", err)
		return
	}

	if fi.Size() != l {
		t.Errorf("size not match")
		return
	}
}

func TestFileBackend(t *testing.T) {
	fb, err := NewFileBackend("testbk", "../data/test")
	if err != nil {
		t.Errorf("error: %s", err)
		return
	}

	err = fb.Write([]byte("data"))
	if err != nil {
		t.Errorf("error: %s", err)
		return
	}

	err = fb.Write([]byte("full"))
	if err != nil {
		t.Errorf("error: %s", err)
		return
	}

	readAndProcess(t, fb, "data", 16)
	readAndProcess(t, fb, "full", 0)
}
