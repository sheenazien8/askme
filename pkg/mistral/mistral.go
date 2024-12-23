package mistral

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type MistralRequest struct {
	Model    string           `json:"model"`
	Messages []MistralMessage `json:"messages"`
	Stream   bool             `json:"stream"`
}

type MistralMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type MistralResponse struct {
	Id      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	Model   string          `json:"model"`
	Choices []MistralChoice `json:"choices"`
}

type MistralChoice struct {
	Index        int          `json:"index"`
	Delta        MistralDelta `json:"delta"`
	FinishReason *string      `json:"finish_reason"`
}

type MistralDelta struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func StreamMistralRequest(apiKey, model, prompt string, responseChan chan<- string) error {
	requestBody := MistralRequest{
		Model:  model,
		Stream: true,
		Messages: []MistralMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to create JSON payload: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.mistral.ai/v1/chat/completions", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Mistral: %v", err)
	}
	defer resp.Body.Close()

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

		var mistralResp MistralResponse
		err = json.Unmarshal(line[startIndex:], &mistralResp)
		if err != nil {
			return fmt.Errorf("failed to parse Mistral response: %v", err)
		}

		if len(mistralResp.Choices) > 0 {
			choice := mistralResp.Choices[0]
			if choice.Delta.Content != "" {
				responseChan <- choice.Delta.Content
			}
			if choice.FinishReason != nil && *choice.FinishReason == "stop" {
				close(responseChan)
				break
			}
		}
	}

	return nil
}

