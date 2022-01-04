# Manifest における環境変数

## シェル環境変数
シェルの環境変数を使用して、Manifest に値を渡すことが可能です。

``` yaml
image:
  location: id.dkr.ecr.zone.amazonaws.com/project-name:${TAG}
```

シェルが環境変数 `TAG=version01` を持っている場合、Manifest は次のように解釈されます。

```yaml
image:
  location: id.dkr.ecr.zone.amazonaws.com/project-name:version01
```

これにより、Copilot は `id.dkr.ecr.zone.amazonaws.com/project-name` の tag `version01` であるコンテナイメージを利用してサービスをデプロイします。

!!! Info
    現時点では、Manifest 内の文字列型フィールドに対してのみシェル環境変数を代入できます。`String` (例 `image.location`), `Array of Strings` (例 `entrypoint`), `Map` で値のタイプが `String` (e.g., `secrets`) などがそれに当たります。

## 予約済み変数
予約済み変数は、Manifest を解釈する際に Copilot によって事前定義された環境変数です。現在、利用可能な予約済み変数は次のとおりです。

- COPILOT_APPLICATION_NAME
- COPILOT_ENVIRONMENT_NAME

```yaml
secrets:
   DB_PASSWORD: /copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/db_password
```

Copilotは、`${COPILOT_APPLICATION_NAME}`と`${COPILOT_ENVIRONMENT_NAME}`を、ワークロードがデプロイされたアプリケーションと環境の名前に置き換えます。
たとえば、次のように実行した場合、

```
$ copilot svc deploy --app my-app --env test
```

service のデプロイに environment を `test`、applicatoon を `my-app` と指定しているため、Copilot は `/copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/db_password` を `/copilot/my-app/test/secrets/db_password` と解釈します。
(アプリケーションに秘密情報を渡す方法の詳細については、[こちら](../developing/secrets.ja.md))を参照してください。)
