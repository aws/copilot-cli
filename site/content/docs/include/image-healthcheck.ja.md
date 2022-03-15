<span class="parent-field">image.</span><a id="image-healthcheck" href="#image-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
コンテナヘルスチェックの設定。この設定はオプションです。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-cmd" href="#image-healthcheck-cmd" class="field">`command`</a> <span class="type">Array of Strings</span>  
コンテナが healthy であると判断するためのコマンド。  
このフィールドに設定する文字列配列の最初のアイテムには、コマンド引数を直接実行するための `CMD`、あるいはコンテナのデフォルトシェルでコマンドを実行する `CMD-SHELL` が利用できます。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-interval" href="#image-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
各ヘルスチェックの実行間の秒単位の間隔です。デフォルト値は１０秒です。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-retries" href="#image-healthcheck-retries" class="field">`retries`</a> <span class="type">Integer</span>  
コンテナが unhealthy と見なされるまでに、失敗したヘルスチェックを再試行する回数です。デフォルト値は２です。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-timeout" href="#image-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
ヘルスチェックの実行開始から失敗とみなすまでに待機する秒単位の期間です。デフォルト値は５秒です。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-start-period" href="#image-healthcheck-start-period" class="field">`start_period`</a> <span class="type">Duration</span> 
ヘルスチェックの実行と失敗がリトライ回数としてカウントされ始める前に、コンテナに対して起動処理を済ませる猶予期間の長さです。秒単位で指定し、デフォルトは０秒です。
