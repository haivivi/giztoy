/**
 * Giztoy Homepage - Internationalization (i18n)
 * Supports English and Chinese
 */

(function() {
    'use strict';

    const translations = {
        en: {
            'nav.docs': 'Documentation',
            'hero.tagline': 'An AI toy framework prepared for humanity',
            'hero.subtitle': 'by an intelligence beyond the stars',
            'hero.quote': 'From MCU firmware to cloud agents, from real-time audio to video streams,<br>one framework to connect all LLMs across every dimension.',
            'hero.btn.docs': 'Enter Documentation',
            'hero.btn.github': 'View on GitHub',
            'hero.btn.star': 'Star on GitHub',
            'hero.hint': 'Click anywhere to fly through the stars',
            
            'features.title': 'Core Capabilities',
            'features.subtitle': 'Everything you need, from edge to cloud',
            'features.dimension.title': 'Full Dimension Coverage',
            'features.dimension.desc': 'From ESP32 chips to cloud agents. Android, iOS, HarmonyOS — your code runs everywhere.',
            'features.build.title': 'Unified Build System',
            'features.build.desc': 'Bazel compiles everything: mobile apps, MCU firmware, Linux services. One system to rule them all.',
            'features.llm.title': 'Universal LLM Support',
            'features.llm.desc': 'OpenAI, Gemini, Claude, MiniMax, DashScope, Doubao — switch AI brains like swapping toys.',
            'features.secure.title': 'Secure Transport',
            'features.secure.desc': 'MQTT for IoT, Noise Protocol + KCP for real-time encrypted audio/video transmission.',
            'features.audio.title': 'Real-time Audio/Video',
            'features.audio.desc': 'Opus, MP3, PCM encoding/decoding with millisecond-level streaming latency.',
            'features.lang.title': 'Multi-Language',
            'features.lang.desc': 'Go for simplicity, Rust for performance, Zig for the edge. Choose your weapon.',
            
            'arch.title': 'Architecture',
            'arch.subtitle': 'Layered design for maximum flexibility',
            'arch.layer1': 'AI Application Layer',
            'arch.layer2': 'API Client Layer',
            'arch.layer3': 'Communication Layer',
            'arch.layer4': 'Audio Processing Layer',
            'arch.layer5': 'Foundation Layer',
            
            'platforms.title': 'Supported Platforms',
            'platforms.subtitle': 'Built with Bazel — one build system for all',
            'platforms.harmonyos': 'HarmonyOS',
            'platforms.wip': 'WIP',
            'platforms.soon': 'Soon',
            
            'cta.title': 'Ready to Play?',
            'cta.subtitle': 'Start building AI-powered applications today.',
            'cta.btn': 'Get Started',
            
            'footer.quote': '"I\'m just a toymaker."',
            'footer.license': 'Apache 2.0',
        },
        zh: {
            'nav.docs': '文档',
            'hero.tagline': '为人类准备的 AI 玩具框架',
            'hero.subtitle': '来自星辰之外的智慧',
            'hero.quote': '从 MCU 固件到云端 Agent，从实时音频到视频流，<br>一个框架连接所有大模型，跨越一切维度。',
            'hero.btn.docs': '查看文档',
            'hero.btn.github': '在 GitHub 上查看',
            'hero.btn.star': '给个 Star',
            'hero.hint': '点击任意位置穿越星海',
            
            'features.title': '核心能力',
            'features.subtitle': '从边缘到云端，应有尽有',
            'features.dimension.title': '全维度覆盖',
            'features.dimension.desc': '从 ESP32 芯片到云端 Agent，Android、iOS、鸿蒙——你的代码无处不在。',
            'features.build.title': '统一构建系统',
            'features.build.desc': 'Bazel 编译一切：手机 App、MCU 固件、Linux 服务。一套系统统治所有。',
            'features.llm.title': '全大模型支持',
            'features.llm.desc': 'OpenAI、Gemini、Claude、MiniMax、通义千问、豆包——像换玩具一样切换 AI。',
            'features.secure.title': '安全传输协议',
            'features.secure.desc': 'MQTT 用于物联网，Noise Protocol + KCP 用于实时加密音视频传输。',
            'features.audio.title': '实时音视频',
            'features.audio.desc': 'Opus、MP3、PCM 编解码，毫秒级流式传输延迟。',
            'features.lang.title': '多语言实现',
            'features.lang.desc': 'Go 求简洁，Rust 求性能，Zig 攻边缘。选择你的武器。',
            
            'arch.title': '架构设计',
            'arch.subtitle': '分层设计，极致灵活',
            'arch.layer1': 'AI 应用层',
            'arch.layer2': 'API 客户端层',
            'arch.layer3': '通信层',
            'arch.layer4': '音频处理层',
            'arch.layer5': '基础设施层',
            
            'platforms.title': '支持平台',
            'platforms.subtitle': '基于 Bazel 构建——一套系统编译所有',
            'platforms.harmonyos': '鸿蒙',
            'platforms.wip': '开发中',
            'platforms.soon': '即将支持',
            
            'cta.title': '准备好玩了吗？',
            'cta.subtitle': '立即开始构建 AI 驱动的应用。',
            'cta.btn': '开始使用',
            
            'footer.quote': '"我只是个做玩具的。"',
            'footer.license': 'Apache 2.0 许可证',
        }
    };

    let currentLang = localStorage.getItem('giztoy-lang') || 
                      (navigator.language.startsWith('zh') ? 'zh' : 'en');

    function updateTexts() {
        const elements = document.querySelectorAll('[data-i18n]');
        elements.forEach(el => {
            const key = el.getAttribute('data-i18n');
            if (translations[currentLang] && translations[currentLang][key]) {
                el.innerHTML = translations[currentLang][key];
            }
        });
        
        // Update lang switch button
        const langText = document.getElementById('lang-text');
        if (langText) {
            langText.textContent = currentLang === 'en' ? 'EN' : '中';
        }
        
        // Update html lang attribute
        document.documentElement.lang = currentLang === 'zh' ? 'zh-CN' : 'en';
        
        // Update docs links for current language
        const docsLinks = document.querySelectorAll('a[href^="docs"]');
        docsLinks.forEach(link => {
            // Only update internal docs links, not external ones
            if (link.href.includes('/docs')) {
                link.href = currentLang === 'zh' ? 'docs/zh/' : 'docs/';
            }
        });
    }

    function toggleLang() {
        currentLang = currentLang === 'en' ? 'zh' : 'en';
        localStorage.setItem('giztoy-lang', currentLang);
        updateTexts();
    }

    // Initialize
    document.addEventListener('DOMContentLoaded', () => {
        updateTexts();
        
        const langSwitch = document.getElementById('lang-switch');
        if (langSwitch) {
            langSwitch.addEventListener('click', toggleLang);
        }
    });

    // Export for external use
    window.giztoyI18n = {
        toggle: toggleLang,
        setLang: (lang) => {
            if (translations[lang]) {
                currentLang = lang;
                localStorage.setItem('giztoy-lang', currentLang);
                updateTexts();
            }
        },
        getLang: () => currentLang
    };

})();
