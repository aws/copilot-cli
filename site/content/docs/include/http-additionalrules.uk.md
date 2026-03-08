<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-path" href="#http-additional-rules-path" class="field">`path`</a> <span class="type">String</span>  
    Запити до цього шляху будуть перенаправлені до вашого сервісу. Кожне правило прослуховування повинно мати унікальний шлях.

{% include 'http-additionalrules-healthcheck.uk.md' %}

<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-deregistration-delay" href="#http-additional-rules-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>  
    Час очікування для цільових обʼєктів для завершення зʼєднань під час скасування реєстрації. Стандартно 60 секунд. Встановлення більшого значення дає цільовим обʼєктам більше часу для коректного завершення зʼєднань, але збільшує час, необхідний для нових розгортань. Діапазон 0с-3600с.

<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-target-container" href="#http-additional-rules-target-container" class="field">`target_container`</a> <span class="type">String</span>  
    Sidecar-контейнер, до якого направляються запити замість основного контейнера сервісу.  
    Якщо порт цільового контейнера встановлено на `443`, тоді протокол встановлюється як `HTTPS`, щоб балансувальник навантаження встановлював TLS-зʼєднання з завданнями Fargate, використовуючи сертифікати, які ви встановлюєте на цільовому контейнері.

<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-target-port" href="#http-additional-rules-target-port" class="field">`target_port`</a> <span class="type">String</span>  
    Порт контейнера, який приймає трафік. Вкажіть це поле, якщо порт контейнера відрізняється від `image.port` для основного контейнера або `sidecar.port` для sidecar-контейнерів.
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-stickiness" href="#http-additional-rules-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
    Вказує, чи увімкнені липкі сеанси.
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-allowed-source-ips" href="#http-additional-rules-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
  CIDR IP-адреси, яким дозволено доступ до вашого сервісу.

  ```yaml
  http:
    additional_rules:
      - allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
  ```

<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-alias" href="#http-additional-rules-alias" class="field">`alias`</a> <span class="type">String або Array of Strings або Array of Maps</span>  
HTTPS доменний псевдонім вашого сервісу.

```yaml
# Версія з рядком.
http:
  additional_rules:
    - alias: example.com
# Або, як масив рядків.
http:
  additional_rules:
    - alias: ["example.com", "v1.example.com"]
# Або, як масив з map.
http:
  additional_rules:
    - alias:
        - name: example.com
          hosted_zone: Z0873220N255IR3MTNR4
        - name: v1.example.com
          hosted_zone: AN0THE9H05TED20NEID
```
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-hosted-zone" href="#http-additional-rules-hosted-zone" class="field">`hosted_zone`</a> <span class="type">String</span>  
ID вашої наявної хостингової зони; може використовуватися лише з `http.alias` та `http.additional_rules.alias`. Якщо у вас є середовище з імпортованими сертифікатами, ви можете вказати хостингову зону, в яку Copilot повинен вставити A-запис після створення балансувальника навантаження.

```yaml
http:
  additional_rules:
    - alias: example.com
      hosted_zone: Z0873220N255IR3MTNR4
# Також дивіться приклад масиву з map для http.alias вище.
```
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-redirect-to-https" href="#http-additional-rules-redirect-to-https" class="field">`redirect_to_https`</a> <span class="type">Boolean</span>  
    Автоматично перенаправляти балансувальник навантаження застосунків з HTTP на HTTPS. Стандартно встановлено `true`.
    
<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-version" href="#http-additional-rules-version" class="field">`version`</a> <span class="type">String</span>  
    Версія протоколу HTTP(S). Має бути одним зі значень: `'grpc'`, `'http1'` або `'http2'`. Якщо не вказано, Стандартно використовується `'http1'`. Якщо використовується gRPC, зверніть увагу, що з вашим додатком має бути пов'язаний домен.
