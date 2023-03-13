# 認証情報

AWS Copilot は、AWS API へのアクセス、[アプリケーションのメタデータ](concepts/applications.ja.md)の保存と検索、アプリケーションのワークロードの展開と運用に AWS クレデンシャルを使用します。

AWS認証情報の設定方法については、[AWS CLIのドキュメント](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html)で詳しく説明されています。
## Application 用の認証情報

Copilot は[デフォルトの認証情報プロバイダーチェーン](https://docs.aws.amazon.com/ja_jp/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials)に沿って認証情報を検索し、認証情報を使用して Service と Environment が関連づけられた [Application のメタデータ](concepts/applications.ja.md)を保存または検索します。
!!! tips
    **[名前付きプロファイル](https://docs.aws.amazon.com/ja_jp/cli/latest/userguide/cli-configure-profiles.html)を使用して** Application 用の認証情報を保存することを推奨します。

もっとも簡単な方法は `[default]` プロファイルを利用することです:

```ini
# ~/.aws/credentials
[default]
aws_access_key_id=AKIAIOSFODNN7EXAMPLE
aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# ~/.aws/config
[default]
region=us-west-2
```

あるいは `AWS_PROFILE` 環境変数を設定して別の名前付きプロファイルを指定できます。例えば Application 用に `[my-app]` という名前のプロファイルを `[default]` プロファイルの代わりに用いることができます。

!!! note
    Application に AWS アカウントのルートユーザーの認証情報を用いることは**できません**。代わりに[こちら](https://docs.aws.amazon.com/ja_jp/IAM/latest/UserGuide/id_root-user.html)で説明されているようにまず IAM ユーザーを作成してください。

```ini
# ~/.aws/config
[profile my-app]
credential_process = /opt/bin/awscreds-custom --username helen
region=us-west-2

# 指定した名前付きプロファイルを用いた Copilot コマンドの実行
$ export AWS_PROFILE=my-app
$ copilot deploy
```

!!! caution
    環境変数 `AWS_ACCESS_KEY_ID` , `AWS_SECRET_ACCESS_KEY` , `AWS_SESSION_TOKEN` を直接用いることは推奨**しません**。なぜならそれらの認証情報が上書きされたり失効したりすると Copilot が Service や Environment を検索できなくなるからです。

サポートされている `config` ファイルの設定についてもっと詳しく知りたい方は[設定ファイルと認証情報ファイルに関するドキュメント](https://docs.aws.amazon.com/ja_jp/cli/latest/userguide/cli-configure-files.html#cli-configure-files-settings)をご参照ください。

## Environment 用の認証情報

Copilot における [Environment](concepts/environments.ja.md) は Application が存在するのとは別の AWS アカウントやリージョンに作成できます。Environment を最初に作成する際 Copilot はどの一時クレデンシャルまたは[名前付きプロファイル](https://docs.aws.amazon.com/ja_jp/cli/latest/userguide/cli-configure-profiles.html)を使用するか尋ねます。

```bash
$ copilot env init

Name: prod-iad

  Which credentials would you like to use to create prod-iad?
  > Enter temporary credentials
  > [profile default]
  > [profile test]
  > [profile prod-iad]
  > [profile prod-pdx]
```

[Application 用の認証情報](#application-用の認証情報)とは異なり、Environment 用の AWS 認証情報は Environment の作成と削除の時のみ必要です。したがって一時的な環境変数からそれらの値を安全に使うことができます。Copilot が尋ねたり使用したりする認証情報はフラグとして扱われます。なぜならデフォルトの認証プロバイダーチェーンは Application 用の認証情報のために使われるからです。
