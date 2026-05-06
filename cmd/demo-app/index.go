package main

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>CRDT-Engine demo</title>
<style>
  body { font-family: -apple-system, system-ui, sans-serif; margin: 24px; max-width: 720px; }
  h1 { font-size: 18px; margin-bottom: 4px; }
  .meta { color: #666; font-size: 12px; margin-bottom: 16px; }
  textarea { width: 100%; height: 320px; font-family: ui-monospace, Menlo, monospace;
             font-size: 14px; padding: 12px; box-sizing: border-box; }
  .status { font-size: 12px; color: #444; margin-top: 8px; }
  .status.ok  { color: #2a7; }
  .status.err { color: #c33; }
  .row { display: flex; gap: 12px; align-items: center; margin-bottom: 12px; }
  input { padding: 6px 8px; font-size: 14px; }
</style>
</head>
<body>
<h1>CRDT-Engine demo</h1>
<div class="meta">Open this page in two tabs (or two browsers) and edit simultaneously. Concurrent inserts at the same position will not interleave (Fugue).</div>

<div class="row">
  <label>doc: <input id="doc" value="demo" /></label>
  <button id="connect">Connect</button>
  <span id="status" class="status">disconnected</span>
</div>

<textarea id="editor" placeholder="connect first..." disabled></textarea>

<script>
(function () {
  const $doc      = document.getElementById('doc');
  const $editor   = document.getElementById('editor');
  const $status   = document.getElementById('status');
  const $connect  = document.getElementById('connect');

  let ws = null;
  let suppressInput = false;
  let lastValue = '';

  function setStatus(text, cls) {
    $status.textContent = text;
    $status.className = 'status ' + (cls || '');
  }

  // Diff strategy: the textarea is a single source of local truth. On every
  // 'input' we compute a single-char diff against lastValue and emit one
  // intent. This is intentionally simple — Fugue's correctness does not
  // depend on the granularity of intents, only on the resulting ops.
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

  async function refreshSnapshot(doc) {
    const r = await fetch('/snapshot?doc=' + encodeURIComponent(doc));
    if (!r.ok) return;
    const j = await r.json();
    suppressInput = true;
    $editor.value = j.text || '';
    lastValue = $editor.value;
    suppressInput = false;
  }

  function applyRemote(type, payload) {
    // The remote payload is a Fugue op: we don't have a tree on the client,
    // so we simply re-pull the snapshot. In a production client you'd keep
    // a local CRDT replica; that's out of scope for this demo.
    const doc = $doc.value;
    refreshSnapshot(doc);
  }

  $connect.addEventListener('click', () => {
    if (ws) { ws.close(); ws = null; }
    const doc = $doc.value || 'demo';
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const url = proto + '://' + location.host + '/ws?doc=' + encodeURIComponent(doc);
    ws = new WebSocket(url);

    ws.addEventListener('open', async () => {
      setStatus('connected', 'ok');
      $editor.disabled = false;
      await refreshSnapshot(doc);
    });
    ws.addEventListener('close', () => {
      setStatus('disconnected', 'err');
      $editor.disabled = true;
    });
    ws.addEventListener('error', () => setStatus('error', 'err'));
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
