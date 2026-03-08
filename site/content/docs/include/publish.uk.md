<div class="separator"></div>

<a id="publish" href="#publish" class="field">`publish`</a> <span class="type">Map</span>  
Розділ `publish` дозволяє сервісам публікувати повідомлення в одну або кілька тем SNS.

```yaml
publish:
  topics:
    - name: orderEvents
```

У наведеному вище прикладі цей маніфест оголошує тему SNS з назвою `orderEvents`, на яку можуть підписатися інші робочі сервіси, розгорнуті в середовищі Copilot. Змінна середовища під назвою `COPILOT_SNS_TOPIC_ARNS` вставляється у ваше робоче навантаження як рядок JSON.

На JavaScript ви можете написати:

```js
const {orderEvents} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS)
```

Для отримання додаткової інформації дивіться сторінку [pub/sub](../../developing/publish-subscribe/).

<span class="parent-field">publish.</span><a id="publish-topics" href="#publish-topics" class="field">`topics`</a> <span class="type">Array of topics</span>  
Список обʼєктів [`topic`](#publish-topics-topic).

<span class="parent-field">publish.topics.</span><a id="publish-topics-topic" href="#publish-topics-topic" class="field">`topic`</a> <span class="type">Map</span>  
Містить конфігурацію для однієї теми SNS.

<span class="parent-field">publish.topics.topic.</span><a id="publish-topics-topic-name" href="#publish-topics-topic-name" class="field">`name`</a> <span class="type">String</span>  
Обовʼязково. Назва теми SNS. Повинна містити лише великі та малі літери, цифри, дефіси та підкреслення.

<span class="parent-field">publish.topics.topic.</span><a id="publish-topics-topic-fifo" href="#publish-topics-topic-fifo" class="field">`fifo`</a> <span class="type">Boolean or Map</span>  
Конфігурація теми SNS FIFO (перший прийшов, перший пішов).  
Якщо ви вкажете `true`, Copilot створить тему з упорядкуванням FIFO.

```yaml
publish:
  topics:
    - name: mytopic
      fifo: true
```

Альтернативно, ви також можете налаштувати розширені параметри теми SNS FIFO.

```yaml
publish:
  topics:
    - name: mytopic
      fifo:
        content_based_deduplication: true
```

<span class="parent-field">publish.topics.topic.fifo.</span><a id="publish-topics-topic-fifo-content-based-deduplication" href="#publish-topics-topic-fifo-content-based-deduplication" class="field">`content_based_deduplication`</a> <span class="type">Boolean</span>   
Якщо тіло повідомлення гарантовано є унікальним для кожного опублікованого повідомлення, ви можете увімкнути дедуплікацію на основі вмісту для теми SNS FIFO.
