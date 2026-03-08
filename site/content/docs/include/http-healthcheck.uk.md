<span class="parent-field">http.</span><a id="http-healthcheck" href="#http-healthcheck" class="field">`healthcheck`</a> <span class="type">String або Map</span>  
Якщо ви вказуєте рядок, Copilot інтерпретує його як шлях, що відкритий у вашому контейнері для обробки запитів перевірки стану цільової групи. Стандартно "/".

```yaml
http:
  healthcheck: '/'
```

Ви також можете вказати healthcheck як map:

```yaml
http:
  healthcheck:
    path: '/'
    port: 8080
    success_codes: '200'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
    grace_period: 60s
```

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-path" href="#http-healthcheck-path" class="field">`path`</a> <span class="type">String</span>  
Місце призначення, куди надсилаються запити перевірки стану.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-port" href="#http-healthcheck-port" class="field">`port`</a> <span class="type">Integer</span>  
Порт, на який надсилаються запити перевірки стану. Стандартно це [`image.port`](./#image-port), або порт, відкритий [`http.target_container`](./#http-target-container), якщо встановлено.  
Якщо відкритий порт `443`, то протокол перевірки стану автоматично встановлюється на HTTPS.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-success-codes" href="#http-healthcheck-success-codes" class="field">`success_codes`</a> <span class="type">String</span>  
Коди стану HTTP, які справні цілі повинні використовувати при відповіді на запит перевірки стану HTTP. Ви можете вказати значення від 200 до 499. Ви можете вказати кілька значень (наприклад, "200,202") або діапазон значень (наприклад, "200-299"). Стандартно це 200.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-healthy-threshold" href="#http-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
Кількість послідовних успішних перевірок стану, необхідних для визнання несправної цілі справною. Стандартно це 5. Діапазон: 2-10.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
Кількість послідовних невдалих перевірок стану, необхідних для визнання цілі несправною. Стандартно це 2. Діапазон: 2-10.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
Приблизний час, у секундах, між перевірками стану окремої цілі. Стандартно це 30с. Діапазон: 5с–300с.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
Час, у секундах, протягом якого відсутність відповіді від цілі означає невдалу перевірку стану. Стандартно це 5с. Діапазон 5с-300с.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-grace-period" href="#http-healthcheck-grace-period" class="field">`grace_period`</a> <span class="type">Duration</span>  
Час, протягом якого ігноруються невдалі перевірки стану цільової групи при запуску контейнера. Стандартно це 60с. Це може бути корисно для розвʼязання проблем з розгортанням контейнерів, які потребують часу, щоб стати справними та почати слухати вхідні зʼєднання, або для прискорення розгортання контейнерів, які гарантовано швидко запускаються.
