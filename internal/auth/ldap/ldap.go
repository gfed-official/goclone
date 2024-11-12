package ldap

import (
	"crypto/tls"
	"fmt"
	"goclone/internal/api/handlers"
	"goclone/internal/config"
	"net/http"
	"regexp"
	"strings"
	uni "unicode"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	ber "github.com/go-asn1-ber/asn1-ber"
	"github.com/go-ldap/ldap/v3"
	"golang.org/x/text/encoding/unicode"
)

const (
	controlTypeLdapServerPolicyHints           = "1.2.840.113556.1.4.2239"
	controlTypeLdapServerPolicyHintsDeprecated = "1.2.840.113556.1.4.2066"
)

type LdapClient struct {
	ldap   ldap.Client
	config config.LdapProvider
}

type ldapControlServerPolicyHints struct {
	oid string
}

func NewLdapManager(config config.LdapProvider) *LdapClient {
    return &LdapClient{config: config}
}

func (cl *LdapClient) Connect() error {
	conn, err := cl.connect()
	if err != nil {
		return fmt.Errorf("Failed to connect to LDAP server: %v", err)
	}

	if cl.config.BindUser != "" {
		err = conn.Bind(cl.config.BindUser, cl.config.BindPassword)
		if err != nil {
			return fmt.Errorf("Failed to bind to LDAP server: %v", err)
		}
	}

	cl.ldap = conn

	return nil
}

func (cl *LdapClient) Login(c *gin.Context) {
    var loginInfo map[string]interface{}
    if err := c.BindJSON(&loginInfo); err != nil {
        c.String(http.StatusBadRequest, "Bad Request")
        return
    }

    username, ok := loginInfo["username"].(string)
    if !ok {
        c.String(http.StatusBadRequest, "Bad Request")
        return
    }

    password, ok := loginInfo["password"].(string)
    if !ok {
        c.String(http.StatusBadRequest, "Bad Request")
        return
    }

    err := cl.Connect()
    if err != nil {
        c.String(http.StatusInternalServerError, "Internal Server Error", err)
        return
    }

    valid, err := cl.LoginReq(username, password)
    if err != nil {
        c.String(http.StatusInternalServerError, "Internal Server Error")
        return
    }

    if !valid {
        c.String(http.StatusUnauthorized, "Unauthorized")
        return
    }

    session := sessions.Default(c)
    session.Set("id", username)

    if err := session.Save(); err != nil {
        c.String(http.StatusInternalServerError, "Internal Server Error")
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Logged in"})
}

func (cl *LdapClient) LoginReq(username, password string) (bool, error) {
	userdn, err := cl.GetUserDN(username)
	if err != nil {
		return false, fmt.Errorf("Failed to get user DN: %v", err)
	}

	err = cl.ldap.Bind(userdn, password)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func (cl *LdapClient) RegisterUser(c *gin.Context) {
    var userInfo map[string]interface{}
    if err := c.BindJSON(&userInfo); err != nil {
        c.String(http.StatusBadRequest, "Bad Request")
        return
    }

    err := cl.RegisterUserReq(userInfo)
    if err != nil {
        c.String(http.StatusInternalServerError, "Internal Server Error")
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "User registered"})
}

func (cl *LdapClient) RegisterUserReq(userInfo map[string]interface{}) error {
    username, ok := userInfo["username"].(string)
    if !ok {
        return fmt.Errorf("Username not provided or invalid")
    }

    if len(username) < 1 || len(username) > 20 {
        return fmt.Errorf("Username must be between 1 and 20 characters")
    }

    regex := regexp.MustCompile("^[a-zA-Z0-9_-]*$")
    if !regex.MatchString(username) {
        return fmt.Errorf("Username must only contain letters, numbers, underscores, and hyphens")
    }

    password, ok := userInfo["password"].(string)
    if !ok {
        return fmt.Errorf("Password not provided or invalid")
    }

    valid := validatePassword(password)
    if !valid {
        return fmt.Errorf("Password must be at least 8 characters long and contain at least one letter and one number")
    }
	dn, err := cl.CreateUser(username)
	if err != nil {
		return fmt.Errorf("Failed to create user: %v", err)
	}

	err = cl.SetPassword(dn, password)
	if err != nil {
		return fmt.Errorf("Failed to set password: %v", err)
	}

	err = cl.AddToGroup(dn, cl.config.UserGroupDN)
	if err != nil {
		return fmt.Errorf("Failed to add user to group: %v", err)
	}

	err = cl.EnableAccount(dn)
	if err != nil {
		return fmt.Errorf("Failed to enable account: %v", err)
	}

	return nil
}

func (cl *LdapClient) connect() (ldap.Client, error) {
	var dialOpts []ldap.DialOpt
	if strings.HasPrefix(cl.config.URL, "ldaps://") {
		dialOpts = append(dialOpts, ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: cl.config.SkipTLSVerify, MinVersion: tls.VersionTLS12}))
	} else {
        return nil, fmt.Errorf("Only ldaps:// is supported")
    }
	return ldap.DialURL(cl.config.URL, dialOpts...)
}

