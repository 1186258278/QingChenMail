/**
 * 晴辰云邮 - 全局加载动画
 * 企业级蓝色主题 - 信封开启纸飞机飞出动效
 */

(function() {
    'use strict';

    // 配置
    const CONFIG = {
        minDisplayTime: 500,
        fadeOutDuration: 350,
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
                <div class="qc-envelope-scene">
                    <!-- 信封主体 -->
                    <div class="qc-envelope">
                        <!-- 信封盖子 -->
                        <div class="qc-envelope-flap"></div>
                        <!-- 信封主体 -->
                        <div class="qc-envelope-body">
                            <div class="qc-envelope-front"></div>
                        </div>
                        <!-- 纸飞机 -->
                        <div class="qc-paper-plane">
                            <svg viewBox="0 0 24 24" fill="none">
                                <path d="M22 2L11 13" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                                <path d="M22 2L15 22L11 13L2 9L22 2Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                            </svg>
                        </div>
                    </div>
                </div>
                
                <!-- 品牌信息 -->
                <div class="qc-brand">
                    <div class="qc-brand-name">${CONFIG.brandName}</div>
                    <div class="qc-brand-subtitle">${CONFIG.brandSubtitle}</div>
                    <div class="qc-brand-line"></div>
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

    // 加载器样式 - 企业级稳重蓝配色
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
            }
            
            #qc-loader.qc-fade-out {
                opacity: 0;
                pointer-events: none;
            }
            
            /* 背景 - 稳重深蓝渐变 */
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
            }
            
            /* ========== 信封场景 ========== */
            .qc-envelope-scene {
                width: 120px;
                height: 100px;
                perspective: 800px;
                margin-bottom: 36px;
            }
            
            .qc-envelope {
                position: relative;
                width: 100%;
                height: 100%;
                transform-style: preserve-3d;
            }
            
            /* 信封主体 */
            .qc-envelope-body {
                position: absolute;
                bottom: 0;
                width: 100%;
                height: 70px;
                background: linear-gradient(135deg, #ffffff 0%, #f1f5f9 100%);
                border-radius: 4px 4px 8px 8px;
                box-shadow: 0 8px 32px rgba(30, 58, 95, 0.35), 0 2px 8px rgba(0,0,0,0.12);
                overflow: hidden;
            }
            
            .qc-envelope-front {
                position: absolute;
                bottom: 0;
                left: 0;
                right: 0;
                height: 40px;
                background: linear-gradient(to bottom, transparent 0%, rgba(30, 58, 95, 0.06) 100%);
                clip-path: polygon(0 100%, 50% 30%, 100% 100%);
            }
            
            /* 信封盖子 */
            .qc-envelope-flap {
                position: absolute;
                top: 0;
                left: 0;
                width: 100%;
                height: 50px;
                background: linear-gradient(180deg, #f8fafc 0%, #e2e8f0 100%);
                clip-path: polygon(0 0, 50% 100%, 100% 0);
                transform-origin: top center;
                transform: rotateX(0deg);
                animation: qc-flap-open 2.5s ease-in-out infinite;
                box-shadow: 0 2px 4px rgba(0,0,0,0.05);
            }
            
            @keyframes qc-flap-open {
                0%, 15% {
                    transform: rotateX(0deg);
                }
                30%, 55% {
                    transform: rotateX(-160deg);
                }
                70%, 100% {
                    transform: rotateX(0deg);
                }
            }
            
            /* 纸飞机 - 亮蓝色 */
            .qc-paper-plane {
                position: absolute;
                top: 50%;
                left: 50%;
                width: 36px;
                height: 36px;
                color: #3B82F6;
                transform: translate(-50%, -50%) scale(0) rotate(-45deg);
                animation: qc-plane-fly 2.5s ease-in-out infinite;
                filter: drop-shadow(0 2px 8px rgba(59, 130, 246, 0.5));
            }
            
            .qc-paper-plane svg {
                width: 100%;
                height: 100%;
            }
            
            @keyframes qc-plane-fly {
                0%, 20% {
                    transform: translate(-50%, -50%) scale(0) rotate(-45deg);
                    opacity: 0;
                }
                30% {
                    transform: translate(-50%, -50%) scale(1) rotate(-45deg);
                    opacity: 1;
                }
                45% {
                    transform: translate(-50%, -80%) scale(1) rotate(-45deg);
                    opacity: 1;
                }
                65% {
                    transform: translate(100px, -150px) scale(0.6) rotate(-45deg);
                    opacity: 0;
                }
                66%, 100% {
                    transform: translate(-50%, -50%) scale(0) rotate(-45deg);
                    opacity: 0;
                }
            }
            
            /* ========== 品牌信息 ========== */
            .qc-brand {
                text-align: center;
                color: white;
            }
            
            .qc-brand-name {
                font-size: 22px;
                font-weight: 600;
                letter-spacing: 3px;
                margin-bottom: 6px;
                text-shadow: 0 2px 12px rgba(0,0,0,0.2);
            }
            
            .qc-brand-subtitle {
                font-size: 11px;
                font-weight: 500;
                opacity: 0.7;
                letter-spacing: 5px;
                margin-bottom: 16px;
            }
            
            /* 品牌线条动画 */
            .qc-brand-line {
                width: 80px;
                height: 2px;
                background: rgba(255,255,255,0.2);
                border-radius: 1px;
                margin: 0 auto;
                position: relative;
                overflow: hidden;
            }
            
            .qc-brand-line::after {
                content: '';
                position: absolute;
                top: 0;
                left: -100%;
                width: 100%;
                height: 100%;
                background: linear-gradient(90deg, transparent, rgba(59, 130, 246, 0.8), transparent);
                animation: qc-line-shine 2s ease-in-out infinite;
            }
            
            @keyframes qc-line-shine {
                0% { left: -100%; }
                50%, 100% { left: 100%; }
            }
            
            /* ========== 页面状态控制 ========== */
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
        if (!document.body) {
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
                }, 150);
            }
        });
    }

    // 自动初始化
    function init() {
        show();

        if (document.readyState === 'complete') {
            hide();
        } else {
            window.addEventListener('load', hide);
        }

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
