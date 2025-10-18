// State
let uploadedFiles = [];
let lastUploadedCID = '';
let stats = { files: 0, size: 0, downloads: 0 };

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    loadStats();
    setupUploadZone();
});

// Tab switching
function switchTab(tabName) {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

    event.target.classList.add('active');
    document.getElementById(tabName + '-tab').classList.add('active');

    if (tabName === 'about') {
        updateStatsDisplay();
    }
}

// Upload Zone Setup
function setupUploadZone() {
    const uploadZone = document.getElementById('uploadZone');
    const fileInput = document.getElementById('fileInput');

    uploadZone.addEventListener('click', () => fileInput.click());

    uploadZone.addEventListener('dragover', (e) => {
        e.preventDefault();
        uploadZone.classList.add('dragover');
    });

    uploadZone.addEventListener('dragleave', () => {
        uploadZone.classList.remove('dragover');
    });

    uploadZone.addEventListener('drop', (e) => {
        e.preventDefault();
        uploadZone.classList.remove('dragover');
        handleFiles(e.dataTransfer.files);
    });

    fileInput.addEventListener('change', (e) => {
        handleFiles(e.target.files);
    });
}

function handleFiles(files) {
    uploadedFiles = Array.from(files);
    displayFileList();
    uploadFiles();
}

function displayFileList() {
    const fileList = document.getElementById('fileList');
    fileList.innerHTML = '';

    uploadedFiles.forEach((file, idx) => {
        const item = document.createElement('div');
        item.className = 'file-item';
        item.innerHTML = `
            <div class="file-icon">📄</div>
            <div class="file-info">
                <div class="file-name">${file.name}</div>
                <div class="file-size">${formatSize(file.size)}</div>
            </div>
            <div class="file-actions">
                <button onclick="removeFile(${idx})">❌ Remove</button>
            </div>
        `;
        fileList.appendChild(item);
    });
}

async function uploadFiles() {
    if (uploadedFiles.length === 0) return;

    const progressContainer = document.getElementById('progressContainer');
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');
    const uploadResult = document.getElementById('uploadResult');

    progressContainer.style.display = 'block';
    uploadResult.classList.remove('show');

    for (let i = 0; i < uploadedFiles.length; i++) {
        const file = uploadedFiles[i];
        const formData = new FormData();
        formData.append('file', file);

        progressText.textContent = `Uploading ${file.name} (${i+1}/${uploadedFiles.length})...`;

        try {
            const response = await fetch('/api/v0/add', {
                method: 'POST',
                body: formData
            });

            if (!response.ok) {
                throw new Error('Upload failed: ' + response.statusText);
            }

            const result = await response.json();
            lastUploadedCID = result.Hash;

            // Update stats
            stats.files++;
            stats.size += file.size;
            saveStats();

            const progress = ((i + 1) / uploadedFiles.length) * 100;
            progressFill.style.width = progress + '%';

        } catch (error) {
            alert('Upload failed: ' + error.message);
            progressContainer.style.display = 'none';
            return;
        }
    }

    // Show success
    progressText.textContent = '✅ Upload complete!';
    setTimeout(() => {
        progressContainer.style.display = 'none';
        showUploadResult();
    }, 1000);
}

function showUploadResult() {
    const result = document.getElementById('uploadResult');
    const cidDisplay = document.getElementById('resultCID');
    cidDisplay.textContent = lastUploadedCID;
    result.classList.add('show');
}

function removeFile(idx) {
    uploadedFiles.splice(idx, 1);
    displayFileList();
}

function copyToClipboard() {
    navigator.clipboard.writeText(lastUploadedCID);
    alert('✅ CID copied to clipboard!');
}

function viewContent() {
    window.open('/ipfs/' + lastUploadedCID, '_blank');
}

function shareLink() {
    const link = window.location.origin + '/ipfs/' + lastUploadedCID;
    navigator.clipboard.writeText(link);
    alert('✅ Share link copied to clipboard!');
}

// Download functionality
async function fetchContent() {
    const cid = document.getElementById('cidInput').value.trim();
    if (!cid) {
        alert('Please enter a CID');
        return;
    }

    try {
        const response = await fetch('/api/v0/object/stat?cid=' + cid);

        if (!response.ok) {
            throw new Error('Content not found');
        }

        const info = await response.json();

        const downloadResult = document.getElementById('downloadResult');
        const downloadInfo = document.getElementById('downloadInfo');

        downloadInfo.innerHTML = `
            <div style="margin: 12px 0;">
                <strong>CID:</strong> <code style="word-break: break-all;">${cid}</code><br>
                <strong>Size:</strong> ${formatSize(info.DataSize || info.CumulativeSize)}
            </div>
        `;

        downloadResult.classList.add('show');

        // Update stats
        stats.downloads++;
        saveStats();

    } catch (error) {
        alert('Failed to fetch content: ' + error.message);
    }
}

function openInBrowser() {
    const cid = document.getElementById('cidInput').value.trim();
    window.open('/ipfs/' + cid, '_blank');
}

function downloadFile() {
    const cid = document.getElementById('cidInput').value.trim();
    window.location.href = '/ipfs/' + cid;
}

// Stats
function saveStats() {
    localStorage.setItem('ipfs-gateway-stats', JSON.stringify(stats));
}

function loadStats() {
    const saved = localStorage.getItem('ipfs-gateway-stats');
    if (saved) {
        stats = JSON.parse(saved);
    }
}

function updateStatsDisplay() {
    document.getElementById('statFiles').textContent = stats.files;
    document.getElementById('statSize').textContent = formatSize(stats.size);
    document.getElementById('statDownloads').textContent = stats.downloads;
}

// Utilities
function formatSize(bytes) {
    if (bytes === 0) return '0 B';
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
    return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
}
