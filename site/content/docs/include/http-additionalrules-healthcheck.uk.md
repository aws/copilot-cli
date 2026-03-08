<span class="parent-field">http.additional_rules.</span><a id="http-additional-rules-healthcheck" href="#http-additional-rules-healthcheck" class="field">`healthcheck`</a> <span class="type">String або Map</span>  
Якщо ви вказуєте рядок, Copilot інтерпретує його як шлях, відкритий у вашому контейнері для обробки запитів перевірки стану цільової групи. Стандартно "/".

```yaml
http:
    additional_rules:
    - healthcheck: '/'
```

Ви також можете вказати healthcheck як map:

```yaml
http:
    additional_rules:
    - healthcheck:
        path: '/'
        port: 8080
        success_codes: '200'
        healthy_threshold: 3
        unhealthy_threshold: 2
        interval: 15s
        timeout: 10s
```

<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-path" href="#http-additional-rules-healthcheck-path" class="field">`path`</a> <span class="type">String</span>  
    Місце призначення, куди надсилаються запити перевірки стану.

<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-port" href="#http-additional-rules-healthcheck-port" class="field">`port`</a> <span class="type">Integer</span>  
    Порт, на який надсилаються запити перевірки стану. Стандартно [`image.port`](./#image-port), або порт, відкритий [`http.target_container`](./#http-target-container), якщо встановлено.  
    Якщо відкритий порт `443`, то протокол перевірки стану автоматично встановлюється на HTTPS.

<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-success-codes" href="#http-additional-rules-healthcheck-success-codes" class="field">`success_codes`</a> <span class="type">String</span>  
    HTTP-коди стану, які повинні використовувати справні цілі при відповіді на перевірку стану HTTP. Можна вказати значення від 200 до 499. Можна вказати декілька значень (наприклад, "200,202") або діапазон значень (наприклад, "200-299"). Стандартно 200.

<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-healthy-threshold" href="#http-additional-rules-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
    Кількість послідовних успішних перевірок стану, необхідних перед тим, як вважати несправну ціль справною. Стандартно 5. Діапазон: 2-10.

<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-unhealthy-threshold" href="#http-additional-rules-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
    Кількість послідовних невдалих перевірок стану, необхідних перед тим, як вважати ціль несправною. Стандартно 2. Діапазон: 2-10.

<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-interval" href="#http-additional-rules-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
    Приблизний час у секундах між перевірками стану окремої цілі. Стандартно 30с. Діапазон: 5с–300с.

<span class="parent-field">http.additional_rules.healthcheck.</span><a id="http-additional-rules-healthcheck-timeout" href="#http-additional-rules-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
    Час у секундах, протягом якого відсутність відповіді від цілі означає невдалу перевірку стану. Стандартно 5с. Діапазон 5с-300с.
