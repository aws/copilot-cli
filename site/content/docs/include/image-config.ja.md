<div class="separator"></div>

<a id="image" href="#image" class="field">`image`</a> <span class="type">Map</span>  
image セクションは、Docker ビルドに関する設定や公開するポートについてのパラメータを含みます。

<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span>  
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

<span class="parent-field">image.</span><a id="image-port" href="#image-port" class="field">`port`</a> <span class="type">Integer</span>  
公開するポート番号。Dockerfile 内に `EXPOSE` インストラクションが記述されている場合、Copilot はそれをパースした値をここに挿入します。  

<span class="parent-field">image.</span><a id="image-labels" href="#image-labels" class="field">`labels`</a> <span class="type">Map</span>  
コンテナに付与したい [Docker ラベル](https://docs.docker.com/config/labels-custom-metadata/)を key/value の Map で指定できます。これは任意設定項目です。
