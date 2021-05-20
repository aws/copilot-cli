# サービス検出

サービス検出はサービス同士がお互いの位置を発見し接続できるようにする方法のことです。典型的にはサービスがパブリックエンドポイントを公開している場合のみお互いに通信でき、その場合でもリクエストはインターネット越しに通信します。 [ECS サービス検出](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/service-discovery.html) によって作成された Service にはプライベートアドレスと DNS 名が付与されるので、お互いの Service はローカルネットワーク (VPC) から離れることもパブリックエンドポイントを公開することもなく通信できます。

## サービス検出の使い方

サービス検出は Copilot CLI を使って作成された全ての Service で有効化されています。以下の Go 言語の例を通して使い方を解説します。 `kudos` という名前の Application と 2 個の Service (`api` と `front-end`) を作成したとします。

!!! attention
    Request-Driven Web Service はサービス検出の利用をサポートしていません。

この例では `front-end` Service はパブリックエンドポイントを持ち、サービス検出のエンドポイントを使って `api` Service を呼び出す場合を考えます。
 

```go
// サービス検出を使って front-end Service から api Service を呼び出す
func ServiceDiscoveryGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    endpoint := fmt.Sprintf("http://api.%s/some-request", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
    resp, err := http.Get(endpoint /* http://api.kudos.local/some-request */)
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

重要な点は `front-end` Service が `api` Service に対して特別なエンドポイントを通してリクエストを送信している点です。

```go
endpoint := fmt.Sprintf("http://api.%s/some-request", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
```

`COPILOT_SERVICE_DISCOVERY_ENDPOINT` は特別な環境変数で Copilot CLI は Service 作成時にこの環境変数を設定します。これは _{app name}.local_ というフォーマットで登録されており、今回の例だと _kudos_ Application の場合、リクエストは `http://api.kudos.local/some-request` に送信されます。 _api_ Service は 80 番ポートで動いているので、 URL のなかでポートを指定していません。しかし Service が例えば 8080 番のような別のポートで動いている場合はリクエストの中にポート番号を含める必要があります。今回の例だと `http://api.kudos.local:8080/some-request` のようになります。

`front-end` Service がリクエストを送信するとき `api.kudos.local` というエンドポイントはプライベート IP アドレスに変換され VPC のなかでプライベートにルーティングされます。
