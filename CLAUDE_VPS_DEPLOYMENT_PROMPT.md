# Instructions for Claude: Complete VPS Deployment

You are running on a VPS and need to deploy the Cryptolink/OxygenPay payment gateway application.

## Current Environment
- Location: `/home/cryptolink/web/cryptolink.cc/public_html`
- Repository should be cloned here or nearby
- You have terminal access and can execute commands

## Your Mission
Deploy a production-ready crypto payment gateway with 9 blockchain support (Bitcoin, Ethereum, Polygon, BSC, TRON, Arbitrum, Avalanche, Solana, Monero).

## Step-by-Step Instructions

### Phase 1: Environment Assessment
First, understand what's already installed and what you need:

```bash
# Check current location
pwd
ls -la

# Check OS version
cat /etc/os-release

# Check what's installed
which go
which node
which npm
which psql
which nginx

# Check running services
systemctl status postgresql nginx 2>/dev/null || echo "Services not running"

# Check if repository exists
ls -la /home/cryptolink/ | grep -i crypto
```

### Phase 2: Install Missing Dependencies

Based on what's missing from Phase 1, install required tools:

**Install Go 1.21:**
```bash
cd /tmp
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

**Install Node.js 18:**
```bash
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt install -y nodejs
node --version
npm --version
```

**Install PostgreSQL:**
```bash
sudo apt update
sudo apt install -y postgresql postgresql-contrib
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

**Install Nginx:**
```bash
sudo apt install -y nginx
sudo systemctl start nginx
sudo systemctl enable nginx
```

**Install other tools:**
```bash
sudo apt install -y git build-essential curl wget vim
sudo apt install -y certbot python3-certbot-nginx
sudo apt install -y ufw fail2ban
```

### Phase 3: Setup Firewall

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw --force enable
sudo ufw status
```

### Phase 4: Directory Structure

Determine the best structure. Since you're in `/home/cryptolink/web/cryptolink.cc/public_html`, let's organize:

```bash
# Create application directories
sudo mkdir -p /opt/oxygen/{bin,config,data/{kms,postgres-backup},logs}
sudo chown -R cryptolink:cryptolink /opt/oxygen

# Or use the existing directory structure
mkdir -p ~/oxygen/{bin,config,data/{kms,postgres-backup},logs}
```

**IMPORTANT: Ask the user which approach they prefer:**
1. System-wide install: `/opt/oxygen/`
2. User install: `/home/cryptolink/oxygen/`

For now, I'll assume `/opt/oxygen/` but adjust if needed.

### Phase 5: PostgreSQL Setup

```bash
# Ask user for database password first
echo "I need a database password for the PostgreSQL user."
echo "Please provide a strong password:"
```

Then create database:

```bash
sudo -u postgres psql << 'EOF'
CREATE DATABASE oxygen;
CREATE USER oxygen_user WITH PASSWORD 'REPLACE_WITH_PASSWORD';
GRANT ALL PRIVILEGES ON DATABASE oxygen TO oxygen_user;
\c oxygen
GRANT ALL ON SCHEMA public TO oxygen_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO oxygen_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO oxygen_user;
\q
EOF

# Test connection
psql -U oxygen_user -d oxygen -h localhost -c "SELECT version();"
```

### Phase 6: Clone Repository

```bash
# Check if repository is already cloned
if [ -d "/home/cryptolink/web/cryptolink.cc/public_html/Cryptolink" ]; then
    cd /home/cryptolink/web/cryptolink.cc/public_html/Cryptolink
    git pull
elif [ -d "/opt/oxygen/src/Cryptolink" ]; then
    cd /opt/oxygen/src/Cryptolink
    git pull
else
    # Clone to appropriate location
    cd /opt/oxygen
    mkdir -p src
    cd src
    git clone https://github.com/H0oligan/Cryptolink.git
    cd Cryptolink
fi

