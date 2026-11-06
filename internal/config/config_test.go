package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckAppSection(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "section present", content: "logger:\n  level: info\ncryptolink:\n  server:\n    port: 3000\n"},
		{name: "section missing", content: "logger:\n  level: info\nkms:\n  store:\n    path: /tmp/kms.db\n", wantErr: true},
		{name: "nested key does not count", content: "other:\n  cryptolink: true\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			assert.NoError(t, os.WriteFile(path, []byte(tc.content), 0o600))

			err := checkAppSection(path)
			assert.Equal(t, tc.wantErr, err != nil)
		})
	}
}
