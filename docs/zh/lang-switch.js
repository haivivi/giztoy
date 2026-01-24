// Language switcher for mdBook
(function() {
    // Detect current language from URL
    const path = window.location.pathname;
    const isZh = path.includes('/docs/zh/');
    const currentLang = isZh ? 'zh' : 'en';
    
    // Create language switch button
    const langSwitch = document.createElement('a');
    langSwitch.className = 'lang-switch';
    langSwitch.title = currentLang === 'en' ? '切换到中文' : 'Switch to English';
    langSwitch.innerHTML = currentLang === 'en' ? '中文' : 'EN';
    
    // Calculate target URL
    langSwitch.href = '#';
    langSwitch.onclick = function(e) {
        e.preventDefault();
        let targetPath;
        if (currentLang === 'en') {
            // Switch to Chinese: /docs/xxx -> /docs/zh/xxx
            targetPath = path.replace('/docs/', '/docs/zh/');
        } else {
            // Switch to English: /docs/zh/xxx -> /docs/xxx
            targetPath = path.replace('/docs/zh/', '/docs/');
        }
        window.location.href = targetPath;
    };
    
    // Insert into the right-buttons area
    function insertButton() {
        const rightButtons = document.querySelector('.right-buttons');
        if (rightButtons) {
            rightButtons.insertBefore(langSwitch, rightButtons.firstChild);
        }
    }
    
    // Run when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', insertButton);
    } else {
        insertButton();
    }
})();
