<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
Розділ Storage дозволяє вам вказати зовнішні томи EFS для монтування ваших контейнерів та sidecar. Це дозволяє отримати доступ до постійного сховища через зони доступності в регіоні для обробки даних або робочих навантажень CMS. Для отримання додаткової інформації дивіться сторінку [storage](../../developing/storage/). Ви також можете вказати розширюване ефемерне сховище на рівні завдання.

<span class="parent-field">storage.</span><a id="ephemeral" href="#ephemeral" class="field">`ephemeral`</a> <span class="type">Int</span>  
Вкажіть, скільки ефемерного сховища завдання потрібно виділити в GiB. Стандартне значення та мінімальне значення — 20 GiB. Максимальний розмір — 200 GiB. Розміри понад 20 GiB призводять до додаткових витрат.

Щоб створити спільний файловий контекст між основним контейнером та sidecar, ви можете використовувати порожній том:

```yaml
storage:
  ephemeral: 100
  volumes:
    scratch:
      path: /var/data
      read_only: false

sidecars:
  mySidecar:
    image: public.ecr.aws/my-image:latest
    mount_points:
      - source_volume: scratch
        path: /var/data
        read_only: false
```

Цей приклад виділить 100 GiB сховища, яке буде спільно використовуватися між sidecar та контейнером завдання. Це може бути корисним для великих наборів даних або для використання sidecar для передачі даних з EFS у сховище завдань для робочих навантажень з високими вимогами до дискового вводу/виводу.

<span class="parent-field">storage.</span><a id="storage-readonlyfs" href="#storage-readonlyfs" class="field">`readonly_fs`</a> <span class="type">Boolean</span>
Вкажіть true, щоб надати вашому контейнеру доступ лише для читання до його кореневої файлової системи.

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
Вкажіть імʼя та конфігурацію будь-яких томів EFS, які ви хочете підключити. Поле `volumes` вказується у вигляді map наступного формату:

```yaml
volumes:
  <volume name>:
    path: "/etc/mountpath"
    efs:
      ...
```

<span class="parent-field">storage.volumes.</span><a id="volume" href="#volume" class="field">`<volume>`</a> <span class="type">Map</span>  
Вкажіть конфігурацію тому.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
Обовʼязково. Вкажіть місце в контейнері, де ви хочете змонтувати ваш том. Має бути не більше 242 символів і складатися лише з символів `a-zA-Z0-9.-_/`.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
Необовʼязково. Стандартно `true`. Визначає, чи є том лише для читання, чи ні. Якщо false, контейнер отримує дозволи `elasticfilesystem:ClientWrite` на файлову систему, і том стає записуваним.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Boolean or Map</span>  
Вкажіть більш детальну конфігурацію EFS. Якщо вказано як boolean або використовуючи лише підполя `uid` та `gid`, створюється керована файлова система EFS та виділена точка доступу для цього робочого навантаження.

```yaml
// Проста керована EFS
efs: true

// Керована EFS з користувацькою POSIX інформацією
efs:
  uid: 10000
  gid: 110000
```

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
Обовʼязково. Ідентифікатор файлової системи, яку ви хочете змонтувати.

<span class="parent-field">storage.volumes.`<volume>`.efs.id.</span><a id="from_cfn" href="#from_cfn" class="field">`from_cfn`</a> <span class="type">String</span> <span class="version">Додано у [v1.30.0](../../../blogs/release-v130/#deployment-actions)</span>  
Імʼя [експорту стека CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html).

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
Необов'язково. Стандартно `/`. Вкажіть місце у файловій системі EFS, яке ви хочете використовувати як корінь вашого тому. Має бути не більше 255 символів і складатися лише з символів `a-zA-Z0-9.-_/`. Якщо використовується точка доступу, `root_dir` має бути або порожнім, або `/`, і `auth.iam` має бути `true`.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="uid" href="#uid" class="field">`uid`</a> <span class="type">Uint32</span>  
Необовʼязково. Має бути вказано разом з `gid`. Конфліктує з `root_dir`, `auth` та `id`. POSIX UID для використання у виділеній точці доступу, створеній для керованої файлової системи EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="gid" href="#gid" class="field">`gid`</a> <span class="type">Uint32</span>  
Необовʼязково. Має бути вказано разом з `uid`. Конфліктує з `root_dir`, `auth` та `id`. POSIX GID для використання у виділеній точці доступу, створеній для керованої файлової системи EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
Вкажіть розширену конфігурацію авторизації для EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Boolean</span>  
Необовʼязково. Стандартно `true`. Чи використовувати авторизацію IAM для визначення, чи дозволено тому підключатися до EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
Необовʼязково. Стандартно `""`. Ідентифікатор точки доступу EFS для підключення. Якщо використовується точка доступу, `root_dir` має бути або порожнім, або `/`, і `auth.iam` має бути `true`.
