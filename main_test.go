package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadConfig(t *testing.T) {
	// Create a temporary config file
	homeDir, err := os.UserHomeDir()
	assert.NoError(t, err)

	configDir := filepath.Join(homeDir, ".config", "askme")
	err = os.MkdirAll(configDir, 0755)
	assert.NoError(t, err)

	configPath := filepath.Join(configDir, "config.yaml")
	configContent := []byte("default_model: test_model")
	err = os.WriteFile(configPath, configContent, 0644)
	assert.NoError(t, err)

	defer os.Remove(configPath)

	config, err := readConfig()
	assert.NoError(t, err)
	assert.Equal(t, "test_model", config.DefaultModel)
}

func TestStreamOllamaRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req OllamaRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "test_model", req.Model)
		assert.Equal(t, "test_prompt", req.Prompt)

		resp := OllamaResponse{
			Response: "test_response",
			Done:     true,
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	responseChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		err := streamOllamaRequest("test_model", "test_prompt", responseChan)
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case response := <-responseChan:
		assert.Equal(t, "test_response", response)
	case err := <-errChan:
		assert.NoError(t, err)
	}
}

func TestSpinner(t *testing.T) {
	spinner := NewSpinner()
	assert.NotNil(t, spinner)

	go func() {
		time.Sleep(500 * time.Millisecond)
		spinner.Stop()
	}()

	spinner.Start()
}
