
<a id="port" href="#port" class="field">`port`</a> <span class="type">Integer</span>  
コンテナの公開するポート番号。(任意項目)

<a id="image" href="#image" class="field">`image`</a> <span class="type">String</span>  
サイドカーコンテナのイメージ URL。(任意項目)

<a id="credentialsParameter" href="#credentialsParameter" class="field">`credentialsParameter`</a> <span class="type">String</span>  
プライベートレジストリの認証情報を保存している秘密情報の ARN。(任意項目)

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
サイドカーコンテナの環境変数。(任意項目)

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>  
サイドカーコンテナで用いる秘密情報。(任意項目)

<a id="mount-points" href="#mount-points" class="field">`mount_points`</a> <span class="type">Array of Maps</span>  
サービスレベルで指定する EFS ボリュームのマウントパス。(任意項目)

<span class="parent-field">mount_points.</span><a id="mount-points-source-volume" href="#mount-points-source-volume" class="field">`source_volume`</a> <span class="type">String</span>  
サイドカーからマウントするときのソースボリューム。(任意項目)

<span class="parent-field">mount_points.</span><a id="mount-points-path" href="#mount-points-path" class="field">`path`</a> <span class="type">String</span>  
サイドカーからボリュームをマウントするときのパス。(任意項目)

<span class="parent-field">mount_points.</span><a id="mount-points-read-only" href="#mount-points-read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
サイドカーにボリュームに対する読み込みのみを許可するかどうか。(デフォルトでは true)

<a id="labels" href="#labels" class="field">`labels`</a> <span class="type">Map</span>  
コンテナに付与する Docker ラベル。(任意項目)

<a id="depends_on" href="#depends_on" class="field">`depends_on`</a> <span class="type">Map</span>  
このコンテナに適用するコンテナの依存関係。(任意項目)

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String or Array of Strings</span>  
サイドカーのデフォルトのエントリーポイントをオーバーライドします。
```yaml
# 文字列バージョン
entrypoint: "/bin/entrypoint --p1 --p2"
# 別の方法として、文字列配列の場合
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```

<a id="command" href="#command" class="field">`command`</a> <span class="type">String or Array of Strings</span>  
サイドカーのデフォルトコマンドを上書きします。

```yaml
# 文字列バージョン
command: ps au
# 別の方法として、文字列配列の場合
command: ["ps", "au"]
```
