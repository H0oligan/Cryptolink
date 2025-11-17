#!/bin/bash
#
# Oxygen Payment Gateway - VPS Setup Script
# Run this on a fresh Ubuntu 22.04 LTS VPS
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/YOUR_USERNAME/Cryptolink/main/scripts/vps-setup.sh | bash
#   or
#   bash vps-setup.sh
#

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
print_success() { echo -e "${GREEN}✓ $1${NC}"; }
print_error() { echo -e "${RED}✗ $1${NC}"; }
print_info() { echo -e "${YELLOW}➜ $1${NC}"; }

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   print_error "This script should NOT be run as root"
   print_info "Run as a regular user with sudo privileges"
   exit 1
fi

# Check if sudo is available
if ! command -v sudo &> /dev/null; then
    print_error "sudo is required but not installed"
    exit 1
fi

print_info "=== Oxygen Payment Gateway - VPS Setup ==="
echo ""

# Gather information
print_info "Please provide the following information:"
echo ""

read -p "Domain name (e.g., pay.example.com): " DOMAIN_NAME
read -p "Admin email address: " ADMIN_EMAIL
read -p "Database password for oxygen_user: " -s DB_PASSWORD
echo ""
read -p "Tatum API key: " TATUM_API_KEY
echo ""

print_info "Starting installation..."
echo ""

# Update system
print_info "Updating system packages..."
sudo apt update > /dev/null 2>&1
sudo DEBIAN_FRONTEND=noninteractive apt upgrade -y > /dev/null 2>&1
print_success "System updated"

# Install dependencies
print_info "Installing required packages..."
sudo DEBIAN_FRONTEND=noninteractive apt install -y \
    git build-essential curl wget vim \
    nginx postgresql postgresql-contrib \
    certbot python3-certbot-nginx \
    ufw fail2ban htop \
    > /dev/null 2>&1
print_success "Packages installed"

# Setup firewall
print_info "Configuring firewall..."
sudo ufw --force reset > /dev/null 2>&1
sudo ufw default deny incoming > /dev/null 2>&1
sudo ufw default allow outgoing > /dev/null 2>&1
sudo ufw allow 22/tcp > /dev/null 2>&1
sudo ufw allow 80/tcp > /dev/null 2>&1
sudo ufw allow 443/tcp > /dev/null 2>&1
sudo ufw --force enable > /dev/null 2>&1
print_success "Firewall configured"

# Install Go
print_info "Installing Go 1.21..."
cd /tmp
wget -q https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
rm go1.21.5.linux-amd64.tar.gz

# Add Go to PATH
if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
    echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
fi
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
print_success "Go installed"

# Install Node.js
print_info "Installing Node.js 18..."
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash - > /dev/null 2>&1
sudo apt install -y nodejs > /dev/null 2>&1
print_success "Node.js installed"

# Create directory structure
print_info "Creating directory structure..."
sudo mkdir -p /opt/oxygen/{bin,config,data/{kms,postgres-backup,monero-wallets},logs,src,scripts}
sudo chown -R $USER:$USER /opt/oxygen
print_success "Directories created"

# Setup PostgreSQL
print_info "Setting up PostgreSQL database..."
sudo systemctl start postgresql > /dev/null 2>&1
sudo systemctl enable postgresql > /dev/null 2>&1

sudo -u postgres psql > /dev/null 2>&1 << EOF
DROP DATABASE IF EXISTS oxygen;
DROP USER IF EXISTS oxygen_user;
CREATE DATABASE oxygen;
CREATE USER oxygen_user WITH PASSWORD '$DB_PASSWORD';
GRANT ALL PRIVILEGES ON DATABASE oxygen TO oxygen_user;
\c oxygen
GRANT ALL ON SCHEMA public TO oxygen_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO oxygen_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO oxygen_user;
EOF

print_success "PostgreSQL configured"

# Clone repository
print_info "Cloning repository..."
if [ -d "/opt/oxygen/src/Cryptolink" ]; then
    rm -rf /opt/oxygen/src/Cryptolink