# Checkout the correct branch
git fetch --all
git checkout claude/claude-md-mi0wzgo8k93ze7jj-01FXsMKiNDR5aWpb65zDWqGG
```

### Phase 7: Configuration

Ask user for these details:
1. Domain name (e.g., cryptolink.cc)
2. Admin email
3. Tatum API key (from https://tatum.io)
4. Database password (from Phase 5)

Then create config:

```bash
# Create configuration
cat > /opt/oxygen/config/oxygen.yml << 'EOFCONFIG'
logger:
  level: info

oxygen:
  postgres:
    dsn: "postgresql://oxygen_user:DATABASE_PASSWORD@localhost:5432/oxygen?sslmode=disable"

  server:
    port: 3000
    web_path: ""
    payment_path: ""

  processing:
    payment_expiration: 15m
    confirmations_threshold: 3

  auth:
    email_allowed:
      - "ADMIN_EMAIL"
    google_oauth_enabled: false

kms:
  server:
    port: 3001
  store:
    path: "/opt/oxygen/data/kms/kms.db"

providers:
  tatum:
    api_key: "TATUM_API_KEY"
    base_url: "https://api.tatum.io"
EOFCONFIG

# Replace placeholders with actual values
sed -i "s/DATABASE_PASSWORD/actual_password_here/g" /opt/oxygen/config/oxygen.yml
sed -i "s/ADMIN_EMAIL/actual_email_here/g" /opt/oxygen/config/oxygen.yml
sed -i "s/TATUM_API_KEY/actual_api_key_here/g" /opt/oxygen/config/oxygen.yml

# Secure config
chmod 600 /opt/oxygen/config/oxygen.yml
```

### Phase 8: Build Frontend Applications

```bash
cd /opt/oxygen/src/Cryptolink

# Build Dashboard UI
cd ui-dashboard
npm install
npm run build
ls -la dist/  # Verify build output

# Build Payment UI
cd ../ui-payment
npm install
npm run build
ls -la dist/  # Verify build output

cd ..
```

### Phase 9: Build Backend

```bash
cd /opt/oxygen/src/Cryptolink

# Download Go dependencies
go mod download
go mod verify

# Build with embedded frontends
EMBED_FRONTEND=1 go build \
  -ldflags "-w -s -X 'main.embedFrontend=true'" \
  -o /opt/oxygen/bin/oxygen \
  main.go

# Verify binary
chmod +x /opt/oxygen/bin/oxygen
/opt/oxygen/bin/oxygen --version
```

### Phase 10: Run Database Migrations

```bash
/opt/oxygen/bin/oxygen migrate-up --config=/opt/oxygen/config/oxygen.yml

# Verify tables were created
psql -U oxygen_user -d oxygen -h localhost -c "\dt"
```

### Phase 11: Create Systemd Services

**KMS Service:**
```bash
sudo tee /etc/systemd/system/oxygen-kms.service << 'EOF'
[Unit]
Description=Oxygen Payment Gateway - KMS Server
After=network.target

[Service]
Type=simple
User=cryptolink
WorkingDirectory=/opt/oxygen/src/Cryptolink
ExecStart=/opt/oxygen/bin/oxygen serve-kms --config=/opt/oxygen/config/oxygen.yml
StandardOutput=append:/opt/oxygen/logs/oxygen-kms.log
StandardError=append:/opt/oxygen/logs/oxygen-kms.log
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
```

**Web Service:**
```bash
sudo tee /etc/systemd/system/oxygen-web.service << 'EOF'
[Unit]
Description=Oxygen Payment Gateway - Web Server
After=network.target postgresql.service oxygen-kms.service
Requires=postgresql.service

