/**
 * 晴辰云邮 - 全局加载动画 (优化版 v2)
 * 企业级蓝色主题 - 极速流畅动效
 */

(function() {
    'use strict';

    // 配置
    const CONFIG = {
        minDisplayTime: 600,  // 最小显示时间，确保动画能被看清
        transitionDelay: 800, // 页面跳转延迟，等待纸飞机飞出
        fadeOutDuration: 300, // 淡出时间
        brandName: '晴辰云邮',
        brandSubtitle: 'QINGCHEN MAIL'
    };

    // 创建加载器 DOM
    function createLoader() {
        const loader = document.createElement('div');
        loader.id = 'qc-loader';
        loader.innerHTML = `
            <div class="qc-loader-backdrop"></div>
            <div class="qc-loader-content">
                <!-- 邮件动画容器 -->
                <div class="qc-mail-anim">
                    <div class="qc-envelope-container">
                        <div class="qc-envelope-back"></div>
                        <div class="qc-paper-plane">
                            <svg viewBox="0 0 24 24" fill="currentColor">
                                <path d="M2 12l20-9-9 20-2-9-9-2z" />
                            </svg>
                        </div>
                        <div class="qc-envelope-front"></div>
                        <div class="qc-envelope-flap"></div>
                    </div>
                    <div class="qc-shadow"></div>
                </div>
                
                <!-- 品牌信息 -->
                <div class="qc-brand">
                    <div class="qc-brand-name">${CONFIG.brandName}</div>
                    <div class="qc-brand-subtitle">${CONFIG.brandSubtitle}</div>
                    <div class="qc-loading-bar">
                        <div class="qc-loading-progress"></div>
                    </div>
                </div>
            </div>
        `;

        // 注入样式
        const style = document.createElement('style');
        style.id = 'qc-loader-styles';
        style.textContent = getLoaderStyles();
        document.head.appendChild(style);

        return loader;
    }

    // 加载器样式 - 优化动效曲线
    function getLoaderStyles() {
        return `
            #qc-loader {
                position: fixed;
                inset: 0;
                z-index: 99999;
                display: flex;
                align-items: center;
                justify-content: center;
                opacity: 1;
                transition: opacity ${CONFIG.fadeOutDuration}ms ease-out;
                font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            }
            
            #qc-loader.qc-fade-out {
                opacity: 0;
                pointer-events: none;
            }
            
            /* 背景 - 稳重深蓝渐变 (保持一致) */
            .qc-loader-backdrop {
                position: absolute;
                inset: 0;
                background: linear-gradient(145deg, #1E3A5F 0%, #0F172A 100%);
            }
            
            .qc-loader-content {
                position: relative;
                display: flex;
                flex-direction: column;
                align-items: center;
                transform: scale(1);
                animation: qc-content-enter 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
            }

            @keyframes qc-content-enter {
                from { transform: scale(0.9); opacity: 0; }
                to { transform: scale(1); opacity: 1; }
            }
            
            /* ========== 信封动画核心 ========== */
            .qc-mail-anim {
                position: relative;
                width: 100px;
                height: 80px;
                margin-bottom: 40px;
            }

            .qc-envelope-container {
                position: relative;
                width: 100%;
                height: 60px;
                margin-top: 20px;
                animation: qc-float 3s ease-in-out infinite;
            }

            @keyframes qc-float {
                0%, 100% { transform: translateY(0); }
                50% { transform: translateY(-6px); }
            }

            /* 信封各部件 */
            .qc-envelope-back {
                position: absolute;
                bottom: 0;
                width: 100%;
                height: 100%;
                background: #e2e8f0;
                border-radius: 6px;
            }

            /* 前挡板 (Body) */
            .qc-envelope-front {
                position: absolute;
                bottom: 0;
                left: 0;
                width: 100%;
                height: 0;
                border-bottom: 60px solid #f1f5f9;
                border-left: 50px solid transparent;
                border-right: 50px solid transparent;
                border-radius: 0 0 6px 6px;
                z-index: 10;
                filter: drop-shadow(0 -2px 2px rgba(0,0,0,0.05));
            }

            /* 盖子 (Flap) */
            .qc-envelope-flap {
                position: absolute;
                top: 0;
                left: 0;
                width: 100%;
                height: 0;
                border-top: 35px solid #f8fafc;
                border-left: 50px solid transparent;
                border-right: 50px solid transparent;
                border-radius: 6px 6px 0 0;
                transform-origin: top;
                z-index: 11; /* 初始状态最高 */
                animation: qc-flap-open 1.6s ease-in-out infinite;
            }

            /* 纸飞机 */
            .qc-paper-plane {
                position: absolute;
                bottom: 10px;
                left: 50%;
                width: 40px;
                height: 40px;
                color: #3B82F6;
                z-index: 5; /* 在 front 后面，back 前面 */
                transform: translateX(-50%) scale(0);
                animation: qc-plane-launch 1.6s ease-in-out infinite;
            }

            /* 底部阴影 */
            .qc-shadow {
                position: absolute;
                bottom: -20px;
                left: 50%;
                transform: translateX(-50%);
                width: 80px;
                height: 10px;
                background: rgba(0,0,0,0.2);
                border-radius: 50%;
                filter: blur(4px);
                animation: qc-shadow-pulse 3s ease-in-out infinite;
            }

            /* 关键帧动画 */
            
            @keyframes qc-flap-open {
                0% { transform: rotateX(0deg); z-index: 11; }
                20% { transform: rotateX(180deg); z-index: 1; } /* 20%时完全打开并放到最底层 */
                60% { transform: rotateX(180deg); z-index: 1; }
                80% { transform: rotateX(0deg); z-index: 11; }
                100% { transform: rotateX(0deg); z-index: 11; }
            }

            @keyframes qc-plane-launch {
                0% { transform: translateX(-50%) translateY(20px) scale(0); opacity: 0; }
                10% { transform: translateX(-50%) translateY(10px) scale(0.8); opacity: 1; } /* 进场 */
                25% { transform: translateX(-50%) translateY(-20px) scale(1) rotate(0deg); } /* 准备起飞 */
                50% { transform: translateX(60px) translateY(-80px) scale(0.6) rotate(45deg); opacity: 0; } /* 飞出 */
                100% { transform: translateX(60px) translateY(-80px) scale(0.6) rotate(45deg); opacity: 0; } /* 保持消失 */
            }

            @keyframes qc-shadow-pulse {
                0%, 100% { transform: translateX(-50%) scale(1); opacity: 0.5; }
                50% { transform: translateX(-50%) scale(0.8); opacity: 0.3; }
            }

            /* ========== 品牌信息 ========== */
            .qc-brand {
                text-align: center;
                color: white;
            }
            
            .qc-brand-name {
                font-size: 20px;
                font-weight: 600;
                letter-spacing: 4px;
                margin-bottom: 8px;
                text-shadow: 0 2px 10px rgba(0,0,0,0.2);
            }
            
            .qc-brand-subtitle {
                font-size: 10px;
                font-weight: 500;
                opacity: 0.6;
                letter-spacing: 3px;
                margin-bottom: 20px;
            }

            .qc-loading-bar {
                width: 120px;
                height: 2px;
                background: rgba(255,255,255,0.1);
                border-radius: 2px;
                overflow: hidden;
            }

            .qc-loading-progress {
                width: 100%;
                height: 100%;
                background: #3B82F6;
                transform: translateX(-100%);
                animation: qc-progress 1.6s ease-in-out infinite;
            }

            @keyframes qc-progress {
                0% { transform: translateX(-100%); }
                50% { transform: translateX(0); }
                100% { transform: translateX(100%); }
            }
            
            /* ========== 页面状态控制 ========== */
            body.qc-loading {
                overflow: hidden !important;
            }
        `;
    }

    // 显示加载器
    function show() {
        if (!document.body) {
            // 如果 body 还没准备好，等待 DOMContentLoaded
            window.addEventListener('DOMContentLoaded', show);
            return;
        }
        
        let loader = document.getElementById('qc-loader');
        if (!loader) {
            loader = createLoader();
            document.body.appendChild(loader);
        }
        
        document.body.classList.add('qc-loading');
        loader.classList.remove('qc-fade-out');
        
        // 强制重绘，确保动画从头开始
        // 技巧：移除再添加动画类，或者克隆节点替换（这里简单处理，依赖 CSS 循环）
        const animContent = loader.querySelector('.qc-mail-anim');
        if(animContent) {
            animContent.style.animation = 'none';
            loader.offsetHeight; /* trigger reflow */
            animContent.style.animation = null; 
        }

        window._qcLoaderStartTime = Date.now();
    }

    // 隐藏加载器
    function hide() {
        const loader = document.getElementById('qc-loader');
        if (!loader) return;

        const elapsed = Date.now() - (window._qcLoaderStartTime || 0);
        const remaining = Math.max(0, CONFIG.minDisplayTime - elapsed);

        setTimeout(() => {
            loader.classList.add('qc-fade-out');
            document.body.classList.remove('qc-loading');
            
            setTimeout(() => {
                // 只有当页面不是正在跳转中（没有新的 loader 出现）时才移除
                if (loader.classList.contains('qc-fade-out')) {
                    loader.remove();
                    const style = document.getElementById('qc-loader-styles');
                    if (style) style.remove();
                }
            }, CONFIG.fadeOutDuration);
        }, remaining);
    }

    // 页面切换时显示加载器
    function setupPageTransition() {
        document.addEventListener('click', (e) => {
            const link = e.target.closest('a');
            
            // 检查是否是有效的站内链接
            if (link && 
                link.href && 
                link.href.startsWith(window.location.origin) && 
                link.getAttribute('href') !== '#' &&
                !link.getAttribute('href').startsWith('javascript:') &&
                !link.target && 
                !link.hasAttribute('download') &&
                !link.hasAttribute('data-no-loader') && // 允许特定链接跳过 loader
                !e.ctrlKey && 
                !e.metaKey) {
                
                // 如果是当前页面锚点跳转，忽略
                const currentUrl = new URL(window.location.href);
                const targetUrl = new URL(link.href);
                if (currentUrl.pathname === targetUrl.pathname && targetUrl.hash) return;

                e.preventDefault();
                show();
                
                // 延迟跳转，确保动画展示
                setTimeout(() => {
                    window.location.href = link.href;
                }, CONFIG.transitionDelay);
            }
        });
    }

    // 初始化
    function init() {
        // 1. 立即显示（如果是页面刚加载）
        // 判断页面是否已经 loaded，如果是，说明是动态注入的 loader，不需要立即显示
        // 但通常 loader.js 是在 head 中引入的，此时 body 可能还没 ready
        if (document.readyState === 'loading') {
            show();
            document.addEventListener('DOMContentLoaded', hide);
            window.addEventListener('load', hide); // 双重保险
        } else {
            // 页面已经加载完了（可能是异步加载的 js）
            // 不需要显示 loader，只需 setup
        }

        setupPageTransition();
    }

    // 暴露全局 API
    window.QCLoader = {
        show: show,
        hide: hide,
        init: init
    };

    init();

})();
