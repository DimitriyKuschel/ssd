package providers

import (
	"fmt"
	"github.com/spf13/viper"
	"path/filepath"
	"ssd/internal/structures"
	"strings"
)

func NewConfigProvider(flags *structures.CliFlags) (*structures.Config, error) {
	var conf structures.Config

	filename := filepath.Base(flags.ConfigPath)
	viper.AddConfigPath(filepath.Dir(flags.ConfigPath))
	viper.SetConfigName(strings.TrimSuffix(filename, filepath.Ext(filename)))
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	err = viper.Unmarshal(&conf)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into config struct: %w", err)
	}

	cnfValidator := NewCnfValidator(&conf)
	err = cnfValidator.Validate()
	if err != nil {
		return nil, err
	}

	conf.AppName = "SimpleStatisticDaemon"
	conf.Path = flags.ConfigPath
	conf.Debug = flags.DebugMode

	return &conf, nil
}
