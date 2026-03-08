??? note "nlb.additional_listeners Map"
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-port" href="#nlb-additional-listeners-port" class="field">`port`</a> <span class="type">String</span>  
    Обовʼязковий. Додатковий порт та протокол, на якому буде прослуховувати Network Load Balancer.
    
    Підтримувані протоколи включають `tcp`, `udp` та `tls`. Якщо протокол не вказано, стандартно використовується `tcp`.
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-healthcheck" href="#nlb-additional-listeners-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
    Вкажіть конфігурацію перевірки справності для вашого додаткового слухача на Network Load Balancer.
    
    ```yaml
    nlb:
      additional_listeners:
        - healthcheck:
            port: 80
            healthy_threshold: 3
            unhealthy_threshold: 2
            interval: 15s
            timeout: 10s
    ```
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-port" href="#nlb-additional-listeners-healthcheck-port" class="field">`port`</a> <span class="type">String</span>  
    Порт, на який надсилаються запити перевірки справності. Вкажіть це, якщо перевірка справності повинна виконуватися на іншому порту, ніж цільовий порт контейнера.
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-healthy-threshold" href="#nlb-additional-listeners-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
    Кількість послідовних успішних перевірок справності, необхідних перед тим, як вважати несправну ціль справною. Стандартно - 3. Діапазон: 2-10.
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-unhealthy-threshold" href="#nlb-additional-listeners-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
    Кількість послідовних невдалих перевірок справності, необхідних перед тим, як вважати ціль несправною. Стандартно - 3. Діапазон: 2-10.
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-interval" href="#nlb-additional-listeners-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
    Приблизний час у секундах між перевірками справності окремої цілі. Значення може бути 10s або 30s. Стандартно - 30s.
    
    <span class="parent-field">nlb.additional_listeners.healthcheck.</span><a id="nlb-additional-listeners-healthcheck-timeout" href="#nlb-additional-listeners-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
    Час у секундах, протягом якого відсутність відповіді від цілі означає невдалу перевірку справності. Стандартно - 10s.
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-target-container" href="#nlb-additional-listeners-target-container" class="field">`target_container`</a> <span class="type">String</span>  
    Sidecar-контейнер, який заміняє контейнер сервісу.
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-target-port" href="#nlb-additional-listeners-target-port" class="field">`target_port`</a> <span class="type">Integer</span>  
    Порт контейнера, який приймає трафік. Вкажіть це поле, якщо порт контейнера відрізняється від `nlb.port`, порту слухача.
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-ssl-policy" href="#nlb-additional-listeners-ssl-policy" class="field">`ssl_policy`</a> <span class="type">String</span>  
    Політика безпеки, яка визначає, які протоколи та шифри підтримуються. Щоб дізнатися більше, перегляньте [цю документацію](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/create-tls-listener.html#describe-ssl-policies).
    
    <span class="parent-field">nlb.additional_listeners.</span><a id="nlb-additional-listeners-stickiness" href="#nlb-additional-listeners-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
    Вказує, чи увімкнені липкі сесії.
