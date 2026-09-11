import {
    OpenFile,
    LoadFile,
    ExtractSingle,
    BatchExtract,
    GetAppInfo,
    OpenInExplorer,
    GetSavedSettings,
    SaveTheme,
    SaveWindowGeometry
} from '../wailsjs/go/main/App';
import { OnFileDrop, EventsOn } from '../wailsjs/runtime/runtime';

// Application State
let currentGroups = [];
let groupMap = new Map();
let selectedGroupIds = new Set();
let activePreviewId = null;
let lastSavedPath = null;
let toastTimeout = null;
let resizeDebounce = null;

// DOM Elements
const statusText = document.getElementById('status-text');
const loadingSpinner = document.getElementById('loading-spinner');
const btnOpenFile = document.getElementById('btn-open-file');
const btnSelectAll = document.getElementById('btn-select-all');
const btnInfo = document.getElementById('btn-info');
const btnThemeToggle = document.getElementById('btn-theme-toggle');
const themeIcon = document.getElementById('theme-icon');
const themeLabel = document.getElementById('theme-label');
const dropzoneGuide = document.getElementById('dropzone-guide');
const btnBrowseGuide = document.getElementById('btn-browse-guide');
const iconGrid = document.getElementById('icon-grid');

// Inspector Elements
const inspectorPreviewImg = document.getElementById('inspector-preview-img');
const previewPlaceholder = document.getElementById('preview-placeholder');
const inspectorTitle = document.getElementById('inspector-title');
const inspectorBadge = document.getElementById('inspector-badge');
const layerCount = document.getElementById('layer-count');
const layersTbody = document.getElementById('layers-tbody');
const btnExtractIco = document.getElementById('btn-extract-ico');
const btnExtractPng = document.getElementById('btn-extract-png');

// Bottom Bar Elements
const selectedSummary = document.getElementById('selected-summary');
const btnBatchPng = document.getElementById('btn-batch-png');
const btnBatchIco = document.getElementById('btn-batch-ico');

// Modal Elements
const aboutModal = document.getElementById('about-modal');
const btnCloseModal = document.getElementById('btn-close-modal');
const btnModalOk = document.getElementById('btn-modal-ok');
const modalAppName = document.getElementById('modal-app-name');
const modalAppVersion = document.getElementById('modal-app-version');
const modalAppDesc = document.getElementById('modal-app-desc');

// Toast Elements
const toast = document.getElementById('toast');
const toastMessage = document.getElementById('toast-message');
const toastAction = document.getElementById('toast-action');

// Theme Management
let currentThemeMode = localStorage.getItem('iconstudio_theme') || 'system';

function applyTheme(mode, persist = true) {
    currentThemeMode = mode;
    localStorage.setItem('iconstudio_theme', mode);

    let effectiveTheme = mode;
    if (mode === 'system') {
        const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
        effectiveTheme = prefersDark ? 'dark' : 'light';
    }

    document.documentElement.setAttribute('data-theme', effectiveTheme);

    if (mode === 'system') {
        themeIcon.textContent = '💻';
        themeLabel.textContent = '시스템';
    } else if (mode === 'dark') {
        themeIcon.textContent = '🌙';
        themeLabel.textContent = '다크';
    } else {
        themeIcon.textContent = '☀️';
        themeLabel.textContent = '라이트';
    }

    if (persist && typeof SaveTheme === 'function') {
        SaveTheme(mode).catch(() => {});
    }
}

function handleThemeToggle() {
    if (currentThemeMode === 'system') {
        applyTheme('dark', true);
    } else if (currentThemeMode === 'dark') {
        applyTheme('light', true);
    } else {
        applyTheme('system', true);
    }
}

// Initialize
window.addEventListener('DOMContentLoaded', async () => {
    try {
        const saved = await GetSavedSettings();
        if (saved && saved.theme) {
            currentThemeMode = saved.theme;
        }
    } catch (e) {}

    applyTheme(currentThemeMode, false);
    initEvents();
    initAppInfo();
});

