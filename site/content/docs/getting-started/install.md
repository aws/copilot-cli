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
    | `curl -Lo copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-darwin && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help` |
    
=== "Linux x86 (64-bit)"

    | Command to install    |
    | :---------- |
    | `curl -Lo copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-linux && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help` |
    
=== "Linux (ARM)"
    
    | Command to install    |
    | :---------- |
    | `curl -Lo copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-linux-arm64 && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help` |


=== "Windows"

    | Command to install    |
    | :---------- |
    | `Invoke-WebRequest -OutFile 'C:\Program Files\copilot.exe' https://github.com/aws/copilot-cli/releases/latest/download/copilot-windows.exe` |

    !!! tip
        Please use the [Windows Terminal](https://github.com/microsoft/terminal) to have the best user experience. If you encounter permissions issues, ensure that you are running your terminal as an administrator.


!!! info
    To download a specific version, replace "latest" with the specific version. For example, to download v0.6.0 on macOS, type:
    ```
    curl -Lo copilot https://github.com/aws/copilot-cli/releases/download/v0.6.0/copilot-darwin && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot &&  copilot --help
    ```