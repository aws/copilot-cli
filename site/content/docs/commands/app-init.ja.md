# app init
```console
$ copilot app init [name] [flags]
```

## コマンドの概要
`copilot app init` はコマンドを実行したディレクトリ内に新しい [Application](../concepts/applications.ja.md) を作成します。 Service は Application 内に作成していきます。

質問への回答の後、CLI は、Service 用に作成されたインフラストラクチャーを管理する為の AWS Identity and Access Management ロールを作成します。作業ディレクトリ配下に新しいサブディレクトリ `copilot/` を確認できます。 `copilot` ディレクトリには Service 用の Manifest ファイルと追加のインフラストラクチャーが格納されます。

`copilot app init` はカスタムドメイン名や AWS タグを利用したい場合、パーミッションバウンダリー用の IAM ポリシーを指定したい場合に利用します。それ以外の典型的なケースでは、同じ動作をする `copilot init` を利用します。

## フラグ
Copilot CLI における全てのコマンドと同じ様に、必要なフラグを指定しなかった場合、必要な情報を全て入力する様に求められます。フラグを指定して情報を指定すると、プロンプトをスキップできます。
```
      --domain string                  Optional. Your existing custom domain name.
  -h, --help                           help for init
      --permissions-boundary           Optional. The name or ARN of an existing IAM policy with which to set a
                                       permissions boundary for all roles generated within the application.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
```
`--domain`　フラグは、 Application が利用している AWS アカウント上の Amazon Route 53 に登録されたドメイン名を指定します。これにより Application 内の全てのサービスが同じドメイン名を利用することが出来るようになります。次の例の様に Service に対してアクセス出来るようになります。[https://{svcName}.{envName}.{appName}.{domain}](https://{svcName}.{envName}.{appName}.{domain})

`--permissions-boundary` フラグは Application が利用している AWS アカウント上にある IAM ポリシーを指定できます。このポリシー名は、Copilot により作成される全ての IAM ポリシーに付加される ARN 名の一部になります。

`--resource-tags` フラグは、Application 内の全てのリソースに対してカスタム[タグ](https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html)を追加します。
コマンド例: `copilot app init --resource-tags department=MyDept,team=MyTeam`

## 実行例
"my-app"という名前の新しい Application を作成します。
```console
$ copilot app init my-app
```
Route 53 に登録済みの既存ドメイン名を利用して新しい Application を作成します。
```console
$ copilot app init --domain example.com
```
リソースタグを指定して新しい Application を作成します。
```console
$ copilot app init --resource-tags department=MyDept,team=MyTeam
```
## 出力例

![Running copilot app init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/app-init.edited.svg?sanitize=true)
