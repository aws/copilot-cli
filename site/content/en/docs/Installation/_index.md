---
title: "Installation"
linkTitle: "Installation"
weight: 3
---

You can install AWS Copilot through [Homebrew](https://brew.sh/) or by downloading the binaries directly.

## Homebrew 🍻

```sh
brew install aws/tap/copilot-cli
```

## Manually
Copy and paste the command into your terminal.

| Platform | Command to install |
|---------|---------
| macOS | `curl -Lo /usr/local/bin/copilot https://github.com/aws/copilot-cli/releases/download/v0.1.0/copilot-darwin-v0.1.0 && chmod +x /usr/local/bin/copilot && copilot --help` |
| Linux | `curl -Lo /usr/local/bin/copilot https://github.com/aws/copilot-cli/releases/download/v0.1.0/copilot-linux-v0.1.0 && chmod +x /usr/local/bin/copilot && copilot --help` |
