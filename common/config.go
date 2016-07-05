package common

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/xtfly/gokits"

	"gopkg.in/yaml.v2"
)

// 定义配置映射的结构体
type Config struct {
	Server bool //是否为服务端
	Crypto *gokits.Crypto

	Name string `yaml:"name"`

	DownDir string `yaml:"downdir,omitempty"` //只有客户端才配置

	Log string `yaml:"log"`

	Net struct {
		IP       string `yaml:"ip"`
		MgntPort int    `yaml:"mgntPort"`
		DataPort int    `yaml:"dataPort"`

		AgentMgntPort int `yaml:"agentMgntPort,omitempty"`
		AgentDataPort int `yaml:"agentDataPort,omitempty"`

		Tls *struct {
			Cert string `yaml:"cert"`
			Key  string `yaml:"key"`
		} `yaml:"tls,omitempty"`
	} `yaml:"net"`

	Auth struct {
		Username string `yaml:"username"`
		Passowrd string `yaml:"passowrd"`
		Factor   string `yaml:"factor"`
		Crc      string `yaml:"crc"`
	} `yaml:"auth"`

	Control *Control `yaml:"control"`
}

type Control struct {
	Speed     int `yaml:"speed"` // Unit: MiBps
	MaxActive int `yaml:"maxActive"`
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
		os.MkdirAll(c.DownDir, os.ModePerm)
	} else {
		if !f.IsDir() {
			fmt.Printf("DownDir is not a directory")
			os.Exit(6)
		}
	}

	if c.Log != "" {
		c.Log = normalFile(c.Log)
	}

	if c.Net.Tls != nil {
		c.Net.Tls.Cert = normalFile(c.Net.Tls.Cert)
		c.Net.Tls.Key = normalFile(c.Net.Tls.Key)
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
			return errors.New("Not set Net.AgentMgntPort in server config file")
		}
		if c.Net.AgentDataPort == 0 {
			return errors.New("Not set Net.AgentDataPort in server config file")
		}
	}

	if !c.Server {
		if c.DownDir == "" {
			return errors.New("Not set DownDir in client config file")
		}
	}

	if c.Auth.Username == "" || c.Auth.Passowrd == "" || c.Auth.Factor == "" || c.Auth.Crc == "" {
		return errors.New("Not set auth in  config file")
	}

	var err error
	c.Crypto, err = gokits.NewCrypto(c.Auth.Factor, c.Auth.Crc)
	if err != nil {
		return err
	}
	c.Auth.Passowrd, err = c.Crypto.DecryptStr(c.Auth.Passowrd)
	if err != nil {
		return err
	}

	return nil
}

func ParserConfig(cfgfile string, server bool) (*Config, error) {
	ncfg := normalFile(cfgfile)
	if bs, err := ioutil.ReadFile(ncfg); err != nil {
		return nil, err
	} else {
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
}
