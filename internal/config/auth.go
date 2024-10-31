package config

import "github.com/go-ldap/ldap/v3"

type Auth struct {
	Ldap LdapProvider `yaml:"ldap"`
}

type CommonAttributes struct {
	UserIdentifier string `yaml:"user_identifier"`
	Email          string `yaml:"email"`
	FirstName      string `yaml:"first_name"`
	LastName       string `yaml:"last_name"`
}

type LdapFields struct {
	CommonAttributes `yaml:",inline"`
	GroupMembership  string `yaml:"memberof"`
}

type LdapProvider struct {
	ProviderName string `yaml:"provider_name"`

	URL                string `yaml:"url"`
	StartTLS           bool   `yaml:"start_tls"`
	LDAPS              bool   `yaml:"ldaps"`
	SkipTLSVerify      bool   `yaml:"skip_tls_verify"`
	TlsCertificatePath string `yaml:"tls_certificate_path"`
	TlsKeyPath         string `yaml:"tls_key_path"`

	BaseDN       string `yaml:"base_dn"`
	BindUser     string `yaml:"bind_user"`
	BindPassword string `yaml:"bind_password"`

	FieldMap LdapFields `yaml:"field_map"`

	LoginFilter        string   `yaml:"login_filter"`
	AdminGroupDN       string   `yaml:"admin_group_dn"`
    UserGroupDN        string   `yaml:"user_group_dn"`
    UserOU             string   `yaml:"user_ou"`
	ParsedAdminGroupDN *ldap.DN `yaml:"-"`
}
