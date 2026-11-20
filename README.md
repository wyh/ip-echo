# ip-echo

极简高性能外网 IP 返回服务（Go 标准库零依赖，静态编译约 6MB，常驻内存约 5~8MB），专用于部署在阿里云 SLB/ALB + Caddy 后面，配合 `ddns-ip-monitor` 每秒轮询。

## 代理头过滤逻辑

依次按以下顺序提取真实外网 IP，并自动过滤内网 / 回环 / 链路本地 / 阿里云 SLB 常用的 `100.64.0.0/10`（CGNAT）地址：

1. `X-Forwarded-For`（从左到右取第一个公网 IP）
2. `X-Real-IP`
3. `Ali-Cdn-Real-Ip`
4. `CF-Connecting-IP`
5. 兜底 `RemoteAddr`

## 部署方式（推荐直接 systemd 单二进制，几乎零开销）

### 1. 编译静态二进制（在有 Go 或 Docker 的机器上）

```bash
# 方式 A：本地有 Go
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ip-echo .

# 方式 B：用 Docker 编译并提取二进制（云主机不用装 Go 也不用常驻跑 Docker）
docker build -t ip-echo:build .
docker create --name ip-echo-tmp ip-echo:build
docker cp ip-echo-tmp:/ip-echo ./ip-echo
docker rm ip-echo-tmp

# 方式 C：使用 GitHub Actions（.github/workflows/build.yml）
# 1) 每次 push 会上传到 Actions Artifacts（网页下载 ip-echo-binaries.zip）
# 2) 打 tag 推送（如 git tag v0.1.0 && git push --tags）会自动发布到 GitHub Release，云主机可直接下载：
#    curl -L -o /usr/local/bin/ip-echo https://github.com/<你的用户名>/<仓库名>/releases/latest/download/ip-echo-linux-amd64
```

### 2. 安装为 systemd 服务

```bash
sudo install -m 755 ip-echo /usr/local/bin/ip-echo
sudo cp ip-echo.service /etc/systemd/system/ip-echo.service
sudo systemctl daemon-reload
sudo systemctl enable --now ip-echo
```

### 3. Caddy 配置

在你的 `Caddyfile` 中加上：

```caddyfile
ip.yourdomain.com {
    reverse_proxy 127.0.0.1:18080
}
```

### 4. 验证

```bash
curl https://ip.yourdomain.com/
# 调试多级代理头过滤情况：
curl https://ip.yourdomain.com/debug
```
