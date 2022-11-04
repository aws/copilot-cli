<div class="separator"></div>

<a id="publish" href="#publish" class="field">`publish`</a> <span class="type">Map</span>  
`publish` セクションを使用すると、サービスは 1 つまたは複数の SNS トピックにメッセージをパブリッシュできます。

```yaml
publish:
  topics:
    - name: orderEvents
```

上記の例では、この Manifest は、Copilot の Environment にデプロイされた他の Worker Service がサブスクライブできる `orderEvents` という名前の SNS トピックを定義しています。`COPILOT_SNS_TOPIC_ARNS` という名前の環境変数が、JSON 文字列としてワークロードに設定されます。  

JavaScriptでは、次のように記述できます。
```js
const {orderEvents} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS)
```
詳しくは、[パブリッシュ / サブスクライブ](../developing/publish-subscribe.ja.md)のページをご覧ください。

<span class="parent-field">publish.</span><a id="publish-topics" href="#publish-topics" class="field">`topics`</a> <span class="type">Array of topics</span>  
[`topic`](#publish-topics-topic) オブジェクトのリスト。

<span class="parent-field">publish.topics.</span><a id="publish-topics-topic" href="#publish-topics-topic" class="field">`topic`</a> <span class="type">Map</span>  
1 つの SNS トピックの設定を保持します。

<span class="parent-field">publish.topics.topic.</span><a id="publish-topics-topic-name" href="#publish-topics-topic-name" class="field">`name`</a> <span class="type">String</span>
必須項目。SNS トピックの名前です。大文字、小文字、数字、ハイフン、アンダースコアのみを含む必要があります。

<span class="parent-field">publish.topics.topic.</span><a id="publish-topics-topic-fifo" href="#publish-topics-topic-fifo" class="field">`fifo`</a> <span class="type">Boolean or Map</span>
FIFO (first in, first out) SNS トピック設定。
`true` と指定した場合、 Copilot は FIFO でトピックを作成します。

```yaml
publish:
  topics:
    - name: mytopic
      fifo: true
```

また, SNS FIFO トピックの高度な設定も可能です。
```yaml
publish:
  topics:
    - name: mytopic
      fifo:
        content_based_deduplication: true
```

<span class="parent-field">publish.topics.topic.fifo.</span><a id="publish-topics-topic-fifo-content-based-deduplication" href="#publish-topics-topic-fifo-content-based-deduplication" class="field">`content_based_deduplication`</a> <span class="type">Boolean</span>
パブリッシュされたメッセージごとにメッセージ本文が一意であることが保証されている場合、SNS FIFO トピックの重複排除を有効化できます。