---
title: Конвейер
---

# Конвейер

Список усіх доступних властивостей для маніфесту Copilot pipeline. Щоб дізнатися більше про конвеєри, дивіться сторінку концепції [Конвеєри](../../concepts/pipelines/).

???+ note "Приклади маніфестів безперервної доставки"

    === "Реліз робочих навантажень"
        ```yaml
        # "app-pipeline" розгорне всі сервіси та завдання з user/repo
        # в середовища "test" та "prod".
        name: app-pipeline
    
        source:
          provider: GitHub
          properties:
            branch: main
            repository: https://github.com/user/repo
            # Опціонально: вкажіть назву наявного зʼєднання CodeStar Connections.
            # connection_name: a-connection
    
        build:
          image: aws/codebuild/amazonlinux2-x86_64-standard:5.0
          # additional_policy: # Додайте додаткові дозволи під час створення ваших контейнерних образів та шаблонів.
    
        stages: 
          - # Стандартно всі робочі навантаження розгортаються одночасно в межах етапу.
            name: test
            pre_deployments:
              db_migration:
                buildspec: ./buildspec.yml
            test_commands:
              - make integ-test
              - echo "ура! Тести пройдено"
          -
            name: prod
            requires_approval: true
        ```

    === "Контроль порядку розгортання"
        ```yaml
        # Альтернативно, ви можете контролювати порядок розгортання стеків на етапі.
        # Див. https://aws.github.io/copilot-cli/blogs/release-v118/#controlling-order-of-deployments-in-a-pipeline
        name: app-pipeline
    
        source:
          provider: Bitbucket
          properties:
            branch: main
            repository:  https://bitbucket.org/user/repo
    
        stages:
          - name: test
            deployments:
              orders:
              warehouse:
              frontend:
                depends_on: [orders, warehouse]
          - name: prod
            require_approval: true
            deployments:
              orders:
              warehouse:
              frontend:
                depends_on: [orders, warehouse]
        ```

    === "Реліз середовищ"
        ```yaml
        # Зміни в маніфестах середовищ також можуть бути випущені через конвеєр.
        name: env-pipeline
    
        source:
          provider: CodeCommit
          properties:
            branch: main
            repository: https://git-codecommit.us-east-2.amazonaws.com/v1/repos/MyDemoRepo
    
        stages:
          - name: test
            deployments:
              deploy-env:
                template_path: infrastructure/test.env.yml
                template_config: infrastructure/test.env.params.json
                stack_name: app-test
          - name: prod
            deployments:
              deploy-prod:
                template_path: infrastructure/prod.env.yml
                template_config: infrastructure/prod.env.params.json
                stack_name: app-prod
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Назва вашого конвеєру.

<div class="separator"></div>

<a id="version" href="#version" class="field">`version`</a> <span class="type">String</span>  
Версія схеми для шаблону. Зараз підтримується лише одна версія, `1`.

<div class="separator"></div>

<a id="source" href="#source" class="field">`source`</a> <span class="type">Map</span>  
Конфігурація для того, як ваш конвеєр запускається.

<span class="parent-field">source.</span><a id="source-provider" href="#source-provider" class="field">`provider`</a> <span class="type">String</span>  
Назва вашого провайдера. Наразі підтримуються `GitHub`, `Bitbucket` та `CodeCommit`.

<span class="parent-field">source.</span><a id="source-properties" href="#source-properties" class="field">`properties`</a> <span class="type">Map</span>  
Конфігурація, специфічна для провайдера, щодо того, як запускається конвеєра.

<span class="parent-field">source.properties.</span><a id="source-properties-ats" href="#source-properties-ats" class="field">`access_token_secret`</a> <span class="type">String</span>  
Назва секрету AWS Secrets Manager, який містить GitHub access token для запуску конвеєра, якщо ваш провайдер — GitHub і ви створили свій конвеєр за допомогою персонального access token.

!!! info "Інформація"
    Починаючи з AWS Copilot v1.4.0, access token більше не потрібен для джерел репозиторіїв GitHub. Замість цього, Copilot буде запускати конвеєр [використовуючи зʼєднання AWS CodeStar](https://docs.aws.amazon.com/codepipeline/latest/userguide/update-github-action-connections.html).

<span class="parent-field">source.properties.</span><a id="source-properties-branch" href="#source-properties-branch" class="field">`branch`</a> <span class="type">String</span>  
Назва гілки у вашому репозиторії, яка запускає конвеєр. Copilot автоматично заповнює це поле вашою поточною локальною гілкою.

<span class="parent-field">source.properties.</span><a id="source-properties-repository" href="#source-properties-repository" class="field">`repository`</a> <span class="type">String</span>  
URL вашого репозиторію.

<span class="parent-field">source.properties.</span><a id="source-properties-connection-name" href="#source-properties-connection-name" class="field">`connection_name`</a> <span class="type">String</span>  
Назва наявного зʼєднання CodeStar Connections. Якщо не вказано, Copilot створить зʼєднання для вас.

<span class="parent-field">source.properties.</span><a id="source-properties-output-artifact-format" href="#source-properties-output-artifact-format" class="field">`output_artifact_format`</a> <span class="type">String</span>  
Опціонально. Формат вихідного артефакту. Значення можуть бути або `CODEBUILD_CLONE_REF`, або `CODE_ZIP`. Якщо не вказано, типово використовується `CODE_ZIP`.

!!! info "Інформація"
    Ця властивість недоступна для конвеєрів з джерелами дій [GitHub version 1](https://docs.aws.amazon.com/codepipeline/latest/userguide/appendix-github-oauth.html), які використовують `access_token_secret`. 

<div class="separator"></div>

<a id="build" href="#build" class="field">`build`</a> <span class="type">Map</span>  
Конфігурація для проєкту CodeBuild.

<span class="parent-field">build.</span><a id="build-image" href="#build-image" class="field">`image`</a> <span class="type">String</span>  
URI, який ідентифікує Docker образ для використання в цьому проєкті збірки. Зараз стандартно використовується `aws/codebuild/amazonlinux2-x86_64-standard:5.0`.

<span class="parent-field">build.</span><a id="build-buildspec" href="#build-buildspec" class="field">`buildspec`</a> <span class="type">String</span>  
Опціонально. Шлях до файлу buildspec, відносно кореня проєкту, для використання в цьому проєкті збірки. Стандартно Copilot створить один для вас, розташований за адресою `copilot/pipelines/[your pipeline name]/buildspec.yml`.

<span class="parent-field">build.</span><a id="build-additional-policy" href="#build-additional-policy" class="field">`additional_policy.`</a><a id="policy-document" href="#policy-document" class="field">`PolicyDocument`</a> <span class="type">Map</span>  
Опціонально. Вкажіть додатковий документ політики для додавання до ролі проєкту збірки. Додатковий документ політики можна вказати у вигляді map в YAML, наприклад:

```yaml
build:
  additional_policy:
    PolicyDocument:
      Version: '2012-10-17'
      Statement:
        - Effect: Allow
          Action:
            - ecr:GetAuthorizationToken
          Resource: '*'
