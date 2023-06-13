# YAML パッチオーバーライド

{% include 'overrides-intro.ja.md' %}

## CDK オーバーライドよりも YAML パッチを使用するのはどの様な場合ですか？

どちらのオプションも Copilot [Manifest](../../manifest/overview.ja.md)によって表面化されない機能にアクセスして設定するという"ガラスを壊す"仕組みです。

1) 他のツールやフレームワーク(例えば、[Node.js](https://nodejs.org) や 
[CDK](https://docs.aws.amazon.com/cdk/v2/guide/home.html))に依存したく無い場合、
2) ほんの少しだけの修正がしたい場合には、YAML　パッチをお勧めします。

## 始め方

`copilot [noun] override` コマンドを実行すると、YAML パッチを使って CloudFormation テンプレートを拡張できます。
例えば、`copilot svc override` コマンドにより、 Load Balanced Web Service のテンプレートを更新します。
コマンドは、以下の様な構造で、`copilot/[name]/override` ディレクトリ配下にサンプルの `cfn.patches.yml` ファイルを作成します。

## どの様な仕組みでしょうか？

`cfn.patches.yml` の構文は、[RFC6902: JSON Patch](https://www.rfc-editor.org/rfc/rfc6902) に準拠します。
現在は、CLI は 3 つのオプションをサポートします: `add`, `remove`, and `replace`. 以下はサンプルの `cfn.patches.yml` ファイルです:

```yaml
- op: add
  path: /Mappings
  value:
    ContainerSettings:
      test: { Cpu: 256, Mem: 512 }
      prod: { Cpu: 1024, Mem: 1024}
- op: remove
  path: /Resources/TaskRole
- op: replace
  path: /Resources/TaskDefinition/Properties/ContainerDefinitions/1/Essential
  value: false
- op: add
  path: /Resources/Service/Properties/ServiceConnectConfiguration/Services/0/ClientAliases/-
  value:
    Port: !Ref TargetPort
    DnsName: yamlpatchiscool
```

各パッチは、CloudFormation テンプレートに順次適用されます。適用されたテンプレートは次のパッチのターゲットになります。
すべてのパッチが正常に適用されるか、エラーが発生するまで、評価は継続されます。

### パスの評価

`path` フィールドに対するパッチは [RFC6901: JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901) 構文に準拠します。 

- 各 `path` の値は `/` 文字で区切られ、対象の CloudFormation プロパティに到達した時点で評価が停止します。
- 対象のパスが配列であった場合、参照トークンは以下のいずれかでなければなりません:
    - 0 から始まる数字で構成される文字。
    - 配列へ追加するために、`add` 操作を実施する場合、`-` という 1 文字を指定する

## 追加の例

既存リソースに対して、新しいプロパティを追加する場合:

```yaml
- op: add
  path: /Resources/LogGroup/Properties/Tags
  value:
    - Key: keyname
      Value: value1
```

配列の特定のインデックスに新しいプロパティを追加する場合:

```yaml
- op: add
  path: /Resources/TaskDefinition/Properties/ContainerDefinitions/0/EnvironmentFiles/0
  value: arn:aws:s3:::bucket_name/key_name
```

配列の最後に新しい要素を追加する場合:

```yaml
- op: add
  path: /Resources/TaskRole/Properties/Policies/-
  value:
    PolicyName: DynamoDBReader
    PolicyDocument:
      Version: "2012-10-17"
      Statement:
        - Effect: Allow
          Action:
            - dynamodb:Get*
          Resource: '*'
```

既存プロパティの値を置換する場合:

```yaml
- op: replace
  path: /Resources/LogGroup/Properties/RetentionInDays
  value: 60
```

配列から要素を削除する場合、正確なインデックスを参照する必要があります:

```yaml
- op: remove
  path: /Resources/ExecutionRole/Properties/Policies/0/PolicyDocument/Statement/1/Action/0
```

リソース全体を削除する場合:

```yaml
- op: remove
  path: /Resources/ExecutionRole
```
