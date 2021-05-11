# pipeline update
```bash
$ copilot pipeline update [flags]
```

## コマンドの概要
`copilot pipeline update` は、ワークスペース内に全ての Service がデプロイ対象となるように Pipeline を作成/更新します。あわせてこの Pipeline のデプロイターゲットが Pipeline 用 Manifest にて Application と紐付けられた Environment 群となるように作成/更新されます。

## フラグ
```bash
-h, --help   help for update
    --yes    Skips confirmation prompt.
```

## 実行例
ワークスペース内の Service 群をデプロイ対象とする形で Pipeline を作成/更新します。
```bash
$ copilot pipeline update
```
