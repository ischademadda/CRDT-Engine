package main

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>CRDT-Engine Real-Time Editor</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
<style>
  :root {
    --bg-base: #090d16;
    --bg-surface: rgba(17, 24, 39, 0.7);
    --bg-card: rgba(31, 41, 55, 0.5);
    --border-color: rgba(255, 255, 255, 0.08);
    --text-primary: #f3f4f6;
    --text-secondary: #9ca3af;
    --accent: #3b82f6;
    --accent-glow: rgba(59, 130, 246, 0.2);
    --success: #10b981;
    --success-glow: rgba(16, 185, 129, 0.15);
    --danger: #ef4444;
    --danger-glow: rgba(239, 68, 68, 0.15);
  }

  * { box-sizing: border-box; margin: 0; padding: 0; }

  body {
    background-color: var(--bg-base);
    background-image: 
      radial-gradient(at 0% 0%, rgba(59, 130, 246, 0.1) 0px, transparent 50%),
      radial-gradient(at 100% 100%, rgba(16, 185, 129, 0.05) 0px, transparent 50%);
    background-attachment: fixed;
    color: var(--text-primary);
    font-family: 'Inter', -apple-system, sans-serif;
    min-height: 100vh;
    padding: 32px;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .container {
    width: 100%;
    max-width: 1200px;
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 20px;
  }

  h1 {
    font-size: 24px;
    font-weight: 700;
    letter-spacing: -0.025em;
    background: linear-gradient(to right, #3b82f6, #10b981);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .meta {
    color: var(--text-secondary);
    font-size: 13px;
    max-width: 600px;
    line-height: 1.5;
  }

  .main-layout {
    display: grid;
    grid-template-columns: 1fr 360px;
    gap: 24px;
  }

  @media (max-width: 900px) {
    .main-layout { grid-template-columns: 1fr; }
  }

  .panel {
    background: var(--bg-surface);
    backdrop-filter: blur(12px);
    border: 1px solid var(--border-color);
    border-radius: 16px;
    padding: 24px;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.25);
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .controls-row {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    align-items: center;
  }

  .form-group {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    font-weight: 500;
    color: var(--text-secondary);
  }

  input {
    background: rgba(17, 24, 39, 0.9);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    color: var(--text-primary);
    padding: 8px 12px;
    font-size: 14px;
    font-family: inherit;
    transition: all 0.2s;
  }

  input:focus {
    border-color: var(--accent);
    outline: none;
    box-shadow: 0 0 0 3px var(--accent-glow);
  }

  button {
    background: var(--accent);
    color: white;
    border: none;
    border-radius: 8px;
    padding: 8px 16px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  button:hover {
    background: #2563eb;
    transform: translateY(-1px);
  }

  button:active {
    transform: translateY(1px);
  }

  .status-badge {
    font-size: 12px;
    font-weight: 600;
    padding: 4px 10px;
    border-radius: 9999px;
    background: rgba(239, 68, 68, 0.1);
    color: var(--danger);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    transition: all 0.2s;
  }

  .status-badge.ok {
    background: rgba(16, 185, 129, 0.1);
    color: var(--success);
  }

  textarea {
    width: 100%;
    height: 400px;
    background: rgba(0, 0, 0, 0.3);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    color: var(--text-primary);
    font-family: 'JetBrains Mono', monospace;
    font-size: 15px;
    line-height: 1.6;
    padding: 16px;
    resize: none;
    transition: border-color 0.2s;
  }

  textarea:focus {
    border-color: rgba(59, 130, 246, 0.5);
    outline: none;
  }

  textarea:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Analytics Dashboard Styles */
  .dashboard-title {
    font-size: 16px;
    font-weight: 700;
    display: flex;
    align-items: center;
    gap: 8px;
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 12px;
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
  }

  .stat-card {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    text-align: center;
    transition: transform 0.2s;
  }

  .stat-card:hover {
    transform: translateY(-2px);
  }

  .stat-val {
    font-size: 20px;
    font-weight: 700;
    font-family: 'JetBrains Mono', monospace;
  }

  .stat-val.chars { color: #60a5fa; }
  .stat-val.inserts { color: #34d399; }
  .stat-val.deletes { color: #f87171; }

  .stat-lbl {
    font-size: 10px;
    color: var(--text-secondary);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .leaders-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: 8px;
  }

  .section-subtitle {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-secondary);
    letter-spacing: 0.05em;
  }

  .leader-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 240px;
    overflow-y: auto;
    padding-right: 4px;
  }

  /* Custom scrollbar for list */
  .leader-list::-webkit-scrollbar { width: 4px; }
  .leader-list::-webkit-scrollbar-track { background: transparent; }
  .leader-list::-webkit-scrollbar-thumb { background: rgba(255, 255, 255, 0.1); border-radius: 2px; }

  .leader-card {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 10px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    transition: all 0.2s;
  }

  .leader-card:hover {
    background: rgba(255, 255, 255, 0.04);
  }

  .leader-info {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 12px;
  }

  .leader-name {
    font-family: 'JetBrains Mono', monospace;
    font-weight: 500;
    color: #e5e7eb;
    max-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .avatar-badge {
    width: 20px;
    height: 20px;
    border-radius: 50%;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 9px;
    font-weight: 700;
    color: white;
  }

  .leader-counts {
    display: flex;
    gap: 8px;
    font-size: 11px;
    font-weight: 600;
  }

  .count-inserts { color: #34d399; }
  .count-deletes { color: #f87171; }

  .bar-container {
    height: 6px;
    width: 100%;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 3px;
    overflow: hidden;
    display: flex;
  }

  .bar-ins { background: #10b981; height: 100%; }
  .bar-del { background: #ef4444; height: 100%; }

  .analytics-offline {
    color: var(--text-secondary);
    font-size: 12px;
    text-align: center;
    padding: 20px 0;
    border: 1px dashed var(--border-color);
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .analytics-offline span {
    font-weight: 600;
    color: var(--danger);
  }

  /* Time Travel panel styles */
  .time-travel-panel {
    grid-column: span 2;
    display: flex;
    flex-direction: column;
    gap: 16px;
    margin-top: 12px;
  }

  .time-travel-layout {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  @media (max-width: 900px) {
    .time-travel-panel { grid-column: span 1; }
    .time-travel-layout { grid-template-columns: 1fr; }
  }

  .revision-log {
    height: 250px;
    overflow-y: auto;
    border: 1px solid var(--border-color);
    border-radius: 12px;
    padding: 12px;
    background: rgba(0, 0, 0, 0.2);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .revision-log::-webkit-scrollbar { width: 4px; }
  .revision-log::-webkit-scrollbar-track { background: transparent; }
  .revision-log::-webkit-scrollbar-thumb { background: rgba(255, 255, 255, 0.1); border-radius: 2px; }

  .revision-item {
    font-size: 13px;
    padding: 8px;
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border-color);
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
    transition: all 0.2s;
  }

  .revision-item:hover {
    background: rgba(255, 255, 255, 0.05);
    border-color: var(--accent);
  }

  .revision-item.selected {
    background: rgba(59, 130, 246, 0.1);
    border-color: var(--accent);
  }

  .revision-meta {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .revision-actor {
    font-family: 'JetBrains Mono', monospace;
    font-weight: 600;
    color: var(--accent);
  }

  .revision-details {
    color: var(--text-primary);
  }

  .revision-time {
    font-size: 11px;
    color: var(--text-secondary);
  }

  .preview-textarea {
    width: 100%;
    height: 250px;
    background: rgba(0, 0, 0, 0.4);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    color: var(--text-secondary);
    font-family: 'JetBrains Mono', monospace;
    font-size: 14px;
    line-height: 1.6;
    padding: 12px;
    resize: none;
  }

  /* --- AUTH / RBAC REALM --- */
  .auth-overlay {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(6, 9, 18, 0.94);
    backdrop-filter: blur(16px);
    z-index: 9999;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.3s ease;
  }
  .auth-card {
    background: rgba(17, 24, 39, 0.8);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 20px;
    padding: 32px;
    width: 100%;
    max-width: 400px;
    box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    gap: 16px;
    position: relative;
  }
  .auth-title {
    font-size: 22px;
    font-weight: 700;
    text-align: center;
    background: linear-gradient(to right, #3b82f6, #10b981);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }
  .auth-subtitle {
    font-size: 12px;
    color: var(--text-secondary);
    text-align: center;
    margin-bottom: 8px;
    line-height: 1.4;
  }
  .auth-field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .auth-field label {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .auth-field select {
    background: rgba(9, 13, 22, 0.9);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    color: var(--text-primary);
    padding: 10px;
    font-size: 14px;
    font-family: inherit;
    outline: none;
    cursor: pointer;
  }
  .auth-field select:focus {
    border-color: var(--accent);
  }
  .auth-btn {
    background: linear-gradient(to right, #3b82f6, #10b981);
    border: none;
    border-radius: 8px;
    padding: 12px;
    font-size: 14px;
    font-weight: 600;
    color: white;
    cursor: pointer;
    transition: all 0.2s;
    margin-top: 8px;
  }
  .auth-btn:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
  }
  
  .session-info {
    display: flex;
    align-items: center;
    gap: 12px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid var(--border-color);
    padding: 6px 12px;
    border-radius: 10px;
  }
  .jwt-badge {
    background: rgba(139, 92, 246, 0.15);
    color: #a78bfa;
    border: 1px solid rgba(139, 92, 246, 0.3);
    padding: 3px 8px;
    border-radius: 6px;
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    cursor: pointer;
    transition: all 0.2s;
  }
  .jwt-badge:hover {
    background: rgba(139, 92, 246, 0.3);
    transform: translateY(-1px);
  }
  
  .jwt-modal {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(4, 6, 12, 0.85);
    backdrop-filter: blur(8px);
    z-index: 10000;
    display: none;
    align-items: center;
    justify-content: center;
  }
  .jwt-modal-card {
    background: #0f172a;
    border: 1px solid var(--border-color);
    border-radius: 16px;
    padding: 24px;
    width: 100%;
    max-width: 550px;
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.6);
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .jwt-part {
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    padding: 10px;
    border-radius: 8px;
    background: rgba(0, 0, 0, 0.4);
    border: 1px solid var(--border-color);
    word-break: break-all;
    line-height: 1.4;
  }
  .jwt-header { color: #f87171; }
  .jwt-payload { color: #60a5fa; }
  .jwt-signature { color: #34d399; }

  .forbidden-banner {
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.25);
    color: #fca5a5;
    border-radius: 10px;
    padding: 10px 14px;
    font-size: 13px;
    font-weight: 500;
    display: none;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
  }
</style>
</head>
<body>

<!-- AUTH OVERLAY -->
<div id="auth-overlay" class="auth-overlay">
  <div class="auth-card">
    <div class="auth-title">🔐 Вход в CRDT-Engine</div>
    <div class="auth-subtitle">Академическая демонстрация ролевого доступа (RBAC) и JWT-авторизации для защиты документов.</div>
    
    <div class="auth-field">
      <label>Имя пользователя (Replica ID):</label>
      <input id="auth-username" value="Ivan" placeholder="Например: Ivan, Maria" />
    </div>

    <div class="auth-field">
      <label>Роль доступа (RBAC):</label>
      <select id="auth-role">
        <option value="editor">Editor (Полный доступ на запись)</option>
        <option value="viewer">Viewer (Только чтение)</option>
      </select>
    </div>

    <div class="auth-field">
      <label>Пароль:</label>
      <input type="password" id="auth-password" value="admin" placeholder="Любой пароль" />
    </div>

    <button id="auth-submit-btn" class="auth-btn">Войти в систему</button>
  </div>
</div>

<!-- JWT DETAILED MODAL -->
<div id="jwt-modal" class="jwt-modal">
  <div class="jwt-modal-card">
    <div class="dashboard-title" style="justify-content: space-between;">
      <span style="display:flex; align-items:center; gap:8px;">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
        Дешифровка JWT Токена
      </span>
      <button id="jwt-close-btn" style="background:transparent; border:none; color:var(--text-secondary); font-size:20px; cursor:pointer;">&times;</button>
    </div>
    
    <div style="font-size: 13px; color: var(--text-secondary);">
      Сгенерированный JWT токен сессии B2B Enterprise (подписан алгоритмом HS256):
    </div>
    
    <div class="jwt-part" style="color: var(--text-primary); font-weight: 500;">
      <span class="jwt-header" id="jwt-raw-header"></span>.<span class="jwt-payload" id="jwt-raw-payload"></span>.<span class="jwt-signature">mock_signature_crdt_engine_auth_service</span>
    </div>

    <div style="display:flex; flex-direction:column; gap:8px;">
      <span class="section-subtitle">Заголовок (JWT Header)</span>
      <pre class="jwt-part jwt-header" id="jwt-decoded-header"></pre>
    </div>

    <div style="display:flex; flex-direction:column; gap:8px;">
      <span class="section-subtitle">Полезная нагрузка (JWT Payload / Claims)</span>
      <pre class="jwt-part jwt-payload" id="jwt-decoded-payload"></pre>
    </div>

    <div style="display:flex; flex-direction:column; gap:8px;">
      <span class="section-subtitle">Подпись (JWT Signature)</span>
      <pre class="jwt-part jwt-signature">HMACSHA256(
  base64UrlEncode(header) + "." +
  base64UrlEncode(payload),
  "crdt_secret_key_university_thesis"
)</pre>
    </div>
  </div>
</div>

<div class="container">
  <header>
    <div>
      <h1>CRDT-Engine <span style="font-size:12px; font-weight:normal; color:var(--accent); vertical-align:middle; background:rgba(59,130,246,0.1); padding:2px 8px; border-radius:4px; margin-left:8px;">v0.4.5 (RBAC Active)</span></h1>
      <div class="meta">Алгоритм Fugue гарантирует отсутствие посимвольного переплетения при одновременной печати.</div>
    </div>
    <div class="controls-row">
      <!-- Session Info Display -->
      <div class="session-info" id="session-info" style="display: none;">
        <span class="jwt-badge" id="jwt-badge">JWT Active</span>
        <span style="font-size: 13px; color: var(--text-secondary);">
          <strong id="session-user" style="color: var(--text-primary);"></strong>
          (<span id="session-role" style="font-weight:600;"></span>)
        </span>
        <button id="logout-btn" style="background:transparent; color:var(--danger); border:none; padding:0; cursor:pointer; font-size:12px; font-weight:600; margin-left:4px;">Выйти</button>
      </div>

      <div class="form-group" style="display: none;">
        <span>документ:</span>
        <input id="doc" value="demo" style="width: 100px;" />
      </div>
      <button id="connect" style="display: none;">Подключиться</button>
      <span id="status" class="status-badge">Отключен</span>
    </div>
  </header>

  <!-- FORBIDDEN BANNER -->
  <div id="forbidden-banner" class="forbidden-banner">
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="color: var(--danger);"><polygon points="7.86 2 16.14 2 22 7.86 22 16.14 16.14 22 7.86 22 2 16.14 2 7.86 7.86 2"></polygon><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line></svg>
    <span><strong>Доступ ограничен:</strong> У вас роль Viewer. Изменения заблокированы сервером по политике безопасности RBAC.</span>
  </div>

  <div class="main-layout">
    <!-- Левая панель: Редактор -->
    <div class="panel">
      <textarea id="editor" placeholder="Сначала подключитесь к документу..." disabled></textarea>
    </div>

    <!-- Правая панель: Аналитика -->
    <div class="panel">
      <div class="dashboard-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="color: var(--accent);"><line x1="18" y1="20" x2="18" y2="10"></line><line x1="12" y1="20" x2="12" y2="4"></line><line x1="6" y1="20" x2="6" y2="14"></line></svg>
        Аналитика Документа
      </div>

      <!-- Live Dashboard -->
      <div id="analytics-content" style="display: none; flex-direction: column; gap: 16px;">
        <div class="stats-grid">
          <div class="stat-card">
            <span class="stat-val chars" id="stat-chars">0</span>
            <span class="stat-lbl">Символов</span>
          </div>
          <div class="stat-card">
            <span class="stat-val inserts" id="stat-inserts">0</span>
            <span class="stat-lbl">Вставок</span>
          </div>
          <div class="stat-card">
            <span class="stat-val deletes" id="stat-deletes">0</span>
            <span class="stat-lbl">Удалений</span>
          </div>
        </div>

        <div class="leaders-section">
          <span class="section-subtitle">Рейтинг участников</span>
          <div class="leader-list" id="leader-list">
            <!-- Сюда вставляется рейтинг -->
          </div>
        </div>
      </div>

      <!-- Offline placeholder -->
      <div id="analytics-offline" class="analytics-offline">
        <span>Аналитика недоступна</span>
        Подключитесь к документу для сбора метрик.
      </div>
    </div>

    <!-- Панель Time Travel & История изменений -->
    <div class="panel time-travel-panel" id="time-travel-panel" style="display: none;">
      <div class="dashboard-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="color: var(--accent);"><circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline></svg>
        История изменений & Time Travel (Event Sourcing)
      </div>
      <div>
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
          <span id="slider-label" style="font-size: 14px; font-weight: 600; color: var(--text-secondary);">Ревизия: 0 из 0</span>
          <button id="checkout-btn" style="padding: 6px 12px; font-size: 12px;" disabled>Откатить редактор к этой версии</button>
        </div>
        <input type="range" id="time-slider" min="0" max="0" value="0" style="width: 100%; cursor: pointer;" />
      </div>
      <div class="time-travel-layout">
        <div>
          <span class="section-subtitle">Лог событий (Append-Only Event Store)</span>
          <div id="revision-log" class="revision-log" style="margin-top: 8px;">
            <!-- Сюда динамически рендерится история -->
          </div>
        </div>
        <div>
          <span class="section-subtitle">Просмотр исторического состояния</span>
          <textarea id="history-preview" class="preview-textarea" style="margin-top: 8px;" placeholder="Переместите ползунок слайдера, чтобы увидеть состояние документа в этой точке времени..." readonly></textarea>
        </div>
      </div>
    </div>

  </div>
</div>

<script>
(function () {
  const $doc      = document.getElementById('doc');
  const $editor   = document.getElementById('editor');
  const $status   = document.getElementById('status');
  const $connect  = document.getElementById('connect');
  const $analyticsContent = document.getElementById('analytics-content');
  const $analyticsOffline = document.getElementById('analytics-offline');
  const $leaderList       = document.getElementById('leader-list');
  const $timeTravelPanel  = document.getElementById('time-travel-panel');
  const $timeSlider       = document.getElementById('time-slider');
  const $sliderLabel      = document.getElementById('slider-label');
  const $revisionLog      = document.getElementById('revision-log');
  const $historyPreview   = document.getElementById('history-preview');
  const $checkoutBtn      = document.getElementById('checkout-btn');

  let ws = null;
  let suppressInput = false;
  let lastValue = '';
  let lastSyncedValue = '';
  let pollInterval = null;
  let historyPollInterval = null;
  let revisionsList = [];

  function setStatus(text, cls) {
    $status.textContent = text;
    $status.className = 'status-badge ' + (cls || '');
  }

  function diff(prev, next) {
    if (next.length === prev.length + 1) {
      for (let i = 0; i < next.length; i++) {
        if (i === prev.length || next[i] !== prev[i]) {
          return { kind: 'insert', pos: i, char: next[i] };
        }
      }
    } else if (next.length === prev.length - 1) {
      for (let i = 0; i < prev.length; i++) {
        if (i === next.length || next[i] !== prev[i]) {
          return { kind: 'delete', pos: i };
        }
      }
    }
    return null;
  }

  function getSingleChange(before, after) {
    if (before === after) return null;
    
    let prefixLen = 0;
    while (prefixLen < before.length && prefixLen < after.length && before[prefixLen] === after[prefixLen]) {
      prefixLen++;
    }
    
    let suffixLen = 0;
    while (suffixLen < before.length - prefixLen && suffixLen < after.length - prefixLen && 
           before[before.length - 1 - suffixLen] === after[after.length - 1 - suffixLen]) {
      suffixLen++;
    }
    
    const added = after.slice(prefixLen, after.length - suffixLen);
    const removed = before.slice(prefixLen, before.length - suffixLen);
    
    return {
      index: prefixLen,
      added: added,
      removed: removed
    };
  }

  async function refreshSnapshot(doc) {
    const r = await fetch('/snapshot?doc=' + encodeURIComponent(doc));
    if (!r.ok) return;
    const j = await r.json();
    const newText = j.text || '';
    
    const oldText = lastSyncedValue;
    const currentText = $editor.value;
    
    const remote = getSingleChange(oldText, newText);
    lastSyncedValue = newText;
    
    if (!remote) {
      // Никаких изменений на сервере нет, или они идентичны текущему состоянию
      return;
    }
    
    let mergedText = currentText;
    const start = $editor.selectionStart;
    const end = $editor.selectionEnd;
    
    let newStart = start;
    let newEnd = end;
    
    // Применение удаленной вставки
    if (remote.added.length > 0) {
      const insertIndex = remote.index;
      mergedText = currentText.slice(0, insertIndex) + remote.added + currentText.slice(insertIndex);
      
      if (insertIndex <= start) {
        newStart += remote.added.length;
      }
      if (insertIndex <= end) {
        newEnd += remote.added.length;
      }
    }
    
    // Применение удаленного удаления
    if (remote.removed.length > 0) {
      const deleteIndex = remote.index;
      const deleteLen = remote.removed.length;
      mergedText = currentText.slice(0, deleteIndex) + currentText.slice(deleteIndex + deleteLen);
      
      if (deleteIndex < start) {
        newStart -= Math.min(deleteLen, start - deleteIndex);
      }
      if (deleteIndex < end) {
        newEnd -= Math.min(deleteLen, end - deleteIndex);
      }
    }
    
    suppressInput = true;
    $editor.value = mergedText;
    lastValue = mergedText;
    $editor.setSelectionRange(newStart, newEnd);
    suppressInput = false;
  }

  function applyRemote(type, payload) {
    const doc = $doc.value;
    refreshSnapshot(doc);
  }

  // Генерация аватара на основе хэша имени реплики
  function getAvatarStyle(name) {
    let hash = 0;
    for (let i = 0; i < name.length; i++) {
      hash = name.charCodeAt(i) + ((hash << 5) - hash);
    }
    const h = Math.abs(hash) % 360;
    return "background: hsl(" + h + ", 70%, 45%);";
  }

  // Обновление дашборда на основе данных REST API
  function renderAnalytics(data) {
    $analyticsOffline.style.display = 'none';
    $analyticsContent.style.display = 'flex';

    document.getElementById('stat-chars').textContent = data.total_chars || 0;
    document.getElementById('stat-inserts').textContent = data.total_inserts || 0;
    document.getElementById('stat-deletes').textContent = data.total_deletes || 0;

    $leaderList.innerHTML = '';
    if (!data.replicas || data.replicas.length === 0) {
      $leaderList.innerHTML = '<div style="font-size: 12px; color: var(--text-secondary); text-align: center; padding: 12px;">Пока нет активности</div>';
      return;
    }

    data.replicas.forEach(rep => {
      const card = document.createElement('div');
      card.className = 'leader-card';

      const total = rep.inserts + rep.deletes || 1;
      const insPct = (rep.inserts / total) * 100;
      const delPct = (rep.deletes / total) * 100;

      const letter = rep.replica_id.substring(0, 2).toUpperCase();
      const style = getAvatarStyle(rep.replica_id);

      card.innerHTML = 
        '<div class="leader-info">' +
          '<span class="leader-name" title="' + rep.replica_id + '">' +
            '<span class="avatar-badge" style="' + style + '">' + letter + '</span>' +
            rep.replica_id +
          '</span>' +
          '<div class="leader-counts">' +
            '<span class="count-inserts">+' + rep.inserts + '</span>' +
            '<span class="count-deletes">-' + rep.deletes + '</span>' +
          '</div>' +
        '</div>' +
        '<div class="bar-container">' +
          '<div class="bar-ins" style="width: ' + insPct + '%"></div>' +
          '<div class="bar-del" style="width: ' + delPct + '%"></div>' +
        '</div>';
      $leaderList.appendChild(card);
    });
  }

  // Опрос API аналитики
  function startAnalyticsPolling(doc) {
    if (pollInterval) clearInterval(pollInterval);
    
    const poll = () => {
      fetch('http://localhost:8082/api/analytics/' + encodeURIComponent(doc))
        .then(r => {
          if (!r.ok) throw new Error();
          return r.json();
        })
        .then(data => {
          renderAnalytics(data);
        })
        .catch(() => {
          $analyticsContent.style.display = 'none';
          $analyticsOffline.style.display = 'block';
          $analyticsOffline.innerHTML = '<span>Аналитика offline</span>Сервер аналитики недоступен на порту 8082.';
        });
    };

    poll();
    pollInterval = setInterval(poll, 2000);
  }

  function stopAnalyticsPolling() {
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
    $analyticsContent.style.display = 'none';
    $analyticsOffline.style.display = 'block';
    $analyticsOffline.innerHTML = '<span>Аналитика отключена</span>Подключитесь к документу для сбора метрик.';
  }

  function startHistoryPolling(doc) {
    if (historyPollInterval) clearInterval(historyPollInterval);
    
    const poll = () => {
      fetch('http://localhost:8083/api/history/' + encodeURIComponent(doc) + '/revisions')
        .then(r => {
          if (!r.ok) throw new Error();
          return r.json();
        })
        .then(data => {
          revisionsList = data || [];
          $timeTravelPanel.style.display = 'flex';
          updateTimeSlider();
        })
        .catch(() => {
          $timeTravelPanel.style.display = 'none';
        });
    };
    
    poll();
    historyPollInterval = setInterval(poll, 2000);
  }

  function stopHistoryPolling() {
    if (historyPollInterval) {
      clearInterval(historyPollInterval);
      historyPollInterval = null;
    }
    $timeTravelPanel.style.display = 'none';
    revisionsList = [];
  }

  function updateTimeSlider() {
    const total = revisionsList.length;
    const isDragging = document.activeElement === $timeSlider;
    
    if (!isDragging) {
      $timeSlider.max = total;
      if ($timeSlider.value == 0 || $timeSlider.value == total - 1 || $timeSlider.value == total) {
        $timeSlider.value = total;
      }
    }
    
    updateSliderUI();
  }

  async function updateSliderUI() {
    const currentVal = parseInt($timeSlider.value, 10);
    const total = revisionsList.length;
    
    $sliderLabel.textContent = 'Ревизия: ' + currentVal + ' из ' + total;
    $timeSlider.disabled = total === 0;
    
    renderRevisionLog(currentVal);
    
    if (currentVal === 0) {
      $historyPreview.value = '';
      $checkoutBtn.disabled = true;
    } else if (currentVal === total) {
      $historyPreview.value = $editor.value;
      $checkoutBtn.disabled = true;
    } else {
      $checkoutBtn.disabled = false;
      try {
        const doc = $doc.value || 'demo';
        const r = await fetch('http://localhost:8083/api/history/' + encodeURIComponent(doc) + '/checkout?version=' + currentVal);
        if (r.ok) {
          const j = await r.json();
          $historyPreview.value = j.text || '';
        }
      } catch(_) {}
    }
  }

  function renderRevisionLog(selectedVersion) {
    $revisionLog.innerHTML = '';
    if (revisionsList.length === 0) {
      $revisionLog.innerHTML = '<div style="font-size: 12px; color: var(--text-secondary); text-align: center; padding: 12px;">Событий пока не зарегистрировано</div>';
      return;
    }
    
    for (let i = revisionsList.length - 1; i >= 0; i--) {
      const rev = revisionsList[i];
      const revNum = i + 1;
      
      const card = document.createElement('div');
      card.className = 'revision-item' + (revNum === selectedVersion ? ' selected' : '');
      card.dataset.version = revNum;
      
      let payload = {};
      try {
        payload = JSON.parse(rev.payload);
      } catch(_) {
        payload = rev.payload || {};
      }
      
      let actor = 'system';
      let details = '';
      
      if (rev.op_type === 'fugue_insert') {
        actor = payload.NodeID ? payload.NodeID.ReplicaID : 'unknown';
        const char = String.fromCodePoint(payload.Value || 32);
        const charDisp = char === '\n' ? '↵ (Enter)' : char === ' ' ? '␣ (Space)' : '"' + char + '"';
        details = 'вставил символ ' + charDisp;
      } else if (rev.op_type === 'fugue_delete') {
        actor = payload.SourceID ? payload.SourceID.ReplicaID : 'unknown';
        details = 'удалил символ';
      }
      
      const timeStr = new Date(rev.created_at).toLocaleTimeString();
      
      card.innerHTML = 
        '<div class="revision-meta">' +
          '<span class="avatar-badge" style="width:16px; height:16px; font-size:7px; ' + getAvatarStyle(actor) + '">' + actor.substring(0, 2).toUpperCase() + '</span>' +
          '<span class="revision-actor">' + actor + '</span>' +
          '<span class="revision-details">' + details + '</span>' +
        '</div>' +
        '<span class="revision-time">#' + revNum + ' @ ' + timeStr + '</span>';
        
      card.addEventListener('click', () => {
        $timeSlider.value = revNum;
        updateSliderUI();
      });
      
      $revisionLog.appendChild(card);
    }
  }

  const $authOverlay = document.getElementById('auth-overlay');
  const $authUsername = document.getElementById('auth-username');
  const $authRole = document.getElementById('auth-role');
  const $authPassword = document.getElementById('auth-password');
  const $authSubmitBtn = document.getElementById('auth-submit-btn');

  const $sessionInfo = document.getElementById('session-info');
  const $sessionUser = document.getElementById('session-user');
  const $sessionRole = document.getElementById('session-role');
  const $logoutBtn = document.getElementById('logout-btn');
  const $jwtBadge = document.getElementById('jwt-badge');

  const $jwtModal = document.getElementById('jwt-modal');
  const $jwtCloseBtn = document.getElementById('jwt-close-btn');
  const $jwtRawHeader = document.getElementById('jwt-raw-header');
  const $jwtRawPayload = document.getElementById('jwt-raw-payload');
  const $jwtDecodedHeader = document.getElementById('jwt-decoded-header');
  const $jwtDecodedPayload = document.getElementById('jwt-decoded-payload');

  const $forbiddenBanner = document.getElementById('forbidden-banner');

  let currentUser = '';
  let currentRole = '';
  let currentToken = '';

  function generateMockJWT(username, role) {
    const headerObj = { alg: "HS256", typ: "JWT" };
    const payloadObj = {
      sub: username,
      role: role,
      iss: "crdt-auth-service",
      exp: Math.floor(Date.now() / 1000) + 3600
    };
    
    // Clean base64 helper (standard JWT format doesn't have '=')
    const cleanB64 = (str) => btoa(unescape(encodeURIComponent(str))).replace(/=/g, '');
    
    const rawHeader = cleanB64(JSON.stringify(headerObj));
    const rawPayload = cleanB64(JSON.stringify(payloadObj));
    
    return {
      token: rawHeader + '.' + rawPayload + '.mock_signature_crdt_engine_auth_service',
      rawHeader: rawHeader,
      rawPayload: rawPayload,
      headerJson: JSON.stringify(headerObj, null, 2),
      payloadJson: JSON.stringify(payloadObj, null, 2)
    };
  }

  $authSubmitBtn.addEventListener('click', () => {
    const username = $authUsername.value.trim();
    if (!username) {
      alert('Пожалуйста, введите имя пользователя!');
      return;
    }
    
    currentUser = username;
    currentRole = $authRole.value;
    
    const jwtData = generateMockJWT(currentUser, currentRole);
    currentToken = jwtData.token;
    
    // Fill Decrypter Modal contents
    $jwtRawHeader.textContent = jwtData.rawHeader;
    $jwtRawPayload.textContent = jwtData.rawPayload;
    $jwtDecodedHeader.textContent = jwtData.headerJson;
    $jwtDecodedPayload.textContent = jwtData.payloadJson;
    
    // Update Header Display
    $sessionUser.textContent = currentUser;
    $sessionRole.textContent = currentRole === 'editor' ? 'Editor' : 'Viewer';
    $sessionRole.style.color = currentRole === 'editor' ? '#10b981' : '#ef4444';
    
    $authOverlay.style.display = 'none';
    $sessionInfo.style.display = 'flex';
    
    // Connect WebSocket
    connectWS();
  });

  $logoutBtn.addEventListener('click', () => {
    if (ws) {
      ws.close();
      ws = null;
    }
    
    currentUser = '';
    currentRole = '';
    currentToken = '';
    
    lastSyncedValue = '';
    lastValue = '';
    
    $editor.value = '';
    $editor.disabled = true;
    
    $sessionInfo.style.display = 'none';
    $authOverlay.style.display = 'flex';
    $forbiddenBanner.style.display = 'none';
    
    stopAnalyticsPolling();
    stopHistoryPolling();
  });

  $jwtBadge.addEventListener('click', () => {
    $jwtModal.style.display = 'flex';
  });
  
  $jwtCloseBtn.addEventListener('click', () => {
    $jwtModal.style.display = 'none';
  });
  
  window.addEventListener('click', (ev) => {
    if (ev.target === $jwtModal) {
      $jwtModal.style.display = 'none';
    }
  });

  function connectWS() {
    const doc = $doc.value || 'demo';
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const url = proto + '://' + location.host + '/ws?doc=' + encodeURIComponent(doc) + 
                '&username=' + encodeURIComponent(currentUser) + 
                '&role=' + encodeURIComponent(currentRole);
                
    ws = new WebSocket(url);

    ws.addEventListener('open', async () => {
      setStatus('Подключен', 'ok');
      
      if (currentRole === 'viewer') {
        $editor.disabled = true;
        $editor.placeholder = "Режим Чтения (Viewer). Изменения ограничены политикой RBAC.";
        $forbiddenBanner.style.display = 'flex';
      } else {
        $editor.disabled = false;
        $editor.placeholder = "Начните вводить текст документа здесь...";
        $forbiddenBanner.style.display = 'none';
      }
      
      await refreshSnapshot(doc);
      startAnalyticsPolling(doc);
      startHistoryPolling(doc);
    });
    
    ws.addEventListener('close', () => {
      setStatus('Отключен', 'err');
      $editor.disabled = true;
      $forbiddenBanner.style.display = 'none';
      stopAnalyticsPolling();
      stopHistoryPolling();
    });
    
    ws.addEventListener('error', () => {
      setStatus('Ошибка', 'err');
      stopAnalyticsPolling();
      stopHistoryPolling();
    });
    ws.addEventListener('message', (ev) => {
      try {
        const m = JSON.parse(ev.data);
        applyRemote(m.type, m.payload);
      } catch (_) {}
    });
  });

  $editor.addEventListener('input', () => {
    if (suppressInput || !ws || ws.readyState !== 1) {
      lastValue = $editor.value;
      return;
    }
    const next = $editor.value;
    const d = diff(lastValue, next);
    lastValue = next;
    if (!d) return;
    const doc = $doc.value || 'demo';
    if (d.kind === 'insert') {
      ws.send(JSON.stringify({ doc_id: doc, type: 'insert_intent',
        payload: { pos: d.pos, char: d.char } }));
    } else {
      ws.send(JSON.stringify({ doc_id: doc, type: 'delete_intent',
        payload: { pos: d.pos } }));
    }
  });
})();
</script>
</body>
</html>`
