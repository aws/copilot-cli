# secret init
```console
$ copilot secret init
```

## コマンドの概要
`copilot secret init` は SSM パラメータストア内にアプリケーション用の [SecureString パラメータ](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/systems-manager-parameter-store.html#what-is-a-parameter)を作成・更新します.

シークレットは既存の Environment ごとに異なる値を設定でき、各 Environment のシークレットは同じ Application/Environment で実行される Service または Job からアクセスできます。

!!! attention 
    Request-Driven Web Service はシークレットの利用をサポートしていません。

## フラグ
```
  -a, --app string              Name of the application.
      --cli-input-yaml string   Optional. A YAML file in which the secret values are specified.
                                Mutually exclusive with the -n, --name and --values flags.
  -h, --help                    help for init
  -n, --name string             The name of the secret.
                                Mutually exclusive with the --cli-input-yaml flag.
      --overwrite               Optional. Whether to overwrite an existing secret.
      --values stringToString   Values of the secret in each environment. Specified as <environment>=<value> separated by commas.
                                Mutually exclusive with the --cli-input-yaml flag. (default [])
```
## 使用例
インタラクティブにシークレットを作成します。コマンドを実行すると、シークレットの名前、そして各 Environment ごとの値を尋ねられます。
```console
$ copilot secret init
```

`db_password` という名前のシークレットを複数の Environment に作成します。コマンドを実行すると、各 Environment ごとの `db_password` の値を尋ねられます。
```console
$ copilot secret init --name db_password
```
`input.yml` からシークレットを作成します。YAML ファイルのフォーマットは<a href="#secret-init-cli-input-yaml">ページ下部</a>をご覧ください。
```console
$ copilot secret init --cli-input-yaml input.yml
```

!!!info
    `--values` フラグはシークレットの値を指定する方法として便利ですが、シェルの履歴でその値が確認できてしまう可能性があります。これを避けるため、`copilot secret init --name` コマンドの質問への回答の形で値を指定するか、あるいは `--cli-input-yaml` フラグを利用してファイルから値を読み込むことを推奨します。

## 作成したシークレットをアプリケーションから参照する

Copilot は `/copilot/<app name>/<env name>/secrets/<secret name>` という名前の SSM パラメータを作成します。
[Service](../manifest/backend-service.ja.md#secrets) あるいは [Job](../manifest/scheduled-job.ja.md#secrets) Manifest の `secrets` セクションでこのパラメータ名を指定することでこのシークレットをアプリケーションから参照できます。

例えば、`my-app` という Application があり、その `prod` と `dev` Environment に `db_host` というシークレットを作ったとすると、Service の Manifest は以下のようになるでしょう。
```yaml
environments:
    prod:
      secrets: 
        DB_PASSWORD: /copilot/my-app/prod/secrets/db_password
    dev:
      secrets:
        DB_PASSWORD: /copilot/my-app/dev/secrets/db_password
```
更新した Manifest をデプロイすると、Service あるいは Job は環境変数 `DB_PASSWORD` にアクセスできるようになります。
この環境変数の値には、`prod` Environment では `/copilot/my-app/prod/secrets/db_password` SSM パラメータの値が、`dev` Environment では `/copilot/my-app/dev/secrets/db_password` SSM パラメータの値がセットされることになります。

これは ECS コンテナエージェントが、コンテナの起動時に SSM パラメータの取得と環境変数への設定を自動的に行ってくれるためです。

## <span id="secret-init-cli-input-yaml">`--cli-input-yaml` フラグの使い方</span>
`--cli-input-yaml` フラグを利用すると、ファイルに Environment 別に定義したシークレットの設定を読み込むことができます。また、同一のファイル内には複数のシークレットを定義できます。Copilot はファイルを読み込んだ上で状況に応じてシークレットを作成、あるいは更新します。

YAML ファイルのフォーマットは以下のようにしてください。
```yaml
<secret A>:
  <env 1>: <the value of secret A in env 1>
  <env 2>: <the value of secret A in env 2>
  <env 3>: <the value of secret A in env 3>
<secret B>:
  <env 1>: <the value of secret B in env 1>
  <env 2>: <the value of secret B in env 2>
```

以下は `dev`、`test`、`prod` Environment に `db_host` と `db_password` というシークレットを作成し、`dev`、`test` Environment に `notification_email` というシークレットを作成する YAML の例です。
この例では `prod` Environment 用の `notification_email` が設定されていないため、`prod` Environment には `notification_email` が作成されません。
```yaml
db_host:
  dev: dev.db.host.com
  test: test.db.host.com
  prod: prod.db.host.com
db_password:
  dev: dev-db-pwd
  test: test-db-pwd
  prod: prod-db-pwd
notification_email:
  dev: dev@email.com
  test: test@email.com
```
