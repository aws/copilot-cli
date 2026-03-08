---
title: "Комунікація між сервісами"
---

# Комунікація між сервісами

## Service Connect <span class="version" > додано у v1.24.0 </span>

[ECS Service Connect](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html) дозволяє клієнтському сервісу підключатися до його залежних сервісів із балансуванням навантаження та стійкістю. Крім того, він спрощує спосіб надання доступу до сервісу для його клієнтів шляхом визначення дружніх псевдонімів. За допомогою Service Connect у Copilot кожному створеному сервісу стандартно надається наступний приватний псевдонім: `http://<назва вашого сервісу>`.

!!! attention "Увага"
    Service Connect ще не підтримується для [Request-Driven Web Services](../../concepts/services/#request-driven-web-service).

### Як використовувати Service Connect?

Уявімо, що у нас є застосунок з назвою `kudos` і два сервіси: `api` та `front-end`, розгорнуті в одному середовищі. Щоб використовувати Service Connect, маніфести обох сервісів повинні мати:

???+ note "Приклади налаштування маніфесту Service Connect"

    === "Базовий"
        ```yaml
        network:
          connect: true # Стандартно "false"
        ```

    === "Власний аліас"
        ```yaml
        network:
          connect:
            alias: frontend.local
        ```

Після розгортання обох сервісів вони повинні мати змогу спілкуватися один з одним, використовуючи стандартну кінцеву точку Service Connect, яка збігаєтся з назвою сервісу. Наприклад, сервіс `front-end` може просто викликати `http://api`.

```go
// Виклик сервісу "api" з сервісу  "front-end".
resp, err := http.Get("http://api/")
```

### Оновлення з Service Discovery

До v1.24, Copilot дозволяв приватну комунікацію між сервісами за допомогою [Service Discovery](#service-discovery). Якщо ви вже використовуєте Service Discovery і хочете уникнути будь-яких змін у коді, ви можете налаштувати поле [`network.connect.alias`](../../manifest/lb-web-service/#network-connect-alias), щоб Service Connect використовував той самий псевдонім, що й Service Discovery. І якщо **обидва**, сервіс і його клієнт, мають увімкнений Service Connect, вони будуть підключатися через Service Connect замість Service Discovery. Наприклад, у маніфесті сервісу `api` ми маємо

```yaml
network:
  connect:
    alias: ${COPILOT_SERVICE_NAME}.${COPILOT_ENVIRONMENT_NAME}.${COPILOT_APPLICATION_NAME}.local
```

і `front-end` також має те саме налаштування. Тоді вони можуть продовжувати використовувати ту саму кінцеву точку для здійснення API викликів через Service Connect замість Service Discovery, щоб скористатися перевагами балансування навантаження та додаткової стійкості.

## Service Discovery

Service Discovery — це спосіб дозволити сервісам знаходити та підключатися один до одного. Зазвичай сервіси можуть спілкуватися один з одним, лише якщо вони надають публічну кінцеву точку, і навіть тоді запити повинні проходити через інтернет. За допомогою [ECS Service Discovery](https://docs.aws.amazon.com/whitepapers/latest/microservices-on-aws/service-discovery.html) кожному створеному сервісу надається приватна адреса та DNS-імʼя — це означає, що кожен сервіс може спілкуватися з іншим, не виходячи за межі локальної мережі (VPC) і не надаючи публічну кінцеву точку.

### Як використовувати Service Discovery?

Service Discovery увімкнено для всіх сервісів, налаштованих за допомогою Copilot CLI. Ми покажемо вам, як його використовувати, на прикладі. Уявімо, що у нас є той самий застосунок `kudos` з двома сервісами: `api` та `front-end`.

У цьому прикладі уявімо, що наш сервіс `front-end` розгорнуто в середовищі `test`, має публічну кінцеву точку і хоче викликати наш сервіс `api`, використовуючи його кінцеву точку service discovery.

```go
// Виклик нашого api сервісу з frontend сервісу за допомогою Service Discovery
func ServiceDiscoveryGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    endpoint := fmt.Sprintf("http://api.%s/some-request", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
    resp, err := http.Get(endpoint /* http://api.test.kudos.local/some-request */)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    w.WriteHeader(http.StatusOK)
    w.Write(body)
}
```

Важливо, що наш сервіс `front-end` робить запит до нашого сервісу `api` через спеціальну кінцеву точку:

```go
endpoint := fmt.Sprintf("http://api.%s/some-request", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
```

`COPILOT_SERVICE_DISCOVERY_ENDPOINT` — це спеціальна змінна середовища, яку Copilot CLI встановлює для вас під час створення сервісу. Вона має формат _{env name}.{app name}.local_, тому в цьому випадку в нашому застосунку _kudos_, коли він розгорнутий у середовищі _test_, запит буде до `http://api.test.kudos.local/some-request`. Оскільки наш сервіс _api_ працює на порту 80, ми не вказуємо порт в URL. Однак, якщо він працював би на іншому порту, наприклад 8080, нам потрібно було б включити порт у запит, а також `http://api.test.kudos.local:8080/some-request`.

Коли наш front-end робить цей запит, кінцева точка `api.test.kudos.local` перетворюється в приватну IP-адресу і маршрутизується приватно в межах вашого VPC.

### Старі середовища та Service Discovery

До Copilot v1.9.0, простір імен service discovery використовував формат _{app name}.local_, без включення середовища. Це обмеження унеможливлювало розгортання кількох середовищ в одному VPC. Будь-які середовища, створені за допомогою Copilot v1.9.0 і новіших версій, можуть спільно використовувати VPC з будь-яким іншим середовищем.

Коли ваші середовища оновлюються, Copilot буде враховувати простір імен service discovery, з яким було створено середовище. Це означає, що кінцеві точки для ваших сервісів не зміняться. Будь-які нові середовища, створені за допомогою Copilot v1.9.0 і вище, будуть використовувати формат _{env name}.{app name}.local_ для service discovery і можуть спільно використовувати VPC зі старими середовищами.
