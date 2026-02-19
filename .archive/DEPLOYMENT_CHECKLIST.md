# Catalyst Deployment Checklist

Use this checklist during deployment to track progress.

---

## Pre-Deployment Verification

- [x] Code compiled successfully (`go build`)
- [x] Binary created: `build/catalyst-server` (26MB)
- [x] Mapping file generated: `config/bizbudding-all-bidders-mapping.json`
- [x] Mapping contains 10 ad units
- [x] Mapping contains 7 bidders per unit
- [x] Deployment package created: `build/catalyst-deployment.tar.gz` (13MB)
- [x] Test scripts created and executable
- [x] Documentation complete

---

## Deployment Steps

### 1. Upload Package
```bash
scp build/catalyst-deployment.tar.gz user@ads.thenexusengine.com:/tmp/
```
- [ ] Package uploaded successfully
- [ ] No transfer errors

### 2. Connect to Server
```bash
ssh user@ads.thenexusengine.com
```
- [ ] SSH connection established

### 3. Stop Service
```bash
cd /opt/catalyst
sudo systemctl stop catalyst
```
- [ ] Service stopped
- [ ] Check: `sudo systemctl status catalyst` shows "inactive"

### 4. Backup Current Version
```bash
sudo cp catalyst-server catalyst-server.backup.$(date +%Y%m%d-%H%M%S)
```
- [ ] Backup created
- [ ] Backup file exists: `ls -lh catalyst-server.backup.*`

### 5. Extract New Version
```bash
sudo tar xzf /tmp/catalyst-deployment.tar.gz --strip-components=1
```
- [ ] Extraction complete
- [ ] No errors during extraction

### 6. Set Permissions
```bash
sudo mv build/catalyst-server ./catalyst-server
sudo chmod +x catalyst-server
```
- [ ] Binary moved to correct location
- [ ] Permissions set correctly

### 7. Verify Files
```bash
ls -lh catalyst-server config/bizbudding-all-bidders-mapping.json
```
- [ ] Binary exists and is executable
- [ ] Mapping file exists
- [ ] Assets directory present

### 8. Start Service
```bash
sudo systemctl start catalyst
```
- [ ] Service started
- [ ] No startup errors

### 9. Wait for Startup
```bash
sleep 3
```
- [ ] Waited 3 seconds for service initialization

### 10. Check Logs
```bash
sudo journalctl -u catalyst -n 50 --no-pager
```
Look for:
- [ ] "Loaded bidder mapping: 10 ad units"
- [ ] "Configured bidders: rubicon, kargo, sovrn, oms, aniview, pubmatic, triplelift"
- [ ] "Catalyst MAI Publisher endpoint registered: /v1/bid"
- [ ] No error messages

---

## Post-Deployment Verification

### Local Health Check (on server)
```bash
curl -s http://localhost:8000/health | jq .
```
- [ ] Returns 200 OK
- [ ] JSON contains `"status": "healthy"`

### Remote Health Check (from local machine)
```bash
curl https://ads.thenexusengine.com/health
```
- [ ] Returns 200 OK
- [ ] Reachable from internet

### SDK Endpoint
```bash
curl -I https://ads.thenexusengine.com/assets/catalyst-sdk.js
```
- [ ] Returns 200 OK
- [ ] Content-Type: application/javascript

### Bid Endpoint Test
```bash
./scripts/test-bid-request.sh
```
- [ ] Test 1 passes (single ad unit)
- [ ] Test 2 passes (multiple ad units)
- [ ] Test 3 passes (unknown ad unit)
- [ ] Response time < 2500ms

### Browser Test
```
https://ads.thenexusengine.com/test-magnite.html
```
- [ ] Page loads without errors
- [ ] SDK initializes
- [ ] Bid request sent
- [ ] Response received
- [ ] `biddersReady('catalyst')` callback fires
- [ ] No JavaScript errors in console

### Log Monitoring
```bash
ssh user@ads.thenexusengine.com 'sudo journalctl -u catalyst -f'
```
Make a test bid request and verify logs show:
- [ ] "Catalyst bid request received"
- [ ] "Found mapping for ad unit: totalprosports.com/..."
- [ ] "Injected parameters for 7 bidders"
- [ ] "Catalyst bid request completed"

### Metrics Endpoint
```bash
curl https://ads.thenexusengine.com/metrics | grep catalyst
```
- [ ] Metrics endpoint responding
- [ ] `catalyst_bid_requests_total` present
- [ ] `catalyst_bid_latency_seconds` present

---

## Smoke Tests (15 minutes)

### Test Each Ad Unit Type

Desktop:
```bash
# Billboard
curl -X POST https://ads.thenexusengine.com/v1/bid -H "Content-Type: application/json" -d '{"accountId":"icisic-media","timeout":2800,"slots":[{"divId":"test","sizes":[[970,250]],"adUnitPath":"totalprosports.com/billboard"}]}'
```
- [ ] Billboard: Response OK

```bash
# Leaderboard
curl -X POST https://ads.thenexusengine.com/v1/bid -H "Content-Type: application/json" -d '{"accountId":"icisic-media","timeout":2800,"slots":[{"divId":"test","sizes":[[728,90]],"adUnitPath":"totalprosports.com/leaderboard"}]}'
```
- [ ] Leaderboard: Response OK

Mobile:
```bash
# Rectangle
curl -X POST https://ads.thenexusengine.com/v1/bid -H "Content-Type: application/json" -d '{"accountId":"icisic-media","timeout":2800,"slots":[{"divId":"test","sizes":[[300,250]],"adUnitPath":"totalprosports.com/rectangle-medium"}]}'
```
- [ ] Rectangle: Response OK

