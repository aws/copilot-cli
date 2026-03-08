---
title: Додаткові ресурси середовища
---

# Mоделювання додаткових ресурсів середовища за допомогою AWS CloudFormation

Додаткові ресурси AWS, які називаються "addons" в CLI, — це будь-які додаткові сервіси AWS, які стандартно не інтегровані в [маніфест сервісу чи середовища](../../../manifest/overview/).
Наприклад, addon може бути таблицею DynamoDB, кошиком S3 або кластером RDS Aurora Serverless, до якого ваш сервіс повинен мати доступ для читання чи запису.

Ви можете визначити додаткові ресурси для робочого навантаження (такого як [Load Balanced Web Service](../../../manifest/lb-web-service/) або [Scheduled Job](../../../manifest/scheduled-job/)). Життєвим циклом надбудов робочого навантаження буде керувати саме робоче навантаження, і вони будуть видалені після видалення робочого навантаження.

Крім того, ви можете визначити додаткові спільні ресурси для середовища. Надбудови середовища не будуть видалені, доки не буде видалено саме середовище.

Ця сторінка описує, як створювати надбудови на рівні середовища. Для надбудов на рівні робочого навантаження відвідайте [Моделювання додаткових ресурсів робочого навантаження з AWS CloudFormation](../workload/).

## Як додати кошик S3, таблицю DDB або кластер Aurora Serverless?

Copilot надає наступні команди, щоб допомогти вам створити певні види надбудов:

* [`storage init`](../../../commands/storage-init/) створить таблицю DynamoDB, кошик S3 або кластер Aurora Serverless.

Ви можете запустити `copilot storage init` з вашого робочого простору, і вам будуть поставлені питання, які допоможуть налаштувати ці ресурси.

## Як додати інші ресурси?

Для інших типів надбудов ви можете додати власні шаблони CloudFormation:

1. Ви можете зберігати власні шаблони у вашому робочому просторі в теці `copilot/environments/addons`.
2. Під час запуску `copilot env deploy`, власні шаблони addons будуть розгорнуті разом з вашим стеком середовища.

???- note "Приклад структури робочого простору з надбудовами середовища"
    ```term
    .
    └── copilot
        └── environments
            ├── addons  # Зберігайте надбудови середовища.
            │   └── mys3.yaml
            ├── dev
            └── prod
    ```

## Як виглядає шаблон надбудови?

Шаблон надбудови середовища може бути [будь-яким дійсним шаблоном CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-anatomy.html), який задовольняє наступні вимоги:

* Включає принаймні один [`Resource`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html).
* Розділ `Parameters` включає `App`, `Env`.

ви можете налаштувати властивості ваших ресурсів за допомогою [Conditions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/conditions-section-structure.html) або [Mappings](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/mappings-section-structure.html).

