<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span>  
オプションの引数で指定した Dockerfile からコンテナをビルドします。後述の [`image.location`](#image-location) フィールドとは排他的な使用となります。

このフィールドに String（文字列）を指定した場合、Copilot はそれを Dockerfile の場所を示すパスと解釈します。その際、指定したパスのディレクトリ部が Docker のビルドコンテキストであると仮定します。以下は build フィールドに文字列を指定する例です。
```yaml
image:
  build: path/to/dockerfile
```
これにより、イメージビルドの際に次のようなコマンドが実行されることになります: `$ docker build --file path/to/dockerfile path/to`

build フィールドには Map も利用できます。
```yaml
image:
  build:
    dockerfile: path/to/dockerfile
    context: context/dir
    target: build-stage
    cache_from:
      - image:tag
    args:
      key: value
```
この例は、Copilot は Docker ビルドコンテキストに context フィールドの値が示すディレクトリを利用し、args 以下のキーバリューのペアをイメージビルド時の --build-args 引数として渡します。上記例と同等の docker build コマンドは次のようになります:  
`$ docker build --file path/to/dockerfile --target build-stage --cache-from image:tag --build-arg key=value context/dir`.

Copilot はあなたの意図を理解するために最善を尽くしますので、記述する情報の省略も可能です。例えば、`context` は指定しているが `dockerfile` は未指定の場合、Copilot は Dockerfile が "Dockerfile" という名前で存在すると仮定しつつ、docker コマンドを `context` ディレクトリ以下で実行します。逆に `dockerfile` は指定しているが `context` が未指定の場合は、Copilot はあなたが `dockerfile` で指定されたディレクトリをビルドコンテキストディレクトリとして利用したいのだと仮定します。

すべてのパスはワークスペースのルートディレクトリからの相対パスと解釈されます。

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span>  
Dockerfile からコンテナイメージをビルドする代わりに、既存のコンテナイメージ名の指定も可能です。`image.location` と [`image.build`](#image-build) の同時利用はできません。
`location` フィールドの制約を含む指定方法は Amazon ECS タスク定義の [`image` パラメータ](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/task_definition_parameters.html#container_definition_image)のそれに従います。

!!! warning
    Windows コンテナイメージを指定する場合、Manifest に `platform: windows/amd64` を指定する必要があります。  
    ARM アーキテクチャベースのコンテナイメージを指定する場合、Manifest に `platform: linux/arm64` を指定する必要があります。

<span class="parent-field">image.</span><a id="image-credential" href="#image-credential" class="field">`credentials`</a> <span class="type">String</span>  
任意項目です。プライベートリポジトリの認証情報の ARN。`credentials` フィールドは、Amazon ECS タスク定義の [`credentialsParameter`](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/private-auth.html) と同じです。 

<span class="parent-field">image.</span><a id="image-labels" href="#image-labels" class="field">`labels`</a> <span class="type">Map</span>  
コンテナに付与したい [Docker ラベル](https://docs.docker.com/config/labels-custom-metadata/)を key/value の Map で指定できます。これは任意設定項目です。

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
