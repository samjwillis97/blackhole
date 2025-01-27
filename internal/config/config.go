package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/viper"
)

var secretsSet bool = false
var appSecrets Secrets

var confSet bool = false
var appConf AppConfig

type Secrets struct {
	DebridApiKey string `mapstructure:"DEBRID_API_KEY"`
	SonarrApiKey string `mapstructure:"SONARR_API_KEY"`
}

type DebridConfig struct {
	Url          string
	WatchPatch   string `mapstructure:"watch_path"`
	MountTimeout int64  `mapstructure:"mount_timeout"` // This is time we will wait for it to appear in the mount
}

type SonarrConfig struct {
	Name           string `mapstructure:"name"`
	Url            string
	WatchPath      string `mapstructure:"watch_path"`
	ProcessingPath string `mapstructure:"processing_path"`
	CompletedPath  string `mapstructure:"completed_path"`
}

type AppConfig struct {
	RealDebrid DebridConfig `mapstructure:"real_debrid"`
	Sonarr     []SonarrConfig
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
	var secrets Secrets

	if v != nil {
		err := v.Unmarshal(&secrets)

		if err != nil {
			panic(errors.New("Failed to unmarshal secrets"))
		}

		secretsSet = true
		appSecrets = secrets
		return
	}

	v = viper.New()

	v.SetConfigFile(".env")
	v.AddConfigPath(".")
	v.ReadInConfig()

	v.AutomaticEnv()

	err := v.Unmarshal(&secrets)
	if err != nil {
		panic(errors.New("Failed to unmarshal secrets"))
	}

	secretsSet = true
	appSecrets = secrets

	validateSecrets()
}

func GetSecrets() Secrets {
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
		panic(errors.New("Invalid path for Real Debrid watch"))
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
}

func validateSecrets() {
	if appSecrets.SonarrApiKey == "" {
		panic(errors.New("Sonarr API key not set"))
	}

	if appSecrets.DebridApiKey == "" {
		panic(errors.New("Debrid API key not set"))
	}
}
