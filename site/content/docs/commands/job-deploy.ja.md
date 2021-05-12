# job deploy
```bash
$ copilot job deploy
```

## 概要

`job deploy` はローカルのコードと設定を元に Job をデプロイします。

`job deploy` は以下のステップを実行します。

1. ローカルの Dockerfile をビルドしてコンテナイメージを作成
2. `--tag` あるいは git ディレクトリにいる場合は最新の git ハッシュ値を使ってコンテナイメージにタグ付け
3. ECR にコンテナイメージをプッシュ
4. Manifest ファイルと Addon をまとめて CloudFormation テンプレートを生成する。
5. ECS のタスク定義と Job を作成/更新

## フラグ

```bash
  -a, --app string                     Name of the application.
  -e, --env string                     Name of the environment.
  -h, --help                           help for deploy
  -n, --name string                    Name of the job.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The container image tag.
```

## 実行例

"report-gen" という Job を "test" Environment にデプロイします。
```bash
$ copilot job deploy --name report-gen --env test
```

追加のリソースタグを付与して Job をデプロイします。
```bash
$ copilot job deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`
```
