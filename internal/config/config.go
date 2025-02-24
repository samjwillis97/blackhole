package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/viper"
)

var secretsSet bool = false
var appSecrets *viper.Viper = nil

var confSet bool = false
var appConf AppConfig

type DebridConfig struct {
	Url          string
	WatchPatch   string `mapstructure:"watch_path"`
	MountTimeout int64  `mapstructure:"mount_timeout"` // This is time we will wait for it to appear in the mount
}

type ArrConfig struct {
	Name           string `mapstructure:"name"`
	Url            string
	WatchPath      string `mapstructure:"watch_path"`
	ProcessingPath string `mapstructure:"processing_path"`
	CompletedPath  string `mapstructure:"completed_path"`
}

type AppConfig struct {
	RealDebrid DebridConfig `mapstructure:"real_debrid"`
	Sonarr     []ArrConfig
	Radarr     []ArrConfig
}

// This seems kinda fucked idk
func InitializeAppConfig(v *viper.Viper) {
	var conf AppConfig

	if v != nil {
		err := v.Unmarshal(&conf)
		if err != nil {
			panic(errors.New("Failed to unmarshal app config"))
		}
		confSet = true
		appConf = conf

		return
	}

	v = viper.New()

	v.SetDefault("real_debrid.mount_timeout", 600)

	v.SetConfigName("blackhole")
	v.SetConfigType("yaml")

	v.AddConfigPath("/etc/blackhole/")
	v.AddConfigPath(".")
	v.ReadInConfig()

	v.AutomaticEnv()
	v.BindEnv("real_debrid.url", "DEBRID_URL")

	err := v.Unmarshal(&conf)
	if err != nil {
		panic(errors.New("Failed to unmarshal app config"))
	}

	confSet = true
	appConf = conf

	validateAppConfig()
}

func InitializeSecrets(v *viper.Viper) {
	if v != nil {
		secretsSet = true
		return
	}

	v = viper.New()

	v.SetConfigFile(".env")
	v.AddConfigPath(".")
	v.ReadInConfig()

	v.AutomaticEnv()

	secretsSet = true
	appSecrets = v
}

func GetSecrets() *viper.Viper {
	if !secretsSet {
		InitializeSecrets(nil)
	}

	return appSecrets
}

func GetAppConfig() AppConfig {
	if !confSet {
		InitializeAppConfig(nil)
	}

	return appConf
}

func validateAppConfig() {
	_, err := url.ParseRequestURI(appConf.RealDebrid.Url)
	if err != nil {
		panic(errors.New("Invalid URL for Real Debrid"))
	}

	if _, err := os.Stat(appConf.RealDebrid.WatchPatch); err != nil {
		panic(errors.New(fmt.Sprintf("Invalid path for Real Debrid watch: %s", appConf.RealDebrid.WatchPatch)))
	}

	for _, v := range appConf.Sonarr {
		_, err = url.ParseRequestURI(v.Url)
		if err != nil {
			panic(errors.New(fmt.Sprintf("Invalid URL for Sonarr: %s", v.Name)))
		}

		if _, err := os.Stat(v.CompletedPath); err != nil {
			panic(errors.New(fmt.Sprintf("Invalid path for Sonarr completed: %s", v.Name)))
		}

		if _, err := os.Stat(v.ProcessingPath); err != nil {
			panic(errors.New(fmt.Sprintf("Invalid path for Sonarr processing: %s", v.Name)))
		}
	}

	for _, v := range appConf.Radarr {
		_, err = url.ParseRequestURI(v.Url)
		if err != nil {
			panic(errors.New(fmt.Sprintf("Invalid URL for Radarr: %s", v.Name)))
		}

		if _, err := os.Stat(v.CompletedPath); err != nil {
			panic(errors.New(fmt.Sprintf("Invalid path for Radarr completed: %s", v.Name)))
		}

		if _, err := os.Stat(v.ProcessingPath); err != nil {
			panic(errors.New(fmt.Sprintf("Invalid path for Radarr processing: %s", v.Name)))
		}
	}
}
