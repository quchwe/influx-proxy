#!/bin/bash

curl -X POST 'http://127.0.0.1:7076/query' --data-urlencode 'q=CREATE DATABASE test'

echo ""

curl -i -X POST 'http://127.0.0.1:7076/write?db=test' --data-binary \
'cpu1,host=server02 value=0.67,id=2i,running=true,status="ok"
cpu1,host=server02,region=us-west value=0.55,id=2i,running=true,status="ok" 1422568543702900257
cpu1,host=server01,region=us-east,direction=in value=2.0,id=1i,running=false,status="fail" 1583599143422568543'

curl -i -X POST 'http://127.0.0.1:7076/write?db=test&precision=s' --data-binary \
'cpu2,host=server02 value=0.67,id=2i,running=true,status="ok"
cpu2,host=server03,region=us-east value=0.93,id=3i 1422568543
cpu2,host=server04,region=us-west,direction=out running=false,status="fail" 1596819659'

curl -i -X POST 'http://127.0.0.1:7076/write?db=test&precision=h' --data-binary \
'mem,host=server05,region=cn\ north,tag\ key=tag\ value idle=64,system=1i,user="Dwayne Johnson",admin=true
mem,host=server06,region=cn\ south,tag\ key=value\ with"quotes" idle=16,system=16i,user="Jay Chou",admin=false  439888
mem,host=server07,region=cn\ south,tag\ key=value\,with"commas" idle=74,system=23i,user="Stephen Chow" 440204'

curl -i -X POST 'http://127.0.0.1:7076/write?db=test&precision=ms' --data-binary \
'svr idle=14,system=31i,user="Dwayne Johnson",admin=true
svr idle=39,system=56i,user="Jay Chou",brief\ desc="the best \"singer\"" 1422568543702
svr idle=47,system=93i,user="Stephen Chow",admin=true,brief\ desc="the best \"novelist\""  1596819420440'