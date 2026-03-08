---
title: Додаткові ресурси робочого навантаження
---

# Моделювання додаткових ресурсів робочого навантаження за допомогою AWS CloudFormation

Додаткові ресурси AWS, які називаються "addons" в CLI, — це будь-які додаткові сервіси AWS, які [маніфест сервісу або середовища](../../../manifest/overview/) стандартно не інтегрує. Наприклад, надбудовою може бути таблиця DynamoDB, S3 кошик, або кластер RDS Aurora Serverless, до якого ваш сервіс повинен мати доступ для читання або запису.

Ви можете визначити додаткові ресурси для робочого навантаження (такого як [Load Balanced Web Service](../../../manifest/lb-web-service/) або [Scheduled Job](../../../manifest/scheduled-job/)). Життєвий цикл надбудов робочого навантаження буде керуватися робочим навантаженням і буде видалений після видалення робочого навантаження.

Альтернативно, ви можете визначити додаткові спільні ресурси для середовища. Надбудови середовища не будуть видалені, доки не буде видалено саме середовище.

Ця сторінка документує, як створювати надбудови на рівні робочого навантаження. Для надбудов на рівні середовища відвідайте [Моделювання додаткових ресурсів середовища з AWS CloudFormation](../environment/).

## Як додати S3 кошик, таблицю DDB або кластер Aurora Serverless?

Copilot надає наступні команди для допомоги у створенні певних типів надбудов:

* [`storage init`](../../../commands/storage-init/) створить таблицю DynamoDB, S3 кошик або кластер Aurora Serverless.

Ви можете запустити `copilot storage init` з вашого робочого простору, і вам будуть поставлені питання, які допоможуть налаштувати ці ресурси.

## Як додати інші ресурси?

Для інших типів надбудов ви можете додати власні шаблони CloudFormation:

1. Ви можете зберігати власні шаблони у вашому робочому просторі в теці `copilot/<workload>/addons`.
2. При запуску `copilot [svc/job] deploy`, власний шаблон надбудов буде розгорнуто разом зі стеком вашого робочого навантаження.

???- note "Приклад структури робочого простору з надбудовами робочого навантаження"
    ```term
    .
    └── copilot
        └── webhook
            ├── addons # Зберігає надбудови, повʼязані з сервісом "webhook".
            │   └── mytable-ddb.yaml
            └── manifest.yaml
    ```

## Як виглядає шаблон надбудови?

Шаблон надбудови робочого навантаження може бути [будь-яким вірним шаблоном CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-anatomy.html), який відповідає наступним вимогам:

* Включає принаймні один [`Resource`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html).
* Секція [`Parameters`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/parameters-section-structure.html) повинна включати `App`, `Env`, `Name`.

Ви можете налаштувати властивості ваших ресурсів за допомогою [Conditions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/conditions-section-structure.html) або [Mappings](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/mappings-section-structure.html).

