/**
 * Giztoy Homepage - Interactive Starfield Navigation
 * 
 * Tap/click to engage warp drive!
 * Experience relativistic doppler shift at high speeds.
 */

(function() {
    'use strict';

    // ============================================
    // Configuration
    // ============================================
    const CONFIG = {
        stars: {
            count: 600,
            depth: 2000,
            // Star colors - mostly white with some colored giants
            colors: [
                // White/common stars (70%)
                { rgb: [255, 255, 255], weight: 20 },   // Pure white
                { rgb: [255, 252, 248], weight: 15 },   // Warm white
                { rgb: [248, 252, 255], weight: 15 },   // Cool white
                // Yellow/orange stars (15%)
                { rgb: [255, 240, 200], weight: 8 },    // Yellow (like Sun)
                { rgb: [255, 220, 180], weight: 4 },    // Orange
                { rgb: [255, 200, 150], weight: 3 },    // Deep orange
                // Blue stars (10%)
                { rgb: [200, 220, 255], weight: 5 },    // Light blue
                { rgb: [170, 200, 255], weight: 3 },    // Blue
                { rgb: [150, 180, 255], weight: 2 },    // Deep blue
                // Red giants (5%)
                { rgb: [255, 180, 150], weight: 3 },    // Light red
                { rgb: [255, 150, 120], weight: 2 },    // Red giant
            ]
        },
        flight: {
            // Click flight
            clickSpeed: 30,        // Speed burst on click
            clickDecay: 0.96,      // How fast click speed decays
            minSpeed: 0.1,
            // Tap detection
            maxTapDuration: 300,   // Max ms to be considered a tap
            // Idle drift
            driftSpeed: 0.08,      // Very slow drift speed (reduced!)
            driftChangeInterval: 8000, // Change direction every 8 seconds
        },
        doppler: {
            threshold: 10,         // Speed threshold to start doppler effect
            maxShift: 0.8,         // Maximum color shift intensity
        }
    };

    // ============================================
    // Canvas Setup
    // ============================================
    const canvas = document.getElementById('starfield');
    const ctx = canvas.getContext('2d');

    let width, height, centerX, centerY;
    let stars = [];
    let initialized = false;
    
    // Flight state
    let speed = 0;
    let targetX = 0;
    let targetY = 0;
    
    // Drift state
    let driftX = 0;
    let driftY = 0;
    let lastDriftChange = 0;
    
    // Tap detection
    let pointerDownTime = 0;
    let pointerDownX = 0;
    let pointerDownY = 0;

    // Weighted random color selection
    function getRandomStarColor() {
        const totalWeight = CONFIG.stars.colors.reduce((sum, c) => sum + c.weight, 0);
        let random = Math.random() * totalWeight;
        for (const color of CONFIG.stars.colors) {
            random -= color.weight;
            if (random <= 0) {
                return [...color.rgb];
            }
        }
        return [255, 255, 255];
    }

    function resize() {
        width = window.innerWidth;
        height = window.innerHeight;
        centerX = width / 2;
        centerY = height / 2;
        canvas.width = width;
        canvas.height = height;
        
        ctx.fillStyle = 'rgb(10, 10, 15)';
        ctx.fillRect(0, 0, width, height);
    }

    // ============================================
    // Doppler Shift Effect
    // ============================================
    function applyDopplerShift(baseColor, starX, starY, currentSpeed, flightDirX, flightDirY) {
        if (currentSpeed < CONFIG.doppler.threshold) {
            return baseColor;
        }
        
        // Calculate star's direction relative to center
        const starDirX = starX / (Math.abs(starX) + Math.abs(starY) + 0.001);
        const starDirY = starY / (Math.abs(starX) + Math.abs(starY) + 0.001);
        
        // Dot product: positive = star is ahead (blueshift), negative = behind (redshift)
        const dotProduct = starDirX * flightDirX + starDirY * flightDirY;
        
        // Calculate shift intensity based on speed and direction
        const speedFactor = Math.min(1, (currentSpeed - CONFIG.doppler.threshold) / 20);
        const shiftIntensity = dotProduct * speedFactor * CONFIG.doppler.maxShift;
        
        let [r, g, b] = baseColor;
        
        if (shiftIntensity > 0) {
            // Blueshift (approaching) - increase blue, decrease red
            b = Math.min(255, b + shiftIntensity * 100);
            g = Math.min(255, g + shiftIntensity * 30);
            r = Math.max(0, r - shiftIntensity * 60);
        } else if (shiftIntensity < 0) {
            // Redshift (receding) - increase red, decrease blue
            const absShift = Math.abs(shiftIntensity);
            r = Math.min(255, r + absShift * 100);
            g = Math.max(0, g - absShift * 20);
            b = Math.max(0, b - absShift * 80);
        }
        
        return [Math.round(r), Math.round(g), Math.round(b)];
    }

    // ============================================
    // Star Class
    // ============================================
    class Star {
        constructor() {
            this.reset(true);
        }

        reset(randomZ = false) {
            this.x = (Math.random() - 0.5) * width * 2;
            this.y = (Math.random() - 0.5) * height * 2;
            this.z = randomZ ? Math.random() * CONFIG.stars.depth : CONFIG.stars.depth;
            this.pz = this.z;
            
            this.baseColor = getRandomStarColor();
            this.brightness = 0.6 + Math.random() * 0.4;
        }

        update(dx, dy, dz) {
            this.pz = this.z;
            this.x -= dx;
            this.y -= dy;
            this.z -= dz;
            
            if (this.z < 1) {
                this.x = (Math.random() - 0.5) * width * 2;
                this.y = (Math.random() - 0.5) * height * 2;
                this.z = CONFIG.stars.depth;
                this.pz = this.z;
            }
        }

        draw(flightDirX, flightDirY, currentSpeed) {
            const fov = 400;
            const scale = fov / this.z;
            const x2d = centerX + this.x * scale;
            const y2d = centerY + this.y * scale;
            
            if (x2d < -50 || x2d > width + 50 || y2d < -50 || y2d > height + 50) {
                return;
            }
            
            const size = Math.max(0.3, (1 - this.z / CONFIG.stars.depth) * 2.5);
            const alpha = Math.min(1, (1 - this.z / CONFIG.stars.depth) * this.brightness * 1.2);
            
            // Apply doppler shift
            const color = applyDopplerShift(
                this.baseColor, 
                this.x, this.y, 
                currentSpeed, 
                flightDirX, flightDirY
            );
            const [r, g, b] = color;
            
            // Draw trail when flying fast
            if (currentSpeed > 8) {
                const pScale = fov / this.pz;
                const px2d = centerX + this.x * pScale;
                const py2d = centerY + this.y * pScale;
                
                const trailAlpha = alpha * Math.min(0.6, (currentSpeed - 8) / 30);
                
                ctx.beginPath();
                ctx.moveTo(px2d, py2d);
                ctx.lineTo(x2d, y2d);
                
                const gradient = ctx.createLinearGradient(px2d, py2d, x2d, y2d);
                gradient.addColorStop(0, `rgba(${r}, ${g}, ${b}, 0)`);
                gradient.addColorStop(1, `rgba(${r}, ${g}, ${b}, ${trailAlpha})`);
                
                ctx.strokeStyle = gradient;
                ctx.lineWidth = size * 0.6;
                ctx.stroke();
            }
            
            // Draw star
            ctx.beginPath();
            ctx.arc(x2d, y2d, size, 0, Math.PI * 2);
            ctx.fillStyle = `rgba(${r}, ${g}, ${b}, ${alpha})`;
            ctx.fill();
        }
    }

    // ============================================
    // Drift Direction
    // ============================================
    function updateDriftDirection(now) {
        if (now - lastDriftChange > CONFIG.flight.driftChangeInterval) {
            const angle = Math.random() * Math.PI * 2;
            driftX = Math.cos(angle);
            driftY = Math.sin(angle);
            lastDriftChange = now;
        }
    }

    // ============================================
    // Initialization
    // ============================================
    function init() {
        resize();
        
        ctx.fillStyle = 'rgb(10, 10, 15)';
        ctx.fillRect(0, 0, width, height);
        
        stars = [];
        for (let i = 0; i < CONFIG.stars.count; i++) {
            stars.push(new Star());
        }
        
        const angle = Math.random() * Math.PI * 2;
        driftX = Math.cos(angle);
        driftY = Math.sin(angle);
        lastDriftChange = performance.now();
        
        for (const star of stars) {
            star.draw(0, 0, 0);
        }
        
        initialized = true;
        
        setTimeout(() => {
            canvas.classList.remove('starfield-loading');
            canvas.classList.add('starfield-ready');
        }, 50);
    }

    // ============================================
    // Animation Loop
    // ============================================
    function animate(now) {
        if (!initialized) {
            requestAnimationFrame(animate);
            return;
        }
        
        updateDriftDirection(now);
        
        if (speed > CONFIG.flight.minSpeed) {
            speed *= CONFIG.flight.clickDecay;
            if (speed < CONFIG.flight.minSpeed) {
                speed = 0;
            }
        }
        
        let dx, dy, dz;
        let flightDirX = 0, flightDirY = 0;
        
        if (speed > CONFIG.flight.minSpeed) {
            flightDirX = targetX;
            flightDirY = targetY;
            dx = targetX * speed * 1.2;
            dy = targetY * speed * 1.2;
            dz = speed;
        } else {
            flightDirX = driftX;
            flightDirY = driftY;
            dx = driftX * CONFIG.flight.driftSpeed * 0.3;
            dy = driftY * CONFIG.flight.driftSpeed * 0.3;
            dz = CONFIG.flight.driftSpeed;
        }
        
        const fadeAlpha = speed > 15 ? 0.15 : (speed > 5 ? 0.25 : 0.4);
        ctx.fillStyle = `rgba(10, 10, 15, ${fadeAlpha})`;
        ctx.fillRect(0, 0, width, height);
        
        for (const star of stars) {
            star.update(dx, dy, dz);
            star.draw(flightDirX, flightDirY, speed);
        }
        
        requestAnimationFrame(animate);
    }

    // ============================================
    // Flight Control
    // ============================================
    function triggerFlight(clientX, clientY) {
        targetX = (clientX - centerX) / centerX;
        targetY = (clientY - centerY) / centerY;
        
        const magnitude = Math.sqrt(targetX * targetX + targetY * targetY);
        if (magnitude > 1) {
            targetX /= magnitude;
            targetY /= magnitude;
        }
        
        speed = CONFIG.flight.clickSpeed;
        
        // Play warp sound
        if (window.giztoySounds && window.giztoySounds.warp) {
            window.giztoySounds.warp();
        }
    }
    
    function onPointerDown(x, y) {
        pointerDownTime = performance.now();
        pointerDownX = x;
        pointerDownY = y;
    }
    
    function onPointerUp(x, y) {
        const duration = performance.now() - pointerDownTime;
        const distance = Math.sqrt(
            Math.pow(x - pointerDownX, 2) + 
            Math.pow(y - pointerDownY, 2)
        );
        
        if (duration < CONFIG.flight.maxTapDuration && distance < 20) {
            triggerFlight(x, y);
        }
    }

    // ============================================
    // Event Listeners
    // ============================================
    window.addEventListener('resize', () => {
        resize();
        stars = [];
        for (let i = 0; i < CONFIG.stars.count; i++) {
            stars.push(new Star());
        }
    });

    document.addEventListener('mousedown', (e) => {
        if (e.target.closest('a, button, .nav, .btn, input, textarea')) return;
        onPointerDown(e.clientX, e.clientY);
    });
    
    document.addEventListener('mouseup', (e) => {
        if (e.target.closest('a, button, .nav, .btn, input, textarea')) return;
        onPointerUp(e.clientX, e.clientY);
    });

    document.addEventListener('touchstart', (e) => {
        if (e.target.closest('a, button, .nav, .btn, input, textarea')) return;
        if (e.touches.length > 0) {
            const touch = e.touches[0];
            onPointerDown(touch.clientX, touch.clientY);
        }
    }, { passive: true });
    
    document.addEventListener('touchend', (e) => {
        if (e.target.closest('a, button, .nav, .btn, input, textarea')) return;
        if (e.changedTouches.length > 0) {
            const touch = e.changedTouches[0];
            onPointerUp(touch.clientX, touch.clientY);
        }
    }, { passive: true });

    document.addEventListener('keydown', (e) => {
        if (e.code === 'Space' && !e.target.matches('input, textarea')) {
            e.preventDefault();
            targetX = 0;
            targetY = 0;
            speed = CONFIG.flight.clickSpeed;
        }
    });

    // ============================================
    // Start
    // ============================================
    if (document.readyState === 'complete') {
        init();
        requestAnimationFrame(animate);
    } else {
        window.addEventListener('load', () => {
            init();
            requestAnimationFrame(animate);
        });
    }

    // ============================================
    // Smooth Scroll
    // ============================================
    document.querySelectorAll('a[href^="#"]').forEach(anchor => {
        anchor.addEventListener('click', function(e) {
            e.preventDefault();
            const target = document.querySelector(this.getAttribute('href'));
            if (target) {
                target.scrollIntoView({ behavior: 'smooth' });
            }
        });
    });

})();
