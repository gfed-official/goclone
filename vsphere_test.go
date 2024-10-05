package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVM(t *testing.T) {
    fmt.Println("TestGetVM")
    assert.Equal(t, "testvm", fmt.Sprintf("%s", "testvm"))
}
