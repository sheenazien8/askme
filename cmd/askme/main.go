package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"askme/pkg/config"
	"askme/pkg/form"
	"askme/pkg/mistral"
	"askme/pkg/ollama"
	"askme/pkg/spinner"
	"askme/pkg/utils"

	"github.com/spf13/pflag"
)

func main() {
	config, err := config.ReadConfig()
	if err != nil {
		fmt.Printf("Warning: Could not read config file: %v\n", err)
	}

	provider := pflag.StringP("provider", "p", config.Provider, "Provider to use (ollama or mistral)")
	model := pflag.StringP("model", "m", config.DefaultModel, "Model to use (can be set in config)")
	file := pflag.StringP("file", "f", "", "File to use as input")
	help := pflag.BoolP("help", "h", false, "Show help information")
	roleSystem := pflag.StringP("role", "r", config.RoleSystem, "Role to use for the system message")
	chatMode := pflag.BoolP("chat", "c", false, "Enter chat mode (interactive conversation)")

	pflag.Parse()

	if *help {
		utils.PrintUsage()
		os.Exit(0)
	}
	// Interactive chat mode
	if *chatMode {
		err := runChat(config, *provider, *model, *roleSystem)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	var prompt string
	if pflag.NArg() > 0 {
		prompt = pflag.Arg(0)
	}

	if prompt == "" {
		form := form.BuildPromptForm(&prompt)
		err := form.Run()
		if err != nil {
			fmt.Printf("Error: Could not run form: %v\n", err)
			os.Exit(1)
		}
	}

	if *model == "" {
		fmt.Println("Error: No model specified. Set a default model in config or use --model")
		utils.PrintUsage()
		os.Exit(1)
	}

	spinner := spinner.NewSpinner()
	spinner.Start()

	responseChan := make(chan string, 1)
	errChan := make(chan error, 1)

	fmt.Printf("Using provider: %s\n", *provider)

	go func() {
		switch *provider {
		case "ollama":
			if !utils.IsOllamaInstalled() {
				err := utils.InstallOllama()
				if err != nil {
					errChan <- fmt.Errorf("failed to install Ollama: %v", err)
				}
			}
			err := ollama.StreamOllamaRequest(ollama.Args{
				URL:    config.OllamaURL,
				Model:  *model,
				Prompt: prompt,
				Role:   *roleSystem,
			}, responseChan)
			if err != nil {
				errChan <- err
			}
		case "mistral":
			err := mistral.StreamMistralRequest(mistral.Args{
				Model:  *model,
				Role:   *roleSystem,
				Prompt: prompt,
				ApiKey: config.MistralAPIKey,
				File:   *file,
			}, responseChan)
			if err != nil {
				errChan <- err
			}
		default:
			errChan <- fmt.Errorf("unsupported provider: %s", *provider)
		}
	}()

	select {
	case firstResponse := <-responseChan:
		spinner.Stop()

		fmt.Printf("Question: %s\n", prompt)
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

func runChat(cfg *config.Config, provider, model, roleSystem string) error {
	if model == "" {
		fmt.Println("Error: No model specified. Set a default model in config or use --model")
		utils.PrintUsage()
		os.Exit(1)
	}
	if provider != "ollama" && provider != "mistral" {
		return fmt.Errorf("unsupported provider: %s", provider)
	}
	if provider == "ollama" && !utils.IsOllamaInstalled() {
		if err := utils.InstallOllama(); err != nil {
			return fmt.Errorf("failed to install Ollama: %v", err)
		}
	}
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Entering chat mode (type 'exit' or 'quit' to stop).")
	var mistralHistory []mistral.MistralMessage
	var transcript string
	if provider == "mistral" && roleSystem != "" {
		mistralHistory = append(mistralHistory, mistral.MistralMessage{Role: "system", Content: roleSystem})
	}
	for {
		fmt.Print(">>> ")
		if !scanner.Scan() {
			fmt.Println()
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println("Exiting chat mode.")
			break
		}
		spinner := spinner.NewSpinner()
		responseChan := make(chan string, 1)
		errChan := make(chan error, 1)
		if provider == "mistral" {
			history := append(mistralHistory, mistral.MistralMessage{Role: "user", Content: input})
			args := mistral.Args{
				ApiKey:   cfg.MistralAPIKey,
				Model:    model,
				Messages: history,
			}
			go func() {
				if err := mistral.StreamMistralRequest(args, responseChan); err != nil {
					errChan <- err
				}
			}()
		} else {
			promptText := transcript + "User: " + input + "\nAssistant: "
			args := ollama.Args{
				URL:    cfg.OllamaURL,
				Model:  model,
				Prompt: promptText,
				Role:   roleSystem,
			}
			go func() {
				if err := ollama.StreamOllamaRequest(args, responseChan); err != nil {
					errChan <- err
				}
			}()
		}
		spinner.Start()
		select {
		case first := <-responseChan:
			spinner.Stop()
			fmt.Print(first)
			var respBuilder strings.Builder
			respBuilder.WriteString(first)
			for tok := range responseChan {
				fmt.Print(tok)
				respBuilder.WriteString(tok)
			}
			fmt.Println()
			finalResp := respBuilder.String()
			if provider == "mistral" {
				// update mistral history
				mistralHistory = append(mistralHistory, mistral.MistralMessage{Role: "user", Content: input})
				mistralHistory = append(mistralHistory, mistral.MistralMessage{Role: "assistant", Content: finalResp})
			} else {
				transcript += "User: " + input + "\nAssistant: " + finalResp + "\n"
			}
		case err := <-errChan:
			spinner.Stop()
			fmt.Printf("Error: %v\n", err)
		}
	}
	return nil
}
