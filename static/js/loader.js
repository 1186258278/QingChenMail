/**
 * 晴辰云邮 - 全局加载动画
 * 邮件飞行特效加载器
 */

(function() {
    'use strict';

    // 配置
    const CONFIG = {
        minDisplayTime: 600,    // 最小显示时间 (ms)
        fadeOutDuration: 400,   // 淡出动画时长 (ms)
        brandName: '晴辰云邮',
        brandSubtitle: 'QingChen Mail'
    };

    // 创建加载器 DOM
    function createLoader() {
        const loader = document.createElement('div');
        loader.id = 'qc-loader';
        loader.innerHTML = `
            <div class="qc-loader-backdrop"></div>
            <div class="qc-loader-content">
                <!-- 邮件飞行动画 -->
                <div class="qc-mail-container">
                    <!-- 邮件轨迹 -->
                    <div class="qc-mail-trail"></div>
                    <div class="qc-mail-trail qc-mail-trail-2"></div>
                    <div class="qc-mail-trail qc-mail-trail-3"></div>
                    
                    <!-- 邮件图标 -->
                    <div class="qc-mail-icon">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                            <rect x="2" y="4" width="20" height="16" rx="2"/>
                            <path d="M22 6L12 13L2 6"/>
                        </svg>
                    </div>
                    
                    <!-- 发送粒子效果 -->
                    <div class="qc-particles">
                        <span></span><span></span><span></span>
                        <span></span><span></span><span></span>
                    </div>
                </div>
                
                <!-- 品牌信息 -->
                <div class="qc-brand">
                    <div class="qc-brand-name">${CONFIG.brandName}</div>
                    <div class="qc-brand-subtitle">${CONFIG.brandSubtitle}</div>
                </div>
                
                <!-- 加载进度条 -->
                <div class="qc-progress">
                    <div class="qc-progress-bar"></div>
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

    // 加载器样式
    function getLoaderStyles() {
        return `
            #qc-loader {
                position: fixed;
                inset: 0;
                z-index: 99999;
                display: flex;
                align-items: center;
                justify-content: center;
                transition: opacity ${CONFIG.fadeOutDuration}ms ease-out;
            }
            
            #qc-loader.qc-fade-out {
                opacity: 0;
                pointer-events: none;
            }
            
            .qc-loader-backdrop {
                position: absolute;
                inset: 0;
                background: linear-gradient(135deg, #667eea 0%, #764ba2 50%, #6B8DD6 100%);
                background-size: 400% 400%;
                animation: qc-gradient 8s ease infinite;
            }
            
            @keyframes qc-gradient {
                0%, 100% { background-position: 0% 50%; }
                50% { background-position: 100% 50%; }
            }
            
            .qc-loader-content {
                position: relative;
                display: flex;
                flex-direction: column;
                align-items: center;
                color: white;
            }
            
            /* 邮件容器 */
            .qc-mail-container {
                position: relative;
                width: 120px;
                height: 120px;
                margin-bottom: 32px;
            }
            
            /* 邮件图标 */
            .qc-mail-icon {
                position: absolute;
                top: 50%;
                left: 50%;
                transform: translate(-50%, -50%);
                width: 56px;
                height: 56px;
                color: white;
                animation: qc-mail-fly 2s ease-in-out infinite;
                filter: drop-shadow(0 4px 12px rgba(0,0,0,0.2));
            }
            
            .qc-mail-icon svg {
                width: 100%;
                height: 100%;
            }
            
            @keyframes qc-mail-fly {
                0%, 100% {
                    transform: translate(-50%, -50%) translateY(0) rotate(-5deg);
                }
                25% {
                    transform: translate(-50%, -50%) translateX(8px) translateY(-12px) rotate(0deg);
                }
                50% {
                    transform: translate(-50%, -50%) translateY(-6px) rotate(5deg);
                }
                75% {
                    transform: translate(-50%, -50%) translateX(-8px) translateY(-12px) rotate(0deg);
                }
            }
            
            /* 邮件轨迹 */
            .qc-mail-trail {
                position: absolute;
                top: 50%;
                left: 50%;
                width: 80px;
                height: 3px;
                background: linear-gradient(90deg, transparent, rgba(255,255,255,0.6), transparent);
                border-radius: 2px;
                transform: translate(-80px, -50%) rotate(-15deg);
                animation: qc-trail 2s ease-in-out infinite;
                opacity: 0;
            }
            
            .qc-mail-trail-2 {
                width: 60px;
                animation-delay: 0.1s;
                transform: translate(-70px, calc(-50% + 8px)) rotate(-10deg);
            }
            
            .qc-mail-trail-3 {
                width: 40px;
                animation-delay: 0.2s;
                transform: translate(-60px, calc(-50% + 16px)) rotate(-5deg);
            }
            
            @keyframes qc-trail {
                0%, 100% { opacity: 0; transform: translate(-80px, -50%) rotate(-15deg) scaleX(0); }
                25% { opacity: 0.8; transform: translate(-60px, -50%) rotate(-10deg) scaleX(1); }
                50% { opacity: 0.4; transform: translate(-40px, -50%) rotate(-5deg) scaleX(0.6); }
                75% { opacity: 0; transform: translate(-20px, -50%) rotate(0deg) scaleX(0); }
            }
            
            /* 粒子效果 */
            .qc-particles {
                position: absolute;
                top: 50%;
                left: 50%;
                transform: translate(-50%, -50%);
                pointer-events: none;
            }
            
            .qc-particles span {
                position: absolute;
                width: 6px;
                height: 6px;
                background: rgba(255,255,255,0.8);
                border-radius: 50%;
                animation: qc-particle 2s ease-out infinite;
            }
            
            .qc-particles span:nth-child(1) { animation-delay: 0s; }
            .qc-particles span:nth-child(2) { animation-delay: 0.15s; }
            .qc-particles span:nth-child(3) { animation-delay: 0.3s; }
            .qc-particles span:nth-child(4) { animation-delay: 0.45s; }
            .qc-particles span:nth-child(5) { animation-delay: 0.6s; }
            .qc-particles span:nth-child(6) { animation-delay: 0.75s; }
            
            @keyframes qc-particle {
                0% {
                    opacity: 0;
                    transform: translate(20px, 0) scale(0);
                }
                20% {
                    opacity: 1;
                    transform: translate(25px, -5px) scale(1);
                }
                100% {
                    opacity: 0;
                    transform: translate(60px, -20px) scale(0);
                }
            }
            
            /* 品牌信息 */
            .qc-brand {
                text-align: center;
                margin-bottom: 24px;
            }
            
            .qc-brand-name {
                font-size: 24px;
                font-weight: 700;
                letter-spacing: 2px;
                margin-bottom: 4px;
                text-shadow: 0 2px 8px rgba(0,0,0,0.2);
            }
            
            .qc-brand-subtitle {
                font-size: 12px;
                font-weight: 500;
                opacity: 0.8;
                letter-spacing: 4px;
                text-transform: uppercase;
            }
            
            /* 进度条 */
            .qc-progress {
                width: 160px;
                height: 3px;
                background: rgba(255,255,255,0.2);
                border-radius: 2px;
                overflow: hidden;
            }
            
            .qc-progress-bar {
                height: 100%;
                background: white;
                border-radius: 2px;
                animation: qc-progress 1.5s ease-in-out infinite;
            }
            
            @keyframes qc-progress {
                0% { width: 0%; margin-left: 0%; }
                50% { width: 60%; margin-left: 20%; }
                100% { width: 0%; margin-left: 100%; }
            }
            
            /* 页面内容隐藏 */
            body.qc-loading > *:not(#qc-loader):not(script):not(style):not(link) {
                visibility: hidden;
            }
            
            body.qc-loaded {
                opacity: 1 !important;
            }
        `;
    }

    // 显示加载器
    function show() {
        // 确保 body 存在
        if (!document.body) {
            // 如果 body 还不存在，等待 DOMContentLoaded
            document.addEventListener('DOMContentLoaded', show);
            return;
        }
        
        let loader = document.getElementById('qc-loader');
        if (!loader) {
            loader = createLoader();
            document.body.appendChild(loader);
        }
        document.body.classList.add('qc-loading');
        loader.classList.remove('qc-fade-out');
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
            document.body.classList.add('qc-loaded');
            
            setTimeout(() => {
                loader.remove();
                const style = document.getElementById('qc-loader-styles');
                if (style) style.remove();
            }, CONFIG.fadeOutDuration);
        }, remaining);
    }

    // 页面切换时显示加载器
    function setupPageTransition() {
        document.addEventListener('click', (e) => {
            const link = e.target.closest('a');
            if (link && 
                link.href && 
                link.href.startsWith(window.location.origin) && 
                !link.target && 
                !link.hasAttribute('download') &&
                !e.ctrlKey && 
                !e.metaKey) {
                e.preventDefault();
                show();
                setTimeout(() => {
                    window.location.href = link.href;
                }, 200);
            }
        });
    }

    // 自动初始化
    function init() {
        // 页面开始加载时显示
        show();

        // DOM 加载完成后隐藏
        if (document.readyState === 'complete') {
            hide();
        } else {
            window.addEventListener('load', hide);
        }

        // 设置页面切换过渡
        setupPageTransition();
    }

    // 暴露全局 API
    window.QCLoader = {
        show: show,
        hide: hide,
        init: init
    };

    // 立即初始化
    init();

})();
