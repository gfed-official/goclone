package config

import (
	"fmt"

	"github.com/spf13/viper"
)

var (
	configErrors = []string{}
)

type Config struct {
	Core struct {
        ExternalURL      string `mapstructure:"external_url"`
        ListeningAddress string `mapstructure:"listening_address"`
        LogPath          string `mapstructure:"log_path"`
	} `mapstructure:"core"`

	Auth Auth `mapstructure:"auth"`
	VirtProvider Provider `mapstructure:"provider"`
}

func LoadConfig(path string) (*Config, error) {
    viper.AddConfigPath(path)
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")

    BindFromEnvironment()
    err := viper.ReadInConfig()
    if err != nil {
        viper.SafeWriteConfig()
    }

    cfg := &Config{}

    fmt.Println(viper.Get("provider.api_password"))
    fmt.Println(viper.AllKeys())

    err = viper.Unmarshal(&cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %v", err)
    }

    return cfg, nil
}

func BindFromEnvironment() {
    // Core configuration
    viper.BindEnv("core.external_url", "GOCLONE_CORE_EXTERNAL_URL")
    viper.BindEnv("core.listening_address", "GOCLONE_CORE_LISTENING_ADDRESS")
    viper.BindEnv("core.log_path", "GOCLONE_CORE_LOG_PATH")

    // Auth configuration
    viper.BindEnv("auth.ldap.provider_name", "GOCLONE_AUTH_LDAP_PROVIDER_NAME")
    viper.BindEnv("auth.ldap.url", "GOCLONE_AUTH_LDAP_URL")
    viper.BindEnv("auth.ldap.start_tls", "GOCLONE_AUTH_LDAP_START_TLS")
    viper.BindEnv("auth.ldap.ldaps", "GOCLONE_AUTH_LDAP_LDAPS")
    viper.BindEnv("auth.ldap.skip_tls_verify", "GOCLONE_AUTH_LDAP_SKIP_TLS_VERIFY")
    viper.BindEnv("auth.ldap.tls_certificate_path", "GOCLONE_AUTH_LDAP_TLS_CERTIFICATE_PATH")
    viper.BindEnv("auth.ldap.tls_key_path", "GOCLONE_AUTH_LDAP_TLS_KEY_PATH")
    viper.BindEnv("auth.ldap.base_dn", "GOCLONE_AUTH_LDAP_BASE_DN")
    viper.BindEnv("auth.ldap.bind_user", "GOCLONE_AUTH_LDAP_BIND_USER")
    viper.BindEnv("auth.ldap.bind_password", "GOCLONE_AUTH_LDAP_BIND_PASSWORD")
    viper.BindEnv("auth.ldap.login_filter", "GOCLONE_AUTH_LDAP_LOGIN_FILTER")
    viper.BindEnv("auth.ldap.admin_group_dn", "GOCLONE_AUTH_LDAP_ADMIN_GROUP_DN")
    viper.BindEnv("auth.ldap.user_group_dn", "GOCLONE_AUTH_LDAP_USER_GROUP_DN")
    viper.BindEnv("auth.ldap.user_ou", "GOCLONE_AUTH_LDAP_USER_OU")

    // Auth.Ldap.FieldMap

    viper.BindEnv("auth.ldap.field_map.user_identifier", "GOCLONE_AUTH_LDAP_FIELD_MAP_USER_IDENTIFIER")
    viper.BindEnv("auth.ldap.field_map.email", "GOCLONE_AUTH_LDAP_FIELD_MAP_EMAIL")
    viper.BindEnv("auth.ldap.field_map.first_name", "GOCLONE_AUTH_LDAP_FIELD_MAP_FIRST_NAME")
    viper.BindEnv("auth.ldap.field_map.last_name", "GOCLONE_AUTH_LDAP_FIELD_MAP_LAST_NAME")
    viper.BindEnv("auth.ldap.field_map.memberof", "GOCLONE_AUTH_LDAP_FIELD_MAP_MEMBEROF")

    // Provider configuration
    viper.BindEnv("provider.name", "GOCLONE_PROVIDER_NAME")
    viper.BindEnv("provider.url", "GOCLONE_PROVIDER_URL")
    viper.BindEnv("provider.api_username", "GOCLONE_PROVIDER_APIUSERNAME")
    viper.BindEnv("provider.api_password", "GOCLONE_PROVIDER_APIPASSWORD")
    viper.BindEnv("provider.max_pod_limit", "GOCLONE_PROVIDER_MAX_POD_LIMIT")
    viper.BindEnv("provider.default_network_id", "GOCLONE_PROVIDER_DEFAULT_NETWORK_ID")
    viper.BindEnv("provider.competition_network_id", "GOCLONE_PROVIDER_COMPETITION_NETWORK_ID")
    viper.BindEnv("provider.domain", "GOCLONE_PROVIDER_DOMAIN")

    // VCenter fields (nested within Provider)
    viper.BindEnv("provider.vcenter.clone_role", "GOCLONE_PROVIDER_VCENTER_CLONE_ROLE")
    viper.BindEnv("provider.vcenter.custom_clone_role", "GOCLONE_PROVIDER_VCENTER_CUSTOM_CLONE_ROLE")
    viper.BindEnv("provider.vcenter.datacenter", "GOCLONE_PROVIDER_VCENTER_DATACENTER")
    viper.BindEnv("provider.vcenter.datastore", "GOCLONE_PROVIDER_VCENTER_DATASTORE")
    viper.BindEnv("provider.vcenter.default_wan_port_group", "GOCLONE_PROVIDER_VCENTER_DEFAULT_WAN_PORT_GROUP")
    viper.BindEnv("provider.vcenter.destination_folder", "GOCLONE_PROVIDER_VCENTER_DESTINATION_FOLDER")
    viper.BindEnv("provider.vcenter.ending_port_group", "GOCLONE_PROVIDER_VCENTER_ENDING_PORT_GROUP")
    viper.BindEnv("provider.vcenter.main_distributed_switch", "GOCLONE_PROVIDER_VCENTER_MAIN_DISTRIBUTED_SWITCH")
    viper.BindEnv("provider.vcenter.natted_router_path", "GOCLONE_PROVIDER_VCENTER_NATTED_ROUTER_PATH")
    viper.BindEnv("provider.vcenter.port_group_suffix", "GOCLONE_PROVIDER_VCENTER_PORT_GROUP_SUFFIX")
    viper.BindEnv("provider.vcenter.preset_template_resource_pool", "GOCLONE_PROVIDER_VCENTER_PRESET_TEMPLATE_RESOURCE_POOL")
    viper.BindEnv("provider.vcenter.router_password", "GOCLONE_PROVIDER_VCENTER_ROUTER_PASSWORD")
    viper.BindEnv("provider.vcenter.router_path", "GOCLONE_PROVIDER_VCENTER_ROUTER_PATH")
    viper.BindEnv("provider.vcenter.router_program", "GOCLONE_PROVIDER_VCENTER_ROUTER_PROGRAM")
    viper.BindEnv("provider.vcenter.router_program_args", "GOCLONE_PROVIDER_VCENTER_ROUTER_PROGRAM_ARGS")
    viper.BindEnv("provider.vcenter.router_username", "GOCLONE_PROVIDER_VCENTER_ROUTER_USERNAME")
    viper.BindEnv("provider.vcenter.starting_port_group", "GOCLONE_PROVIDER_VCENTER_STARTING_PORT_GROUP")
    viper.BindEnv("provider.vcenter.target_resource_pool", "GOCLONE_PROVIDER_VCENTER_TARGET_RESOURCE_POOL")
    viper.BindEnv("provider.vcenter.template_folder", "GOCLONE_PROVIDER_VCENTER_TEMPLATE_FOLDER")
    viper.BindEnv("provider.vcenter.competition_end_port_group", "GOCLONE_PROVIDER_VCENTER_COMPETITION_END_PORT_GROUP")
    viper.BindEnv("provider.vcenter.competition_resource_pool", "GOCLONE_PROVIDER_VCENTER_COMPETITION_RESOURCE_POOL")
    viper.BindEnv("provider.vcenter.competition_start_port_group", "GOCLONE_PROVIDER_VCENTER_COMPETITION_START_PORT_GROUP")
    viper.BindEnv("provider.vcenter.competition_wan_port_group", "GOCLONE_PROVIDER_VCENTER_COMPETITION_WAN_PORT_GROUP")
}