fi

cd /opt/oxygen/src
# Note: Update this with your actual repository URL
git clone https://github.com/H0oligan/Cryptolink.git > /dev/null 2>&1 || {
    print_error "Failed to clone repository"
    print_info "Please update the repository URL in the script"
    exit 1
}

cd Cryptolink
print_success "Repository cloned"

# Create configuration file
print_info "Creating configuration file..."
cat > /opt/oxygen/config/oxygen.yml << EOF
logger:
  level: info

oxygen:
  postgres:
    dsn: "postgresql://oxygen_user:$DB_PASSWORD@localhost:5432/oxygen?sslmode=disable"

  server:
    port: 3000
    web_path: ""
    payment_path: ""

  processing:
    payment_expiration: 15m
    confirmations_threshold: 3

  auth:
    email_allowed:
      - "$ADMIN_EMAIL"
    google_oauth_enabled: false

kms:
  server:
    port: 3001
  store:
    path: "/opt/oxygen/data/kms/kms.db"

providers:
  tatum:
    api_key: "$TATUM_API_KEY"
    base_url: "https://api.tatum.io"
EOF

chmod 600 /opt/oxygen/config/oxygen.yml
print_success "Configuration created"

# Build frontend - Dashboard
print_info "Building Dashboard UI (this may take a few minutes)..."
cd /opt/oxygen/src/Cryptolink/ui-dashboard
npm install > /dev/null 2>&1
npm run build > /dev/null 2>&1
print_success "Dashboard UI built"

# Build frontend - Payment
print_info "Building Payment UI..."
cd /opt/oxygen/src/Cryptolink/ui-payment
npm install > /dev/null 2>&1
npm run build > /dev/null 2>&1
print_success "Payment UI built"

# Build backend
print_info "Building backend application..."
cd /opt/oxygen/src/Cryptolink
go mod download > /dev/null 2>&1
EMBED_FRONTEND=1 go build \
  -ldflags "-w -s -X 'main.embedFrontend=true'" \
  -o /opt/oxygen/bin/oxygen \
  main.go
chmod +x /opt/oxygen/bin/oxygen
print_success "Backend built"

# Run migrations
print_info "Running database migrations..."
/opt/oxygen/bin/oxygen migrate-up --config=/opt/oxygen/config/oxygen.yml > /dev/null 2>&1
print_success "Migrations completed"

# Create systemd services
print_info "Creating systemd services..."

# KMS Service
sudo tee /etc/systemd/system/oxygen-kms.service > /dev/null << EOF
[Unit]
Description=Oxygen Payment Gateway - KMS Server
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=/opt/oxygen/src/Cryptolink
ExecStart=/opt/oxygen/bin/oxygen serve-kms --config=/opt/oxygen/config/oxygen.yml
StandardOutput=append:/opt/oxygen/logs/oxygen-kms.log
StandardError=append:/opt/oxygen/logs/oxygen-kms.log
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Web Service
sudo tee /etc/systemd/system/oxygen-web.service > /dev/null << EOF
[Unit]
Description=Oxygen Payment Gateway - Web Server
After=network.target postgresql.service oxygen-kms.service
Requires=postgresql.service

[Service]
Type=simple
User=$USER
WorkingDirectory=/opt/oxygen/src/Cryptolink
ExecStart=/opt/oxygen/bin/oxygen serve-web --config=/opt/oxygen/config/oxygen.yml
StandardOutput=append:/opt/oxygen/logs/oxygen-web.log
StandardError=append:/opt/oxygen/logs/oxygen-web.log
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Scheduler Service
sudo tee /etc/systemd/system/oxygen-scheduler.service > /dev/null << EOF
[Unit]
Description=Oxygen Payment Gateway - Scheduler
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=$USER
WorkingDirectory=/opt/oxygen/src/Cryptolink
ExecStart=/opt/oxygen/bin/oxygen run-scheduler --config=/opt/oxygen/config/oxygen.yml
StandardOutput=append:/opt/oxygen/logs/oxygen-scheduler.log
StandardError=append:/opt/oxygen/logs/oxygen-scheduler.log
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable oxygen-kms oxygen-web oxygen-scheduler > /dev/null 2>&1
sudo systemctl start oxygen-kms
sleep 2
sudo systemctl start oxygen-web
sudo systemctl start oxygen-scheduler
print_success "Services created and started"

