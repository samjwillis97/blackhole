package config

import (
	"errors"
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
	Url            string
	WatchPath      string `mapstructure:"watch_path"`
	ProcessingPath string `mapstructure:"processing_path"`
	CompletedPath  string `mapstructure:"completed_path"`
}

type AppConfig struct {
	RealDebrid DebridConfig `mapstructure:"real_debrid"`
	Sonarr     SonarrConfig
}

// This seems kinda fucked idk
func InitializeAppConfig(v *viper.Viper) {
	var conf AppConfig

	if v != nil {
		err := v.Unmarshal(&conf)
		if err != nil {
			panic(err)
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
		panic(err)
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
			panic(err)
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
		panic(err)
	}

	secretsSet = true
	appSecrets = secrets
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
		panic(err)
	}

	if _, err := os.Stat(appConf.RealDebrid.WatchPatch); errors.Is(err, os.ErrNotExist) {
		panic(err)
	}

	_, err = url.ParseRequestURI(appConf.Sonarr.Url)
	if err != nil {
		panic(err)
	}

	if _, err := os.Stat(appConf.Sonarr.CompletedPath); errors.Is(err, os.ErrNotExist) {
		panic(err)
	}

	if _, err := os.Stat(appConf.Sonarr.ProcessingPath); errors.Is(err, os.ErrNotExist) {
		panic(err)
	}
}

// TODO Validate secrets
