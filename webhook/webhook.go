package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func SendJSON(url string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = httpPost(url, b)
	if err != nil {
		return err
	}
	return err
}

func httpPost(url string, payload []byte) ([]byte, error) {
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP status code: %d", response.StatusCode)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return content, nil
}
