---
title: CDK Перевизначення
---

# CDK Перевизначення

{% include 'overrides-intro.uk.md' %}

## Коли слід використовувати CDK перевизначення замість YAML патчів?

Обидва варіанти є механізмом "розбиття скла" для доступу та налаштування функціональності, яка не представлена в Copilot [маніфестах](../../../manifest/overview/).

Ми рекомендуємо використовувати AWS Cloud Development Kit (CDK) перевизначення замість [YAML патчів](../yamlpatch/), якщо ви хочете використати виразну потужність мови програмування. CDK дозволяє вам робити безпечні та потужні модифікації вашого шаблону CloudFormation.

## Як почати

Ви можете розширити ваш шаблон CloudFormation за допомогою CDK, запустивши команду `copilot [noun] override`. Наприклад, ви можете запустити `copilot svc override` для оновлення шаблону Load Balanced Web Service.

Команда згенерує новий CDK застосунок у теці `copilot/[name]/override` з наступною структурою:

```console
.
├── bin/
│   └── override.ts
├── .gitignore
├── cdk.json
├── package.json
├── README.md
├── stack.ts
└── tsconfig.json
```

Ви можете почати, відредагувавши файл `stack.ts`. Наприклад, якщо ви вирішили перевизначити властивості ECS сервісу за допомогою `copilot svc override`, наступний файл `stack.ts` буде згенеровано для вас для модифікації:

```typescript
import * as cdk from 'aws-cdk-lib';
import { aws_ecs as ecs } from 'aws-cdk-lib';

export class TransformedStack extends cdk.Stack {
    constructor (scope: cdk.App, id: string, props?: cdk.StackProps) {
        super(scope, id, props);
         this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
            templateFile: path.join('.build', 'in.yaml'),
        });
        this.appName = template.getParameter('AppName').valueAsString;
        this.envName = template.getParameter('EnvName').valueAsString;

        this.transformService();
    }
 
    // TODO: implement me.
    transformService() {
      const service = this.template.getResource("Service") as ecs.CfnService;
      throw new error("not implemented");
    }
}
```

## Як це працює?

