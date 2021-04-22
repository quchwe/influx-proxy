// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import (
	"bytes"
	"io"
	"log"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
)

type Backends struct {
	*HttpBackend
	fb   *FileBackend
	pool *ants.Pool

	flushSize       int
	flushTime       int
	rewriteInterval int
	ticker          *time.Ticker
	ch_write        chan []byte      // nolint:golint
	ch_timer        <-chan time.Time // nolint:golint
	buffer          *bytes.Buffer
	writeCounter    int
	wg              sync.WaitGroup
}

// maybe ch_timer is not the best way.
func NewBackends(cfg *BackendConfig, name string, datadir string) (bs *Backends, err error) {
	bs = &Backends{
		HttpBackend:     NewHttpBackend(cfg),
		flushSize:       cfg.FlushSize,
		flushTime:       cfg.FlushTime,
		rewriteInterval: cfg.RewriteInterval,
		ticker:          time.NewTicker(time.Millisecond * time.Duration(cfg.RewriteInterval)),
		ch_write:        make(chan []byte, 16),
	}
	bs.fb, err = NewFileBackend(name, datadir)
	if err != nil {
		return
	}
	bs.pool, err = ants.NewPool(cfg.ConnPoolSize)
	if err != nil {
		return
	}

	go bs.worker()
	return
}

func (bs *Backends) worker() {
	for {
		select {
		case p, ok := <-bs.ch_write:
			if !ok {
				// closed
				bs.Flush()
				bs.wg.Wait()
				bs.HttpBackend.Close()
				bs.fb.Close()
				return
			}
			bs.WriteBuffer(p)

		case <-bs.ch_timer:
			bs.Flush()

		case <-bs.ticker.C:
			bs.Idle()
		}
	}
}

func (bs *Backends) Write(p []byte) (err error) {
	bs.ch_write <- p
	return
}

func (bs *Backends) Close() (err error) {
	bs.pool.Release()
	close(bs.ch_write)
	return
}

func (bs *Backends) WriteBuffer(p []byte) {
	bs.writeCounter++

	if bs.buffer == nil {
		bs.buffer = &bytes.Buffer{}
	}

	n, err := bs.buffer.Write(p)
	if err != nil {
		log.Printf("error: %s", err)
		return
	}
	if n != len(p) {
		err = io.ErrShortWrite
		log.Printf("error: %s", err)
		return
	}

	if p[len(p)-1] != '\n' {
		err = bs.buffer.WriteByte('\n')
		if err != nil {
			log.Printf("error: %s", err)
			return
		}
	}

	switch {
	case bs.writeCounter >= bs.flushSize:
		bs.Flush()
	case bs.ch_timer == nil:
		bs.ch_timer = time.After(time.Millisecond * time.Duration(bs.flushTime))
	}
}

func (bs *Backends) Flush() {
	if bs.buffer == nil {
		return
	}

	p := bs.buffer.Bytes()
	bs.buffer = nil
	bs.ch_timer = nil
	bs.writeCounter = 0

	if len(p) == 0 {
		return
	}

	bs.wg.Add(1)
	bs.pool.Submit(func() {
		defer bs.wg.Done()
		var buf bytes.Buffer
		err := Compress(&buf, p)
		if err != nil {
			log.Printf("compress error: %s", err)
			return
		}

		p = buf.Bytes()

		// maybe blocked here, run in another goroutine
		if bs.HttpBackend.IsActive() {
			err = bs.HttpBackend.WriteCompressed(p)
			switch err {
			case nil:
				return
			case ErrBadRequest:
				log.Printf("bad request, drop all data")
				return
			case ErrNotFound:
				log.Printf("bad backend, drop all data")
				return
			default:
				log.Printf("write http error: %s, length: %d", bs.HttpBackend.URL, len(p))
			}
		}

		err = bs.fb.Write(p)
		if err != nil {
			log.Printf("write file error: %s", err)
		}
		// don't try to run rewrite loop directly.
		// that need a lock.
	})
}

func (bs *Backends) Idle() {
	if !bs.HttpBackend.IsRewriting() && bs.fb.IsData() {
		bs.HttpBackend.SetRewriting(true)
		go bs.RewriteLoop()
	}

	// TODO: report counter
}

func (bs *Backends) RewriteLoop() {
	for bs.fb.IsData() {
		if !bs.HttpBackend.IsActive() {
			time.Sleep(time.Millisecond * time.Duration(bs.rewriteInterval))
			continue
		}
		err := bs.Rewrite()
		if err != nil {
			time.Sleep(time.Millisecond * time.Duration(bs.rewriteInterval))
			continue
		}
	}
	bs.HttpBackend.SetRewriting(false)
}

func (bs *Backends) Rewrite() (err error) {
	p, err := bs.fb.Read()
	if err != nil {
		return
	}
	if p == nil {
		return
	}

	err = bs.HttpBackend.WriteCompressed(p)

	switch err {
	case nil:
	case ErrBadRequest:
		log.Printf("bad request, drop all data")
		err = nil
	case ErrNotFound:
		log.Printf("bad backend, drop all data")
		err = nil
	default:
		log.Printf("rewrite http error: %s, length: %d", bs.HttpBackend.URL, len(p))

		err = bs.fb.RollbackMeta()
		if err != nil {
			log.Printf("rollback meta error: %s", err)
		}
		return
	}

	err = bs.fb.UpdateMeta()
	if err != nil {
		log.Printf("update meta error: %s", err)
		return
	}
	return
}
