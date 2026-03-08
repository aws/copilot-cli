<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>    
Підмережі та групи безпеки, прикріплені до ваших завдань.

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String або Map</span>  
Якщо використовується як рядок, значення повинно бути одним із `'public'` або `'private'`. Стандартно завдання запускаються в публічних підмережах.

!!! info "Інформація"
    Якщо ви запускаєте завдання в `'private'` підмережах і використовуєте VPC, створену Copilot, Copilot автоматично додасть NAT шлюзи до вашого середовища для підключення до Інтернету. (Див. [ціни](https://aws.amazon.com/vpc/pricing/).) Крім того, під час запуску `copilot env init` ви можете імпортувати наявний VPC з NAT шлюзами або з кінцевими точками VPC для ізольованих робочих навантажень. Див. нашу сторінку [ресурси користувацького середовища](../../developing/custom-environment-resources/) для отримання додаткової інформації.

Якщо використовується як map, ви можете вказати, в яких підмережах Copilot повинен запускати завдання ECS. Наприклад:

```yaml
network:
  vpc:
    placement:
      subnets: ["SubnetID1", "SubnetID2"]
```

<span class="parent-field">network.vpc.placement.</span><a id="network-vpc-placement-subnets" href="#network-vpc-placement-subnets" class="field">`subnets`</a> <span class="type">Array of Strings або Map</span>  
Як список рядків, ідентифікатори підмереж, де Copilot повинен запускати завдання ECS.

Як map, пари імʼя-значення, за якими фільтрувати ваші підмережі. Зверніть увагу, що фільтри обʼєднуються за допомогою `AND`, а значення для кожного фільтра обʼєднуються за допомогою `OR`. Наприклад, підмережі з теґами `org: bi` і `type: public`, а також підмережі з теґами `org: bi` і `type: private` будуть відповідати

```yaml
network:
  vpc:
    placement:
      subnets:
        from_tags:
          org: bi
          type:
            - public
            - private
```

<span class="parent-field">network.vpc.placement.subnets</span><a id="network-vpc-placement-subnets-from-tags" href="#network-vpc-placement-subnets-from-tags" class="field">`from_tags`</a> <span class="type">Map of String та String або Array of Strings</span>  
Набори теґів, за якими фільтрувати підмережі, де Copilot повинен запускати завдання ECS.

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings або Map</span>  
Додаткові ідентифікатори груп безпеки, повʼязані з вашими завданнями.

```yaml
network:
  vpc:
    security_groups: [sg-0001, sg-0002]
```

Copilot включає групу безпеки, щоб контейнери у вашому середовищі могли спілкуватися один з одним. Щоб вимкнути стандартну групу безпеки, ви можете вказати форму `Map`:

```yaml
network:
  vpc:
    security_groups:
      deny_default: true
      groups: [sg-0001, sg-0002]
```

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-from-cfn" href="#network-vpc-security-groups-from-cfn" class="field">`from_cfn`</a> <span class="type">String</span>  
Назва [експорту стека CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html).

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-deny-default" href="#network-vpc-security-groups-deny-default" class="field">`deny_default`</a> <span class="type">Boolean</span>  
Вимкніть стандартну групу безпеки, яка дозволяє вхідний трафік від усіх сервісів у вашому середовищі.

<span class="parent-field">network.vpc.security_groups.</span><a id="network-vpc-security-groups-groups" href="#network-vpc-security-groups-groups" class="field">`groups`</a> <span class="type">Array of Strings</span>  
Додаткові ідентифікатори груп безпеки, повʼязані з вашими завданнями.

<span class="parent-field">network.vpc.security_groups.groups</span><a id="network-vpc-security-groups-groups-from-cfn" href="#network-vpc-security-groups-groups-from-cfn" class="field">`from_cfn`</a> <span class="type">String</span>  
Назва [експорту стека CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-stack-exports.html).
