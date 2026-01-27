# QingChen Mail

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](../LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8.svg)](https://golang.org)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com/1186258278/QingChenMail/actions)
[![Company](https://img.shields.io/badge/Company-QingChen%20Cloud-orange.svg)](https://qingchencloud.com)

> Enterprise-grade self-hosted email marketing and delivery solution.

[中文](../README.md) | [English](README_en.md) | [Installation Guide](INSTALL_zh-CN.md)

![Overview](image/en/14.png)

## Overview

QingChen Mail is a high-performance, lightweight email delivery system written in Go. It is designed to replace expensive third-party EDM services with a secure, private, and controllable self-hosted solution.

It supports dual delivery modes (Direct Send & SMTP Relay), automatic DKIM/SPF configuration, and subdomain isolation, ensuring high deliverability and domain reputation protection.

## Key Features

*   **Dual Delivery Modes**:
    *   **Direct Send**: Built-in DNS resolver and MX delivery engine. Supports automatic DKIM signing.
    *   **SMTP Relay**: Seamless integration with third-party relays (Gmail, Outlook, AWS SES, etc.).
*   **High Deliverability**:
    *   **Subdomain Isolation**: Support sending via subdomains (e.g., `support@mail.example.com`) to protect root domain reputation.
    *   **Auto Authentication**: Automated generation of SPF, DKIM, and DMARC records.
*   **Inbound Routing**: Built-in SMTP server for receiving emails with intelligent forwarding rules (wildcard/prefix matching).
*   **Visual Dashboard**: comprehensive management of domains, templates, keys, and logs.
*   **Developer Ready**:
    *   RESTful API with persistent API keys (`sk_live_...`).
    *   Template engine with variable substitution support.
*   **Zero-Maintenance**:
    *   Single binary deployment.
    *   Auto-calibration of database schema on startup.

## Screenshots

| Dashboard | Marketing |
| :---: | :---: |
| ![Dashboard](image/en/02.png) | ![Marketing](image/en/06.png) |

| Contacts | Templates |
| :---: | :---: |
| ![Contacts](image/en/05.png) | ![Templates](image/en/10.png) |

| SMTP Relays | Domains |
| :---: | :---: |
| ![SMTP](image/en/04.png) | ![Domains](image/en/08.png) |

| Inbox | API Keys |
| :---: | :---: |
| ![Inbox](image/en/07.png) | ![Keys](image/en/09.png) |

| Logs | Login |
| :---: | :---: |
| ![Logs](image/en/11.png) | ![Login](image/en/01.png) |

| Files | Settings |
| :---: | :---: |
| ![Files](image/en/12.png) | ![Settings](image/en/15.png) |

## Quick Start

### 1. Download & Run
Download the latest release for your platform and run it directly:

```bash
./goemail
```

### 2. Access Dashboard
Open your browser and visit: `http://localhost:9901`

*   **Default User**: `admin`
*   **Default Password**: `123456` (⚠️ Please change it immediately after login)

### 3. Configuration
For production environments, it is recommended to configure HTTPS and bind to a public IP. Edit `config.json` or use the Settings page in the dashboard.

## Security

**Sensitive Data Warning**:
*   Do NOT commit `config.json` or `goemail.db` to public repositories.
*   Use `config.example.json` as a template for version control.

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.

---

© 2026 Wuhan QingChen TianXia Network Technology Co., Ltd. All rights reserved.
