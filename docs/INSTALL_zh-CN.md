# 编译与部署指南

本文档将指导您如何从源码编译 QingChen Mail，并在 Linux 服务器上进行生产级部署。

> **最新版本**: v1.1.0 - 新增两步验证 (2FA)、证书管理、数据清理等功能

## 🛠️ 1. 编译指南

### 环境准备
*   **Go**: 版本需 >= 1.21 ([下载地址](https://go.dev/dl/))
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
2.  点击"创建密钥"，您将获得一个以 `sk_live_` 开头的密钥。

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

详细的 API 文档 (包含 Golang/Python/Node.js/Java 代码示例) 请在系统启动后访问后台的 **"API 文档"** 菜单。

---

## 🔐 4. 安全加固

### 4.1 启用两步验证 (2FA)

强烈建议为管理员账户启用两步验证，防止密码泄露导致的未授权访问。

1. 登录后台 → **系统设置** → **安全设置**
2. 点击 "启用两步验证"
3. 使用 Google Authenticator / Microsoft Authenticator 扫描二维码
4. 输入 6 位验证码完成绑定

**忘记两步验证怎么办？**
```bash
# 在服务器上执行
./goemail -reset-totp
```

### 4.2 配置 SSL/TLS 证书

为 STARTTLS 收件和 HTTPS 访问配置证书：

**方式 1: Let's Encrypt 自动申请** (推荐)
1. 后台 → **域名管理** → 打开证书夹
2. 点击 "申请新证书"
3. 选择 DNS 验证方式，按提示添加 TXT 记录

**方式 2: 手动上传证书**
1. 后台 → **域名管理** → 打开证书夹
2. 点击 "上传证书"
3. 粘贴证书内容和私钥

### 4.3 数据清理策略

系统支持自动清理过期数据，避免数据库膨胀：

1. 后台 → **系统设置** → **数据清理**
2. 启用自动清理，设置保留天数
3. 系统将在每天凌晨 3:00 自动执行清理

---

## 🔧 5. 命令行参数

| 参数 | 说明 |
|:---|:---|
| `-reset` | 重置管理员密码为 `123456` |
| `-reset-totp` | 关闭管理员的两步验证 |

```bash
# 忘记密码
./goemail -reset

# 忘记两步验证
./goemail -reset-totp
```

---

## ❓ 6. 常见问题

### Q: 启用 2FA 后无法登录？
A: 在服务器执行 `./goemail -reset-totp` 关闭两步验证。

### Q: Let's Encrypt 证书申请失败？
A: 检查 DNS TXT 记录是否正确添加，等待 DNS 生效 (通常 1-5 分钟)。

### Q: 收件箱邮件显示乱码？
A: 系统已支持 GBK/ISO-8859-1 等编码自动解码。如仍有问题，请提交 Issue。

### Q: 如何备份数据？
A: 后台 → **系统设置** → **安全设置** → **下载完整备份**，会打包 config.json 和数据库文件。
