# 验证环境相关

## 部署图

![img](testenv.svg)

## 安装 kafka 2.7.2

1. 2.7.2 Released November 15, 2021
2. 2.7.1 Released May 10, 2020
3. 2.7.0 Released Dec 21, 2020

- 2.7.2 是最后一个还是只用 zk 的版本，主要是去除ZK的版本，是否稳定，社区其他对应的工具是否都支持了，例如 kafka manager 或者其他 golang 和 java 的 SDK
  是否又有成熟的，还有跟旧版本的兼容性，例如，telegraf的kafka output，最新的肯定不是最好的，最少1年以上的版本吧，我估计社区差不多成熟了
- 小版本找最新的就行，一般都是修复bug的，功能上没有改动

[kafka quickstart](https://kafka.apache.org/documentation/#quickstart)

```sh
# Start the ZooKeeper service
# Note: Soon, ZooKeeper will no longer be required by Apache Kafka.
$ bin/zookeeper-server-start.sh config/zookeeper.properties
# Start the Kafka broker service
$ bin/kafka-server-start.sh config/server.properties
```

[kafka_2.13-2.7.2/config/zookeeper.properties](http://127.0.0.1:8334/view/home/footstone/bingoo/kafka_2.13-2.7.2/config/zookeeper.properties)

```properties
# the directory where the snapshot is stored.
dataDir=data/zookeeper-data
# the port at which the clients will connect
clientPort=2081
```

[kafka_2.13-2.7.2/config/server.properties](http://127.0.0.1:8334/view/home/footstone/bingoo/kafka_2.13-2.7.2/config/server.properties)

```properties
listeners=PLAINTEXT://:59092
log.dirs=data/kafka-logs
zookeeper.connect=localhost:2081
```

```sh
[footstone@fs02-192-168-126-16 kafka_2.13-2.7.2]$ more start.sh
nohup bin/zookeeper-server-start.sh config/zookeeper.properties 3>&1>> zk.nohup.log &
nohup bin/kafka-server-start.sh config/server.properties 2>&1 >> kafka.nohup.log &
```

## RSS usage tracking

```sh
[footstone@fs02-192-168-126-16 bingoo]$ ps aux | awk 'NR==1 || /elasticproxy$/'
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
footsto+  9396  1.5  0.0 718192 17488 pts/12   Sl   17:10   0:00 elasticproxy
footsto+ 14008  4.5  0.0 717360 16656 pts/4    Sl   17:10   0:00 elasticproxy

[footstone@fs02-192-168-126-16 bingoo]$ ps aux | awk 'NR==1 || /elasticproxy$/'
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
footsto+  9396  0.2  0.0 719536 20524 ?        Sl   5月19   2:33 elasticproxy
footsto+ 14008  0.0  0.0 719536 20716 ?        Sl   5月19   0:10 elasticproxy

[footstone@fs02-192-168-126-16 ~]$ ps aux | awk 'NR==1 || /elasticproxy$/'
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
footsto+  9396  0.2  0.0 720048 20196 ?        Sl   5月19  14:24 elasticproxy
footsto+ 14008  0.0  0.0 720304 21532 ?        Sl   5月19   0:56 elasticproxy
```

`gurl GET 192.168.112.67:9200/person/_search size=1 -pb q=@some`

```sh
$ gurl GET 192.168.112.67:9200/person/_search size=1 -pb q=赫连菴嘗 | jj hits.hits.0._source
{
  "addr": "浙江省丽水市黆抝路5481号枚鍫小区1单元2242室",
  "idcard": "112534199405317057",
  "name": "赫连菴嘗",
  "sex": "女"
}
```

## performance test

1. prepare data file: `for i in {1..10000}; do jj 'name=@姓名' 'sex=@random(男,女)' 'addr=@地址' 'idcard=@身份证' -ug; done > persons.txt`
2. run performance: `BLOW_STATUS=-201 berf 192.168.126.16:9200/person/_doc/@ksuid -b persons.txt:line  -opt eval,json`

```sh
[footstone@fs03-192-168-126-18 ~]$ BLOW_STATUS=-201  berf 192.168.112.67:9200/p1/_doc/@ksuid -b persons.txt:line  -opt eval,json -auth ZWxhc3RpYzoxcWF6WkFRIQ -vv -c100 -t10
Log details to: ./blow_20220524140203_1054048039.log
Berf benchmarking http://192.168.112.67:9200/p1/_doc/@ksuid using 100 goroutine(s), 10 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Elapsed               21.125s
  Count/RPS       10000 473.355
    201           10000 473.355
  ReadWrite    1.266 1.547 Mbps
  Connections               100

Statistics    Min      Mean      StdDev       Max
  Latency   5.128ms  209.447ms  137.803ms  1.323051s
  RPS         1.2     472.16     143.72     723.95

Latency Percentile:
  P50           P75       P90        P95        P99       P99.9     P99.99
  196.603ms  271.413ms  341.25ms  401.681ms  838.484ms  1.140714s  1.323051s

Latency Histogram:
  51.622ms   1167  11.67%  ■■■■■■■■■■■■■■■
  133.401ms  3093  30.93%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  213.418ms  2477  24.77%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  308.181ms  2378  23.78%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  385.386ms   712   7.12%  ■■■■■■■■■
  479.66ms    155   1.55%  ■■
  552.677ms    12   0.12%
  791.933ms     6   0.06%
[footstone@fs03-192-168-126-18 ~]$ BLOW_STATUS=-201  berf 192.168.126.16:9200/p1/_doc/@ksuid -b persons.txt:line  -opt eval,json -auth ZWxhc3RpYzoxcWF6WkFRIQ -vv -c100 -t10
Log details to: ./blow_20220524140329_66237848.log
Berf benchmarking http://192.168.126.16:9200/p1/_doc/@ksuid using 100 goroutine(s), 10 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Elapsed                7.113s
  Count/RPS      10000 1405.819
    201          10000 1405.819
  ReadWrite    3.763 4.593 Mbps
  Connections               100

Statistics    Min      Mean     StdDev       Max
  Latency   1.484ms  68.747ms  113.924ms  977.429ms
  RPS       463.28   1399.24     696.2     2587.19

Latency Percentile:
  P50         P75        P90        P95        P99       P99.9     P99.99
  35.762ms  67.452ms  124.852ms  193.116ms  743.217ms  890.006ms  977.429ms

Latency Histogram:
  10.122ms    835   8.35%  ■■■■■■■■■
  26.082ms   3792  37.92%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  46.582ms   2720  27.20%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  112.42ms   2057  20.57%  ■■■■■■■■■■■■■■■■■■■■■■
  249.59ms    414   4.14%  ■■■■
  610.73ms    151   1.51%  ■■
  854.449ms    30   0.30%
  977.429ms     1   0.01%
[footstone@fs03-192-168-126-18 ~]$ BLOW_STATUS=-201  berf 192.168.126.16:2900/p2/_doc/@ksuid -b persons.txt:line  -opt eval,json -auth ZWxhc3RpYzoxcWF6WkFRIQ -vv -c100 -t10
Log details to: ./blow_20220524140353_2840828187.log
Berf benchmarking http://192.168.126.16:2900/p2/_doc/@ksuid using 100 goroutine(s), 10 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Elapsed                13.45s
  Count/RPS       10000 743.493
    201           10000 743.493
  ReadWrite    1.793 2.429 Mbps
  Connections               100

Statistics    Min      Mean      StdDev       Max
  Latency   4.616ms  133.416ms  131.082ms  1.307864s
  RPS        92.56    756.42     258.34     965.01

Latency Percentile:
  P50           P75        P90        P95        P99       P99.9     P99.99
  104.384ms  134.038ms  200.177ms  310.902ms  871.721ms  1.136343s  1.307864s

Latency Histogram:
  34.144ms    696   6.96%  ■■■■■
  77.75ms    1738  17.38%  ■■■■■■■■■■■■■
  114.878ms  5492  54.92%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  183.91ms   1658  16.58%  ■■■■■■■■■■■■
  424.456ms   227   2.27%  ■■
  680.738ms   144   1.44%  ■
  951.154ms    37   0.37%
  1.233362s     8   0.08%
```