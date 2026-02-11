document.addEventListener('DOMContentLoaded', loadTasks);

async function loadTasks() {
    const res = await fetch('/api/tasks');
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
            <td>${task.last_run ? new Date(task.last_run).toLocaleString() : 'Never'}</td>
            <td>
                <button onclick="editTask(${task.id})">Edit</button>
                <button onclick="showLogs(${task.id})">Logs</button>
                <button onclick="deleteTask(${task.id})">Delete</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
}

async function showLogs(id) {
    const res = await fetch(`/api/tasks/${id}/logs`);
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
        document.getElementById('modalTitle').innerText = 'Edit Task';
    } else {
        document.getElementById('taskForm').reset();
        document.getElementById('taskId').value = '';
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
        enabled: document.getElementById('enabled').checked
    };

    if (id) {
        await fetch(`/api/tasks/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(task)
        });
    } else {
        await fetch('/api/tasks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(task)
        });
    }
    
    closeModal();
    loadTasks();
});

async function editTask(id) {
    const res = await fetch('/api/tasks');
    const tasks = await res.json();
    const task = tasks.find(t => t.id === id);
    openModal(task);
}

async function deleteTask(id) {
    if (confirm('Are you sure?')) {
        await fetch(`/api/tasks/${id}`, { method: 'DELETE' });
        loadTasks();
    }
}
