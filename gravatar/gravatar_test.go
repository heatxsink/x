package gravatar

import (
	"fmt"
	"testing"
)

func TestGetUrl(t *testing.T) {
	url := GetURL("ngranado@gmail.com", 100)
	fmt.Println(url)
}

func TestNotExist(t *testing.T) {
	url := GetURL("zzz1234zzzqqqwww@gmail.com", 100)
	fmt.Println(url)
}
