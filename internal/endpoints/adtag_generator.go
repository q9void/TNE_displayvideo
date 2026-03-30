package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/adtag"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// AdTagGeneratorHandler handles ad tag generation UI
type AdTagGeneratorHandler struct {
	serverURL string
	store     *storage.PublisherStore
}

// NewAdTagGeneratorHandler creates a new ad tag generator handler
func NewAdTagGeneratorHandler(serverURL string, store *storage.PublisherStore) *AdTagGeneratorHandler {
	return &AdTagGeneratorHandler{
		serverURL: serverURL,
		store:     store,
	}
}

// HandleGeneratorUI serves the tag generator UI
func (h *AdTagGeneratorHandler) HandleGeneratorUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>TNE Catalyst Ad Tag Generator</title>
<style>
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
  background: #f5f5f5;
}
h1 {
  color: #333;
  border-bottom: 2px solid #007bff;
  padding-bottom: 10px;
}
.container {
  background: white;
  padding: 30px;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  margin-bottom: 20px;
}
.form-group {
  margin-bottom: 20px;
}
label {
  display: block;
  margin-bottom: 5px;
  font-weight: 600;
  color: #555;
}
input[type="text"],
input[type="number"],
select {
  width: 100%;
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
  box-sizing: border-box;
}
.size-group {
  display: flex;
  gap: 10px;
}
.size-group input {
  flex: 1;
}
button {
  background: #007bff;
  color: white;
  border: none;
  padding: 12px 24px;
  border-radius: 4px;
  font-size: 16px;
  cursor: pointer;
  transition: background 0.2s;
}
button:hover {
  background: #0056b3;
}
.tabs {
  display: flex;
  gap: 10px;
  margin-bottom: 20px;
  border-bottom: 2px solid #ddd;
}
.tab {
  padding: 10px 20px;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  margin-bottom: -2px;
  transition: all 0.2s;
}
.tab.active {
  border-bottom-color: #007bff;
  color: #007bff;
  font-weight: 600;
}
.tab:hover {
  background: #f8f9fa;
}
.code-container {
  background: #f8f9fa;
  border: 1px solid #ddd;
  border-radius: 4px;
  padding: 20px;
  margin-top: 20px;
  position: relative;
}
pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: 'Courier New', monospace;
  font-size: 13px;
}
.copy-button {
  position: absolute;
  top: 10px;
  right: 10px;
  padding: 6px 12px;
  font-size: 12px;
}
.size-preset {
  display: inline-block;
  padding: 6px 12px;
  margin: 5px;
  background: #e9ecef;
  border: 1px solid #dee2e6;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s;
}
.size-preset:hover {
  background: #007bff;
  color: white;
  border-color: #007bff;
}
.info {
  background: #e7f3ff;
  border-left: 4px solid #007bff;
  padding: 15px;
  margin: 20px 0;
  border-radius: 4px;
}
.output {
  display: none;
}
.output.active {
  display: block;
}
</style>
</head>
<body>
<h1>🎯 TNE Catalyst Ad Tag Generator</h1>

<div class="container">
  <h2>Ad Unit Configuration</h2>

  <div class="form-group">
    <label>Publisher ID *</label>
    <input type="text" id="publisherId" placeholder="pub-123456" required>
  </div>

  <div class="form-group">
    <label>Placement ID *</label>
    <input type="text" id="placementId" placeholder="homepage-banner-1" required>
  </div>

  <div class="form-group">
    <label>Ad Size *</label>
    <div class="size-group">
      <input type="number" id="width" placeholder="Width (px)" min="1" value="300">
      <input type="number" id="height" placeholder="Height (px)" min="1" value="250">
    </div>
    <div style="margin-top: 10px;">
      <strong>Common Sizes:</strong>
      <span class="size-preset" onclick="setSize(300, 250)">300x250</span>
      <span class="size-preset" onclick="setSize(728, 90)">728x90</span>
      <span class="size-preset" onclick="setSize(970, 250)">970x250</span>
      <span class="size-preset" onclick="setSize(300, 600)">300x600</span>
      <span class="size-preset" onclick="setSize(160, 600)">160x600</span>
      <span class="size-preset" onclick="setSize(320, 50)">320x50 (Mobile)</span>
      <span class="size-preset" onclick="setSize(320, 100)">320x100 (Mobile)</span>
    </div>
  </div>

  <div class="form-group">
    <label>Format</label>
    <select id="format">
      <option value="async">Async JavaScript (Recommended)</option>
      <option value="gam">GAM 3rd Party Script</option>
      <option value="iframe">Iframe</option>
      <option value="sync">Sync JavaScript</option>
    </select>
  </div>

  <button onclick="generateTag()">Generate Ad Tag</button>
