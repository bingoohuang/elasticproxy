# 使用说明

elasticproxy 通过反向代理和 kafka 对象，提供两个 elasticsearch 集群之间的数据同步功能。如果需要双向同步，则反方向部署一套同步系统即可。

## 前提

1. kafka 必须在 0.10.2.0 以上版本（建议 2.7.2 ）

   > 理由：2.7.2 是最后一个还是只用 zk 的版本，主要是去除 ZK 的版本，是否稳定，社区其他对应的工具是否都支持了，例如 kafka manager 或者其他 golang 和 java 的 SDK
   > 是否又有成熟的，还有跟旧版本的兼容性，例如，telegraf 的 kafka output，最新的肯定不是最好的，最少 1 年以上的版本吧，我估计社区差不多成熟了
   > 小版本找最新的就行，一般都是修复 bug 的，功能上没有改动

   发布日期：

   - 2.7.2 Released November 15, 2021
   - 2.7.1 Released May 10, 2020
   - 2.7.0 Released Dec 21, 2020

2. elasticsearch 建议在 6.8.5 以上

- 理由：目前一键部署都是默认该版本，
- 发布时间 [Elasticsearch 6.8.5 November 21, 2019](https://www.elastic.co/cn/downloads/past-releases#elasticsearch)
- 最新： Elasticsearch 8.2.0 May 04, 2022

## 安装

1. 方式 1：通过相关渠道获得可执行文件
2. 方式 2：自己编译（下载源码, make install 编译本机版本，make linux 编译 amd64 linux 版本）

## 初始化

1. 创建两个工作目录

   - elasticproxy 目录， 提供 elasticsearch 的反向代理和写 kafka 功能，比如 `mkdir -p elasticproxy && cd elasticproxy`
   - kafka2elastic 目录， 提供消费 kafka 同步到另外一个 elasticsearch 的功能，比如 `mkdir -p kafka2elastic && cd kafka2elastic`

2. 在各自工作目录下执行初始化，`elasticproxy -init` 初始化，生成示例配置文件 `conf.yml` 以及 控制脚本 `ctl`
3. 编辑配置 `conf.yml`， 配置文件，修改配置，参考对应下面示例部署图的两个示例配置文件

   - elasticproxy 的示例配置 [conf1.yml](testdata/testenv/conf1.yml)
   - kafka2elastic 的示例配置 [conf2.yml](testdata/testenv/conf2.yml)

4. 在各自工作目录下启动程序 `./ctl start` (相对应的命令，停止 `./ctl stop`，重启 `./ctl restart`)
5. 在各自工作目录下跟踪查看日志 `./ctl tail`

## 示例部署图

![img](testdata/testenv/testenv.svg)

其中：

1. 使用 `conf1.yml` 配置的 elasticproxy 程序，负责提供一个 http 反向代理，接收 http 请求，反向代理到真实的的 elasticsearch 上，并且对写请求，写入 kafka 消息队列
2. 使用 `conf2.yml` 配置的 elasticproxy 程序，负责消费 kafka，同步到另外的 elasticsearch 上

## 验证安装是否成功

通过反向代理，写入数据： `gurl -b '{"addr":"@地址","idcard":"@身份证","name":"@姓名","sex":"@性别"}' http://192.168.126.16:2900/test1/_doc/@ksuid`

```sh
# gurl 'name=@姓名' 'sex=@random(男,女)' 'addr=@地址' 'idcard=@身份证' -ugly :2900/person/_doc/@ksuid -raw -pa
Conn-Session: 127.0.0.1:29492->127.0.0.1:2900 (reused: false, wasIdle: false, idle: 0s)
POST /person/_doc/29drIbdJSxhDAGoe8LbuDWCxK4j HTTP/1.1
Host: 127.0.0.1:2900
Accept: application/json
Accept-Encoding: gzip, deflate
Content-Type: application/json
Gurl-Date: Wed, 25 May 2022 04:42:04 GMT
User-Agent: gurl/1.0.0

{"addr":"河南省周口市竓橺路3897号鼪蘫小区4单元361室","idcard":"431379197705031778","name":"司徒擸蟰","sex":"女"}
HTTP/1.1 201 Created
Server: fasthttp
Date: Wed, 25 May 2022 04:42:04 GMT
Content-Type: application/json; charset=UTF-8
Content-Length: 182
Location: /person/_doc/29drIbdJSxhDAGoe8LbuDWCxK4j

{"_index":"person","_type":"_doc","_id":"29drIbdJSxhDAGoe8LbuDWCxK4j","_version":1,"result":"created","_shards":{"total":2,"successful":2,"failed":0},"_seq_no":527,"_primary_term":1}
   DNS Lookup   TCP Connection   Request Transfer   Server Processing   Response Transfer
[       0 ms  |          0 ms  |            0 ms  |           24 ms  |             0 ms  ]
              |                |                  |                  |                   |
  namelookup: 0 ms             |                  |                  |                   |
                      connect: 0 ms               |                  |                   |
                                   wrote request: 0 ms               |                   |
                                                      starttransfer: 24 ms               |
                                                                                  total: 25 ms
2022/05/25 12:42:04.483320 main.go:164: current request cost: 25.49819ms
2022/05/25 12:42:04.483370 main.go:69: complete, total cost: 54.35165ms
```

查询数据（查询反向代理、查询真实 es 的两个集群），替换其中的 IP 和 端口即可，应该查的相同的数据

```sh
# gurl :9200/person/_doc/29drIbdJSxhDAGoe8LbuDWCxK4j -auth ZWxhc3RpYzoxcWF6WkFRIQ
Conn-Session: 127.0.0.1:2497->127.0.0.1:9200 (reused: false, wasIdle: false, idle: 0s)
GET /person/_doc/29drIbdJSxhDAGoe8LbuDWCxK4j HTTP/1.1
Host: 127.0.0.1:9200
Accept: application/json
Accept-Encoding: gzip, deflate
Authorization: Basic ZWxhc3RpYzoxcWF6WkFRIQ==
Content-Type: application/json
Gurl-Date: Wed, 25 May 2022 04:44:37 GMT
User-Agent: gurl/1.0.0


HTTP/1.1 200 OK
Content-Type: application/json; charset=UTF-8
Content-Encoding: gzip
Content-Length: 277

{
  "_index": "person",
  "_type": "_doc",
  "_id": "29drIbdJSxhDAGoe8LbuDWCxK4j",
  "_version": 1,
  "_seq_no": 527,
  "_primary_term": 1,
  "found": true,
  "_source": {
    "addr": "河南省周口市竓橺路3897号鼪蘫小区4单元361室",
    "idcard": "431379197705031778",
    "name": "司徒擸蟰",
    "sex": "女"
  }
}

   DNS Lookup   TCP Connection   Request Transfer   Server Processing   Response Transfer
[       0 ms  |          0 ms  |            0 ms  |            6 ms  |             0 ms  ]
              |                |                  |                  |                   |
  namelookup: 0 ms             |                  |                  |                   |
                      connect: 0 ms               |                  |                   |
                                   wrote request: 0 ms               |                   |
                                                      starttransfer: 6 ms                |
                                                                                  total: 7 ms
2022/05/25 12:44:37.150376 main.go:164: current request cost: 7.38298ms
2022/05/25 12:44:37.150446 main.go:69: complete, total cost: 7.529988ms
```

## 诊断

1. 进入目标工作目录（ctl 和 conf.yml 所在的目录)

   ```sh
   # cd elasticproxy/
   # pwd
   /home/footstone/elasticproxy
   # ls
   conf.yml  cpu.profile  ctl  var
   ```

