# env deploy <span class="version" >を [v1.20.0](../../blogs/release-v120.ja.md) にて追加</span> 
```console
$ copilot env deploy
```

## コマンドの概要

`copilot env deploy` は、[Environment Manifest](../manifest/environment.ja.md) の設定を受け取り、Environment インフラをデプロイします。

## フラグ

```
  -a, --app string    Name of the application.
      --diff          Compares the generated CloudFormation template to the deployed stack.
      --force         Optional. Force update the environment stack template.
  -h, --help          help for deploy
  -n, --name string   Name of the environment.
      --no-rollback   Optional. Disable automatic stack
                      rollback in case of deployment failure.
                      We do not recommend using this flag for a
                      production environment.
```

## 実行例
デプロイメントを実行する前に、変更される内容を確認するために、`--diff` を使用します。

```console
$ copilot env deploy --name test --diff
~ Resources:
    ~ Cluster:
        ~ Properties:
            ~ ClusterSettings:
                ~ - (changed item)
                  ~ Value: enabled -> disabled

Continue with the deployment? (y/N)
```

!!!info "`copilot env package --diff`"
    デプロイメントを実行する必要がなく、差分だけを確認したい場合があります。
    `copilot env package --diff` は差分を表示してコマンドが終了します。