</div>

<div id="results" style="display:none;">
  <div class="container">
    <h2>Generated Ad Tag</h2>

    <div class="info">
      <strong>How to use:</strong> Copy the code below and paste it into your webpage where you want the ad to appear.
    </div>

    <div class="tabs">
      <div class="tab active" onclick="showTab('html')">HTML</div>
      <div class="tab" onclick="showTab('javascript')">JavaScript Only</div>
      <div class="tab" onclick="showTab('test')">Test</div>
    </div>

    <div class="output active" id="html-output">
      <div class="code-container">
        <button class="copy-button" onclick="copyCode('html-code')">Copy</button>
        <pre id="html-code"></pre>
      </div>
    </div>

    <div class="output" id="javascript-output">
      <div class="code-container">
        <button class="copy-button" onclick="copyCode('js-code')">Copy</button>
        <pre id="js-code"></pre>
      </div>
    </div>

    <div class="output" id="test-output">
      <div class="info">
        <strong>Live Preview:</strong> This is how the ad will look on your page.
      </div>
      <div id="test-container" style="border: 2px dashed #ddd; padding: 20px; margin-top: 20px;"></div>
    </div>
  </div>
</div>

<script>
function setSize(width, height) {
  document.getElementById('width').value = width;
  document.getElementById('height').value = height;
}

function showTab(tab) {
  // Update tabs
  document.querySelectorAll('.tab').forEach(function(el) {
    el.classList.remove('active');
  });
  event.target.classList.add('active');

  // Update outputs
  document.querySelectorAll('.output').forEach(function(el) {
    el.classList.remove('active');
  });
  document.getElementById(tab + '-output').classList.add('active');
}

function generateTag() {
  var publisherId = document.getElementById('publisherId').value;
  var placementId = document.getElementById('placementId').value;
  var width = parseInt(document.getElementById('width').value);
  var height = parseInt(document.getElementById('height').value);
  var format = document.getElementById('format').value;

  if (!publisherId || !placementId || !width || !height) {
    alert('Please fill in all required fields');
    return;
  }

  // Call API to generate tag
  fetch('/admin/adtag/generate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      publisherId: publisherId,
      placementId: placementId,
      width: width,
      height: height,
      format: format
    })
  })
  .then(function(response) {
    return response.json();
  })
  .then(function(data) {
    document.getElementById('html-code').textContent = data.html;
    document.getElementById('js-code').textContent = data.javascript || 'Not available for this format';

    // Load test ad
    document.getElementById('test-container').innerHTML = data.html;

    // Show results
    document.getElementById('results').style.display = 'block';
    document.getElementById('results').scrollIntoView({ behavior: 'smooth' });
  })
  .catch(function(error) {
    alert('Error generating tag: ' + error.message);
  });
}

