package utils

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/pflag"
)

func IsOllamaInstalled() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

func InstallOllama() error {
	// TODO: add installation logic here
	cmd := exec.Command("sh", "-c", "echo 'Installing Ollama...'")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func PrintUsage() {
	fmt.Println("Usage: askme [--model=<model_name>] <prompt>")
	fmt.Println("\nOptions:")
	pflag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  askme \"Explain Go channels\"")
	fmt.Println("  askme -m codegemma \"What are goroutines?\"")
}

