# elasticproxy

This little program acts as a http proxy for ElasticSearch.

![image](elasticproxy.svg)

1. build: `make install`
2. initialize: `elasticproxy -init` (this will create ctl shell script for convenience of startup or stop)
3. start [httplive](https://github.com/bingoohuang/httplive), for backup mocking: `httplive -l`
   , [download httplive](http://7.d5k.co/httplive/dl/)
4. start: `./ctl start -b http://127.0.0.1:5003/backup`
5. tail log: `./ctl tail`

## usage

```sh
$ elasticproxy -h
Usage of elasticproxy:
  -backups (-b) value   backup elastic URLs
  -fla9 string  Flags config file, a scaffold one will created when it does not exist.
  -init Create initial ctl and exit
  -port (-p) int        port to listen on (default 2900)
  -primary (-a) string  primary elastic URL (default "http://127.0.0.1:9200/")
  -version (-v) Create initial ctl and exit
```

```sh
$ ./ctl tail    
2022-04-27 17:07:23.604 [INFO ] 16843 --- [1    ] [-]  : log file created:~/logs/elasticproxy/elasticproxy.log
2022-04-27 17:07:31.155 [INFO ] 16843 --- [23   ] [-]  : {"direction":"primary","duration":"29.512085ms","method":"POST","path":"/person/doc","remote_addr":"127.0.0.1:50693","status":201,"target":"http://127.0.0.1:9200/person/doc"}
2022-04-27 17:07:31.157 [INFO ] 16843 --- [22   ] [-]  : {"direction":"backup","duration":"2.130054ms","status":200,"target":"http://127.0.0.1:5003/backup/person/doc"}
2022-04-27 17:07:42.886 [INFO ] 16843 --- [34   ] [-]  : {"direction":"primary","duration":"21.780433ms","method":"GET","path":"/_search","remote_addr":"127.0.0.1:50736","status":200,"target":"http://127.0.0.1:9200/_search"}
2022-04-27 17:07:43.374 [INFO ] 16843 --- [34   ] [-]  : {"direction":"primary","duration":"3.710797ms","method":"GET","path":"/favicon.ico","remote_addr":"127.0.0.1:50736","status":200,"target":"http://127.0.0.1:9200/favicon.ico"}
```

## help commands

1. `docker-compose up && docker-compose rm -fsv`
1. `jj -gu 'name=@姓名' 'sex=@random(男,女)' 'addr=@地址' 'idcard=@身份证' | gurl POST :2900/person/doc`
   , [download jj](http://7.d5k.co/httplive/dl/)
1. `gurl http://127.0.0.1:2900/_search` vs `http://127.0.0.1:9200/_search`
   , [download gurl](http://7.d5k.co/httplive/dl/)
