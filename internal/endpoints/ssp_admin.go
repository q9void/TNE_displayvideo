package endpoints

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// SSPAdminHandler serves the SSP ID management UI and its API endpoints.
type SSPAdminHandler struct {
	store    *storage.PublisherStore
	basePath string
}

// NewSSPAdminHandler creates a new SSPAdminHandler mounted at basePath.
func NewSSPAdminHandler(store *storage.PublisherStore, basePath string) *SSPAdminHandler {
	return &SSPAdminHandler{store: store, basePath: strings.TrimSuffix(basePath, "/")}
}

func (h *SSPAdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, h.basePath)
	if sub == "" {
		sub = "/"
	}
	switch {
	case sub == "" || sub == "/":
		if r.Method == http.MethodGet {
			h.serveUI(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case sub == "/configs":
		if r.Method == http.MethodGet {
			h.listConfigs(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasPrefix(sub, "/configs/"):
		if r.Method == http.MethodPut {
			h.updateConfig(w, r, strings.TrimPrefix(sub, "/configs/"))
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

func (h *SSPAdminHandler) listConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.GetAllSlotBidderConfigs(r.Context())
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch slot bidder configs")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if configs == nil {
		configs = []storage.SlotBidderConfigRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
}

func (h *SSPAdminHandler) updateConfig(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid config ID", http.StatusBadRequest)
		return
	}
	var body struct {
		BidderParams json.RawMessage `json:"bidder_params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if !json.Valid(body.BidderParams) {
		http.Error(w, "bidder_params must be valid JSON", http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateSlotBidderParams(r.Context(), id, body.BidderParams); err != nil {
		logger.Log.Error().Err(err).Int("id", id).Msg("Failed to update slot bidder params")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (h *SSPAdminHandler) serveUI(w http.ResponseWriter, r *http.Request) {
	// Load data server-side to avoid relying on browser auth for the API fetch
	configs, err := h.store.GetAllSlotBidderConfigs(r.Context())
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to load configs for admin UI")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if configs == nil {
		configs = []storage.SlotBidderConfigRow{}
	}
	configsJSON, _ := json.Marshal(configs)

	// Build auth header so the page can make authenticated PUT calls
	user := os.Getenv("ADMIN_USER")
	pass := os.Getenv("ADMIN_PASSWORD")
	authHeader := ""
	if user != "" && pass != "" {
		authHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	}

	page := strings.ReplaceAll(sspAdminHTML, "__API_BASE__", h.basePath+"/configs")
	page = strings.ReplaceAll(page, `"__INITIAL_DATA__"`, string(configsJSON))
	page = strings.ReplaceAll(page, "__AUTH_HEADER__", authHeader)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(page))
}

const sspAdminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>NXS Catalyst Admin</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#0f1117;color:#e2e8f0;min-height:100vh;font-size:13px}
header{background:#1a1d27;border-bottom:1px solid #2d3148;padding:14px 20px;display:flex;align-items:center;gap:12px}
header h1{font-size:1rem;font-weight:700;color:#fff}
.subtitle{font-size:0.75rem;color:#6b7280}
.stats{display:flex;gap:20px;margin-left:auto}
.stat{text-align:right}
.stat-value{font-size:1rem;font-weight:700;color:#818cf8}
.stat-label{font-size:0.65rem;color:#6b7280;text-transform:uppercase;letter-spacing:0.05em}
.toolbar{padding:10px 20px;display:flex;gap:8px;align-items:center;background:#13151f;border-bottom:1px solid #1e2130;flex-wrap:wrap}
.toolbar select,.toolbar input{background:#0f1117;border:1px solid #2d3148;color:#e2e8f0;padding:5px 8px;border-radius:5px;font-size:0.8rem;outline:none}
.toolbar select:focus,.toolbar input:focus{border-color:#818cf8}
.toolbar label{font-size:0.72rem;color:#6b7280}
.toolbar .grp{display:flex;align-items:center;gap:4px}
.toolbar input[type=text]{width:180px}
#count{font-size:0.75rem;color:#4b5563;margin-left:auto}
.tbl-wrap{overflow-x:auto;padding:0}
table{width:100%;border-collapse:collapse;font-size:0.8rem}
thead tr{background:#1a1d27;position:sticky;top:0;z-index:10}
thead th{padding:9px 12px;text-align:left;font-weight:600;color:#9ca3af;font-size:0.72rem;text-transform:uppercase;letter-spacing:0.05em;border-bottom:1px solid #2d3148;white-space:nowrap}
tbody tr{border-bottom:1px solid #1a1c2a}
tbody tr:hover{background:#13151f}
tbody tr.editing-row{background:#120f05}
td{padding:8px 12px;vertical-align:top}
.td-account{font-family:monospace;color:#818cf8;font-weight:600;white-space:nowrap}
.td-domain{font-family:monospace;color:#a5b4fc;white-space:nowrap}
.td-slot{font-family:monospace;color:#c7d2fe;word-break:break-all;max-width:220px}
.td-bidder{font-family:monospace;color:#fbbf24;font-weight:600;white-space:nowrap}
.td-device span{font-size:0.7rem;padding:2px 6px;border-radius:6px;background:#1a2535;color:#60a5fa;border:1px solid #1e3a5f;white-space:nowrap}
.td-params{font-family:monospace;color:#9ca3af;word-break:break-all;max-width:320px;line-height:1.6}
.kv{display:inline-block;margin:1px 6px 1px 0}
.k{color:#6b7280}
.v{color:#a5b4fc}
.td-status span{font-size:0.68rem;padding:2px 7px;border-radius:8px;white-space:nowrap}
.s-active{background:#052e16;color:#4ade80;border:1px solid #14532d}
.s-inactive{background:#450a0a;color:#f87171;border:1px solid #7f1d1d}
.td-actions{white-space:nowrap;text-align:right}
.btn{border:none;border-radius:5px;cursor:pointer;font-size:0.75rem;font-weight:500;padding:4px 10px}
.btn-edit{background:#312e81;color:#a5b4fc}
.btn-edit:hover{background:#3730a3}
.btn-save{background:#14532d;color:#86efac}
.btn-save:hover{background:#166534}
.btn-cancel{background:#374151;color:#9ca3af}
.btn-cancel:hover{background:#4b5563}
.btn-save:disabled{opacity:0.4;cursor:not-allowed}
.edit-td{padding:8px 12px 12px}
.edit-td textarea{width:100%;background:#0a0b10;border:1px solid #374151;color:#e2e8f0;font-family:monospace;font-size:0.78rem;padding:7px;border-radius:5px;resize:vertical;min-height:72px}
.edit-td textarea.err{border-color:#ef4444}
.edit-footer{display:flex;gap:8px;margin-top:6px;align-items:center}
.err-msg{font-size:0.72rem;color:#f87171}
.toast{position:fixed;bottom:20px;right:20px;background:#14532d;color:#86efac;padding:10px 16px;border-radius:7px;font-size:0.82rem;font-weight:500;display:none;z-index:1000;border:1px solid #166534}
.toast.err{background:#450a0a;color:#fca5a5;border-color:#7f1d1d}
.empty{text-align:center;color:#4b5563;padding:60px;font-size:0.85rem}
</style>
</head>
<body>

<header>
  <div>
    <h1>NXS Catalyst Admin</h1>
    <div class="subtitle">SSP IDs &amp; bidder param management</div>
  </div>
  <div class="stats" id="stats"></div>
</header>

<div class="toolbar">
  <div class="grp"><label>NXS ID</label><select id="f-account" onchange="applyFilters()"><option value="">All</option></select></div>
  <div class="grp"><label>Domain</label><select id="f-domain" onchange="applyFilters()"><option value="">All</option></select></div>
  <div class="grp"><label>Bidder</label><select id="f-bidder" onchange="applyFilters()"><option value="">All</option></select></div>
  <div class="grp"><label>Status</label>
    <select id="f-status" onchange="applyFilters()">
      <option value="">All</option>
      <option value="active">Active</option>
      <option value="inactive">Inactive</option>
    </select>
  </div>
  <div class="grp"><input type="text" id="f-search" oninput="applyFilters()" placeholder="Search slot pattern..."></div>
  <span id="count"></span>
</div>

<div class="tbl-wrap">
<table id="tbl">
<thead>
  <tr>
    <th>NXS ID</th>
    <th>Domain</th>
    <th>Slot Pattern</th>
    <th>Bidder</th>
    <th>Device</th>
    <th>Params</th>
    <th>Status</th>
    <th></th>
  </tr>
</thead>
<tbody id="tbody"></tbody>
</table>
</div>

<div class="toast" id="toast"></div>

<script>
var API_BASE = '__API_BASE__';
var AUTH_HEADER = '__AUTH_HEADER__';
var allConfigs = "__INITIAL_DATA__";
var editingId = null;

function init() {
  try {
    if (!Array.isArray(allConfigs) || allConfigs.length === 0) {
      document.getElementById('tbody').innerHTML = '<tr><td colspan="8" class="empty">No configurations found</td></tr>';
      return;
    }
    populateFilters();
    renderStats();
    buildTable();
  } catch(e) {
    document.getElementById('tbody').innerHTML =
      '<tr><td colspan="8" style="color:#f87171;padding:20px;font-family:monospace">Error: ' + esc(e.message) + '<br><pre>' + esc(e.stack||'') + '</pre></td></tr>';
  }
}

function populateFilters() {
  fillSel('f-account', uniq(allConfigs.map(function(r){return r.account_id;})));
  fillSel('f-domain',  uniq(allConfigs.map(function(r){return r.domain;})));
  fillSel('f-bidder',  uniq(allConfigs.map(function(r){return r.bidder_code;})));
}

function fillSel(id, vals) {
  var sel = document.getElementById(id);
  while (sel.options.length > 1) sel.remove(1);
  vals.forEach(function(v) { sel.add(new Option(v, v)); });
}

function uniq(arr) {
  return arr.filter(function(v,i,a){return a.indexOf(v)===i;}).sort();
}

function renderStats() {
  var accounts = uniq(allConfigs.map(function(r){return r.account_id;}));
  var domains  = uniq(allConfigs.map(function(r){return r.domain;}));
  var slots    = uniq(allConfigs.map(function(r){return r.account_id+'|'+r.domain+'|'+r.slot_pattern;}));
  document.getElementById('stats').innerHTML =
    '<div class="stat"><div class="stat-value">'+accounts.length+'</div><div class="stat-label">NXS IDs</div></div>' +
    '<div class="stat"><div class="stat-value">'+domains.length+'</div><div class="stat-label">Domains</div></div>' +
    '<div class="stat"><div class="stat-value">'+slots.length+'</div><div class="stat-label">Ad Units</div></div>' +
    '<div class="stat"><div class="stat-value">'+allConfigs.length+'</div><div class="stat-label">Bidder Configs</div></div>';
}

// Build the table once from allConfigs; filtering just toggles display
function buildTable() {
  var tbody = document.getElementById('tbody');
  var frag = document.createDocumentFragment();
  allConfigs.forEach(function(r) {
    var tr = document.createElement('tr');
    tr.setAttribute('data-id', r.id);
    tr.setAttribute('data-account', r.account_id);
    tr.setAttribute('data-domain', r.domain);
    tr.setAttribute('data-bidder', r.bidder_code);
    tr.setAttribute('data-status', r.status);
    tr.setAttribute('data-slot', r.slot_pattern.toLowerCase());
    tr.innerHTML = buildRowHTML(r);
    frag.appendChild(tr);
  });
  tbody.appendChild(frag);
  updateCount(allConfigs.length);
}

function buildRowHTML(r) {
  var params = r.bidder_params || {};
  var paramsHTML = Object.keys(params).map(function(k) {
    return '<span class="kv"><span class="k">'+esc(k)+':</span> <span class="v">'+esc(String(params[k]))+'</span></span>';
  }).join('');
  var statusClass = r.status === 'active' ? 's-active' : 's-inactive';
  return '<td class="td-account">'+esc(r.account_id)+'</td>' +
    '<td class="td-domain">'+esc(r.domain)+'</td>' +
    '<td class="td-slot">'+esc(r.slot_pattern)+'</td>' +
    '<td class="td-bidder">'+esc(r.bidder_code)+'</td>' +
    '<td class="td-device"><span>'+esc(r.device_type)+'</span></td>' +
    '<td class="td-params">'+paramsHTML+'</td>' +
    '<td class="td-status"><span class="'+statusClass+'">'+esc(r.status)+'</span></td>' +
    '<td class="td-actions"><button class="btn btn-edit" onclick="startEdit('+r.id+')">Edit</button></td>';
}

function applyFilters() {
  var account = document.getElementById('f-account').value;
  var domain  = document.getElementById('f-domain').value;
  var bidder  = document.getElementById('f-bidder').value;
  var status  = document.getElementById('f-status').value;
  var search  = document.getElementById('f-search').value.toLowerCase();
  var rows = document.getElementById('tbody').getElementsByTagName('tr');
  var visible = 0;
  for (var i = 0; i < rows.length; i++) {
    var tr = rows[i];
    // skip the edit row (no data-account attr)
    if (!tr.getAttribute('data-account')) { continue; }
    var show = (!account || tr.getAttribute('data-account') === account) &&
               (!domain  || tr.getAttribute('data-domain') === domain) &&
               (!bidder  || tr.getAttribute('data-bidder') === bidder) &&
               (!status  || tr.getAttribute('data-status') === status) &&
               (!search  || tr.getAttribute('data-slot').indexOf(search) !== -1);
    tr.style.display = show ? '' : 'none';
    if (show) visible++;
    // also hide any edit row immediately following
    var next = tr.nextElementSibling;
    if (next && next.classList.contains('edit-row')) {
      next.style.display = show ? '' : 'none';
    }
  }
  updateCount(visible);
}

function updateCount(n) {
  document.getElementById('count').textContent = n + ' of ' + allConfigs.length + ' rows';
}

function startEdit(id) {
  // Cancel any existing edit
  var existing = document.getElementById('edit-row-'+editingId);
  if (existing) {
    existing.remove();
    var oldTr = document.querySelector('tr[data-id="'+editingId+'"]');
    if (oldTr) oldTr.classList.remove('editing-row');
  }
  editingId = id;

  var cfg = allConfigs.find(function(r){return r.id === id;});
  if (!cfg) return;
  var tr = document.querySelector('tr[data-id="'+id+'"]');
  if (!tr) return;
  tr.classList.add('editing-row');

  var editTr = document.createElement('tr');
  editTr.id = 'edit-row-' + id;
  editTr.className = 'edit-row editing-row';
  var prettyJSON = JSON.stringify(cfg.bidder_params || {}, null, 2);
  editTr.innerHTML = '<td colspan="8" class="edit-td">' +
    '<textarea id="ta-'+id+'" oninput="validateJSON('+id+')">'+esc(prettyJSON)+'</textarea>' +
    '<div class="edit-footer">' +
    '<button class="btn btn-save" id="save-'+id+'" onclick="saveEdit('+id+')">Save changes</button>' +
    '<button class="btn btn-cancel" onclick="cancelEdit('+id+')">Cancel</button>' +
    '<span class="err-msg" id="err-'+id+'"></span>' +
    '</div></td>';

  tr.parentNode.insertBefore(editTr, tr.nextSibling);
  document.getElementById('ta-'+id).focus();
}

function cancelEdit(id) {
  var editTr = document.getElementById('edit-row-'+id);
  if (editTr) editTr.remove();
  var tr = document.querySelector('tr[data-id="'+id+'"]');
  if (tr) tr.classList.remove('editing-row');
  editingId = null;
}

function validateJSON(id) {
  var ta  = document.getElementById('ta-'+id);
  var err = document.getElementById('err-'+id);
  var btn = document.getElementById('save-'+id);
  try {
    JSON.parse(ta.value);
    ta.classList.remove('err');
    err.textContent = '';
    btn.disabled = false;
  } catch(e) {
    ta.classList.add('err');
    err.textContent = e.message;
    btn.disabled = true;
  }
}

async function saveEdit(id) {
  var ta  = document.getElementById('ta-'+id);
  var btn = document.getElementById('save-'+id);
  var err = document.getElementById('err-'+id);
  var parsed;
  try { parsed = JSON.parse(ta.value); }
  catch(e) { err.textContent = e.message; return; }

  btn.disabled = true;
  btn.textContent = 'Saving...';

  try {
    var headers = {'Content-Type':'application/json'};
    if (AUTH_HEADER) headers['Authorization'] = AUTH_HEADER;
    var res = await fetch(API_BASE+'/'+id, {
      method: 'PUT',
      headers: headers,
      body: JSON.stringify({bidder_params: parsed})
    });
    if (!res.ok) throw new Error('HTTP '+res.status+': '+(await res.text()));

    // Update local cache and re-render the row cells
    var cfg = allConfigs.find(function(r){return r.id === id;});
    if (cfg) {
      cfg.bidder_params = parsed;
      var tr = document.querySelector('tr[data-id="'+id+'"]');
      if (tr) tr.innerHTML = buildRowHTML(cfg);
    }

    cancelEdit(id);
    toast('Saved config #'+id, false);
  } catch(e) {
    btn.disabled = false;
    btn.textContent = 'Save changes';
    err.textContent = 'Save failed: '+e.message;
    toast('Save failed: '+e.message, true);
  }
}

function toast(msg, isErr) {
  var el = document.getElementById('toast');
  el.textContent = msg;
  el.className = 'toast'+(isErr?' err':'');
  el.style.display = 'block';
  setTimeout(function(){ el.style.display='none'; }, 3500);
}

function esc(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

init();
</script>
</body>
</html>`
