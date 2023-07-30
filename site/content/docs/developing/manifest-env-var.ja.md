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

Manifest ファイルにおいて、環境変数が次の様に使われている場合、`Array of Strings` から補完できます。

```yaml
network:
  vpc:
    security_groups: ${SECURITY_GROUPS}
```

シェルが環境変数 `SECURITY_GROUPS=["sg-06b511534b8fa8bbb","sg-06b511534b8fa8bbb","sg-0e921ad50faae7777"]` を持っている場合、 Manifest　の例は次の様に解釈されます。

```yaml
network:
  vpc:
    security_groups:
      - sg-06b511534b8fa8bbb
      - sg-06b511534b8fa8bbb
      - sg-0e921ad50faae7777
```

!!! Info
    現時点では、Manifest 内の文字列型フィールドに対してのみシェル環境変数を代入できます。`String` (例 `image.location`), `Array of Strings` (例 `entrypoint`), `Map` で値のタイプが `String` または、`Array of Strings` (e.g., `secrets`) などがそれに当たります。

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
