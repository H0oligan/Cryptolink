package config

import (
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/olekukonko/tablewriter"
	"github.com/cryptolink/cryptolink/internal/auth"
	"github.com/cryptolink/cryptolink/internal/db/connection/pg"
	"github.com/cryptolink/cryptolink/internal/log"
	"github.com/cryptolink/cryptolink/internal/provider/bitcoin"
	"github.com/cryptolink/cryptolink/internal/provider/pricefeed"
	"github.com/cryptolink/cryptolink/internal/provider/rpc"
	"github.com/cryptolink/cryptolink/internal/provider/trongrid"
	"github.com/cryptolink/cryptolink/internal/server/http"
	"github.com/cryptolink/cryptolink/internal/service/evmcollector"
	"github.com/cryptolink/cryptolink/internal/service/processing"
	"github.com/cryptolink/cryptolink/internal/service/watcher"
	"github.com/cryptolink/cryptolink/internal/util"
	"github.com/samber/lo"
)

type Config struct {
	// compile-time parameters
	GitCommit     string
	GitVersion    string
	EmbedFrontend bool

	Env    string     `yaml:"env" env:"APP_ENV" env-default:"production" env-description:"Environment [production, local, sandbox]"`
	Debug  bool       `yaml:"debug" env:"APP_DEBUG" env-default:"false" env-description:"Enables debug mode"`
	Logger log.Config `yaml:"logger"`

	Oxygen Oxygen `yaml:"oxygen"`

	Providers Providers `yaml:"providers"`

	Notifications Notifications `yaml:"notifications"`

	Evm Evm `yaml:"evm"`
}

type Oxygen struct {
	Server       http.Config       `yaml:"server"`
	Auth         auth.Config       `yaml:"auth"`
	Postgres     pg.Config         `yaml:"postgres"`
	Processing   processing.Config `yaml:"processing"`
	Watcher      watcher.Config    `yaml:"watcher"`
	Subscription Subscription      `yaml:"subscription"`
}

type Subscription struct {
	AdminMerchantID int64 `yaml:"admin_merchant_id" env:"OXYGEN_SUBSCRIPTION_ADMIN_MERCHANT_ID" env-description:"Admin merchant ID for receiving subscription payments"`
}

type Providers struct {
	RPC       rpc.Config       `yaml:"rpc"`
	PriceFeed pricefeed.Config `yaml:"pricefeed"`
	Trongrid  trongrid.Config  `yaml:"trongrid"`
	Bitcoin   bitcoin.Config   `yaml:"bitcoin"`
}

type Notifications struct {
	SlackWebhookURL string `yaml:"slack_webhook_url" env:"NOTIFICATIONS_SLACK_WEBHOOK_URL" env-description:"Internal variable"`
}

// Evm holds EVM chain configuration for smart contract collector wallets.
type Evm struct {
	evmcollector.Config `yaml:",inline"`
}

var once = sync.Once{}
var cfg = &Config{}
var errCfg error

func New(gitCommit, gitVersion, configPath string, skipConfig, embedFrontend bool) (*Config, error) {
	once.Do(func() {
		cfg = &Config{
			GitCommit:     gitCommit,
			GitVersion:    gitVersion,
			EmbedFrontend: embedFrontend,
		}

		if skipConfig {
			errCfg = cleanenv.ReadEnv(cfg)
			return
		}

		errCfg = cleanenv.ReadConfig(configPath, cfg)
	})

	return cfg, errCfg
}

func PrintUsage(w io.Writer) error {
	desc, err := cleanenv.GetDescription(&Config{}, nil)
	if err != nil {
		return err
	}

	const delimiter = "||"

	// 1 line == 1 env var
	desc = strings.ReplaceAll(desc, "\n    \t", delimiter)

	lines := strings.Split(desc, "\n")

	// remove header
	lines = lines[1:]

	// hide internal vars
	lines = util.FilterSlice(lines, func(line string) bool {
		return !strings.Contains(strings.ToLower(line), "internal variable")
	})

	// remove duplicates
	lines = lo.Uniq(lines)

	// sort a-z (skip header)
	sort.Strings(lines[1:])

	// write as a table
	t := tablewriter.NewWriter(w)
	t.SetBorder(false)
	t.SetAutoWrapText(false)
	t.SetHeader([]string{"ENV", "Description"})
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)

	for _, line := range lines {
		cells := strings.Split(line, delimiter)
		cells = util.MapSlice(cells, strings.TrimSpace)
		t.Append(cells)
	}

	t.Render()

	return nil
}