!!! info ""
    Ми рекомендуємо дотримуватись [найкращих практик Amazon IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html) при визначенні AWS Managed Policies для додаткових ресурсів, включаючи:

    * [Надавання мінімальних привілеїв](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#grant-least-privilege) для політик, визначених у вашій теці `addons/`.
    * [Використання умов політик для додаткової безпеки](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#use-policy-conditions) щоб обмежити ваші політики доступом тільки до ресурсів, визначених у вашій теці `addons/`.

### Створення секції `Parameters`

Є кілька параметрів, які Copilot вимагає визначити у ваших шаблонах.

!!! info ""
    ```yaml
    Parameters:
        App:
            Type: String
        Env:
            Type: String
        Name:
            Type: String
    ```

#### Налаштування секції `Parameters`

Якщо ви хочете визначити параметри, додаткові до тих, які вимагає Copilot, ви можете зробити це за допомогою файлу `addons.parameters.yml`.

```term
.
└── addons/
    ├── template.yml
    └── addons.parameters.yml # Додайте цей файл у вашу теку addons/.
```

1. У вашому файлі шаблону додайте додаткові параметри у секцію `Parameters`.
2. У вашому `addons.parameters.yml` визначте значення цих додаткових параметрів. Вони можуть посилатися на значення з вашого стека робочого навантаження.

???- note "Приклади: Налаштування параметрів надбудови"
    ```yaml
    # У "webhook/addons/my-addon.yml"
    Parameters:
      # Параметри, які вимагає AWS Copilot.
      App:
        Type: String
      Env:
        Type: String
      Name:
        Type: String
      # Додаткові параметри, визначені у addons.parameters.yml
      ServiceName:
        Type: String
    ```
    ```yaml
    # У "webhook/addons/addons.parameters.yml"
    Parameters:
        ServiceName: !GetAtt Service.Name
    ```

### Створення секцій `Conditions` та `Mappings`

Зазвичай, ви захочете налаштувати ресурси вашої надбудови по-різному залежно від певних умов. Наприклад, ви можете умовно налаштувати ємність вашого ресурсу БД залежно від того, чи розгортається він у промисловому або тестовому середовищі. Для цього ви можете використовувати секції `Conditions` та `Mappings`.

???- note "Приклади: Умовне налаштування надбудов"
    === "Використання `Mappings`"
        ```yaml
        Mappings:
            MyAuroraServerlessEnvScalingConfigurationMap:
                dev:
                    "DBMinCapacity": 0.5
                    "DBMaxCapacity": 8
                test:
                    "DBMinCapacity": 1
                    "DBMaxCapacity": 32
                prod:
                    "DBMinCapacity": 1
                    "DBMaxCapacity": 64
        Resources:
            MyCluster:
                Type: AWS::RDS::DBCluster
                Properties:
                    ScalingConfiguration:
                        MinCapacity: !FindInMap
                            - MyAuroraServerlessEnvScalingConfigurationMap
                            - !Ref Env
                            - DBMinCapacity
                        MaxCapacity: !FindInMap
                            - MyAuroraServerlessEnvScalingConfigurationMap
                            - !Ref Env
                            - DBMaxCapacity
        ```

    === "Використання `Conditions`"
        ```yaml
        Conditions:
          IsProd: !Equals [!Ref Env, "prod"]

        Resources:
          MyCluster:
            Type: AWS::RDS::DBCluster
            Properties:
              ScalingConfiguration:
                  MinCapacity: !If [IsProd, 1, 0.5]
                  MaxCapacity: !If [IsProd, 8, 64]
        ```

### Створення секції [`Outputs`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)

Ви можете використовувати секцію `Outputs` для визначення будь-яких значень, які можуть бути використані іншими ресурсами; наприклад, сервісом, стеком CloudFormation тощо.

#### Додатки робочого навантаження: Підключення до ваших робочих навантажень

Ось кілька можливих способів доступу до [`Resources`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html) надбудов з вашого завдання ECS або екземпляра App Runner:

* Якщо вам потрібно додати додаткові політики до ролі завдання ECS або ролі екземпляра App Runner, ви можете визначити ресурс [IAM ManagedPolicy](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-iam-managedpolicy.html) надбудови у вашому шаблоні, який містить додаткові дозволи, а потім [вивести](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html) його. Дозвіл буде додано до вашої ролі завдання або екземпляра.
* Якщо вам потрібно додати групу безпеки до вашого сервісу ECS, ви можете визначити [Групу Безпеки](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group.html) у вашому шаблоні, а потім додати її як [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html). Група безпеки буде автоматично приєднана до вашого сервісу ECS.
* Якщо ви хочете додати секрет до вашого завдання ECS, ви можете визначити [Секрет](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-secretsmanager-secret.html) у вашому шаблоні, а потім додати його як [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html). Секрет буде додано до вашого контейнера і може бути доступний як змінна середовища у верхньому регістрі SNAKE_CASE.
* Якщо ви хочете додати будь-яке значення ресурсу як змінну середовища, ви можете створити [Output](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html) для ваших завдань ECS. Воно буде додано до вашого контейнера і може бути доступне як змінна середовища у верхньому регістрі SNAKE_CASE.

## Приклади

### Шаблон надбудови робочого навантаження для таблиці DynamoDB

Ось приклад шаблону для надбудови таблиці DynamoDB на рівні робочого навантаження:

```yaml
# Ви можете використовувати будь-який з цих параметрів для створення умов або map у вашому шаблоні.
Parameters:
  App:
    Type: String
    Description: Тут назва вашої надбудови.
  Env:
    Type: String
    Description: Назва середовища, до якого розгортається ваш сервіс, завдання або робочий процес.
  Name:
    Type: String
    Description: Назва вашого робочого навантаження.

Resources:
  # Створіть ваш ресурс тут, наприклад, AWS::DynamoDB::Table:
  # MyTable:
  #   Type: AWS::DynamoDB::Table
  #   Properties:
  #     ...

  # 1. На додаток до вашого ресурсу, якщо вам потрібно отримати доступ до ресурсу з вашого завдання ECS
  # тоді вам потрібно створити AWS::IAM::ManagedPolicy, який містить дозволи для вашого ресурсу.
  #
  # Наприклад, нижче наведено приклад політики для MyTable:
  MyTableAccessPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
          - Sid: DDBActions
            Effect: Allow
            Action:
              - dynamodb:BatchGet*
              - dynamodb:DescribeStream
              - dynamodb:DescribeTable
              - dynamodb:Get*
              - dynamodb:Query
              - dynamodb:Scan
              - dynamodb:BatchWrite*
              - dynamodb:Create*
              - dynamodb:Delete*
              - dynamodb:Update*
              - dynamodb:PutItem
            Resource: !Sub ${ MyTable.Arn}

Outputs:
  # 1. Вам потрібно вивести IAM ManagedPolicy, щоб Copilot міг додати його як керовану політику до вашої ролі завдання ECS.
  MyTableAccessPolicyArn:
    Description: "ARN ManagedPolicy для приєднання до ролі завдання."
    Value: !Ref MyTableAccessPolicy

  # 2. Якщо ви хочете додати властивість вашого ресурсу як змінну середовища до вашого завдання ECS,
  # тоді вам потрібно визначити вивід для цього.
  #
  # Наприклад, вивід MyTableName буде додано у верхньому регістрі зміїного регістру, MY_TABLE_NAME, до вашого завдання.
  MyTableName:
    Description: "Назва цієї таблиці DynamoDB."
    Value: !Ref MyTable
```
