---
title: Змінні середовища в маніфесті
---

# Змінні середовища в маніфесті

## Змінні середовища оболонки

У файлах маніфесту можна використовувати змінні середовища вашої оболонки для заповнення значень:

``` yaml
image:
  location: id.dkr.ecr.zone.amazonaws.com/project-name:${TAG}
```

Припустимо, що в оболонці встановлено `TAG=version01`, приклад маніфесту буде створено як

```yaml
image:
  location: id.dkr.ecr.zone.amazonaws.com/project-name:version01
```

Коли Copilot визначає контейнер, він використовуватиме образ, розташований за адресою `id.dkr.ecr.zone.amazonaws.com/project-name` з теґом `version01`.

Також можна інтерполювати `Array of Strings` зі змінних середовища у ваших файлах маніфесту:

```yaml
network:
  vpc:
    security_groups: ${SECURITY_GROUPS}
```

Припустимо, що в оболонці встановлено `SECURITY_GROUPS=["sg-06b511534b8fa8bbb","sg-06b511534b8fa8bbb","sg-0e921ad50faae7777"]`, приклад маніфесту буде створено як

```yaml
network:
  vpc:
    security_groups:
      - sg-06b511534b8fa8bbb
      - sg-06b511534b8fa8bbb
      - sg-0e921ad50faae7777
```

!!! Info "Інформація"
    Зараз ви можете замінювати змінні середовища оболонки лише для полів, які приймають рядки, включаючи `String` (наприклад, `image.location`), `Масив рядків` (наприклад, `entrypoint`) або `Map`, де тип значення — `String` або `Масив рядків` (наприклад, `secrets`).

## Попередньо визначені змінні

Попередньо визначені змінні — це зарезервовані змінні, які будуть використані Copilot при інтерпретації маніфесту. Наразі доступні попередньо визначені змінні середовища включають:

- COPILOT_APPLICATION_NAME
- COPILOT_ENVIRONMENT_NAME

```yaml
secrets:
   DB_PASSWORD: /copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/db_password
```

Copilot замінить `${COPILOT_APPLICATION_NAME}` та `${COPILOT_ENVIRONMENT_NAME}` на імена застосунку та середовища, де розгорнуто робоче навантаження. Наприклад, коли ви запускаєте

```
copilot svc deploy --app my-app --env test
```

для розгортання служби в середовищі `test` у вашому застосунку `my-app`, Copilot перетворить `/copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/db_password` як `/copilot/my-app/test/secrets/db_password`. (Для отримання додаткової інформації про інʼєкцію секретів дивіться [тут](../../developing/secrets/)).

## Екранування

Якщо підстановка змінних небажана, додайте слеш:

```yaml
command: echo hello \${name}
# або command: "echo \\${name}"
variable:
  name: world
```

У цьому випадку Copilot не намагатиметься замінити `${name}` на значення змінної середовища `name`.
