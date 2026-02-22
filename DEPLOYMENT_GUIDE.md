# Complete VPS Deployment Guide - Cryptolink/OxygenPay

## 📋 Table of Contents
1. [Architecture Overview](#architecture-overview)
2. [VPS Requirements](#vps-requirements)
3. [Initial VPS Setup](#initial-vps-setup)
4. [Install Required Tools](#install-required-tools)
5. [Directory Structure](#directory-structure)
6. [Database Setup](#database-setup)
7. [Clone and Build Application](#clone-and-build-application)
8. [Configuration](#configuration)
9. [Build Frontend Applications](#build-frontend-applications)
10. [Build Backend Binary](#build-backend-binary)
11. [Setup Systemd Services](#setup-systemd-services)
12. [Nginx Reverse Proxy](#nginx-reverse-proxy)
13. [SSL/TLS Certificate](#ssltls-certificate)
14. [Monero Wallet RPC Setup](#monero-wallet-rpc-setup)
15. [Security Hardening](#security-hardening)
16. [Monitoring & Logging](#monitoring--logging)
17. [Backup Strategy](#backup-strategy)
18. [Deployment Checklist](#deployment-checklist)

---

## 1. Architecture Overview

### Recommended Architecture: **Monolith with Embedded UIs**

```
┌─────────────────────────────────────────────────────────────┐
│                         Your VPS                             │
│                                                              │
│  ┌────────────────┐                                         │
│  │   Nginx        │  :80, :443 (SSL/TLS)                   │
│  │  Reverse Proxy │                                         │
│  └────────┬───────┘                                         │
│           │                                                  │
│           ├──────► /api/*       → :3000 (Oxygen Web)       │
│           ├──────► /dashboard/* → :3000 (Dashboard UI)     │
│           └──────► /payment/*   → :3000 (Payment UI)       │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Oxygen Application (Single Go Binary)              │   │
│  │                                                      │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌───────────┐ │   │
│  │  │ Web Server   │  │ KMS Server   │  │ Scheduler │ │   │
│  │  │ Port: 3000   │  │ Port: 3001   │  │ (Cron)    │ │   │
│  │  └──────┬───────┘  └──────┬───────┘  └─────┬─────┘ │   │
│  └─────────┼─────────────────┼─────────────────┼───────┘   │
│            │                 │                 │            │
│            └────────┬────────┴─────────────────┘            │
│                     │                                        │
│  ┌──────────────────▼──────────────────┐                   │
│  │      PostgreSQL Database            │                   │
│  │      Port: 5432 (internal)          │                   │
│  └─────────────────────────────────────┘                   │
│                                                              │
│  ┌──────────────────────────────────────┐                  │
│  │  Monero Wallet RPC (Optional)        │                  │
│  │  Port: 18082 (internal)              │                  │
│  └──────────────────────────────────────┘                  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### Why This Architecture?

✅ **Single Binary Deployment**: Easy to manage, update, and deploy
✅ **Embedded Frontends**: No need to configure separate web servers
✅ **Efficient**: Lower resource usage, faster communication
✅ **Simplified**: One service to monitor, fewer moving parts

**Alternative**: You can separate services if you need horizontal scaling later.

---

## 2. VPS Requirements

### Minimum Specifications
- **CPU**: 2 cores
- **RAM**: 4 GB
- **Storage**: 40 GB SSD
- **OS**: Ubuntu 22.04 LTS (recommended) or 20.04 LTS
- **Network**: 1 Gbps connection

### Recommended Specifications (Production)
- **CPU**: 4 cores
- **RAM**: 8 GB
- **Storage**: 80 GB SSD
- **OS**: Ubuntu 22.04 LTS
- **Network**: 1 Gbps connection

### Domain Setup
You'll need:
- A domain name (e.g., `yourdomain.com`)
- DNS A record pointing to your VPS IP
- Subdomains (optional):
  - `api.yourdomain.com` - API endpoints
  - `dashboard.yourdomain.com` - Merchant dashboard
  - `pay.yourdomain.com` - Payment pages

---

## 3. Initial VPS Setup

### Step 1: Connect to VPS

```bash
# Replace with your VPS IP
ssh root@YOUR_VPS_IP
```

### Step 2: Create Non-Root User

```bash
# Create user
adduser oxygen

# Add to sudo group
usermod -aG sudo oxygen

# Switch to new user
su - oxygen
```

### Step 3: Setup SSH Key Authentication

```bash
# On your local machine
ssh-keygen -t ed25519 -C "your_email@example.com"

# Copy to VPS
ssh-copy-id oxygen@YOUR_VPS_IP

# Test connection
ssh oxygen@YOUR_VPS_IP
```

### Step 4: Update System

```bash
sudo apt update
sudo apt upgrade -y
sudo apt autoremove -y
```

### Step 5: Configure Firewall

```bash
# Install UFW
sudo apt install ufw -y

# Default policies
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH
sudo ufw allow 22/tcp

# Allow HTTP and HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Enable firewall
sudo ufw enable

# Check status
sudo ufw status verbose
```

---

## 4. Install Required Tools

### Install Go (1.20+)

```bash
# Download Go
cd /tmp
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz

# Extract
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export GOPATH=$HOME/go' >> ~/.bashrc
echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bashrc
source ~/.bashrc

# Verify
go version
```

### Install Node.js and npm (for frontend)

```bash
# Install Node.js 18.x LTS
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt install -y nodejs

# Verify
node --version
npm --version
```

### Install PostgreSQL

```bash
# Install PostgreSQL 15
sudo apt install postgresql postgresql-contrib -y

# Start and enable service
sudo systemctl start postgresql
sudo systemctl enable postgresql

# Check status
sudo systemctl status postgresql
```

### Install Git

```bash
sudo apt install git -y
git --version
```

### Install Nginx

```bash
sudo apt install nginx -y
sudo systemctl start nginx
sudo systemctl enable nginx
```

### Install Certbot (SSL/TLS)

```bash
sudo apt install certbot python3-certbot-nginx -y
```

### Install Additional Tools

```bash
# Development tools
sudo apt install -y build-essential curl wget vim

# Monitoring tools
sudo apt install -y htop iotop nethogs

# Log management
sudo apt install -y logrotate
```

---

## 5. Directory Structure

### Create Application Directories

```bash
# Create main application directory
sudo mkdir -p /opt/oxygen
sudo chown oxygen:oxygen /opt/oxygen

# Create subdirectories
mkdir -p /opt/oxygen/{bin,config,data,logs}
mkdir -p /opt/oxygen/data/{kms,postgres-backup,monero-wallets}

# Create systemd service directory (for user services)
mkdir -p ~/.config/systemd/user/
```

### Final Directory Structure

```
/opt/oxygen/
├── bin/                    # Compiled binaries
│   └── oxygen             # Main application binary
├── config/                 # Configuration files
│   ├── oxygen.yml         # Main config
│   └── nginx/             # Nginx configs
├── data/                   # Application data
│   ├── kms/               # KMS encrypted keys
│   ├── postgres-backup/   # Database backups
│   └── monero-wallets/    # Monero wallet files (if using)
├── logs/                   # Application logs
│   ├── oxygen-web.log
│   ├── oxygen-kms.log
│   └── oxygen-scheduler.log
└── src/                    # Source code (git clone)
    └── Cryptolink/
```

---

## 6. Database Setup

### Step 1: Configure PostgreSQL

```bash
# Switch to postgres user
sudo -u postgres psql
```

```sql
-- Create database
CREATE DATABASE oxygen;

-- Create user
CREATE USER oxygen_user WITH PASSWORD 'CHOOSE_STRONG_PASSWORD';

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE oxygen TO oxygen_user;

-- Grant schema privileges (PostgreSQL 15+)
\c oxygen
GRANT ALL ON SCHEMA public TO oxygen_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO oxygen_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO oxygen_user;

-- Exit
\q
```

### Step 2: Configure PostgreSQL for Local Access

```bash
# Edit postgresql.conf (optional - only if remote access needed)
sudo vim /etc/postgresql/15/main/postgresql.conf

# Find and uncomment:
# listen_addresses = 'localhost'

# Edit pg_hba.conf
sudo vim /etc/postgresql/15/main/pg_hba.conf

# Add this line (local unix socket connection):
# local   oxygen    oxygen_user                     scram-sha-256

# Restart PostgreSQL
sudo systemctl restart postgresql
```

### Step 3: Test Database Connection

```bash
# Test connection
psql -U oxygen_user -d oxygen -h localhost

# If successful, you'll see:
# oxygen=>

# Exit
\q
```

### Step 4: Setup Automated Backups

```bash
# Create backup script
vim /opt/oxygen/scripts/backup-db.sh
```

```bash
#!/bin/bash
# PostgreSQL Backup Script

BACKUP_DIR="/opt/oxygen/data/postgres-backup"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DB_NAME="oxygen"
DB_USER="oxygen_user"

# Create backup
pg_dump -U $DB_USER -d $DB_NAME | gzip > $BACKUP_DIR/oxygen_backup_$TIMESTAMP.sql.gz

# Keep only last 7 days
find $BACKUP_DIR -name "oxygen_backup_*.sql.gz" -mtime +7 -delete

echo "Backup completed: oxygen_backup_$TIMESTAMP.sql.gz"
```

```bash
# Make executable
chmod +x /opt/oxygen/scripts/backup-db.sh

# Add to crontab (daily at 2 AM)
crontab -e

# Add this line:
# 0 2 * * * /opt/oxygen/scripts/backup-db.sh >> /opt/oxygen/logs/backup.log 2>&1
```

---

## 7. Clone and Build Application

### Step 1: Clone Repository

```bash
cd /opt/oxygen
git clone https://github.com/YOUR_USERNAME/Cryptolink.git src/Cryptolink
cd src/Cryptolink
```

### Step 2: Checkout Your Branch

```bash
# List branches
git branch -a

# Checkout your feature branch (if needed)
git checkout claude/claude-md-mi0wzgo8k93ze7jj-01FXsMKiNDR5aWpb65zDWqGG

# Or create and checkout main deployment branch
git checkout -b production
```

---

## 8. Configuration

### Step 1: Create Configuration File

```bash
# Copy example config
cp /opt/oxygen/src/Cryptolink/config/oxygen.example.yml /opt/oxygen/config/oxygen.yml

# Edit configuration
vim /opt/oxygen/config/oxygen.yml
```

### Step 2: Configure oxygen.yml

```yaml
logger:
  level: info  # Use 'info' for production, 'debug' for troubleshooting

oxygen:
  postgres:
    dsn: "postgresql://oxygen_user:YOUR_PASSWORD@localhost:5432/oxygen?sslmode=disable"

  server:
    port: 3000
    # Leave web_path and payment_path empty - will use embedded UIs
    web_path: ""
    payment_path: ""

  processing:
    payment_expiration: 15m
    confirmations_threshold: 3

  auth:
    # Add admin email addresses
    email_allowed:
      - "admin@yourdomain.com"
    google_oauth_enabled: true
    google_oauth_client_id: "YOUR_GOOGLE_CLIENT_ID"
    google_oauth_client_secret: "YOUR_GOOGLE_CLIENT_SECRET"
    google_oauth_redirect_url: "https://yourdomain.com/auth/google/callback"

kms:
  server:
    port: 3001
  store:
    path: "/opt/oxygen/data/kms/kms.db"

providers:
  tatum:
    api_key: "YOUR_TATUM_API_KEY"
    base_url: "https://api.tatum.io"

  # Optional: Monero
  monero:
    wallet_rpc_endpoint: "http://localhost:18082/json_rpc"
    testnet_wallet_rpc_endpoint: "http://localhost:28082/json_rpc"
    rpc_username: "monero_user"
    rpc_password: "STRONG_PASSWORD"
    timeout: 60s
```

### Step 3: Secure Configuration File

```bash
# Set proper permissions
chmod 600 /opt/oxygen/config/oxygen.yml
chown oxygen:oxygen /opt/oxygen/config/oxygen.yml
```

### Step 4: Environment Variables (Optional Alternative)

If you prefer environment variables over config file:

```bash
# Create .env file
vim /opt/oxygen/config/.env
```

```bash
# Database
OXYGEN_POSTGRES_DSN="postgresql://oxygen_user:PASSWORD@localhost:5432/oxygen?sslmode=disable"

# Server
OXYGEN_SERVER_PORT=3000

# KMS
KMS_STORE_PATH="/opt/oxygen/data/kms/kms.db"
KMS_SERVER_PORT=3001

# Providers
PROVIDERS_TATUM_APIKEY="YOUR_TATUM_API_KEY"
PROVIDERS_TATUM_BASEURL="https://api.tatum.io"

# Monero (if using)
PROVIDERS_MONERO_WALLET_RPC_ENDPOINT="http://localhost:18082/json_rpc"
PROVIDERS_MONERO_RPC_USERNAME="monero_user"
PROVIDERS_MONERO_RPC_PASSWORD="STRONG_PASSWORD"

# Logging
LOGGER_LEVEL="info"
```

---

## 9. Build Frontend Applications

### Step 1: Build Dashboard UI

```bash
cd /opt/oxygen/src/Cryptolink/ui-dashboard

# Install dependencies
npm install

# Build for production
npm run build

# Verify build
ls -la dist/
```

### Step 2: Build Payment UI

```bash
cd /opt/oxygen/src/Cryptolink/ui-payment

# Install dependencies
npm install

# Build for production
npm run build

# Verify build
ls -la dist/
```

---

## 10. Build Backend Binary

### Step 1: Install Go Dependencies

```bash
cd /opt/oxygen/src/Cryptolink

# Download dependencies
go mod download

# Verify
go mod verify
```

### Step 2: Build Application Binary

```bash
cd /opt/oxygen/src/Cryptolink

# Build with embedded frontends
EMBED_FRONTEND=1 go build \
  -ldflags "-w -s \
    -X 'main.gitVersion=$(git describe --tags --always)' \
    -X 'main.gitCommit=$(git rev-parse HEAD)' \
    -X 'main.embedFrontend=true'" \
  -o /opt/oxygen/bin/oxygen \
  main.go

# Verify binary
/opt/oxygen/bin/oxygen --version

# Make executable
chmod +x /opt/oxygen/bin/oxygen
```

### Step 3: Run Database Migrations

```bash
cd /opt/oxygen/src/Cryptolink

# Run migrations
/opt/oxygen/bin/oxygen migrate-up --config=/opt/oxygen/config/oxygen.yml

# Verify
psql -U oxygen_user -d oxygen -c "\dt"
```

---

## 11. Setup Systemd Services

### Service 1: Oxygen Web Server

```bash
sudo vim /etc/systemd/system/oxygen-web.service
```

```ini
[Unit]
Description=Oxygen Payment Gateway - Web Server
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=oxygen
Group=oxygen
WorkingDirectory=/opt/oxygen/src/Cryptolink

# Main command
ExecStart=/opt/oxygen/bin/oxygen serve-web --config=/opt/oxygen/config/oxygen.yml

# Logging
StandardOutput=append:/opt/oxygen/logs/oxygen-web.log
StandardError=append:/opt/oxygen/logs/oxygen-web.log

# Restart policy
Restart=always
RestartSec=10

# Security
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### Service 2: Oxygen KMS Server

```bash
sudo vim /etc/systemd/system/oxygen-kms.service
```

```ini
[Unit]
Description=Oxygen Payment Gateway - KMS Server
After=network.target
Before=oxygen-web.service

[Service]
Type=simple
User=oxygen
Group=oxygen
WorkingDirectory=/opt/oxygen/src/Cryptolink

# Main command
ExecStart=/opt/oxygen/bin/oxygen serve-kms --config=/opt/oxygen/config/oxygen.yml

# Logging
StandardOutput=append:/opt/oxygen/logs/oxygen-kms.log
StandardError=append:/opt/oxygen/logs/oxygen-kms.log

# Restart policy
Restart=always
RestartSec=10

# Security
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### Service 3: Oxygen Scheduler

```bash
sudo vim /etc/systemd/system/oxygen-scheduler.service
```

```ini
[Unit]
Description=Oxygen Payment Gateway - Scheduler
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=oxygen
Group=oxygen
WorkingDirectory=/opt/oxygen/src/Cryptolink

# Main command
ExecStart=/opt/oxygen/bin/oxygen run-scheduler --config=/opt/oxygen/config/oxygen.yml

# Logging
StandardOutput=append:/opt/oxygen/logs/oxygen-scheduler.log
StandardError=append:/opt/oxygen/logs/oxygen-scheduler.log

# Restart policy
Restart=always
RestartSec=10

# Security
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### Enable and Start Services

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable services (start on boot)
sudo systemctl enable oxygen-kms
sudo systemctl enable oxygen-web
sudo systemctl enable oxygen-scheduler

# Start services
sudo systemctl start oxygen-kms
sudo systemctl start oxygen-web
sudo systemctl start oxygen-scheduler

# Check status
sudo systemctl status oxygen-kms
sudo systemctl status oxygen-web
sudo systemctl status oxygen-scheduler

# View logs
sudo journalctl -u oxygen-web -f
sudo journalctl -u oxygen-kms -f
sudo journalctl -u oxygen-scheduler -f
```

---

## 12. Nginx Reverse Proxy

### Step 1: Create Nginx Configuration

```bash
sudo vim /etc/nginx/sites-available/oxygen
```

```nginx
# Upstream servers
upstream oxygen_web {
    server 127.0.0.1:3000;
}

# HTTP to HTTPS redirect
server {
    listen 80;
    listen [::]:80;
    server_name yourdomain.com www.yourdomain.com;

    # Certbot challenge
    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    # Redirect all other traffic to HTTPS
    location / {
        return 301 https://$server_name$request_uri;
    }
}

# HTTPS server
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name yourdomain.com www.yourdomain.com;

    # SSL certificates (will be added by Certbot)
    # ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    # ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES256-GCM-SHA384;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Logging
    access_log /var/log/nginx/oxygen-access.log;
    error_log /var/log/nginx/oxygen-error.log;

    # Max upload size
    client_max_body_size 10M;

    # API endpoints
    location /api/ {
        proxy_pass http://oxygen_web;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Dashboard UI
    location /dashboard {
        proxy_pass http://oxygen_web;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    # Payment UI
    location /payment {
        proxy_pass http://oxygen_web;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    # Root
    location / {
        proxy_pass http://oxygen_web;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### Step 2: Enable Site and Test

```bash
# Create symlink
sudo ln -s /etc/nginx/sites-available/oxygen /etc/nginx/sites-enabled/

# Remove default site (optional)
sudo rm /etc/nginx/sites-enabled/default

# Test configuration
sudo nginx -t

# Reload Nginx
sudo systemctl reload nginx
```

---

## 13. SSL/TLS Certificate

### Get Let's Encrypt Certificate

```bash
# Stop nginx temporarily
sudo systemctl stop nginx

# Get certificate
sudo certbot certonly --standalone -d yourdomain.com -d www.yourdomain.com

# Start nginx
sudo systemctl start nginx

# Or use webroot method (nginx running)
sudo certbot --nginx -d yourdomain.com -d www.yourdomain.com

# Test auto-renewal
sudo certbot renew --dry-run
```

### Auto-Renewal Setup

Certbot automatically sets up renewal. Verify:

```bash
# Check systemd timer
sudo systemctl list-timers | grep certbot

# Manual renewal test
sudo certbot renew --dry-run
```

---

## 14. Monero Wallet RPC Setup

**(Only if using Monero)**

### Step 1: Download Monero

```bash
cd /tmp
wget https://downloads.getmonero.org/cli/linux64

# Extract
tar -xjf linux64

# Move to /opt
sudo mv monero-x86_64-linux-gnu-v0.18.3.1 /opt/monero

# Create symlinks
sudo ln -s /opt/monero/monerod /usr/local/bin/monerod
sudo ln -s /opt/monero/monero-wallet-rpc /usr/local/bin/monero-wallet-rpc
```

### Step 2: Create Systemd Service

```bash
sudo vim /etc/systemd/system/monero-wallet-rpc.service
```

```ini
[Unit]
Description=Monero Wallet RPC
After=network.target

[Service]
Type=simple
User=oxygen
Group=oxygen
WorkingDirectory=/opt/oxygen/data/monero-wallets

ExecStart=/usr/local/bin/monero-wallet-rpc \
  --rpc-bind-port 18082 \
  --rpc-bind-ip 127.0.0.1 \
  --rpc-login monero_user:STRONG_PASSWORD \
  --wallet-dir /opt/oxygen/data/monero-wallets \
  --daemon-address node.xmr.to:18081 \
  --log-level 2 \
  --log-file /opt/oxygen/logs/monero-wallet-rpc.log

Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Step 3: Start Service

```bash
sudo systemctl daemon-reload
sudo systemctl enable monero-wallet-rpc
sudo systemctl start monero-wallet-rpc
sudo systemctl status monero-wallet-rpc
```

---

## 15. Security Hardening

### 1. Fail2Ban (Prevent Brute Force)

```bash
# Install
sudo apt install fail2ban -y

# Configure
sudo vim /etc/fail2ban/jail.local
```

```ini
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 5

[sshd]
enabled = true
port = 22
logpath = /var/log/auth.log

[nginx-http-auth]
enabled = true
port = http,https
logpath = /var/log/nginx/*error.log
```

```bash
# Start service
sudo systemctl enable fail2ban
sudo systemctl start fail2ban

# Check status
sudo fail2ban-client status
```

### 2. Secure PostgreSQL

```bash
# Edit postgresql.conf
sudo vim /etc/postgresql/15/main/postgresql.conf

# Ensure:
listen_addresses = 'localhost'

# Restart
sudo systemctl restart postgresql
```

### 3. Regular Updates

```bash
# Enable unattended upgrades
sudo apt install unattended-upgrades -y
sudo dpkg-reconfigure -plow unattended-upgrades
```

### 4. File Permissions

```bash
# Secure data directories
chmod 700 /opt/oxygen/data/kms
chmod 700 /opt/oxygen/data/monero-wallets
chmod 600 /opt/oxygen/config/oxygen.yml

# Secure log files
chmod 640 /opt/oxygen/logs/*.log
```

---

## 16. Monitoring & Logging

### Setup Log Rotation

```bash
sudo vim /etc/logrotate.d/oxygen
```

```
/opt/oxygen/logs/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 oxygen oxygen
    sharedscripts
    postrotate
        systemctl reload oxygen-web oxygen-kms oxygen-scheduler
    endscript
}
```

### Monitor Services

```bash
# Create monitoring script
vim /opt/oxygen/scripts/monitor.sh
```

```bash
#!/bin/bash
# Check if services are running

SERVICES=("oxygen-web" "oxygen-kms" "oxygen-scheduler" "postgresql" "nginx")

for service in "${SERVICES[@]}"; do
    if ! systemctl is-active --quiet $service; then
        echo "[$service] is DOWN" | mail -s "Service Alert" admin@yourdomain.com
        systemctl restart $service
    fi
done
```

```bash
chmod +x /opt/oxygen/scripts/monitor.sh

# Add to crontab (every 5 minutes)
crontab -e
# */5 * * * * /opt/oxygen/scripts/monitor.sh
```

---

## 17. Backup Strategy

### Automated Backup Script

```bash
vim /opt/oxygen/scripts/full-backup.sh
```

```bash
#!/bin/bash
# Complete backup script

BACKUP_ROOT="/opt/oxygen/data/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="$BACKUP_ROOT/backup_$TIMESTAMP"

mkdir -p $BACKUP_DIR

# 1. Database backup
pg_dump -U oxygen_user oxygen | gzip > $BACKUP_DIR/database.sql.gz

# 2. KMS data
tar -czf $BACKUP_DIR/kms-data.tar.gz /opt/oxygen/data/kms/

# 3. Configuration
cp /opt/oxygen/config/oxygen.yml $BACKUP_DIR/

# 4. Monero wallets (if using)
if [ -d "/opt/oxygen/data/monero-wallets" ]; then
    tar -czf $BACKUP_DIR/monero-wallets.tar.gz /opt/oxygen/data/monero-wallets/
fi

# Keep only last 30 days
find $BACKUP_ROOT -type d -name "backup_*" -mtime +30 -exec rm -rf {} +

echo "Backup completed: $BACKUP_DIR"
```

```bash
chmod +x /opt/oxygen/scripts/full-backup.sh

# Schedule (daily at 3 AM)
crontab -e
# 0 3 * * * /opt/oxygen/scripts/full-backup.sh >> /opt/oxygen/logs/backup.log 2>&1
```

---

## 18. Deployment Checklist

### Pre-Deployment

- [ ] VPS provisioned with Ubuntu 22.04 LTS
- [ ] Domain name configured with DNS A record
- [ ] SSH key authentication setup
- [ ] Firewall (UFW) configured
- [ ] All required tools installed

### Application Setup

- [ ] PostgreSQL installed and configured
- [ ] Database created with proper user
- [ ] Application cloned from Git
- [ ] Configuration file created and secured
- [ ] Frontend applications built
- [ ] Backend binary compiled with embedded UIs
- [ ] Database migrations executed successfully

### Services Configuration

- [ ] Systemd services created (web, kms, scheduler)
- [ ] All services started and enabled
- [ ] Services running without errors
- [ ] Logs accessible and readable

### Web Server & SSL

- [ ] Nginx installed and configured
- [ ] Reverse proxy working
- [ ] SSL/TLS certificate obtained
- [ ] HTTPS working correctly
- [ ] HTTP to HTTPS redirect working

### Security

- [ ] Fail2Ban installed and configured
- [ ] File permissions secured
- [ ] Database access restricted to localhost
- [ ] Configuration files protected (chmod 600)
- [ ] Regular security updates enabled

### Monitoring & Backup

- [ ] Log rotation configured
- [ ] Service monitoring script setup
- [ ] Database backup scheduled
- [ ] Full system backup scheduled
- [ ] Backup restoration tested

### Optional (Monero)

- [ ] Monero wallet RPC installed
- [ ] Monero service running
- [ ] Wallet directory secured
- [ ] Backup includes wallet files

### Testing

- [ ] Access dashboard: https://yourdomain.com/dashboard
- [ ] Access API: https://yourdomain.com/api/v1/health
- [ ] Create test payment
- [ ] Test transaction on testnet
- [ ] Verify logs are being written
- [ ] Test backup restoration

### Production Launch

- [ ] Switch to mainnet API keys (Tatum)
- [ ] Update configuration to production mode
- [ ] Set logger level to 'info'
- [ ] Monitor logs for first 24 hours
- [ ] Verify transaction processing
- [ ] Setup external monitoring (UptimeRobot, etc.)

---

## Quick Command Reference

### Service Management
```bash
# Restart all services
sudo systemctl restart oxygen-web oxygen-kms oxygen-scheduler

# View logs in real-time
sudo journalctl -u oxygen-web -f

# Check service status
sudo systemctl status oxygen-*
```

### Database Operations
```bash
# Connect to database
psql -U oxygen_user -d oxygen

# Backup database
pg_dump -U oxygen_user oxygen > backup.sql

# Restore database
psql -U oxygen_user oxygen < backup.sql
```

### Application Updates
```bash
# Pull latest code
cd /opt/oxygen/src/Cryptolink
git pull origin production

# Rebuild frontend
cd ui-dashboard && npm run build
cd ../ui-payment && npm run build

# Rebuild binary
cd ..
EMBED_FRONTEND=1 go build -o /opt/oxygen/bin/oxygen main.go

# Restart services
sudo systemctl restart oxygen-*
```

### Monitoring
```bash
# Check disk space
df -h

# Check memory
free -h

# Check CPU
htop

# Check network
sudo nethogs

# View all logs
tail -f /opt/oxygen/logs/*.log
```

---

## Troubleshooting

### Services won't start
```bash
# Check logs
sudo journalctl -u oxygen-web -n 50 --no-pager
sudo journalctl -u oxygen-kms -n 50 --no-pager

# Check permissions
ls -la /opt/oxygen/data/kms/
ls -la /opt/oxygen/config/

# Test binary manually
/opt/oxygen/bin/oxygen serve-web --config=/opt/oxygen/config/oxygen.yml
```

### Database connection errors
```bash
# Verify PostgreSQL is running
sudo systemctl status postgresql

# Test connection
psql -U oxygen_user -d oxygen -h localhost

# Check configuration
grep "postgres" /opt/oxygen/config/oxygen.yml
```

### Nginx errors
```bash
# Test configuration
sudo nginx -t

# Check logs
sudo tail -f /var/log/nginx/error.log

# Restart Nginx
sudo systemctl restart nginx
```

---

## Support & Resources

- **Documentation**: /home/user/Cryptolink/CLAUDE.md
- **API Docs**: https://yourdomain.com/api/docs
- **Tatum Docs**: https://docs.tatum.io
- **Solana Docs**: https://docs.solana.com
- **Monero Docs**: https://www.getmonero.org/resources/developer-guides/

---

**Deployment Guide Version:** 1.0
**Last Updated:** 2025-11-17
**Ready for Production Deployment** ✅
