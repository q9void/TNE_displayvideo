# 🚀 LAUNCH CATALYST TO AWS

Quick guide to get Catalyst live in AWS us-east-1 (near major SSPs).

## Prerequisites Check

```bash
# 1. Terraform installed?
terraform version
# If not: brew install terraform

# 2. AWS CLI configured?
aws sts get-caller-identity
# If not: aws configure

# 3. Deployment package ready?
ls -lh build/catalyst-deployment.tar.gz
# If not: tar czf build/catalyst-deployment.tar.gz build/catalyst-server assets/*.js config/*.json
```

---

## Option 1: Quick Deploy (Automated) ⚡

### Step 1: Get Your IP Address
```bash
curl https://whatismyip.com
# Copy the IP address shown
```

### Step 2: Configure AWS
```bash
cd infrastructure/aws
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:
```hcl
ssh_key_name = "catalyst-key"    # Change this to your key name
your_ip      = "YOUR_IP/32"      # Paste your IP from step 1
```

### Step 3: Create SSH Key (if needed)
```bash
# Check if you have a key
ls ~/.ssh/*.pem

# If not, create one:
aws ec2 create-key-pair --region us-east-1 --key-name catalyst-key --query 'KeyMaterial' --output text > ~/.ssh/catalyst-key.pem
chmod 400 ~/.ssh/catalyst-key.pem
```

### Step 4: Deploy Infrastructure
```bash
./deploy.sh
```

This will:
- ✅ Create EC2 instance in us-east-1
- ✅ Assign Elastic IP
- ✅ Configure security groups
- ✅ Install Nginx + Go
- ✅ Setup systemd service

**Time:** ~5 minutes

### Step 5: Update DNS

Copy the Elastic IP from the output, then update DNS:
```
A Record: ads.thenexusengine.com → <ELASTIC_IP>
```

Wait 5-10 minutes for DNS propagation.

### Step 6: Deploy Catalyst

```bash
# Get the IP from Terraform output
PUBLIC_IP=$(cd infrastructure/aws && terraform output -raw public_ip)

# Upload Catalyst
cd ../..  # Back to project root
scp -i ~/.ssh/catalyst-key.pem build/catalyst-deployment.tar.gz ubuntu@$PUBLIC_IP:/tmp/

# SSH and deploy
ssh -i ~/.ssh/catalyst-key.pem ubuntu@$PUBLIC_IP << 'REMOTE'
sudo tar xzf /tmp/catalyst-deployment.tar.gz -C /opt/catalyst --strip-components=1
sudo chown -R catalyst:catalyst /opt/catalyst
sudo chmod +x /opt/catalyst/catalyst-server
sudo systemctl start catalyst
sudo systemctl status catalyst
REMOTE
```

### Step 7: Setup SSL

```bash
ssh -i ~/.ssh/catalyst-key.pem ubuntu@$PUBLIC_IP

# On server - replace with your email
sudo certbot --nginx -d ads.thenexusengine.com --non-interactive --agree-tos --email your@email.com
```

### Step 8: Test

```bash
# Health check
curl https://ads.thenexusengine.com/health

# Bid request
curl -X POST https://ads.thenexusengine.com/v1/bid \
  -H "Content-Type: application/json" \
  -d '{"accountId":"icisic-media","timeout":2800,"slots":[{"divId":"test","sizes":[[728,90]],"adUnitPath":"totalprosports.com/leaderboard"}]}'

# Browser test
open https://ads.thenexusengine.com/test-magnite.html
```

---

## Option 2: Manual Setup (Step-by-step) 📋

If you prefer manual control:

### 1. Check Current AWS Config
```bash
aws configure list
```

Make sure you have an AWS profile with EC2 permissions. If not:
```bash
aws configure --profile catalyst
# Enter: Access Key ID
# Enter: Secret Access Key
# Region: us-east-1
# Output: json

# Use this profile
export AWS_PROFILE=catalyst
```

### 2. Deploy with Terraform
```bash
cd infrastructure/aws

# Initialize
terraform init

# Preview what will be created
terraform plan

# Deploy (will ask for confirmation)
terraform apply
```

### 3. Get Connection Info
```bash
terraform output
```

### 4. Continue from Step 5 in Option 1 above

---

## Quick Reference

### SSH to Server
```bash
PUBLIC_IP=$(cd infrastructure/aws && terraform output -raw public_ip)
ssh -i ~/.ssh/catalyst-key.pem ubuntu@$PUBLIC_IP
```

### Check Catalyst Status
```bash
ssh -i ~/.ssh/catalyst-key.pem ubuntu@$PUBLIC_IP 'sudo systemctl status catalyst'
```

### View Logs
```bash
ssh -i ~/.ssh/catalyst-key.pem ubuntu@$PUBLIC_IP 'sudo journalctl -u catalyst -f'
```

### Restart Catalyst
```bash
ssh -i ~/.ssh/catalyst-key.pem ubuntu@$PUBLIC_IP 'sudo systemctl restart catalyst'
```

---

## Costs

**Monthly estimate:** ~$35
- EC2 t3.medium: $30
- Storage (30GB): $3
- Data transfer: ~$2/TB

---

## What You're Getting

### Server Specs
- **Location:** AWS us-east-1 (N. Virginia) - 5-15ms to major SSPs
- **Instance:** t3.medium (2 vCPU, 4GB RAM)
- **OS:** Ubuntu 22.04 LTS
- **Storage:** 30GB SSD

### Pre-configured
- ✅ Go 1.21
- ✅ Nginx reverse proxy
- ✅ SSL with Certbot
- ✅ Systemd service (auto-restart)
- ✅ Security groups
- ✅ Elastic IP (static)

### Ports Open
- 22: SSH (your IP only)
- 80: HTTP redirect to HTTPS
- 443: HTTPS
- 8000: Catalyst (also via Nginx)

---

## Troubleshooting

### "Permission denied" when using SSH key
```bash
chmod 400 ~/.ssh/catalyst-key.pem
```

### Can't connect to server
```bash
# Check if instance is running
cd infrastructure/aws
terraform show | grep instance_state

# Check your IP (might have changed)
curl https://whatismyip.com
# Update terraform.tfvars with new IP and run: terraform apply
```

### Terraform "key pair not found"
```bash
# List your key pairs in us-east-1
aws ec2 describe-key-pairs --region us-east-1

# Create if missing
aws ec2 create-key-pair --region us-east-1 --key-name catalyst-key --query 'KeyMaterial' --output text > ~/.ssh/catalyst-key.pem
chmod 400 ~/.ssh/catalyst-key.pem
```

---

## Success Checklist

- [ ] Infrastructure deployed (Terraform)
- [ ] DNS updated and propagated
- [ ] Catalyst binary deployed
- [ ] SSL certificate installed
- [ ] Health check returns 200
- [ ] Bid endpoint works
- [ ] Browser test successful
- [ ] Logs show no errors
- [ ] Metrics endpoint accessible

---

## Ready to Launch?

```bash
cd infrastructure/aws
./deploy.sh
```

**Total time:** 20-30 minutes from start to live 🚀