Як видно з наведеного вище файлу `stack.ts`, Copilot використовуватиме [модуль cloudformation_include](https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.cloudformation_include-readme.html) наданий CDK для допомоги в здійсненні трансформацій. Ця бібліотека є рекомендацією CDK з їхнього керівництва ["Імпорт або міграція наявного шаблону AWS CloudFormation"](https://docs.aws.amazon.com/cdk/v2/guide/use_cfn_template.html). Вона дозволяє доступ до ресурсів, які не представлені в маніфесті Copilot як [L1 конструкції](https://docs.aws.amazon.com/cdk/v2/guide/constructs.html).  
Обʼєкт `CfnInclude` ініціалізується з прихованого шаблону CloudFormation `.build/in.yaml`. Це спосіб комунікації між Copilot та CDK. Copilot записує шаблон CloudFormation, згенерований маніфестом, у теку `.build/`, який потім розбирається бібліотекою `cloudformation_include` у конструкцію CDK.

Кожного разу, коли ви запускаєте `copilot [noun] package` або `copilot [noun] deploy`, Copilot спочатку згенерує шаблон CloudFormation з файлу маніфесту, а потім передасть його вашому CDK застосунку для перевизначення властивостей.

Ми наполегливо рекомендуємо використовувати прапорець `--diff` з командою `package` або `deploy`, щоб спочатку візуалізувати ваші зміни CDK перед розгортанням.

## Приклади

Наступний приклад модифікує ресурс [`nlb`](../../../manifest/lb-web-service/#nlb) Load Balanced Web Service для призначення Elastic IP адрес Network Load Balancer.

У цьому прикладі ви можете побачити, як:

- Видалити властивість ресурсу.
- Створити нові ресурси.
- Модифікувати властивість наявного ресурсу.

??? note "Переглянути приклад `stack.ts`"

    ```typescript
    import * as cdk from 'aws-cdk-lib';
    import * as path from 'path';
    import { aws_elasticloadbalancingv2 as elbv2 } from 'aws-cdk-lib';
    import { aws_ec2 as ec2 } from 'aws-cdk-lib';
    
    interface TransformedStackProps extends cdk.StackProps {
        readonly appName: string;
        readonly envName: string;
    }
    
    export class TransformedStack extends cdk.Stack {
        public readonly template: cdk.cloudformation_include.CfnInclude;
        public readonly appName: string;
        public readonly envName: string;
    
        constructor (scope: cdk.App, id: string, props: TransformedStackProps) {
            super(scope, id, props);
            this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
                templateFile: path.join('.build', 'in.yml'),
            });
            this.appName = props.appName;
            this.envName = props.envName;
            this.transformPublicNetworkLoadBalancer();
        }
    
        /**
         * transformPublicNetworkLoadBalancer видаляє властивості "Subnets" з NLB,
         * і додає SubnetMappings з попередньо визначеними Elastic IP адресами.
         */
        transformPublicNetworkLoadBalancer() {
            const elasticIPs = [new ec2.CfnEIP(this, 'ElasticIP1'), new ec2.CfnEIP(this, 'ElasticIP2')];
            const publicSubnets = cdk.Fn.importValue(`${this.appName}-${this.envName}-PublicSubnets`);
    
            // Застосувати перевизначення.
            const nlb = this.template.getResource("PublicNetworkLoadBalancer") as elbv2.CfnLoadBalancer;
            nlb.addDeletionOverride('Properties.Subnets');
            nlb.subnetMappings = [{
                allocationId: elasticIPs[0].attrAllocationId,
                subnetId: cdk.Fn.select(0, cdk.Fn.split(",", publicSubnets)),
            }, {
                allocationId: elasticIPs[1].attrAllocationId,
                subnetId: cdk.Fn.select(1, cdk.Fn.split(",", publicSubnets)),
            }]
        }
    }
    ```

Наступний приклад демонструє, як можна додати властивість лише для певного середовища, наприклад, для промислового середовища:

??? note "Переглянути приклад `stack.ts`"

    ```typescript
    import * as cdk from 'aws-cdk-lib';
    import * as path from 'path';
    import { aws_iam as iam } from 'aws-cdk-lib';
    
    interface TransformedStackProps extends cdk.StackProps {
        readonly appName: string;
        readonly envName: string;
    }
    
    export class TransformedStack extends cdk.Stack {
        public readonly template: cdk.cloudformation_include.CfnInclude;
        public readonly appName: string;
        public readonly envName: string;
    
        constructor (scope: cdk.App, id: string, props: TransformedStackProps) {
            super(scope, id, props);
            this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
                templateFile: path.join('.build', 'in.yml'),
            });
            this.appName = props.appName;
            this.envName = props.envName;
            this.transformEnvironmentManagerRole();
        }
        
        transformEnvironmentManagerRole() {
            const environmentManagerRole = this.template.getResource("EnvironmentManagerRole") as iam.CfnRole;
            if (this.envName === "prod") {
                let assumeRolePolicy = environmentManagerRole.assumeRolePolicyDocument
                let statements = assumeRolePolicy.Statement
                statements.push({
                     "Effect": "Allow",
                     "Principal": { "Service": "ec2.amazonaws.com" },
                     "Action": "sts:AssumeRole"
                })
            }
        }
    }
    ```

Наступний приклад демонструє, як можна видалити ресурс, створену Copilot стандартну групу логів, яка містить журнали сервісу.

??? note "Переглянути приклад `stack.ts`"

    ```typescript
    import * as cdk from 'aws-cdk-lib';
    import * as path from 'path';

    interface TransformedStackProps extends cdk.StackProps {
        readonly appName: string;
        readonly envName: string;
    }

    export class TransformedStack extends cdk.Stack {
        public readonly template: cdk.cloudformation_include.CfnInclude;
        public readonly appName: string;
        public readonly envName: string;

        constructor(scope: cdk.App, id: string, props: TransformedStackProps) {
            super(scope, id, props);
            this.template = new cdk.cloudformation_include.CfnInclude(this, 'Template', {
            templateFile: path.join('.build', 'in.yml'),
            });
            this.appName = props.appName;
            this.envName = props.envName;
            // Видаляє ресурс стандартну групу логів.
            this.template.node.tryRemoveChild("LogGroup")
        }
    }
    ```
