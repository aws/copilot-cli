---
title: Надсилання локальних артефактів
---

# Надсилання локальних артефактів <span class="version" > додано в [v1.21.0](../../../../blogs/release-v121/) </span>

Copilot підтримує завантаження локальних файлів, на які посилаються ваші шаблони надбудов, до S3, і заміну відповідних властивостей ресурсів на завантажене розташування S3.
При виконанні [`copilot svc deploy`](../../../commands/svc-deploy/) або [`copilot svc package --upload-assets`](../../../commands/svc-package/), певні поля підтримуваних ресурсів будуть оновлені з розташуванням S3 перед тим, як шаблон надбудов буде надісланий до CloudFormation. Ваші шаблони на диску не будуть змінені. Щоб побачити повний список підтримуваних ресурсів, перегляньте [документацію AWS CLI](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/cloudformation/package.html).

Ця функція може використовуватися для розгортання локальних функцій Lambda, що зберігаються в тому ж репозиторії, що й інший сервіс Copilot. Наприклад, щоб розгорнути JavaScript функцію Lambda разом із сервісом copilot, ви можете додати цей ресурс до вашого [шаблону надбудови](../workload/):

???+ note "Приклад функції Lambda"
    === "copilot/service-name/addons/lambda.yml"

        ```yaml hl_lines="4"
          ExampleFunction:
            Type: AWS::Lambda::Function
            Properties:
              Code: lambdas/example/
              Handler: "index.handler"
              Timeout: 900
              MemorySize: 512
              Role: !GetAtt "ExampleFunctionRole.Arn"
              Runtime: nodejs20.x
        ```

    === "lambdas/example/index.js"

        ```js
        exports.handler = function (event, context) {
            console.log('приклад події:', event);
            context.succeed('успіх!');
        };
        ```

При виконанні `copilot svc deploy`, тека `lambdas/example` буде заархівована та надіслана до S3, а властивість `Code` буде оновлена до:

```yaml
Code:
  S3Bucket: copilotBucket
  S3Key: hashOfLambdasExampleZip
```

перед тим, як шаблон надбудови буде завантажений та розгорнутий Copilot. Якщо ви вказуєте файл, файл буде безпосередньо завантажений до S3. Якщо ви вказуєте теку, тека буде заархівована перед завантаженням до S3.
Для деяких ресурсів, які вимагають архіву (наприклад, `AWS::Serverless::Function`), файл також буде заархівований перед завантаженням.

Шляхи до файлів вважаються відносними до батьківської теки `copilot/` у вашому репозиторії. Для наведеного вище прикладу структура тек виглядатиме так:

```bash
.
├── copilot
│   └── example-service
│       ├── addons
│       │   └── lambda.yml
│       └── manifest.yml
└── lambdas
    └── example
        └── index.js
```

Абсолютні шляхи також підтримуються, хоча вони можуть не працювати так добре на різних машинах.

## Приклад: Обробка потоків DynamoDB Lambda

Цей приклад покаже створення таблиці [Amazon Dynamo DB](https://aws.amazon.com/dynamodb/) з підключеною функцією lambda для обробки подій з [потоку таблиці](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html). Ця архітектура може бути корисною, якщо у вас є сервіс, який потребує мінімізації затримки при зберіганні даних, але може запустити окремий процес, який займає більше часу для обробки даних.

#### Передумови

- [Розгорнутий сервіс copilot](../../../concepts/services/)

#### Кроки

1. Згенеруйте надбудову таблиці DynamoDB для вашого сервісу, виконавши `copilot storage init` (Більше інформації [тут!](../../storage/))
2. Додайте властивість [`StreamSpecification`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-dynamodb-table.html#cfn-dynamodb-table-streamspecification) до згенерованого ресурсу [`AWS::DynamoDB::Table`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-dynamodb-table.html):
  ```yaml title="copilot/service-name/addons/ddb.yml"
  StreamSpecification:
    StreamViewType: NEW_AND_OLD_IMAGES
  ```
3. Додайте функцію Lambda, роль IAM та ресурс для показу потоків подій Lambda, переконавшись, що ви надали доступ до потоку таблиці DynamoDB у ролі IAM:
  ```yaml title="copilot/service-name/addons/ddb.yml" hl_lines="4 37 43"
    recordProcessor:
      Type: AWS::Lambda::Function
      Properties:
        Code: lambdas/record-processor/ # локальний шлях до lambda для обробки записів
        Handler: "index.handler"
        Timeout: 60
        MemorySize: 512
        Role: !GetAtt "recordProcessorRole.Arn"
        Runtime: nodejs20.x

    recordProcessorRole:
      Type: AWS::IAM::Role
      Properties:
        AssumeRolePolicyDocument:
          Version: 2012-10-17
          Statement:
            - Effect: Allow
              Principal:
                Service:
                  - lambda.amazonaws.com
              Action:
                - sts:AssumeRole
        Path: /
        ManagedPolicyArns:
          - !Sub arn:${AWS::Partition}:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
        Policies:
          - PolicyDocument:
              Version: 2012-10-17
              Statement:
                - Effect: Allow
                  Action:
                    - dynamodb:DescribeStream
                    - dynamodb:GetRecords
                    - dynamodb:GetShardIterator
                    - dynamodb:ListStreams
                  # замініть <table> на імʼя згенерованої таблиці
                  Resource: !Sub ${<table>.Arn}/stream/*

    tableStreamMappingToRecordProcessor:
      Type: AWS::Lambda::EventSourceMapping
      Properties:
        FunctionName: !Ref recordProcessor
        EventSourceArn: !GetAtt <table>.StreamArn # замініть <table> тут також
        BatchSize: 1
        StartingPosition: LATEST
  ```
4. Створіть вашу функцію lambda:
  ```js title="lambdas/record-processor/index.js"
  "use strict";
  const { unmarshall } = require('@aws-sdk/util-dynamodb');

  exports.handler = async function (event, context) {
    for (const record of event?.Records) {
      if (record?.eventName != "INSERT") {
        continue;
      }

      // обробка нових записів
      const item = unmarshall(record?.dynamodb?.NewImage);
      console.log("обробка елемента", item);
    }
  };
  ```
5. Виконайте `copilot svc deploy`, щоб розгорнути вашу функцію lambda!🎉 Як тільки ваш сервіс додасть записи до таблиці, функція lambda буде викликана і зможе обробляти нові записи.
