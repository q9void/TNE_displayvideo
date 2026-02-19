# Cloudflare Configuration Fix for /v1/bid Endpoint

## Issue
Browser POST requests to `https://ads.thenexusengine.com/v1/bid` fail with `net::ERR_FAILED`, but:
- ✅ OPTIONS (CORS preflight) succeeds
- ✅ Direct curl requests succeed
- ✅ Server is working correctly

## Root Cause
Cloudflare security features are blocking legitimate browser POST requests to the `/v1/bid` endpoint.

---

## Step 1: Check Cloudflare Firewall Events

1. Log into Cloudflare dashboard
2. Select domain: **thenexusengine.com**
3. Go to **Security** → **Events**
4. Filter by:
   - **Path:** `/v1/bid`
   - **Action:** Block
   - **Time:** Last hour

Look for events matching:
- Source IP: `195.224.98.35` (the user's IP from nginx logs)
- User Agent: Chrome
- Method: POST

---

## Step 2: Identify Blocking Rules

Common Cloudflare features that block legitimate API requests:

### A. Bot Fight Mode
**Location:** Security → Bots
**Issue:** Blocks XHR/fetch requests from browsers
**Fix:**
1. Disable "Bot Fight Mode" OR
2. Add page rule exception for `/v1/bid`

### B. WAF Managed Rules
**Location:** Security → WAF → Managed Rules
**Issue:** OWASP Core Ruleset blocks POST with JSON bodies
**Fix:**
1. Create WAF exception for `/v1/bid`:
   ```
   Rule: Skip all managed rules
   When: URI Path contains "/v1/bid"
   ```

### C. Browser Integrity Check
**Location:** Security → Settings
**Issue:** Blocks requests without full browser fingerprint
**Fix:** Disable for API endpoints

### D. Challenge Passage
**Location:** Security → Settings
**Issue:** Requires interactive challenge before POST
**Fix:** Set to "Essentially Off" for `/v1/bid`

---

## Step 3: Create Cloudflare Page Rule

**Recommended Fix:**

1. Go to **Rules** → **Page Rules**
2. Click **Create Page Rule**
3. Configure:
   ```
   URL: ads.thenexusengine.com/v1/bid*

   Settings:
   - Security Level: Essentially Off
   - Browser Integrity Check: Off
   - Cache Level: Bypass
   ```
4. Save and deploy

---

## Step 4: Alternative - Firewall Rule Bypass

**For more granular control:**

1. Go to **Security** → **WAF** → **Custom Rules**
2. Create rule:
   ```
   Rule name: Allow /v1/bid API

   Expression:
   (http.request.uri.path contains "/v1/bid")

   Action: Skip
   - All remaining custom rules
   - All managed rules
   - User Agent Blocking
   ```
3. Deploy

---

## Step 5: Verify CORS Configuration

While OPTIONS works, ensure POST is allowed:

1. Go to **Security** → **Settings**
2. Ensure **CORS** headers include:
   ```
   Access-Control-Allow-Origin: https://dev.totalprosports.com
   Access-Control-Allow-Methods: GET, POST, OPTIONS
   Access-Control-Allow-Headers: Content-Type, X-Requested-With
   Access-Control-Max-Age: 86400
   ```

---

## Step 6: Test After Changes

After making Cloudflare changes, test from browser console:

```javascript
fetch('https://ads.thenexusengine.com/v1/bid', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    accountId: '12345',
    slots: [{
      divId: 'test',
      sizes: [[300, 250]]
    }]
  })
})
.then(r => r.json())
.then(console.log)
.catch(console.error);
```

**Expected result:**
```json
{"bids":[],"responseTime":33}
```

---

## Step 7: Check Rate Limiting

If requests still fail:

1. Go to **Security** → **WAF** → **Rate Limiting Rules**
2. Check if `/v1/bid` is rate-limited
3. If yes, increase threshold or add exception for legitimate origins

---

## Temporary Workaround (Testing Only)

To verify this is a Cloudflare issue, temporarily:

1. Go to **SSL/TLS** → **Overview**
2. Enable **Development Mode** (bypasses Cloudflare cache and some security)
3. Test browser request
4. **IMPORTANT:** Disable Development Mode after testing (auto-disables after 3 hours)

---

## Long-Term Solution

For production, create a **separate hostname** for API endpoints that bypasses Cloudflare security:

1. Create DNS record: `api.thenexusengine.com` → origin server IP
2. Set **Proxy status** to "DNS only" (grey cloud)
3. Update SDK to use `https://api.thenexusengine.com/v1/bid`

This gives you full control over API security at the nginx/server level.

---

## Monitoring

After fixing, monitor in Cloudflare:

1. **Analytics** → **Traffic** → Check `/v1/bid` request volume
2. **Security** → **Events** → Ensure no blocks
3. **Speed** → **Performance** → Monitor API latency

---

## Quick Diagnostics

Run from browser console to see exact error:

```javascript
fetch('https://ads.thenexusengine.com/v1/bid', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({accountId: '12345', slots: []})
})
.then(r => {
  console.log('Status:', r.status);
  console.log('Headers:', [...r.headers.entries()]);
  return r.text();
})
.then(body => console.log('Body:', body))
.catch(err => console.error('Error:', err, err.message));
```

Look for Cloudflare-specific headers in response:
- `CF-RAY` - Request ID for support tickets
- `CF-Cache-Status` - Cache behavior
- `CF-Mitigated` - Security action taken

---

## Support Resources

If issue persists:

1. **Cloudflare Support Ticket:**
   - Include CF-RAY ID from blocked request
   - Mention: "Legitimate API endpoint blocked by security"
   - Request: Review firewall events for `/v1/bid`

2. **Server-Side Logs:**
   ```bash
   ssh catalyst "docker logs catalyst-nginx 2>&1 | grep 'v1/bid'"
   ```
   Should show POST requests if reaching origin.

3. **Browser Network Tab:**
   - Check if request even leaves browser
   - Look for "Blocked by CORS" vs "net::ERR_FAILED"
   - "net::ERR_FAILED" = Cloudflare block
   - "CORS error" = Server CORS config

---

## Expected Outcome

After proper Cloudflare configuration:

✅ Browser POST requests succeed
✅ `/v1/bid` returns `{"bids":[], "responseTime": <ms>}`
✅ SDK logs: "Catalyst bid request completed"
✅ Server logs show: `POST /v1/bid` with 200 status

---

**Last Updated:** 2026-02-16
**Issue Type:** Cloudflare Security False Positive
**Priority:** High - Blocking production traffic
