<div class="separator"></div>

<a id="deployment" href="#deployment" class="field">`deployment`</a> <span class="type">Map</span>  
Розділ deployment містить параметри для контролю кількості завдань, що виконуються під час розгортання, та порядку зупинки та запуску завдань.

<span class="parent-field">deployment.</span><a id="deployment-rolling" href="#deployment-rolling" class="field">`rolling`</a> <span class="type">String</span>  
Стратегія розгортання є rolling. Дійсні значення:

- `"default"`: Створює нові завдання в кількості, що відповідає бажаній кількості, з оновленим визначенням завдання, перед зупинкою старих завдань. Під капотом це означає встановлення [`minimumHealthyPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#minimumHealthyPercent) на 100 і [`maximumPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#maximumPercent) на 200.
- `"recreate"`: Зупиняє всі запущені завдання, а потім запускає нові завдання. Під капотом це означає встановлення [`minimumHealthyPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#minimumHealthyPercent) на 0 і [`maximumPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#maximumPercent) на 100.

<span class="parent-field">deployment.</span><a id="deployment-rollback-alarms" href="#deployment-rollback-alarms" class="field">`rollback_alarms`</a> <span class="type">Array of Strings або Map</span>
!!! info "Інформація"
    Якщо на початку розгортання сигнал тривоги знаходиться в стані "In alarm", Amazon ECS НЕ буде відстежувати сигнали тривоги протягом усього розгортання. Для отримання додаткової інформації читайте документацію [тут](https://docs.aws.amazon.com/AmazonECS/latest/userguide/deployment-alarm-failure.html).

Як список рядків, імена наявних сигналів тривоги CloudWatch, які потрібно повʼязати з вашим сервісом, що можуть викликати [відкат розгортання](https://docs.aws.amazon.com/AmazonECS/latest/userguide/deployment-alarm-failure.html).

```yaml
deployment:
  rollback_alarms: ["MyAlarm-ELB-4xx", "MyAlarm-ELB-5xx"]
```

У вигляді map — метрика тривоги та поріг для тривог, створених Copilot. Доступні метрики:
