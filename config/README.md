# Configuration

This directory is intentionally **empty** in the public repository. Real configuration files live here on each operator's own server and are gitignored.

## Files you will create here

### `cryptolink.yml` (required)

Main runtime config. Holds your DB DSN, RPC URLs, SMTP creds, HMAC secrets, and other operator-specific values. Generate it on first install — there is no shipped example because every value is operator-specific and several are sensitive.

To see the full list of supported keys and their environment-variable equivalents, after building the binary run:

```bash
./bin/cryptolink env
```

You can either set values in `cryptolink.yml` or override them with environment variables.

### `.env` (optional, recommended)

Used by `deploy.sh` for things you don't want even in `cryptolink.yml`:

- `SMTP_PASSWORD` — Brevo / Sendgrid / Mailgun password
- `DB_PASSWORD` — Postgres password (also referenced by `cryptolink.yml`)
- Anything else you'd rather keep out of YAML

Loaded into the deploy shell. Not loaded by the Go binary directly (use environment variables for that).

## What CryptoLink does NOT store on disk

- **Your private keys.** Never. Not in `cryptolink.yml`, not in `.env`, not anywhere on the server. Both for operators (deploying contracts) and merchants (owning collector clones), keys live in the browser wallet (MetaMask / TronLink) or on a hardware wallet.
- **Merchant xpub keys** are stored — but an xpub can only *derive* addresses, never spend. Your private key never reaches CryptoLink.

## Everything in this directory is gitignored

```
config/*.yml          # ignored
config/.env           # ignored
config/.env.*         # ignored
config/db_*           # ignored
```

The exception is this README. If you add anything else here, double-check it appears in `git status` as untracked / ignored before committing.
