# Production Deployment Checklist - TNE Catalyst

**Version:** 1.0
**Date:** 2026-01-19
**Target Environment:** Production

---

## Pre-Deployment Checklist

### ✅ Security Requirements

- [ ] **Redis password configured** (REDIS_PASSWORD in .env.production)
  ```bash
  # Generate strong password
  openssl rand -base64 32
  # Add to .env.production: REDIS_PASSWORD=<generated-password>
  ```

- [ ] **PostgreSQL password changed** (default "changeme" must be replaced)
  ```bash
  # Generate strong password
  openssl rand -base64 32
  # Update .env.production: DB_PASSWORD=<generated-password>
  ```

- [ ] **JWT secret configured** (for API authentication)
  ```bash
  # Generate JWT secret
  openssl rand -base64 64
  # Add to .env.production: JWT_SECRET=<generated-secret>
  ```

- [ ] **HTTPS/TLS certificates configured** (Let's Encrypt or commercial)
  ```bash
  # Check certificates exist
  ls -la deployment/ssl/
  # Should see: server.crt, server.key
  ```

- [ ] **Firewall rules configured** (only ports 80, 443 exposed)
  ```bash
  # Check open ports
  sudo ufw status
  # Should only show 80/tcp, 443/tcp ALLOW
  ```

### ✅ Infrastructure Requirements

- [ ] **S3 bucket created** for backups
  ```bash
  aws s3 mb s3://catalyst-prod-backups
  aws s3api put-bucket-versioning \
    --bucket catalyst-prod-backups \
    --versioning-configuration Status=Enabled
  ```

- [ ] **S3 encryption enabled**
  ```bash
  aws s3api put-bucket-encryption \
    --bucket catalyst-prod-backups \
    --server-side-encryption-configuration '{
      "Rules": [{
        "ApplyServerSideEncryptionByDefault": {
          "SSEAlgorithm": "AES256"
        }
      }]
    }'
  ```

- [ ] **IAM user created** for backup uploads
  ```bash
  aws iam create-user --user-name catalyst-backup
  aws iam attach-user-policy \
    --user-name catalyst-backup \
    --policy-arn arn:aws:iam::aws:policy/AmazonS3FullAccess
  aws iam create-access-key --user-name catalyst-backup
  # Save AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
  ```

- [ ] **Database initialized** with schema
  ```bash
  # Check migrations exist
  ls deployment/migrations/
  # Should see: 001_initial_schema.sql, etc.
  ```

- [ ] **DNS records configured**
  ```bash
  # Check A record points to server
  dig +short example.com
  # Should return server IP
  ```

- [ ] **CDN configured** (optional but recommended)
  ```bash
  # Cloudflare, Fastly, or AWS CloudFront
  # Should proxy traffic to origin server
  ```

### ✅ Configuration Validation

- [ ] **Environment file complete** (.env.production)
  ```bash
  # Run validation script
  ./verify-production-config.sh
  ```

- [ ] **Docker Compose syntax valid**
  ```bash
  docker-compose -f docker-compose.yml config >/dev/null
  echo "Exit code: $?" # Should be 0
  ```

- [ ] **No hardcoded secrets** in repository
  ```bash
  # Check for leaked secrets
  git secrets --scan
  # Or use: trufflehog --regex --entropy=True .
  ```

- [ ] **Log levels set to production** (INFO or WARN, not DEBUG)
  ```bash
  grep LOG_LEVEL .env.production
  # Should be: LOG_LEVEL=info or LOG_LEVEL=warn
  ```

- [ ] **Resource limits configured** (CPU, memory)
  ```bash
  # Check docker-compose.yml has deploy.resources.limits
  grep -A3 "limits:" docker-compose.yml
  ```

- [ ] **SDK caching re-enabled** (nginx config)
  ```bash
  # ⚠️ CRITICAL: Remove DEV-ONLY no-cache block before production
  # Edit nginx/conf.d/catalyst.conf and remove this section:
  #
  #   # DEV ONLY: Disable caching for SDK during development
  #   # TODO PRODUCTION: Remove this block before production deployment
  #   location ~* /assets/catalyst-sdk.*\.js$ {
  #       ...
  #   }
  #
  # The SDK should use the default /assets/ caching behavior:
  #   expires 1h;
  #   add_header Cache-Control "public, immutable";
  #
  # After removing, restart nginx:
  docker-compose restart nginx
  # Verify SDK is cached:
  curl -I https://ads.thenexusengine.com/assets/catalyst-sdk-v1.0.0.js | grep Cache-Control
  # Should show: Cache-Control: public, immutable
  ```

### ✅ Monitoring & Observability

- [ ] **Prometheus configured** and accessible
  ```bash
  curl -s http://localhost:9090/-/healthy
  # Should return: Prometheus is Healthy.
  ```

- [ ] **Grafana configured** with dashboards
  ```bash
  curl -s http://localhost:3000/api/health
  # Should return: {"commit":"...","database":"ok"}
  ```

- [ ] **Health check endpoints working**
  ```bash
  curl http://localhost:8000/health
  curl http://localhost:8000/health/ready
  # Both should return 200 OK
  ```

- [ ] **Log aggregation configured** (optional)
  ```bash
  # ELK stack, Loki, or CloudWatch Logs
  # Verify logs are being shipped
  ```

- [ ] **Alert rules configured** (PagerDuty, OpsGenie)
  ```bash
  # Check alertmanager config
  cat prometheus/alertmanager.yml
  ```

### ✅ Backup & Recovery

- [ ] **Backup service running**
  ```bash
  docker-compose ps backup
  # Should show: Up
  ```

- [ ] **Initial backup completed**
  ```bash
  docker exec catalyst-backup ls -lh /backups/latest/
  # Should see: latest.sql.gz
  ```

- [ ] **Backup uploaded to S3**
  ```bash
  aws s3 ls s3://catalyst-prod-backups/latest/
  # Should see: latest.sql.gz
  ```

- [ ] **Disaster recovery plan reviewed**
  ```bash
  cat deployment/DISASTER-RECOVERY.md
  # Team should be familiar with procedures
  ```

- [ ] **Restore tested** (in staging)
  ```bash
  # Verify restore works before going live
  FORCE=true ./restore-postgres.sh /backups/latest/latest.sql.gz
  ```

### ✅ Performance & Capacity

- [ ] **Load testing completed**
  ```bash
  # Run load tests with expected traffic + 50%
  # Check CPU, memory, response times under load
  ```

- [ ] **Database indexes created**
  ```bash
  docker exec catalyst-postgres psql -U catalyst -d catalyst -c "\di"
  # Should see indexes on: publishers(api_key), bidders(bidder_id)
  ```

- [ ] **Connection pooling configured**
  ```bash
  grep DB_MAX_CONNS .env.production
  # Should be set appropriately for workload
  ```

- [ ] **Rate limiting configured**
  ```bash
  grep RATE_LIMIT .env.production
  # Should be set to reasonable values
  ```

- [ ] **Cache TTLs optimized**
  ```bash
  grep REDIS_TTL .env.production
  # Should be set based on data freshness requirements
  ```

### ✅ Compliance & Privacy

- [ ] **Privacy middleware enabled**
  ```bash
  grep PBS_ENFORCE_GDPR .env.production
  # Should be: PBS_ENFORCE_GDPR=true
  grep PBS_ENFORCE_CCPA .env.production
  # Should be: PBS_ENFORCE_CCPA=true
  ```

- [ ] **IP anonymization enabled** (GDPR)
  ```bash
  grep PBS_ANONYMIZE_IP .env.production
  # Should be: PBS_ANONYMIZE_IP=true
  ```

- [ ] **Geo enforcement enabled**
  ```bash
  grep PBS_GEO_ENFORCEMENT .env.production
  # Should be: PBS_GEO_ENFORCEMENT=true
  ```

- [ ] **Privacy policy published**
  ```bash
  # Privacy policy should be accessible at /privacy
  curl https://example.com/privacy
  ```

- [ ] **Terms of service published**
  ```bash
  # Terms should be accessible at /terms
  curl https://example.com/terms
  ```

### ✅ Testing & Quality

- [ ] **All tests passing**
  ```bash
  go test ./...
  # Should show: PASS for all packages
  ```

- [ ] **Test coverage ≥80%**
  ```bash
  go test -cover ./...
  # Should show: coverage: 80%+ of statements
  ```

- [ ] **Linting clean**
  ```bash
  golangci-lint run
  # Should show: no issues found
  ```

- [ ] **Security scan clean**
  ```bash
  gosec ./...
  # Should show: no security issues found
  ```

- [ ] **Dependency vulnerabilities checked**
  ```bash
  go list -json -m all | nancy sleuth
  # Should show: no known vulnerabilities
  ```

### ✅ Documentation

- [ ] **API documentation published**
  ```bash
  # Swagger/OpenAPI docs accessible
  curl http://localhost:8000/swagger/doc.json
  ```

- [ ] **Runbook available** for on-call
  ```bash
  cat deployment/RUNBOOK.md
  # Should contain common issues and resolutions
  ```

- [ ] **Architecture diagram current**
  ```bash
  # Team should have access to latest architecture
  ```

- [ ] **Changelog updated**
  ```bash
  cat CHANGELOG.md
  # Should include all recent changes
  ```

---

## Deployment Steps

### Step 1: Pre-Deployment Verification

```bash
# Run automated verification
./deployment/verify-production-config.sh

# Expected output:
# ✅ All checks passed
# Ready for production deployment
```

### Step 2: Database Backup (Pre-Deploy)

```bash
# Take final backup before deployment
docker exec catalyst-backup /usr/local/bin/backup-postgres.sh

# Verify backup succeeded
docker exec catalyst-backup ls -lh /backups/latest/
```

### Step 3: Deploy Application

```bash
# Navigate to deployment directory
cd deployment

# Pull latest images
docker-compose pull

# Deploy with zero-downtime
docker-compose up -d --no-deps --build catalyst

# Wait for health check
until curl -sf http://localhost:8000/health/ready; do
  echo "Waiting for service..."
  sleep 2
done

echo "✅ Deployment successful"
```

### Step 4: Post-Deployment Verification

```bash
# Check all services running
docker-compose ps

# Verify health endpoints
curl http://localhost:8000/health
curl http://localhost:8000/health/ready

# Check logs for errors
docker-compose logs --tail=100 catalyst | grep -i error

# Verify metrics
curl http://localhost:8000/metrics | grep catalyst_
```

### Step 5: Smoke Tests

```bash
# Test critical endpoints
./deployment/smoke-tests.sh

# Expected output:
# ✅ Health check: PASS
# ✅ Auction endpoint: PASS
# ✅ Metrics endpoint: PASS
# ✅ Database connectivity: PASS
# ✅ Redis connectivity: PASS
```

### Step 6: Monitor for 1 Hour

```bash
# Watch logs in real-time
docker-compose logs -f catalyst

# Monitor metrics
# Open Grafana: http://localhost:3000
# Watch dashboard: "Catalyst Production Metrics"

# Check error rates
# Should be: < 1% error rate
```

### Step 7: Enable Traffic

```bash
# If using traffic splitting (blue-green)
# Gradually increase traffic to new deployment
# 5% → 25% → 50% → 100% over 1 hour

# Update nginx config or load balancer
# Monitor error rates at each step
```

---

## Rollback Procedure

If issues detected, rollback immediately:

```bash
# Stop new deployment
docker-compose stop catalyst

# Restore previous image
docker-compose up -d catalyst:previous-tag

# If database changes, restore backup
FORCE=true ./restore-postgres.sh /backups/daily/daily_YYYYMMDD.sql.gz

# Verify rollback
curl http://localhost:8000/health/ready
```

---

## Post-Deployment Tasks

- [ ] **Update status page** (if applicable)
- [ ] **Notify stakeholders** of successful deployment
- [ ] **Update changelog** with deployment notes
- [ ] **Schedule post-deployment review** (within 48 hours)
- [ ] **Monitor for 24 hours** for stability
- [ ] **Document any issues** encountered
- [ ] **Update runbook** with lessons learned

---

## Emergency Contacts

| Role | Contact | Escalation |
|------|---------|------------|
| DevOps Lead | TBD | TBD |
| Database Admin | TBD | TBD |
| Security Team | TBD | TBD |
| On-Call Engineer | TBD | PagerDuty |
| AWS Support | Create ticket | Critical priority |

---

## Success Criteria

Deployment is considered successful when:

- ✅ All health checks passing
- ✅ Error rate < 1%
- ✅ Response time p95 < 200ms
- ✅ CPU utilization < 70%
- ✅ Memory utilization < 80%
- ✅ No database errors
- ✅ Backups running successfully
- ✅ Monitoring and alerts active

---

## Known Issues & Workarounds

*(Document any known issues that don't block production)*

None currently.

---

**Deployment Sign-Off:**

- [ ] DevOps Lead: _________________ Date: _______
- [ ] Engineering Manager: _________ Date: _______
- [ ] Security Review: _____________ Date: _______

---

**Deployment Completed:**

- Date/Time: _________________
- Deployed By: _______________
- Version/Commit: ____________
- Issues Encountered: _________
