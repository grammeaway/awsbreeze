# awsbreeze 
An AWS news feed that doesn't blow. 

awsbreeze is a TUI for reading the latest AWS news, without having to interact with the official "What's New" page. News are fetched from the AWS news RSS feed, and headlines are displayed in a format thats's easy to get an overview of. Read the full article in your browser with a single keypress. 

Built using Golang, and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework.

## Installation
### Installing with Go on your system
Clone the repository and run the following commands to install the dependencies and build the binary:

```bash
go install github.com/grammeaway/awsbreeze@latest
```
This will install the `awsbreeze` binary in your `$GOPATH/bin` directory. Make sure to add this directory to your `PATH` environment variable if it's not already included.

### Installing with pre-built binaries
Download the latest release matching your OS from the [releases page](https://github.com/grammeaway/awsbreeze/releases).

Unzip the downloaded file and move the `awsbreeze` binary to a directory in your `PATH`, such as `/usr/local/bin` on Linux or macOS, or `C:\Program Files\` on Windows.

### Installing the nightly build (through Go)
If you want to try the latest features and bug fixes, you can install the nightly build by running the following command:

```bash
go install github.com/grammeaway/awsbreeze@main
```

## Verifying the installation
After installing, you can verify that `awsbreeze` is installed correctly by running the following command in your terminal:

```bash
awsbreeze version
```

## Config 
awsbreeze stores its recently-read articles, in a cache file called `seen.json` in the user's cache directory. The config file is automatically created when you run the program for the first time. The program used to read the config from a file in the user's home directory, but this was changed to use the cache directory as of v0.0.4, to avoid cluttering the home directory with configuration files. For this reason, on launch, the program will check if the previous config file exists in the home directory, and if so, it will move it to the cache directory.

## Roadmap
If time permits, I plan to add the following features:

- [ ] Read the full article in the TUI
- [ ] Bookmark/save articles


## Contributing
If you want to contribute, feel free to open an issue or a pull request. 

