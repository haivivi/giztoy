// Match Benchmark - Frontend Application
// API base URL (empty = same origin, can be configured)
const API_BASE = window.MATCHTEST_API || '';

// State
let report = { models: [], test_count: 0, rule_count: 0, timestamp: '' };
let models = [];
let isLive = false;
let sortCol = 'pass_rate';
let sortDir = 'desc';

// DOM Elements
const metaEl = document.getElementById('meta');
const statusBanner = document.getElementById('status-banner');
const statusTitle = document.getElementById('status-title');
const statusInfo = document.getElementById('status-info');
const progressGrid = document.getElementById('progress-grid');
const overviewBody = document.getElementById('overview-body');
const resultsThead = document.getElementById('results-thead');
const resultsBody = document.getElementById('results-body');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    fetchReport();
    checkLive();
    setupSorting();
});

// API Functions
async function fetchReport() {
    try {
        const resp = await fetch(`${API_BASE}/api/report`);
        report = await resp.json();
        models = report.models || [];
        updateMeta();
        renderOverviewTable();
        renderResultsTable();
    } catch (e) {
        console.error('Failed to fetch report:', e);
    }
}

async function checkLive() {
    try {
        const resp = await fetch(`${API_BASE}/api/status`);
        const status = await resp.json();
        if (status.status === 'running' || status.status === 'idle') {
            connectSSE();
        }
    } catch (e) {
        // Static mode, no SSE
    }
}

// SSE Connection
function connectSSE() {
    const eventSource = new EventSource(`${API_BASE}/api/events`);

    eventSource.onmessage = function (event) {
        const data = JSON.parse(event.data);
        handleUpdate(data);
    };

    eventSource.onerror = function () {
        console.log('SSE connection lost, retrying...');
        eventSource.close();
        setTimeout(connectSSE, 2000);
    };
}

function handleUpdate(data) {
    if (data.type === 'init' || data.type === 'start') {
        isLive = true;
        updateProgressBanner(data.status, data.models || []);
    } else if (data.type === 'case_done' || data.type === 'model_done' || data.type === 'model_start') {
        updateProgressBanner(data.status, data.models || []);
    } else if (data.type === 'all_done') {
        isLive = false;
        statusBanner.className = 'status-banner done';
        statusTitle.textContent = '✓ Benchmark Complete';
        statusInfo.textContent = data.status?.duration || '';
        fetchReport();
    }
}

function updateProgressBanner(status, progressModels) {
    if (status?.status === 'running') {
        statusBanner.className = 'status-banner running';
        statusTitle.textContent = '⏳ Running Benchmark...';
    }

    let html = '';
    progressModels.forEach(m => {
        const statusClass = m.status === 'done' ? 'done' : (m.status === 'running' ? 'running' : '');
        html += `
            <div class="progress-card ${statusClass}">
                <div class="progress-model">${escapeHtml(m.model)}</div>
                <div class="progress-bar-bg">
                    <div class="progress-bar" style="width: ${m.percent || 0}%"></div>
                </div>
                <div class="progress-stats">
                    <span>${m.done || 0}/${m.total || 0}</span>
                    <span class="pass">✓${m.passed || 0}</span>
                    <span class="fail">✗${m.failed || 0}</span>
                </div>
            </div>
        `;
    });
    progressGrid.innerHTML = html;
}

// Rendering Functions
function updateMeta() {
    metaEl.textContent = `${report.timestamp || '-'} · ${report.rule_count || 0} rules · ${report.test_count || 0} tests`;
}

function renderOverviewTable() {
    if (models.length === 0) {
        overviewBody.innerHTML = '<tr><td colspan="9" style="text-align:center; color: var(--text-muted);">Waiting for results...</td></tr>';
        return;
    }

    // Sort models
    const sorted = [...models].sort((a, b) => {
        let av, bv;
        switch (sortCol) {
            case 'model': av = a.model; bv = b.model; break;
            case 'total': av = a.total_cases; bv = b.total_cases; break;
            case 'pass_rate': av = a.pass_rate; bv = b.pass_rate; break;
            case 'passed': av = a.passed; bv = b.passed; break;
            case 'failed': av = a.failed; bv = b.failed; break;
            case 'errors': av = a.errors; bv = b.errors; break;
            case 'p50': av = a.p50_ms; bv = b.p50_ms; break;
            case 'p95': av = a.p95_ms; bv = b.p95_ms; break;
            case 'p99': av = a.p99_ms; bv = b.p99_ms; break;
            default: return 0;
        }
        if (typeof av === 'string') {
            return sortDir === 'asc' ? av.localeCompare(bv) : bv.localeCompare(av);
        }
        return sortDir === 'asc' ? av - bv : bv - av;
    });

    let html = '';
    sorted.forEach(m => {
        const rateClass = m.pass_rate >= 80 ? 'high' : (m.pass_rate >= 50 ? 'mid' : 'low');
        html += '<tr>';
        html += `<td>${escapeHtml(m.model)}</td>`;
        html += `<td>${m.total_cases ?? 0}</td>`;
        html += `<td><span class="pass-rate ${rateClass}">${(m.pass_rate || 0).toFixed(1)}%</span></td>`;
        html += `<td style="color: var(--green);">${m.passed || 0}</td>`;
        html += `<td style="color: var(--red);">${m.failed || 0}</td>`;
        html += `<td style="color: var(--yellow);">${m.errors || 0}</td>`;
        html += `<td class="speed">${m.p50_ms ?? 0}ms</td>`;
        html += `<td class="speed">${m.p95_ms ?? 0}ms</td>`;
        html += `<td class="speed">${m.p99_ms ?? 0}ms</td>`;
        html += '</tr>';
    });
    overviewBody.innerHTML = html;

    // Update header sort indicators
    document.querySelectorAll('.overview-table th').forEach(th => {
        th.classList.remove('sorted-asc', 'sorted-desc');
        if (th.dataset.col === sortCol) {
            th.classList.add(sortDir === 'asc' ? 'sorted-asc' : 'sorted-desc');
        }
    });
}

