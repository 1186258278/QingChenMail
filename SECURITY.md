# Security Policy

## Supported Versions

Only the latest version of QingChen Mail is currently supported with security updates.

| Version | Supported          |
| ------- | ------------------ |
| Latest  | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of QingChen Mail seriously. If you have found a security vulnerability, please do NOT open a public issue.

Instead, please send an email to **keh5@vip.qq.com**.

We will try to review your report as soon as possible and get back to you.

## Sensitive Configuration

Please ensure you never commit the following files to a public repository:
*   `config.json` (Contains your JWT secrets and private keys)
*   `goemail.db` (Contains your user data)

We have already configured `.gitignore` to exclude these files by default, but please be careful when force-adding files.
