#!/bin/bash

echo "query test:"

curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "cpu1")'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "cpu2")'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "cpu3")'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "cpu4")'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "measurement with spaces, commas and \"quotes\"")'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "\"measurement with spaces, commas and \"quotes\"\"")'


echo ""
echo "json test:"

curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/json' -d '{"query": "from(bucket:\"mybucket\") |> range(start:0) |> filter(fn: (r) => r._measurement == \"cpu1\")"}'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/json' -d '{"query": "from(bucket:\"mybucket\") |> range(start:0) |> filter(fn: (r) => r._measurement == \"cpu2\")"}'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/json' -d '{"query": "from(bucket:\"mybucket\") |> range(start:0) |> filter(fn: (r) => r._measurement == \"cpu3\")"}'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/json' -d '{"query": "from(bucket:\"mybucket\") |> range(start:0) |> filter(fn: (r) => r._measurement == \"cpu4\")"}'


echo ""
echo "error test:"

curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d ''
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from(bucket:"mybucket")'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from(bucket:"mybucket") |> range(start:0)'
curl -X POST 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H 'Content-type: application/vnd.flux' -d 'from(bucket:"mybucket") |> filter(fn: (r) => r._measurement == "cpu1")'


echo ""
echo ""
echo "gzip test:"

queries=(
    'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "cpu1")'
    'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "cpu2")'
    'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "cpu3")'
    'from(bucket:"mybucket") |> range(start:0) |> filter(fn: (r) => r._measurement == "cpu4")'
)

len=${#queries[*]}
i=0
while (($i<$len)); do
    query=${queries[$i]}
    curl -X POST -s 'http://127.0.0.1:7076/api/v2/query?org=myorg' -H "Accept-Encoding: gzip" -H 'Content-type: application/vnd.flux' -d "$query" | gzip -d
    i=$(($i+1))
done
