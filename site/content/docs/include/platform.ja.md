<div class="separator"></div>

<a id="platform" href="#platform" class="field">`platform`</a> <span class="type">String or Map</span>  
`docker build --platform` で渡すオペレーティングシステムとアーキテクチャ。（`[os]/[arch]` の形式で指定）  
例えば `linux/arm64` や `windows/x86_64` などが指定できます。デフォルトは `linux/x86_64` です。

自動生成された値を上書きして `osfamily` や `architecture` を使うこともできます。  
例えば、Windows ユーザーは以下の設定を

```yaml
platform: windows/x86_64
```

Map を使ってデフォルトで `WINDOWS_SERVER_2019_CORE` を利用するように変更できます。

```yaml
platform:
  osfamily: windows_server_2019_full
  architecture: x86_64
```
