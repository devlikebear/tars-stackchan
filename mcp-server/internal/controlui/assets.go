package controlui

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>TARS Stack-chan Control</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f7f5f0;
      --panel: #ffffff;
      --ink: #1d2528;
      --muted: #647070;
      --line: #d9dfdc;
      --accent: #0f766e;
      --accent-strong: #115e59;
      --warn: #b45309;
      --danger: #b91c1c;
      --ok: #15803d;
      --shadow: 0 8px 24px rgba(29, 37, 40, 0.08);
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }

    * { box-sizing: border-box; }

    body {
      margin: 0;
      min-width: 320px;
      background: var(--bg);
      color: var(--ink);
    }

    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      padding: 18px 24px 12px;
      border-bottom: 1px solid var(--line);
      background: rgba(255, 255, 255, 0.82);
      position: sticky;
      top: 0;
      z-index: 2;
      backdrop-filter: blur(14px);
    }

    h1 {
      margin: 0;
      font-size: 22px;
      line-height: 1.15;
      font-weight: 740;
      letter-spacing: 0;
    }

    main {
      display: grid;
      grid-template-columns: minmax(280px, 0.95fr) minmax(340px, 1.45fr);
      gap: 14px;
      padding: 16px 24px 24px;
      max-width: 1180px;
      margin: 0 auto;
    }

    section, .topbar {
      background: var(--panel);
      border: 1px solid var(--line);
      box-shadow: var(--shadow);
      border-radius: 8px;
    }

    .topbar {
      grid-column: 1 / -1;
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 1px;
      overflow: hidden;
    }

    .metric {
      padding: 12px 14px;
      border-right: 1px solid var(--line);
      min-width: 0;
    }

    .metric:last-child { border-right: 0; }

    .label {
      display: block;
      color: var(--muted);
      font-size: 12px;
      font-weight: 680;
      line-height: 1.2;
      margin-bottom: 5px;
    }

    .value {
      display: block;
      min-height: 20px;
      font-size: 14px;
      line-height: 1.35;
      overflow-wrap: anywhere;
    }

    section {
      padding: 16px;
      min-width: 0;
    }

    h2 {
      margin: 0 0 14px;
      font-size: 16px;
      line-height: 1.2;
      letter-spacing: 0;
    }

    .stack { display: grid; gap: 14px; }
    .controls { display: grid; gap: 16px; }

    .button-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(108px, 1fr));
      gap: 8px;
    }

    button, select, input, textarea {
      font: inherit;
      border-radius: 8px;
      border: 1px solid var(--line);
      background: #ffffff;
      color: var(--ink);
    }

    button {
      min-height: 38px;
      padding: 8px 11px;
      cursor: pointer;
      font-weight: 700;
    }

    button.primary {
      background: var(--accent);
      border-color: var(--accent);
      color: #ffffff;
    }

    button:hover { border-color: var(--accent); }
    button.primary:hover { background: var(--accent-strong); }

    .form-grid {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 12px;
    }

    .field {
      display: grid;
      gap: 7px;
      min-width: 0;
    }

    input, select, textarea {
      width: 100%;
      min-height: 38px;
      padding: 8px 10px;
    }

    input[type="range"] {
      padding: 0;
      accent-color: var(--accent);
    }

    input[type="color"] {
      padding: 4px;
      height: 38px;
    }

    textarea {
      min-height: 78px;
      resize: vertical;
    }

    .status-grid {
      display: grid;
      gap: 10px;
    }

    .status-row {
      display: grid;
      grid-template-columns: 100px minmax(0, 1fr);
      gap: 10px;
      align-items: start;
      padding-bottom: 9px;
      border-bottom: 1px solid var(--line);
    }

    .status-row:last-child {
      border-bottom: 0;
      padding-bottom: 0;
    }

    .badge {
      display: inline-flex;
      align-items: center;
      min-height: 24px;
      padding: 3px 9px;
      border-radius: 999px;
      background: #e7f5ef;
      color: var(--ok);
      font-size: 12px;
      font-weight: 780;
    }

    .badge.warn {
      background: #fff7ed;
      color: var(--warn);
    }

    .badge.error {
      background: #fef2f2;
      color: var(--danger);
    }

    pre {
      margin: 0;
      padding: 12px;
      min-height: 170px;
      max-height: 300px;
      overflow: auto;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #fbfcfb;
      color: #243033;
      font-size: 12px;
      line-height: 1.5;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
    }

    .wide { grid-column: 1 / -1; }

    @media (max-width: 820px) {
      header { align-items: flex-start; flex-direction: column; padding: 16px; }
      main { grid-template-columns: 1fr; padding: 12px; }
      .topbar { grid-template-columns: 1fr 1fr; }
      .metric:nth-child(2n) { border-right: 0; }
      .form-grid { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <header>
    <h1>TARS Stack-chan Control</h1>
    <button class="primary" id="refreshStatus">Refresh</button>
  </header>

  <main>
    <div class="topbar">
      <div class="metric">
        <span class="label">Bridge</span>
        <span class="value" id="bridgeMode">-</span>
      </div>
      <div class="metric">
        <span class="label">Base URL</span>
        <span class="value" id="baseUrl">-</span>
      </div>
      <div class="metric">
        <span class="label">Token</span>
        <span class="value" id="tokenState">-</span>
      </div>
      <div class="metric">
        <span class="label">Connection</span>
        <span class="value"><span class="badge warn" id="connectionState">checking</span></span>
      </div>
    </div>

    <section>
      <h2>Status</h2>
      <div class="status-grid">
        <div class="status-row"><span class="label">Device</span><span class="value" id="device">-</span></div>
        <div class="status-row"><span class="label">Firmware</span><span class="value" id="firmware">-</span></div>
        <div class="status-row"><span class="label">IP</span><span class="value" id="ip">-</span></div>
        <div class="status-row"><span class="label">Capabilities</span><span class="value" id="capabilities">-</span></div>
      </div>
    </section>

    <div class="stack">
      <section>
        <h2>Expression</h2>
        <div class="button-grid" id="expressions">
          <button data-emotion="neutral">Neutral</button>
          <button data-emotion="happy">Happy</button>
          <button data-emotion="sad">Sad</button>
          <button data-emotion="angry">Angry</button>
          <button data-emotion="surprised">Surprised</button>
          <button data-emotion="sleepy">Sleepy</button>
          <button data-emotion="blink">Blink</button>
        </div>
      </section>

      <section>
        <h2>Head</h2>
        <div class="form-grid">
          <label class="field"><span class="label">Pan <span id="panValue">0</span></span><input id="pan" type="range" min="-90" max="90" value="0"></label>
          <label class="field"><span class="label">Tilt <span id="tiltValue">45</span></span><input id="tilt" type="range" min="5" max="85" value="45"></label>
          <label class="field"><span class="label">Speed <span id="speedValue">0.6</span></span><input id="speed" type="range" min="0" max="1" step="0.1" value="0.6"></label>
        </div>
        <p><button class="primary" id="moveHead">Move</button></p>
      </section>
    </div>

    <section>
      <h2>LED</h2>
      <div class="form-grid">
        <label class="field"><span class="label">Pattern</span><select id="ledPattern"><option>solid</option><option>blink</option><option>pulse</option><option>off</option></select></label>
        <label class="field"><span class="label">Color</span><input id="ledColor" type="color" value="#00aeef"></label>
        <label class="field"><span class="label">Brightness <span id="brightnessValue">0.5</span></span><input id="brightness" type="range" min="0" max="1" step="0.1" value="0.5"></label>
      </div>
      <p><button class="primary" id="setLed">Set LED</button></p>
    </section>

    <section>
      <h2>Motion</h2>
      <div class="button-grid" id="motions">
        <button data-motion="home">Home</button>
        <button data-motion="nod">Nod</button>
        <button data-motion="shake">Shake</button>
        <button data-motion="look_around">Look Around</button>
      </div>
    </section>

    <section class="wide">
      <h2>Speech</h2>
      <label class="field">
        <span class="label">stackchan_speak</span>
        <textarea id="speechText" maxlength="240">hello stack-chan</textarea>
      </label>
      <p><button class="primary" id="speak">Speak</button></p>
    </section>

    <section class="wide">
      <h2>Log</h2>
      <pre id="log"></pre>
    </section>
  </main>

  <script>
    const $ = (id) => document.getElementById(id);
    const logEl = $('log');

    function log(label, value) {
      const stamp = new Date().toLocaleTimeString();
      const line = '[' + stamp + '] ' + label + '\n' + JSON.stringify(value, null, 2) + '\n\n';
      logEl.textContent = line + logEl.textContent;
    }

    async function request(path, options = {}) {
      const response = await fetch(path, {
        headers: { 'Content-Type': 'application/json' },
        ...options
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw body;
      }
      return body;
    }

    async function loadConfig() {
      const config = await request('/api/config');
      $('bridgeMode').textContent = config.bridge_mode || '-';
      $('baseUrl').textContent = config.base_url || '-';
      $('tokenState').textContent = config.token_set ? 'configured' : 'missing';
      if (!config.token_set) $('tokenState').className = 'badge warn';
    }

    async function refreshStatus() {
      try {
        const status = await request('/api/status');
        $('connectionState').textContent = status.connected ? 'connected' : 'offline';
        $('connectionState').className = 'badge ' + (status.connected ? '' : 'warn');
        $('device').textContent = status.device || '-';
        $('firmware').textContent = status.firmware || '-';
        $('ip').textContent = status.ip || '-';
        $('capabilities').textContent = (status.capabilities || []).join(', ') || '-';
        log('status', status);
      } catch (error) {
        $('connectionState').textContent = 'error';
        $('connectionState').className = 'badge error';
        log('status error', error);
      }
    }

    async function postAction(path, payload) {
      try {
        const result = await request(path, { method: 'POST', body: JSON.stringify(payload) });
        log(path, result);
        refreshStatus();
      } catch (error) {
        log(path + ' error', error);
      }
    }

    function bindRange(id, labelId) {
      const input = $(id);
      const label = $(labelId);
      input.addEventListener('input', () => { label.textContent = input.value; });
    }

    document.querySelectorAll('#expressions button').forEach((button) => {
      button.addEventListener('click', () => postAction('/api/expression', { emotion: button.dataset.emotion }));
    });

    document.querySelectorAll('#motions button').forEach((button) => {
      button.addEventListener('click', () => postAction('/api/motion', { name: button.dataset.motion }));
    });

    $('moveHead').addEventListener('click', () => postAction('/api/head', {
      pan_deg: Number($('pan').value),
      tilt_deg: Number($('tilt').value),
      speed: Number($('speed').value)
    }));

    $('setLed').addEventListener('click', () => postAction('/api/leds', {
      pattern: $('ledPattern').value,
      color: $('ledColor').value.toUpperCase(),
      brightness: Number($('brightness').value)
    }));

    $('speak').addEventListener('click', () => postAction('/api/speech', { text: $('speechText').value }));
    $('refreshStatus').addEventListener('click', refreshStatus);

    bindRange('pan', 'panValue');
    bindRange('tilt', 'tiltValue');
    bindRange('speed', 'speedValue');
    bindRange('brightness', 'brightnessValue');

    loadConfig().then(refreshStatus).catch((error) => log('config error', error));
  </script>
</body>
</html>
`
