<div class="separator"></div>

<a id="nlb" href="#nlb" class="field">`nlb`</a> <span class="type">Map</span>  
Розділ nlb містить параметри, повʼязані з інтеграцією вашого сервісу з Network Load Balancer.

Network Load Balancer активується лише якщо ви вказали поле `nlb`. Зауважте, що для Load-Balanced Web Service має бути увімкнений принаймні один із балансувальників навантаження — Application Load Balancer або Network Load Balancer.

<span class="parent-field">nlb.</span><a id="nlb-port" href="#nlb-port" class="field">`port`</a> <span class="type">String</span>  
Обовʼязковий параметр. Порт та протокол, які Network Load Balancer буде прослуховувати.

Підтримувані протоколи включають `tcp`, `udp` та `tls`. Якщо протокол не вказано, стандартно використовується `tcp`. Наприклад:

```yaml
nlb:
  port: 80
```

буде прослуховувати порт 80 для запитів `tcp`. Це те саме, що і

```yaml
nlb:
  port: 80/tcp
```

Ви можете легко увімкнути термінування TLS. Наприклад:

```yaml
nlb:
  port: 443/tls
```

<span class="parent-field">nlb.</span><a id="nlb-healthcheck" href="#nlb-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
Вкажіть конфігурацію перевірки стану для вашого Network Load Balancer.

```yaml
nlb:
  healthcheck:
    port: 80
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
```

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-port" href="#nlb-healthcheck-port" class="field">`port`</a> <span class="type">String</span>  
Порт, на який надсилаються запити перевірки стану. Вкажіть це, якщо перевірка стану повинна виконуватися на іншому порту, ніж порт цільового контейнера.

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-healthy-threshold" href="#nlb-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
Кількість послідовних успішних перевірок стану, необхідних для визнання несправної цілі справною. Стандартно 3. Діапазон: 2-10.

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-unhealthy-threshold" href="#nlb-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
Кількість послідовних невдалих перевірок стану, необхідних для визнання цілі несправною. Стандартно 3. Діапазон: 2-10.

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-grace-period" href="#nlb-healthcheck-grace-period" class="field">`grace_period`</a> <span class="type">Duration</span>  
Час, протягом якого ігноруються невдалі перевірки стану цільової групи при запуску контейнера. Стандартно 60s. Це може бути корисно для розвʼязання проблем з розгортанням контейнерів, які потребують часу для досягнення справного стану та початку прослуховування вхідних зʼєднань, або для прискорення розгортання контейнерів, які гарантовано швидко запускаються.

!!! info "Інформація"
    Відповідно до [документації](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/target-group-health-checks.html) на момент написання, 'unhealthy threshold' повинен дорівнювати 'healthy threshold' для Network Load Balancer.

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-interval" href="#nlb-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
Приблизний час, у секундах, між перевірками стану окремої цілі. Значення може бути 10s або 30s. Стандартно 30s.

<span class="parent-field">nlb.healthcheck.</span><a id="nlb-healthcheck-timeout" href="#nlb-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
Час, у секундах, протягом якого відсутність відповіді від цілі означає невдалу перевірку стану. Стандартно 10s.

<span class="parent-field">nlb.</span><a id="nlb-target-container" href="#nlb-target-container" class="field">`target_container`</a> <span class="type">String</span>  
Контейнер-sidecar, який замінює контейнер сервісу.

<span class="parent-field">nlb.</span><a id="nlb-target-port" href="#nlb-target-port" class="field">`target_port`</a> <span class="type">Integer</span>  
Порт контейнера, який отримує трафік. Вкажіть це поле, якщо порт контейнера відрізняється від `nlb.port`, порту прослуховувача.

<span class="parent-field">nlb.</span><a id="nlb-ssl-policy" href="#nlb-ssl-policy" class="field">`ssl_policy`</a> <span class="type">String</span>  
Політика безпеки, яка визначає, які протоколи та шифри підтримуються. Щоб дізнатися більше, дивіться [цей документ](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/create-tls-listener.html#describe-ssl-policies).

<span class="parent-field">nlb.</span><a id="nlb-stickiness" href="#nlb-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
Вказує, чи увімкнені сесії з липкістю.

<span class="parent-field">nlb.</span><a id="nlb-alias" href="#nlb-alias" class="field">`alias`</a> <span class="type">String or Array of Strings</span>  
Аліаси домену для вашого сервісу.

```yaml
# Версія з рядком.
nlb:
  alias: example.com
# Альтернативно, як масив рядків.
nlb:
  alias: ["example.com", "v1.example.com"]
```
<span class="parent-field">nlb.</span><a id="nlb-additional-listeners" href="#nlb-additional-listeners" class="field">`additional_listeners`</a> <span class="type">Array of Maps</span>  
Налаштуйте кілька прослуховувачів NLB.

{% include 'nlb-additionallisteners.uk.md' %}
