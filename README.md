# ElasticProxy

This little program acts as a read-only proxy for ElasticSearch. It is intended to be used as
ElasticSearch server for Kibana, ensuring that Kibana can only perform GET calls on ElasticSearch.

1. build: `make install`
2. initialize: `elasticproxy -init`
3. start: `./ctl start`
4. tail log: `./ctl tail`

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
2022-04-27 16:34:15.091 [INFO ] 15152 --- [1    ] [-]  : log file created:~/logs/elasticproxy/elasticproxy.log
2022-04-27 16:35:52.730 [INFO ] 15152 --- [18   ] [-]  : {"duration":"25.620513ms","method":"POST","path":"/person/doc","remote_addr":"127.0.0.1:59931","status":201}
2022-04-27 16:36:47.631 [INFO ] 15152 --- [34   ] [-]  : {"duration":"40.725229ms","method":"GET","path":"/_search","remote_addr":"127.0.0.1:60132","status":200}
2022-04-27 16:36:48.274 [INFO ] 15152 --- [34   ] [-]  : {"duration":"7.867817ms","method":"GET","path":"/favicon.ico","remote_addr":"127.0.0.1:60132","status":200}
```

## help commands

1. `docker-compose up && docker-compose rm -fsv`
1. `jj -gu 'name=@姓名' 'sex=@random(男,女)' 'addr=@地址' 'idcard=@身份证' | gurl POST :2900/person/doc`
1. `gurl http://127.0.0.1:2900/_search` vs `http://127.0.0.1:9200/_search`