function setupSorting() {
    document.querySelectorAll('.overview-table th').forEach(th => {
        th.addEventListener('click', () => {
            const col = th.dataset.col;
            if (!col) return;
            if (sortCol === col) {
                sortDir = sortDir === 'asc' ? 'desc' : 'asc';
            } else {
                sortCol = col;
                sortDir = th.dataset.type === 'string' ? 'asc' : 'desc';
            }
            renderOverviewTable();
        });
    });
}

function renderResultsTable() {
    // Build case index: caseIdx -> model -> result
    const caseResults = {};
    models.forEach(m => {
        (m.cases || []).forEach((c, idx) => {
            if (!caseResults[idx]) {
                caseResults[idx] = { input: c.input, expected: c.expected, models: {} };
            }
            caseResults[idx].models[m.model] = c;
        });
    });

    // Update header
    let headerHtml = '<tr><th>Test Case</th>';
    models.forEach(m => {
        headerHtml += `<th>${escapeHtml(m.model)}</th>`;
    });
    headerHtml += '</tr>';
    resultsThead.innerHTML = headerHtml;

    // Update body
    if (Object.keys(caseResults).length === 0) {
        resultsBody.innerHTML = `<tr><td colspan="${models.length + 1}" style="text-align:center; color: var(--text-muted);">No results yet</td></tr>`;
        return;
    }

    let html = '';
    Object.entries(caseResults).forEach(([idx, data]) => {
        const rowId = 'row-' + idx;
        const detailId = 'detail-' + idx;
        const expectedArgs = formatArgs(data.expected?.args);

        // Main row
        html += `<tr id="${rowId}" onclick="toggleDetail('${idx}')">`;
        html += '<td class="case-cell">';
        html += `<div class="case-input">${escapeHtml(truncate(data.input, 50))}</div>`;
        html += `<div class="case-expected"><code>${escapeHtml(data.expected?.rule || '')}</code>`;
        if (expectedArgs) html += ' ' + escapeHtml(expectedArgs);
        html += '</div>';
        html += '</td>';

        models.forEach(m => {
            const result = data.models[m.model];
            html += `<td>${getStatusIcon(result)}</td>`;
        });
        html += '</tr>';

        // Detail row
        html += `<tr id="${detailId}" class="detail-row">`;
        html += `<td colspan="${models.length + 1}">`;
        html += `<div><strong>Input:</strong> ${escapeHtml(data.input)}</div>`;
        html += `<div style="margin-top: 0.5rem;"><strong>Expected:</strong> <code>${escapeHtml(data.expected?.rule || '')}</code> ${escapeHtml(expectedArgs)}</div>`;
        html += '<div class="detail-content" style="margin-top: 1rem;">';

        models.forEach(m => {
            const result = data.models[m.model];
            if (!result) return;

            const actualArgs = formatArgs(result.actual?.args);
            const statusClass = result.status === 'pass' ? 'pass' : 'fail';

            html += '<div class="detail-model">';
            html += `<div class="detail-model-name">${escapeHtml(m.model)}</div>`;
            html += `<div class="detail-item"><span class="detail-label">Status:</span> <span class="detail-value ${statusClass}">${escapeHtml(result.status)}</span></div>`;
            html += `<div class="detail-item"><span class="detail-label">Rule:</span> <span class="detail-value">${escapeHtml(result.actual?.rule || '-')}</span></div>`;
            if (actualArgs) {
                html += `<div class="detail-item"><span class="detail-label">Args:</span> <span class="detail-value">${escapeHtml(actualArgs)}</span></div>`;
            }
            html += `<div class="detail-item"><span class="detail-label">Time:</span> <span class="detail-value">${result.duration_ms}ms</span></div>`;
            if (result.error) {
                html += `<div class="detail-item"><span class="detail-label">Error:</span> <span class="detail-value fail">${escapeHtml(result.error)}</span></div>`;
            }
            html += '</div>';
        });

        html += '</div></td></tr>';
    });

    resultsBody.innerHTML = html;
}

// Utility Functions
function getStatusIcon(result) {
    if (!result) return '<span class="status status-pending">○</span>';
    if (result.status === 'error') return `<span class="status status-error" title="Error: ${escapeHtml(result.error || '')}">⚠</span>`;
    if (result.status === 'pass') return '<span class="status status-pass">✓</span>';
    // Check partial match (rule matches but args differ)
    if (result.actual && result.actual.rule === result.expected?.rule) {
        return '<span class="status status-partial" title="Rule matched, args differ">◐</span>';
    }
    return '<span class="status status-fail">✗</span>';
}

function truncate(str, len) {
    if (!str) return '';
    return str.length > len ? str.substring(0, len) + '...' : str;
}

function formatArgs(args) {
    if (!args || Object.keys(args).length === 0) return '';
    return Object.entries(args).map(([k, v]) => `${k}=${v}`).join(', ');
}

function escapeHtml(str) {
    if (!str) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

// Global function for row click
window.toggleDetail = function(idx) {
    const detailRow = document.getElementById('detail-' + idx);
    if (detailRow) {
        detailRow.classList.toggle('expanded');
    }
};
