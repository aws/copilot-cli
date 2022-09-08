<div class="separator"></div>

<a id="platform" href="#platform" class="field">`platform`</a> <span class="type">String or Map</span>  
`docker build --platform` で渡すオペレーティングシステムとアーキテクチャ。（`[os]/[arch]` の形式で指定）  
例えば `linux/arm64` や `windows/x86_64` などが指定できます。デフォルトは `linux/x86_64` です。

異なる `osfamily` や `architecture` を明示的に指定することもできます。
例えば、Windows では以下の設定はデフォルトで `WINDOWS_SERVER_2019_CORE` が利用されますが

```yaml
platform: windows/x86_64
```

Map を使って `WINDOWS_SERVER_2019_FULL` を明示的に指定できます。

```yaml
platform:
  osfamily: windows_server_2019_full
  architecture: x86_64
```
```yaml
platform:
  osfamily: windows_server_2019_core
  architecture: x86_64
```
```yaml
platform:
  osfamily: windows_server_2022_core
  architecture: x86_64
```
```yaml
platform:
  osfamily: windows_server_2022_full
  architecture: x86_64
```
