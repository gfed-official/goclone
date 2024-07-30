package main

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

func registerUser(username string, password string, ldappassword string) (string, int) {
	l, err := ldap.DialURL("ldap://ldap:389")
	if err != nil {
		message := "Failed to connect to LDAP server."
		return message, 1
	}
	defer l.Close()

	// Bind with Admin
	err = l.Bind("cn=admin,dc=kamino,dc=labs", ldappassword)
	if err != nil {
		message := "Failed to bind with LDAP server."
		return message, 1
	}

	addRequest := ldap.NewAddRequest("uid="+username+",ou=users,dc=kamino,dc=labs", nil)
	addRequest.Attribute("objectClass", []string{"top", "posixAccount", "shadowAccount", "inetOrgPerson"})
	addRequest.Attribute("uid", []string{username})
	addRequest.Attribute("cn", []string{username})
	addRequest.Attribute("sn", []string{username})
	addRequest.Attribute("userPassword", []string{password})
	addRequest.Attribute("loginShell", []string{"/bin/bash"})
	addRequest.Attribute("uidNumber", []string{"10000"})
	addRequest.Attribute("gidNumber", []string{"10000"})
	addRequest.Attribute("homeDirectory", []string{"/home/" + username})
	addRequest.Attribute("shadowLastChange", []string{"0"})
	addRequest.Attribute("shadowMax", []string{"99999"})
	addRequest.Attribute("shadowWarning", []string{"7"})
	err = l.Add(addRequest)

	if err != nil {
		//log.Println(fmt.Sprint(err) + ": " + stderr.String())
		if strings.Contains(err.Error(), "68") {
			message := fmt.Sprintf("Username %s is not available!", username)
			return message, 1
		}
		message := "Failed to register your account. Please contact an administrator."
		return message, 1
	}

	modifyRequest := ldap.NewModifyRequest("cn=Kamino Users,ou=groups,dc=kamino,dc=labs", nil)
	modifyRequest.Add("uniqueMember", []string{"uid=" + username + ",ou=users,dc=kamino,dc=labs"})
	err = l.Modify(modifyRequest)

	if err != nil {
		message := "Failed to register your account. Please contact an administrator."
		return message, 1
	}

	message := "Account created successfully!"
	return message, 0
}
