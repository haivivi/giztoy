// Mermaid initialization for mdBook
// mermaid.min.js is loaded via additional-js in book.toml

document.addEventListener('DOMContentLoaded', function() {
    // Configure mermaid
    mermaid.initialize({
        startOnLoad: false,
        theme: 'default',
        securityLevel: 'loose',
        flowchart: {
            useMaxWidth: true,
            htmlLabels: true
        }
    });

    // Find all mermaid code blocks and render them
    const mermaidBlocks = document.querySelectorAll('pre code.language-mermaid');
    mermaidBlocks.forEach(function(block) {
        const pre = block.parentElement;
        const code = block.textContent;
        
        // Create a container for the rendered diagram
        const container = document.createElement('div');
        container.className = 'mermaid';
        container.textContent = code;
        
        // Replace the pre element with the mermaid container
        pre.parentNode.replaceChild(container, pre);
    });

    // Render all mermaid diagrams
    mermaid.run();
});
