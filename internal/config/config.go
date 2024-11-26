package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
)

type Core struct {
    ExternalURL      string `mapstructure:"external_url"`
    ListeningAddress string `mapstructure:"listening_address"`
    LogPath          string `mapstructure:"log_path"`
    OtlpEndpoint     string `mapstructure:"otlp_endpoint"`
    Tracer           trace.Tracer
}

type Config struct {
    Core Core `mapstructure:"core"`
    Auth Auth `mapstructure:"auth"`
    Provider Provider `mapstructure:"provider"`
}

func LoadConfig(path string) (*Config, error) {
    viper.AddConfigPath(path)
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")

    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    viper.AutomaticEnv()
    //BindFromEnvironment()


    cfg := &Config{}
    bindEnvs(cfg)
    err := viper.Unmarshal(&cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %v", err)
    }

    return cfg, nil
}

func bindEnvs(config interface{}, parentKeys ...string) error {
    if config == nil {
        return nil
    }
    configValue := reflect.ValueOf(config)
    if configValue.Kind() == reflect.Ptr {
        configValue = configValue.Elem()
    }
    configType := configValue.Type()

    for i := 0; i < configValue.NumField(); i++ {
        field := configType.Field(i)
        fieldValue := configValue.Field(i)
        mapstructureTag := field.Tag.Get("mapstructure")
        if mapstructureTag == "" {
            mapstructureTag = field.Name
        }
        keys := append(parentKeys, mapstructureTag)
        if fieldValue.Kind() == reflect.Struct {
            bindEnvs(fieldValue.Addr().Interface(), keys...)
        } else {
            key := strings.Join(keys, ".")
            key = strings.Replace(key, ",squash", "", -1)
            key = strings.Replace(key, "..", ".", -1)
            envVar := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
            viper.BindEnv(key, "GOCLONE_" + envVar)
        }
    }
    return nil
}
