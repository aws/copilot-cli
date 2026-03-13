<span class="parent-field">image.</span><a id="image-depends-on" href="#image-depends-on" class="field">`depends_on`</a> <span class="type">Map</span>  
任意項目。コンテナに追加する [Container Dependencies](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/APIReference/API_ContainerDependency.html) の任意の key/value の Map。Map の key はコンテナ名で、value は依存関係を表す値 (依存条件) として `start`、`healthy`、`complete`、`success` のいずれかを指定できます。なお、必須コンテナに `complete` や `success` の依存条件を指定することはできません。

設定例:
```yaml
image:
  build: ./Dockerfile
  depends_on:
    nginx: start
    startup: success
```
上記の例では、タスクのメインコンテナは `nginx` サイドカーが起動し、`startup` コンテナが正常に完了してから起動します。