2. 查看当前进程运行状态（中间有 PID）

   ```sh
   # ./ctl status
   elasticproxy started, pid=14008
   ```

3. 通知采集 5 分钟之内的进程运行信息，注意下面的 `$pid` 请换成上面的实际值，比如 `14008`，同时可以使用 `./ctl tail` 查看最新的日志

   ```sh
   # echo 5m > jj.cpu
   # kill -USR1 $pid
   # ./ctl tail
   2022-05-24 14:36:16.777 [INFO ] 14008 --- [20   ] [-]  : cpu.profile started
   2022-05-24 14:36:16.777 [INFO ] 14008 --- [20   ] [-]  : after 5m, cpu.profile will be generated
   ...
   2022-05-24 14:39:36.307 [INFO ] 14008 --- [51   ] [-]  : kafka write size: 397, message: {"body":{"addr":"江苏省无锡市嶻哎路439号喸幈小区16单元1950室","idcard":"364948198406056606","name":"闻人猶穽","sex":"男"},"clusterIds":["29HMKmglGqiRromQBrI3nyWQcyw"],"header":{"Content-Type":["application/json"]},"host":"127.0.0.1:2900","labels":null,"method":"POST","remoteAddr":"127.0.0.1:64420","requestUri":"/person/_doc/29bGScTpHCQboxtWDsRhhoZK7g9"},to kafka
   2022-05-24 14:39:36.325 [INFO ] 14008 --- [51   ] [-]  : kafka.produce result {"Partition":0,"Offset":134329,"Topic":"elastic16.backup"}
   2022-05-24 14:40:32.453 [INFO ] 14008 --- [791645] [-]  : access log: {"RemoteAddr":"127.0.0.1:2345","Method":"POST","Path":"/person/_doc/29bGaDkJt9kkqmYpvtRYq4ZlxGJ","Target":"http://192.168.126.16:9200/person/_doc/29bGaDkJt9kkqmYpvtRYq4ZlxGJ","Direction":"primary","Duration":"29.114144ms","StatusCode":201,"ResponseBody":{"_index":"person","_type":"_doc","_id":"29bGaDkJt9kkqmYpvtRYq4ZlxGJ","_version":1,"result":"created","_shards":{"total":2,"successful":2,"failed":0},"_seq_no":7,"_primary_term":1}}
   2022-05-24 14:40:36.308 [INFO ] 14008 --- [51   ] [-]  : kafka write size: 393, message: {"body":{"addr":"广东省韶关市說芲路4348号哣廳小区17单元234室","idcard":"316970200006169943","name":"叶絕炬","sex":"女"},"clusterIds":["29HMKmglGqiRromQBrI3nyWQcyw"],"header":{"Content-Type":["application/json"]},"host":"127.0.0.1:2900","labels":null,"method":"POST","remoteAddr":"127.0.0.1:2345","requestUri":"/person/_doc/29bGaDkJt9kkqmYpvtRYq4ZlxGJ"},to kafka
   2022-05-24 14:40:36.324 [INFO ] 14008 --- [51   ] [-]  : kafka.produce result {"Partition":0,"Offset":134330,"Topic":"elastic16.backup"}
   2022-05-24 14:41:16.798 [INFO ] 14008 --- [791584] [-]  : cpu.profile collected
   ```

