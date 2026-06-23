# Mirror

## Feature

- [/douyu/](https://mirror.flxxyz.com/douyu/api/RoomApi/room/452628)
- [/gist/](https://mirror.flxxyz.com/gist/flxxyz/b338666ba7e8cd040b78e667976bf34a.js)
- [/githubraw/](https://mirror.flxxyz.com/githubraw/flxxyz/flxxyz/master/README.md)
- [/gistraw/](https://mirror.flxxyz.com/gistraw/flxxyz/b7ec986055f06269960c1bdf7af66bec/raw/ce7a4ab952d67a13f8bd7c35ede4dfebb9219b9b/CheckIPvNSupport.go)
- [/githubassets/](https://mirror.flxxyz.com/githubassets/apple-touch-icon-144x144.png)
- [/direct/](https://mirror.flxxyz.com/direct/https://github.com/flxxyz/flxxyz/raw/refs/heads/master/README.md)

## Docker

```bash
docker compose up --build -d   # 本地构建并启动（代码变动后重跑此命令重建镜像）
docker compose down            # 停止
```

端口等配置写到 `.env`（如 `PORT=9000`），改完重跑 `up` 生效。host 网络模式，应用直接听宿主机端口。

## Environmental variable

| 名称                 | 用途                          |
|:-------------------|:----------------------------|
| SITE_URL           | 用于镜像 gist js 中出现的地址          |
| HTTP_PROXY         | 用于本地调试(无法访问gist的情况)          |
| HOST               | 用于服务对外监听的地址                 |
| PORT               | 用于服务对外暴露的端口                 |
| GOMAXPROCS         | 不用设置为1，因为没有任何的并发性           |
| GOGC               | 数字越小，达到触发GC的条件越快            |
| GOMEMLIMIT         | 限制软内存用量，单位MiB               |
| CACHE_GIST         | gist 缓存有效期，Go duration，默认 1m |
| CACHE_GITHUBASSETS | githubassets 缓存有效期，默认 30m   |
| CACHE_GITHUBRAW    | githubraw 缓存有效期，默认 1m       |
| CACHE_DOUYU        | douyu 缓存有效期，默认 5s           |
| CACHE_DIRECT       | direct 缓存有效期，默认 1m          |
