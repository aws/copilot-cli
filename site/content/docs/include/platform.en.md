<div class="separator"></div>

<a id="platform" href="#platform" class="field">`platform`</a> <span class="type">String or Map</span>  
Operating system and architecture (formatted as `[os]/[arch]`) to pass with `docker build --platform`. For example, `linux/arm64` or `windows/x86_64`. The default is `linux/x86_64`.

Override the generated string to build with a different valid `osfamily` or `architecture`. For example, Windows users might change the string
```yaml
platform: windows/x86_64
```
which defaults to `WINDOWS_SERVER_2019_CORE`, using a map:
```yaml
platform:
  osfamily: windows_server_2019_full
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
