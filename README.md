InfluxDB Proxy
======

This project adds a basic high availability layer to InfluxDB.

NOTE: influx-proxy must be built with Go 1.14+ with Go module support, don't implement udp.

Why
---

We used [InfluxDB Relay](https://github.com/influxdata/influxdb-relay) before, but it doesn't support some demands.
We use grafana for visualizing time series data, so we need add datasource for grafana. We need change the datasource config when influxdb is down.
We need transfer data across idc, but Relay doesn't support gzip.
It's inconvenient to analyse data with connecting different influxdb.
Therefore, we made [InfluxDB Proxy](https://github.com/shell909090/influx-proxy). More details please visit [https://github.com/shell909090/influx-proxy](https://github.com/shell909090/influx-proxy).

Forked from the above InfluxDB Proxy, after many improvements and optimizations, [InfluxDB Proxy v1](https://github.com/chengshiwen/influx-proxy/tree/branch-1.x) has released, which no longer depends on python and redis, and supports more features.

Features
--------

* Support gzip.
* Support query.
* Support some cluster influxql.
* Filter some dangerous influxql.
* Transparent for client, like cluster for client.
* Cache data to file when write failed, then rewrite.
* Load config file and no longer depend on python and redis.
* Support precision query parameter when writing data.
* Support influxdb-java, influxdb shell and grafana.
* Support authentication and https.
* Support version display.

Requirements
-----------

* Golang >= 1.14 with Go module support

Usage
------------

#### Quickstart

```sh
$ git clone -b branch-1.x https://github.com/chengshiwen/influx-proxy.git
$ cd influx-proxy
$ make
$ ./bin/influx-proxy -config proxy.json
```

#### Usage

```sh
$ ./bin/influx-proxy -h
Usage of ./bin/influx-proxy:
  -config string
        proxy config file with json/yaml/toml format (default "proxy.json")
  -version
        proxy version
```

#### Build Release

```sh
$ # build current platform
$ make build
$ # build linux amd64
$ make linux
```

Description
-----------

The architecture is fairly simple, one InfluxDB Proxy process and two or more InfluxDB processes. The Proxy should point HTTP requests with measurements to the two InfluxDB servers.

The setup should look like this:

```
        ┌─────────────────┐
        │writes & queries │
        └─────────────────┘
                 │
                 ▼
         ┌───────────────┐
         │               │
         │InfluxDB Proxy │
         |  (only http)  |
         │               │
         └───────────────┘
                 │
                 ▼
        ┌─────────────────┐
        │   measurements  │
        └─────────────────┘
          |              |
        ┌─┼──────────────┘
        │ └──────────────┐
        ▼                ▼
  ┌──────────┐      ┌──────────┐
  │          │      │          │
  │ InfluxDB │      │ InfluxDB │
  │          │      │          │
  └──────────┘      └──────────┘
```

Measurements match principle:

* **Exact match first**. For instance, we use `cpu.load` for measurement's name. The KEYMAPS has `cpu` and `cpu.load` keys.
It will use the `cpu.load` corresponding backends.

* **Prefix match then**. For instance, we use `cpu.load` for measurement's name. The KEYMAPS only has `cpu` key.
It will use the `cpu` corresponding backends.

* **\_default\_ match finally**. For instance, we use `cpu.load` for measurement's name. The KEYMAPS only has `_default_` key.
It will use the `_default_` corresponding backends.

Configuration
--------

The configuration file supports format `json`, `yaml` and `toml`, such as [proxy.json](proxy.json), [proxy.yaml](conf/proxy.yaml) and [proxy.toml](conf/proxy.toml).

The configuration settings are as follows:

#### BACKENDS

* `url`: influxdb addr or other http backend which supports influxdb line protocol
* `db`: influxdb db
* `username`: influxdb username, default is "" for no auth
* `password`: influxdb password, default is "" for no auth
* `flush_size`: default config is 10000, wait 10000 points write
* `flush_time`: default config is 1000ms, wait 1 second write whether point count has bigger than flush_size config
* `timeout`: default config is 10000ms, write timeout until 10 seconds
* `check_interval`: default config is 1000ms, check backend active every 1 second
* `rewrite_interval`: default config is 10000ms, rewrite every 10 seconds
* `conn_pool_size`: default config is 20, create a connection pool which size is 20
* `write_only`: default is false

#### KEYMAPS

* `measurement: [BACKENDS keys]`
  * the key must be in the BACKENDS
  * http request with the measurement matching the above principles will be forwarded to the backends

#### NODE

* `listen_addr`: proxy listen addr, default is ":7076"
* `db`: proxy db, client's db must be same with it, default is "" for no limit
* `username`: proxy username, default is "" for no auth
* `password`: proxy password, default is "" for no auth
* `data_dir`: data dir to save .dat .rec, default is "data"
* `log_path`: log file path, default "" for stdout
* `idle_timeout`: keep-alives wait time, default is 10000ms
* `stat_interval`: interval to collect statistics, default is 10000ms
* `write_tracing`: enable logging for the write, default is false
* `query_tracing`: enable logging for the query, default is false
* `https_enabled`: enable https, default is false
* `https_cert`: the ssl certificate to use when https is enabled, default is ""
* `https_key`: use a separate private key location, default is ""

Query Commands
--------

#### Unsupported commands

The following commands are forbid.

* `ALTER`
* `GRANT`
* `REVOKE`
* `KILL`
* `SELECT INTO`
* `ON clause`
* `Multiple queries` delimited by semicolon `;`
* `Multiple measurements` delimited by comma `,`

#### Supported commands

Only support match the following commands.

* `select from`
* `show from`
* `show measurements`
* `show series`
* `show field keys`
* `show tag keys`
* `show tag values`
* `show retention policies`
* `show stats`
* `show databases`
* `create database`
* `delete from`
* `drop series from`
* `drop measurement`

Benchmark
-----------

There are three tools for benchmarking InfluxDB, which can also be applied to InfluxDB Proxy:

* [influx-stress](https://github.com/chengshiwen/influx-stress) is a stress tool for generating artificial load on InfluxDB.
* [influxdb-comparisons](https://github.com/influxdata/influxdb-comparisons) contains code for benchmarking InfluxDB against other databases and time series solutions.
* [tsbs](https://github.com/timescale/tsbs) (Time Series Benchmark Suite) is a tool for comparing and evaluating databases for time series data.

License
-------

MIT.
