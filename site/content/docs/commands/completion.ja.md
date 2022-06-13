# completion
```console
$ copilot completion [shell] [flags]
```

## コマンドの概要
`copilot completion` は、bash や zsh、fish のシェル補完コードを表示します。コマンドをインタラクティブに補完するためには、このコードが評価されなければなりません。

それぞれのシェルで自動補完を設定する方法については、ヘルプメニューを参照してください。

## フラグ
```
-h, --help   help for completion
```

## 例
zsh の補完をインストールします。
```console
$ source <(copilot completion zsh)
$ copilot completion zsh > "${fpath[1]}/_copilot" # to autoload on startup
```
bash の補完を homebrew を使って macOS にインストールします。
```console
$ brew install bash-completion   # if running 3.2
$ brew install bash-completion@2 # if running Bash 4.1+
$ copilot completion bash > /usr/local/etc/bash_completion.d
```
bash の補完を Linux にインストールします。
```console
$ source <(copilot completion bash)
$ copilot completion bash > copilot.sh
$ sudo mv copilot.sh /etc/bash_completion.d/copilot
```
fish の補完を Linux にインストールします。
```console
$ source <(copilot completion fish)
$ copilot completion fish > ~/.config/fish/completions/copilot.fish
```
