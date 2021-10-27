<div class="separator"></div>

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String or Array of Strings</span>  
コンテナイメージのデフォルトエントリポイントをオーバーライドします。
```yaml
# 文字列による指定。
entrypoint: "/bin/entrypoint --p1 --p2"
# あるいは文字列配列による指定も可能。
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```
