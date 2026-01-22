// 基础配置
const API_BASE = '/api/v1';

// 认证管理
const Auth = {
    getToken: () => localStorage.getItem('token'),
    setToken: (token) => {
        localStorage.setItem('token', token);
        document.cookie = `token=${token}; path=/; max-age=86400`;
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
    div.innerHTML = `<span>${msg}</span>`;
    document.body.appendChild(div);
    
    requestAnimationFrame(() => {
        div.classList.remove('translate-y-10', 'opacity-0');
    });

    setTimeout(() => {
        div.classList.add('translate-y-10', 'opacity-0');
        setTimeout(() => div.remove(), 300);
    }, 3000);
}

// 工具函数
const Utils = {
    formatDate: (str) => new Date(str).toLocaleString(),
    escapeHtml: (unsafe) => {
        if (!unsafe) return '';
        return unsafe
             .replace(/&/g, '&amp;')
             .replace(/</g, '&lt;')
             .replace(/>/g, '&gt;')
             .replace(/"/g, '&quot;')
             .replace(/'/g, '&#039;');
    }
};
