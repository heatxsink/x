package shellkit

import (
	"testing"
)

func TestExecute(t *testing.T) {
	err := Execute("ls", "-alh", "/dev/tty")
	if err != nil {
		t.Error(err)
	}
}

func TestExecuteFail(t *testing.T) {
	err := Execute("lss", "-alh", "/dev/")
	if err != nil {
		t.Error(err)
	}
}
