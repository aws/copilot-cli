<div class="separator"></div>

<a id="platform" href="#platform" class="field">`platform`</a> <span class="type">String або Map</span>  
Операційна система та архітектура (формат `[os]/[arch]`), які передаються з `docker build --platform`. Наприклад, `linux/arm64` або `windows/x86_64`. Стандартно використовується `linux/x86_64`.

Перевизначте згенерований рядок, щоб створити з іншою дійсною `osfamily` або `architecture`. Наприклад, користувачі Windows можуть змінити рядок

```yaml
platform: windows/x86_64
```

який стандартно використовується `WINDOWS_SERVER_2019_CORE`, використовуючи map:

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
