// 基础配置
const API_BASE = '/api/v1';

// 认证管理
const Auth = {
    getToken: () => localStorage.getItem('token'),
    setToken: (token) => {
        // 生产环境中，最好由后端 Set-Cookie 并指定 HttpOnly
        // 这里作为后备或本地调试
        localStorage.setItem('token', token);
        document.cookie = `token=${token}; path=/; max-age=86400; SameSite=Strict`;
    },
    logout: () => {
        localStorage.removeItem('token');
        document.cookie = 'token=; path=/; max-age=0';
        window.location.href = '/login.html';
    },
    check: () => {
        if (!Auth.getToken()) {
            window.location.href = '/login.html';
            return false;
        }
        return true;
    }
};

// API 请求封装
async function request(endpoint, options = {}) {
    if (!options.headers) options.headers = {};
    const token = Auth.getToken();
    if (token) options.headers['Authorization'] = 'Bearer ' + token;
    
    // 默认 JSON
    if (!options.body && options.method !== 'GET' && options.method !== 'DELETE') {
        options.headers['Content-Type'] = 'application/json';
    }

    try {
        const res = await fetch(API_BASE + endpoint, options);
        if (res.status === 401) {
            Auth.logout();
            return null;
        }
        const data = await res.json();
        if (!res.ok) {
            const error = new Error(data.error || 'Request failed');
            error.data = data; // Attach full response data
            throw error;
        }
        return data;
    } catch (err) {
        console.error(err);
        let msg = err.message;
        
        // 尝试翻译错误信息
        if (typeof I18n !== 'undefined' && I18n.isReady) {
            // 精确匹配
            const errorMap = {
                "Invalid captcha code": "error.invalid_captcha",
                "Captcha code required": "error.captcha_required",
                "Domain already exists": "error.domain_exists",
                "Unauthorized": "error.unauthorized",
                "Invalid credentials": "error.invalid_credentials",
                "Invalid token or API key": "error.invalid_token",
                "Wrong old password": "error.wrong_old_pass",
                "SMTP not found": "error.smtp_not_found",
                "SMTP config not found": "error.smtp_not_found",
                "File not found": "error.file_not_found",
                "File not on disk": "error.file_not_found",
                "Domain not found": "error.domain_not_found",
                "Template not found": "error.template_not_found",
                "Rule not found": "error.rule_not_found",
                "Invalid match_type": "error.invalid_match_type",
                "Invalid forward_to address": "error.invalid_forward",
                "No contacts found": "error.no_contacts",
                "SSL enabled but cert/key file path missing": "error.ssl_config_missing",
                "Failed to generate token": "error.unknown",
                "Bing API failed": "error.bing_failed",
                "Image download failed": "error.bing_failed",
                "File save failed": "error.unknown"
            };

            if (errorMap[msg]) {
                msg = I18n.t(errorMap[msg]);
            } else {
                // 模糊匹配 (前缀)
                if (msg.startsWith("Certificate file not found")) msg = I18n.t('error.cert_not_found') + ": " + msg.split(': ')[1];
                else if (msg.startsWith("Key file not found")) msg = I18n.t('error.key_not_found') + ": " + msg.split(': ')[1];
                else if (msg.startsWith("Failed to queue email")) msg = I18n.t('error.queue_failed') + ": " + msg.split(': ')[1];
            }
        }
        
        showToast(msg, 'error');
        throw err;
    }
}

// 简易 Toast 提示
function showToast(msg, type = 'success') {
    const div = document.createElement('div');
    const color = type === 'success' ? 'bg-green-600' : 'bg-red-600';
    div.className = `fixed bottom-5 right-5 ${color} text-white px-6 py-3 rounded-lg shadow-lg transform transition-all duration-300 translate-y-10 opacity-0 z-50 flex items-center`;
    
    // 使用 textContent 防止 XSS
    const span = document.createElement('span');
    span.textContent = msg;
    div.appendChild(span);
    
    document.body.appendChild(div);
    
    requestAnimationFrame(() => {
        div.classList.remove('translate-y-10', 'opacity-0');
    });

    setTimeout(() => {
        div.classList.add('translate-y-10', 'opacity-0');
        setTimeout(() => div.remove(), 300);
    }, 3000);
}

