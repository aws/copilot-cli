---
title: Спостережуваність
---

# Спостережуваність

## Початок роботи

Copilot може налаштувати збір трасування для сервісів, встановивши наступне в маніфесті:

```yaml
observability:
  tracing: awsxray
```

Для [сервісів з керуванням запитами](../../concepts/services/#request-driven-web-service), Copilot увімкне вбудовану конфігурацію трасування App Runner [tracing configuration](https://docs.aws.amazon.com/apprunner/latest/dg/monitor-xray.html).

Для [сервісів з балансуванням навантаження](../../concepts/services/#load-balanced-web-service), [бекенд-сервісів](../../concepts/services/#backend-service) та [сервісів робочих навантажень](../../concepts/services/#worker-service), Copilot розгорне [AWS OpenTelemetry Collector](https://github.com/aws-observability/aws-otel-collector) як [sidecar](../sidecars/).

## Інструментування вашого сервісу

Інструментування вашого сервісу для надсилання телеметричних даних здійснюється за допомогою [SDK для відповідних мов](https://opentelemetry.io/docs/instrumentation/). Приклади надаються в документації OpenTelemetry для кожної підтримуваної мови. Ви також можете переглянути документацію та приклади, надані [AWS Distro for OpenTelemetry](https://aws-otel.github.io/docs/introduction).

### Приклад застосунку

Це невеликий [Express.js](https://expressjs.com/) сервіс з налаштованим інструментуванням для всіх його кінцевих точок. Щоб почати, встановіть необхідні залежності:

```sh
npm install express \
	@opentelemetry/api \
	@opentelemetry/sdk-trace-node \
	@opentelemetry/auto-instrumentations-node \
	@opentelemetry/exporter-trace-otlp-grpc \
	@opentelemetry/id-generator-aws-xray \
	@opentelemetry/propagator-aws-xray
```

Потім збережіть наступне у `tracer.js`:

```js title="tracer.js" linenums="1"
const { BatchSpanProcessor } = require("@opentelemetry/sdk-trace-base");
const { Resource } = require("@opentelemetry/resources");
const { trace } = require("@opentelemetry/api");
const { AWSXRayIdGenerator } = require("@opentelemetry/id-generator-aws-xray");
const { SemanticResourceAttributes } = require("@opentelemetry/semantic-conventions");
const { NodeTracerProvider } = require("@opentelemetry/sdk-trace-node");
const { AWSXRayPropagator } = require("@opentelemetry/propagator-aws-xray");
const { OTLPTraceExporter } = require("@opentelemetry/exporter-trace-otlp-grpc");
const { getNodeAutoInstrumentations } = require("@opentelemetry/auto-instrumentations-node");

module.exports = (serviceName) => {
  const tracerConfig = {
    idGenerator: new AWSXRayIdGenerator(),
    instrumentations: [getNodeAutoInstrumentations()],
    resource: Resource.default().merge(
      new Resource({
        [SemanticResourceAttributes.SERVICE_NAME]: serviceName,
      })
    ),
  };

  const tracerProvider = new NodeTracerProvider(tracerConfig);
  const otlpExporter = new OTLPTraceExporter();

  tracerProvider.addSpanProcessor(new BatchSpanProcessor(otlpExporter));
  tracerProvider.register({
    propagator: new AWSXRayPropagator(),
  });

  return trace.getTracer("example-instrumentation");
};
```

`tracer.js` експортує функцію, яка повертає трасувальник, налаштований на [автоматичне](https://www.npmjs.com/package/@opentelemetry/auto-instrumentations-node#user-content-supported-instrumentations) експортування трейсів з сервера Express.js за допомогою [OpenTelemetry Protocol](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md). Ми будемо використовувати цю функцію в `app.js`, передаючи імʼя сервісу, `copilot-observability`.

```js title="app.js" linenums="1"
'use strict';
const tracer = require('./tracer')('copilot-observability');
const app = require("express")();
const port = 8080;

app.get("/", (req, res) => {
	res.send("Hello World");
});

app.listen(port, () => {
	console.log(`Listening for requests on http://localhost:${port}`);
});
```

Тепер, якщо ви розгорнете цей сервіс за допомогою Copilot і увімкнете спостережуваність у маніфесті, ви зможете [побачити трасування, створені цим сервісом!](../observability/#viewing-traces-in-cloudwatch)

### Включення журналів трасування

!!! attention "Увага"
	 Цей розділ не застосовується до сервісів з керуванням запитами

Оскільки Copilot налаштовує [детектор ресурсів ECS](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor#amazon-ecs) на колекторі, трасування, створені вашим сервісом, будуть включати групу журналів, до якої ваш сервіс записує. Якщо ви включите ідентифікатор трасування у ваші журнали, журнали, повʼязані з трасуванням, зʼявляться разом з трасуванням у X-Ray. Це може бути корисним для розуміння та налагодження трасування.

X-Ray [форматує свої ідентифікатори трасування](https://docs.aws.amazon.com/xray/latest/devguide/xray-api-sendingdata.html#xray-api-traceids) трохи [інакше, ніж OpenTelemetry](https://opentelemetry.io/docs/reference/specification/trace/api/#spancontext), тому вам знадобиться функція, як ця, щоб взяти ідентифікатор трасування OpenTelemetry і відформатувати його для X-Ray:

```js
function getXRayTraceId(span) {
	const id = span.spanContext().traceId;
	if (id.length < 9) {
		return id;
	}

	return "1-" + id.substring(0, 8) + "-" + id.substring(8);
}
```

Потім ви можете включити ідентифікатор трасування X-Ray у ваші журнали таким чином:

```js
console.log("[%s] Корисне повідомлення журналу", getXRayTraceId(span));
```

## Перегляд трасувань у CloudWatch <a id="viewing-traces-in-cloudwatch"></a>

Після того, як ви інструментували свій сервіс і розгорнули його за допомогою Copilot, ви готові переглядати трасування з вашого сервісу! Залежно від того, як ви інструментували свій сервіс, вам, ймовірно, потрібно буде надіслати кілька запитів до нього, перш ніж трасування зʼявляться в консолі AWS.

Спочатку відкрийте консоль CloudWatch і натисніть на `X-Ray traces/Service map` у меню. Тут ви можете побачити візуальну мапу взаємодії сервісів:
![X-Ray Service Map](https://user-images.githubusercontent.com/10566468/166842664-da44756f-7a4b-4e5d-9981-42927b0deb65.png)

Далі ви можете переглянути деталі конкретного трасування, натиснувши на `X-Ray traces/Traces` у меню та вибравши трасування зі списку.

У цьому прикладі ви можете побачити сервіс, `js-copilot-observability`, який виконує деякі внутрішні проміжні програмні засоби Express.js, а потім використовує [AWS SDK для Javascript](https://aws.amazon.com/sdk-for-javascript/) для виклику `s3:listBuckets`:
![X-Ray Trace Details](https://user-images.githubusercontent.com/10566468/166842693-65558de5-5a6b-4777-b687-812406580fb6.png)
