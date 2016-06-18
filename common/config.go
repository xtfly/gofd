package common

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/xtfly/gokits"
	"gopkg.in/yaml.v2"
)

// 定义配置映射的结构体
type Config struct {
	Name   string `yaml:"name"`
	Server bool   //是否为服务端

	DownDir string `yaml:"downdir"` //只有客户端才配置

	Log struct {
		Level     string `yaml:"level"`
		File      string `yaml:"file,omitempty"`
		FileSize  int    `yaml:"fileSzie,omitempty"`
		FileCount int    `yaml:"fileCount,omitempty"`
	} `yaml:"log"`

	Net struct {
		IP       string `yaml:"ip"`
		MgntPort int    `yaml:"mgntPort"`
		DataPort int    `yaml:"dataPort"`

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

func normalFile(cfgfile string) string {
	if !strings.HasPrefix(cfgfile, "/") {
		return filepath.Join(gokits.GetProcPwd(), cfgfile)
	}
	return cfgfile
}

func (c *Config) defaultValue() {
	if !strings.HasPrefix(c.DownDir, "/") {
		c.DownDir = filepath.Join(gokits.GetProcPwd(), c.DownDir)
	}

	if c.Log.FileCount == 0 {
		c.Log.FileCount = 10
	}
	if c.Log.FileSize == 0 {
		c.Log.FileSize = 10
	}

	if c.Log.File != "" && !strings.HasPrefix(c.Log.File, "/") {
		c.Log.File = filepath.Join(gokits.GetProcPwd(), c.Log.File)
	}

	if c.Control == nil {
		c.Control = &Control{Speed: 10, MaxActive: 5, CacheSize: 50}
	}
}

func ParserConfig(cfgfile string) (*Config, error) {
	ncfg := normalFile(cfgfile)
	if bs, err := ioutil.ReadFile(ncfg); err != nil {
		return nil, err
	} else {
		cfg := new(Config)
		if err := yaml.Unmarshal(bs, cfg); err != nil {
			return nil, err
		}
		cfg.defaultValue()
		return cfg, nil
	}
}
