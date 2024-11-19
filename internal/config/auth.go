package config

type Auth struct {
	Ldap LdapProvider `mapstructure:"ldap"`
}

type CommonAttributes struct {
	UserIdentifier string `mapstructure:"user_identifier"`
	Email          string `mapstructure:"email"`
	FirstName      string `mapstructure:"first_name"`
	LastName       string `mapstructure:"last_name"`
}

type LdapFields struct {
	CommonAttributes `mapstructure:",squash"`
	GroupMembership  string `mapstructure:"memberof"`
}

type LdapProvider struct {
	ProviderName string `mapstructure:"provider_name"`

	URL                string `mapstructure:"url"`
	StartTLS           bool   `mapstructure:"start_tls"`
	LDAPS              bool   `mapstructure:"ldaps"`
	SkipTLSVerify      bool   `mapstructure:"skip_tls_verify"`
	TlsCertificatePath string `mapstructure:"tls_certificate_path"`
	TlsKeyPath         string `mapstructure:"tls_key_path"`

	BaseDN       string `mapstructure:"base_dn"`
	BindUser     string `mapstructure:"bind_user"`
	BindPassword string `mapstructure:"bind_password"`

	FieldMap LdapFields `mapstructure:"field_map"`

	LoginFilter        string   `mapstructure:"login_filter"`
	AdminGroupDN       string   `mapstructure:"admin_group_dn"`
    UserGroupDN        string   `mapstructure:"user_group_dn"`
    UserOU             string   `mapstructure:"user_ou"`
}
