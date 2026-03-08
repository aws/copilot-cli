---
title: Власні ресурси середовища
---

# Власні ресурси середовища

## Імпорт наявних ресурсів

При створенні нового [середовища](../../concepts/environments/) з Copilot, вам надається можливість імпортувати наявні ресурси VPC. (Використовуйте [прапорці з командою `env init`](../../commands/env-init/#what-are-the-flags) або інтерактивний режим, показаний нижче.)

```bash
$ copilot env init
What is your environment's name? env-name
Which credentials would you like to use to create name? [profile default]

  Would you like to use the default configuration for a new environment?
    - A new VPC with 2 AZs, 2 public subnets and 2 private subnets
    - A new ECS Cluster
    - New IAM Roles to manage services and jobs in your environment
  [Use arrows to move, type to filter]
    Yes, use default.
    Yes, but I'd like configure the default resources (CIDR ranges).
  > No, I'd like to import existing resources (VPC, subnets).
```

Ви можете використовувати функцію імпорту для додавання VPC з двома публічними та двома приватними підмережами, лише двома публічними підмережами та без приватних підмереж (наприклад, default VPC), або лише двома приватними підмережами та без публічних підмереж для ваших робочих навантажень, які не мають доступу до Інтернету. (Для отримання додаткової інформації про ресурси, які вам знадобляться для ізольованих мереж, перейдіть [сюди](https://github.com/aws/copilot-cli/discussions/2378).)

## Зміна стандартних ресурсів Copilot

Коли ви вибираєте стандартну конфігурацію, Copilot дотримується [найкращих практик AWS](https://aws.amazon.com/blogs/containers/amazon-ecs-availability-best-practices/) і створює VPC з двома публічними та двома приватними підмережами, з однією з кожного типу в одній з двох зон доступності. Якщо вам потрібні додаткові зони доступності або потрібно змінити діапазони CIDR, ви можете вибрати змінити ці налаштування:
```bash
$ copilot env init --container-insights
What is your environment's name? env-name
Which credentials would you like to use to create name? [profile default]

  Would you like to use the default configuration for a new environment?
    - A new VPC with 2 AZs, 2 public subnets and 2 private subnets
    - A new ECS Cluster
    - New IAM Roles to manage services and jobs in your environment
  [Use arrows to move, type to filter]
    Yes, use default.
  > Yes, but I'd like configure the default resources (CIDR ranges).
    No, I'd like to import existing resources (VPC, subnets).

  What VPC CIDR would you like to use? [? for help] (10.0.0.0/16)

  Which availability zones would you like to use?  [Use arrows to move, space to select, type to filter, ? for more help]
  [x]  us-west-2a
  [x]  us-west-2b
  > [x]  us-west-2c
  [ ]  us-west-2d

  What CIDR would you like to use for your public subnets? [? for help] (10.0.0.0/24,10.0.1.0/24) 10.0.0.0/24,10.0.1.0/24,10.0.2.0/24
  What CIDR would you like to use for your private subnets? [? for help] (10.0.2.0/24,10.0.3.0/24) 10.0.3.0/24,10.0.4.0/24,10.0.5.0/24
```

## Міркування

* Якщо ви імпортуєте наявний VPC, ми рекомендуємо дотримуватися [найкращих практик безпеки для вашого VPC](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-security-best-practices.html) та [розділу Безпека та Фільтрація з FAQ Amazon VPC](https://aws.amazon.com/vpc/faqs/#Security_and_Filtering).
* Якщо ви використовуєте приватну зону хостингу, [ви повинні](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/hosted-zone-private-considerations.html#hosted-zone-private-considerations-vpc-settings) встановити `enableDnsHostname` та `enableDnsSupport` на true.
* Для розгортання робочих навантажень, що мають доступ до Інтернету, у [приватних підмережах](../../manifest/lb-web-service/#network-vpc-placement), ваш VPC потребуватиме [NAT шлюз](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-nat-gateway.html).