[Service]
Type=simple
User=cryptolink
WorkingDirectory=/opt/oxygen/src/Cryptolink
ExecStart=/opt/oxygen/bin/oxygen serve-web --config=/opt/oxygen/config/oxygen.yml
StandardOutput=append:/opt/oxygen/logs/oxygen-web.log
StandardError=append:/opt/oxygen/logs/oxygen-web.log
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
```

**Scheduler Service:**
```bash
sudo tee /etc/systemd/system/oxygen-scheduler.service << 'EOF'
[Unit]
Description=Oxygen Payment Gateway - Scheduler
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=cryptolink
WorkingDirectory=/opt/oxygen/src/Cryptolink
ExecStart=/opt/oxygen/bin/oxygen run-scheduler --config=/opt/oxygen/config/oxygen.yml
StandardOutput=append:/opt/oxygen/logs/oxygen-scheduler.log
StandardError=append:/opt/oxygen/logs/oxygen-scheduler.log
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
```

**Start Services:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable oxygen-kms oxygen-web oxygen-scheduler
sudo systemctl start oxygen-kms
sleep 3
sudo systemctl start oxygen-web
sudo systemctl start oxygen-scheduler

# Check status
sudo systemctl status oxygen-kms
sudo systemctl status oxygen-web
sudo systemctl status oxygen-scheduler
```

### Phase 12: Configure Nginx

Ask user for their domain name, then:

```bash
DOMAIN="cryptolink.cc"  # Replace with actual domain

sudo tee /etc/nginx/sites-available/oxygen << EOF
upstream oxygen_web {
    server 127.0.0.1:3000;
}

server {
    listen 80;
    server_name $DOMAIN www.$DOMAIN;

    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    location / {
        return 301 https://\$server_name\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name $DOMAIN www.$DOMAIN;

    # SSL certificates will be added by Certbot

    add_header Strict-Transport-Security "max-age=31536000" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;

    client_max_body_size 10M;

    location / {
        proxy_pass http://oxygen_web;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_cache_bypass \$http_upgrade;
    }
}
EOF

# Enable site
sudo ln -sf /etc/nginx/sites-available/oxygen /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default

# Test and reload
sudo nginx -t
sudo systemctl reload nginx
```

### Phase 13: Setup SSL Certificate

```bash
# Get Let's Encrypt certificate
sudo certbot --nginx -d cryptolink.cc -d www.cryptolink.cc \
    --email admin@cryptolink.cc \
    --agree-tos \
    --non-interactive

# Test auto-renewal
sudo certbot renew --dry-run
```

### Phase 14: Verification

Run these checks and report results:

```bash
echo "=== Service Status ==="
sudo systemctl status oxygen-kms oxygen-web oxygen-scheduler

echo -e "\n=== Health Check ==="
curl -k https://localhost:3000/api/v1/health || curl http://localhost:3000/api/v1/health

echo -e "\n=== Database Check ==="
psql -U oxygen_user -d oxygen -h localhost -c "SELECT COUNT(*) FROM merchants;"

echo -e "\n=== Log Check ==="
tail -20 /opt/oxygen/logs/oxygen-web.log

echo -e "\n=== Public Access ==="
curl https://cryptolink.cc/api/v1/health

echo -e "\n=== Nginx Status ==="
sudo systemctl status nginx

echo -e "\n=== Listening Ports ==="
sudo ss -tulpn | grep -E ':(80|443|3000|3001|5432)'
```

### Phase 15: Setup Monitoring & Backups

**Backup script:**
```bash
cat > /opt/oxygen/scripts/backup-db.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="/opt/oxygen/data/postgres-backup"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
pg_dump -U oxygen_user oxygen | gzip > $BACKUP_DIR/backup_$TIMESTAMP.sql.gz
find $BACKUP_DIR -name "backup_*.sql.gz" -mtime +7 -delete
echo "Backup completed: backup_$TIMESTAMP.sql.gz"
EOF

chmod +x /opt/oxygen/scripts/backup-db.sh

# Schedule daily backups
(crontab -l 2>/dev/null; echo "0 2 * * * /opt/oxygen/scripts/backup-db.sh >> /opt/oxygen/logs/backup.log 2>&1") | crontab -
```

