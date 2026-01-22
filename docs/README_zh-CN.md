# 晴辰云邮 (QingChen Mail)

> **企业级自建邮件营销与投递系统 | Enterprise Self-hosted Email Marketing Solution**

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()
[![Company](https://img.shields.io/badge/Company-QingChen%20Cloud-orange.svg)](https://qingchencloud.com)

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

*   **全可视化配置**: 仪表盘、域名、SMTP、模板、密钥、系统设置全界面管理。
*   **多模式发送**: 
    *   **Direct Send (直连)**: 内置 DNS 查询与 MX 投递，支持 DKIM 自动签名。
    *   **SMTP Relay (中继)**: 支持 Gmail, Outlook, 阿里云, 腾讯云等第三方中继。
    *   **异步队列**: 内置高性能消息队列，支持失败自动重试。
*   **邮件转发**: 
    *   **SMTP 接收服务**: 接收发往域名邮箱的邮件。
    *   **灵活匹配规则**: 支持全部/精确/前缀三种匹配模式。
    *   **自动转发**: 将收到的邮件转发到指定的外部邮箱 (如 Gmail)。
*   **企业级集成**: 
    *   **模板引擎**: 支持服务器端模板渲染，通过 API 传入变量自动生成邮件内容。
    *   **API Key 管理**: 生成永久有效的 `sk_live_...` 密钥，方便后端服务对接。
    *   **附件增强**: 支持 Base64 上传或**远程 URL 自动下载**，并提供**文件管理**与留痕功能。
*   **运维友好**: 
    *   **自定义监听**: 支持绑定 IP 和自定义端口。
    *   **HTTPS 支持**: 一键配置 SSL 证书，支持反向代理或直接暴露。
    *   **数据备份**: 支持一键导出数据库和配置文件。
*   **高送达率**: 自动生成 SPF/DKIM/DMARC 配置向导。

## 🛠️ 快速开始

1.  **启动**: 运行编译后的 `./goemail.exe`。
2.  **访问**: 打开 [http://localhost:9901](http://localhost:9901)。
3.  **登录**: 默认 `admin` / `123456` (**⚠️ 首次登录后请务必在“系统设置”中修改密码**)。

👉 **[查看详细编译与部署指南 (Linux/Windows)](INSTALL_zh-CN.md)**

## 🛡️ 安全注意事项 (部署前必读)

如果您计划将本项目托管到 GitHub 或其他公共仓库，请务必执行以下清理步骤：
1.  **切勿上传 `config.json`**：其中包含您的私钥和 JWT 密钥。请使用 `config.example.json` 作为模板。
2.  **切勿上传 `goemail.db`**：其中包含您的所有用户数据和邮件日志。
3.  **切勿上传 `data/` 和 `logs/` 目录**。

本项目已提供标准的 `.gitignore` 文件，请确保它生效。

## 📚 开发对接 (API)

系统内置了交互式 API 文档，包含 **AI 接入提示词** 和 **动态代码示例**。

1.  登录后台，点击左侧菜单 **“API 文档”**。
2.  复制提示词给 ChatGPT/Cursor，快速生成对接代码。
3.  接口地址: `POST /api/v1/send` (支持 `Authorization: Bearer sk_...`)。

## ⚙️ 高级配置

进入后台 **“系统设置”** 页面，您可以：
*   修改服务监听地址 (Host) 和端口 (Port)。
*   开启 HTTPS (SSL) 并上传证书。
*   配置系统默认发信域名和 DKIM 签名。
*   重置 JWT 密钥以强制下线所有用户。

## ⚡ 技术栈

*   **Core**: Golang + Gin + GORM + SQLite
*   **UI**: HTML5 + TailwindCSS + Chart.js
*   **Mail**: go-mail + go-msgauth (DKIM)

## 📄 版权与许可

© 2026 武汉晴辰天下网络科技有限公司 (Wuhan QingChen TianXia Network Technology Co., Ltd.) 版权所有。

本项目采用 [MIT License](LICENSE) 许可证。您可以免费使用、修改和分发，但请保留版权声明。

官方网站: [https://qingchencloud.com/](https://qingchencloud.com/)