# Configure Nginx
print_info "Configuring Nginx..."
sudo tee /etc/nginx/sites-available/oxygen > /dev/null << EOF
upstream oxygen_web {
    server 127.0.0.1:3000;
}

server {
    listen 80;
    server_name $DOMAIN_NAME;

    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    location / {
        return 301 https://\$server_name\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name $DOMAIN_NAME;

    # SSL certificates will be configured by Certbot

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

sudo ln -sf /etc/nginx/sites-available/oxygen /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t > /dev/null 2>&1
sudo systemctl reload nginx
print_success "Nginx configured"

# Setup SSL
print_info "Setting up SSL certificate..."
sudo certbot --nginx -d $DOMAIN_NAME \
    --email $ADMIN_EMAIL \
    --agree-tos \
    --non-interactive \
    --redirect > /dev/null 2>&1 && print_success "SSL certificate installed" || print_info "SSL setup skipped (run certbot manually if needed)"

# Setup Fail2Ban
print_info "Configuring Fail2Ban..."
sudo tee /etc/fail2ban/jail.local > /dev/null << EOF
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
EOF

sudo systemctl enable fail2ban > /dev/null 2>&1
sudo systemctl restart fail2ban > /dev/null 2>&1
print_success "Fail2Ban configured"

# Create backup script
print_info "Setting up automated backups..."
cat > /opt/oxygen/scripts/backup-db.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="/opt/oxygen/data/postgres-backup"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
pg_dump -U oxygen_user oxygen | gzip > $BACKUP_DIR/backup_$TIMESTAMP.sql.gz
find $BACKUP_DIR -name "backup_*.sql.gz" -mtime +7 -delete
EOF

chmod +x /opt/oxygen/scripts/backup-db.sh

# Add to crontab
(crontab -l 2>/dev/null; echo "0 2 * * * /opt/oxygen/scripts/backup-db.sh >> /opt/oxygen/logs/backup.log 2>&1") | crontab -
print_success "Automated backups configured"

# Summary
echo ""
echo "════════════════════════════════════════════════════════════════"
print_success "Installation completed successfully!"
echo "════════════════════════════════════════════════════════════════"
echo ""
print_info "Access your application:"
echo "  Dashboard: https://$DOMAIN_NAME/dashboard"
echo "  API Docs:  https://$DOMAIN_NAME/api/docs"
echo "  Health:    https://$DOMAIN_NAME/api/v1/health"
echo ""
print_info "Service management:"
echo "  Status:  sudo systemctl status oxygen-web oxygen-kms oxygen-scheduler"
echo "  Logs:    sudo journalctl -u oxygen-web -f"
echo "  Restart: sudo systemctl restart oxygen-web"
echo ""
print_info "Important files:"
echo "  Config:  /opt/oxygen/config/oxygen.yml"
echo "  Logs:    /opt/oxygen/logs/"
echo "  Data:    /opt/oxygen/data/"
echo ""
print_info "Next steps:"
echo "  1. Test the application: curl https://$DOMAIN_NAME/api/v1/health"
echo "  2. Setup Google OAuth (if needed) in /opt/oxygen/config/oxygen.yml"
echo "  3. Configure additional blockchains in the dashboard"
echo "  4. Review logs: tail -f /opt/oxygen/logs/oxygen-web.log"
echo ""
print_info "Documentation:"
echo "  /opt/oxygen/src/Cryptolink/DEPLOYMENT_GUIDE.md"
echo "  /opt/oxygen/src/Cryptolink/DEPLOYMENT_QUICKSTART.md"
echo ""
echo "════════════════════════════════════════════════════════════════"
