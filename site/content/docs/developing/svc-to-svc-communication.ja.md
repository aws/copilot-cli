# サービス間通信

## Service Connect <span class="version" > v1.24.0 </span>

[ECS Service Connect](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/service-connect.html) を使うとクライアント Service が負荷分散された弾力的な方法で、ダウンストリームの Service に接続できます。さらに分かりやすいエイリアスを指定することで、Service をクライアントに公開する方法を簡単にします。Copilot における Service Connect は、作成した各 Service にデフォルトで次の様なプライベートエイリアスを付与します：`http://<your service name>` 。

!!! attention
    Service Connect は [Request-Driven Web Services](../concepts/services.ja.md#request-driven-web-service) ではまだサポートされていません。

### Service Connect の使い方は？
`kudos` という Application と同じ Environment にデプロイされた `api` と `front-end` という 2 つの Service があるとします。Service Connect を利用する為には、両方の Service の Manifest に設定が必要です。

???+ note "Service Connect Manifest の設定例"

    === "Basic"
        ```yaml
        network:
          connect: true # Defaults to "false"
        ```

    === "Custom Alias"
        ```yaml
        network:
          connect:
            alias: frontend.local
        ```

両方の Service のデプロイ後、 Service はデフォルトの Service Connect エンドポイントを使ってお互いに通信できるはずです。エンドポイントは Service 名と同じです。例えば、`front-end` Service は、単純に `http://api` を呼び出せます。

```go
// Calling the "api" service from the "front-end" service.
resp, err := http.Get("http://api/")
```

### サービスディスカバリからの更新

v1.24 以前の Copilot では、[サービスディスカバリ](#service-discovery) を使用したプライベートなサービス間通信が可能でした。既にサービスディスカバリを利用していて、コードの変更を避けたい場合、[`network.connect.alias`](../manifest/lb-web-service.ja.md#network-connect-alias) を設定し、Service Connect がサービスディスカバリと同じエイリアスを使う様にします。Service とそのクライアントの両方が Service Connect を有効にしている場合、サービスディスカバリの代わりに Service Connect を経由して接続します。例えば、 `api` Service の Manifest を次の様にします。


```yaml
network:
  connect:
    alias: ${COPILOT_SERVICE_NAME}.${COPILOT_ENVIRONMENT_NAME}.${COPILOT_APPLICATION_NAME}.local
```
`front-end` Service も同様の設定にします。そうすると、サービスディスカバリの代わりに、Service Connect 経由で API 呼び出しをする際に同じエンドポイントを利用し続けられます。

## サービスディスカバリ

サービスディスカバリ はサービス同士が違いに発見し、通信する為の仕組みです。一般的には、サービスはパブリックエンドポイントを公開した場合のみ、互いに通信できます。その場合でも、リクエストはインターネットを経由する必要があるでしょう。[ECS サービスディスカバリ](https://docs.aws.amazon.com/ja_jp/whitepapers/latest/microservices-on-aws/service-discovery.html)を使うと、作成した各サービスはプライベート IP アドレスと DNS 名が付与されます。つまり、各サービスはローカルネットワーク (VPC) を出ることなく、パブリックエンドポイントを公開せずに、他のサービスと通信します。

### サービスディスカバリを使うには？

サービスディスカバリは Copilot CLI を利用して設定されたすべての Service で有効化されています。例を使ってどの様に利用するか説明します。同様に `api` と `front-end` という 2 つの Service がある `kudos` という Application があるとします。

この例では、 `test` という Envrionment に `front-end` Service がデプロイされていて、パブリックエンドポイントを持っています。そして、サービスディスカバリエンドポイントを利用して `api` Service を呼び出そうとしています。

```go
// Calling our api service from the front-end service using Service Discovery
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

重要なのは、`front-end` Service が特別なエンドポイントを経由して `api` Servcie に対してリクエストしていることです。

```go
endpoint := fmt.Sprintf("http://api.%s/some-request", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
```

`COPILOT_SERVICE_DISCOVERY_ENDPOINT` は特別な環境変数で、Copilot CLI が Service を作成する際に設定されます。その形式は、_{env name}.{app name}.local_ です。 つまり、今回の _kudos_ Application の場合、_test_ Environment にデプロイすると `http://api.test.kudos.local/some-request` にリクエストされます。_api_ Service は 80 番ポートで動いているので、URL ではポートを指定していません。他のポートで動いている場合、例えば 8080 で動いている場合は、`http://api.test.kudos.local:8080/some-request` の様にリクエストにポートを含める必要があります。

front-end Service がこのリクエストを行うとエンドポイント `api.test.kudos.local` はプライベート IP アドレスに解決され、 VPC 内でプライベートにルーティングされます。

### 古い Environment と サービスディスカバリ

Copilot v1.9.0 より前のバージョンでは、 サービスディスカバリの名前空間は Environment を含めず、_{app name}.local_ という形式を使っていました。 この制限により、同じ VPC に複数の Envrionment をデプロイ出来ませんでした。Copilot v1.9.0 以降で作成された Environment は、他のどの Environment とも VPC を共有できます。

Envrionment を更新すると、Copilot は Envrionment が作成された時のサービスディスカバリ名前空間を尊重します。これは、 Service のエンドポイントは変更されないことを意味しています。Copilot v1.9.0 以降のバージョンで作成した新しい Envrionment はサービスディスカバリに _{env name}.{app name}.local_  形式を利用し、古い Envrionment と VPC を共有できます。
