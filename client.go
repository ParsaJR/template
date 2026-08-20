package liara

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// https://openapi.liara.ir/?urls.primaryName=DNS

const (
	apiBaseURL string = "https://dns-service.iran.liara.ir/api/v1/"
)

// Client is a struct, that knows how to interact with the Liara's api thorugh
// its methods.
type client struct {
	Liara_api_token string
	BaseURL         string
	httpClient      *http.Client
}

// newClient creates a new Liara API client.
func newClient(token string) *client {
	return &client{
		Liara_api_token: token,
		BaseURL:         apiBaseURL,
		httpClient: &http.Client{
			Timeout: time.Second * 20,
		},
	}
}

// Gets all the dns records in a specific zone.
func (c client) APIGetRecords(ctx context.Context, zone string) ([]APIRecord, error) {
	endpoint := c.BaseURL + fmt.Sprintf("zones/%s/dns-records", zone)

	resp, err := c.do(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result GetRecordsResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// Inserts a single dns record through Liara api, and returns it.
func (c client) APIPostRecord(ctx context.Context, zone string, record APIRecord) (APIRecord, error) {
	endpoint := c.BaseURL + fmt.Sprintf("zones/%s/dns-records", zone)

	body, err := json.Marshal(record)
	if err != nil {
		return APIRecord{}, err
	}

	payload := bytes.NewReader(body)

	resp, err := c.do(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return APIRecord{}, err
	}

	defer resp.Body.Close()

	var result PostRecordResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return APIRecord{}, err
	}

	return result.Data, nil
}

// APIUpdateRecord updates a single DNS record through the Liara API
// and returns the updated record.
// It needs the entire entity to be replaces with another.
func (c client) APIUpdateRecord(ctx context.Context, zone string, id string, record APIRecord) (APIRecord, error) {
	endpoint := c.BaseURL + fmt.Sprintf("zones/%s/dns-records/%s", zone, id)

	body, err := json.Marshal(record)
	if err != nil {
		return APIRecord{}, err
	}

	payload := bytes.NewReader(body)

	resp, err := c.do(ctx, http.MethodPut, endpoint, payload)

	if err != nil {
		return APIRecord{}, err
	}
	defer resp.Body.Close()

	var result PostRecordResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return APIRecord{}, err
	}

	return result.Data, nil
}

// APIDeleteRecord updates a single DNS record through the Liara API
// and returns the deleted record.
func (c client) APIDeleteRecord(ctx context.Context, zone string, id string) error {

	endpoint := c.BaseURL + fmt.Sprintf("zones/%s/dns-records/%s", zone, id)

	_, err := c.do(ctx, http.MethodDelete, endpoint, nil)

	if err != nil {
		return err
	}

	return nil
}

// A helper function to handle the repetitive portions of the request process to
// Liara.
func (c client) do(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Liara_api_token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {

		var apiErr APIError

		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return nil, fmt.Errorf("api request failed with status %s", resp.Status)
		}

		return nil, fmt.Errorf(
			"api request failed: %s",
			apiErr.Error(),
		)
	}

	return resp, nil
}
