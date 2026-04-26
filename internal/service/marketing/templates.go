package marketing

import "fmt"

// EmailTemplate represents a predefined marketing email template.
type EmailTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Subject     string `json:"subject"`
	BodyHTML    string `json:"body_html"`
}

// GetTemplates returns the 5 predefined marketing email templates.
func GetTemplates() []EmailTemplate {
	return []EmailTemplate{
		templateWelcome(),
		templateNonCustodial(),
		templateFreePlan(),
		templateSubscriptionVsFees(),
		templateEnterprise(),
	}
}

// GetTemplateByID returns a single template by its ID.
func GetTemplateByID(id string) *EmailTemplate {
	for _, t := range GetTemplates() {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

const emailWrapper = `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#050505;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
<div style="max-width:600px;margin:0 auto;padding:20px;">
  <div style="background:linear-gradient(135deg,#0a0a0a 0%%,#111111 100%%);border:1px solid #1e1e1e;border-radius:12px;overflow:hidden;">
    <div style="background:linear-gradient(90deg,#059669 0%%,#10b981 100%%);padding:24px 32px;">
      <h1 style="color:#fff;margin:0;font-size:22px;font-weight:700;letter-spacing:-0.5px;">⛓ CryptoLink</h1>
      <p style="color:rgba(255,255,255,0.85);margin:4px 0 0 0;font-size:13px;">%s</p>
    </div>
    <div style="padding:32px;">
      %s
    </div>
    <div style="padding:20px 32px;border-top:1px solid #1e1e1e;">
      <table style="width:100%%"><tr>
        <td style="text-align:center;">
          <a href="https://cryptolink.cc" style="display:inline-block;background:#10b981;color:#fff;padding:12px 28px;border-radius:8px;text-decoration:none;font-weight:600;font-size:14px;">Visit CryptoLink</a>
        </td>
      </tr></table>
      <p style="color:#64748b;font-size:12px;text-align:center;margin-top:16px;">© 2026 CryptoLink — Self-hosted, non-custodial crypto payments</p>
    </div>
  </div>
</div>
</body>
</html>`

func wrap(tagline, content string) string {
	return fmt.Sprintf(emailWrapper, tagline, content)
}

func templateWelcome() EmailTemplate {
	body := wrap("Decentralized Crypto Payments, Your Way", `
      <h2 style="color:#10b981;margin-top:0;font-size:20px;">Welcome to the Future of Crypto Payments</h2>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        CryptoLink is a <strong style="color:#10b981;">self-hosted, non-custodial</strong> crypto payment gateway designed for merchants who value control, privacy, and simplicity.
      </p>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        Unlike traditional payment processors, CryptoLink never holds your funds. Every payment goes <strong>directly to your wallet</strong> — no middleman, no delays, no risk of frozen accounts.
      </p>
      <div style="background:#0a0a0a;border:1px solid #1e1e1e;border-radius:8px;padding:20px;margin:20px 0;">
        <h3 style="color:#10b981;margin-top:0;font-size:16px;">What you get:</h3>
        <ul style="color:#94a3b8;font-size:14px;line-height:1.8;padding-left:20px;">
          <li><strong style="color:#e2e8f0;">7 blockchains</strong> — BTC, ETH, TRON, Polygon, BSC, Arbitrum, Avalanche</li>
          <li><strong style="color:#e2e8f0;">17 currencies</strong> — Including USDT and USDC on multiple chains</li>
          <li><strong style="color:#e2e8f0;">Non-custodial</strong> — Funds go directly to your wallet via smart contracts</li>
          <li><strong style="color:#e2e8f0;">Free plan available</strong> — Start accepting crypto with zero upfront cost</li>
          <li><strong style="color:#e2e8f0;">API + Payment links</strong> — Integrate easily or share payment links</li>
        </ul>
      </div>
      <p style="color:#94a3b8;font-size:14px;">Start accepting crypto payments in minutes. No KYC. No sign-up fees. Your keys, your coins.</p>
`)

	return EmailTemplate{
		ID:          "welcome",
		Name:        "Welcome to CryptoLink",
		Description: "Introduction to CryptoLink — what it is, key features, and why it's different.",
		Subject:     "Welcome to CryptoLink — Accept Crypto, Your Way",
		BodyHTML:    body,
	}
}

func templateNonCustodial() EmailTemplate {
	body := wrap("Your Keys, Your Coins — Always", `
      <h2 style="color:#10b981;margin-top:0;font-size:20px;">Why Non-Custodial Payments Matter</h2>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        Every month, centralized payment processors freeze merchant accounts, delay withdrawals, or impose surprise restrictions. With CryptoLink, <strong style="color:#10b981;">that can never happen</strong>.
      </p>
      <div style="display:flex;gap:16px;margin:20px 0;">
        <div style="flex:1;background:#0a0a0a;border:1px solid #1e1e1e;border-radius:8px;padding:16px;">
          <h4 style="color:#ef4444;margin-top:0;">❌ Custodial Gateways</h4>
          <ul style="color:#94a3b8;font-size:13px;line-height:1.7;padding-left:16px;">
            <li>They hold your funds</li>
            <li>Withdrawal delays (days/weeks)</li>
            <li>Account freezes without warning</li>
            <li>KYC/AML compliance overhead</li>
            <li>Counterparty risk</li>
          </ul>
        </div>
        <div style="flex:1;background:#0a0a0a;border:1px solid #10b981;border-radius:8px;padding:16px;">
          <h4 style="color:#10b981;margin-top:0;">✅ CryptoLink (Non-Custodial)</h4>
          <ul style="color:#94a3b8;font-size:13px;line-height:1.7;padding-left:16px;">
            <li>Funds go directly to your wallet</li>
            <li>Instant settlement on-chain</li>
            <li>No account freezes — it's your wallet</li>
            <li>Privacy-first: minimal data collection</li>
            <li>Zero counterparty risk</li>
          </ul>
        </div>
      </div>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        CryptoLink uses <strong>smart contract collectors</strong> on EVM and TRON chains, and <strong>xpub-derived addresses</strong> for Bitcoin. Every payment is verifiable on-chain, and you maintain full custody at all times.
      </p>
      <p style="color:#94a3b8;font-size:14px;">Your business. Your wallet. Your rules.</p>
`)

	return EmailTemplate{
		ID:          "non_custodial",
		Name:        "Why Non-Custodial Matters",
		Description: "Deep dive into the security and privacy benefits of non-custodial architecture.",
		Subject:     "Your Funds, Your Control — Why Non-Custodial Matters",
		BodyHTML:    body,
	}
}

func templateFreePlan() EmailTemplate {
	body := wrap("Start Accepting Crypto — Completely Free", `
      <h2 style="color:#10b981;margin-top:0;font-size:20px;">The CryptoLink Free Plan</h2>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        Not ready to commit? No problem. Our <strong style="color:#10b981;">Free plan</strong> gives you everything you need to start accepting crypto payments with <strong>zero cost</strong>.
      </p>
      <div style="background:#0a0a0a;border:1px solid #1e1e1e;border-radius:8px;padding:24px;margin:20px 0;text-align:center;">
        <h3 style="color:#10b981;font-size:32px;margin:0;">$0<span style="font-size:16px;color:#64748b;">/month</span></h3>
        <p style="color:#94a3b8;margin:8px 0 0 0;font-size:14px;">No credit card required. No hidden fees.</p>
      </div>
      <div style="background:#0a0a0a;border:1px solid #1e1e1e;border-radius:8px;padding:20px;margin:20px 0;">
        <h3 style="color:#e2e8f0;margin-top:0;font-size:16px;">What's included in the Free plan:</h3>
        <ul style="color:#94a3b8;font-size:14px;line-height:1.8;padding-left:20px;">
          <li>Up to <strong style="color:#e2e8f0;">$1,000</strong> monthly volume</li>
          <li><strong style="color:#e2e8f0;">1 merchant store</strong></li>
          <li>All 7 blockchains and 17 currencies</li>
          <li>Payment links + API access</li>
          <li>Smart contract collectors (non-custodial)</li>
          <li>Email notifications</li>
          <li><strong style="color:#10b981;">Zero transaction fees from CryptoLink</strong></li>
        </ul>
      </div>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        That's right — <strong>we don't take a cut of your sales</strong>. The only cost is your optional monthly subscription if you need higher limits. Start free, scale when you're ready.
      </p>
`)

	return EmailTemplate{
		ID:          "free_plan",
		Name:        "Free Plan Features",
		Description: "Highlight the free tier — what's included and why there's no catch.",
		Subject:     "Start Accepting Crypto for Free — No Hidden Fees",
		BodyHTML:    body,
	}
}

func templateSubscriptionVsFees() EmailTemplate {
	body := wrap("Save Money on Every Transaction", `
      <h2 style="color:#10b981;margin-top:0;font-size:20px;">Flat Subscription vs Per-Transaction Fees</h2>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        Most crypto payment processors charge <strong style="color:#ef4444;">1-3% per transaction</strong>. That means the more you sell, the more you pay. With CryptoLink, you pay a <strong style="color:#10b981;">flat monthly subscription</strong> — your transaction fees stay at <strong>zero</strong>.
      </p>
      <div style="background:#0a0a0a;border:1px solid #1e1e1e;border-radius:8px;padding:20px;margin:20px 0;">
        <h3 style="color:#e2e8f0;margin-top:0;font-size:16px;">Let's do the math:</h3>
        <table style="width:100%%;border-collapse:collapse;font-size:14px;margin-top:12px;">
          <tr style="border-bottom:1px solid #1e1e1e;">
            <td style="padding:10px;color:#64748b;">Monthly Volume</td>
            <td style="padding:10px;color:#ef4444;text-align:center;"><strong>Competitor (1.5%%)</strong></td>
            <td style="padding:10px;color:#10b981;text-align:center;"><strong>CryptoLink</strong></td>
          </tr>
          <tr style="border-bottom:1px solid #1e1e1e;">
            <td style="padding:10px;color:#e2e8f0;">$5,000</td>
            <td style="padding:10px;color:#ef4444;text-align:center;">$75/mo in fees</td>
            <td style="padding:10px;color:#10b981;text-align:center;">$9.99/mo flat</td>
          </tr>
          <tr style="border-bottom:1px solid #1e1e1e;">
            <td style="padding:10px;color:#e2e8f0;">$25,000</td>
            <td style="padding:10px;color:#ef4444;text-align:center;">$375/mo in fees</td>
            <td style="padding:10px;color:#10b981;text-align:center;">$29.99/mo flat</td>
          </tr>
          <tr style="border-bottom:1px solid #1e1e1e;">
            <td style="padding:10px;color:#e2e8f0;">$100,000</td>
            <td style="padding:10px;color:#ef4444;text-align:center;">$1,500/mo in fees</td>
            <td style="padding:10px;color:#10b981;text-align:center;">$79.99/mo flat</td>
          </tr>
          <tr>
            <td style="padding:10px;color:#e2e8f0;">$500,000</td>
            <td style="padding:10px;color:#ef4444;text-align:center;">$7,500/mo in fees</td>
            <td style="padding:10px;color:#10b981;text-align:center;">$199.99/mo flat</td>
          </tr>
        </table>
      </div>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        The savings are massive, especially as you grow. A merchant doing $100K/month saves <strong style="color:#10b981;">over $1,400/month</strong> with CryptoLink compared to a typical 1.5%% processor.
      </p>
      <p style="color:#94a3b8;font-size:14px;">Predictable costs. Zero surprises. Switch to a subscription model and keep more of what you earn.</p>
`)

	return EmailTemplate{
		ID:          "subscription_vs_fees",
		Name:        "Subscription vs Per-Transaction Fees",
		Description: "Compare CryptoLink's flat subscription to competitors' per-transaction fees with real math.",
		Subject:     "Stop Paying 1-3% Per Transaction — There's a Better Way",
		BodyHTML:    body,
	}
}

func templateEnterprise() EmailTemplate {
	body := wrap("Scale Without Limits", `
      <h2 style="color:#10b981;margin-top:0;font-size:20px;">Enterprise-Grade Crypto Payments</h2>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        CryptoLink grows with your business. From a freelancer accepting their first crypto payment to an enterprise processing millions — we have a plan that fits.
      </p>
      <div style="margin:20px 0;">
        <table style="width:100%%;border-collapse:collapse;font-size:13px;">
          <tr style="background:#0a0a0a;border-bottom:1px solid #1e1e1e;">
            <td style="padding:12px;color:#64748b;">Plan</td>
            <td style="padding:12px;color:#64748b;text-align:center;">Price</td>
            <td style="padding:12px;color:#64748b;text-align:center;">Volume</td>
            <td style="padding:12px;color:#64748b;text-align:center;">Stores</td>
          </tr>
          <tr style="border-bottom:1px solid #1e1e1e;">
            <td style="padding:12px;color:#e2e8f0;font-weight:600;">Free</td>
            <td style="padding:12px;color:#10b981;text-align:center;">$0</td>
            <td style="padding:12px;color:#94a3b8;text-align:center;">$1K/mo</td>
            <td style="padding:12px;color:#94a3b8;text-align:center;">1</td>
          </tr>
          <tr style="border-bottom:1px solid #1e1e1e;">
            <td style="padding:12px;color:#e2e8f0;font-weight:600;">Starter</td>
            <td style="padding:12px;color:#10b981;text-align:center;">$9.99</td>
            <td style="padding:12px;color:#94a3b8;text-align:center;">$10K/mo</td>
            <td style="padding:12px;color:#94a3b8;text-align:center;">3</td>
          </tr>
          <tr style="border-bottom:1px solid #1e1e1e;">
            <td style="padding:12px;color:#e2e8f0;font-weight:600;">Growth</td>
            <td style="padding:12px;color:#10b981;text-align:center;">$29.99</td>
            <td style="padding:12px;color:#94a3b8;text-align:center;">$50K/mo</td>
            <td style="padding:12px;color:#94a3b8;text-align:center;">10</td>
          </tr>
          <tr style="border-bottom:1px solid #1e1e1e;">
            <td style="padding:12px;color:#e2e8f0;font-weight:600;">Business</td>
            <td style="padding:12px;color:#10b981;text-align:center;">$79.99</td>
            <td style="padding:12px;color:#94a3b8;text-align:center;">$250K/mo</td>
            <td style="padding:12px;color:#94a3b8;text-align:center;">25</td>
          </tr>
          <tr>
            <td style="padding:12px;color:#e2e8f0;font-weight:600;">Enterprise</td>
            <td style="padding:12px;color:#10b981;text-align:center;">$199.99</td>
            <td style="padding:12px;color:#10b981;text-align:center;">Unlimited</td>
            <td style="padding:12px;color:#10b981;text-align:center;">Unlimited</td>
          </tr>
        </table>
      </div>
      <p style="color:#e2e8f0;font-size:15px;line-height:1.6;">
        Every plan includes <strong>zero transaction fees</strong>, all 17 supported cryptocurrencies across 7 blockchains, and full non-custodial architecture. The only difference is volume and store limits.
      </p>
      <div style="background:#0a0a0a;border:1px solid #10b981;border-radius:8px;padding:20px;margin:20px 0;text-align:center;">
        <h3 style="color:#10b981;margin-top:0;">Why pay more as you grow?</h3>
        <p style="color:#94a3b8;font-size:14px;">With CryptoLink, your costs stay predictable no matter how much you process. Process $1M/month for $199.99 — that's <strong style="color:#10b981;">0.02%%</strong> effective rate.</p>
      </div>
      <p style="color:#94a3b8;font-size:14px;">Ready to scale? Upgrade your plan in seconds from your merchant dashboard.</p>
`)

	return EmailTemplate{
		ID:          "enterprise",
		Name:        "Enterprise Growth",
		Description: "Showcase all plans with pricing table and the massive savings at scale.",
		Subject:     "Scale Your Crypto Payments — From Free to Enterprise",
		BodyHTML:    body,
	}
}

