package mistral

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

type MistralRequest struct {
	Model    string           `json:"model"`
	Messages []MistralMessage `json:"messages"`
	Stream   bool             `json:"stream"`
}

type MistralMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type MistralFileMessage map[string]interface{}

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

// Args contains parameters for a Mistral API request, including optional chat history.
type Args struct {
   ApiKey   string
   Model    string
   // Prompt is the user input for one-shot requests; ignored if Messages is provided.
   Prompt   string
   // File path for file-based input; ignored in chat mode.
   File     string
   // Role is the system role content for one-shot requests; ignored if Messages is provided.
   Role     string
   // Messages is the slice of chat messages for multi-turn conversations; if non-empty, Prompt and Role are ignored.
   Messages []MistralMessage
}

func StreamMistralRequest(args Args, responseChan chan<- string) error {
   // Build messages: use provided history if available, else single-turn conversation
   var messages []MistralMessage
   if len(args.Messages) > 0 {
       messages = args.Messages
   } else {
       messages = []MistralMessage{
           {Role: "system", Content: args.Role},
           {Role: "user", Content: args.Prompt},
       }
   }
   requestBody := MistralRequest{
       Model:    args.Model,
       Stream:   true,
       Messages: messages,
   }

	apiClient := vortex.New(vortex.Opt{
		BaseURL: "https://api.mistral.ai",
	})

	resp, err := apiClient.
		SetHeaders(map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", args.ApiKey),
		}).
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
		}).
		Post("/v1/chat/completions", requestBody)

	if err != nil {
		log.Printf("Generate curl command: %s", resp.Request.GenerateCurlCommand())

		return fmt.Errorf("failed to send request to Mistral API: %v", err)
	}

	return nil
}
