---
title: Публікація/Підписка
---

# Архітектура Публікації/Підписки

[Worker Services](../../manifest/worker-service/) в Copilot використовують поле `publish`, спільне для всіх типів сервісів та завдань, щоб дозволити користувачам легко створювати логіку публікації/підписки (publish/subscribe) для передачі повідомлень між сервісами.

Поширеним шаблоном в AWS є комбінація SNS та SQS для доставки та обробки повідомлень. [SNS](https://docs.aws.amazon.com/sns/latest/dg/welcome.html) — це надійна система доставки повідомлень, яка може надсилати повідомлення до різних підписаних кінцевих точок з гарантіями доставки повідомлень.

[SQS](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html) — це черга повідомлень, яка дозволяє асинхронну обробку повідомлень. Черги можуть наповнюватися одним або кількома темами SNS або фільтрами подій AWS EventBridge.

Поєднання цих двох сервісів ефективно розділяє відправку та отримання повідомлень, що означає, що видавці не повинні турбуватися про те, які черги фактично підписані на їхні теми, а код сервісу-обробника не повинен турбуватися про те, звідки надходять повідомлення.

## Надсилання повідомлень від видавця

Щоб дозволити наявному сервісу публікувати повідомлення в SNS, просто встановіть поле `publish` в його маніфесті. Ми рекомендуємо використовувати назву для теми, яка описує її функцію.

```yaml
# manifest.yml for api service
name: api
type: Backend Service

publish:
  topics:
    - name: ordersTopic
```

Це створить [тему SNS](https://docs.aws.amazon.com/sns/latest/dg/welcome.html) та встановить політику ресурсів на тему, щоб дозволити чергам SQS у вашому обліковому записі AWS створювати підписки.

Copilot також вставляє ARNs будь-якої теми SNS у ваш контейнер під змінною середовища `COPILOT_SNS_TOPIC_ARNS`. JSON-рядок має формат:

```json
{
  "firstTopicName": "arn:aws:sns:us-east-1:123456789012:firstTopic",
  "secondTopicName": "arn:aws:sns:us-east-1:123456789012:secondTopic",
}
```

### Приклад на Javascript

Після розгортання сервісу публікації ви можете надсилати повідомлення до SNS через AWS SDK для SNS.

```javascript
const { SNSClient, PublishCommand } = require("@aws-sdk/client-sns");
const client = new SNSClient({ region: "us-west-2" });
const {ordersTopic} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS);
const out = await client.send(new PublishCommand({
   Message: "hello",
   TopicArn: ordersTopic,
 }));
```

## Підписка на тему з Worker Service

Щоб підписатися на наявну тему SNS за допомогою сервісу-обробника, вам потрібно відредагувати маніфест сервісу-обробника. Використовуючи поле [`subscribe`](../../manifest/worker-service/#subscribe) у маніфесті, ви можете визначити підписки на наявні теми SNS, які надаються іншими сервісами у вашому середовищі. У цьому прикладі ми використаємо тему `ordersTopic`, яку надав сервіс `api` з попереднього розділу. Ми також налаштуємо чергу сервісу-обробника, щоб увімкнути чергу для невдалих повідомлень. Поле `tries` вказує SQS, скільки разів спробувати повторно доставити невдале повідомлення перед відправкою його до черги для подальшого аналізу.

```yaml
name: orders-worker
type: Worker Service

subscribe:
  topics:
    - name: ordersTopic
      service: api
  queue:
    dead_letter:
      tries: 5
```

Copilot створить підписку між чергою цього сервісу-обробника та темою `ordersTopic` з сервісу `api`. Він також вставить URI черги у контейнер сервісу під змінною середовища `COPILOT_QUEUE_URI`.

Якщо ви вказуєте одну або кілька черг, специфічних для теми, ви можете отримати доступ до цих URI черг через змінну `COPILOT_TOPIC_QUEUE_URIS`. Ця змінна є JSON map від унікального ідентифікатора черги, специфічної для теми, до її URI.

Наприклад, сервіс-обробник з чергою, специфічною для теми `orders` з сервісу `merchant`, та FIFO тема `transactions` з сервісу `merchant` матиме наступну структуру JSON.

```json
// COPILOT_TOPIC_QUEUE_URIS
{
  "merchantOrdersEventsQueue": "https://sqs.eu-central-1.amazonaws.com/...",
  "merchantTransactionsfifoEventsQueue": "https://sqs.eu-central-1.amazonaws.com/..."
}
```

### Приклад на Javascript

Центральна бізнес-логіка контейнера сервісу-обробника полягає у витягуванні повідомлень з черги. Щоб зробити це за допомогою AWS SDK, ви можете використовувати клієнти SQS для вашої мови програмування. У Javascript логіка витягування, обробки та видалення повідомлень з черги виглядатиме наступним чином.

```javascript
const { SQSClient, ReceiveMessageCommand, DeleteMessageCommand } = require("@aws-sdk/client-sqs");
const client = new SQSClient({ region: "us-west-2" });
const out = await client.send(new ReceiveMessageCommand({
            QueueUrl: process.env.COPILOT_QUEUE_URI,
            WaitTimeSeconds: 10,
}));

console.log(`results: ${JSON.stringify(out)}`);

if (out.Messages === undefined || out.Messages.length === 0) {
    return;
}

// Process the message here.

await client.send( new DeleteMessageCommand({
    QueueUrl: process.env.COPILOT_QUEUE_URI,
    ReceiptHandle: out.Messages[0].ReceiptHandle,
}));
```
