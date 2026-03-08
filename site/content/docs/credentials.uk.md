---
title: Облікові дані
---

# Облікові дані

AWS Copilot використовує облікові дані AWS для доступу до AWS API, зберігання та пошуку [метаданих застосунку](../concepts/applications/), розгортання та керування робочими навантаженнями застосунку.

Ви можете дізнатися більше про налаштування облікових даних AWS у [документації AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html).

## Облікові дані застосунку

Copilot використовує облікові дані AWS зі [стандартного ланцюжка постачальника облікових даних](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials) для зберігання та пошуку [метаданих вашого застосунку](../concepts/applications/): які сервіси та середовища до нього належать.

!!! tip "Рекомендація"
    Ми **рекомендуємо використовувати [іменований профіль](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html)** для зберігання облікових даних вашого застосунку.

Найзручніший спосіб — налаштувати профіль `[default]` з обліковими даними вашого застосунку:

```ini
# ~/.aws/credentials
[default]
aws_access_key_id=AKIAIOSFODNN7EXAMPLE
aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# ~/.aws/config
[default]
region=us-west-2
```

Альтернативно, ви можете встановити змінну середовища `AWS_PROFILE`, щоб вказати інший іменований профіль. Наприклад, ми можемо мати профіль `[my-app]`, який можна використовувати для вашого застосунку Copilot замість профілю `[default]`.

!!! note "Примітка"
    Ви **не можете** використовувати облікові дані кореневого користувача AWS для вашого застосунку. Будь ласка, спочатку створіть користувача IAM, як описано [тут](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_root-user.html).

```ini
# ~/.aws/config
[profile my-app]
credential_process = /opt/bin/awscreds-custom --username helen
region=us-west-2

# Потім ви можете виконувати команди Copilot, використовуючи альтернативний профіль:
$ export AWS_PROFILE=my-app
$ copilot deploy
```

!!! caution "Увага"
    Ми **не рекомендуємо** використовувати змінні середовища: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` безпосередньо для пошуку метаданих вашого застосунку, оскільки якщо вони будуть перевизначені або закінчаться, Copilot не зможе знайти ваші сервіси або середовища.

Щоб дізнатися більше про всі підтримувані налаштування файлу `config`: [Налаштування файлів конфігурації та облікових даних](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html#cli-configure-files-settings).

## Облікові дані середовища

[Облікові дані середовища](../concepts/environments/) Copilot можуть бути створені в облікових записах AWS та регіонах, відмінних від вашого застосунку. Під час ініціалізації середовища Copilot запропонує вам ввести тимчасові облікові дані або [іменований профіль](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) для створення вашого середовища:

```bash
$ copilot env init

Name: prod-iad

  Which credentials would you like to use to create prod-iad?
  > Enter temporary credentials
  > [profile default]
  > [profile test]
  > [profile prod-iad]
  > [profile prod-pdx]
```

На відміну від [Облікових даних застосунку](#облікові-дані-застосунку), облікові дані AWS для середовища потрібні лише для створення або видалення. Тому безпечно використовувати значення з тимчасових змінних середовища. Copilot запитує або приймає облікові дані як прапорці, оскільки стандартний ланцюжок зарезервований для облікових даних вашого застосунку.
