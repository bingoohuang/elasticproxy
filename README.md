# ElasticProxy

This little program acts as a read-only proxy for ElasticSearch. It is intended to be used as
ElasticSearch server for Kibana, ensuring that Kibana can only perform GET calls on ElasticSearch.


1. `docker-compose up && docker-compose rm -fsv`
2. `jj -gu 'name=@姓名' 'sex=@random(男,女)' 'addr=@地址' 'idcard=@身份证' | gurl POST :2900/person/doc`
3. `gurl http://127.0.0.1:2900/_search` vs `http://127.0.0.1:9200/_search`
4. 