package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func StreamOllamaRequest(url, model, prompt string, responseChan chan<- string) error {
	requestBody := OllamaRequest{
		Model:  model,
		Prompt: prompt,
	}

	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to create JSON payload: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API returned error status: %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading response: %v", err)
		}

		if len(line) == 0 {
			continue
		}

		var ollamaResp OllamaResponse
		err = json.Unmarshal(line, &ollamaResp)
		if err != nil {
			return fmt.Errorf("failed to parse Ollama response: %v", err)
		}

		if ollamaResp.Response != "" {
			responseChan <- ollamaResp.Response
		}

		if ollamaResp.Done {
			close(responseChan)
			break
		}
	}

	return nil
}
