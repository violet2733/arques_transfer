package config

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Fixer struct {
		Url       string `yaml:"url"`
		AccessKey string `yaml:"api_key"`
	} `yaml:"fixer"`
	Gopax struct {
		ApiKey    string `yaml:"api_key"`
		SecretKey string `yaml:"secret_key"`
	} `yaml:"gopax"`
	Binance struct {
		ApiKey    string `yaml:"api_key"`
		SecretKey string `yaml:"secret_key"`
	} `yaml:"binance"`
	AdminSql struct {
		Url  string `yaml:"url"`
		Port int    `yaml:"port"`
		Db   string `yaml:"db"`
		Id   string `yaml:"id"`
		Pw   string `yaml:"pw"`
	} `yaml:"adminsql"`
	Redis struct {
		Url  string `yaml:"url"`
		Port int    `yaml:"port"`
	} `yaml:"redis"`
	Slack struct {
		Url string `yaml:"url"`
	} `yaml:"slack"`
}

var config Config

func rootDir() string {
	_, b, _, _ := runtime.Caller(0)
	d := path.Join(path.Dir(b))
	return filepath.Dir(d)
}

func init() {
	dir := rootDir() + "/config.yml"
	data, err := ioutil.ReadFile(dir)
	if err != nil {
		fmt.Printf("(%s)Failed to open config file: %s\n", err, dir)
		return
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Printf("(%s) Failed to unmarshall\n", err)
		return
	}
	fmt.Printf("Config loaded from %s\n", dir)
}

func Get() *Config {
	return &config
}