4. 运行压力测试，或者等待（最好是在系统有业务负载的时候）5 分钟，查看生成的 `cpu.profile` 文件的大小

   ```sh
   # ls -lh *cpu*
   -rw-rw-rw- 1 footstone footstone 61K 5月  24 14:19 cpu.profile
   ```

5. 下载 `cpu.profile`，使用 go 工具开启可视化查看 `go tool pprof -http :9402 cpu.profile`
   ![img.png](_assets/img.png)

## 压测数据

20 万单条 POST 数据压测结果

| 目标               | TPS      | 损失 |
|------------------|----------|------|
| elasticproxy 代理  | 1110.994 | 11%  |
| elasticsearch 原始 | 1399.198 | -    |

观测网络连接数 `watch "netstat -atpn | grep :9200 | grep {pid} | wc -l"`，可以看到压测期间，elasticproxy 代理 到 elasticsearch 原始之间的连接数稳定在 100，符合预期（长连接，会话保持）。

数据准备：

```sh
# JJ_N=200000 jj -gu name=@姓名 'sex=@random(男,女)' addr=@地址 idcard=@身份证 > p20w.txt
```

原始输出：

```sh
# BLOW_STATUS=-201 berf 192.168.126.16:9200/p1/_doc/@ksuid -b p20w.txt:line  -opt eval,json -vv -auth ZWxhc3RpYzoxcWF6WkFRIQ
Log details to: ./blow_20220527155629_3542801236.log
Berf benchmarking http://192.168.126.16:9200/p1/_doc/@ksuid using 100 goroutine(s), 12 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
Elapsed             2m22.939s
Count/RPS     200000 1399.198
201         200000 1399.198
ReadWrite    3.774 4.571 Mbps
Connections               100

Statistics    Min      Mean     StdDev       Max
Latency   2.217ms  71.299ms  273.114ms  11.617517s
RPS        87.01   1494.32    721.85     3030.97

Latency Percentile:
P50         P75        P90       P95        P99       P99.9      P99.99
37.043ms  64.267ms  125.635ms  208.65ms  579.017ms  1.526334s  11.397509s

Latency Histogram:
46.373ms    166357  83.18%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
99.684ms     24422  12.21%  ■■■■■■
206.209ms     5981   2.99%  ■
507.83ms      2595   1.30%  ■
746.348ms      448   0.22%
921.397ms       74   0.04%
9.388871s      122   0.06%
11.617517s       1   0.00%

# BLOW_STATUS=-201 berf 192.168.126.16:2900/p1/_doc/@ksuid -b p20w.txt:line  -opt eval,json -vv
Log details to: ./blow_20220527155904_3829225044.log
Berf benchmarking http://192.168.126.16:2900/p1/_doc/@ksuid using 100 goroutine(s), 12 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
Elapsed              3m0.018s
Count/RPS     200000 1110.994
201         200000 1110.994
ReadWrite    3.342 3.212 Mbps
Connections               100

Statistics    Min      Mean     StdDev       Max
Latency   2.733ms  89.883ms  149.395ms  3.637019s
RPS        35.01   1122.11    533.63     2147.45

Latency Percentile:
P50         P75        P90        P95       P99       P99.9     P99.99
55.435ms  85.651ms  148.403ms  250.402ms  798.82ms  1.579418s  3.381517s

Latency Histogram:
59.748ms   162295  81.15%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
143.142ms   31360  15.68%  ■■■■■■■■
410.403ms    4297   2.15%  ■
748.614ms    1422   0.71%
996.066ms     262   0.13%
1.318155s     191   0.10%
2.590528s     171   0.09%
3.632266s       2   0.00%
```

