package common

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/xtfly/gokits"
	"gopkg.in/yaml.v2"
)

// 定义配置映射的结构体
type Config struct {
	Server bool   //是否为服务端
	Name   string `yaml:"name"`

	DownDir string `yaml:"downdir,omitempty"` //只有客户端才配置

	Log string `yaml:"log"`

	Net struct {
		IP       string `yaml:"ip"`
		MgntPort int    `yaml:"mgntPort"`
		DataPort int    `yaml:"dataPort"`

		ClientMgntPort int `yaml:"clientMgntPort,omitempty"`
		ClientDataPort int `yaml:"clientDataPort,omitempty"`

		Tls *struct {
			Cert string `yaml:"cert"`
			Key  string `yaml:"key"`
		} `yaml:"tls,omitempty"`
	} `yaml:"net"`

	Auth struct {
		Username string `yaml:"username"`
		Passowrd string `yaml:"passowrd"`
	} `yaml:"auth"`

	Control *Control `yaml:"control"`
}

type Control struct {
	Speed     int `yaml:"speed"` // Unit: MiBps
	MaxActive int `yaml:"maxActive"`
	CacheSize int `yaml:"cacheSize"` // Unit: MiB
}

func normalFile(dir string) string {
	if !strings.HasPrefix(dir, "/") {
		return filepath.Join(gokits.GetPwd(), dir)
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

	if c.Log != "" && !strings.HasPrefix(c.Log, "/") {
		c.Log = filepath.Join(gokits.GetProcPwd(), c.Log)
	}

	if c.Control == nil {
		c.Control = &Control{Speed: 10, MaxActive: 5, CacheSize: 25}
	}

	if c.Control.Speed == 0 {
		c.Control.Speed = 10
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
		if c.Net.ClientMgntPort == 0 {
			return errors.New("Not set Net.ClientMgntPort in server config file")
		}
		if c.Net.ClientDataPort == 0 {
			return errors.New("Not set Net.ClientDataPort in server config file")
		}
	}

	if !c.Server {
		if c.DownDir == "" {
			return errors.New("Not set DownDir in client config file")
		}
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
