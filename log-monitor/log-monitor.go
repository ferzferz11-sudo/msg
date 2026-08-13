package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Configuration via environment variables
var (
	port        = getEnv("LOG_PORT", "8090")
	serviceName = getEnv("LOG_SERVICE", "lavender-server")
	title       = getEnv("LOG_TITLE", "Lava Server Logs")
	pathPrefix  = getEnv("LOG_PATH_PREFIX", "/server-logs")
	logFile     = getEnv("LOG_FILE", "/root/LavenderMessenger/run/server.log")
	colorScheme = getEnv("LOG_COLOR_SCHEME", "blue") // "blue" for prod, "yellow" for dev
)

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// Color scheme
type colors struct {
	primary   string
	header    string
	statusDot string
	active    string
}

func getColors() colors {
	if colorScheme == "yellow" {
		return colors{
			primary:   "#d29922",
			header:    "#d29922",
			statusDot: "#d29922",
			active:    "#d29922",
		}
	}
	return colors{
		primary:   "#58a6ff",
		header:    "#58a6ff",
		statusDot: "#3fb950",
		active:    "#1f6feb",
	}
}

func main() {
	c := getColors()

	// Determine raw endpoint and clear/raw URLs
	rawPath := pathPrefix + "/raw"
	clearPath := pathPrefix + "/clear"
	healthPath := pathPrefix + "/health"

	// Choose icon based on scheme
	icon := "🖥️"
	if colorScheme == "yellow" {
		icon = "🧪"
	}

	// Main page — auto-refreshing log viewer
	http.HandleFunc(pathPrefix, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")

		// Build clear button HTML
		clearBtn := `<button onclick="clearLog()">Clear</button>`
		clearJS := `
  if (!confirm('Clear all server logs?')) return;
  fetch('` + clearPath + `', {method: 'POST'})
    .then(() => {
      document.getElementById('log-content').innerHTML = '';
      lastData = '';
    });`
		clearConfirm := "Clear all server logs?"

		if colorScheme == "yellow" {
			clearBtn = `<button onclick="clearLog()" id="btn-clear" style="background:#da3633;border-color:#da3633;color:#fff;">🗑 Clear</button>`
			clearConfirm = "Clear all dev server logs?"
			clearJS = `
  if (!confirm('` + clearConfirm + `')) return;
  const btn = document.getElementById('btn-clear');
  btn.textContent = '⏳ Clearing...';
  btn.disabled = true;
  fetch('` + clearPath + `', {method: 'POST'})
    .then(() => {
      document.getElementById('log-content').innerHTML = '<div class="log-line info">— Logs cleared —</div>';
      lastData = '';
      btn.textContent = '🗑 Clear';
      btn.disabled = false;
    })
    .catch(() => {
      btn.textContent = '🗑 Clear';
      btn.disabled = false;
    });`
		}

		html := fmt.Sprintf(`<!DOCTYPE html>
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
  color: %s;
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
  background: %s;
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
.toolbar button.active { background: %s; border-color: %s; %s }
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
.filter-bar input[type="checkbox"] { accent-color: %s; }
#log-content { min-height: 200px; }
.auto-scroll { margin-left: auto; }
</style>
</head>
<body>
<div class="header">
  <h1>%s %s</h1>
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
  %s
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
  `+clearJS+`
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
    const resp = await fetch('`+rawPath+`?t=' + Date.now());
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
</html>`,
			title, c.header, c.statusDot, c.active, c.active,
			func() string {
				if colorScheme == "yellow" {
					return "color: #000;"
				}
				return ""
			}(),
			c.active, icon, title, clearBtn)
		w.Write([]byte(html))
	})

	// Raw log endpoint
	http.HandleFunc(rawPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")

		cmd := exec.Command("journalctl", "-u", serviceName, "--no-pager", "-n", "100", "--output=short-iso")
		out, err := cmd.Output()
		if err != nil || len(out) == 0 {
			// Fallback: try log file (prod only)
			if logFile != "" {
				data, err := os.ReadFile(logFile)
				if err == nil && len(data) > 0 {
					w.Write(data)
					return
				}
			}
			fmt.Fprintf(w, "No logs for service: %s\n", serviceName)
			return
		}
		w.Write(out)
	})

	// Clear logs endpoint
	http.HandleFunc(clearPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if colorScheme == "yellow" {
			// Dev: also signal the service
			exec.Command("systemctl", "kill", "-s", "USR1", serviceName).Run()
			exec.Command("journalctl", "--vacuum-size=1M").Run()
		}
		exec.Command("journalctl", "--rotate").Run()
		exec.Command("journalctl", "--vacuum-time=1s").Run()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Health check
	http.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"%s","time":"%s"}`, serviceName, time.Now().Format(time.RFC3339))
	})

	log.Printf("Log monitor starting on port %s (service: %s, path: %s, color: %s)", port, serviceName, pathPrefix, colorScheme)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Helper to check if a string contains a substring (for template)
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
