package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/heatxsink/x/exp/http/clients"
)

func SendJSON(url string, data interface{}) error {
	return SendJSONWithClient(clients.Default(), url, data)
}

func SendJSONWithClient(client *http.Client, url string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	statusCode, content, err := post(client, url, b)
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

func post(client *http.Client, url string, payload []byte) (int, []byte, error) {
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return -1, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
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

func postWithContext(ctx context.Context, client *http.Client, url string, headers map[string]string, payload []byte) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	for k, v := range headers {
		request.Header.Add(k, v)
	}
	return client.Do(request)
}

func SendWithContextAndRetry(ctx context.Context, retries int, delay time.Duration, client *http.Client, url string, headers map[string]string, data interface{}) error {
	var lerr error
	var err error
	for range retries {
		err = SendWithContext(ctx, client, url, headers, data)
		if err != nil {
			lerr = errors.Join(err)
			time.Sleep(delay)
			continue
		}
		return nil
	}
	if lerr != nil {
		return lerr
	}
	return nil
}

func SendWithContext(ctx context.Context, client *http.Client, url string, headers map[string]string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	response, err := postWithContext(ctx, client, url, headers, b)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	switch response.StatusCode {
	case 200:
		return nil
	case 204:
		return nil
	default:
		return fmt.Errorf("HTTP status code: %d HTTP body: %s", response.StatusCode, string(content))
	}
}
