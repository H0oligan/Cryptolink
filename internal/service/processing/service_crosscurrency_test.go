package processing

import "testing"

func TestChooseCrossCurrencyInvoice(t *testing.T) {
	tests := []struct {
		name     string
		detFiat  float64
		expected []float64
		wantIdx  int
		wantOK   bool
	}{
		{
			name:     "no open invoices cannot attribute",
			detFiat:  161.56,
			expected: nil,
			wantIdx:  -1, wantOK: false,
		},
		{
			name:     "single invoice is always the match (coverage gated by caller)",
			detFiat:  161.56,
			expected: []float64{150.00},
			wantIdx:  0, wantOK: true,
		},
		{
			name:     "single invoice matches even when received is far below it",
			detFiat:  10.00,
			expected: []float64{150.00},
			wantIdx:  0, wantOK: true,
		},
		{
			name:     "clear closest among two attributes to it",
			detFiat:  160.00,
			expected: []float64{150.00, 300.00},
			wantIdx:  0, wantOK: true,
		},
		{
			name:     "closest is the second invoice",
			detFiat:  290.00,
			expected: []float64{150.00, 300.00},
			wantIdx:  1, wantOK: true,
		},
		{
			name:     "two invoices of equal amount are indistinguishable",
			detFiat:  160.00,
			expected: []float64{150.00, 150.00},
			wantIdx:  -1, wantOK: false,
		},
		{
			name:     "two invoices closer than the margin refuse to attribute",
			detFiat:  100.00,
			expected: []float64{100.00, 101.00}, // gap 1.0 < margin 2.0 (2% of 100)
			wantIdx:  -1, wantOK: false,
		},
		{
			name:     "two invoices just past the margin attribute to the nearer",
			detFiat:  100.00,
			expected: []float64{100.00, 105.00}, // gap 5.0 >= margin 2.0
			wantIdx:  0, wantOK: true,
		},
		{
			name:     "small payment is protected by the $1 margin floor",
			detFiat:  10.00,
			expected: []float64{10.00, 10.50}, // gap 0.5 < floor 1.0
			wantIdx:  -1, wantOK: false,
		},
		{
			name:     "three invoices attribute to the middle closest",
			detFiat:  200.00,
			expected: []float64{150.00, 205.00, 400.00},
			wantIdx:  1, wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, ok := chooseCrossCurrencyInvoice(tt.detFiat, tt.expected)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if idx != tt.wantIdx {
				t.Errorf("idx = %d, want %d", idx, tt.wantIdx)
			}
		})
	}
}

func TestCrossCurrencyAmbiguityMargin(t *testing.T) {
	tests := []struct {
		name    string
		detFiat float64
		want    float64
	}{
		{name: "floor applies to small payments", detFiat: 10.00, want: 1.0},
		{name: "floor applies at zero", detFiat: 0.00, want: 1.0},
		{name: "2% scaling kicks in above the floor", detFiat: 100.00, want: 2.0},
		{name: "2% scaling for large payments", detFiat: 1000.00, want: 20.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := crossCurrencyAmbiguityMargin(tt.detFiat); got != tt.want {
				t.Errorf("crossCurrencyAmbiguityMargin(%.2f) = %.4f, want %.4f", tt.detFiat, got, tt.want)
			}
		})
	}
}
