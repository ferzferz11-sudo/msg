package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func main() {
	port := os.Getenv("LOG_PORT")
	if port == "" {
		port = "8091"
	}

	serviceName := os.Getenv("LOG_SERVICE")
	if serviceName == "" {
		serviceName = "lavender-server-dev"
	}

	title := os.Getenv("LOG_TITLE")
	if title == "" {
		title = "Lava Dev Server Logs"
	}

	// Main page — auto-refreshing log viewer
	http.HandleFunc("/server-logs-dev", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  background: #0d1117;
  color: #c9d1d9;
  padding: 16px;
  font-size: 13px;
  line-height: 1.5;
}
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 8px 8px 0 0;
  margin-bottom: 0;
}
.header h1 {
  font-size: 16px;
  color: #d29922;
}
.header .status {
  display: flex;
  align-items: center;
  gap: 8px;
}
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%%;
  background: #d29922;
  animation: pulse 2s infinite;
}
@keyframes pulse {
  0%%, 100%% { opacity: 1; }
  50%% { opacity: 0.4; }
}
.toolbar {
  display: flex;
  gap: 8px;
  padding: 8px 16px;
  background: #161b22;
  border: 1px solid #30363d;
  border-top: none;
}
.toolbar button {
  padding: 4px 12px;
  background: #21262d;
  border: 1px solid #30363d;
  color: #c9d1d9;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}
.toolbar button:hover { background: #30363d; }
.toolbar button.active { background: #d29922; border-color: #d29922; color: #000; }
.log-container {
  background: #0d1117;
  border: 1px solid #30363d;
  border-top: none;
  border-radius: 0 0 8px 8px;
  max-height: calc(100vh - 120px);
  overflow-y: auto;
  padding: 12px 16px;
}
.log-line {
  white-space: pre-wrap;
  word-break: break-all;
  padding: 1px 0;
}
.log-line.error { color: #f85149; }
.log-line.warn { color: #d29922; }
.log-line.info { color: #58a6ff; }
.log-line.owl { color: #a371f7; }
.log-line.grpc { color: #3fb950; }
.log-line.time { color: #8b949e; }
.filter-bar {
  display: flex;
  gap: 6px;
  padding: 8px 16px;
  background: #161b22;
  border: 1px solid #30363d;
  border-top: none;
  flex-wrap: wrap;
}
.filter-bar label {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: #8b949e;
  cursor: pointer;
}
.filter-bar input[type="checkbox"] { accent-color: #d29922; }
#log-content { min-height: 200px; }
.auto-scroll { margin-left: auto; }
</style>
</head>
<body>
<div class="header">
  <h1>🧪 %s</h1>
  <div class="status">
    <span class="status-dot"></span>
    <span id="status-text">Live</span>
  </div>
</div>
<div class="toolbar">
  <button onclick="setRefresh(1000)" id="btn-1s">1s</button>
  <button onclick="setRefresh(3000)" id="btn-3s" class="active">3s</button>
  <button onclick="setRefresh(5000)" id="btn-5s">5s</button>
  <button onclick="setRefresh(10000)" id="btn-10s">10s</button>
  <button onclick="togglePause()" id="btn-pause">⏸ Pause</button>
  <button onclick="clearLog()" id="btn-clear" style="background:#da3633;border-color:#da3633;color:#fff;">🗑 Clear</button>
  <button onclick="scrollToBottom()">⬇ Bottom</button>
  <label class="auto-scroll"><input type="checkbox" id="auto-scroll" checked> Auto-scroll</label>
</div>
<div class="filter-bar">
  <label><input type="checkbox" id="filter-error" checked> Error</label>
  <label><input type="checkbox" id="filter-warn" checked> Warn</label>
  <label><input type="checkbox" id="filter-info" checked> Info</label>
  <label><input type="checkbox" id="filter-owl" checked> OWL</label>
  <label><input type="checkbox" id="filter-grpc" checked> gRPC</label>
  <label><input type="checkbox" id="filter-other" checked> Other</label>
  <input type="text" id="filter-text" placeholder="Filter text..." style="background:#0d1117;border:1px solid #30363d;color:#c9d1d9;padding:2px 8px;border-radius:4px;font-size:11px;flex:1;max-width:300px;">
</div>
<div class="log-container">
  <div id="log-content">Loading...</div>
</div>

<script>
let refreshInterval = 3000;
let timer = null;
let paused = false;
let lastData = '';

function setRefresh(ms) {
  refreshInterval = ms;
  document.querySelectorAll('.toolbar button').forEach(b => b.classList.remove('active'));
  document.getElementById('btn-' + (ms >= 10000 ? '10s' : ms >= 5000 ? '5s' : ms >= 3000 ? '3s' : '1s')).classList.add('active');
  restartTimer();
}

function togglePause() {
  paused = !paused;
  document.getElementById('btn-pause').textContent = paused ? '▶ Resume' : '⏸ Pause';
  document.getElementById('status-text').textContent = paused ? 'Paused' : 'Live';
  document.querySelector('.status-dot').style.background = paused ? '#d29922' : '#3fb950';
}

function restartTimer() {
  if (timer) clearInterval(timer);
  timer = setInterval(fetchLogs, refreshInterval);
}

function clearLog() {
  if (!confirm('Clear all dev server logs?')) return;
  const btn = document.getElementById('btn-clear');
  btn.textContent = '⏳ Clearing...';
  btn.disabled = true;
  fetch('/server-logs-dev/clear', {method: 'POST'})
    .then(() => {
      document.getElementById('log-content').innerHTML = '<div class="log-line info">— Logs cleared —</div>';
      lastData = '';
      btn.textContent = '🗑 Clear';
      btn.disabled = false;
    })
    .catch(() => {
      btn.textContent = '🗑 Clear';
      btn.disabled = false;
    });
}

function scrollToBottom() {
  const c = document.querySelector('.log-container');
  c.scrollTop = c.scrollHeight;
}

function classifyLine(line) {
  const l = line.toLowerCase();
  if (l.includes('error') || l.includes('fatal') || l.includes('panic')) return 'error';
  if (l.includes('warn') || l.includes('warning')) return 'warn';
  if (l.includes('owl') || l.includes('openrouter') || l.includes('ai ')) return 'owl';
  if (l.includes('grpc') || l.includes('stream') || l.includes('rpc')) return 'grpc';
  if (l.includes('info') || l.includes('listening') || l.includes('started') || l.includes('connected')) return 'info';
  return 'other';
}

function shouldShow(line) {
  const cls = classifyLine(line);
  const text = document.getElementById('filter-text').value.toLowerCase();
  if (text && !line.toLowerCase().includes(text)) return false;
  switch(cls) {
    case 'error': return document.getElementById('filter-error').checked;
    case 'warn': return document.getElementById('filter-warn').checked;
    case 'info': return document.getElementById('filter-info').checked;
    case 'owl': return document.getElementById('filter-owl').checked;
    case 'grpc': return document.getElementById('filter-grpc').checked;
    default: return document.getElementById('filter-other').checked;
  }
}

async function fetchLogs() {
  if (paused) return;
  try {
    const resp = await fetch('/server-logs-dev/raw?t=' + Date.now());
    const text = await resp.text();
    if (text === lastData) return;
    lastData = text;
    const lines = text.split('\n').filter(l => l.trim());
    const content = document.getElementById('log-content');
    content.innerHTML = '';
    lines.forEach(line => {
      const div = document.createElement('div');
      div.className = 'log-line ' + classifyLine(line);
      div.textContent = line;
      if (shouldShow(line)) content.appendChild(div);
    });
    if (document.getElementById('auto-scroll').checked) scrollToBottom();
  } catch(e) { console.error('fetch error:', e); }
}

fetchLogs();
restartTimer();
</script>
</body>
</html>`, title, title)
	})

	// Raw log endpoint — last 100 lines
	http.HandleFunc("/server-logs-dev/raw", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")

		cmd := exec.Command("journalctl", "-u", serviceName, "--no-pager", "-n", "100", "--output=short-iso")
		out, err := cmd.Output()
		if err != nil || len(out) == 0 {
			fmt.Fprintf(w, "No logs for service: %s\n", serviceName)
			return
		}
		w.Write(out)
	})

	// Clear logs endpoint for dev — rotates journal and restarts log monitor
	http.HandleFunc("/server-logs-dev/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Flush stdout/stderr of the service
		exec.Command("systemctl", "kill", "-s", "USR1", serviceName).Run()
		// Rotate journal
		exec.Command("journalctl", "--rotate").Run()
		// Vacuum all but last 1MB
		exec.Command("journalctl", "--vacuum-size=1M").Run()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Health check
	http.HandleFunc("/server-logs-dev/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"%s","time":"%s"}`, serviceName, time.Now().Format(time.RFC3339))
	})

	log.Printf("Dev log monitor starting on port %s (service: %s)", port, serviceName)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