function copyCode(elementId) {
  var code = document.getElementById(elementId).textContent;
  navigator.clipboard.writeText(code).then(function() {
    var button = event.target;
    var originalText = button.textContent;
    button.textContent = 'Copied!';
    setTimeout(function() {
      button.textContent = originalText;
    }, 2000);
  });
}
</script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// HandleGenerateTag handles API requests to generate ad tags
func (h *AdTagGeneratorHandler) HandleGenerateTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req struct {
		PublisherID string `json:"publisherId"`
		PlacementID string `json:"placementId"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Format      string `json:"format"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Create ad tag config
	config := &adtag.AdTagConfig{
		ServerURL:   h.serverURL,
		PublisherID: req.PublisherID,
		PlacementID: req.PlacementID,
		Width:       req.Width,
		Height:      req.Height,
	}

	// Determine format
	format := adtag.FormatAsync
	switch req.Format {
	case "sync":
		format = adtag.FormatSync
	case "iframe":
		format = adtag.FormatIframe
	case "gam":
		format = adtag.FormatGAM
	}

	// Generate tag
	generator := adtag.NewGenerator(h.serverURL)
	tag, err := generator.Generate(config, format)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to generate ad tag")
		http.Error(w, "Failed to generate tag", http.StatusInternalServerError)
		return
	}

	// Return response
	response := map[string]string{
		"html":       tag.HTML,
		"javascript": tag.JavaScript,
		"iframeUrl":  tag.IframeURL,
		"gamScript":  tag.GAMScript,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleAssets serves static assets
func HandleAssets(w http.ResponseWriter, r *http.Request) {
	// Serve tne-ads.js
	if r.URL.Path == "/assets/tne-ads.js" {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")

		// In production, read from file
		// For now, serve inline
		http.ServeFile(w, r, "assets/tne-ads.js")
		return
	}

	http.NotFound(w, r)
}

// HandleCatalystSDK serves the Catalyst MAI Publisher SDK
func HandleCatalystSDK(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Serve catalyst-sdk.js from file
	http.ServeFile(w, r, "assets/catalyst-sdk.js")
}

// BulkTagResult holds the generated tags for a single ad slot.
type BulkTagResult struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	SlotPattern string `json:"slot_pattern"`
	SlotName    string `json:"slot_name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	AsyncTag    string `json:"async_tag"`
	GAMScript   string `json:"gam_script"`
	IframeURL   string `json:"iframe_url"`
}

// HandleBulkExportTags generates ad tags for all slots (optionally filtered by account).
// GET /admin/adtag/export-bulk?account_id=NXS001&download=1
func (h *AdTagGeneratorHandler) HandleBulkExportTags(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "store not configured", http.StatusInternalServerError)
		return
	}

	accountID := r.URL.Query().Get("account_id")
	download := r.URL.Query().Get("download") == "1"

	slots, err := h.store.GetAdSlotsForExport(context.Background(), accountID)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to query slots for export")
		http.Error(w, "Failed to query slots", http.StatusInternalServerError)
		return
	}

	gen := adtag.NewGenerator(h.serverURL)
	results := make([]BulkTagResult, 0, len(slots))

	for _, s := range slots {
		cfg := &adtag.AdTagConfig{
			ServerURL:   h.serverURL,
			PublisherID: s.AccountID,
			PlacementID: s.SlotPattern,
			Width:       s.Width,
			Height:      s.Height,
		}

		asyncTag, _ := gen.Generate(cfg, adtag.FormatAsync)
		gamTag, _ := gen.Generate(cfg, adtag.FormatGAM)
		iframeTag, _ := gen.Generate(cfg, adtag.FormatIframe)

		r := BulkTagResult{
			AccountID:   s.AccountID,
			AccountName: s.AccountName,
			SlotPattern: s.SlotPattern,
			SlotName:    s.SlotName,
			Width:       s.Width,
			Height:      s.Height,
		}
		if asyncTag != nil {
			r.AsyncTag = asyncTag.HTML
		}
		if gamTag != nil {
			r.GAMScript = gamTag.GAMScript
		}
		if iframeTag != nil {
			r.IframeURL = iframeTag.IframeURL
		}
		results = append(results, r)
	}

	if download {
		// Plain-text download: one block per slot separated by comments
		label := accountID
		if label == "" {
			label = "all"
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="tags-%s.txt"`, label))
		var sb strings.Builder
		for _, res := range results {
			sb.WriteString(fmt.Sprintf("/* ===== %s / %s (%dx%d) =====\n   Async Tag\n ===== */\n", res.AccountID, res.SlotPattern, res.Width, res.Height))
			sb.WriteString(res.AsyncTag)
			sb.WriteString("\n\n")
			sb.WriteString(fmt.Sprintf("/* ===== %s / %s - GAM Script ===== */\n", res.AccountID, res.SlotPattern))
			sb.WriteString(res.GAMScript)
			sb.WriteString("\n\n")
		}
		w.Write([]byte(sb.String())) //nolint:errcheck
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results) //nolint:errcheck
}
