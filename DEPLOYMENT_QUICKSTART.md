# 🚀 Quick Deployment Reference

**Copy-paste commands for rapid VPS deployment**

## 📋 Prerequisites
- Ubuntu 22.04 LTS VPS (4GB RAM, 2 CPU, 40GB SSD)
- Domain name pointed to VPS IP
- Root/sudo access

---

## 1️⃣ One-Command Initial Setup

```bash
# Run as root, then follow prompts
curl -fsSL https://raw.githubusercontent.com/YOUR_USERNAME/Cryptolink/main/scripts/vps-setup.sh | bash
```

**Or manual setup:**

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install all dependencies
sudo apt install -y git build-essential curl wget vim nginx postgresql postgresql-contrib certbot python3-certbot-nginx ufw fail2ban htop

# Setup firewall
sudo ufw allow 22/tcp && sudo ufw allow 80/tcp && sudo ufw allow 443/tcp && sudo ufw --force enable

# Install Go 1.21
cd /tmp && wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc && source ~/.bashrc

# Install Node.js 18
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt install -y nodejs
```

---

## 2️⃣ Create Directory Structure

```bash
# Create directories
sudo mkdir -p /opt/oxygen/{bin,config,data/{kms,postgres-backup,monero-wallets},logs,src}
sudo chown -R $USER:$USER /opt/oxygen
mkdir -p /opt/oxygen/scripts
```

---

## 3️⃣ Setup PostgreSQL

```bash
# Create database and user
sudo -u postgres psql << EOF
CREATE DATABASE oxygen;
CREATE USER oxygen_user WITH PASSWORD 'CHANGE_THIS_PASSWORD';
GRANT ALL PRIVILEGES ON DATABASE oxygen TO oxygen_user;
\c oxygen
GRANT ALL ON SCHEMA public TO oxygen_user;
\q
EOF

# Test connection
psql -U oxygen_user -d oxygen -h localhost
```

---

## 4️⃣ Clone & Build Application

```bash
# Clone repository
cd /opt/oxygen/src
git clone https://github.com/YOUR_USERNAME/Cryptolink.git
cd Cryptolink

# Install frontend dependencies and build
cd ui-dashboard && npm install && npm run build && cd ..
cd ui-payment && npm install && npm run build && cd ..

# Build backend with embedded frontends
EMBED_FRONTEND=1 go build -ldflags "-w -s -X 'main.embedFrontend=true'" -o /opt/oxygen/bin/oxygen main.go

# Make executable
chmod +x /opt/oxygen/bin/oxygen
```

---

## 5️⃣ Configuration

```bash
# Copy and edit config
cp /opt/oxygen/src/Cryptolink/config/oxygen.example.yml /opt/oxygen/config/oxygen.yml
vim /opt/oxygen/config/oxygen.yml

# IMPORTANT: Update these values:
# - oxygen.postgres.dsn (database connection)
# - providers.tatum.api_key (get from tatum.io)
# - auth.email_allowed (your admin email)
# - Google OAuth credentials (if using)

# Secure config
chmod 600 /opt/oxygen/config/oxygen.yml
```

---

## 6️⃣ Run Migrations

```bash
cd /opt/oxygen/src/Cryptolink
/opt/oxygen/bin/oxygen migrate-up --config=/opt/oxygen/config/oxygen.yml
```

---

## 7️⃣ Create Systemd Services

### Web Service
```bash
sudo tee /etc/systemd/system/oxygen-web.service << 'EOF'
[Unit]
Description=Oxygen Payment Gateway - Web Server
After=network.target postgresql.service

[Service]
Type=simple
User=YOUR_USERNAME
WorkingDirectory=/opt/oxygen/src/Cryptolink
ExecStart=/opt/oxygen/bin/oxygen serve-web --config=/opt/oxygen/config/oxygen.yml
StandardOutput=append:/opt/oxygen/logs/oxygen-web.log
StandardError=append:/opt/oxygen/logs/oxygen-web.log
Restart=always

