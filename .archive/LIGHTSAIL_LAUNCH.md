# 🚀 Launch Catalyst on Lightsail (Docker)

## Fastest Way to Production (30 minutes)

### What You'll Get
- ✅ Catalyst running in Docker
- ✅ Nginx reverse proxy with SSL
- ✅ Static IP in us-east-1 (near major SSPs)
- ✅ $20/month fixed cost
- ✅ Auto-restart on failure

---

## Quick Launch (3 Commands)

```bash
# 1. Create Lightsail instance
cd infrastructure/lightsail
./deploy.sh

# 2. Update DNS (use IP from output)
#    Point ads.thenexusengine.com → <STATIC_IP>

# 3. Deploy Docker containers
./deploy-docker.sh
```

That's it! Your ad server is live.

---

## Detailed Steps

### Step 1: Create Instance (3 min)

```bash
cd infrastructure/lightsail
./deploy.sh
```

**Output:**
```
✓ Instance created
✓ Static IP: 1.2.3.4
✓ SSH key saved
```

**Cost:** $20/month for 2GB RAM, 2 vCPU, 60GB SSD

### Step 2: Update DNS (5 min)

Go to your DNS provider and add:
```
Type: A
Name: ads.thenexusengine.com
Value: <IP_FROM_STEP_1>
TTL: 300
```

**Wait 5-10 minutes** for DNS propagation. Test with:
```bash
dig ads.thenexusengine.com
```

### Step 3: Deploy Catalyst (5 min)

```bash
./deploy-docker.sh
```

This will:
- Install Docker on Lightsail
- Build Catalyst image
- Start containers (Catalyst + Nginx)
- Configure networking

### Step 4: Setup SSL (2 min)

After DNS propagates:

```bash
# SSH to server
ssh -i ~/.ssh/lightsail-us-east-1.pem ec2-user@<IP>

# Get SSL certificate
cd ~/catalyst
docker-compose run --rm certbot certonly \
  --webroot \
  --webroot-path=/var/www/certbot \
  --email your@email.com \
  --agree-tos \
  -d ads.thenexusengine.com

# Restart nginx to load certificate
docker-compose restart nginx
```

### Step 5: Test (2 min)

```bash
# Health check
curl https://ads.thenexusengine.com/health

# Bid request
curl -X POST https://ads.thenexusengine.com/v1/bid \
  -H "Content-Type: application/json" \
  -d '{"accountId":"icisic-media","timeout":2800,"slots":[{"divId":"test","sizes":[[728,90]],"adUnitPath":"totalprosports.com/leaderboard"}]}'

# Browser
open https://ads.thenexusengine.com/test-magnite.html
```

---

## Management Commands

### SSH Access
```bash
ssh -i ~/.ssh/lightsail-us-east-1.pem ec2-user@<IP>
```

### View Logs
```bash
cd ~/catalyst
docker-compose logs -f catalyst
```

### Restart
```bash
docker-compose restart catalyst
```

### Update Code
```bash
# Build new image locally
docker build -t catalyst-server:latest .
docker save catalyst-server:latest | gzip > /tmp/catalyst-image.tar.gz

# Upload and reload
scp -i ~/.ssh/lightsail-us-east-1.pem /tmp/catalyst-image.tar.gz ec2-user@<IP>:~/catalyst/
ssh -i ~/.ssh/lightsail-us-east-1.pem ec2-user@<IP>
cd ~/catalyst
docker load < catalyst-image.tar.gz
docker-compose up -d
```

---

## Why Lightsail + Docker?

| Feature | Benefit |
|---------|---------|
| **Fixed Price** | $20/month, no surprises |
| **Docker** | Easy updates, isolated services |
| **Static IP** | Free, persistent IP address |
| **Backups** | One-click snapshots |
| **Simple** | No complex networking |
| **Location** | us-east-1 (5-15ms to SSPs) |

---

## Troubleshooting

### Can't SSH?
```bash
# Re-download key
aws lightsail download-default-key-pair --region us-east-1 --query 'privateKeyBase64' --output text | base64 -d > ~/.ssh/lightsail-us-east-1.pem
chmod 400 ~/.ssh/lightsail-us-east-1.pem
```

### Service not starting?
```bash
ssh -i ~/.ssh/lightsail-us-east-1.pem ec2-user@<IP>
cd ~/catalyst
docker-compose logs --tail=100
```

### SSL not working?
```bash
# Make sure DNS is pointing to correct IP
dig ads.thenexusengine.com

# Check certificate
docker-compose exec nginx ls -la /etc/letsencrypt/live/
```

---

## Cost Breakdown

| Item | Cost |
|------|------|
| Lightsail 2GB | $20/month |
| Static IP | $0 (included) |
| Data Transfer | $0 (1TB free) |
| SSL | $0 (Let's Encrypt) |
| **Total** | **$20/month** |

Compare to EC2: ~$40-60/month for similar setup!

---

## Success Checklist

- [ ] Lightsail instance created
- [ ] Static IP assigned
- [ ] DNS updated and propagated
- [ ] Docker containers running
- [ ] SSL certificate installed
- [ ] Health check returns 200
- [ ] Bid endpoint works
- [ ] Logs show no errors

---

## Ready?

```bash
cd infrastructure/lightsail
./deploy.sh
```

**Total time:** ~30 minutes to production 🚀

**Cost:** $20/month

**Performance:** 5-15ms to major SSPs
