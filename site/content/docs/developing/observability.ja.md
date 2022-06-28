# 可観測性 (Observability)

## はじめに
Copilot では、Manifest にて以下のように設定することにより、Service のトレース収集を設定することができます。
```yaml
observability:
  tracing: awsxray
```

[Request-Driven Web Service](../concepts/services.ja.md#request-driven-web-service) の場合、Copilot は App Runner に組み込まれた[トレース設定](https://docs.aws.amazon.com/ja_jp/apprunner/latest/dg/monitor-xray.html)を有効にします。

[Load Balanced Web Service](../concepts/services.ja.md#load-balanced-web-service)、[Backend Service](../concepts/services.ja.md#backend-service)、[Worker Service](../concepts/services.ja.md#worker-service) の場合、Copilot は AWS OpenTelemetry Collector を[サイドカー](./sidecars.ja.md)としてデプロイします。

## Service のインストルメント化
テレメトリーデータを送信するための Service のインストルメント化 (訳注: 計装、 アプリケーションに計測のためのコードを追加すること) は、[各言語毎の SDK](https://opentelemetry.io/docs/instrumentation/)で行います。サンプルは、OpenTelemetry のドキュメントでサポートされている各言語で提供されています。また、[AWS Distro for OpenTelemetry](https://aws-otel.github.io/docs/introduction) が提供するドキュメントやサンプルをご覧いただいたけます。

### アプリケーションの例

これは、すべてのエンドポイントがインストルメント化された [Express.js](https://expressjs.com/) による小さな Service です。開始するには、必要な依存関係をインストールします。

```
npm install express \
	@opentelemetry/api \
	@opentelemetry/sdk-trace-node \
	@opentelemetry/auto-instrumentations-node \
	@opentelemetry/exporter-trace-otlp-grpc \
	@opentelemetry/id-generator-aws-xray \
	@opentelemetry/propagator-aws-xray
```

次に、以下を tracer.js に保存します。

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

`tracer.js` は、[OpenTelemetry プロトコル](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md)を使用して Express.js サーバからトレースを[自動的](https://www.npmjs.com/package/@opentelemetry/auto-instrumentations-node#user-content-supported-instrumentations)にエクスポートするように構成されたトレーサーを返す関数をエクスポートします。この関数を `app.js` で使用し、Service 名として `copilot-observability` を渡します。

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

これで、Copilot を使用してこの Service をデプロイし、Manifest で observability を有効にすると、[この Service によって生成されたトレースを確認できます](#cloudwatch-%E3%81%A7%E3%83%88%E3%83%AC%E3%83%BC%E3%82%B9%E3%82%92%E8%A1%A8%E7%A4%BA%E3%81%99%E3%82%8B)。

### トレースログのインクルード
!!!attention
	このセクションは Request-Driven Web Services には適用されません。

Copilot はコレクターで [ECS resource detector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor#amazon-ecs) を構成するため、Service によって生成されたトレースには、サービスがログを記録しているロググループが含まれます。
ログにトレース ID を含めると、トレースに関連するログが X-Ray でトレースと一緒に表示されます。これは、トレースを理解しデバッグするのに便利です。

X-Ray は [OpenTelemetry とは少し異なる形式](https://opentelemetry.io/docs/reference/specification/trace/api/#spancontext)で[トレース ID をフォーマット](https://docs.aws.amazon.com/ja_jp/xray/latest/devguide/xray-api-sendingdata.html#xray-api-traceids)するので、OpenTelemetry のトレース ID を取得して X-Ray 用にフォーマットするには、このような関数が必要になります。
```js
function getXRayTraceId(span) {
	const id = span.spanContext().traceId;
	if (id.length < 9) {
		return id;
	}

	return "1-" + id.substring(0, 8) + "-" + id.substring(8);
}
```

次に、X-Ray トレース ID をログに次のように含めることができます。
```js
console.log("[%s] A useful log message", getXRayTraceId(span));
```

## CloudWatch でトレースを表示する
Service をインストルメント化し Copilot を使用してデプロイすることで、Service のトレースを表示する準備が整いました。Service のインストルメント化の方法によっては、AWSコンソールにトレースが表示される前に、数回リクエストを送信する必要があります。

まず、CloudWatch のコンソールを開き、メニューの `X-Ray トレース/サービスマップ` をクリックします。ここでは、相互作用しているサービスのビジュアルマップを見ることができます。
![X-Ray Service Map](https://user-images.githubusercontent.com/10566468/166842664-da44756f-7a4b-4e5d-9981-42927b0deb65.png)

次に、メニューの `X-Ray トレース/トレース` をクリックし、リストからトレースを選択すると、特定のトレースの詳細を表示することができます。

この例では、`js-copilot-observability` という Service が内部で Express.js のミドルウェアを実行し、[AWS SDK for Javascript](https://aws.amazon.com/jp/sdk-for-javascript/) を使って `s3:listBuckets` を呼び出していることが分かります。
![X-Ray Trace Details](https://user-images.githubusercontent.com/10566468/166842693-65558de5-5a6b-4777-b687-812406580fb6.png)
