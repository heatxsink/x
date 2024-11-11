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
	statusCode, content, err := post(url, b)
	if err != nil {
		return err
	}
	switch statusCode {
	case 200:
		return nil
	case 204:
		return nil
	default:
		return fmt.Errorf("HTTP status code: %d HTTP body: %s", statusCode, string(content))
	}
}

func post(url string, payload []byte) (int, []byte, error) {
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return -1, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return -1, nil, err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return -1, nil, err
	}
	return response.StatusCode, content, nil
}
