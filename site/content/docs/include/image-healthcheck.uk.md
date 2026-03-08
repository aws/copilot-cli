<span class="parent-field">image.</span><a id="image-healthcheck" href="#image-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
Необовʼязкова конфігурація для перевірки стану контейнера.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-cmd" href="#image-healthcheck-cmd" class="field">`command`</a> <span class="type">Array of Strings</span>  
Команда для визначення стану контейнера. Масив рядків може починатися з `CMD` для виконання аргументів команди безпосередньо або `CMD-SHELL` для запуску команди з використанням стандартної оболонки контейнера.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-interval" href="#image-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
Період часу між перевірками стану, в секундах. Стандартно 10с.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-retries" href="#image-healthcheck-retries" class="field">`retries`</a> <span class="type">Integer</span>  
Кількість спроб перед тим, як контейнер буде визнано несправним. Стандартно 2.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-timeout" href="#image-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
Час очікування перед тим, як перевірка стану буде визнана невдалою, в секундах. Стандартно 5с.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-start-period" href="#image-healthcheck-start-period" class="field">`start_period`</a> <span class="type">Duration</span>  
Тривалість періоду підготовки контейнера перед тим, як невдалі перевірки стану будуть враховані в максимальну кількість спроб. Стандартно 0с.
