```go
# AskMe CLI

AskMe is a command-line interface (CLI) tool that allows you to generate responses based on a given prompt using a specified model. This tool is designed to be simple and easy to use, providing a seamless experience for querying and generating responses.

## Features

- Prompt-based interaction
- Configurable default model
- Spinner animation while generating responses
- Error handling and user prompts

## Installation

To install AskMe, clone the repository and build the Go application:

```sh
git clone https://github.com/sheenazien8/askme.git
cd askme-cli
make build
```

## Usage

You can use AskMe by running the `askme` command followed by your prompt. You can also specify a model using the `--model` flag.

### Basic Usage

```sh
./bin/askme "Explain Go channels"
```

### Specify a Model

```sh
./bin/askme --model codegen "What are goroutines?"
```

### Help

To display help information, use the `--help` flag:

```sh
./bin/askme --help
```

## Configuration

You can set a default model in the configuration file located at `~/.config/askme/config.yaml`. The configuration file should be in YAML format and contain the following structure:

```yaml
provider: ollama | mistral
default_model: your_default_model_name
mistral_api_key: <if you using mistral>
```

## Example

```sh
./bin/askme "What is the capital of France?"
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any improvements or bug fixes.

## Roadmap

Here are some planned features and improvements for future releases:

- [ ] Implement logic ollama installation
- [ ] Add unit tests and improve test coverage
- [ ] Enhance error handling and user feedback
- [x] Add provider support

## Acknowledgements

Special thanks to the developers and contributors of the libraries and tools used in this project.