## 压测机压测数据

```sh
[root@localhost ~]# sysinfo -format json -show host | jj
{
  "OS": "linux",
  "HostInfo": {
    "Hostname": "localhost.localdomain",
    "Uptime": 601381,
    "UptimeHuman": "6 days",
    "Procs": 414,
    "OS": "linux",
    "Platform": "centos",
    "HostID": "00000000-0000-0000-0000-002590c24096",
    "PlatformVersion": "7.5.1804",
    "KernelVersion": "3.10.0-862.el7.x86_64",
    "KernelArch": "x86_64",
    "OsRelease": "NAME=\"CentOS Linux\" VERSION=\"7 (Core)\"",
    "MemAvailable": "45.91GiB/62.73GiB, 00.73%",
    "NumCPU": 32,
    "CpuMhz": 3300,
    "CpuModel": "Intel(R) Xeon(R) CPU E5-2670 0 @ 2.60GHz"
  }
}
```

```yml
#[root@localhost esproxytest]# cat /opt/elasticproxy/conf.yml
---

# 备份请求缓冲区大小，0 时阻塞
chanSize: 2048

source:

  proxies:
    - port: 2900

destination:

  primaries:
    - url: http://192.168.126.222:9200/
      timeout: 3s
      header:
        Authorization: Basic c2NvdHQ6dGlnZXI= # The <TOKEN> is computed as base64(USERNAME:PASSWORD)
        #es-secondary-authorization: Basic <TOKEN> # The <TOKEN> is computed as base64(USERNAME:PASSWORD)
        #es-secondary-authorization: ApiKey <TOKEN> # The <TOKEN> is computed as base64(API key ID:API key)

  kafkas:
    - topic: elastic.backup
      version: 2.7.2
#      disabled: true
      codec: none
      sync: false
      brokers:
        - 192.168.126.200:9092
        - 192.168.126.200:9192
        - 192.168.126.200:9292
```

