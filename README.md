# awsbreeze 
An AWS news feed that doesn't blow. 

awsbreeze is a TUI for reading the latest AWS news, without having to interact with the official "What's New" page. News are fetched from the AWS news RSS feed, and headlines are displayed in a format thats's easy to get an overview of. Read the full article in your browser with a single keypress. 

Built using Golang, and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework.

## Installing with Go on your system
Clone the repository and run the following commands to install the dependencies and build the binary:

```bash
go install github.com/grammeaway/awsbreeze@latest
```
This will install the `awsbreeze` binary in your `$GOPATH/bin` directory. Make sure to add this directory to your `PATH` environment variable if it's not already included.

## Installing with pre-built binaries
Download the latest release matching your OS from the [releases page](https://github.com/grammeaway/awsbreeze/releases).

Unzip the downloaded file and move the `awsbreeze` binary to a directory in your `PATH`, such as `/usr/local/bin` on Linux or macOS, or `C:\Program Files\` on Windows.

## Roadmap
If time permits, I plan to add the following features:

- [ ] Read the full article in the TUI
- [ ] Bookmark/save articles


## Contributing
If you want to contribute, feel free to open an issue or a pull request. 

