---
title: Перевизначення завдання
---

# Перевизначення завдання

!!! Attention "Увага"

    :warning: Перевизначення завдання (Task definition overrides) застаріло.

    Ми рекомендуємо використовувати перевизначення [YAML patch](../yamlpatch/), оскільки це дозволяє редагувати весь шаблон CloudFormation і
    підтримує операцію `remove`.

Copilot генерує шаблони CloudFormation використовуючи конфігурацію, вказану в [маніфесті](../../../manifest/overview/). Однак, існують поля, які неможливо налаштувати в маніфесті. Наприклад, ви можете захотіти налаштувати [`Ulimits`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-containerdefinitions.html#cfn-ecs-taskdefinition-containerdefinition-ulimits) для вашого робочого контейнера, але це не доступно в нашому маніфесті.

Ви можете налаштувати додаткові [налаштування визначення завдання ECS](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html) вказавши правила `taskdef_overrides`, які будуть застосовані до шаблону CloudFormation, що Copilot генерує з маніфесту.

## Як вказати правила перевизначення?

Для кожного правила перевизначення потрібно вказати **шлях** до поля ресурсу CloudFormation, яке ви хочете перевизначити, та **значення** цього поля.

Нижче наведено приклад правильного поля `taskdef_overrides`, яке можна застосувати до файлу маніфесту:

``` yaml
taskdef_overrides:
- path: ContainerDefinitions[0].Cpu
  value: 512
- path: ContainerDefinitions[0].Memory
  value: 1024
```

Кожне правило застосовується послідовно до шаблону CloudFormation. Отриманий шаблон CloudFormation стає ціллю для наступного правила. Оцінка продовжується, доки всі правила не будуть успішно застосовані або не виникне помилка.

## Оцінка шляху

- Поле `path` є шляхом, розділеним символом `'.'`, до цільового [поля визначення завдання у властивостях CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html).

- Copilot рекурсивно вставляє поля, якщо вони не існують у шаблоні CloudFormation. Наприклад: якщо правило має шлях `A.B[-].C` (`B` і `C` не існують), Copilot вставить поле `B` і `C`. Конкретний приклад можна знайти [нижче](#add-ulimits-to-the-main-container).

- Якщо цільовий шлях вказує на члена, який вже існує, значення цього члена замінюється.

- Щоб додати нового члена до поля `list`, такого як `Ulimits`, ви можете використовувати спеціальний символ `-`: `Ulimits[-]`.

!!! Attention "Увага"

    Наступні поля у визначенні завдання не дозволено змінювати.

    * [Family](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-taskdefinition.html#cfn-ecs-taskdefinition-family)
    * [ContainerDefinitions[<index>].Name](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-containerdefinitions.html#cfn-ecs-taskdefinition-containerdefinition-name)

## Тестування

Щоб переконатися, що ваші правила перевизначення працюють як очікується, ми рекомендуємо запустити `copilot svc package` або `copilot job package`, щоб переглянути згенерований шаблон CloudFormation.

## Приклади

### Додати `Ulimits` до головного контейнера <a id="add-ulimits-to-the-main-container"></a>

``` yaml
taskdef_overrides:
  - path: ContainerDefinitions[0].Ulimits[-]
    value:
      Name: "cpu"
      SoftLimit: 1024
      HardLimit: 2048
```

### Відкрити додатковий UDP порт

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].PortMappings[-].ContainerPort"
    value: 2056
  # PortMappings[1] отримує привʼязку порту, додану попереднім правилом, оскільки за замовчуванням Copilot створює прив'язку порту.
  - path: "ContainerDefinitions[0].PortMappings[1].Protocol"
    value: "udp"
```

### Надати доступ лише для читання до кореневої файлової системи

``` yaml
taskdef_overrides:
  - path: "ContainerDefinitions[0].ReadonlyRootFilesystem"
    value: true
```
