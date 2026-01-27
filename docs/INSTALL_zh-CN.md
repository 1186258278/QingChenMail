# 编译与部署指南

本文档将指导您如何从源码编译 QingChen Mail，并在 Linux 服务器上进行生产级部署。

## 🛠️ 1. 编译指南

### 环境准备
*   **Go**: 版本需 >= 1.25 ([下载地址](https://go.dev/dl/))
*   **Git**: 用于拉取代码

### 编译步骤

#### Windows
```powershell
# 下载依赖
go mod tidy

# 编译 (生成 goemail.exe)
go build -o goemail.exe main.go
```

#### Linux (推荐)
如果您在 Windows 上开发，但在 Linux 服务器上部署，请使用交叉编译命令：

```bash
# 启用 CGO 禁用 (推荐，生成静态链接文件，无依赖)
$env:CGO_ENABLED="0"
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -o goemail main.go
```

#### macOS
```bash
go build -o goemail main.go
```

---

## 🚀 2. Linux 部署指南 (CentOS/Ubuntu/Debian)

### 2.1 目录规划
建议将程序部署在 `/opt/goemail` 目录下。

```bash
# 创建目录
mkdir -p /opt/goemail
cd /opt/goemail

# 上传编译好的二进制文件 'goemail' 到此目录
# 上传 static/ 目录到此目录 (必须包含，否则后台无法访问)
# 赋予执行权限
chmod +x goemail
```

### 2.2 Systemd 服务配置 (后台运行)
创建一个 systemd 服务文件，以便开机自启和后台运行。

`vim /etc/systemd/system/goemail.service`

写入以下内容：

```ini
[Unit]
Description=QingChen Mail Service
After=network.target

[Service]
# 根据实际安装路径修改
WorkingDirectory=/opt/goemail
ExecStart=/opt/goemail/goemail
Restart=always
# 推荐使用非 root 用户运行，但如果需要监听 25 端口，则必须用 root，或者使用 setcap
User=root
Group=root

[Install]
WantedBy=multi-user.target
```

**启动服务**：

```bash
systemctl daemon-reload
systemctl enable goemail
systemctl start goemail
systemctl status goemail
```

### 2.3 Nginx 反向代理 (可选，推荐)
为了通过域名安全访问 (如 `https://edm.yourdomain.com`)，建议使用 Nginx。

`vim /etc/nginx/conf.d/goemail.conf`

```nginx
server {
    listen 80;
    server_name edm.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:9901;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

### 2.4 防火墙设置
确保服务器开放了以下端口：
*   **9901** (Web 面板，如果用了 Nginx 则只需开放 80/443)
*   **25** (SMTP 发信与收信，必须开放)

---

## 🔌 3. API 对接指南

QingChen Mail 提供了标准的 RESTful API。

### 获取 API 密钥
1.  登录后台 -> **密钥管理**。
2.  点击“创建密钥”，您将获得一个以 `sk_live_` 开头的密钥。

### 发送邮件示例 (Curl)

```bash
curl -X POST http://localhost:9901/api/v1/send \
  -H "Authorization: Bearer sk_live_xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "to": ["user@example.com"],
    "subject": "Hello from API",
    "html": "<h1>Test Email</h1><p>This is a test.</p>"
  }'
```

详细的 API 文档 (包含 Golang/Python/Node.js/Java 代码示例) 请在系统启动后访问后台的 **“API 文档”** 菜单。