function initEvents() {
    btnOpenFile.addEventListener('click', handleOpenFile);
    btnBrowseGuide.addEventListener('click', handleOpenFile);
    btnSelectAll.addEventListener('click', handleToggleSelectAll);
    btnThemeToggle.addEventListener('click', handleThemeToggle);
    btnInfo.addEventListener('click', openAboutModal);
    btnCloseModal.addEventListener('click', closeAboutModal);
    btnModalOk.addEventListener('click', closeAboutModal);

    // Watch for Windows OS theme changes
    if (window.matchMedia) {
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
            if (currentThemeMode === 'system') {
                applyTheme('system', false);
            }
        });
    }

    // Debounce save window geometry on resize
    window.addEventListener('resize', () => {
        clearTimeout(resizeDebounce);
        resizeDebounce = setTimeout(() => {
            if (typeof SaveWindowGeometry === 'function') {
                SaveWindowGeometry().catch(() => {});
            }
        }, 600);
    });

    btnExtractIco.addEventListener('click', () => handleExtractSingle('ico'));
    btnExtractPng.addEventListener('click', () => handleExtractSingle('png'));

    btnBatchIco.addEventListener('click', () => handleBatchExtract('ico'));
    btnBatchPng.addEventListener('click', () => handleBatchExtract('png'));

    toastAction.addEventListener('click', () => {
        if (lastSavedPath) {
            OpenInExplorer(lastSavedPath);
        }
    });

    // Prevent browser opening file as new tab
    window.addEventListener('dragover', (e) => {
        e.preventDefault();
    }, false);
    window.addEventListener('drop', (e) => {
        e.preventDefault();
    }, false);

    // Native Wails Drag & Drop listener (useDropTarget=false to allow drop anywhere on window)
    try {
        OnFileDrop((x, y, paths) => {
            if (paths && paths.length > 0) {
                loadTargetFile(paths[0]);
            }
        }, false);
    } catch (err) {
        console.warn('OnFileDrop registration warning:', err);
    }

    // Backend EventsEmit fallback listener
    try {
        EventsOn('file-dropped', (filePath) => {
            if (filePath) {
                loadTargetFile(filePath);
            }
        });
    } catch (err) {
        console.warn('EventsOn file-dropped registration warning:', err);
    }
}

async function initAppInfo() {
    try {
        const info = await GetAppInfo();
        if (info) {
            modalAppName.textContent = info.name || 'Icon Studio';
            modalAppVersion.textContent = `${info.version} (Build ${info.build})`;
            modalAppDesc.textContent = info.description || '';
        }
    } catch (e) {
        console.error('Failed to load app info', e);
    }
}

async function handleOpenFile() {
    try {
        setLoading(true, '파일 선택 중...');
        const result = await OpenFile();
        if (result && result.groups) {
            renderParsedResult(result);
        } else {
            setLoading(false, currentGroups.length > 0 ? `총 ${currentGroups.length}개 발견` : '파일을 열거나 창 위로 끌어다 놓으세요');
        }
    } catch (err) {
        setLoading(false, '파일 열기 오류');
        showToast(`오류: ${err}`);
    }
}

async function loadTargetFile(filePath) {
    try {
        setLoading(true, `${getFileName(filePath)} 분석 중...`);
        const result = await LoadFile(filePath);
        if (result && result.groups) {
            renderParsedResult(result);
        } else {
            setLoading(false, '아이콘 리소스 없음');
            showToast('아이콘 리소스를 찾을 수 없습니다.');
        }
    } catch (err) {
        setLoading(false, '분석 실패');
        showToast(`오류: ${err}`);
    }
}

function renderParsedResult(result) {
    currentGroups = result.groups || [];
    groupMap.clear();
    selectedGroupIds.clear();

    currentGroups.forEach(g => groupMap.set(g.group_id, g));

    dropzoneGuide.classList.add('hidden');
    iconGrid.classList.remove('hidden');
    iconGrid.innerHTML = '';

    // Create cards
    currentGroups.forEach((group) => {
        const card = document.createElement('div');
        card.className = 'icon-card';
        card.id = `card-${group.group_id}`;

        let uhdBadgeHtml = group.has_uhd ? `<span class="badge-uhd">UHD</span>` : '';

        card.innerHTML = `
            ${uhdBadgeHtml}
            <div class="card-img-wrapper">
                <img src="${group.thumbnail}" class="card-thumb" alt="Icon #${group.group_id}" title="아이콘 #${group.group_id} (${group.max_res}px, ${group.total_layers}개 규격)"/>
            </div>
        `;

        card.addEventListener('click', (e) => handleCardClick(e, group.group_id));
        iconGrid.appendChild(card);
    });

    btnSelectAll.disabled = currentGroups.length === 0;
    setLoading(false, `${result.file_name} (총 ${currentGroups.length}개 발견)`);

    // Select first icon by default
    if (currentGroups.length > 0) {
        const firstId = currentGroups[0].group_id;
        selectedGroupIds.add(firstId);
        inspectIcon(firstId);
        updateSelectionUI();
    }
}

