package main

import (
	"crypto/tls"
	"fmt"
	"strings"

	ber "github.com/go-asn1-ber/asn1-ber"
	"github.com/go-ldap/ldap/v3"
	"golang.org/x/text/encoding/unicode"
)

const (
	controlTypeLdapServerPolicyHints           = "1.2.840.113556.1.4.2239"
	controlTypeLdapServerPolicyHintsDeprecated = "1.2.840.113556.1.4.2066"
)

type Client struct {
	ldap ldap.Client
}

type ldapControlServerPolicyHints struct {
	oid string
}

func (cl *Client) Connect() error {
	conn, err := cl.connect()
	if err != nil {
		return fmt.Errorf("Failed to connect to LDAP server: %v", err)
	}

	if ldapConfig.BindDN != "" {
		err = conn.Bind(ldapConfig.BindDN, ldapConfig.BindPassword)
		if err != nil {
			return fmt.Errorf("Failed to bind to LDAP server: %v", err)
		}
	}

	cl.ldap = conn

	return nil
}

func (cl *Client) registerUser(name, password string) error {
	dn, err := cl.CreateUser(name)
	if err != nil {
		return fmt.Errorf("Failed to create user: %v", err)
	}

	err = cl.SetPassword(dn, password)
	if err != nil {
		return fmt.Errorf("Failed to set password: %v", err)
	}

	err = cl.AddToGroup(dn, ldapConfig.GroupDN)
	if err != nil {
		return fmt.Errorf("Failed to add user to group: %v", err)
	}

	err = cl.EnableAccount(dn)
	if err != nil {
		return fmt.Errorf("Failed to enable account: %v", err)
	}

	return nil
}

func (cl *Client) connect() (ldap.Client, error) {
	var dialOpts []ldap.DialOpt
	if strings.HasPrefix(ldapConfig.URL, "ldaps://") {
		dialOpts = append(dialOpts, ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: ldapConfig.InsecureTLS, MinVersion: tls.VersionTLS12}))
	}
	return ldap.DialURL(ldapConfig.URL, dialOpts...)
}

func (cl *Client) CreateUser(name string) (string, error) {
	var attributes []ldap.Attribute

	attributes = append(attributes, ldap.Attribute{
		Type: "objectClass",
		Vals: []string{"top", "person", "organizationalPerson", "user"},
	})
	attributes = append(attributes, ldap.Attribute{
		Type: "sAMAccountName",
		Vals: []string{name},
	})
	attributes = append(attributes, ldap.Attribute{
		Type: "cn",
		Vals: []string{name},
	})
	attributes = append(attributes, ldap.Attribute{
		Type: "Description",
		Vals: []string{"Registered by Goclone"},
	})

	dn := fmt.Sprintf("%s=%s,%s", ldapConfig.UserAttribute, name, ldapConfig.BaseDN)

	req := ldap.AddRequest{
		DN:         dn,
		Attributes: attributes,
	}

	err := cl.ldap.Add(&req)
	if err != nil {
		return "", fmt.Errorf("Failed to add user: %v", err)
	}

	return dn, nil
}

func (cl *Client) AddToGroup(userdn, groupdn string) error {
	req := ldap.NewModifyRequest(groupdn, nil)
	req.Add("member", []string{userdn})
	return cl.ldap.Modify(req)
}

func getSupportedControl(conn ldap.Client) ([]string, error) {
	req := ldap.NewSearchRequest("", ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false, "(objectClass=*)", []string{"supportedControl"}, nil)
	res, err := conn.Search(req)
	if err != nil {
		return nil, err
	}
	return res.Entries[0].GetAttributeValues("supportedControl"), nil
}

func (c *ldapControlServerPolicyHints) Encode() *ber.Packet {
	packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "Control")
	packet.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, c.GetControlType(), "Control Type (LDAP_SERVER_POLICY_HINTS_OID)"))
	packet.AppendChild(ber.NewBoolean(ber.ClassUniversal, ber.TypePrimitive, ber.TagBoolean, true, "Criticality"))

	p2 := ber.Encode(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, nil, "Control Value (Policy Hints)")
	seq := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "PolicyHintsRequestValue")
	seq.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, 1, "Flags"))
	p2.AppendChild(seq)
	packet.AppendChild(p2)

	return packet
}

func (c *ldapControlServerPolicyHints) GetControlType() string {
	return c.oid
}

func (c *ldapControlServerPolicyHints) String() string {
	return "Enforce password history policies during password set: " + c.GetControlType()
}

func (cl *Client) SetPassword(userdn string, password string) error {
	// requires ldaps connection

	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	// The password needs to be enclosed in quotes
	pwdEncoded, err := utf16.NewEncoder().String(fmt.Sprintf("\"%s\"", password))
	if err != nil {
		return err
	}

	// add additional control to request if supported
	controlTypes, err := getSupportedControl(cl.ldap)
	if err != nil {
		return err
	}
	control := []ldap.Control{}
	for _, oid := range controlTypes {
		if oid == controlTypeLdapServerPolicyHints || oid == controlTypeLdapServerPolicyHintsDeprecated {
			control = append(control, &ldapControlServerPolicyHints{oid: oid})
			break
		}
	}

	passReq := ldap.NewModifyRequest(userdn, control)
	passReq.Replace("unicodePwd", []string{pwdEncoded})
	return cl.ldap.Modify(passReq)
}

func (cl *Client) EnableAccount(userdn string) error {
	req := ldap.NewModifyRequest(userdn, nil)
	req.Replace("userAccountControl", []string{"512"})
	return cl.ldap.Modify(req)
}