**Setup Fail2Ban:**
```bash
sudo apt install -y fail2ban
sudo systemctl enable fail2ban
sudo systemctl start fail2ban
```

### Phase 16: Final Report

Provide a complete summary:

```bash
cat << 'REPORT'
================================================================
                   DEPLOYMENT COMPLETED
================================================================

Application Status:
-------------------
✓ PostgreSQL database: oxygen
✓ User: oxygen_user
✓ Services running:
  - oxygen-kms (port 3001)
  - oxygen-web (port 3000)
  - oxygen-scheduler (background)

Access Points:
--------------
Dashboard: https://cryptolink.cc/dashboard
API Docs:  https://cryptolink.cc/api/docs
Health:    https://cryptolink.cc/api/v1/health

Blockchain Support:
-------------------
✓ Bitcoin (BTC)
✓ Ethereum (ETH)
✓ Polygon (MATIC)
✓ Binance Smart Chain (BSC)
✓ TRON (TRX)
✓ Arbitrum (ARB)
✓ Avalanche (AVAX)
✓ Solana (SOL)
✓ Monero (XMR)

Management Commands:
--------------------
View logs:    sudo journalctl -u oxygen-web -f
Restart:      sudo systemctl restart oxygen-web
Status:       sudo systemctl status oxygen-*
Database:     psql -U oxygen_user -d oxygen -h localhost

Files & Directories:
--------------------
Binary:       /opt/oxygen/bin/oxygen
Config:       /opt/oxygen/config/oxygen.yml
Logs:         /opt/oxygen/logs/
Data:         /opt/oxygen/data/
Source:       /opt/oxygen/src/Cryptolink/

Security:
---------
✓ Firewall (UFW) enabled
✓ SSL/TLS certificate active
✓ Fail2Ban running
✓ Database localhost-only

Next Steps:
-----------
1. Login to dashboard: https://cryptolink.cc/dashboard
2. Configure merchant account
3. Add payment methods
4. Test payment creation
5. Setup webhooks
6. Configure additional blockchains

Documentation:
--------------
Full Guide:        /opt/oxygen/src/Cryptolink/DEPLOYMENT_GUIDE.md
Quick Reference:   /opt/oxygen/src/Cryptolink/DEPLOYMENT_QUICKSTART.md
Implementation:    /opt/oxygen/src/Cryptolink/IMPLEMENTATION_COMPLETE.md

================================================================
REPORT
```

## Error Handling

If you encounter errors at any step:

1. **Share the exact error message**
2. **Show the relevant logs**: `tail -50 /opt/oxygen/logs/oxygen-web.log`
3. **Check service status**: `sudo systemctl status oxygen-web`
4. **Verify prerequisites**: Check that all dependencies are installed
5. **Ask for clarification** if you need information from the user

## Important Notes

- **Replace all placeholder values** (DATABASE_PASSWORD, ADMIN_EMAIL, TATUM_API_KEY, DOMAIN) with actual values from the user
- **Ask for user input** when needed - don't assume values
- **Verify each step** before proceeding to the next
- **Test thoroughly** before declaring success
- **Provide clear status updates** at each phase

## What to Ask the User

Before starting, confirm:
1. ✅ Domain name (e.g., cryptolink.cc)
2. ✅ Admin email address
3. ✅ Strong database password
4. ✅ Tatum API key (from https://tatum.io)
5. ✅ Preferred installation location (/opt/oxygen or /home/cryptolink/oxygen)

## Success Criteria

Deployment is successful when:
- ✅ All 3 services are running
- ✅ HTTPS is working
- ✅ Health endpoint responds: `https://cryptolink.cc/api/v1/health`
- ✅ Dashboard is accessible: `https://cryptolink.cc/dashboard`
- ✅ Database has tables
- ✅ Logs show no errors
- ✅ SSL certificate is valid

Good luck! Execute these steps carefully and report your progress.