// 表格排序组件
const TableSort = {
    // 当前排序状态
    state: {},
    
    // 初始化可排序表头
    init: (tableId, onSort) => {
        const table = document.getElementById(tableId);
        if (!table) return;
        
        const headers = table.querySelectorAll('th[data-sort]');
        headers.forEach(th => {
            th.classList.add('cursor-pointer', 'select-none', 'hover:bg-gray-100', 'transition');
            th.innerHTML += `
                <span class="sort-icon ml-1 inline-block transition-transform">
                    <svg class="w-3 h-3 inline text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4"/>
                    </svg>
                </span>
            `;
            
            th.addEventListener('click', () => {
                const field = th.dataset.sort;
                const currentDir = TableSort.state[tableId]?.field === field ? TableSort.state[tableId].dir : null;
                const newDir = currentDir === 'asc' ? 'desc' : 'asc';
                
                // 更新状态
                TableSort.state[tableId] = { field, dir: newDir };
                
                // 更新 UI
                headers.forEach(h => {
                    const icon = h.querySelector('.sort-icon');
                    if (h === th) {
                        icon.innerHTML = newDir === 'asc' 
                            ? '<svg class="w-3 h-3 inline text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7"/></svg>'
                            : '<svg class="w-3 h-3 inline text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>';
                    } else {
                        icon.innerHTML = '<svg class="w-3 h-3 inline text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4"/></svg>';
                    }
                });
                
                // 执行排序回调
                if (onSort) onSort(field, newDir);
            });
        });
    },
    
    // 客户端排序数据
    sortData: (data, field, dir) => {
        return [...data].sort((a, b) => {
            let valA = a[field];
            let valB = b[field];
            
            // 处理日期
            if (field.includes('_at') || field.includes('time') || field.includes('date')) {
                valA = new Date(valA).getTime();
                valB = new Date(valB).getTime();
            }
            // 处理数字
            else if (typeof valA === 'number' || !isNaN(parseFloat(valA))) {
                valA = parseFloat(valA) || 0;
                valB = parseFloat(valB) || 0;
            }
            // 字符串
            else {
                valA = String(valA || '').toLowerCase();
                valB = String(valB || '').toLowerCase();
            }
            
            if (valA < valB) return dir === 'asc' ? -1 : 1;
            if (valA > valB) return dir === 'asc' ? 1 : -1;
            return 0;
        });
    }
};

