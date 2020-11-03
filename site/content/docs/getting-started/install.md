You can install AWS Copilot through [Homebrew](https://brew.sh/) or by downloading the binaries directly.

## Homebrew 🍻

```sh
brew install aws/tap/copilot-cli
```

## Manually
Copy and paste the command into your terminal.

=== "macOS"

    | Command to install    |
    | :---------- |
    | `curl -Lo /usr/local/bin/copilot https://github.com/aws/copilot-cli/releases/download/v0.5.0/copilot-darwin-v0.5.0 && chmod +x /usr/local/bin/copilot && copilot --help` |

=== "Linux"

    | Command to install    |
    | :---------- |
    | `curl -Lo /usr/local/bin/copilot https://github.com/aws/copilot-cli/releases/download/v0.5.0/copilot-linux-v0.5.0 && chmod +x /usr/local/bin/copilot && copilot --help` |

=== "Windows"

    | Command to install    |
    | :---------- |
    | `Invoke-WebRequest -OutFile 'C:\Program Files\copilot.exe' https://github.com/aws/copilot-cli/releases/download/v0.5.0/copilot-windows-v0.5.0.exe` |

    !!! note
        Please use the [Windows Terminal](https://github.com/microsoft/terminal) to have the best user experience. If you encounter permissions issues, ensure that you are running your terminal as an administrator.
