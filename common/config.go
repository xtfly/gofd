package common

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/xtfly/gokits/gcrypto"
	"github.com/xtfly/log4g"

	"gopkg.in/yaml.v2"
)

var (
	LOG = log4g.GetLogger("gofd")
)

// Config is struct maping the yaml configuration file
type Config struct {
	Server bool //是否为服务端
	Crypto *gcrypto.Crypto

	Name    string `yaml:"name"`
	DownDir string `yaml:"downdir,omitempty"` //只有客户端才配置
	Log     string `yaml:"log"`

	Net struct {
		IP       string `yaml:"ip"`
		MgntPort int    `yaml:"mgntPort"`
		DataPort int    `yaml:"dataPort"`

		AgentMgntPort int `yaml:"agentMgntPort,omitempty"`
		AgentDataPort int `yaml:"agentDataPort,omitempty"`

		TLS *struct {
			Cert string `yaml:"cert"`
			Key  string `yaml:"key"`
		} `yaml:"tls,omitempty"`
	} `yaml:"net"`

	Auth struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Factor   string `yaml:"factor"`
	} `yaml:"auth"`

	Control *Control `yaml:"control"`
}

// Control is some config item for controling the p2p session
type Control struct {
	Speed     int `yaml:"speed"`     // Unit: MiBps
	MaxActive int `yaml:"maxActive"` //
	CacheSize int `yaml:"cacheSize"` // Unit: MiB
}

func normalFile(dir string) string {
	if !filepath.IsAbs(dir) {
		pwd, _ := os.Getwd()
		dir = filepath.Join(pwd, dir)
		dir, _ = filepath.Abs(dir)
		dir = filepath.Clean(dir)
		return dir
	}
	return dir
}

func (c *Config) defaultValue() {
	c.DownDir = normalFile(c.DownDir)
	f, err := os.Stat(c.DownDir)
	if err == nil || !os.IsExist(err) {
		if err := os.MkdirAll(c.DownDir, os.ModePerm); err != nil {
			fmt.Printf("mkdir %s failed", c.DownDir)
			os.Exit(6)
		}
	} else {
		if !f.IsDir() {
			fmt.Printf("%s is not a directory", c.DownDir)
			os.Exit(6)
		}
	}

	if c.Log != "" {
		c.Log = normalFile(c.Log)
	}

	if c.Net.TLS != nil {
		c.Net.TLS.Cert = normalFile(c.Net.TLS.Cert)
		c.Net.TLS.Key = normalFile(c.Net.TLS.Key)
	}

	if c.Control == nil {
		c.Control = &Control{Speed: 10, MaxActive: 5, CacheSize: 25}
	}

	if c.Control.Speed == 0 {
		c.Control.Speed = 20
	}
	if c.Control.MaxActive == 0 {
		c.Control.MaxActive = 5
	}
	if c.Control.CacheSize == 0 {
		c.Control.CacheSize = 25
	}
}

func (c *Config) validate() error {
	if c.Server {
		if c.Net.AgentMgntPort == 0 {
			return errors.New("not set Net.AgentMgntPort in server config file")
		}
		if c.Net.AgentDataPort == 0 {
			return errors.New("not set Net.AgentDataPort in server config file")
		}
	}

	if !c.Server {
		if c.DownDir == "" {
			return errors.New("Not set DownDir in client config file")
		}
	}

	if c.Auth.Username == "" || c.Auth.Password == "" || c.Auth.Factor == "" {
		return errors.New("not set auth in config file")
	}

	var err error
	c.Crypto, err = gcrypto.NewCrypto(c.Auth.Factor)
	if err != nil {
		return err
	}
	c.Auth.Password, err = c.Crypto.DecryptStr(c.Auth.Password)
	if err != nil {
		return err
	}

	return nil
}

// ParserConfig return the Config instance when parse the configuration file
func ParserConfig(cfgfile string, server bool) (*Config, error) {
	ncfg := normalFile(cfgfile)
	bs, err := ioutil.ReadFile(ncfg)
	if err != nil {
		return nil, err
	}
	cfg := new(Config)
	cfg.Server = server
	if err := yaml.Unmarshal(bs, cfg); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.defaultValue()
	return cfg, nil
}
