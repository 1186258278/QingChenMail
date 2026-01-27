# 晴辰云邮 (QingChen Mail)

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8.svg)](https://golang.org)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com/1186258278/QingChenMail/actions)
[![Company](https://img.shields.io/badge/Company-晴辰云-orange.svg)](https://qingchencloud.com)

> **企业级自建邮件营销与投递系统**

[中文](README.md) | [English](docs/README_en.md) | [部署指南](docs/INSTALL_zh-CN.md)

![使用指南](docs/image/zh/14.png)

---

**晴辰云邮** 是一款专为企业和开发者打造的轻量级、高性能邮件营销系统。它集成了邮件发送、接收、转发与数据统计功能，支持 **直连投递 (Direct Send)** 与 **SMTP 中继** 双模式，让您彻底摆脱昂贵的第三方 EDM 服务限制。

✨ **核心亮点**：`开箱即用` · `子域名隔离` · `自动 DKIM/SPF` · `隐私优先`

## 🚀 核心特性

*   **全可视化配置**: 仪表盘、域名、SMTP、模板、密钥、系统设置全界面管理。
*   **企业级发信引擎**: 
    *   **智能路由**: 支持直连投递 (Direct) 与第三方中继 (SMTP Relay) 自动切换。
    *   **高送达率**: 自动生成 SPF/DKIM/DMARC 记录，支持 **子域名隔离发信** (如 `support@mail.example.com`)，保护主域名信誉。
    *   **异步队列**: 内置高性能消息队列，支持失败自动重试与并发控制。
*   **邮件网关能力**: 
    *   **SMTP 接收服务**: 自带 SMTP Server，支持接收发往域名邮箱的邮件。
    *   **智能转发**: 支持通配符/前缀匹配规则，将入站邮件自动转发至您的私人邮箱 (如 Gmail/QQ)。
*   **开发者友好**: 
    *   **RESTful API**: 提供标准 API 接口与永久密钥 (`sk_live_...`)，轻松集成到您的业务系统。
    *   **模板引擎**: 支持变量替换 (`{username}`)，实现千人千面的个性化营销。
*   **运维零负担**: 
    *   **单文件部署**: 编译后仅一个二进制文件，无复杂依赖。
    *   **自动校准**: 每次重启自动检查并修复数据库结构，自动清理孤儿数据。
*   **安全加固**: 支持 HTTPS (SSL)、密码加密存储及敏感配置脱敏。

## 📸 系统截图

| 仪表盘 | 营销任务 |
| :---: | :---: |
| ![仪表盘](docs/image/zh/02.png) | ![营销任务](docs/image/zh/06.png) |

| 联系人 | 邮件模板 |
| :---: | :---: |
| ![联系人](docs/image/zh/05.png) | ![模板](docs/image/zh/10.png) |

| 发送通道 | 域名管理 |
| :---: | :---: |
| ![发送通道](docs/image/zh/04.png) | ![域名管理](docs/image/zh/08.png) |

| 收件箱 | 密钥管理 |
| :---: | :---: |
| ![收件箱](docs/image/zh/07.png) | ![密钥管理](docs/image/zh/09.png) |

| 发送日志 | 登录页面 |
| :---: | :---: |
| ![发送日志](docs/image/zh/11.png) | ![登录页面](docs/image/zh/01.png) |

| 文件管理 | 系统设置 |
| :---: | :---: |
| ![文件管理](docs/image/zh/12.png) | ![系统设置](docs/image/zh/15.png) |

## 🛠️ 快速开始

**1. 下载运行**

前往 [Releases](https://github.com/1186258278/QingChenMail/releases) 下载对应平台的二进制文件，直接运行：

```bash
./goemail          # Linux/macOS
goemail.exe        # Windows
```

**2. 访问后台**

打开浏览器访问：`http://localhost:9901`

*   **默认账号**: `admin`
*   **默认密码**: `123456` (**⚠️ 首次登录后请立即修改密码**)

**3. 生产部署**

推荐配置 HTTPS 并绑定公网 IP。详见 👉 **[部署指南](docs/INSTALL_zh-CN.md)**

## 📚 API 对接

系统内置交互式 API 文档，包含 **AI 接入提示词** 和 **动态代码示例**。

1.  登录后台，点击左侧菜单 **「API 文档」**
2.  复制提示词给 ChatGPT/Cursor，快速生成对接代码
3.  接口地址: `POST /api/v1/send` (支持 `Authorization: Bearer sk_...`)

## ⚙️ 高级配置

进入后台 **「系统设置」** 页面，您可以：

*   修改服务监听地址 (Host) 和端口 (Port)
*   开启 HTTPS (SSL) 并上传证书
*   配置系统默认发信域名和 DKIM 签名
*   重置 JWT 密钥以强制下线所有用户

## 🛡️ 安全须知

如果您计划将本项目托管到公共仓库，请注意：

*   ❌ **切勿上传** `config.json` (包含私钥和 JWT 密钥)
*   ❌ **切勿上传** `goemail.db` (包含用户数据和邮件日志)
*   ❌ **切勿上传** `data/` 和 `logs/` 目录

本项目已提供标准的 `.gitignore` 文件来排除这些敏感文件。

## ⚡ 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go + Gin + GORM + SQLite |
| 前端 | HTML5 + TailwindCSS + Chart.js |
| 邮件 | go-mail + go-msgauth (DKIM) |

## 📄 开源协议

本项目采用 [MIT License](LICENSE) 许可证。您可以免费使用、修改和分发，但请保留版权声明。

---

© 2026 武汉晴辰天下网络科技有限公司 版权所有

官网: [qingchencloud.com](https://qingchencloud.com)
