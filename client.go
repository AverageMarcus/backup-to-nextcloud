package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	nextcloudURL string
	username     string
	password     string
}

type DirectoryItem struct {
	Directory    string
	FileName     string
	LastModified *time.Time
}

func NewClient(nextcloudURL, username, password string) *Client {
	return &Client{
		nextcloudURL: nextcloudURL,
		username:     username,
		password:     password,
	}
}

func (c *Client) CreateDestDirectory(ctx context.Context, destinationDirectory string) error {
	path := fmt.Sprintf("%s/remote.php/dav/files/%s/%s", c.nextcloudURL, c.username, destinationDirectory)
	method := "MKCOL"

	response, err := c.doRequest(ctx, path, method, nil, nil)
	if err != nil {
		if strings.Contains(response, "The resource you tried to create already exists") {
			// Directory already exists so safe to skip
			return nil
		}
	}
	return err
}

func (c *Client) UploadFile(ctx context.Context, destinationDirectory string, file *os.File) error {
	path := fmt.Sprintf("%s/remote.php/dav/files/%s/%s/%s", c.nextcloudURL, c.username, destinationDirectory, filepath.Base(file.Name()))
	method := "PUT"

	headers := map[string]string{}
	info, err := os.Stat(file.Name())
	if err == nil {
		headers["X-OC-Mtime"] = fmt.Sprintf("%d", info.ModTime().Unix())
	}

	_, err = c.doRequest(ctx, path, method, file, headers)
	return err
}

func (c *Client) ListDirectoryContents(ctx context.Context, destinationDirectory string) ([]DirectoryItem, error) {
	directoryItems := []DirectoryItem{}

	path := fmt.Sprintf("%s/remote.php/dav/files/%s/%s", c.nextcloudURL, c.username, destinationDirectory)
	method := "PROPFIND"

	response, err := c.doRequest(ctx, path, method, nil, nil)

	if err != nil {
		return directoryItems, err
	}

	var directoryContents Multistatus
	if err := xml.Unmarshal([]byte(response), &directoryContents); err != nil {
		logger.ErrorContext(ctx, "Error unmarshalling XML response",
			slog.String("destinationDirectory", destinationDirectory),
			slog.Any("error", err),
		)
		return directoryItems, err
	}

	hrefPrefix := fmt.Sprintf("/remote.php/dav/files/%s/%s/", c.username, destinationDirectory)

	for _, item := range directoryContents.Responses {
		if !item.IsCollection() {
			var lastModified *time.Time

			if item.GetLastModified() != "" {
				parsedDate, err := time.Parse(time.RFC1123, item.GetLastModified())
				if err == nil {
					lastModified = &parsedDate
				}
			}

			file := DirectoryItem{
				Directory:    destinationDirectory,
				FileName:     strings.TrimPrefix(item.Href, hrefPrefix),
				LastModified: lastModified,
			}
			directoryItems = append(directoryItems, file)
		}
	}

	return directoryItems, nil
}

func (c *Client) DeleteFile(ctx context.Context, destinationDirectory, fileName string) error {
	path := fmt.Sprintf("%s/remote.php/dav/files/%s/%s/%s", c.nextcloudURL, c.username, destinationDirectory, fileName)
	method := "DELETE"

	_, err := c.doRequest(ctx, path, method, nil, nil)
	return err
}

func (c *Client) doRequest(ctx context.Context, path, method string, payload *os.File, additionalHeaders map[string]string) (string, error) {
	client := &http.Client{}

	var err error
	var req *http.Request

	if payload != nil {
		req, err = http.NewRequestWithContext(ctx, method, path, payload)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, path, nil)
	}
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create request",
			slog.String("path", path),
			slog.String("method", method),
			slog.Any("error", err),
		)
		return "", err
	}

	req.SetBasicAuth(c.username, c.password)

	if payload != nil {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	for key, val := range additionalHeaders {
		req.Header.Set(key, val)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to perform request",
			slog.String("path", path),
			slog.String("method", method),
			slog.Any("error", err),
		)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		logger.ErrorContext(ctx, "Unexpected error response returned",
			slog.String("path", path),
			slog.String("method", method),
			slog.Int("statusCode", resp.StatusCode),
			slog.String("statusMessage", resp.Status),
			slog.String("response", string(bodyBytes)),
		)
		return string(bodyBytes), fmt.Errorf("Unexpected error response returned")
	} else {
		logger.DebugContext(ctx, "Request successful",
			slog.String("path", path),
			slog.String("method", method),
			slog.Int("statusCode", resp.StatusCode),
			slog.String("statusMessage", resp.Status),
		)
	}

	return string(bodyBytes), nil
}
