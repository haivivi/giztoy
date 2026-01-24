/**
 * Giztoy Homepage - Synthesized Sound Effects
 * Using Web Audio API to generate sci-fi sounds
 */

(function() {
    'use strict';

    let audioCtx = null;
    let initialized = false;

    // Lazy initialize AudioContext (requires user interaction)
    function getAudioContext() {
        if (!audioCtx) {
            audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        }
        if (audioCtx.state === 'suspended') {
            audioCtx.resume();
        }
        return audioCtx;
    }

    // ============================================
    // Warp Drive Sound - For starfield click
    // Airy whoosh with prismatic shimmer
    // ============================================
    function playWarpSound() {
        const ctx = getAudioContext();
        const now = ctx.currentTime;
        const duration = 0.45;
        
        const masterGain = ctx.createGain();
        masterGain.gain.value = 0.1;
        masterGain.connect(ctx.destination);
        
        // 1. Main airy whoosh - highpass filtered noise
        const noiseLength = ctx.sampleRate * duration;
        const noiseBuffer = ctx.createBuffer(1, noiseLength, ctx.sampleRate);
        const noiseData = noiseBuffer.getChannelData(0);
        for (let i = 0; i < noiseLength; i++) {
            noiseData[i] = Math.random() * 2 - 1;
        }
        
        const noise = ctx.createBufferSource();
        noise.buffer = noiseBuffer;
        
        const highpass = ctx.createBiquadFilter();
        highpass.type = 'highpass';
        highpass.frequency.setValueAtTime(1500, now);
        highpass.frequency.exponentialRampToValueAtTime(800, now + duration);
        
        const bandpass = ctx.createBiquadFilter();
        bandpass.type = 'bandpass';
        bandpass.frequency.setValueAtTime(2000, now);
        bandpass.frequency.exponentialRampToValueAtTime(4000, now + 0.08);
        bandpass.frequency.exponentialRampToValueAtTime(1500, now + duration);
        bandpass.Q.value = 0.7;
        
        const noiseGain = ctx.createGain();
        noiseGain.gain.setValueAtTime(0, now);
        noiseGain.gain.linearRampToValueAtTime(0.5, now + 0.03);
        noiseGain.gain.setValueAtTime(0.4, now + 0.1);
        noiseGain.gain.exponentialRampToValueAtTime(0.01, now + duration);
        
        noise.connect(highpass);
        highpass.connect(bandpass);
        bandpass.connect(noiseGain);
        noiseGain.connect(masterGain);
        
        noise.start(now);
        noise.stop(now + duration);
        
        // 2. Prismatic shimmer - harmonic chord with slight vibrato
        // Like light refracting through a prism
        const prismFreqs = [1800, 2250, 2700]; // Major chord harmonics
        
        prismFreqs.forEach((baseFreq, i) => {
            const prism = ctx.createOscillator();
            const prismGain = ctx.createGain();
            
            // Triangle wave for softer tone
            prism.type = 'triangle';
            
            // Slight frequency drift for shimmering effect
            const drift = 1 + (Math.random() - 0.5) * 0.02;
            prism.frequency.setValueAtTime(baseFreq * drift, now);
            prism.frequency.exponentialRampToValueAtTime(baseFreq * 1.2, now + 0.06);
            prism.frequency.exponentialRampToValueAtTime(baseFreq * 0.9, now + duration);
            
            // Staggered entry for sparkle effect
            const delay = i * 0.015;
            prismGain.gain.setValueAtTime(0, now);
            prismGain.gain.linearRampToValueAtTime(0, now + delay);
            prismGain.gain.linearRampToValueAtTime(0.06, now + delay + 0.02);
            prismGain.gain.exponentialRampToValueAtTime(0.01, now + duration * 0.6);
            
            prism.connect(prismGain);
            prismGain.connect(masterGain);
            prism.start(now);
            prism.stop(now + duration);
        });
        
        // 3. Very subtle low body
        const body = ctx.createOscillator();
        const bodyGain = ctx.createGain();
        
        body.type = 'sine';
        body.frequency.setValueAtTime(150, now);
        body.frequency.exponentialRampToValueAtTime(100, now + duration);
        
        bodyGain.gain.setValueAtTime(0, now);
        bodyGain.gain.linearRampToValueAtTime(0.06, now + 0.02);
        bodyGain.gain.exponentialRampToValueAtTime(0.01, now + duration * 0.4);
        
        body.connect(bodyGain);
        bodyGain.connect(masterGain);
        body.start(now);
        body.stop(now + duration);
    }

    // ============================================
    // UI Click Sound - For buttons
    // A soft, futuristic "pip"
    // ============================================
    function playClickSound() {
        const ctx = getAudioContext();
        const now = ctx.currentTime;
        
        const masterGain = ctx.createGain();
        masterGain.gain.value = 0.06; // Soft volume
        masterGain.connect(ctx.destination);
        
        // High pitched blip
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        
        osc.type = 'sine';
        osc.frequency.setValueAtTime(880, now);
        osc.frequency.exponentialRampToValueAtTime(1100, now + 0.02);
        osc.frequency.exponentialRampToValueAtTime(700, now + 0.08);
        
        gain.gain.setValueAtTime(0, now);
        gain.gain.linearRampToValueAtTime(0.5, now + 0.005);
        gain.gain.exponentialRampToValueAtTime(0.01, now + 0.08);
        
        osc.connect(gain);
        gain.connect(masterGain);
        
        osc.start(now);
        osc.stop(now + 0.08);
    }

    // ============================================
    // Hover Sound - Very subtle sci-fi feedback
    // ============================================
    function playHoverSound() {
        const ctx = getAudioContext();
        const now = ctx.currentTime;
        
        const masterGain = ctx.createGain();
        masterGain.gain.value = 0.025; // Very soft
        masterGain.connect(ctx.destination);
        
        // Soft rising tone
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        
        osc.type = 'sine';
        osc.frequency.setValueAtTime(400, now);
        osc.frequency.exponentialRampToValueAtTime(600, now + 0.06);
        
        gain.gain.setValueAtTime(0, now);
        gain.gain.linearRampToValueAtTime(0.5, now + 0.01);
        gain.gain.exponentialRampToValueAtTime(0.01, now + 0.06);
        
        osc.connect(gain);
        gain.connect(masterGain);
        
        osc.start(now);
        osc.stop(now + 0.06);
        
        // Tiny shimmer
        const shimmer = ctx.createOscillator();
        const shimmerGain = ctx.createGain();
        
        shimmer.type = 'sine';
        shimmer.frequency.setValueAtTime(1200, now);
        shimmer.frequency.exponentialRampToValueAtTime(800, now + 0.04);
        
        shimmerGain.gain.setValueAtTime(0, now);
        shimmerGain.gain.linearRampToValueAtTime(0.2, now + 0.01);
        shimmerGain.gain.exponentialRampToValueAtTime(0.01, now + 0.04);
        
        shimmer.connect(shimmerGain);
        shimmerGain.connect(masterGain);
        
        shimmer.start(now);
        shimmer.stop(now + 0.04);
    }

    // ============================================
    // Language Switch Sound
    // A "mode change" beep
    // ============================================
    function playToggleSound() {
        const ctx = getAudioContext();
        const now = ctx.currentTime;
        
        const masterGain = ctx.createGain();
        masterGain.gain.value = 0.05; // Soft volume
        masterGain.connect(ctx.destination);
        
        // Two-tone beep
        const osc1 = ctx.createOscillator();
        const osc2 = ctx.createOscillator();
        const gain = ctx.createGain();
        
        osc1.type = 'sine';
        osc2.type = 'sine';
        
        osc1.frequency.setValueAtTime(523, now); // C5
        osc1.frequency.setValueAtTime(659, now + 0.06); // E5
        
        osc2.frequency.setValueAtTime(784, now); // G5
        osc2.frequency.setValueAtTime(1047, now + 0.06); // C6
        
        gain.gain.setValueAtTime(0, now);
        gain.gain.linearRampToValueAtTime(0.5, now + 0.01);
        gain.gain.setValueAtTime(0.5, now + 0.05);
        gain.gain.linearRampToValueAtTime(0.4, now + 0.07);
        gain.gain.exponentialRampToValueAtTime(0.01, now + 0.15);
        
        osc1.connect(gain);
        osc2.connect(gain);
        gain.connect(masterGain);
        
        osc1.start(now);
        osc2.start(now);
        osc1.stop(now + 0.15);
        osc2.stop(now + 0.15);
    }

    // ============================================
    // Initialize Event Listeners
    // ============================================
    function init() {
        if (initialized) return;
        initialized = true;

        // Starfield clicks are handled by homepage.js
        // We expose the warp sound for it to call
        
        // Button clicks
        document.querySelectorAll('.btn, button').forEach(btn => {
            btn.addEventListener('click', (e) => {
                if (btn.classList.contains('lang-switch')) {
                    playToggleSound();
                } else {
                    playClickSound();
                }
            });
        });

        // Navigation links
        document.querySelectorAll('.nav-link').forEach(link => {
            link.addEventListener('click', playClickSound);
        });
        
        // Card hover sounds
        document.querySelectorAll('.feature-card, .arch-layer, .platform-item').forEach(card => {
            card.addEventListener('mouseenter', playHoverSound);
        });
    }

    // Initialize on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    // Export for use by homepage.js
    window.giztoySounds = {
        warp: playWarpSound,
        click: playClickSound,
        hover: playHoverSound,
        toggle: playToggleSound
    };

})();