function handleCardClick(event, groupId) {
    if (event.ctrlKey || event.metaKey) {
        // Multi-select toggle
        if (selectedGroupIds.has(groupId)) {
            selectedGroupIds.delete(groupId);
        } else {
            selectedGroupIds.add(groupId);
        }
    } else {
        // Single selection
        selectedGroupIds.clear();
        selectedGroupIds.add(groupId);
    }

    inspectIcon(groupId);
    updateSelectionUI();
}

function handleToggleSelectAll() {
    if (selectedGroupIds.size === currentGroups.length) {
        selectedGroupIds.clear();
        btnSelectAll.textContent = '전체 선택';
    } else {
        currentGroups.forEach(g => selectedGroupIds.add(g.group_id));
        btnSelectAll.textContent = '선택 해제';
    }
    updateSelectionUI();
}

function updateSelectionUI() {
    const count = selectedGroupIds.size;
    selectedSummary.textContent = `${count}개 선택됨`;

    const hasSelection = count > 0;
    btnBatchIco.disabled = !hasSelection;
    btnBatchPng.disabled = !hasSelection;

    // Update card styles
    currentGroups.forEach(g => {
        const el = document.getElementById(`card-${g.group_id}`);
        if (el) {
            if (selectedGroupIds.has(g.group_id)) {
                el.classList.add('selected');
            } else {
                el.classList.remove('selected');
            }
        }
    });

    if (currentGroups.length > 0) {
        btnSelectAll.textContent = (count === currentGroups.length) ? '선택 해제' : '전체 선택';
    }
}

function inspectIcon(groupId) {
    activePreviewId = groupId;
    const group = groupMap.get(groupId);
    if (!group) return;

    // Update preview
    if (group.thumbnail) {
        inspectorPreviewImg.src = group.thumbnail;
        inspectorPreviewImg.classList.remove('hidden');
        previewPlaceholder.classList.add('hidden');
    }

    inspectorTitle.textContent = `아이콘 #${groupId} (${group.total_layers}개 레이어)`;
    
    if (group.has_uhd) {
        inspectorBadge.textContent = 'UHD 256px';
        inspectorBadge.classList.remove('hidden');
    } else {
        inspectorBadge.textContent = `${group.max_res}px`;
        inspectorBadge.classList.remove('hidden');
    }

    layerCount.textContent = group.total_layers;

    // Render layers table
    layersTbody.innerHTML = '';
    const sortedLayers = [...group.layers].sort((a, b) => b.width - a.width);

    sortedLayers.forEach(l => {
        const tr = document.createElement('tr');
        const formatBadge = l.is_png ? '<span style="color:#818CF8;font-weight:600;">PNG</span>' : 'DIB';
        tr.innerHTML = `
            <td>${l.width} × ${l.height} px</td>
            <td>${l.bit_count} bit</td>
            <td>${formatBadge}</td>
        `;
        layersTbody.appendChild(tr);
    });

    btnExtractIco.disabled = false;
    btnExtractPng.disabled = false;
}

async function handleExtractSingle(format) {
    if (!activePreviewId) return;

    try {
        const path = await ExtractSingle(activePreviewId, format);
        if (path) {
            lastSavedPath = path;
            showToast(`저장 완료: ${getFileName(path)}`, true);
        }
    } catch (err) {
        showToast(`저장 실패: ${err}`);
    }
}

async function handleBatchExtract(format) {
    if (selectedGroupIds.size === 0) return;

    const ids = Array.from(selectedGroupIds);
    try {
        setLoading(true, '일괄 저장 중...');
        const count = await BatchExtract(ids, format);
        setLoading(false, `${currentGroups.length}개 발견`);
        if (count > 0) {
            showToast(`${count}개의 ${format.toUpperCase()} 파일이 저장되었습니다.`);
        }
    } catch (err) {
        setLoading(false);
        showToast(`일괄 저장 오류: ${err}`);
    }
}

function setLoading(isLoading, text) {
    if (isLoading) {
        loadingSpinner.classList.remove('hidden');
    } else {
        loadingSpinner.classList.add('hidden');
    }
    if (text) {
        statusText.textContent = text;
    }
}

function showToast(message, showAction = false) {
    if (toastTimeout) {
        clearTimeout(toastTimeout);
    }
    toastMessage.textContent = message;
    if (showAction) {
        toastAction.classList.remove('hidden');
    } else {
        toastAction.classList.add('hidden');
    }
    toast.classList.remove('hidden');

    toastTimeout = setTimeout(() => {
        toast.classList.add('hidden');
    }, 4500);
}

function openAboutModal() {
    aboutModal.classList.remove('hidden');
}

function closeAboutModal() {
    aboutModal.classList.add('hidden');
}

function getFileName(filePath) {
    if (!filePath) return '';
    const parts = filePath.split(/[\\/]/);
    return parts[parts.length - 1];
}
