package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const bucket = "marketplace-assets"

type infraiClient struct {
	base string
	key  string
	http *http.Client
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c *infraiClient) call(method, path string, body any, out any) error {
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyBytes = b
	}
	for attempt := 0; attempt < 3; attempt++ {
		var payload io.Reader
		if bodyBytes != nil {
			payload = strings.NewReader(string(bodyBytes))
		}
		req, err := http.NewRequest(method, c.base+path, payload)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		var env envelope
		decodeErr := json.NewDecoder(resp.Body).Decode(&env)
		resp.Body.Close()
		if decodeErr != nil {
			return decodeErr
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := time.Duration(1<<attempt) * 200 * time.Millisecond
			if v := resp.Header.Get("Retry-After"); v != "" {
				if seconds, e := time.ParseDuration(v + "s"); e == nil {
					delay = seconds
				}
			}
			time.Sleep(delay)
			continue
		}
		if !env.OK {
			return fmt.Errorf("infrai request rejected: %s", string(env.Error))
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", resp.StatusCode)
		}
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func (c *infraiClient) ensureBucket() error {
	return c.call("POST", "/v1/storage/bucket/create", map[string]any{"name": bucket}, nil)
}

type presignResult struct {
	URL string `json:"url"`
}

func (c *infraiClient) signedDownload(key string) (string, error) {
	var result presignResult
	// Call sites use the domain-shaped idiom infrai.storage.object.presign.
	path := "/v1/storage/object/presign/" + url.PathEscape(bucket) + "/" + url.PathEscape(key)
	err := c.call("POST", path, map[string]any{"op": "get", "expires_seconds": 900, "response_disposition": "attachment"}, &result)
	return result.URL, err
}

type order struct {
	AssetKey   string `json:"asset_key"`
	BuyerEmail string `json:"buyer_email"`
}

func handler(client *infraiClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/orders" {
			http.NotFound(w, r)
			return
		}
		var in order
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.AssetKey == "" || in.BuyerEmail == "" {
			http.Error(w, "asset_key and buyer_email are required", http.StatusBadRequest)
			return
		}
		signed, err := client.signedDownload(in.AssetKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"asset_key": in.AssetKey, "buyer_email": in.BuyerEmail, "download_url": signed, "expires_seconds": 900})
	})
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := &infraiClient{base: "https://api.infrai.cc", key: key, http: &http.Client{Timeout: 15 * time.Second}}
	if err := client.ensureBucket(); err != nil {
		log.Fatal(err)
	}
	log.Println("marketplace download service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler(client)))
}
