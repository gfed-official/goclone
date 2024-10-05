package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPods(t *testing.T) {
    pods, err := GetAllPods()
    if err != nil {
        t.Error(err)
    }
    assert.NotEmpty(t, pods)
    fmt.Println(pods)
}
