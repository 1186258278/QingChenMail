# GoEmail Pro 多语言开发规范

本项目采用 **前后端分离** 的国际化 (i18n) 策略。
前端基于原生 JS 实现轻量级加载，后端遵循 "English First" 原则。

## 1. 核心架构

- **后端 (Go)**: API 错误信息统一返回 **英文** (如 `Invalid password`)，不处理多语言逻辑，保持轻量。
- **前端 (JS)**: 
  - `static/js/i18n.js`: 核心引擎，负责加载语言包和渲染页面。
  - `static/locales/`: 存放 JSON 格式的语言包。
  - **自动翻译**: 前端 `common.js` 会拦截 API 的英文错误，并在 UI 层展示对应的翻译。

## 2. 目录结构

```text
static/
├── js/
│   └── i18n.js            # i18n 核心库
└── locales/               # 语言包根目录
    ├── zh-CN/             # 中文语言包
    │   ├── common.json    # 全局通用 (侧边栏、按钮、错误提示)
    │   ├── login.json     # 登录页
    │   ├── dashboard.json # 仪表盘
    │   └── ...
    └── en/                # 英文语言包 (结构必须与 zh-CN 一致)
        ├── common.json
        └── ...
```

## 3. 开发流程

### 3.1 新增页面支持

假设你要开发一个新页面 `profile.html`。

**Step 1: 声明模块**
在 HTML 的 `<body>` 标签中添加 `data-i18n-module` 属性：
```html
<!-- 这会告诉 i18n.js 自动加载 locales/{lang}/profile.json -->
<body data-i18n-module="profile">
```

**Step 2: 创建语言包**
创建 `static/locales/zh-CN/profile.json` 和 `en/profile.json`。
```json
// zh-CN/profile.json
{
    "profile.title": "个人资料",
    "profile.save_btn": "保存修改"
}
```

**Step 3: HTML 静态文本**
使用 `data-i18n` 属性：
```html
<h1 data-i18n="profile.title">个人资料</h1>
<button data-i18n="profile.save_btn">保存修改</button>
```

**Step 4: 输入框 Placeholder**
使用 `data-i18n-attr` 属性，格式为 `属性名:Key`：
```html
<input type="text" placeholder="请输入昵称" data-i18n-attr="placeholder:profile.nickname_ph">
```

### 3.2 JS 动态文本

在 JavaScript 中使用 `I18n.t(key, params)`：

```javascript
// 简单翻译
alert(I18n.t('common.success'));

// 带变量 (需要在 json 中定义: "hello": "你好, {name}")
alert(I18n.t('profile.hello', { name: 'Admin' }));
```

**注意**: 确保代码在 `i18n-ready` 事件后执行，或者在 `DOMContentLoaded` 后执行（`i18n.js` 会自动处理初始化）。

### 3.3 后端错误处理

后端返回标准英文错误，例如 `User not found`。
前端在 `common.json` 中定义映射：
```json
{
    "error.user_not_found": "用户不存在"
}
```
然后在 `static/js/common.js` 的 `errorMap` 中添加映射关系。

## 4. Key 命名规范

- **Common**: `common.xxx` (如 `common.confirm`, `common.cancel`)
- **Page**: `page_name.section.element` (建议扁平化，如 `login.username`, `domains.add_btn`)
- **Error**: `error.code_name` (如 `error.invalid_token`)

## 5. 调试

- **切换语言**: 点击界面上的语言切换按钮，或在控制台执行 `I18n.changeLanguage('en')`。
- **强制刷新**: 如果修改了 JSON 未生效，可能是浏览器缓存，请清除缓存或在 URL 后加随机参。
