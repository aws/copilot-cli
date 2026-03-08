---
title: Зберігання
---

# Зберігання

Є два способи додати зберігання до робочих навантажень Copilot: використовуючи [`copilot storage init`](#database-and-artifacts) для створення баз даних та S3 кошиків; та підключаючи наявну файлову систему EFS за допомоги поля [`storage`](#file-systems) у маніфесті.

## База даних та артефакти <a id="database-and-artifacts"></a>

Щоб додати базу даних або S3 кошик до вашого сервісу, завдання або середовища, просто виконайте команду [`copilot storage init`](../../commands/storage-init/).

```console
# Для покрокових інструкцій.
$ copilot storage init -t S3

# Щоб створити кошик з назвою "my-bucket", який доступний для сервісу "api" і розгортається та видаляється разом з "api".
$ copilot storage init -n my-bucket -t S3 -w api -l workload
```

Вищенаведена команда створить шаблон Cloudformation для S3 кошика в теці [addons](../addons/workload/) для сервісу "api".
Наступного разу, коли ви запустите `copilot deploy -n api`, кошик буде створено, дозвіл на доступ до нього буде додано до ролі завдання `api`, а назва кошика буде вставлена в контейнер `api` в змінну середовища `MY_BUCKET_NAME`.

!!!info "Іфнормація"
    Усі назви перетворюються на SCREAMING_SNAKE_CASE на основі використання дефісів або підкреслень. Ви можете переглянути змінні середовища для даного сервісу, запустивши `copilot svc show`.

Ви також можете створити [таблицю DynamoDB](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Introduction.html) за допомогою `copilot storage init`. Наприклад, щоб створити шаблон Cloudformation для таблиці з ключем сортування та локальним вторинним індексом, ви можете виконати наступну команду:

```console
# Для покрокових інструкцій.
$ copilot storage init -t DynamoDB

# Або пропустіть підказки, надавши прапорці.
$ copilot storage init -n users -t DynamoDB -w api -l workload --partition-key id:N --sort-key email:S --lsi post-count:N
```

Це створить таблицю DynamoDB з назвою `${app}-${env}-${svc}-users`. Її ключем розділу буде `id`, атрибут `Number`; її ключем сортування буде `email`, атрибут `String`; і вона матиме [локальний вторинний індекс](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/LSI.html) (по суті, альтернативний ключ сортування) на атрибуті `Number` `post-count`.

Також можна створити кластер [RDS Aurora Serverless v2](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-serverless-v2.html) за допомогою `copilot storage init`.
```console
# Для покрокових інструкцій.
$ copilot storage init -t Aurora

# Або пропустіть підказки, надавши прапорці.
$ copilot storage init -n my-cluster -t Aurora -w api -l workload --engine PostgreSQL --initial-db my_db
```

Це створить кластер RDS Aurora Serverless v2, який використовує PostgreSQL з базою даних з назвою `my_db`. Змінна середовища з назвою `MYCLUSTER_SECRET` буде вставлена у ваш робочий процес як JSON-рядок. Поля включають `'host'`, `'port'`, `'dbname'`, `'username'`, `'password'`, `'dbClusterIdentifier'` та `'engine'`.

### Зберігання середовища

Прапорець `-l` є скороченням для `--lifecycle`. У наведених вище прикладах значення прапорця `-l` - `workload`. Це означає, що ресурси зберігання будуть створені як додаток до сервісу або завдання. Зберігання буде розгорнуто коли ви запустите `copilot [svc/job] deploy`, і буде видалено, коли ви запустите `copilot [svc/job] delete`.

Альтернативно, якщо ви хочете, щоб ваше зберігання зберігалося навіть після видалення сервісу або завдання, ви можете створити ресурс зберігання середовища. Ресурс зберігання середовища створюється як надбудова середовища: він розгортається, коли ви запускаєте `copilot env deploy`, і не видаляється, поки ви не запустите `copilot env delete`.

## Файлові системи <a id="file-systems"></a>

Є два способи використання файлової системи EFS з Copilot: використання керованого EFS та імпорт власної файлової системи.

!!! Attention "Увага"
    EFS не підтримується для сервісів на базі Windows.

### Керований EFS <a id="managed-efs"></a>

Найпростіший спосіб почати використовувати EFS для зберігання на рівні сервісу або завдання — це вбудована можливість керованого EFS від Copilot. Щоб почати, просто увімкніть ключ `efs` у маніфесті під назвою вашого тому.

```yaml
name: frontend

storage:
  volumes:
    myManagedEFSVolume:
      efs: true
      path: /var/efs
      read_only: false
```

Цей маніфест призведе до створення тому EFS на рівні середовища з точкою доступу та виділеною текою за шляхом `/frontend` у файловій системі EFS, створеній спеціально для вашого сервісу. Ваш контейнер зможе отримати доступ до цієї теки та всіх її вкладених теках за шляхом `/var/efs` у власній файловій системі. Тека `/frontend` та файлова система EFS будуть зберігатися до тих пір, поки ви не видалите своє середовище. Використання точки доступу для кожного сервісу гарантує, що жоден з сервісів не зможе отримати доступ до даних іншого.

Ви також можете налаштувати UID та GID, які використовуються для точки доступу, вказавши поля `uid` та `gid` у розширеній конфігурації EFS. Якщо ви не вкажете UID або GID, Copilot вибере псевдовипадковий UID та GID для точки доступу на основі [контрольної суми CRC32](https://stackoverflow.com/a/14210379/5890422) імені сервісу.

```yaml
storage:
  volumes:
    myManagedEFSVolume:
      efs:
        uid: 1000
        gid: 10000
      path: /var/efs
      read_only: false
```

`uid` та `gid` не можуть бути вказані з будь-якою іншою розширеною конфігурацією EFS.

#### Під капотом

Коли ви увімкнете керований EFS, Copilot створює наступні ресурси на рівні середовища:

* [Файлова система EFS](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-efs-filesystem.html).
* [Точки монтування](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-efs-mounttarget.html) у кожній з приватних підмереж вашого середовища
* Правила групи безпеки, що дозволяють групі безпеки середовища отримати доступ до точок монтування.

На рівні сервісу Copilot створює:

* [Точку доступу EFS](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-efs-accesspoint.html). Точка доступу посилається на теку, створену CFN, названий на честь сервісу або завдання, з яким ви хочете використовувати EFS.

Ви можете побачити ресурси на рівні середовища, створені за допомогою виклику `copilot env show --json --resources` та аналізу вихідних даних за допомогою вашого улюбленого процесора JSON. Наприклад:

```console
> copilot env show -n test --json --resources | jq '.resources[] | select( .type | contains("EFS") )'
```

#### Розширені випадки використання

##### Наповнення керованого тому EFS

Іноді ви можете захотіти заповнити створений том EFS даними перед тим, як ваш сервіс почне приймати трафік. Існує кілька способів зробити це, залежно від вимог вашого основного контейнера та того, чи потребує він цих даних для запуску.

###### Використання sidecar контейнера

Ви можете змонтувати створений том EFS у sidecar контейнері, використовуючи поле [`mount_points`](../../developing/sidecars/), та використовувати директиви `COMMAND` або `ENTRYPOINT` вашого sidecar контейнера для копіювання даних з файлової системи sidecar контейнера або завантаження даних з S3 або іншого хмарного сервісу.

Якщо ви позначите sidecar контейнер як неважливий за допомогою `essential:false`, він запуститься, виконає свою роботу та завершить роботу, коли контейнери сервісу піднімуться та стабілізуються.

Це може не підходити для робочих навантажень, які залежать від наявності правильних даних у томі EFS.

###### Використання `copilot svc exec`

Для робочих навантажень, де дані повинні бути присутніми до підняття контейнерів завдань, ми рекомендуємо спочатку використовувати контейнер-заповнювач.

Наприклад, розгорніть свій сервіс `frontend` з наступними значеннями у маніфесті:

```yaml
name: frontend
type: Load Balanced Web Service

image:
  location: amazon/amazon-ecs-sample
exec: true

storage:
  volumes:
    myVolume:
      efs: true
      path: /var/efs
      read_only: false
```

Потім, коли ваш сервіс стабільний, запустіть:

```console
copilot svc exec
```

Це відкриє інтерактивну оболонку, з якої ви зможете додавати пакети, такі як `curl` або `wget`, завантажувати дані з інтернету, створювати структуру каталогів тощо.

!!!info "Інформація"
    Цей метод налаштування контейнерів не рекомендується для виробничих середовищ; контейнери є ефемерними, і якщо ви хочете, щоб певне програмне забезпечення було присутнє у контейнерах вашого сервісу, обовʼязково додайте його за допомогою директиви `RUN` у Dockerfile.

Коли ви заповните теку, змініть свій маніфест, щоб видалити директиву `exec` та оновити поле `build` до бажаної конфігурації збірки Docker або розташування образу.

```yaml
name: frontend
type: Load Balanced Web Service

image:
  build: ./Dockerfile
storage:
  volumes:
    myVolume:
      efs: true
      path: /var/efs
      read_only: false
```

### Зовнішній EFS

Монтування зовнішнього тому EFS у завданнях Copilot вимагає двох речей:

1. Щоб ви створили [файлову систему EFS](https://docs.aws.amazon.com/efs/latest/ug/whatisefs.html) у регіоні бажаного середовища.
2. Щоб ви створили [точку монтування EFS](https://docs.aws.amazon.com/efs/latest/ug/accessing-fs.html) за допомогою групи безпеки середовища Copilot у кожній підмережі вашого середовища.

Коли ці передумови виконані, ви можете увімкнути зберігання EFS, використовуючи простий синтаксис у вашому маніфесті. Вам знадобиться ідентифікатор файлової системи та, якщо використовується, конфігурація точки доступу для файлової системи.

!!!info "Інформація"
    Ви можете використовувати дану файлову систему EFS лише в одному середовищі одночасно. Точки монтування обмежені однією на зону доступності; тому ви повинні видалити будь-які наявні точки монтування перед тим, як перенести файлову систему до Copilot, якщо ви використовували її в іншій VPC.

### Синтаксис маніфесту

Найпростіший можливий том EFS можна вказати за допомогою наступного синтаксису:

```yaml
storage:
  volumes:
    myEFSVolume: # Це змінний ключ і може бути встановлений на довільний рядок.
      path: '/etc/mount1'
      efs:
        id: fs-1234567
```

Це створить том, змонтований лише для читання, у контейнері вашого сервісу або завдання, використовуючи файлову систему `fs-1234567`. Якщо точки монтування не створені у підмережах середовища, завдання не зможе запуститися.

Повний синтаксис для зовнішніх томів EFS наведено нижче.

```yaml
storage:
  volumes:
    <volume name>:
      path: <mount path>             # Обовʼязково. Шлях всередині контейнера.
      read_only: <boolean>           # Стандартно: true
      efs:
        id: <filesystem ID>          # Обовʼязково.
        root_dir: <filesystem root>  # Необовʼязково. Стандартно "/". Не можна вказувати
                                     # якщо використовується точка доступу.
        auth:
          iam: <boolean>             # Необовʼязково. Чи використовувати авторизацію IAM при
                                     # монтуванні цієї файлової системи.
          access_point_id: <access point ID> # Необовʼязково. Ідентифікатор точки доступу EFS
                                             # для використання при монтуванні цієї файлової системи.
        uid: <uint32>                # Необовʼязково. UID для керованої точки доступу EFS.
        gid: <uint32>                # Необовʼязково. GID для керованої точки доступу EFS. Не можна вказувати
                                     # з `id`, `root_dir` або `auth`.
```

### Створення точок монтування

Існує кілька способів створення точок монтування для наявної файлової системи EFS: [використовуючи AWS CLI](#with-the-aws-cli) та [використовуючи CloudFormation](#cloudformation).

#### За допомогою AWS CLI <a id="with-the-aws-cli"></a>

Щоб створити точки монтування для наявної файлової системи, вам знадобляться

1. ідентифікатор цієї файлової системи.
2. середовище Copilot, розгорнуте в тому ж обліковому записі та регіоні.

Щоб отримати ідентифікатор файлової системи, ви можете використовувати AWS CLI:

```console
$ EFS_FILESYSTEMS=$(aws efs describe-file-systems | \
  jq '.FileSystems[] | {ID: .FileSystemId, CreationTime: .CreationTime, Size: .SizeInBytes.Value}')
```

Якщо ви виведете (`echo`) цю змінну, ви повинні побачити, яку файлову систему вам потрібно. Призначте її змінній `$EFS_ID` та продовжуйте.

Вам також знадобляться публічні підмережі середовища Copilot та група безпеки середовища. Ця команда jq відфільтрує вихідні дані виклику describe-stacks до просто бажаного значення вихідних даних.

!!!info "Інформація"
    Файлова система, яку ви використовуєте, МАЄ бути в тому ж регіоні, що й ваше середовище Copilot!

```console
$ SUBNETS=$(aws cloudformation describe-stacks --stack-name ${YOUR_APP}-${YOUR_ENV} \
  | jq '.Stacks[] | .Outputs[] | select(.OutputKey == "PublicSubnets") | .OutputValue')
$ SUBNET1=$(echo $SUBNETS | jq -r 'split(",") | .[0]')
$ SUBNET2=$(echo $SUBNETS | jq -r 'split(",") | .[1]')
$ ENV_SG=$(aws cloudformation describe-stacks --stack-name ${YOUR_APP}-${YOUR_ENV} \
  | jq -r '.Stacks[] | .Outputs[] | select(.OutputKey == "EnvironmentSecurityGroup") | .OutputValue')
```

Коли у вас є ці дані, створіть точки монтування.

```console
$ MOUNT_TARGET_1_ID=$(aws efs create-mount-target \
    --subnet-id $SUBNET_1 \
    --security-groups $ENV_SG \
    --file-system-id $EFS_ID | jq -r .MountTargetID)
$ MOUNT_TARGET_2_ID=$(aws efs create-mount-target \
    --subnet-id $SUBNET_2 \
    --security-groups $ENV_SG \
    --file-system-id $EFS_ID | jq -r .MountTargetID)
```

Після цього ви можете вказати конфігурацію `storage` у маніфесті, як зазначено вище.

##### Очищення

Видаліть точки монтування за допомогою AWS CLI.

```console
$ aws efs delete-mount-target --mount-target-id $MOUNT_TARGET_1
$ aws efs delete-mount-target --mount-target-id $MOUNT_TARGET_2
```

#### CloudFormation

Ось приклад того, як ви можете створити відповідну інфраструктуру EFS для зовнішньої файлової системи, використовуючи стек CloudFormation.

Після створення середовища розгорніть наступний шаблон CloudFormation у тому ж обліковому записі та регіоні, що й середовище.

Розмістіть наступний шаблон CloudFormation у файлі з назвою `efs.yml`.

```yaml
Parameters:
  App:
    Type: String
    Description: Назва вашого засьосунку.
  Env:
    Type: String
    Description: Назва середовища, до якого розгортається ваш сервіс, завдання або робочий процес.

Resources:
  EFSFileSystem:
    Metadata:
      'aws:copilot:description': 'Файлова система EFS для постійного зберігання завдань та сервісів'
    Type: AWS::EFS::FileSystem
    Properties:
      PerformanceMode: generalPurpose
      ThroughputMode: bursting
      Encrypted: true

  MountTargetPublicSubnet1:
    Type: AWS::EFS::MountTarget
    Properties:
      FileSystemId: !Ref EFSFileSystem
      SecurityGroups:
        - Fn::ImportValue:
            !Sub "${App}-${Env}-EnvironmentSecurityGroup"
      SubnetId: !Select
        - 0
        - !Split
            - ","
            - Fn::ImportValue:
                !Sub "${App}-${Env}-PublicSubnets"

  MountTargetPublicSubnet2:
    Type: AWS::EFS::MountTarget
    Properties:
      FileSystemId: !Ref EFSFileSystem
      SecurityGroups:
        - Fn::ImportValue:
            !Sub "${App}-${Env}-EnvironmentSecurityGroup"
      SubnetId: !Select
        - 1
        - !Split
            - ","
            - Fn::ImportValue:
                !Sub "${App}-${Env}-PublicSubnets"
Outputs:
  EFSVolumeID:
    Value: !Ref EFSFileSystem
    Export:
      Name: !Sub ${App}-${Env}-FilesystemID
```

Потім запустіть:

```console
$ aws cloudformation deploy
    --stack-name efs-cfn \
    --template-file ecs.yml
    --parameter-overrides App=${YOUR_APP} Env=${YOUR_ENV}
```

Це створить файлову систему EFS та точки монтування, необхідні вашим завданням, використовуючи вихідні дані зі стека середовища Copilot.

Щоб отримати ідентифікатор файлової системи EFS, ви можете виконати виклик `describe-stacks`:

```console
$ aws cloudformation describe-stacks --stack-name efs-cfn | \
    jq -r '.Stacks[] | .Outputs[] | .OutputValue'
```

Потім у маніфесті сервісу, який ви хочете мати доступ до файлової системи EFS, додайте наступну конфігурацію.

```yaml
storage:
  volumes:
    copilotVolume: # Це змінний ключ і може бути встановлений на довільний рядок.
      path: '/etc/mount1'
      read_only: true # Встановіть на false, якщо ваш сервіс потребує доступу для запису.
      efs:
        id: <your filesystem ID>
```

Нарешті, запустіть `copilot svc deploy`, щоб переналаштувати свій сервіс для монтування файлової системи за адресою `/etc/mount1`.

##### Очищення

Щоб очистити це, видаліть конфігурацію `storage` з маніфесту та повторно розгорніть сервіс:
```console
copilot svc deploy
```

Потім видаліть стек.

```console
aws cloudformation delete-stack --stack-name efs-cfn
```
