package processing

import "testing"

func TestDecideCrossCurrencyAccept(t *testing.T) {
	tests := []struct {
		name          string
		openInvoices  int
		detFiat       float64
		invoicePrice  float64
		wantAccept    bool
		wantDust      bool
		wantUnderpaid bool
	}{
		{
			name:         "single invoice fully covered auto-accepts",
			openInvoices: 1, detFiat: 161.56, invoicePrice: 150.00,
			wantAccept: true,
		},
		{
			name:         "single invoice exactly at price auto-accepts",
			openInvoices: 1, detFiat: 150.00, invoicePrice: 150.00,
			wantAccept: true,
		},
		{
			name:         "single invoice within tolerance auto-accepts",
			openInvoices: 1, detFiat: 149.95, invoicePrice: 150.00,
			wantAccept: true,
		},
		{
			name:          "single invoice fiat underpayment is rejected",
			openInvoices:  1, detFiat: 100.00, invoicePrice: 150.00,
			wantUnderpaid: true,
		},
		{
			name:         "zero open invoices never auto-accepts",
			openInvoices: 0, detFiat: 161.56, invoicePrice: 0,
			wantAccept: false,
		},
		{
			name:         "multiple open invoices never auto-accepts (ambiguous)",
			openInvoices: 2, detFiat: 161.56, invoicePrice: 150.00,
			wantAccept: false,
		},
		{
			name:         "sub-threshold dust is ignored, not alerted",
			openInvoices: 1, detFiat: 0.10, invoicePrice: 150.00,
			wantDust: true,
		},
		{
			name:         "dust floor takes precedence even with multiple invoices",
			openInvoices: 3, detFiat: 0.01, invoicePrice: 0,
			wantDust: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accept, dust, underpaid := decideCrossCurrencyAccept(tt.openInvoices, tt.detFiat, tt.invoicePrice)
			if accept != tt.wantAccept {
				t.Errorf("accept = %v, want %v", accept, tt.wantAccept)
			}
			if dust != tt.wantDust {
				t.Errorf("dust = %v, want %v", dust, tt.wantDust)
			}
			if underpaid != tt.wantUnderpaid {
				t.Errorf("underpaid = %v, want %v", underpaid, tt.wantUnderpaid)
			}
		})
	}
}
