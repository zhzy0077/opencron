document.addEventListener('DOMContentLoaded', initializeUI);

function getApiKey() {
    return localStorage.getItem('opencron_api_key') || '';
}

function setStatus(message) {
    const status = document.getElementById('statusMessage');
    status.textContent = message || '';
}

async function apiFetch(url, options = {}) {
    const headers = {
        ...(options.headers || {})
    };
    const apiKey = getApiKey();
    if (apiKey) {
        headers['X-API-Key'] = apiKey;
    }

    const response = await fetch(url, { ...options, headers });
    if (response.status === 401) {
        setStatus('Unauthorized (401). Set a valid API Key.');
    } else {
        setStatus('');
    }
    return response;
}

function initializeUI() {
    const apiKeyInput = document.getElementById('apiKey');
    apiKeyInput.value = getApiKey();

    document.getElementById('saveApiKeyBtn').addEventListener('click', () => {
        localStorage.setItem('opencron_api_key', apiKeyInput.value.trim());
        setStatus('API Key saved.');
        loadTasks();
    });

    document.getElementById('clearApiKeyBtn').addEventListener('click', () => {
        localStorage.removeItem('opencron_api_key');
        apiKeyInput.value = '';
        setStatus('API Key cleared.');
        loadTasks();
    });

    loadTasks();
}

async function loadTasks() {
    const res = await apiFetch('/api/tasks');
    if (!res.ok) {
        const tbody = document.querySelector('tbody');
        tbody.innerHTML = '';
        return;
    }
    const tasks = await res.json();
    const tbody = document.querySelector('tbody');
    tbody.innerHTML = '';
    
    tasks.forEach(task => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${task.id}</td>
            <td>${task.name}</td>
            <td>${task.schedule}</td>
            <td>${task.command}</td>
            <td>${task.enabled ? 'Yes' : 'No'}</td>
            <td>${task.one_shot ? 'Yes' : 'No'}</td>
            <td>${task.last_run ? new Date(task.last_run).toLocaleString() : 'Never'}</td>
            <td>
                <button onclick="runTask(${task.id})">Run</button>
                <button onclick="toggleTask(${task.id}, ${!task.enabled})">${task.enabled ? 'Disable' : 'Enable'}</button>
                <button onclick="editTask(${task.id})">Edit</button>
                <button onclick="showLogs(${task.id})">Logs</button>
                <button onclick="deleteTask(${task.id})">Delete</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
}

async function showLogs(id) {
    const res = await apiFetch(`/api/tasks/${id}/logs`);
    if (!res.ok) {
        document.getElementById('logContent').innerText = `Failed to load logs (${res.status}).`;
        document.getElementById('logModal').style.display = 'block';
        return;
    }
    const logs = await res.text();
    document.getElementById('logContent').innerText = logs;
    document.getElementById('logModal').style.display = 'block';
}

function closeLogModal() {
    document.getElementById('logModal').style.display = 'none';
}

function openModal(task) {
    document.getElementById('taskModal').style.display = 'block';
    if (task) {
        document.getElementById('taskId').value = task.id;
        document.getElementById('name').value = task.name;
        document.getElementById('schedule').value = task.schedule;
        document.getElementById('command').value = task.command;
        document.getElementById('enabled').checked = task.enabled;
        document.getElementById('one_shot').checked = task.one_shot;
        document.getElementById('modalTitle').innerText = 'Edit Task';
    } else {
        document.getElementById('taskForm').reset();
        document.getElementById('taskId').value = '';
        document.getElementById('one_shot').checked = false;
        document.getElementById('modalTitle').innerText = 'Add Task';
    }
}

function closeModal() {
    document.getElementById('taskModal').style.display = 'none';
}

document.getElementById('taskForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('taskId').value;
    const task = {
        name: document.getElementById('name').value,
        schedule: document.getElementById('schedule').value,
        command: document.getElementById('command').value,
        enabled: document.getElementById('enabled').checked,
        one_shot: document.getElementById('one_shot').checked
    };

    if (id) {
        await apiFetch(`/api/tasks/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(task)
        });
    } else {
        await apiFetch('/api/tasks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(task)
        });
    }
    
    closeModal();
    loadTasks();
});

async function editTask(id) {
    const res = await apiFetch('/api/tasks');
    if (!res.ok) {
        return;
    }
    const tasks = await res.json();
    const task = tasks.find(t => t.id === id);
    openModal(task);
}

async function deleteTask(id) {
    if (confirm('Are you sure?')) {
        await apiFetch(`/api/tasks/${id}`, { method: 'DELETE' });
        loadTasks();
    }
}

async function runTask(id) {
    const res = await apiFetch(`/api/tasks/${id}/run`, { method: 'POST' });
    if (!res.ok) {
        setStatus(`Failed to run task ${id} (${res.status}).`);
        return;
    }
    setStatus(`Task ${id} triggered.`);
    await loadTasks();
}

async function toggleTask(id, enabled) {
    const res = await apiFetch(`/api/tasks/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled })
    });
    if (!res.ok) {
        setStatus(`Failed to ${enabled ? 'enable' : 'disable'} task ${id} (${res.status}).`);
        return;
    }
    setStatus(`Task ${id} ${enabled ? 'enabled' : 'disabled'}.`);
    await loadTasks();
}
