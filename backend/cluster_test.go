// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func CreateTestInfluxCluster() (ic *InfluxCluster, err error) {
	fileConfig := &FileConfigSource{}
	nodeConfig := &NodeConfig{DataDir: "../data/test", StatInterval: 10000}
	ic = NewInfluxCluster(fileConfig, nodeConfig)
	backends := make(map[string]BackendAPI)
	bkcfgs := make(map[string]*BackendConfig)
	cfg, _ := CreateTestBackendConfig("test1")
	bkcfgs["test1"] = cfg
	cfg, _ = CreateTestBackendConfig("test2")
	bkcfgs["test2"] = cfg
	cfg, _ = CreateTestBackendConfig("write_only")
	cfg.WriteOnly = true
	bkcfgs["write_only"] = cfg
	for name, cfg := range bkcfgs {
		backends[name], err = NewBackends(cfg, name, ic.datadir)
		if err != nil {
			return
		}
	}
	ic.backends = backends
	m2bs := make(map[string][]BackendAPI)
	m2bs["cpu"] = append(m2bs["cpu"], backends["write_only"], backends["test1"])
	m2bs["write_only"] = append(m2bs["write_only"], backends["write_only"])
	ic.m2bs = m2bs

	return
}

func TestInfluxdbClusterWrite(t *testing.T) {
	ic, err := CreateTestInfluxCluster()
	if err != nil {
		t.Error(err)
		return
	}
	tests := []struct {
		name string
		args []byte
		unit string
		want error
	}{
		{
			name: "cpu",
			args: []byte("cpu value=1,value2=2 1434055562000010000"),
			want: nil,
		},
		{
			name: "cpu",
			args: []byte("cpu value=3,value2=4 1434055562000010000"),
			unit: "ns",
			want: nil,
		},
		{
			name: "cpu.load",
			args: []byte("cpu.load value=3,value2=4 1434055562000010"),
			unit: "u",
			want: nil,
		},
		{
			name: "load.cpu",
			args: []byte("load.cpu value=3,value2=4 1434055562000"),
			unit: "ms",
			want: nil,
		},
		{
			name: "test",
			args: []byte("test value=3,value2=4 1434055562"),
			unit: "s",
		},
	}
	for _, tt := range tests {
		err := ic.Write(tt.args, tt.unit)
		if err != nil {
			t.Error(tt.name, err)
			continue
		}
	}
	time.Sleep(time.Second)
}

func TestInfluxdbClusterPing(t *testing.T) {
	ic, err := CreateTestInfluxCluster()
	if err != nil {
		t.Error(err)
		return
	}
	version, err := ic.Ping()
	if err != nil {
		t.Error(err)
		return
	}
	if version == "" {
		t.Error("empty version")
		return
	}
	time.Sleep(time.Second)
}

