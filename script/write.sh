#!/bin/bash

curl -i -X POST 'http://127.0.0.1:7076/api/v2/write?org=myorg&bucket=mybucket' --data-binary \
'cpu1,host=server02 value=0.67,id=2i,running=true,status="ok"
cpu1,host=server02,region=us-west value=0.55,id=2i,running=true,status="ok" 1422568543702900257
cpu1,host=server01,region=us-east,direction=in value=2.0,id=1i,running=false,status="fail" 1583599143422568543'

curl -i -X POST 'http://127.0.0.1:7076/api/v2/write?org=myorg&bucket=mybucket&precision=s' --data-binary \
'cpu2,host=server02 float=1422568543,long=1422568544i,running=true,status="ok"
cpu2,host=server03,region=us-east float=1422568543702,long=1422568543703i 1422568543
cpu2,host=server04,region=us-west,direction=out float=1422568543702568543,long=1422568543702568544i,running=false,status="fail" 1596819659'

curl -i -X POST 'http://127.0.0.1:7076/api/v2/write?org=myorg&bucket=mybucket&precision=us' --data-binary \
'cpu3,host=server05,region=cn\ north,tag\ key=tag\ value idle=64,system=1i,user="Dwayne Johnson",admin=true
cpu3,host=server06,region=cn\ south,tag\ key=value\=with"equals" idle=16,system=16i,user="Jay Chou",admin=false  1583596800000000
cpu3,host=server07,region=cn\ south,tag\ key=value\,with"commas" idle=74,system=23i,user="Stephen Chow" 1584734400000000'

curl -i -X POST 'http://127.0.0.1:7076/api/v2/write?org=myorg&bucket=mybucket&precision=ms' --data-binary \
'cpu4 idle=14,system=31i,user="Dwayne Johnson",admin=true,character="\", ,\,\\,\\\,\\\\"
cpu4 idle=39,system=56i,user="Jay Chou",brief\ desc="the best \"singer\"" 1422568543702
cpu4 idle=47,system=93i,user="Stephen Chow",admin=true,brief\ desc="the best \"novelist\""  1596819420440'

curl -i -X POST 'http://127.0.0.1:7076/api/v2/write?org=myorg&bucket=mybucket' --data-binary \
'measurement\ with\ spaces\,\ commas\ and\ "quotes",tag\ key\ with\ equals\ \==tag\ value\ with"spaces" field_k\ey\ with\ \=="string field value, multiple backslashes \,\\,\\\,\\\\"
"measurement\ with\ spaces\,\ commas\ and\ "quotes"",tag\ key\ with\ equals\ \==tag\,value\,with"commas" field_k\ey\ with\ \=="string field value, only \" need be escaped"'


echo ""

curl -i -X POST 'http://127.0.0.1:7076/write?db=mydb' --data-binary \
'cpu5,host=server02 value=0.67,id=2i,running=true,status="ok"
cpu5,host=server02,region=us-west value=0.55,id=2i,running=true,status="ok" 1422568543702900257
cpu5,host=server01,region=us-east,direction=in value=2.0,id=1i,running=false,status="fail" 1583599143422568543'

curl -i -X POST 'http://127.0.0.1:7076/write?db=mydb&precision=s' --data-binary \
'cpu6,host=server02 float=1422568543,long=1422568544i,running=true,status="ok"
cpu6,host=server03,region=us-east float=1422568543702,long=1422568543703i 1422568543
cpu6,host=server04,region=us-west,direction=out float=1422568543702568543,long=1422568543702568544i,running=false,status="fail" 1596819659'

curl -i -X POST 'http://127.0.0.1:7076/write?db=mydb&rp=myrp&precision=h' --data-binary \
'cpu7,host=server05,region=cn\ north,tag\ key=tag\ value idle=64,system=1i,user="Dwayne Johnson",admin=true
cpu7,host=server06,region=cn\ south,tag\ key=value\=with"equals" idle=16,system=16i,user="Jay Chou",admin=false  439888
cpu7,host=server07,region=cn\ south,tag\ key=value\,with"commas" idle=74,system=23i,user="Stephen Chow" 440204'

curl -i -X POST 'http://127.0.0.1:7076/write?db=mydb&rp=myrp&precision=ms' --data-binary \
'cpu8 idle=14,system=31i,user="Dwayne Johnson",admin=true,character="\", ,\,\\,\\\,\\\\"
cpu8 idle=39,system=56i,user="Jay Chou",brief\ desc="the best \"singer\"" 1422568543702
cpu8 idle=47,system=93i,user="Stephen Chow",admin=true,brief\ desc="the best \"novelist\""  1596819420440'

curl -i -X POST 'http://127.0.0.1:7076/write?db=mydb' --data-binary \
'measurement\ v1\ with\ spaces\,\ commas\ and\ "quotes",tag\ key\ with\ equals\ \==tag\ value\ with"spaces" field_k\ey\ with\ \=="string field value, multiple backslashes \,\\,\\\,\\\\"
"measurement\ v1\ with\ spaces\,\ commas\ and\ "quotes"",tag\ key\ with\ equals\ \==tag\,value\,with"commas" field_k\ey\ with\ \=="string field value, only \" need be escaped"'
