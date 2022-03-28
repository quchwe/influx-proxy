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


BASEDIR=$(cd $(dirname $0)/..; pwd)

function set_token() {
    until docker logs influxdb-$1 2>&1 | grep ':8086' &>/dev/null; do
        counter=$((counter+1))
        if [ $counter -eq 30 ]; then
            echo "error: influxdb is not ready"
            exit 1
        fi
        sleep 0.5
    done
    docker exec -it influxdb-$1 influx setup -u influxdb -p influxdb -o myorg -b mybucket -f &> /dev/null
    INFLUX_TOKEN=$(docker exec -it influxdb-$1 bash -c "influx auth list -u influxdb | tail -n 1" | cut -f 3)
    if [[ -n $(sed --version 2> /dev/null | grep "GNU sed") ]]; then
        sed -i "$2s#\"token\": \".*\"#\"token\": \"${INFLUX_TOKEN}\"#" ${BASEDIR}/proxy.json
    else
        sed -i '' "$2s#\"token\": \".*\"#\"token\": \"${INFLUX_TOKEN}\"#" ${BASEDIR}/proxy.json
    fi
}

set_token 1 9
set_token 2 14
set_token 3 24
set_token 4 29
