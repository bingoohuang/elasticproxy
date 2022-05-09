# elasticproxy

This little program acts as a http proxy for ElasticSearch.

![image](elasticproxy.svg)

## Features TODO

- [x] HTTPClient(Primary/Rests) timeout 2022-05-06
- [x] Verify _bulk api 2022-05-05
- [x] Backup retries 2022-05-05
- [x] Loop back avoiding 2022-04-29, ClusterID included in the kafka bean
- [x] Kafka consuming to replay elastic write events 2022-04-28
- [x] YAML config file 2022-04-28
- [x] Backup to Kafka 2022-04-28

## build, start

1. build: `make install`
2. initialize: `elasticproxy -init` (this will create ctl shell script for convenience of startup or stop)
3. start [httplive](https://github.com/bingoohuang/httplive), for backup mocking: `httplive -l`
   , [download httplive](http://d5k.co/httplive/dl/)
4. edit the created conf.yml file at the current working directory, [example](initassets/conf.yml)
5. start: `./ctl start`
6. tail log: `./ctl tail`
7. build test: `gurl :2900/person/_bulk -b testdata/bulk.json -raw`

## usage

```sh
$ elasticproxy -h
Usage of elasticproxy:
  -conf (-c) string     config file (default "./conf.yml")
  -init Create initial ctl and exit
  -version (-v) Create initial ctl and exit
```

```sh
$ ./ctl tail
2022-04-29 12:12:38.509 [INFO ] 16071 --- [82   ] [-]  : rest http://127.0.0.1:9200/person/doc/28SMVg28QLnm8zCM0wGCLsZzT6J do status: 201
2022-04-29 12:12:38.510 [INFO ] 16071 --- [82   ] [-]  : {"direction":"primary","duration":"25.848878ms","method":"POST","path":"/person/doc/28SMVg28QLnm8zCM0wGCLsZzT6J","remote_addr":"127.0.0.1:53824","status":201,"target":"/person/doc/28SMVg28QLnm8zCM0wGCLsZzT6J"}
2022-04-29 12:12:38.512 [INFO ] 16071 --- [1    ] [-]  : {"direction":"backup","duration":"2.087094ms","status":200,"target":"http://127.0.0.1:5003/backup/person/doc/28SMVg28QLnm8zCM0wGCLsZzT6J"}
2022-04-29 12:12:38.512 [INFO ] 16071 --- [1    ] [-]  : kafka write size: 397, message: {"labels":{"SRC":"PROXY"},"host":"127.0.0.1:2900","remoteAddr":"127.0.0.1:53824","method":"POST","requestUri":"/person/doc/28SMVg28QLnm8zCM0wGCLsZzT6J","header":{"Content-Type":["application/json"]},"body":{"addr":"四川省甘孜藏族自治州鑃纺路3694号睟鯳小区16单元2036室","idcard":"467063198902092332","name":"庄樿迡","sex":"男"},"clusterIds":["28SHI8ckjhQp6oHTaCYTKLbVFxP"]},to kafka
2022-04-29 12:12:38.518 [INFO ] 16071 --- [61   ] [-]  : kafka.claimed group: elastic.bb01, len: 397, value: {"labels":{"SRC":"PROXY"},"host":"127.0.0.1:2900","remoteAddr":"127.0.0.1:53824","method":"POST","requestUri":"/person/doc/28SMVg28QLnm8zCM0wGCLsZzT6J","header":{"Content-Type":["application/json"]},"body":{"addr":"四川省甘孜藏族自治州鑃纺路3694号睟鯳小区16单元2036室","idcard":"467063198902092332","name":"庄樿迡","sex":"男"},"clusterIds":["28SHI8ckjhQp6oHTaCYTKLbVFxP"]}, time: 2022-04-29 12:12:38.512 +0800 CST, topic: elastic.bb06, offset: 7, partition: 0
2022-04-29 12:12:38.518 [INFO ] 16071 --- [61   ] [-]  : already wrote to ClusterID 28SHI8ckjhQp6oHTaCYTKLbVFxP, ignoring
2022-04-29 12:12:38.519 [INFO ] 16071 --- [1    ] [-]  : kafka.produce result &{Partition:0 Offset:7 Topic:elastic.bb06}
```

## help commands

1. `docker-compose up && docker-compose rm -fsv`
2. `gurl 'name=@姓名' 'sex=@random(男,女)' 'addr=@地址' 'idcard=@身份证' -ugly :2900/person/doc/@ksuid -raw`
   , [download gurl](http://d5k.co/httplive/dl/)
3. `gurl http://127.0.0.1:2900/_search` vs `http://127.0.0.1:9200/_search`
4. elastic search
    1. `gurl :9200/person/_search q=@名字`
    2. `gurl :2900/person/_doc/@id -ugly -pb`
    2. `gurl :2900/person/_search q=_id:@id -ugly -pb`
    3. `gurl GET :2900/person/_search -b '{"query":{"terms":{"_id":["@id"]}}}' -ugly -pb`

```sh
bingoobjca@bogon elasticproxy % cat testdata/bulk.json | jj -gu
{"index":{"_index":"test","_id":"28jr6LKsyySD86DrIwUONkUGswA"}}
{"addr":"黑龙江省哈尔滨市崪諣路7451号倞鮮小区17单元2194室","idcard":"825491199903301279","name":"陶諗蛃","sex":"男"}
{"index":{"_index":"test","_id":"28jr6NCcbfEiTjSBAIyRa0f3zjX"}}
{"addr":"河南省周口市芰鴔路4159号侜緸小区9单元1033室","idcard":"131471199102232071","name":"堵韹司","sex":"男"}
```

## resources

1. [solrdump/esdump](https://github.com/gobars/solrdump)
