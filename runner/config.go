package runner

import (
	"github.com/spf13/viper"
)

type Variable struct {
	Name  string `mapstructure:"name"`
	Value string `mapstructure:"value"`
}

type Spec struct {
	Variables []Variable `mapstructure:"variables"`
}

type Config struct {
	APIVersion string `mapstructure:"apiVersion"`
	Kind       string `mapstructure:"kind"`
	Spec       Spec   `mapstructure:"spec"`
}

func NewConfig(configDir string) (*Config, error) {
	// we ONLY search for files named beaver.yml
	viper.SetConfigName("beaver")
	viper.AddConfigPath(configDir)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	return &config, nil
}
