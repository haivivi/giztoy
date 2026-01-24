/**
 * Giztoy Homepage - Interactive Starfield Navigation
 * 
 * Click anywhere to fly towards that direction!
 * Like piloting a spaceship through the stars.
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
            // Realistic star colors - mostly white with subtle tints
            colors: [
                [255, 255, 255],   // Pure white (most common)
                [255, 255, 255],   // Pure white
                [255, 255, 255],   // Pure white
                [255, 255, 250],   // Warm white
                [250, 250, 255],   // Cool white
                [255, 252, 240],   // Slight yellow (like our sun)
                [240, 248, 255],   // Slight blue (hot stars)
            ]
        },
        flight: {
            maxSpeed: 40,
            acceleration: 1.5,
            deceleration: 0.94,
            minSpeed: 0.05,
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
    let isFlying = false;

    function resize() {
        width = window.innerWidth;
        height = window.innerHeight;
        centerX = width / 2;
        centerY = height / 2;
        canvas.width = width;
        canvas.height = height;
        
        // Clear completely on resize
        ctx.fillStyle = 'rgb(10, 10, 15)';
        ctx.fillRect(0, 0, width, height);
    }

    // ============================================
    // Star Class
    // ============================================
    class Star {
        constructor() {
            this.reset(true);
        }

        reset(randomZ = false) {
            // Position in 3D space
            this.x = (Math.random() - 0.5) * width * 2;
            this.y = (Math.random() - 0.5) * height * 2;
            this.z = randomZ ? Math.random() * CONFIG.stars.depth : CONFIG.stars.depth;
            
            // Previous Z for trail
            this.pz = this.z;
            
            // Star appearance
            const colorIndex = Math.floor(Math.random() * CONFIG.stars.colors.length);
            this.color = CONFIG.stars.colors[colorIndex];
            this.brightness = 0.6 + Math.random() * 0.4;
        }

        update() {
            this.pz = this.z;
            
            // Move star based on flight direction
            this.x -= targetX * speed * 1.2;
            this.y -= targetY * speed * 1.2;
            this.z -= speed;
            
            // Reset if passed camera
            if (this.z < 1) {
                // Reset to far away, in a spread area
                this.x = (Math.random() - 0.5) * width * 2;
                this.y = (Math.random() - 0.5) * height * 2;
                this.z = CONFIG.stars.depth;
                this.pz = this.z;  // Important: reset pz too to avoid trail artifact
            }
        }

        draw() {
            const fov = 400;
            const scale = fov / this.z;
            const x2d = centerX + this.x * scale;
            const y2d = centerY + this.y * scale;
            
            // Skip if off screen
            if (x2d < -50 || x2d > width + 50 || y2d < -50 || y2d > height + 50) {
                return;
            }
            
            // Size based on depth
            const size = Math.max(0.3, (1 - this.z / CONFIG.stars.depth) * 2.5);
            
            // Alpha based on depth
            const alpha = Math.min(1, (1 - this.z / CONFIG.stars.depth) * this.brightness * 1.2);
            
            const [r, g, b] = this.color;
            
            // Draw trail when flying fast
            if (speed > 8) {
                const pScale = fov / this.pz;
                const px2d = centerX + this.x * pScale;
                const py2d = centerY + this.y * pScale;
                
                const trailAlpha = alpha * Math.min(0.6, (speed - 8) / 30);
                
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
    // Initialization
    // ============================================
    function init() {
        resize();
        
        // Clear canvas completely first
        ctx.fillStyle = 'rgb(10, 10, 15)';
        ctx.fillRect(0, 0, width, height);
        
        // Create stars
        stars = [];
        for (let i = 0; i < CONFIG.stars.count; i++) {
            stars.push(new Star());
        }
        
        // Draw initial static frame (no animation yet)
        for (const star of stars) {
            star.draw();
        }
        
        // Mark as initialized
        initialized = true;
        
        // Fade in the canvas after a short delay to ensure everything is painted
        setTimeout(() => {
            canvas.classList.remove('starfield-loading');
            canvas.classList.add('starfield-ready');
        }, 50);
    }

    // ============================================
    // Animation Loop
    // ============================================
    function animate() {
        if (!initialized) {
            requestAnimationFrame(animate);
            return;
        }
        
        // Clear with motion blur effect
        // Use less transparency when still, more when moving fast
        const fadeAlpha = speed > 15 ? 0.15 : (speed > 5 ? 0.25 : 0.5);
        ctx.fillStyle = `rgba(10, 10, 15, ${fadeAlpha})`;
        ctx.fillRect(0, 0, width, height);
        
        // Update speed
        if (isFlying) {
            speed = Math.min(speed + CONFIG.flight.acceleration, CONFIG.flight.maxSpeed);
        } else {
            speed *= CONFIG.flight.deceleration;
            if (speed < CONFIG.flight.minSpeed) {
                speed = 0;
            }
        }
        
        // Update and draw stars
        for (const star of stars) {
            if (speed > 0) {
                star.update();
            }
            star.draw();
        }
        
        requestAnimationFrame(animate);
    }

    // ============================================
    // Flight Control
    // ============================================
    function startFlight(clientX, clientY) {
        targetX = (clientX - centerX) / centerX;
        targetY = (clientY - centerY) / centerY;
        
        // Normalize
        const magnitude = Math.sqrt(targetX * targetX + targetY * targetY);
        if (magnitude > 1) {
            targetX /= magnitude;
            targetY /= magnitude;
        }
        
        isFlying = true;
    }

    function stopFlight() {
        isFlying = false;
    }

    function updateFlightDirection(clientX, clientY) {
        if (isFlying) {
            targetX = (clientX - centerX) / centerX;
            targetY = (clientY - centerY) / centerY;
            
            const magnitude = Math.sqrt(targetX * targetX + targetY * targetY);
            if (magnitude > 1) {
                targetX /= magnitude;
                targetY /= magnitude;
            }
        }
    }

    // ============================================
    // Event Listeners
    // ============================================
    window.addEventListener('resize', () => {
        resize();
        // Reinitialize stars on resize
        stars = [];
        for (let i = 0; i < CONFIG.stars.count; i++) {
            stars.push(new Star());
        }
    });

    // Mouse events
    document.addEventListener('mousedown', (e) => {
        if (e.target.closest('a, button, .nav, .btn, input, textarea')) return;
        e.preventDefault();
        startFlight(e.clientX, e.clientY);
    });

    document.addEventListener('mouseup', stopFlight);
    
    // Also stop when mouse leaves window
    document.addEventListener('mouseleave', stopFlight);
    window.addEventListener('blur', stopFlight);

    document.addEventListener('mousemove', (e) => {
        updateFlightDirection(e.clientX, e.clientY);
    });

    // Touch events
    document.addEventListener('touchstart', (e) => {
        if (e.target.closest('a, button, .nav, .btn, input, textarea')) return;
        const touch = e.touches[0];
        startFlight(touch.clientX, touch.clientY);
    }, { passive: true });

    document.addEventListener('touchend', stopFlight);
    document.addEventListener('touchcancel', stopFlight);

    document.addEventListener('touchmove', (e) => {
        const touch = e.touches[0];
        updateFlightDirection(touch.clientX, touch.clientY);
    }, { passive: true });

    // Keyboard
    document.addEventListener('keydown', (e) => {
        if (e.code === 'Space' && !e.target.matches('input, textarea')) {
            e.preventDefault();
            targetX = 0;
            targetY = 0;
            isFlying = true;
        }
    });

    document.addEventListener('keyup', (e) => {
        if (e.code === 'Space') {
            stopFlight();
        }
    });

    // ============================================
    // Start (wait for page fully loaded)
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
