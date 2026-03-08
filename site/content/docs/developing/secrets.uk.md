---
title: Секрети
---

# Секрети

Секрети — це конфіденційні дані, такі як токени OAuth, секретні ключі або ключі API — інформація, яка потрібна у вашому коді, але яку не слід зберігати у вихідному коді. У AWS Copilot CLI секрети передаються як змінні середовища (детальніше про [розробку зі змінними середовища](../../developing/environment-variables/)), але через їх конфіденційний характер з ними поводяться інакше.

## Як додати Секрети?

Додавання секретів вимагає зберігання їх в [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) (SSM)
або в [AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/latest/userguide/intro.html), а потім додавання посилання на секрет у вашому [маніфесті](../../manifest/overview/).

Ви можете легко створити секрет в SSM як `SecureString` за допомогою [`copilot secret init`](../../commands/secret-init/)!

## Використання власних Секретів

### В SSM

Якщо ви хочете використовувати власні секрети, обовʼязково додайте два теґи до ваших секретів:

| Ключ                    | Значення                                                    |
| ----------------------- | ----------------------------------------------------------- |
| `copilot-application`   | Назва програми, з якої ви хочете отримати доступ до секрету|
| `copilot-environment`   | Назва середовища, з якого ви хочете отримати доступ до секрету|

Copilot вимагає теґи `copilot-application` та `copilot-environment` для обмеження доступу до цього секрету.

Припустимо, у вас є (правильно позначений теґами!) параметр SSM з назвою `GH_WEBHOOK_SECRET` зі значенням `secretvalue1234`. Ви можете змінити ваш файл маніфесту, щоб передати це значення:

```yaml
secrets:
  GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET
```

Після розгортання цього оновленого маніфесту, ваш сервіс або завдання зможуть отримати доступ до змінної середовища `GITHUB_WEBHOOK_SECRET`, яка матиме значення параметра SSM `GH_WEBHOOK_SECRET`, `secretvalue1234`. Це працює тому, що агент ECS вирішить параметр SSM при запуску вашого завдання і встановить змінну середовища для вас.

### В Secrets Manager

Подібно до SSM, спочатку переконайтеся, що ваш секрет у Secrets Manager має теґи `copilot-application` та `copilot-environment`.

Припустимо, у вас є секрет у Secrets Manager з наступною конфігурацією:

| Поле   | Значення                                                                                                                                                                 |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Name  | `mysql`                                                                                                                                                     |
| ARN    | `arn:aws:secretsmanager:us-west-2:111122223333:secret:demo/test/mysql-Yi6mvL`                                                                                        |
| Value | `{"engine": "mysql","username": "user1","password": "i29wwX!%9wFV","host": "my-database-endpoint.us-east-1.rds.amazonaws.com","dbname": "myDatabase","port": "3306"`} |
| Tags   | `copilot-application=demo`, `copilot-environment=test` |


Ви можете змінити ваш файл маніфесту наступним чином:

```yaml
secrets:
  # Варіант 1. Звернення до секрету за назвою, якщо назва вашого секрету не закінчується дефісом, за яким слідують 6 символів (наприклад, mysql). Якщо закінчується (наприклад, mysql-dbconf), див. Варіант 2.
  DB:
    secretsmanager: 'mysql'
  # Ви можете посилатися на конкретний ключ у блобі JSON.
  DB_PASSWORD:
    secretsmanager: 'mysql:password::'

  # Варіант 2. Звернення до секрету за назвою, з випадковим 6-символьним суфіксом.
  # Якщо назва секрету містить дефіс і 6 літер (наприклад, mysql-dbconf замість mysql), то ви повинні додати 6-символьний суфікс. Інакше secretsmanager не зможе знайти ваш секрет.
  MYSQL_DB:
    secretsmanager: 'demo/test/mysql-dbconf-Vi3nwL'

  # Варіант 3. Як варіант, ви можете посилатися на секрет за ARN.
  DB: "'arn:aws:secretsmanager:us-west-2:111122223333:secret:demo/test/mysql-Yi6mvL'"
```
