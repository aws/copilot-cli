# pipeline delete
```bash
$ copilot pipeline delete [flags]
```

## コマンドの概要
`copilot pipeline delete` は、ワークスペースに紐付いている Pipeline を削除します。

## フラグ
```bash
    --delete-secret   Deletes AWS Secrets Manager secret associated with a pipeline source repository.
-h, --help            help for delete
    --yes             Skips confirmation prompt.
```

## 実行例
ワークスペースに紐付いている Pipeline を削除します。
```bash
$ copilot pipeline delete
```
