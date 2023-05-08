
<a id="port" href="#port" class="field">`port`</a> <span class="type">Integer</span>  
コンテナの公開するポート番号。(任意項目)

<a id="image" href="#image" class="field">`image`</a> <span class="type">String or Map</span> 
サイドカーコンテナのイメージ URL。(必須項目)

{% include 'image-config.ja.md' %}

<a id="essential" href="#essential" class="field">`essential`</a> <span class="type">Bool</span>
サイドカーコンテナが必須のコンテナかどうか。(任意項目。デフォルトでは true)

<a id="credentialsParameter" href="#credentialsParameter" class="field">`credentialsParameter`</a> <span class="type">String</span>  
プライベートレジストリの認証情報を保存している秘密情報の ARN。(任意項目)

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
サイドカーコンテナの環境変数。(任意項目)

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>  
サイドカーコンテナで用いる秘密情報。(任意項目)

<a id="envFile" href="#envFile" class="field">`env_file`</a> <span class="type">String</span>  
サイドカーコンテナに設定する環境変数を含むファイルのワークスペースのルートからのパス。環境変数ファイルに関する詳細については、[環境変数ファイルの指定に関する考慮事項](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations)を確認してください。


<a id="mount-points" href="#mount-points" class="field">`mount_points`</a> <span class="type">Array of Maps</span>  
サービスレベルで指定する EFS ボリュームのマウントパス。(任意項目)

<span class="parent-field">mount_points.</span><a id="mount-points-source-volume" href="#mount-points-source-volume" class="field">`source_volume`</a> <span class="type">String</span>  
サイドカーからマウントするときのソースボリューム。(必須項目)

<span class="parent-field">mount_points.</span><a id="mount-points-path" href="#mount-points-path" class="field">`path`</a> <span class="type">String</span>  
サイドカーからボリュームをマウントするときのパス。(必須項目)

<span class="parent-field">mount_points.</span><a id="mount-points-read-only" href="#mount-points-read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
サイドカーにボリュームに対する読み込みのみを許可するかどうか。(デフォルトでは true)

<a id="labels" href="#labels" class="field">`labels`</a> <span class="type">Map</span>  
コンテナに付与する Docker ラベル。(任意項目)

<a id="depends_on" href="#depends_on" class="field">`depends_on`</a> <span class="type">Map</span>  
このコンテナに適用するコンテナの依存関係。(任意項目)

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String or Array of Strings</span>  
サイドカーのデフォルトのエントリポイントをオーバーライドします。
```yaml
# 文字列で指定する場合
entrypoint: "/bin/entrypoint --p1 --p2"
# 別の方法として、文字列配列の場合
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```

<a id="command" href="#command" class="field">`command`</a> <span class="type">String or Array of Strings</span>  
サイドカーのデフォルトコマンドを上書きします。

```yaml
# 文字列で指定する場合
command: ps au
# 別の方法として、文字列配列の場合
command: ["ps", "au"]
```

<a id="healthcheck" href="#healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>
サイドカーコンテナヘルスチェックの設定。この設定はオプションです。

<span class="parent-field">healthcheck.</span><a id="healthcheck-cmd" href="#healthcheck-cmd" class="field">`command`</a> <span class="type">Array of Strings</span>
サイドカーコンテナが healthy であると判断するためのコマンド。
このフィールドに設定する文字列配列の最初のアイテムには、コマンド引数を直接実行するための `CMD`、あるいはコンテナのデフォルトシェルでコマンドを実行する `CMD-SHELL` が利用できます。

<span class="parent-field">healthcheck.</span><a id="healthcheck-interval" href="#healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>
各ヘルスチェックの実行間の秒単位の間隔です。デフォルト値は１０秒です。

<span class="parent-field">healthcheck.</span><a id="healthcheck-retries" href="#healthcheck-retries" class="field">`retries`</a> <span class="type">Integer</span>
コンテナが unhealthy と見なされるまでに、失敗したヘルスチェックを再試行する回数です。デフォルト値は２です。

<span class="parent-field">healthcheck.</span><a id="healthcheck-timeout" href="#healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>
ヘルスチェックの実行開始から失敗とみなすまでに待機する秒単位の期間です。デフォルト値は５秒です。

<span class="parent-field">healthcheck.</span><a id="healthcheck-start-period" href="#healthcheck-start-period" class="field">`start_period`</a> <span class="type">Duration</span>
ヘルスチェックの実行と失敗がリトライ回数としてカウントされ始める前に、コンテナに対して起動処理を済ませる猶予期間の長さです。秒単位で指定し、デフォルト値は０秒です。