!!! info ""
    Ми рекомендуємо дотримуватися [найкращих практик Amazon IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html) під час визначення політик AWS Managed для додаткових ресурсів, включаючи:

    * [Надавайте мінімальні привілеї](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#grant-least-privilege) для політик, визначених у вашій теці `addons/`.
    * [Використовуйте умови політики для додаткової безпеки](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#use-policy-conditions) для обмеження доступу ваших політик лише до ресурсів, визначених у вашій теці `addons/`.


### Створення розділу `Parameters`

Є кілька параметрів, які Copilot вимагає визначити у ваших шаблонах.

!!! info ""
    ```yaml
    Parameters:
        App:
            Type: String
        Env:
            Type: String
    ```


#### Налаштування розділу `Parameters`

Якщо ви хочете визначити параметри, крім тих, які вимагає Copilot, ви можете зробити це за допомогою файлу
`addons.parameters.yml`.

```term
.
└── addons/
    ├── template.yml
    └── addons.parameters.yml # Додайте цей файл у вашу теку addons/.
```

1. У вашому файлі шаблону додайте додаткові параметри в розділ `Parameters`.
2. У вашому `addons.parameters.yml` визначте значення цих додаткових параметрів. Вони можуть посилатися на значення з вашого стеку середовища.

???- note "Приклади: Налаштування параметрів надбудови"
    ```yaml
    # У "environments/addons/my-addon.yml"
    Parameters:
      # Параметри, які вимагає AWS Copilot.
      App:
        Type: String
      Env:
        Type: String
      # Додаткові параметри, визначені в addons.parameters.yml
      ClusterName:
        Type: String
    ```
    ```yaml
    # В "environments/addons/addons.parameters.yml"
    Parameters:
        ClusterName: !Ref Cluster
    ```

### Створення розділів `Conditions` та `Mappings`

Як правило ви захочете налаштувати ресурси вашої надбудови залежно від певних умов. Наприклад, ви можете умовно налаштувати ємність вашого ресурсу DB залежно від того, чи розгортається він в промисловому, чи тестовому середовищі. Для цього ви можете використовувати розділи `Conditions` та `Mappings`.

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

### Створення розділу [`Outputs`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)

Ви можете використовувати розділ `Outputs` для визначення будь-яких значень, які можуть бути використані іншими ресурсами; наприклад, сервісом, стеком CloudFormation тощо.

#### Надбудова середовища: Підключення до ваших робочих навантажень

Значення з надбудови середовища може бути використане надбудовою робочого навантаження або маніфестом робочого навантаження.
Для цього спочатку потрібно експортувати значення з надбудови середовища за допомогою розділу `Outputs`.

???+ note "Приклад: Експорт значень з надбудов середовища"
    ```yaml
    Outputs:
        MyTableARN:
            Value: !GetAtt ServiceTable.Arn
            Export:
                Name: !Sub ${App}-${Env}-MyTableARN  # Це значення може бути використане маніфестом робочого навантаження або addon-ом робочого навантаження.
        MyTableName:
            Value: !Ref ServiceTable
            Export:
                Name: !Sub ${App}-${Env}-MyTableName
    ```

Важливо додати блок `Export`. Інакше ваш стек робочого навантаження або ваші надбудови робочого навантаження не зможуть отримати доступ до значення. Ви будете використовувати `Export.Name` для посилання на значення з ваших ресурсів на рівні робочого навантаження.

???- hint "Розгляньте: Просторове іменування вашого `Export.Name`"
    Ви можете вказати будь-яке імʼя для `Export.Name`.
    Тобто, воно не обовʼязково повинно бути з префіксом `!Sub ${App}-${Env}`; воно може бути просто `MyTableName`.

    Однак, в межах одного регіону AWS, `Export.Name` повинен бути унікальним. Тобто, ви не можете мати два експорти з імʼям `MyTableName` в `us-east-1`.

    Тому ми рекомендуємо вам використовувати просторове іменування ваших експортів з `${App}` та `${Env}`, щоб зменшити ймовірність зіткнення імен. Крім того, це робить зрозумілим, яким застосунком та середовищем керується значення.

    З просторовим іменуванням, наприклад, якщо імʼя вашого застосунку `"my-app"`, і ви розгорнули надбудови в середовищі `test`, то кінцеве імʼя експорту буде `my-app-test-MyTableName`.


##### Посилання з надбудови робочого навантаження

У ваших надбудовах робочого навантаження ви можете посилатися на значення з ваших надбудов середовища, якщо це значення експортоване. Для цього використовуйте функцію [`Fn::ImportValue`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/intrinsic-function-reference-importvalue.html) з імʼям експорту цього значення, щоб імпортувати його з надбудови середовища.

???- note "Приклад: Політика IAM для доступу до таблиці DynamoDB на рівні середовища"
    ```yaml
    Parameters:
      App:
        Type: String
        Description: Імʼя вашого застосунку.
      Env:
        Type: String
        Description: Імʼя середовища, до якого розгортається ваш сервіс, завдання або робочий процес.
      Name:
        Type: String
        Description: Імʼя вашого робочого навантаження.
    Resources:
      MyTableAccessPolicy:
        Type: AWS::IAM::ManagedPolicy
        Properties:
          Description: Надає доступ CRUD до таблиці Dynamo DB
          PolicyDocument:
            Version: '2012-10-17'
            Statement:
              - Sid: DDBActions
                Effect: Allow
                Action:
                  - dynamodb:* # ПРИМІТКА: Зменшіть дозволи у вашому реальному застосунку. Це зроблено, щоб цей приклад не був занадто довгим!
                Resource:
                  Fn::ImportValue:                # <- Ми імпортуємо ARN таблиці з надбудов середовища.
                    !Sub ${App}-${Env}-MyTableARN # <- Імʼя експорту, яке ми використовували.
    ```

##### Посилання з маніфесту робочого навантаження

Ви також можете посилатися на значення з ваших надбудов середовища у маніфесті робочого навантаження для
[`variables`](../../../manifest/lb-web-service/#variables-from-cfn), [`secrets`](../../../manifest/lb-web-service/#secrets-from-cfn) та [`security_groups`](../../../manifest/lb-web-service/#network-vpc-security-groups-from-cfn), якщо це значення експортоване. Для цього використовуйте поля `from_cfn` у маніфестах робочого навантаження з імʼям експорту цього значення.

???- note "Приклади: використання `from_cfn`"
    === "Вставка змінної середовища"
        ```yaml
        name: db-front
        type: Backend Service
        variables:
          MY_TABLE_NAME:
            from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyTableName
        ```

    === "Вставка секрету"
        ```yaml
        name: db-front
        type: Backend Service

        secrets:
          MY_CLUSTER_CREDS:
            from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyClusterSecret
        ```

    === "Приєднання групи безпеки"
        ```yaml
        name: db-front
        type: Backend Service

        security_groups:
            - from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyClusterAllowedSecurityGroup
        ```

## Приклади

### Покрокова інструкція з надбудовами середовища

Дивіться наш [блог пост v1.25.0](../../../../blogs/release-v125/#environment-addons) для детальної покрокової інструкції!
