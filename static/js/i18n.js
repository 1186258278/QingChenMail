/**
 * 轻量级国际化模块 (No Dependency)
 * 支持：
 * 1. 自动检测语言
 * 2. 异步加载语言包 (Common + Page Module)
 * 3. 自动扫描 [data-i18n] 并替换文本
 * 4. 动态 t(key) 函数
 */
const I18n = {
    locale: 'zh-CN', // 默认语言
    translations: {}, // 存储合并后的翻译字典
    isReady: false,

    // 初始化
    init: async function() {
        // 1. 确定语言
        const saved = localStorage.getItem('locale');
        if (saved) {
            this.locale = saved;
        } else {
            // 自动检测浏览器语言
            const navLang = navigator.language || navigator.userLanguage; 
            if (navLang && !navLang.startsWith('zh')) {
                this.locale = 'en';
            }
        }
        
        // 设置 HTML lang 属性
        document.documentElement.lang = this.locale;

        // 2. 确定需要加载的模块
        // 默认加载 common, 页面可通过 <body data-i18n-module="login"> 指定专属模块
        const modules = ['common'];
        const pageModule = document.body.getAttribute('data-i18n-module');
        if (pageModule) modules.push(pageModule);

        // 3. 并行加载语言包
        try {
            const promises = modules.map(m => 
                fetch(`/dashboard/locales/${this.locale}/${m}.json?v=${new Date().getTime()}`)
                    .then(res => {
                        if (!res.ok) {
                            console.warn(`Locale file not found: ${m}`);
                            return {};
                        }
                        return res.json();
                    })
                    .catch(err => {
                        console.error(`Failed to load locale: ${m}`, err);
                        return {};
                    })
            );

            const results = await Promise.all(promises);
            
            // 合并所有结果
            results.forEach(pkg => {
                Object.assign(this.translations, pkg);
            });

            this.isReady = true;
            this.render();
            
            // 触发自定义事件，通知页面 JS 可以执行依赖翻译的逻辑了
            document.dispatchEvent(new Event('i18n-ready'));

        } catch (e) {
            console.error('I18n init error:', e);
        }
    },

    // 翻译函数: t('common.ok') -> "确定"
    t: function(key, params = {}) {
        const val = this.translations[key] || key;
        // 简单的变量替换 {name}
        return val.replace(/{(\w+)}/g, (match, k) => {
            return typeof params[k] !== 'undefined' ? params[k] : match;
        });
    },

    // 渲染页面上所有 data-i18n 元素
    render: function() {
        document.querySelectorAll('[data-i18n], [data-i18n-attr], [data-i18n-html]').forEach(el => {
            const key = el.getAttribute('data-i18n');
            const htmlKey = el.getAttribute('data-i18n-html');
            // 支持 placeholder 翻译 data-i18n-attr="placeholder:login.username"
            const attrConfig = el.getAttribute('data-i18n-attr');
            
            if (key) {
                // 安全修复：默认使用 textContent 防止 XSS
                el.textContent = this.t(key);
            }
            
            // 明确需要 HTML 渲染的使用 data-i18n-html (谨慎使用)
            if (htmlKey) {
                el.innerHTML = this.t(htmlKey);
            }

            if (attrConfig) {
                // 格式: "placeholder:key1;title:key2"
                attrConfig.split(';').forEach(pair => {
                    const [attr, k] = pair.split(':');
                    if (attr && k) {
                        el.setAttribute(attr, this.t(k));
                    }
                });
            }
        });
    },

    // 切换语言
    changeLanguage: function(lang) {
        if (lang === this.locale) return;
        localStorage.setItem('locale', lang);
        location.reload();
    }
};

// 显式暴露给全局 window 对象
window.I18n = I18n;

// 智能启动：如果 DOM 已经加载完成，直接运行；否则等待事件
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => I18n.init());
} else {
    // 已经加载完了 (interactive 或 complete)
    I18n.init();
}
