package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/sheenazien8/vortex"
)

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
    System string `json:"system"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type Args struct {
	URL    string
	Model  string
	Prompt string
	Role   string
}

func StreamOllamaRequest(args Args, responseChan chan<- string) error {
	requestBody := OllamaRequest{
		Model:  args.Model,
		Prompt: args.Prompt,
        System: args.Role,
	}

	apiClient := vortex.New(vortex.Opt{
		BaseURL: args.URL,
	})

	resp, err := apiClient.
		Stream(func(resp *http.Response) error {
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("Mistral API returned error status: %d", resp.StatusCode)
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

				startIndex := bytes.IndexByte(line, '{')
				if startIndex == -1 {
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
		}).
		Post("/", requestBody)

	if err != nil {
		log.Printf("Generate curl command: %s", resp.Request.GenerateCurlCommand())

		return fmt.Errorf("failed to send request to Mistral API: %v", err)
	}

	return nil
}
