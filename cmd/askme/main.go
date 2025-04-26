package main

import (
	"bufio"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"askme/pkg/config"
	"askme/pkg/form"
	"askme/pkg/mistral"
	"askme/pkg/ollama"
	"askme/pkg/spinner"
	"askme/pkg/utils"

	"github.com/spf13/pflag"
)

func buildContext(root string) (string, error) {
	var builder strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "bin" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		var ext string
		if strings.HasSuffix(path, ".blade.php") {
			ext = ".blade.php"
		} else {
			ext = filepath.Ext(path)
		}
		switch ext {
		case ".blade.php", ".go", ".md", ".txt", ".yaml", ".yml", ".php", ".env", ".json":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		builder.WriteString(fmt.Sprintf("File: %s (%d lines)\n", path, len(lines)))
		lang := ""
		switch ext {
		case ".blade.php":
			lang = "html"
		case ".go":
			lang = "go"
		case ".md":
			lang = "markdown"
		case ".yaml", ".yml":
			lang = "yaml"
		case ".php":
			lang = "php"
		case ".json":
			lang = "json"
		default:
			lang = ""
		}
		builder.WriteString("```" + lang + "\n")
		for i, line := range lines {
			builder.WriteString(fmt.Sprintf("%4d: %s\n", i+1, line))
		}
		builder.WriteString("```\n\n")
		return nil
	})
	return builder.String(), err
}

func applyChatContext(provider string, contextFlag bool, roleSystem string) (newRole string, initialHistory []mistral.MistralMessage, err error) {
	newRole = roleSystem
	if !contextFlag {
		return newRole, nil, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return newRole, nil, err
	}
	ctx, err := buildContext(cwd)
	if err != nil {
		return newRole, nil, err
	}
	header := "You are now in code context mode. Files listed below include their contents with line numbers. Refer to file paths and line numbers when answering."
	if provider == "mistral" {
		sys := header + "\n\n" + ctx
		initialHistory = []mistral.MistralMessage{{Role: "system", Content: sys}}
	} else {
		if newRole != "" {
			newRole = newRole + "\n\n" + header + "\n\n" + ctx
		} else {
			newRole = header + "\n\n" + ctx
		}
	}
	return newRole, initialHistory, nil
}

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
	contextFlag := pflag.Bool("ct", false, "Enable reading project context from root")

	pflag.Parse()

	if *help {
		utils.PrintUsage()
		os.Exit(0)
	}
	if *contextFlag && !*chatMode {
		fmt.Println("Error: --ct context mode only works with --chat. Use --chat or omit --ct.")
		os.Exit(1)
	}
	if *chatMode {
		err := runChat(config, *provider, *model, *roleSystem, *contextFlag)
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

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFA500")).
		Padding(0, 1).
		Foreground(lipgloss.Color("#FFFFFF"))
	info := fmt.Sprintf(
		" Provider: %s\n Model: %s\n Context Mode: %t ",
		*provider, *model, *contextFlag,
	)
	fmt.Println(style.Render(info))
	spinner := spinner.NewSpinner()
	spinner.Start()

	responseChan := make(chan string, 1)
	errChan := make(chan error, 1)

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
		var respBuilder strings.Builder
		respBuilder.WriteString(firstResponse)
		if !*contextFlag {
			fmt.Print(firstResponse)
		}
		for response := range responseChan {
			respBuilder.WriteString(response)
			if !*contextFlag {
				fmt.Print(response)
			}
		}
		fullResp := respBuilder.String()
		if *contextFlag {
			extractAndPrintCode(fullResp)
		}
		if hasDiffPatch(fullResp) {
			if promptApply() {
				if err := applyPatch(fullResp); err != nil {
					fmt.Printf("Failed to apply patch: %v\n", err)
					os.Exit(1)
				} else {
					fmt.Println("Patch applied successfully.")
				}
			}
		}

	case err := <-errChan:
		spinner.Stop()

		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func hasDiffPatch(resp string) bool {
	return strings.Contains(resp, "diff --git") || strings.Contains(resp, "*** Begin Patch") || strings.Contains(resp, "@@ ")
}

func promptApply() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Apply patch? [y/N]: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	return input == "y" || input == "Y" || strings.EqualFold(input, "yes")
}

func applyPatch(patch string) error {
	tmpFile, err := os.CreateTemp("", "askme-patch-*.diff")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(patch); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()
	cmd := exec.Command("git", "apply", tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func extractAndPrintCode(resp string) {
	re := regexp.MustCompile("(?s)```[\\w]*\\n(.*?)```")
	matches := re.FindAllStringSubmatch(resp, -1)
	if len(matches) > 0 {
		fmt.Println("// rest of code")
		for _, m := range matches {
			lines := strings.Split(m[1], "\n")
			for i, line := range lines {
				fmt.Printf("%4d: %s\n", i+1, line)
			}
		}
		fmt.Println("// rest of code")
	} else {
		fmt.Println(resp)
	}
}

func runChat(cfg *config.Config, provider, model, roleSystem string, contextFlag bool) error {
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
	var mistralHistory []mistral.MistralMessage
	var err error
	if contextFlag {
		roleSystem, mistralHistory, err = applyChatContext(provider, contextFlag, roleSystem)
		if err != nil {
			return err
		}
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFA500")).
		Padding(0, 1).
		Foreground(lipgloss.Color("#FFFFFF"))
	info := fmt.Sprintf(
		" Provider: %s\n Model: %s\n Context Mode: %t ",
		provider, model, contextFlag,
	)
	fmt.Println(style.Render(info))
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Entering chat mode (type 'exit' or 'quit' to stop).")
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
