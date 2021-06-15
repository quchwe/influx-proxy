// Copyright 2021 Shiwen Cheng. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package backend

import (
	"log"

	"github.com/spf13/viper"
)

var (
	Version   = "not build"
	GitCommit = "not build"
	BuildTime = "not build"
)

type NodeConfig struct {
	ListenAddr   string `mapstructure:"listen_addr"`
	DB           string `mapstructure:"db"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	DataDir      string `mapstructure:"data_dir"`
	LogPath      string `mapstructure:"log_path"`
	IdleTimeout  int    `mapstructure:"idle_timeout"`
	StatInterval int    `mapstructure:"stat_interval"`
	WriteTracing bool   `mapstructure:"write_tracing"`
	QueryTracing bool   `mapstructure:"query_tracing"`
	HTTPSEnabled bool   `mapstructure:"https_enabled"`
	HTTPSCert    string `mapstructure:"https_cert"`
	HTTPSKey     string `mapstructure:"https_key"`
}

type BackendConfig struct { // nolint:golint
	URL             string `mapstructure:"url"`
	DB              string `mapstructure:"db"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	FlushSize       int    `mapstructure:"flush_size"`
	FlushTime       int    `mapstructure:"flush_time"`
	Timeout         int    `mapstructure:"timeout"`
	CheckInterval   int    `mapstructure:"check_interval"`
	RewriteInterval int    `mapstructure:"rewrite_interval"`
	ConnPoolSize    int    `mapstructure:"conn_pool_size"`
	WriteOnly       bool   `mapstructure:"write_only"`
}

// KEYMAPS
// measurement: [BACKENDS keys], the key must be in the BACKENDS

type FileConfigSource struct {
	BACKENDS map[string]BackendConfig `mapstructure:"BACKENDS"`
	KEYMAPS  map[string][]string      `mapstructure:"KEYMAPS"`
	NODE     NodeConfig               `mapstructure:"NODE"`
}

func NewFileConfigSource(cfgfile string) (fcs *FileConfigSource, err error) {
	fcs = &FileConfigSource{}
	viper.SetConfigFile(cfgfile)
	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	viper.Unmarshal(fcs)
	return
}

func (fcs *FileConfigSource) LoadNode() (nodecfg NodeConfig) {
	nodecfg = fcs.NODE
	if nodecfg.ListenAddr == "" {
		nodecfg.ListenAddr = ":7076"
	}
	if nodecfg.DataDir == "" {
		nodecfg.DataDir = "data"
	}
	if nodecfg.IdleTimeout == 0 {
		nodecfg.IdleTimeout = 10000
	}
	if nodecfg.StatInterval == 0 {
		nodecfg.StatInterval = 10000
	}
	return
}

func (fcs *FileConfigSource) LoadBackends() (backends map[string]*BackendConfig, err error) {
	backends = make(map[string]*BackendConfig)
	for name, val := range fcs.BACKENDS {
		cfg := &BackendConfig{
			URL:             val.URL,
			DB:              val.DB,
			Username:        val.Username,
			Password:        val.Password,
			FlushSize:       val.FlushSize,
			FlushTime:       val.FlushTime,
			Timeout:         val.Timeout,
			CheckInterval:   val.CheckInterval,
			RewriteInterval: val.RewriteInterval,
			ConnPoolSize:    val.ConnPoolSize,
			WriteOnly:       val.WriteOnly,
		}
		if cfg.FlushSize == 0 {
			cfg.FlushSize = 10000
		}
		if cfg.FlushTime == 0 {
			cfg.FlushTime = 1000
		}
		if cfg.Timeout == 0 {
			cfg.Timeout = 10000
		}
		if cfg.CheckInterval == 0 {
			cfg.CheckInterval = 1000
		}
		if cfg.RewriteInterval == 0 {
			cfg.RewriteInterval = 10000
		}
		if cfg.ConnPoolSize == 0 {
			cfg.ConnPoolSize = 20
		}
		backends[name] = cfg
	}
	log.Printf("%d backends loaded from file", len(backends))
	return
}

func (fcs *FileConfigSource) LoadMeasurements() (m_map map[string][]string, err error) { // nolint:golint
	m_map = fcs.KEYMAPS
	log.Printf("%d measurements loaded from file", len(m_map))
	return
}
