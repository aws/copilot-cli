# svc deploy
```bash
$ copilot svc deploy
```

## コマンドの概要

`copilot svc deploy` は、ローカルのコードや設定を元にデプロイします。

Service デプロイの手順は以下の通りです。

1. ローカルの Dockerfile をビルドしてコンテナイメージを作成
2. `--tag` の値、または最新の git sha (git ディレクトリで作業している場合) を利用してタグ付け
3. コンテナイメージを ECR にプッシュ
4. Manifest ファイルとアドオンを CloudFormation にパッケージ
5. ECS タスク定義とサービスを作成 / 更新

## フラグ

```bash
  -e, --env string                     Name of the environment.
  -h, --help                           help for deploy
  -n, --name string                    Name of the service.
      --resource-tags stringToString   Optional. Labels with a key and value separated with commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The service's image tag.
```