func TestInfluxdbClusterQuery(t *testing.T) {
	ic, err := CreateTestInfluxCluster()
	if err != nil {
		t.Error(err)
		return
	}
	w := NewDummyResponseWriter()
	w.Header().Add("X-Influxdb-Version", Version)
	q := url.Values{}
	q.Set("db", "test")

	tests := []struct {
		name  string
		query string
		want  error
	}{
		{
			name:  "cpu",
			query: "SELECT * from cpu where time < now() - 1m",
			want:  nil,
		},
		{
			name:  "test",
			query: "SELECT cpu_load from test",
			want:  ErrUnknownMeasurement,
		},
		{
			name:  "cpu_load",
			query: " select cpu_load from cpu",
			want:  nil,
		},
		{
			name:  "cpu.load",
			query: " select cpu_load from \"cpu.load\" WHERE time > now() - 1m",
			want:  nil,
		},
		{
			name:  "load.cpu",
			query: " select cpu_load from \"load.cpu\" WHERE time > now() - 1m",
			want:  ErrUnknownMeasurement,
		},
		{
			name:  "show_tag_keys",
			query: "SHOW tag keys from \"cpu\" ",
			want:  nil,
		},
		{
			name:  "show_tag_values",
			query: "SHOW tag values WITH key = \"host\"",
			want:  nil,
		},
		{
			name:  "show_field_keys",
			query: "SHOW field KEYS from \"cpu\" ",
			want:  nil,
		},
		{
			name:  "delete_cpu",
			query: " DELETE FROM \"cpu\" WHERE time < '2000-01-01T00:00:00Z'",
			want:  nil,
		},
		{
			name:  "show_series",
			query: "show series",
			want:  nil,
		},
		{
			name:  "show_measurements",
			query: "SHOW measurements ",
			want:  nil,
		},
		{
			name:  "show_retention_policies",
			query: " SHOW retention policies limit 10",
			want:  nil,
		},
		{
			name:  "cpu.load_with_host1",
			query: " select cpu_load from \"cpu.load\" WHERE host =~ /^$/",
			want:  nil,
		},
		{
			name:  "cpu.load_with_host2",
			query: " select cpu_load from \"cpu.load\" WHERE host =~ /^()$/",
			want:  nil,
		},
		{
			name:  "cpu.load_into_from",
			query: "select * into \"cpu.load_new\" from \"cpu.load\"",
			want:  ErrIllegalQL,
		},
		{
			name:  "cpu.load_into_from_group_by",
			query: "select * into \"cpu.load_new\" from \"cpu.load\" GROUP BY *",
			want:  ErrIllegalQL,
		},
		{
			name:  "write.only",
			query: " select cpu_load from write_only",
			want:  nil,
		},
		{
			name:  "drop_series",
			query: "DROP series from \"cpu.load\"",
			want:  nil,
		},
		{
			name:  "drop_measurement",
			query: "DROP measurement \"cpu.load\"",
			want:  nil,
		},
		{
			name:  "empty",
			query: "",
			want:  ErrEmptyQuery,
		},
		{
			name:  "select.empty.measurement",
			query: "select * from",
			want:  ErrGetMeasurement,
		},
		{
			name:  "select.illegal",
			query: "select * measurement",
			want:  ErrIllegalQL,
		},
		{
			name:  "show.tag.illegal",
			query: "show TAG from cpu",
			want:  ErrIllegalQL,
		},
		{
			name:  "show.tag.on.illegal",
			query: "show TAG keys on test from cpu",
			want:  ErrOnClauseForbidden,
		},
		{
			name:  "show.tag.empty.measurement",
			query: "show TAG values from ",
			want:  ErrGetMeasurement,
		},
		{
			name:  "show.series.empty.measurement",
			query: "show series from",
			want:  ErrGetMeasurement,
		},
		{
			name:  "show.measurement.illegal",
			query: "show measurement",
			want:  ErrIllegalQL,
		},
		{
			name:  "show.stat.illegal",
			query: "show stat",
			want:  ErrIllegalQL,
		},
		{
			name:  "drop.illegal",
			query: "drop",
			want:  ErrIllegalQL,
		},
		{
			name:  "delete.empty.measurement",
			query: "delete from ",
			want:  ErrIllegalQL,
		},
		{
			name:  "drop.series.illegal",
			query: "drop series",
			want:  ErrIllegalQL,
		},
		{
			name:  "drop.series.empty.measurement",
			query: "drop series from",
			want:  ErrGetMeasurement,
		},
		{
			name:  "drop.empty.measurement",
			query: "drop measurement",
			want:  ErrIllegalQL,
		},
		{
			name:  "create.database.illegal",
			query: "CREATE DATABASE",
			want:  ErrDatabaseNotFound,
		},
		{
			name:  "drop.database.illegal",
			query: "drop database ",
			want:  ErrIllegalQL,
		},
		{
			name:  "show.tag.db.illegal",
			query: "show TAG keys test from mem",
			want:  ErrIllegalQL,
		},
	}

	for _, tt := range tests {
		q.Set("q", tt.query)
		req, _ := http.NewRequest("GET", "http://localhost:7076/query?"+q.Encode(), nil)
		req.URL.Query()
		err = ic.Query(w, req)
		if err != tt.want {
			t.Error(tt.name, tt.want, err)
		}
		w.buffer.Reset()
	}
}
