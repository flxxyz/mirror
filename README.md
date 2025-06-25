# Mirror

## Feature

- gist

## Environmental variable

| 名称         | 用途                  |
|:-----------|:--------------------|
| SITE_URL   | 用于镜像 gist js 中出现的地址 |
| HTTP_PROXY | 用于本地调试(无法访问gist的情况) |
| HOST       | 用于服务对外监听的地址         |
| PORT       | 用于服务对外暴露的端口         |
| GOMAXPROCS | 不用设置为1，因为没有任何的并发性   |
| GOGC       | 数字越小，达到触发GC的条件越快    |
| GOMEMLIMIT | 限制软内存用量，单位MiB       |
