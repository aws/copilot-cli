<div class="separator"></div>

<a id="publish" href="#publish" class="field">`publish`</a> <span class="type">Map</span>  
`publish` セクションを使用すると、サービスは 1 つまたは複数の SNS トピックにメッセージをパブリッシュできます。

```yaml
publish:
  topics:
    - name: order-events
```

上記の例では、この Manifest は、`order-events` という名前の SNS トピックを定義しています。Copilot の Environment にデプロイされた、他の Worker Service は `order-events` トピックをサブスクライブできます。

<span class="parent-field">publish.</span><a id="publish-topics" href="#publish-topics" class="field">`topics`</a> <span class="type">Array of topics</span>  
[`topic`](#publish-topics-topic) オブジェクトのリスト。

<span class="parent-field">publish.topics.</span><a id="publish-topics-topic" href="#publish-topics-topic" class="field">`topic`</a> <span class="type">Map</span>  
1 つの SNS トピックの設定を保持します。

<span class="parent-field">topic.</span><a id="topic-name" href="#topic-name" class="field">`name`</a> <span class="type">String</span>  
必須項目。SNS トピックの名前です。大文字、小文字、数字、ハイフン、アンダースコアのみを含む必要があります。