---

## Performance Check

### Response Time
```bash
for i in {1..10}; do
  time curl -s -X POST https://ads.thenexusengine.com/v1/bid \
    -H "Content-Type: application/json" \
    -d '{"accountId":"icisic-media","timeout":2800,"slots":[{"divId":"test","sizes":[[728,90]],"adUnitPath":"totalprosports.com/leaderboard"}]}' \
    > /dev/null
done
```
- [ ] Average response time < 2500ms
- [ ] No timeouts
- [ ] Consistent performance

### Concurrent Requests
```bash
for i in {1..5}; do
  curl -s -X POST https://ads.thenexusengine.com/v1/bid \
    -H "Content-Type: application/json" \
    -d '{"accountId":"icisic-media","timeout":2800,"slots":[{"divId":"test-'$i'","sizes":[[728,90]],"adUnitPath":"totalprosports.com/leaderboard"}]}' &
done
wait
```
- [ ] All requests complete
- [ ] No errors
- [ ] Server stable

---

## Error Scenarios

### Unknown Ad Unit
```bash
curl -X POST https://ads.thenexusengine.com/v1/bid \
  -H "Content-Type: application/json" \
  -d '{"accountId":"icisic-media","timeout":2800,"slots":[{"divId":"test","sizes":[[728,90]],"adUnitPath":"unknown.com/test"}]}'
```
- [ ] Returns 200 (not 500)
- [ ] Returns empty bids: `{"bids":[],"responseTime":...}`
- [ ] Warning in logs: "No mapping found for ad unit"

### Invalid Request
```bash
curl -X POST https://ads.thenexusengine.com/v1/bid \
  -H "Content-Type: application/json" \
  -d '{"invalid":"json"}'
```
- [ ] Returns 400 Bad Request
- [ ] Error message in response

### Missing Required Fields
```bash
curl -X POST https://ads.thenexusengine.com/v1/bid \
  -H "Content-Type: application/json" \
  -d '{"accountId":"icisic-media"}'
```
- [ ] Returns 400 Bad Request
- [ ] Error: "at least one slot is required"

---

## Service Health

### Service Status
```bash
ssh user@ads.thenexusengine.com 'sudo systemctl status catalyst'
```
- [ ] Status: active (running)
- [ ] No failed starts
- [ ] Uptime > 0 seconds

### Memory Usage
```bash
ssh user@ads.thenexusengine.com 'free -h'
```
- [ ] Available memory > 1GB
- [ ] No swap usage

### Disk Space
```bash
ssh user@ads.thenexusengine.com 'df -h'
```
- [ ] / partition < 80% full
- [ ] No disk space warnings

### Process Check
```bash
ssh user@ads.thenexusengine.com 'ps aux | grep catalyst-server'
```
- [ ] Process running
- [ ] CPU usage reasonable (<50%)
- [ ] Memory usage reasonable (<500MB)

---

## Rollback Plan (if needed)

If any issues occur:

```bash
ssh user@ads.thenexusengine.com
cd /opt/catalyst
sudo systemctl stop catalyst
sudo cp catalyst-server.backup.YYYYMMDD-HHMMSS catalyst-server
sudo systemctl start catalyst
```

- [ ] Backup file identified
- [ ] Rollback procedure documented
- [ ] Team knows rollback command

---

## Sign-Off

### Technical Validation
- [ ] All health checks pass
- [ ] All API tests pass
- [ ] Browser test successful
- [ ] No errors in logs
- [ ] Performance acceptable
- [ ] Monitoring working

### Documentation
- [ ] Deployment documented
- [ ] Issues logged (if any)
- [ ] Team notified

### Production Ready
- [ ] Service running stable for 30 minutes
- [ ] Ready for real traffic
- [ ] Monitoring configured
- [ ] On-call team informed

---

## Next Steps

After successful deployment:

1. **Notify Stakeholders**
   - [ ] BizBudding/MAI Publisher team
   - [ ] Internal development team
   - [ ] Operations team

2. **Share Endpoints**
   - [ ] Production bid endpoint: `https://ads.thenexusengine.com/v1/bid`
   - [ ] SDK endpoint: `https://ads.thenexusengine.com/assets/catalyst-sdk.js`
   - [ ] Test page: `https://ads.thenexusengine.com/test-magnite.html`

3. **Monitor for 24 Hours**
   - [ ] Watch metrics dashboard
   - [ ] Review logs daily
   - [ ] Track error rates
   - [ ] Monitor performance

4. **Coordinate Integration**
   - [ ] Schedule call with BizBudding
   - [ ] Provide API documentation
   - [ ] Support integration testing
   - [ ] Plan production cutover

---

## Deployment Sign-Off

**Deployed By:** ___________________

**Date:** ___________________

**Time:** ___________________

**Server:** ads.thenexusengine.com

**Version:** 1.0.0 (Multi-Bidder)

**Status:** ___________________

**Notes:**
_______________________________________________________
_______________________________________________________
_______________________________________________________

---

## Contact Information

**On-Call Engineer:** ___________________

**Escalation Contact:** ___________________

**Monitoring Dashboard:** https://ads.thenexusengine.com/admin/dashboard

**Logs:** `ssh user@ads.thenexusengine.com 'sudo journalctl -u catalyst -f'`

---

**Deployment Complete:** [ ] YES  [ ] NO (rollback)

**Ready for Production Traffic:** [ ] YES  [ ] NO