压测结果 | 直连 elasticsearch | 连接 elasticproxy + kafka(async)
---------|--------------------|--------------------------------
TPS      | 12071              | 15253

### 直连 elasticsearch 压测结果

```sh
[root@localhost esproxytest]# BLOW_STATUS=-201 berf 192.168.126.222:9200/p1/_doc -b p20w.txt:line  -opt eval,json -vv -auth c2NvdHQ6dGlnZXI
Log details to: ./blow_20220701014905_2472593851.log
Berf benchmarking http://192.168.126.222:9200/p1/_doc using 100 goroutine(s), 32 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Elapsed                 16.568s
  Count/RPS      200000 12071.381
    201          200000 12071.381
  ReadWrite    31.114 36.064 Mbps
  Connections                 100

Statistics    Min      Mean    StdDev      Max
  Latency   1.139ms  8.174ms   7.079ms  1.722797s
  RPS       9780.67  12014.57  1463.03  13947.46

Latency Percentile:
  P50       P75      P90       P95       P99      P99.9    P99.99
  7.17ms  9.002ms  12.793ms  15.808ms  22.522ms  42.31ms  51.322ms

Latency Histogram:
  2.932ms      2134   1.07%
  7.465ms    178464  89.23%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  12.784ms    14127   7.06%  ■■■
  18.454ms     3649   1.82%  ■
  26.166ms     1452   0.73%
  31.435ms       60   0.03%
  61.853ms      113   0.06%
  1.562749s       1   0.00%
```

### 连 elasticproxy 压测结果

```sh
[root@localhost esproxytest]# BLOW_STATUS=-201 berf :2900/p1/_doc -b p20w.txt:line  -opt eval,json -vv
Log details to: ./blow_20220701014838_3701546041.log
Berf benchmarking http://127.0.0.1:2900/p1/_doc using 100 goroutine(s), 32 GoMaxProcs.
@Real-time charts is on http://127.0.0.1:28888

Summary:
  Elapsed                 13.111s
  Count/RPS      200000 15253.521
    201          200000 15253.521
  ReadWrite    44.296 40.080 Mbps
  Connections                 100

Statistics    Min       Mean    StdDev      Max
  Latency   1.301ms   6.456ms   3.483ms  988.964ms
  RPS       12052.16  15274.48  1110.53  16268.07

Latency Percentile:
  P50        P75      P90      P95       P99      P99.9     P99.99
  6.002ms  7.151ms  8.735ms  10.508ms  16.083ms  32.187ms  63.162ms

Latency Histogram:
  5.301ms   118596  59.30%  ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  7.122ms    62182  31.09%  ■■■■■■■■■■■■■■■■■■■■■
  9.061ms    11359   5.68%  ■■■■
  12.706ms    5305   2.65%  ■■
  17.629ms    2039   1.02%  ■
  21.128ms     386   0.19%
  25.413ms      31   0.02%
  43.747ms     102   0.05%
```
