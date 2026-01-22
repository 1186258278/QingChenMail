document.addEventListener('DOMContentLoaded', () => {
    // 自动注入 i18n.js (如果页面没有)
    if (!document.querySelector('script[src*="i18n.js"]')) {
        const script = document.createElement('script');
        script.src = '/dashboard/js/i18n.js';
        script.async = true; // 异步加载
        document.head.appendChild(script);
    }

    // 渲染侧边栏
    const path = window.location.pathname;
    const navItems = [
        { name: '仪表盘', i18n: 'nav.dashboard', icon: 'M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z', link: '/dashboard/' },
        { name: '发送中心', i18n: 'nav.send', icon: 'M12 19l9 2-9-18-9 18 9-2zm0 0v-8', link: '/dashboard/send.html' },
        { name: '发送通道', i18n: 'nav.smtp', icon: 'M13 10V3L4 14h7v7l9-11h-7z', link: '/dashboard/smtp.html' },
        { name: '联系人', i18n: 'nav.contacts', icon: 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z', link: '/dashboard/contacts.html' },
        { name: '营销任务', i18n: 'nav.campaigns', icon: 'M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z', link: '/dashboard/campaigns.html' },
        { name: '收件箱', i18n: 'nav.inbox', icon: 'M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4', link: '/dashboard/inbox.html' },
        { name: '域名管理', i18n: 'nav.domains', icon: 'M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9', link: '/dashboard/domains.html' },
        { name: '密钥管理', i18n: 'nav.keys', icon: 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z', link: '/dashboard/keys.html' },
        { name: '邮件模板', i18n: 'nav.templates', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z', link: '/dashboard/templates.html' },
        { name: '发送日志', i18n: 'nav.logs', icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01', link: '/dashboard/logs.html' },
        { name: '文件管理', i18n: 'nav.files', icon: 'M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z', link: '/dashboard/files.html' },
        { name: 'API 文档', i18n: 'nav.api', icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4', link: '/dashboard/api_docs.html' },
        { name: '使用指南', i18n: 'nav.guide', icon: 'M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253', link: '/dashboard/guide.html' },
        { name: '系统设置', i18n: 'nav.settings', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z', link: '/dashboard/settings.html' },
    ];

    const sidebar = `
    <aside class="w-72 bg-white/80 backdrop-blur-xl border-r border-gray-200 fixed h-full z-20 flex flex-col transition-all duration-300">
        <div class="h-20 flex items-center px-8">
            <div class="w-8 h-8 bg-blue-600 rounded-xl flex items-center justify-center mr-3 shadow-lg shadow-blue-500/30">
                <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z"></path></svg>
            </div>
            <div>
                <h1 class="text-xl font-bold tracking-tight text-gray-900" data-i18n="common.title">晴辰云邮</h1>
                <p class="text-xs text-gray-500" data-i18n="common.subtitle">QingChen Cloud</p>
            </div>
        </div>
        
        <nav class="flex-1 px-4 space-y-2 py-4 overflow-y-auto">
            ${navItems.map(item => {
                const isActive = path === item.link || (item.link !== '/dashboard/' && path.startsWith(item.link));
                return `
                <a href="${item.link}" class="flex items-center px-4 py-3.5 rounded-2xl transition-all duration-200 group ${isActive ? 'bg-blue-50 text-blue-600 shadow-sm' : 'text-gray-500 hover:bg-gray-50 hover:text-gray-900'}">
                    <svg class="w-5 h-5 mr-3 transition-colors ${isActive ? 'text-blue-600' : 'text-gray-400 group-hover:text-gray-600'}" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="${item.icon}"></path></svg>
                    <span class="font-medium text-sm" data-i18n="${item.i18n}">${item.name}</span>
                    ${isActive ? '<div class="ml-auto w-1.5 h-1.5 rounded-full bg-blue-600"></div>' : ''}
                </a>
            `}).join('')}
        </nav>
        
        <div class="p-4 border-t border-gray-100 space-y-4">
            <!-- 语言切换 -->
            <div class="flex items-center justify-center space-x-2 bg-gray-50 p-2 rounded-xl">
                <button onclick="I18n.changeLanguage('zh-CN')" class="text-xs px-2 py-1 rounded transition-colors ${!localStorage.getItem('locale') || localStorage.getItem('locale') === 'zh-CN' ? 'bg-white shadow text-blue-600 font-bold' : 'text-gray-500 hover:text-gray-700'}">中文</button>
                <button onclick="I18n.changeLanguage('en')" class="text-xs px-2 py-1 rounded transition-colors ${localStorage.getItem('locale') === 'en' ? 'bg-white shadow text-blue-600 font-bold' : 'text-gray-500 hover:text-gray-700'}">EN</button>
            </div>

            <div class="px-4 text-xs text-gray-400 leading-relaxed">
                <p data-i18n="common.copyright">&copy; 2026 武汉晴辰天下网络科技有限公司</p>
                <a href="https://qingchencloud.com/" target="_blank" class="text-blue-500 hover:text-blue-600" data-i18n="common.website">qingchencloud.com</a>
            </div>
            <button onclick="Auth.logout()" class="flex items-center w-full px-4 py-3 text-sm font-medium text-gray-500 hover:bg-red-50 hover:text-red-600 rounded-2xl transition-colors">
                <svg class="w-5 h-5 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"></path></svg>
                <span data-i18n="common.logout">退出登录</span>
            </button>
        </div>
    </aside>
    
    <div class="ml-72 min-h-screen bg-[#F5F7FA] p-8 transition-all duration-300" id="main-content">
        <!-- Content -->
    </div>`;

    const bodyContent = document.body.innerHTML;
    document.body.innerHTML = sidebar;
    document.getElementById('main-content').innerHTML = bodyContent;
    document.body.classList.remove('hidden');
    
    // 全局样式注入
    const style = document.createElement('style');
    style.innerHTML = `
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
        .glass-card { background: rgba(255, 255, 255, 0.7); backdrop-filter: blur(20px); border: 1px solid rgba(255, 255, 255, 0.5); }
    `;
    document.head.appendChild(style);

    // 如果 I18n 已经准备好，重新渲染一下侧边栏
    if (typeof I18n !== 'undefined' && I18n.isReady) {
        I18n.render();
    }
});
