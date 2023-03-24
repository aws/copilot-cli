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