[Install]
WantedBy=multi-user.target
EOF
```

### KMS Service
```bash
sudo tee /etc/systemd/system/oxygen-kms.service << 'EOF'
[Unit]
Description=Oxygen Payment Gateway - KMS Server
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
WorkingDirectory=/opt/oxygen/src/Cryptolink
ExecStart=/opt/oxygen/bin/oxygen serve-kms --config=/opt/oxygen/config/oxygen.yml
StandardOutput=append:/opt/oxygen/logs/oxygen-kms.log
StandardError=append:/opt/oxygen/logs/oxygen-kms.log
Restart=always

[Install]
WantedBy=multi-user.target
EOF
```

### Scheduler Service
```bash
sudo tee /etc/systemd/system/oxygen-scheduler.service << 'EOF'
[Unit]
Description=Oxygen Payment Gateway - Scheduler
After=network.target postgresql.service

[Service]
Type=simple
User=YOUR_USERNAME
WorkingDirectory=/opt/oxygen/src/Cryptolink
ExecStart=/opt/oxygen/bin/oxygen run-scheduler --config=/opt/oxygen/config/oxygen.yml
StandardOutput=append:/opt/oxygen/logs/oxygen-scheduler.log
StandardError=append:/opt/oxygen/logs/oxygen-scheduler.log
Restart=always

[Install]
WantedBy=multi-user.target
EOF
```

### Start All Services
```bash
# Replace YOUR_USERNAME with your actual username
sudo sed -i "s/YOUR_USERNAME/$USER/g" /etc/systemd/system/oxygen-*.service

# Reload and start
sudo systemctl daemon-reload
sudo systemctl enable oxygen-kms oxygen-web oxygen-scheduler
sudo systemctl start oxygen-kms oxygen-web oxygen-scheduler

# Check status
sudo systemctl status oxygen-kms oxygen-web oxygen-scheduler
```

---

## 8️⃣ Configure Nginx

```bash
# Create config (replace yourdomain.com)
sudo tee /etc/nginx/sites-available/oxygen << 'EOF'
upstream oxygen_web {
    server 127.0.0.1:3000;
}