```

або альтернативно у вигляді JSON:

```yaml
build:
  additional_policy:
    PolicyDocument: 
      {
        "Statement": [
          {
            "Action": ["ecr:GetAuthorizationToken"],
            "Effect": "Allow",
            "Resource": "*"
          }
        ],
        "Version": "2012-10-17"
      }
```

<div class="separator"></div>

<a id="stages" href="#stages" class="field">`stages`</a> <span class="type">Array of Maps</span>  
Упорядкований список середовищ, до яких буде розгорнуто ваш конвеєр.

<span class="parent-field">stages.</span><a id="stages-name" href="#stages-name" class="field">`name`</a> <span class="type">String</span>  
Назва середовища для розгортання ваших сервісів.

<span class="parent-field">stages.</span><a id="stages-approval" href="#stages-approval" class="field">`requires_approval`</a> <span class="type">Boolean</span>  
Опціонально. Вказує, чи додати крок ручного затвердження перед розгортанням (або перед діями перед розгортанням, якщо ви їх додали). Стандартно `false`.

<span class="parent-field">stages.</span><a id="stages-predeployments" href="#stages-predeployments" class="field">`pre_deployments`</a> <span class="type">Map</span> <span class="version">Додано у [v1.30.0](../../../blogs/release-v130/#deployment-actions)</span>  
Опціонально. Додайте дії, які будуть виконані перед розгортанням.

```yaml
stages:
  - name: <env name>
    pre_deployments:
      <action name>:
        buildspec: <path to local buildspec>
        depends_on: [<other action's name>, ...]
```

<span class="parent-field">stages.pre_deployments.</span><a id="stages-predeployments-name" href="#stages-predeployments-name" class="field">`<name>`</a> <span class="type">Map</span> <span class="version">Додано у [v1.30.0](../../../blogs/release-v130/#deployment-actions)</span>  
Назва дії перед розгортанням.

<span class="parent-field">stages.pre_deployments.`<name>`.</span><a id="stages-predeployments-buildspec" href="#stages-predeployments-buildspec" class="field">`buildspec`</a> <span class="type">String</span> <span class="version">Додано у [v1.30.0](../../../blogs/release-v130/#deployment-actions)</span>  
Шлях до файлу buildspec, відносно кореня проєкту, для використання в цьому проєкті збірки.

<span class="parent-field">stages.pre_deployments.`<name>`.</span><a id="stages-predeployments-dependson" href="#stages-predeployments-dependson" class="field">`depends_on`</a> <span class="type">Array of Strings</span> <span class="version">Додано у [v1.30.0](../../../blogs/release-v130/#deployment-actions)</span>  
Опціонально. Назви інших дій перед розгортанням, які повинні бути виконані перед виконанням цієї дії. Стандартно немає залежностей.

!!! info "Інформація"
    Для отримання додаткової інформації про дії перед та після розгортання дивіться [блог пост v1.30.0](../../../blogs/release-v130/) та сторінку [Конвеєри](../../concepts/pipelines/).

<span class="parent-field">stages.</span><a id="stages-deployments" href="#stages-deployments" class="field">`deployments`</a> <span class="type">Map</span>  
Опціонально. Контролюйте, які стеки CloudFormation розгортати та їх порядок.  
Залежності `deployments` вказуються у вигляді map наступного формату:

```yaml
stages:
  - name: test
    deployments:
      <service or job name>:
      <other service or job name>:
        depends_on: [<name>, ...]
```

Наприклад, якщо ваш git репозиторій має наступну структуру:

```
copilot
├── api
│   └── manifest.yml
└── frontend
    └── manifest.yml
```

І ви хочете контролювати порядок розгортання, так щоб `api` розгорталося перед `frontend`, тоді ви можете налаштувати свій етап наступним чином:

```yaml
stages:
  - name: test
    deployments:
      api:
      frontend:
        depends_on:
          - api
```

Ви також можете обмежити, які мікросервіси випускати в рамках вашого конвеєра. У наступному маніфесті ми вказуємо розгортати лише `api`, а не `frontend`:

```yaml
stages:
  - name: test
    deployments:
      api:
```

Нарешті, якщо `deployments` не вказано, стандартно Copilot розгорне всі ваші сервіси та завдання в git репозиторії паралельно.

<span class="parent-field">stages.deployments.</span><a id="stages-deployments-name" href="#stages-deployments-name" class="field">`<name>`</a> <span class="type">Map</span>  
Назва завдання або сервісу для розгортання.  

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-dependson" href="#stages-deployments-dependson" class="field">`depends_on`</a> <span class="type">Array of Strings</span>  
Опціонально. Назва інших завдань або сервісів, які повинні бути розгорнуті перед розгортанням цього мікросервісу. Стандартно немає залежностей.  

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-stackname" href="#stages-deployments-stackname" class="field">`stack_name`</a> <span class="type">String</span>  
Опціонально. Назва стека для створення або оновлення. Стандартно `<app name>-<stage name>-<deployment name>`.  
Наприклад, якщо ваш застосунок називається `demo`, назва етапу `test`, а назва сервісу `frontend`, тоді назва стека буде `demo-test-frontend`.  

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-templatepath" href="#stages-deployments-templatepath" class="field">`template_path`</a> <span class="type">String</span>  
Опціонально. Шлях до шаблону CloudFormation, створеного під час фази `build`. Стандартно `infrastructure/<deployment name>-<stage name>.yml`.

<span class="parent-field">stages.deployments.`<name>`.</span><a id="stages-deployments-templateconfig" href="#stages-deployments-templatepath" class="field">`template_config`</a> <span class="type">String</span>  
Опціонально. Шлях до конфігурації шаблону CloudFormation, створеної під час фази `build`. Стандартно `infrastructure/<deployment name>-<stage name>.params.json`.

<span class="parent-field">stages.</span><a id="stages-postdeployments" href="#stages-postdeployments" class="field">`post_deployments`</a> <span class="type">Map</span><span class="version">Додано у [v1.30.0](../../../blogs/release-v130/#deployment-actions)</span>  
Опціонально. Додайте дії, які будуть виконані після розгортання. Конфліктує з `stages.test_commands`.

```yaml
stages:
  - name: <env name>
    post_deployments:
      <action name>:
        buildspec: <path to local buildspec>
        depends_on: [<other action's name>, ...]
```

<span class="parent-field">stages.post_deployments.</span><a id="stages-postdeployments-name" href="#stages-postdeployments-name" class="field">`<name>`</a> <span class="type">Map</span> <span class="version">Додано у [v1.30.0](../../../blogs/release-v130/#deployment-actions)</span>  
Назва дії після розгортання.

<span class="parent-field">stages.post_deployments.`<name>`.</span><a id="stages-postdeployments-buildspec" href="#stages-postdeployments-buildspec" class="field">`buildspec`</a> <span class="type">String</span> <span class="version">Додано у [v1.30.0](../../../blogs/release-v130/#deployment-actions)</span>  
Шлях до файлу buildspec, відносно кореня проєкту, для використання в цьому проєкті збірки.

<span class="parent-field">stages.post_deployments.`<name>`.</span><a id="stages-postdeployments-depends_on" href="#stages-postdeployments-dependson" class="field">`depends_on`</a> <span class="type">Array of Strings</span> <span class="version">Додано у [v1.30.0](../../../blogs/release-v130/#deployment-actions)</span>  
Опціонально. Назви інших дій після розгортання, які повинні бути виконані перед виконанням цієї дії. Стандартно немає залежностей.

<span class="parent-field">stages.</span><a id="stages-test-cmds" href="#stages-test-cmds" class="field">`test_commands`</a> <span class="type">Array of Strings</span>  
Опціонально. Команди для запуску інтеграційних або кінцевих тестів після розгортання. Стандартно немає перевірок після розгортання. Конфліктує з `stages.post_deployment`.