// 骨架屏组件
const Skeleton = {
    // 生成表格骨架屏
    table: (rows = 5, cols = 4) => {
        let html = '';
        for (let i = 0; i < rows; i++) {
            html += '<tr class="animate-pulse">';
            for (let j = 0; j < cols; j++) {
                const width = j === 0 ? 'w-24' : (j === cols - 1 ? 'w-16' : 'w-32');
                html += `<td class="px-6 py-4"><div class="h-4 bg-gray-200 rounded ${width}"></div></td>`;
            }
            html += '</tr>';
        }
        return html;
    },
    
    // 生成卡片骨架屏
    card: (count = 3) => {
        let html = '';
        for (let i = 0; i < count; i++) {
            html += `
                <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6 animate-pulse">
                    <div class="flex items-center space-x-3 mb-4">
                        <div class="h-6 bg-gray-200 rounded w-32"></div>
                        <div class="h-5 bg-gray-200 rounded-full w-16"></div>
                    </div>
                    <div class="h-4 bg-gray-200 rounded w-48 mb-3"></div>
                    <div class="flex space-x-6">
                        <div class="h-4 bg-gray-200 rounded w-20"></div>
                        <div class="h-4 bg-gray-200 rounded w-20"></div>
                        <div class="h-4 bg-gray-200 rounded w-20"></div>
                    </div>
                </div>
            `;
        }
        return html;
    },
    
    // 生成列表项骨架屏
    list: (count = 5) => {
        let html = '';
        for (let i = 0; i < count; i++) {
            html += `
                <div class="flex items-center justify-between p-4 border-b border-gray-100 animate-pulse">
                    <div class="flex items-center space-x-4">
                        <div class="w-10 h-10 bg-gray-200 rounded-full"></div>
                        <div>
                            <div class="h-4 bg-gray-200 rounded w-32 mb-2"></div>
                            <div class="h-3 bg-gray-200 rounded w-48"></div>
                        </div>
                    </div>
                    <div class="h-8 bg-gray-200 rounded w-20"></div>
                </div>
            `;
        }
        return html;
    },
    
    // 生成统计卡片骨架屏
    stats: (count = 4) => {
        let html = '<div class="grid grid-cols-1 md:grid-cols-4 gap-6">';
        for (let i = 0; i < count; i++) {
            html += `
                <div class="bg-white/80 backdrop-blur rounded-2xl p-6 border border-white shadow-sm animate-pulse">
                    <div class="h-4 bg-gray-200 rounded w-20 mb-3"></div>
                    <div class="h-10 bg-gray-200 rounded w-24"></div>
                </div>
            `;
        }
        html += '</div>';
        return html;
    },
    
    // 显示骨架屏
    show: (container, type = 'table', options = {}) => {
        const el = typeof container === 'string' ? document.getElementById(container) : container;
        if (!el) return;
        
        switch (type) {
            case 'table':
                el.innerHTML = Skeleton.table(options.rows || 5, options.cols || 4);
                break;
            case 'card':
                el.innerHTML = Skeleton.card(options.count || 3);
                break;
            case 'list':
                el.innerHTML = Skeleton.list(options.count || 5);
                break;
            case 'stats':
                el.innerHTML = Skeleton.stats(options.count || 4);
                break;
        }
    }
};

// 工具函数
const Utils = {
    formatDate: (str) => new Date(str).toLocaleString(),
    // HTML 转义
    escapeHtml: (unsafe) => {
        if (!unsafe) return '';
        return String(unsafe)
             .replace(/&/g, '&amp;')
             .replace(/</g, '&lt;')
             .replace(/>/g, '&gt;')
             .replace(/"/g, '&quot;')
             .replace(/'/g, '&#039;');
    },
    // HTML 净化 (生产环境建议使用 DOMPurify)
    sanitizeHtml: (html) => {
        if (!html) return '';
        // 创建一个临时 div 来解析 HTML
        const div = document.createElement('div');
        div.innerHTML = html;
        
        // 移除危险标签
        const dangerousTags = ['script', 'iframe', 'object', 'embed', 'form', 'input', 'link', 'style'];
        dangerousTags.forEach(tag => {
            const elements = div.querySelectorAll(tag);
            elements.forEach(el => el.remove());
        });
        
        // 移除危险属性
        const allElements = div.querySelectorAll('*');
        allElements.forEach(el => {
            // 移除所有 on* 事件处理器
            const attrs = [...el.attributes];
            attrs.forEach(attr => {
                if (attr.name.startsWith('on') || 
                    attr.value.toLowerCase().includes('javascript:') ||
                    attr.value.toLowerCase().includes('vbscript:')) {
                    el.removeAttribute(attr.name);
                }
            });
            // 移除 src/href 中的 javascript:
            ['src', 'href', 'action'].forEach(attrName => {
                const val = el.getAttribute(attrName);
                if (val && (val.toLowerCase().trim().startsWith('javascript:') || 
                            val.toLowerCase().trim().startsWith('vbscript:'))) {
                    el.removeAttribute(attrName);
                }
            });
        });
        
        return div.innerHTML;
    }
};
