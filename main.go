package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

type Config struct {
	DefaultModel string `yaml:"default_model"`
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type Spinner struct {
	stop    chan struct{}
	stopped chan struct{}
}

func NewSpinner() *Spinner {
	return &Spinner{
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (s *Spinner) Start() {
	go func() {
		spinChars := []string{"|", "/", "-", "\\"}
		i := 0
		for {
			select {
			case <-s.stop:
				close(s.stopped)
				return
			default:
				fmt.Printf("\r%s Generating response", spinChars[i])
				i = (i + 1) % len(spinChars)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (s *Spinner) Stop() {
	close(s.stop)
	<-s.stopped
	fmt.Print("\r")
}

func readConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %v", err)
	}

	configPath := filepath.Join(homeDir, ".config", "askme", "config.yaml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{}, nil
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

func isOllamaInstalled() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

func installOllama() error {
	// TODO: add installation logic here
	cmd := exec.Command("sh", "-c", "echo 'Installing Ollama...'")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	if !isOllamaInstalled() {
		fmt.Println("Ollama is not installed.")
		fmt.Print("Do you want to install it now? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if response == "y" || response == "Y" {
			err := installOllama()
			if err != nil {
				fmt.Printf("Error: Could not install Ollama: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Ollama installed successfully.")
		} else {
			fmt.Println("Please install Ollama and try again.")
			os.Exit(1)
		}
	}

	config, err := readConfig()
	if err != nil {
		fmt.Printf("Warning: Could not read config file: %v\n", err)
	}

	model := pflag.StringP("model", "m", config.DefaultModel, "Ollama model to use (can be set in config)")
	help := pflag.BoolP("help", "h", false, "Show help information")

	pflag.Parse()

	if *help {
		printUsage()
		os.Exit(0)
	}

	var prompt string
	if pflag.NArg() > 0 {
		// Use the first non-flag argument as the prompt
		prompt = pflag.Arg(0)
	}

	if prompt == "" {
		form := buildPromptForm(&prompt)
		err := form.Run()
		if err != nil {
			fmt.Printf("Error: Could not run form: %v\n", err)
			os.Exit(1)
		}
	}

	if *model == "" {
		fmt.Println("Error: No model specified. Set a default model in config or use --model")
		printUsage()
		os.Exit(1)
	}

	spinner := NewSpinner()
	spinner.Start()

	responseChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		err := streamOllamaRequest(*model, prompt, responseChan)
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case firstResponse := <-responseChan:
		spinner.Stop()

		fmt.Print(firstResponse)

		for response := range responseChan {
			fmt.Print(response)
		}

	case err := <-errChan:
		spinner.Stop()

		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func buildPromptForm(prompt *string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Placeholder("Enter a prompt").
				Value(prompt).
				Validate(func(s string) error {

					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("A prompt is required")
					}
					return nil
				}),
		),
	)
}

func streamOllamaRequest(model, prompt string, responseChan chan<- string) error {
	requestBody := OllamaRequest{
		Model:  model,
		Prompt: prompt,
	}

	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to create JSON payload: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:11434/api/generate", bytes.NewBuffer(jsonPayload))
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

func printUsage() {
	fmt.Println("Usage: askme [--model=<model_name>] <prompt>")
	fmt.Println("\nOptions:")
	pflag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  askme \"Explain Go channels\"")
	fmt.Println("  askme -m codegemma \"What are goroutines?\"")
}
