// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package service

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/chengshiwen/influx-proxy/backend"
	"github.com/chengshiwen/influx-proxy/util"
)

type HttpService struct { // nolint:golint
	ip           *backend.Proxy
	token        string
	writeTracing bool
	queryTracing bool
}

func NewHttpService(cfg *backend.ProxyConfig) (hs *HttpService) { // nolint:golint
	ip := backend.NewProxy(cfg)
	hs = &HttpService{
		ip:           ip,
		token:        cfg.Token,
		writeTracing: cfg.WriteTracing,
		queryTracing: cfg.QueryTracing,
	}
	return
}

func (hs *HttpService) Register(mux *http.ServeMux) {
	mux.HandleFunc("/ping", hs.HandlerPing)
	mux.HandleFunc("/api/v2/query", hs.HandlerQuery)
	mux.HandleFunc("/api/v2/write", hs.HandlerWrite)
	mux.HandleFunc("/health", hs.HandlerHealth)
	mux.HandleFunc("/replica", hs.HandlerReplica)
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
}

func (hs *HttpService) HandlerPing(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	hs.WriteHeader(w, 204)
}

func (hs *HttpService) HandlerQuery(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	if !hs.checkMethodAndAuth(w, req, "POST") {
		return
	}

	// use org, ignore orgID
	org := req.URL.Query().Get("org")
	if org == "" {
		hs.WriteError(w, req, 400, "org not found")
		return
	}

	var contentType = "application/json"
	if ct := req.Header.Get("Content-Type"); ct != "" {
		contentType = ct
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		hs.WriteError(w, req, 400, err.Error())
		return
	}
	rbody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		hs.WriteError(w, req, 400, err.Error())
		return
	}
	var query string
	switch mt {
	case "application/vnd.flux":
		query = string(rbody)
	case "application/json":
		fallthrough
	default:
		var r struct {
			Query string `json:"query"`
		}
		if err = json.Unmarshal(rbody, &r); err != nil {
			hs.WriteError(w, req, 400, fmt.Sprintf("failed parsing request body as JSON; if sending a raw Flux script, set 'Content-Type: application/vnd.flux' in your request headers: %s", err))
			return
		}
		query = r.Query
	}

	req.Body = ioutil.NopCloser(bytes.NewBuffer(rbody))
	err = hs.ip.Query(w, req, org, query)
	if err != nil {
		log.Printf("query error: %s, query: %s %s %s, client: %s", err, req.Method, org, query, req.RemoteAddr)
		hs.WriteError(w, req, 400, err.Error())
		return
	}
	if hs.queryTracing {
		log.Printf("query: %s %s %s, client: %s", req.Method, org, query, req.RemoteAddr)
	}
}

func (hs *HttpService) HandlerWrite(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	if !hs.checkMethodAndAuth(w, req, "POST") {
		return
	}

	precision := req.URL.Query().Get("precision")
	switch precision {
	case "", "ns", "us", "ms", "s":
		// it's valid
		if precision == "" {
			precision = "ns"
		}
	default:
		hs.WriteError(w, req, 400, fmt.Sprintf("invalid precision %q (use ns, us, ms and s)", precision))
		return
	}

	// use org and bucket, ignore orgID
	org := req.URL.Query().Get("org")
	if org == "" {
		hs.WriteError(w, req, 400, "org not found")
		return
	}
	bucket := req.URL.Query().Get("bucket")
	if bucket == "" {
		hs.WriteError(w, req, 400, "bucket not found")
		return
	}

	body := req.Body
	if req.Header.Get("Content-Encoding") == "gzip" {
		b, err := gzip.NewReader(body)
		if err != nil {
			hs.WriteError(w, req, 400, "unable to decode gzip body")
			return
		}
		defer b.Close()
		body = b
	}
	p, err := ioutil.ReadAll(body)
	if err != nil {
		hs.WriteError(w, req, 400, err.Error())
		return
	}

	err = hs.ip.Write(p, org, bucket, precision)
	if err == nil {
		hs.WriteHeader(w, 204)
	}
	if hs.writeTracing {
		log.Printf("write: %s %s %s %s, client: %s", org, bucket, precision, p, req.RemoteAddr)
	}
}

func (hs *HttpService) HandlerHealth(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	if !hs.checkMethodAndAuth(w, req, "GET") {
		return
	}
	hs.Write(w, req, 200, hs.ip.GetHealth())
}

func (hs *HttpService) HandlerReplica(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	if !hs.checkMethodAndAuth(w, req, "GET") {
		return
	}

	org := req.URL.Query().Get("org")
	bucket := req.URL.Query().Get("bucket")
	meas := req.URL.Query().Get("meas")
	if org != "" && bucket != "" && meas != "" {
		key := backend.GetKey(org, bucket, meas)
		backends := hs.ip.GetBackends(key)
		data := make([]map[string]interface{}, len(backends))
		for i, b := range backends {
			c := hs.ip.Circles[i]
			data[i] = map[string]interface{}{
				"backend": map[string]string{"name": b.Name, "url": b.Url},
				"circle":  map[string]interface{}{"id": c.CircleId, "name": c.Name},
			}
		}
		hs.Write(w, req, 200, data)
	} else {
		hs.WriteError(w, req, 400, "invalid org, bucket or meas")
	}
}

func (hs *HttpService) Write(w http.ResponseWriter, req *http.Request, status int, data interface{}) {
	if status >= 400 {
		hs.WriteError(w, req, status, data.(string))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	hs.WriteHeader(w, status)
	pretty := req.URL.Query().Get("pretty") == "true"
	w.Write(util.MarshalJSON(data, pretty))
}

func (hs *HttpService) WriteError(w http.ResponseWriter, req *http.Request, status int, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Influxdb-Error", err)
	hs.WriteHeader(w, status)
	rsp := backend.ResponseFromError(err)
	pretty := req.URL.Query().Get("pretty") == "true"
	w.Write(util.MarshalJSON(rsp, pretty))
}

func (hs *HttpService) WriteBody(w http.ResponseWriter, body []byte) {
	hs.WriteHeader(w, 200)
	w.Write(body)
}

func (hs *HttpService) WriteText(w http.ResponseWriter, status int, text string) {
	hs.WriteHeader(w, status)
	w.Write([]byte(text + "\n"))
}

func (hs *HttpService) WriteHeader(w http.ResponseWriter, status int) {
	w.Header().Set("X-Influxdb-Version", backend.Version)
	w.WriteHeader(status)
}

func (hs *HttpService) checkMethodAndAuth(w http.ResponseWriter, req *http.Request, methods ...string) bool {
	return hs.checkMethod(w, req, methods...) && hs.checkAuth(w, req)
}

func (hs *HttpService) checkMethod(w http.ResponseWriter, req *http.Request, methods ...string) bool {
	for _, method := range methods {
		if req.Method == method {
			return true
		}
	}
	hs.WriteError(w, req, 405, "method not allow")
	return false
}

func (hs *HttpService) checkAuth(w http.ResponseWriter, req *http.Request) bool {
	if hs.token == "" {
		return true
	}
	token := strings.TrimSpace(strings.TrimPrefix(req.Header.Get("Authorization"), "Token "))
	if token == hs.token {
		return true
	}
	hs.WriteError(w, req, 401, "authentication failed")
	return false
}