func (cl *LdapClient) CreateUser(name string) (string, error) {
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

	dn := fmt.Sprintf("%s=%s,%s", cl.config.FieldMap.UserIdentifier, name, cl.config.UserOU)

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

func (cl *LdapClient) AddToGroup(userdn, groupdn string) error {
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

func (cl *LdapClient) SetPassword(userdn string, password string) error {
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

func (cl *LdapClient) EnableAccount(userdn string) error {
	req := ldap.NewModifyRequest(userdn, nil)
	req.Replace("userAccountControl", []string{"512"})
	return cl.ldap.Modify(req)
}

func (cl *LdapClient) SearchEntry(req *ldap.SearchRequest) (*ldap.Entry, error) {
	res, err := cl.ldap.Search(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to search entry: %v", err)
	}
	if len(res.Entries) == 0 {
		return nil, nil
	}
	return res.Entries[0], nil
}

func (cl *LdapClient) Disconnect() error {
	if cl.ldap == nil {
		return nil
	}
	return cl.ldap.Close()
}

func (cl *LdapClient) GetUserDN(username string) (string, error) {
	req := ldap.NewSearchRequest(
		cl.config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=user)(%s=%s))", "sAMAccountName", username),
		[]string{"dn"},
		nil,
	)

	entry, err := cl.SearchEntry(req)
	if err != nil {
		return "", fmt.Errorf("Failed to search for user: %v", err)
	}

	if entry == nil {
		return "", fmt.Errorf("User not found")
	}

	return entry.DN, nil
}

func (cl *LdapClient) IsAdmin(c *gin.Context) {
    user := handlers.GetUser(c)
    if user == "" {
        c.String(http.StatusUnauthorized, "Unauthorized")
        c.Abort()
        return
    }

    isAdmin, err := cl.IsAdminReq(user)
    if err != nil {
        c.String(http.StatusInternalServerError, "Internal Server Error")
        c.Abort()
        return
    }

    if !isAdmin {
        c.String(http.StatusForbidden, "Forbidden")
        c.Abort()
        return
    }

    session := sessions.Default(c)
    session.Set("isAdmin", true)
    if err := session.Save(); err != nil {
        c.String(http.StatusInternalServerError, "Internal Server Error")
        return
    }

    c.Next()
}

func (cl *LdapClient) IsAdminReq(username string) (bool, error) {
	req := ldap.NewSearchRequest(
		cl.config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=user)(%s=%s))", "sAMAccountName", username),
		[]string{"adminCount"},
		nil,
	)

	entry, err := cl.SearchEntry(req)
	if err != nil {
		return false, fmt.Errorf("Failed to search for user: %v", err)
	}

	if entry == nil {
		return false, fmt.Errorf("User not found")
	}

	for _, count := range entry.GetAttributeValues("adminCount") {
		if count == "1" {
			return true, nil
		}
	}

	return false, nil
}

func (cl *LdapClient) UserExists(username string) (bool, error) {
	req := ldap.NewSearchRequest(
		cl.config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=user)(%s=%s))", "sAMAccountName", username),
		[]string{"dn"},
		nil,
	)

	entry, err := cl.SearchEntry(req)
	if err != nil {
		return false, fmt.Errorf("Failed to search for user: %v", err)
	}

	return entry != nil, nil
}

func (cl *LdapClient) DeleteUser(username string) error {
	userdn, err := cl.GetUserDN(username)
	if err != nil {
		return fmt.Errorf("Failed to get user DN: %v", err)
	}

	req := ldap.NewDelRequest(userdn, nil)
	err = cl.ldap.Del(req)
	if err != nil {
		return fmt.Errorf("Failed to delete user: %v", err)
	}

	return nil
}

func validatePassword(password string) bool {
	var number, letter bool
	if len(password) < 8 {
		return false
	}
	for _, c := range password {
		switch {
		case uni.IsNumber(c):
			number = true
		case uni.IsLetter(c):
			letter = true
		}
	}

	return number && letter
}