server {
    listen 80;
    server_name yourdomain.com www.yourdomain.com;

    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    location / {
        return 301 https://$server_name$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com www.yourdomain.com;

    # SSL will be configured by Certbot

    add_header Strict-Transport-Security "max-age=31536000" always;

    location / {
        proxy_pass http://oxygen_web;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF

# Enable site
sudo ln -sf /etc/nginx/sites-available/oxygen /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default

# Test and reload
sudo nginx -t && sudo systemctl reload nginx
```

---

## 9️⃣ Setup SSL Certificate

```bash
# Replace with your domain and email
sudo certbot --nginx -d yourdomain.com -d www.yourdomain.com --email your@email.com --agree-tos --non-interactive

# Test auto-renewal
sudo certbot renew --dry-run
```

---

## 🔟 Verify Deployment

```bash
# Check services
sudo systemctl status oxygen-kms oxygen-web oxygen-scheduler

# View logs
tail -f /opt/oxygen/logs/oxygen-web.log

# Test endpoints
curl https://yourdomain.com/api/v1/health
curl https://yourdomain.com/dashboard
```

---

## 🔄 Daily Operations

### View Logs
```bash
# Real-time logs
sudo journalctl -u oxygen-web -f

# Last 100 lines
sudo journalctl -u oxygen-web -n 100 --no-pager

# All services
tail -f /opt/oxygen/logs/*.log
```

### Restart Services
```bash
# Restart all
sudo systemctl restart oxygen-kms oxygen-web oxygen-scheduler

# Restart specific service
sudo systemctl restart oxygen-web

# Reload Nginx
sudo systemctl reload nginx
```

### Database Backup
```bash
# Manual backup
pg_dump -U oxygen_user oxygen | gzip > /opt/oxygen/data/postgres-backup/backup_$(date +%Y%m%d).sql.gz

# Restore from backup
gunzip -c backup.sql.gz | psql -U oxygen_user oxygen
```

### Update Application
```bash
# Pull latest code
cd /opt/oxygen/src/Cryptolink
git pull

# Rebuild frontends
cd ui-dashboard && npm run build && cd ..
cd ui-payment && npm run build && cd ..

# Rebuild binary
EMBED_FRONTEND=1 go build -ldflags "-w -s" -o /opt/oxygen/bin/oxygen main.go

# Restart services
sudo systemctl restart oxygen-kms oxygen-web oxygen-scheduler
```

---

## 🆘 Troubleshooting

### Services won't start
```bash
# Check logs
sudo journalctl -u oxygen-web -n 50 --no-pager
sudo journalctl -u oxygen-kms -n 50 --no-pager

# Test manually
/opt/oxygen/bin/oxygen serve-web --config=/opt/oxygen/config/oxygen.yml
```

### Database connection issues
```bash
# Check PostgreSQL
sudo systemctl status postgresql

# Test connection
psql -U oxygen_user -d oxygen -h localhost

# Check config
grep -A5 "postgres:" /opt/oxygen/config/oxygen.yml
```

### Port already in use
```bash
# Find process using port 3000
sudo lsof -i :3000

# Kill process
sudo kill -9 <PID>
```

### Frontend not showing
```bash
# Check if frontends are built
ls -la /opt/oxygen/src/Cryptolink/ui-dashboard/dist/
ls -la /opt/oxygen/src/Cryptolink/ui-payment/dist/

# Rebuild
cd /opt/oxygen/src/Cryptolink/ui-dashboard && npm run build
cd /opt/oxygen/src/Cryptolink/ui-payment && npm run build

# Rebuild binary with embedded UIs
cd /opt/oxygen/src/Cryptolink
EMBED_FRONTEND=1 go build -o /opt/oxygen/bin/oxygen main.go

# Restart
sudo systemctl restart oxygen-web
```

---

## 📊 Monitoring

### Resource Usage
```bash
# CPU and Memory
htop

# Disk space
df -h

# Network
sudo nethogs

# Database size
sudo -u postgres psql -c "SELECT pg_size_pretty(pg_database_size('oxygen'));"
```

### Application Health
```bash
# Health endpoint
curl https://yourdomain.com/api/v1/health

# Service status
systemctl status oxygen-*

# Recent errors
grep -i error /opt/oxygen/logs/*.log | tail -20
```

---

## 🔐 Security Checklist

- [ ] Firewall (UFW) enabled and configured
- [ ] SSH key authentication only (disable password)
- [ ] Fail2Ban installed and running
- [ ] SSL/TLS certificate active
- [ ] Database accessible only from localhost
- [ ] Config files have proper permissions (600)
- [ ] Regular system updates enabled
- [ ] Strong passwords for all services
- [ ] Automated backups configured

---

## 📞 Getting Help

**Check logs first:**
```bash
sudo journalctl -u oxygen-web -n 100 --no-pager
tail -100 /opt/oxygen/logs/oxygen-web.log
```

**Useful files:**
- `/opt/oxygen/config/oxygen.yml` - Configuration
- `/opt/oxygen/logs/` - All logs
- `/etc/nginx/sites-available/oxygen` - Nginx config
- `/etc/systemd/system/oxygen-*.service` - Service definitions

**Documentation:**
- Full Guide: `/opt/oxygen/src/Cryptolink/DEPLOYMENT_GUIDE.md`
- Project Docs: `/opt/oxygen/src/Cryptolink/CLAUDE.md`
- Implementation Status: `/opt/oxygen/src/Cryptolink/IMPLEMENTATION_COMPLETE.md`

---

**Quick Start Version:** 1.0
**For Production Deployment** ✅
