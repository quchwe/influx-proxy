#!/bin/bash

if [[ $# -gt 1 ]] || [[ "$1" != "" && "$1" != "https" ]]; then
    echo "Usage: $0 [https]"
    exit 1
fi

if [[ "$1" == "https" ]]; then
    echo "Mode: $1"
    OPTIONS="-v ${PWD}/cert:/cert -e INFLUXD_TLS_CERT=/cert/tls.crt -e INFLUXD_TLS_KEY=/cert/tls.key"
else
    echo "Mode: default"
fi

docker run -d --name influxdb-1 -p 8086:8086 ${OPTIONS} influxdb:2.1
docker run -d --name influxdb-2 -p 8087:8086 ${OPTIONS} influxdb:2.1
docker run -d --name influxdb-3 -p 8088:8086 ${OPTIONS} influxdb:2.1
docker run -d --name influxdb-4 -p 8089:8086 ${OPTIONS} influxdb:2.1
