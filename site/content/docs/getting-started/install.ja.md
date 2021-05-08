# Copilot のインストール

AWS Copilotは、[Homebrew](https://brew.sh/) を使ってインストールするか、バイナリを直接ダウンロードしてインストールすることができます。

## Homebrew でインストール 🍻

```sh
brew install aws/tap/copilot-cli
```

## 手動でインストール
以下のコマンドをターミナルにコピー＆ペーストして実行します。

=== "macOS"

    | インストール用コマンド    |
    | :---------- |
    | `curl -Lo copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-darwin && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help` |
    
=== "Linux x86 (64-bit)"

    | インストール用コマンド    |
    | :---------- |
    | `curl -Lo copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-linux && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help` |
    
=== "Linux (ARM)"
    
    | インストール用コマンド    |
    | :---------- |
    | `curl -Lo copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-linux-arm64 && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot && copilot --help` |


=== "Windows"

    | インストール用コマンド    |
    | :---------- |
    | `Invoke-WebRequest -OutFile 'C:\Program Files\copilot.exe' https://github.com/aws/copilot-cli/releases/latest/download/copilot-windows.exe` |

    !!! tip
        より快適にご利用いただくために、[Windows Terminal](https://github.com/microsoft/terminal) をご利用ください。権限の問題が発生した場合は、ターミナルを管理者として実行していることを確認してください。


!!! info
    特定のバージョンをダウンロードするには、"latest" を特定のバージョンに置き換えてください。例えば、macOS で v0.6.0 をダウンロードするには、次のように入力します。
    ```
    curl -Lo copilot https://github.com/aws/copilot-cli/releases/download/v0.6.0/copilot-darwin && chmod +x copilot && sudo mv copilot /usr/local/bin/copilot &&  copilot --help
    ```
