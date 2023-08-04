# svc show
```console
$ copilot svc show
```

## コマンドの概要

`copilot svc show` はデプロイ済みの Service 情報を表示します。Service の種類に応じて、エンドポイント、設定、変数や Environment ごとの関連する S3 オブジェクトが含まれます。

## フラグ

```
-a, --app string        Name of the application.
-h, --help              help for show
    --json              Optional. Output in JSON format.
    --manifest string   Optional. Name of the environment in which the service was deployed;
                        output the manifest file used for that deployment.
-n, --name string       Name of the service.
    --resources         Optional. Show the resources in your service.
```

## 実行例
デプロイされた Environment で Service 設定を出力します。
```console
$ copilot svc show -n api
```

"prod" Environment に "api" Service をデプロイするために使用される Manifest ファイルを出力します。
```console
$ copilot svc show -n api --manifest prod
```

## 出力例

![Running copilot svc show](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-show.svg?sanitize=true)