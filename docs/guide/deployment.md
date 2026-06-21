# 部署指南

## Docker Compose

```yaml
services:
  ccx:
    image: crpi-i19l8zl0ugidq97v.cn-hangzhou.personal.cr.aliyuncs.com/bene/ccx:latest
    ports:
      - '3000:3000'
    volumes:
      - ./.config:/app/.config
    environment:
      - ENV=production
      - PROXY_ACCESS_KEY=your-proxy-key
      # 额外代理访问密钥（可选，逗号分隔；启用后必须设置独立 ADMIN_ACCESS_KEY）
      # - EXTRA_PROXY_ACCESS_KEYS=extra-proxy-key-1,extra-proxy-key-2
      # 管理 API 独立密钥（可选，未设置时回退到 PROXY_ACCESS_KEY）
      # - ADMIN_ACCESS_KEY=your-admin-secret-key
    restart: unless-stopped
```

## 系统服务

### Linux (systemd)

```ini
[Unit]
Description=CCX AI API Gateway
After=network.target

[Service]
Type=simple
ExecStart=/opt/ccx/ccx
WorkingDirectory=/opt/ccx
Environment=PROXY_ACCESS_KEY=your-proxy-key
# 额外代理访问密钥（可选，逗号分隔；启用后必须设置独立 ADMIN_ACCESS_KEY）
#Environment=EXTRA_PROXY_ACCESS_KEYS=extra-proxy-key-1,extra-proxy-key-2
# 管理 API 独立密钥（可选，未设置时回退到 PROXY_ACCESS_KEY）
#Environment=ADMIN_ACCESS_KEY=your-admin-secret-key
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### macOS (launchd)

参考 `docs/service/com.ccx.gateway.plist` 配置文件。

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | 3000 | 服务端口 |
| `ENV` | production | 运行环境 |
| `PROXY_ACCESS_KEY` | - | 代理访问密钥（必填） |
| `EXTRA_PROXY_ACCESS_KEYS` | - | 额外代理访问密钥（可选，逗号分隔；仅用于代理 API） |
| `ADMIN_ACCESS_KEY` | - | 管理界面密钥（可选；配置额外代理密钥时必填且必须独立） |
| `QUIET_POLLING_LOGS` | true | 静默轮询日志 |
| `MAX_REQUEST_BODY_SIZE_MB` | 50 | 请求体大小限制 |

完整环境变量列表请参考项目根目录的 `ENVIRONMENT.md`。
